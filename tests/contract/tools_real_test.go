package contract_test

// Tests for TASK-008.06: Strengthen contract tests with real handler invocations.
// These tests exercise actual MCP tool handlers against a real workspace,
// asserting on response values rather than just key existence.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mark3labs/mcp-go/client"
	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/backlogit/backlogit/internal/config"
	"github.com/backlogit/backlogit/internal/core"
	mcpinternal "github.com/backlogit/backlogit/internal/mcp"
)

func setupRealMCPServer(t *testing.T) *mcpinternal.Server {
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

func callToolForTest(t *testing.T, s *mcpinternal.Server, toolName string, args map[string]any) (*mcplib.CallToolResult, error) {
	t.Helper()
	ctx := context.Background()
	c, err := client.NewInProcessClient(s.MCPServer())
	if err != nil {
		return nil, fmt.Errorf("create in-process client: %w", err)
	}
	if err := c.Start(ctx); err != nil {
		return nil, fmt.Errorf("start in-process client: %w", err)
	}
	defer c.Close() //nolint:errcheck

	initReq := mcplib.InitializeRequest{}
	initReq.Params.ClientInfo = mcplib.Implementation{Name: "test", Version: "0.0.1"}
	initReq.Params.ProtocolVersion = mcplib.LATEST_PROTOCOL_VERSION
	if _, err := c.Initialize(ctx, initReq); err != nil {
		return nil, fmt.Errorf("initialize in-process client: %w", err)
	}

	req := mcplib.CallToolRequest{}
	req.Params.Name = toolName
	req.Params.Arguments = args
	return c.CallTool(ctx, req)
}

func callToolAndParseJSON(t *testing.T, s *mcpinternal.Server, toolName string, args map[string]any) map[string]any {
	t.Helper()
	result, err := callToolForTest(t, s, toolName, args)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.IsError, "tool call should not return error")
	require.NotEmpty(t, result.Content)

	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok, "expected TextContent in result")

	var data map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &data))
	return data
}

// TASK-008.06: Create item through real handler and verify artifact data.
func TestCreateItem_Real_CreatesArtifactFile(t *testing.T) {
	// Arrange
	s := setupRealMCPServer(t)

	// Act
	data := callToolAndParseJSON(t, s, "backlogit_create_item", map[string]any{
		"title":         "Real contract test task",
		"artifact_type": "task",
		"status":        "queued",
		"description":   "Created by contract test",
	})

	// Assert — real values, not just key existence
	assert.NotEmpty(t, data["id"], "created artifact must have an ID")
	assert.Equal(t, "Real contract test task", data["title"])
	assert.Equal(t, "task", data["artifact_type"])
}

// TASK-008.06: Get item through real handler returns actual artifact data.
func TestGetItem_Real_ReturnsArtifactData(t *testing.T) {
	// Arrange
	s := setupRealMCPServer(t)

	// Create an artifact first
	createData := callToolAndParseJSON(t, s, "backlogit_create_item", map[string]any{
		"title":         "Get test artifact",
		"artifact_type": "feature",
	})
	id := createData["id"].(string)

	// Act
	getData := callToolAndParseJSON(t, s, "backlogit_get_item", map[string]any{
		"id": id,
	})

	// Assert — verify actual data, not just presence
	assert.Equal(t, id, getData["id"])
	assert.Equal(t, "Get test artifact", getData["title"])
	assert.Equal(t, "feature", getData["artifact_type"])
}

// TASK-008.06: Update item modifies fields and returns updated artifact.
func TestUpdateItem_Real_ModifiesFields(t *testing.T) {
	// Arrange
	s := setupRealMCPServer(t)
	createData := callToolAndParseJSON(t, s, "backlogit_create_item", map[string]any{
		"title":         "Update target",
		"artifact_type": "task",
	})
	id := createData["id"].(string)

	// Act
	updateData := callToolAndParseJSON(t, s, "backlogit_update_item", map[string]any{
		"id":       id,
		"title":    "Updated title",
		"priority": "high",
	})

	// Assert — fields should actually be updated
	assert.Equal(t, id, updateData["id"])
	assert.Equal(t, "Updated title", updateData["title"])
}

// TASK-008.06: Invalid artifact type returns an error result.
func TestCreateItem_Real_InvalidTypeReturnsError(t *testing.T) {
	// Arrange
	s := setupRealMCPServer(t)

	// Act
	result, err := callToolForTest(t, s, "backlogit_create_item", map[string]any{
		"title":         "Should fail",
		"artifact_type": "nonexistent_type",
	})

	// Assert — should return an error result
	require.NoError(t, err) // Go error should be nil; MCP error is in result
	require.NotNil(t, result)
	assert.True(t, result.IsError, "invalid type should produce error result")
}

// TASK-008.06: Get item with missing ID returns an error result.
func TestGetItem_Real_MissingIDReturnsError(t *testing.T) {
	// Arrange
	s := setupRealMCPServer(t)

	// Act
	result, err := callToolForTest(t, s, "backlogit_get_item", map[string]any{
		"id": "NONEXISTENT999",
	})

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "missing item should produce error result")
}

func TestStash_Real_AddFetchAndHarvest(t *testing.T) {
	s := setupRealMCPServer(t)

	stashData := callToolAndParseJSON(t, s, "backlogit_stash", map[string]any{
		"kind":     "feature",
		"priority": "critical",
		"text":     "Group security cleanup work",
	})
	stashID := stashData["id"].(string)
	assert.NotEmpty(t, stashID)
	assert.Equal(t, "critical", stashData["priority"])

	fetchResult, err := callToolForTest(t, s, "backlogit_fetch_stash", map[string]any{"priority": "critical"})
	require.NoError(t, err)
	require.NotNil(t, fetchResult)
	require.False(t, fetchResult.IsError)

	harvestData := callToolAndParseJSON(t, s, "backlogit_harvest_stash", map[string]any{
		"stash_id":      stashID,
		"artifact_type": "feature",
		"description":   "Harvested from stash",
	})
	entry := harvestData["entry"].(map[string]any)
	artifact := harvestData["artifact"].(map[string]any)
	assert.Equal(t, stashID, entry["id"])
	assert.Equal(t, "critical", entry["priority"])
	assert.Equal(t, "feature", artifact["artifact_type"])
	assert.Equal(t, "critical", artifact["priority"])
}
