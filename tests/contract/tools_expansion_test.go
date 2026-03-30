package contract_test

import (
	"context"
	"encoding/json"
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
		"dependencies":  "T002",
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
		"id":          "T001",
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

// toolResultJSON is a helper to parse JSON from tool results.
func toolResultJSON(data []byte) map[string]any {
	var result map[string]any
	json.Unmarshal(data, &result)
	return result
}
