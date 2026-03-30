package mcp_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/config"
	"github.com/backlogit/backlogit/internal/core"
	mcpinternal "github.com/backlogit/backlogit/internal/mcp"
)

// TASK-002.05.02: Implement dynamic MCP tool generation from templates.

func setupDynamicTestServer(t *testing.T) *mcpinternal.Server {
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

func TestRegisterDynamicTools_CreatesTypeSpecificTools(t *testing.T) {
	// Arrange
	s := setupDynamicTestServer(t)
	templates := []mcpinternal.DynamicTemplateInput{
		{
			Name:         "task-template",
			ArtifactType: "task",
			Sections: []mcpinternal.DynamicSectionInput{
				{Name: "description", Flag: "description", Required: true},
				{Name: "acceptance-criteria", Flag: "acceptance-criteria", Required: false},
			},
		},
		{
			Name:         "bug-template",
			ArtifactType: "bug",
			Sections: []mcpinternal.DynamicSectionInput{
				{Name: "steps-to-reproduce", Flag: "steps-to-reproduce", Required: true},
			},
		},
	}

	// Act
	err := mcpinternal.RegisterDynamicTools(s, templates)

	// Assert
	require.NoError(t, err)
	tools := s.ListTools()
	assert.Contains(t, tools, "backlogit_create_task")
	assert.Contains(t, tools, "backlogit_create_bug")
	assert.Contains(t, tools, "backlogit_update_task_section")
	assert.Contains(t, tools, "backlogit_update_bug_section")
}

func TestRegisterDynamicTools_EmptyTemplates(t *testing.T) {
	// Arrange
	s := setupDynamicTestServer(t)

	// Act
	err := mcpinternal.RegisterDynamicTools(s, nil)

	// Assert
	require.NoError(t, err)
}

func TestRegisterDynamicTools_RejectsStaticCollision(t *testing.T) {
	// Arrange — "create_item" would collide with static "backlogit_create_item"
	s := setupDynamicTestServer(t)
	templates := []mcpinternal.DynamicTemplateInput{
		{
			Name:         "item-template",
			ArtifactType: "item",
			Sections: []mcpinternal.DynamicSectionInput{
				{Name: "desc", Flag: "desc", Required: true},
			},
		},
	}

	// Act
	err := mcpinternal.RegisterDynamicTools(s, templates)

	// Assert — should reject because "backlogit_create_item" already exists
	require.Error(t, err)
	assert.Contains(t, err.Error(), "collision")
}

func TestRegisterDynamicTools_DuplicateTypeRejected(t *testing.T) {
	// Arrange — two templates for the same type
	s := setupDynamicTestServer(t)
	templates := []mcpinternal.DynamicTemplateInput{
		{Name: "task-1", ArtifactType: "task", Sections: []mcpinternal.DynamicSectionInput{{Name: "desc", Flag: "desc", Required: true}}},
		{Name: "task-2", ArtifactType: "task", Sections: []mcpinternal.DynamicSectionInput{{Name: "notes", Flag: "notes", Required: false}}},
	}

	// Act
	err := mcpinternal.RegisterDynamicTools(s, templates)

	// Assert
	require.Error(t, err)
}
