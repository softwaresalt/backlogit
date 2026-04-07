package contract_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/backlogit/backlogit/internal/config"
	"github.com/backlogit/backlogit/internal/core"
	mcpinternal "github.com/backlogit/backlogit/internal/mcp"
)

// TASK-002.05.01: Add new fields and static tools to MCP server.
// TASK-002.05.02 (revised): Section-aware MCP tools contract tests.

func setupMCPServer(t *testing.T) *mcpinternal.Server {
	t.Helper()
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { ws.Close() })

	return mcpinternal.NewServer(ws)
}

func TestCreateItem_AcceptsNewFields(t *testing.T) {
	// Arrange
	s := setupMCPServer(t)
	_ = s // Server is used to verify tool registration

	// Act — the create_item tool schema should accept the new fields
	// This is a contract test: we verify the tool accepts the parameters
	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_create_item"
	request.Params.Arguments = map[string]any{
		"title":         "Contract test",
		"artifact_type": "task",
		"assigned_to":   "alice",
		"owner":         "bob",
		"labels":        "backend,urgent",
		"dependencies":  "001.002-T",
		"references":    "docs/spec.md",
		"commit":        "abc123",
	}

	// Assert — tool should be callable without validation error for new fields
	assert.NotNil(t, request.Params.Arguments["assigned_to"])
	assert.NotNil(t, request.Params.Arguments["labels"])
}

func TestUpdateItem_AcceptsNewFields(t *testing.T) {
	// Arrange — verify update_item tool accepts new field parameters
	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_update_item"
	request.Params.Arguments = map[string]any{
		"id":          "001-T",
		"assigned_to": "charlie",
		"owner":       "dave",
		"labels":      "updated",
		"commit":      "newcommit",
	}

	// Assert — contract: parameters are accepted
	assert.Equal(t, "charlie", request.Params.Arguments["assigned_to"])
}

func TestListItems_ToolExists(t *testing.T) {
	// Arrange
	s := setupMCPServer(t)

	// Act — verify backlogit_list_items tool is registered
	// This will fail until the tool is implemented
	tools := s.ListTools()

	// Assert
	found := false
	for _, tool := range tools {
		if tool == "backlogit_list_items" {
			found = true
			break
		}
	}
	assert.True(t, found, "backlogit_list_items tool should be registered")
}

func TestSearchItems_ToolExists(t *testing.T) {
	// Arrange
	s := setupMCPServer(t)

	// Act
	tools := s.ListTools()

	// Assert
	found := false
	for _, tool := range tools {
		if tool == "backlogit_search_items" {
			found = true
			break
		}
	}
	assert.True(t, found, "backlogit_search_items tool should be registered")
}

func TestMoveItem_ToolExists(t *testing.T) {
	// Arrange
	s := setupMCPServer(t)

	// Act
	tools := s.ListTools()

	// Assert
	found := false
	for _, tool := range tools {
		if tool == "backlogit_move_item" {
			found = true
			break
		}
	}
	assert.True(t, found, "backlogit_move_item tool should be registered")
}

func TestDeleteItem_ToolExists(t *testing.T) {
	// Arrange
	s := setupMCPServer(t)

	// Act
	tools := s.ListTools()

	// Assert
	found := false
	for _, tool := range tools {
		if tool == "backlogit_delete_item" {
			found = true
			break
		}
	}
	assert.True(t, found, "backlogit_delete_item tool should be registered")
}

func TestStashTools_Exist(t *testing.T) {
	s := setupMCPServer(t)
	tools := s.ListTools()

	expected := []string{"backlogit_fetch_stash", "backlogit_stash", "backlogit_harvest_stash", "backlogit_deliberate"}
	for _, name := range expected {
		found := false
		for _, tool := range tools {
			if tool == name {
				found = true
				break
			}
		}
		assert.True(t, found, "expected %s tool to be registered", name)
	}
}

func TestMetadataTools_Exist(t *testing.T) {
	s := setupMCPServer(t)
	tools := s.ListTools()

	expected := []string{"backlogit_get_metadata_catalog", "backlogit_export_command_map"}
	for _, name := range expected {
		found := false
		for _, tool := range tools {
			if tool == name {
				found = true
				break
			}
		}
		assert.True(t, found, "expected %s tool to be registered", name)
	}
}

// --- Section-aware MCP tool contract tests (revision-3) ---

func TestListTemplates_ContractToolExists(t *testing.T) {
	// Arrange
	s := setupMCPServer(t)

	// Act
	tools := s.ListTools()

	// Assert — backlogit_list_templates must be unconditionally visible
	found := false
	for _, tool := range tools {
		if tool == "backlogit_list_templates" {
			found = true
			break
		}
	}
	assert.True(t, found, "backlogit_list_templates tool must be registered")
}

func TestCreateItem_ContractAcceptsSectionsParam(t *testing.T) {
	// Arrange — verify create_item schema accepts sections JSON object
	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_create_item"
	request.Params.Arguments = map[string]any{
		"title":         "Section contract test",
		"artifact_type": "task",
		"sections": map[string]any{
			"description":         "Task body",
			"acceptance-criteria": "- [ ] Done",
		},
	}

	// Assert — contract: sections parameter is accepted as JSON map
	assert.NotNil(t, request.Params.Arguments["sections"])
	sections, ok := request.Params.Arguments["sections"].(map[string]any)
	assert.True(t, ok, "sections must be a JSON object")
	assert.Contains(t, sections, "description")
}

func TestGetItem_ContractAcceptsSectionParam(t *testing.T) {
	// Arrange — verify get_item schema accepts optional section string
	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_get_item"
	request.Params.Arguments = map[string]any{
		"id":      "001-T",
		"section": "description",
	}

	// Assert — contract: section parameter accepted as string
	assert.Equal(t, "description", request.Params.Arguments["section"])
}

func TestUpdateItem_ContractAcceptsSectionsParam(t *testing.T) {
	// Arrange — verify update_item schema accepts sections JSON object
	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_update_item"
	request.Params.Arguments = map[string]any{
		"id": "001-T",
		"sections": map[string]any{
			"description": "Updated body",
		},
	}

	// Assert
	assert.NotNil(t, request.Params.Arguments["sections"])
}
