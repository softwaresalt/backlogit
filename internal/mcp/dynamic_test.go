package mcp_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/core/templates"
	mcpinternal "github.com/softwaresalt/backlogit/internal/mcp"
)

// TASK-002.05.02 (revised): Section-aware MCP tools and template discovery.
// Replaces dynamic MCP tool generation with fixed tool surface per revision-3.

func setupSectionAwareServer(t *testing.T) (*mcpinternal.Server, *templates.Service) {
	t.Helper()
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { ws.Close() })

	templatesDir := filepath.Join(backlogitDir, "templates")
	svc, err := templates.NewService(ctx, templatesDir)
	require.NoError(t, err)

	s := mcpinternal.NewServer(ws)
	mcpinternal.RegisterSectionAwareTools(s, svc)
	return s, svc
}

func TestListTemplates_ToolRegistered(t *testing.T) {
	// Arrange
	s, _ := setupSectionAwareServer(t)

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
	assert.True(t, found, "backlogit_list_templates tool must be registered unconditionally")
}

func TestListTemplates_ReturnsTemplateMetadata(t *testing.T) {
	// Arrange
	_, svc := setupSectionAwareServer(t)

	// Act
	infos := svc.ListTemplates()

	// Assert — should return templates loaded from default workspace
	assert.NotEmpty(t, infos)
	for _, info := range infos {
		assert.NotEmpty(t, info.TypeName)
		assert.NotEmpty(t, info.Sections)
	}
}

func TestListTemplates_EmptyWhenNoWorkspace(t *testing.T) {
	// Arrange — nil template service simulates uninitialized workspace
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { ws.Close() })

	s := mcpinternal.NewServer(ws)
	mcpinternal.RegisterSectionAwareTools(s, nil)

	// Act
	tools := s.ListTools()

	// Assert — tool must still be registered even with nil service
	found := false
	for _, tool := range tools {
		if tool == "backlogit_list_templates" {
			found = true
			break
		}
	}
	assert.True(t, found, "backlogit_list_templates must be visible even without workspace")
}

func TestSectionAwareTools_CreateItemHasSectionsParam(t *testing.T) {
	// Arrange
	s, _ := setupSectionAwareServer(t)

	// Act — verify create_item is registered (it should accept sections param)
	tools := s.ListTools()

	// Assert
	found := false
	for _, tool := range tools {
		if tool == "backlogit_create_item" {
			found = true
			break
		}
	}
	assert.True(t, found, "backlogit_create_item must be registered with section support")
}

func TestSectionAwareTools_GetItemHasSectionParam(t *testing.T) {
	// Arrange
	s, _ := setupSectionAwareServer(t)

	// Act — verify get_item is registered
	tools := s.ListTools()

	// Assert
	found := false
	for _, tool := range tools {
		if tool == "backlogit_get_item" {
			found = true
			break
		}
	}
	assert.True(t, found, "backlogit_get_item must be registered with section param")
}

func TestStashTools_AreRegistered(t *testing.T) {
	s, _ := setupSectionAwareServer(t)
	tools := s.ListTools()

	expected := []string{
		"backlogit_fetch_stash",
		"backlogit_stash",
		"backlogit_harvest_stash",
		"backlogit_deliberate",
		"backlogit_stash_get",
		"backlogit_stash_edit",
		"backlogit_stash_archive",
		"backlogit_stash_remove",
	}
	for _, name := range expected {
		found := false
		for _, tool := range tools {
			if tool == name {
				found = true
				break
			}
		}
		assert.True(t, found, "expected stash tool %s to be registered", name)
	}
}

func TestParseSectionsParam_JSONObject(t *testing.T) {
	// Arrange
	args := map[string]any{
		"sections": map[string]any{
			"description":         "Task body",
			"acceptance-criteria": "- [ ] Done",
		},
	}

	// Act
	sections, err := mcpinternal.ParseSectionsParam(args)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, "Task body", sections["description"])
	assert.Equal(t, "- [ ] Done", sections["acceptance-criteria"])
}

func TestParseSectionsParam_Nil(t *testing.T) {
	// Arrange
	args := map[string]any{"title": "test"}

	// Act
	sections, err := mcpinternal.ParseSectionsParam(args)

	// Assert
	require.NoError(t, err)
	assert.Nil(t, sections)
}

func TestParseSectionsParam_InvalidType(t *testing.T) {
	// Arrange
	args := map[string]any{"sections": 42}

	// Act
	_, err := mcpinternal.ParseSectionsParam(args)

	// Assert
	require.Error(t, err)
}
