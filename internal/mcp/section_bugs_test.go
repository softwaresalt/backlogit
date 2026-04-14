package mcp

// Tests for TASK-008.01 (section extraction in handleGetItem) and
// TASK-008.02 (wire sections param through handleCreateItem).
//
// These tests use package-internal access to call unexported handlers directly.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/core/templates"
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

func setupBugFixServer(t *testing.T) (*Server, *core.Workspace) {
	t.Helper()
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { ws.Close() })

	s := NewServer(ws)

	// Wire a real template service (008.02 fix target: NewServer currently passes nil)
	templatesDir := filepath.Join(backlogitDir, "templates")
	svc, err := templates.NewService(ctx, templatesDir)
	require.NoError(t, err)
	RegisterSectionAwareTools(s, svc)

	return s, ws
}

func seedArtifactWithSections(t *testing.T, ws *core.Workspace) *models.Artifact {
	t.Helper()
	ctx := context.Background()
	artifact, err := core.CreateArtifact(ctx, ws, "Feature with sections", "feature")
	require.NoError(t, err)

	filePath, err := core.FindArtifactPath(ctx, ws, artifact.ID)
	require.NoError(t, err)
	raw, err := os.ReadFile(filePath)
	require.NoError(t, err)

	content := string(raw) +
		"\n<!-- BEGIN:description -->\nThis is the description section\n<!-- END:description -->\n" +
		"\n<!-- BEGIN:acceptance-criteria -->\n- [ ] Criterion 1\n- [ ] Criterion 2\n<!-- END:acceptance-criteria -->\n"
	require.NoError(t, os.WriteFile(filePath, []byte(content), 0o644))

	return artifact
}

func extractResultJSON(t *testing.T, result *mcplib.CallToolResult) map[string]any {
	t.Helper()
	require.NotNil(t, result)
	require.NotEmpty(t, result.Content)
	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok, "expected TextContent in result")
	var data map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &data))
	return data
}

// TASK-008.01: handleGetItem with section param returns only named section content.
func TestHandleGetItem_WithSection_ReturnsNamedSection(t *testing.T) {
	// Arrange
	s, ws := setupBugFixServer(t)
	artifact := seedArtifactWithSections(t, ws)
	ctx := context.Background()

	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_get_item"
	request.Params.Arguments = map[string]any{
		"id":      artifact.ID,
		"section": "description",
	}

	// Act
	result, err := s.handleGetItem(ctx, request)

	// Assert
	require.NoError(t, err)
	data := extractResultJSON(t, result)
	content, ok := data["content"].(string)
	require.True(t, ok, "response should have 'content' string field")
	assert.Contains(t, content, "This is the description section")
	assert.NotContains(t, content, "acceptance-criteria",
		"should return only the requested section, not others")
}

// TASK-008.01: handleGetItem with missing section returns descriptive error.
func TestHandleGetItem_WithSection_MissingSectionReturnsError(t *testing.T) {
	// Arrange
	s, ws := setupBugFixServer(t)
	artifact := seedArtifactWithSections(t, ws)
	ctx := context.Background()

	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_get_item"
	request.Params.Arguments = map[string]any{
		"id":      artifact.ID,
		"section": "nonexistent-section",
	}

	// Act
	result, err := s.handleGetItem(ctx, request)

	// Assert — should return error, not empty output
	require.NoError(t, err) // MCP errors are in the result, not the Go error
	require.NotNil(t, result)
	assert.True(t, result.IsError, "missing section should produce an error result")
}

// TASK-008.01: handleGetItem without section param returns full artifact (existing behavior).
func TestHandleGetItem_WithoutSection_ReturnsFull(t *testing.T) {
	// Arrange
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	artifact, err := core.CreateArtifact(ctx, ws, "Normal feature", "feature")
	require.NoError(t, err)
	require.NoError(t, db.UpsertItem(ctx, ws.DB, artifact))

	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_get_item"
	request.Params.Arguments = map[string]any{
		"id": artifact.ID,
	}

	// Act
	result, err := s.handleGetItem(ctx, request)

	// Assert — full artifact data returned
	require.NoError(t, err)
	data := extractResultJSON(t, result)
	assert.Equal(t, artifact.ID, data["id"])
	assert.Equal(t, "Normal feature", data["title"])
}

// TASK-008.02: handleCreateItem with sections param writes section content to file.
func TestHandleCreateItem_WithSections_WritesContent(t *testing.T) {
	// Arrange
	s, _ := setupBugFixServer(t)
	ctx := context.Background()

	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_create_item"
	request.Params.Arguments = map[string]any{
		"title":         "Feature with sections",
		"artifact_type": "feature",
		"sections": map[string]any{
			"description":         "This is the task description",
			"acceptance-criteria": "- [ ] Must pass all tests",
		},
	}

	// Act
	result, err := s.handleCreateItem(ctx, request)

	// Assert — artifact created and sections written to file
	require.NoError(t, err)
	data := extractResultJSON(t, result)
	id, ok := data["id"].(string)
	require.True(t, ok, "response should contain artifact ID")

	// Read the created file and verify sections are present
	ws := s.Workspace
	filePath, err := core.FindArtifactPath(ctx, ws, id)
	require.NoError(t, err)
	raw, err := os.ReadFile(filePath)
	require.NoError(t, err)
	content := string(raw)
	assert.Contains(t, content, "This is the task description",
		"file should contain the description section content")
	assert.Contains(t, content, "Must pass all tests",
		"file should contain the acceptance-criteria section content")
}

// Test for writeSectionsToFile mixed case: one section already exists in the
// artifact body, another is missing. Only the missing section should be appended;
// the existing section should be updated in place without duplication.
func TestWriteSectionsToFile_MixedExistingAndNew(t *testing.T) {
	_, ws := setupBugFixServer(t)
	ctx := context.Background()

	// Create artifact with one pre-existing section ("description").
	artifact := seedArtifactWithSections(t, ws)

	// Write two sections: "description" (exists) and "notes" (new).
	sections := map[string]string{
		"description": "Updated description content",
		"notes":       "Brand-new notes section",
	}
	err := writeSectionsToFile(ctx, ws, artifact, sections)
	require.NoError(t, err)

	// Read back and verify.
	filePath, err := core.FindArtifactPath(ctx, ws, artifact.ID)
	require.NoError(t, err)
	raw, err := os.ReadFile(filePath)
	require.NoError(t, err)
	content := string(raw)

	// The existing section should be updated, not duplicated.
	assert.Contains(t, content, "Updated description content",
		"existing section should be updated")
	assert.NotContains(t, content, "This is the description section",
		"old section content should be replaced, not duplicated")

	// The new section should be appended.
	assert.Contains(t, content, "Brand-new notes section",
		"missing section should be appended")
	assert.Contains(t, content, "<!-- BEGIN:notes -->",
		"appended section should have BEGIN marker")
	assert.Contains(t, content, "<!-- END:notes -->",
		"appended section should have END marker")

	// acceptance-criteria (pre-existing, not in update map) should be untouched.
	assert.Contains(t, content, "Criterion 1",
		"sections not in update map should be preserved")
}
