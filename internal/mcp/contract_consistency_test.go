package mcp

// 026.015-T: MCP contract consistency integration tests.
//
// These tests exercise cross-cutting invariants of the MCP layer:
//
//   - Not-found errors from any handler never surface as error="internal".
//   - List and Get shipment handlers produce identical per-item shapes.
//   - Concurrent ensureWorkspace calls produce no races and return one workspace.
//   - Deleting an item removes all orphaned satellite rows.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
)

// contractRequest builds a minimal CallToolRequest from a string argument map.
func contractRequest(args map[string]any) mcplib.CallToolRequest {
	req := mcplib.CallToolRequest{}
	req.Params.Arguments = args
	return req
}

// contractErrorType extracts the "error" field from a CallToolResult that is
// expected to be an error.
func contractErrorType(t *testing.T, result *mcplib.CallToolResult) string {
	t.Helper()
	require.True(t, result.IsError, "expected error result but got success")
	require.NotEmpty(t, result.Content)
	text, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok, "expected text content, got %T", result.Content[0])
	var resp struct {
		Error string `json:"error"`
	}
	require.NoError(t, json.Unmarshal([]byte(text.Text), &resp))
	return resp.Error
}

// TestMCP_NotFoundErrors_NeverSurfaceAsInternal calls each handler with a
// non-existent ID and asserts the response is error="not_found", never
// error="internal". This is the regression guard for 026.011-T.
func TestMCP_NotFoundErrors_NeverSurfaceAsInternal(t *testing.T) {
	s, _ := setupBugFixServer(t)
	ctx := context.Background()

	tests := []struct {
		name    string
		handler func() (*mcplib.CallToolResult, error)
	}{
		{
			name: "handleGetItem missing id",
			handler: func() (*mcplib.CallToolResult, error) {
				return s.handleGetItem(ctx, contractRequest(map[string]any{"id": "MISSING-001"}))
			},
		},
		{
			name: "handleDeleteItem missing id",
			handler: func() (*mcplib.CallToolResult, error) {
				return s.handleDeleteItem(ctx, contractRequest(map[string]any{"id": "MISSING-002"}))
			},
		},
		{
			name: "handleMoveItem missing id",
			handler: func() (*mcplib.CallToolResult, error) {
				return s.handleMoveItem(ctx, contractRequest(map[string]any{
					"id":     "MISSING-003",
					"status": "done",
				}))
			},
		},
		{
			name: "handleGetShipment missing id",
			handler: func() (*mcplib.CallToolResult, error) {
				return s.handleGetShipment(ctx, contractRequest(map[string]any{"id": "MISSING-004"}))
			},
		},
		{
			name: "handleArchiveItem missing id",
			handler: func() (*mcplib.CallToolResult, error) {
				return s.handleArchiveItem(ctx, contractRequest(map[string]any{"id": "MISSING-005"}))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.handler()
			require.NoError(t, err)
			errType := contractErrorType(t, result)
			assert.NotEqual(t, "internal", errType,
				"%s: not-found must not surface as error=internal (got %q)", tt.name, errType)
			assert.Equal(t, "not_found", errType,
				"%s: missing resource must return error=not_found", tt.name)
		})
	}
}

// TestMCP_ShipmentList_SameShapeAsGet verifies that each shipment returned by
// handleListShipments has the same structure (id, status, custom_fields.items)
// as the response from handleGetShipment for the same ID.
func TestMCP_ShipmentList_SameShapeAsGet(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()

	feat, err := core.CreateArtifact(ctx, ws, "Contract feature", "feature")
	require.NoError(t, err)
	task, err := core.CreateArtifact(ctx, ws, "Contract task", "task", core.WithParent(feat.ID))
	require.NoError(t, err)

	ship, err := core.CreateShipment(ctx, ws, "Contract shipment", []string{task.ID})
	require.NoError(t, err)

	// Get via single-item handler.
	getResult, err := s.handleGetShipment(ctx, contractRequest(map[string]any{"id": ship.ID}))
	require.NoError(t, err)
	require.False(t, getResult.IsError)
	var getShape map[string]any
	getText, _ := getResult.Content[0].(mcplib.TextContent)
	require.NoError(t, json.Unmarshal([]byte(getText.Text), &getShape))

	// Get via list handler.
	listResult, err := s.handleListShipments(ctx, contractRequest(map[string]any{}))
	require.NoError(t, err)
	require.False(t, listResult.IsError)
	var listShapes []map[string]any
	listText, _ := listResult.Content[0].(mcplib.TextContent)
	require.NoError(t, json.Unmarshal([]byte(listText.Text), &listShapes))

	var listShape map[string]any
	for _, m := range listShapes {
		if m["id"] == ship.ID {
			listShape = m
			break
		}
	}
	require.NotNil(t, listShape, "shipment must appear in list result")

	// Both shapes must agree on all scalar fields.
	for _, key := range []string{"id", "title", "status", "artifact_type"} {
		assert.Equal(t, getShape[key], listShape[key],
			"field %q differs between get and list", key)
	}

	// Both must have custom_fields.items as a non-null array.
	getCF, _ := getShape["custom_fields"].(map[string]any)
	listCF, _ := listShape["custom_fields"].(map[string]any)
	require.NotNil(t, getCF["items"], "get custom_fields.items must not be null")
	require.NotNil(t, listCF["items"], "list custom_fields.items must not be null")

	_, getIsArray := getCF["items"].([]interface{})
	_, listIsArray := listCF["items"].([]interface{})
	assert.True(t, getIsArray, "get custom_fields.items must be a JSON array")
	assert.True(t, listIsArray, "list custom_fields.items must be a JSON array")
}

// TestMCP_ConcurrentEnsureWorkspace_NoRace spawns goroutines that all call
// ensureWorkspace simultaneously. Validates pointer identity (all get the same
// workspace) and confirms no panic. Run the package with -race to fully
// exercise the mutex.
func TestMCP_ConcurrentEnsureWorkspace_NoRace(t *testing.T) {
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	s := NewServerForRoot(root)
	ctx := context.Background()

	const goroutines = 30
	results := make([]*core.Workspace, goroutines)
	errs := make([]error, goroutines)
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := range goroutines {
		go func(idx int) {
			defer wg.Done()
			results[idx], errs[idx] = s.ensureWorkspace(ctx)
		}(i)
	}
	wg.Wait()

	if results[0] != nil {
		t.Cleanup(func() { results[0].Close() })
	}

	for i, e := range errs {
		require.NoError(t, e, "goroutine %d must not return an error", i)
	}
	for i := 1; i < goroutines; i++ {
		assert.Same(t, results[0], results[i],
			"goroutine %d received a different workspace pointer", i)
	}
}

// TestMCP_DeleteItem_LeavesNoOrphans creates an item with dependencies and
// links, deletes it via handleDeleteItem, then queries the DB directly to
// confirm that no satellite rows reference the deleted item's ID.
func TestMCP_DeleteItem_LeavesNoOrphans(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()

	feat, err := core.CreateArtifact(ctx, ws, "Orphan feature", "feature")
	require.NoError(t, err)
	taskA, err := core.CreateArtifact(ctx, ws, "Task A", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	taskB, err := core.CreateArtifact(ctx, ws, "Task B", "task", core.WithParent(feat.ID))
	require.NoError(t, err)

	// Seed a dependency and a link from taskA.
	require.NoError(t, db.UpsertDependency(ctx, ws.DB, taskA.ID, taskB.ID, "blocks"))
	require.NoError(t, db.AddLink(ctx, ws.DB, taskA.ID, taskB.ID, "related_to"))

	// Delete taskA via the MCP handler.
	result, err := s.handleDeleteItem(ctx, contractRequest(map[string]any{
		"id": taskA.ID,
	}))
	require.NoError(t, err)
	require.False(t, result.IsError, "delete of existing item must succeed")

	// Verify item row is gone.
	_, getErr := db.GetItem(ctx, ws.DB, taskA.ID)
	assert.Error(t, getErr, "deleted item must not be retrievable from DB")

	// Verify no satellite rows reference taskA.ID.
	tables := []struct{ table, col string }{
		{"item_deps", "item_id"},
		{"item_deps", "depends_on"},
		{"item_links", "source_id"},
		{"item_links", "target_id"},
		{"commit_links", "item_id"},
		{"item_log_entries", "item_id"},
		{"item_logs", "item_id"},
	}
	for _, tc := range tables {
		var count int
		row := ws.DB.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM "+tc.table+" WHERE "+tc.col+" = ?", taskA.ID)
		require.NoError(t, row.Scan(&count))
		assert.Zero(t, count,
			"%s.%s must have no rows referencing deleted item %s",
			tc.table, tc.col, taskA.ID)
	}
}
