package mcp

// U3 (071.005-T) + U9 (071.009-T): MCP parity for the deterministic-gates slice.
// backlogit_doctor gains a `target` param (single-file validation, structured
// result), and backlogit_update_item gains a `size` param routed through the
// body-preserving SetArtifactSize seam.

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
)

func seedTask(t *testing.T, ws *core.Workspace) string {
	t.Helper()
	ctx := context.Background()
	feat, err := core.CreateArtifact(ctx, ws, "Gate feature", "feature")
	require.NoError(t, err)
	task, err := core.CreateArtifact(ctx, ws, "Gate task", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	return task.ID
}

func TestHandleDoctor_Target_ValidReturnsPass(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	id := seedTask(t, ws)

	path, err := core.FindArtifactPath(ctx, ws, id)
	require.NoError(t, err)
	rel, err := filepath.Rel(ws.RootPath, path)
	require.NoError(t, err)

	req := mcplib.CallToolRequest{}
	req.Params.Name = "backlogit_doctor"
	req.Params.Arguments = map[string]any{"target": rel}

	result, err := s.handleDoctor(ctx, req)
	require.NoError(t, err)
	require.False(t, result.IsError, "valid target must not be an error result")

	data := extractResultJSON(t, result)
	assert.Equal(t, "target", data["mode"])
	assert.Equal(t, true, data["ok"])
	assert.Equal(t, "pass", data["kind"])
	assert.Equal(t, id, data["artifact_id"])
	assert.NotEmpty(t, data["schema_version"])
}

func TestHandleDoctor_Target_OutOfScopeIsScopeKind(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	_ = ws

	req := mcplib.CallToolRequest{}
	req.Params.Name = "backlogit_doctor"
	req.Params.Arguments = map[string]any{"target": filepath.Join("..", "escape.md")}

	result, err := s.handleDoctor(ctx, req)
	require.NoError(t, err)
	data := extractResultJSON(t, result)
	assert.Equal(t, false, data["ok"])
	assert.Equal(t, "scope", data["kind"])
}

func TestHandleUpdateItem_Size_PersistsUnderCustomFields(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	id := seedTask(t, ws)

	before, err := db.GetItem(ctx, ws.DB, id)
	require.NoError(t, err)

	req := mcplib.CallToolRequest{}
	req.Params.Name = "backlogit_update_item"
	req.Params.Arguments = map[string]any{"id": id, "size": "M"}

	result, err := s.handleUpdateItem(ctx, req)
	require.NoError(t, err)
	require.False(t, result.IsError, "size update must succeed")

	after, err := db.GetItem(ctx, ws.DB, id)
	require.NoError(t, err)
	require.NotNil(t, after.CustomFields)
	assert.Equal(t, "M", after.CustomFields["size"])
	assert.Equal(t, before.Title, after.Title, "title must be preserved")
	assert.Equal(t, before.Status, after.Status, "status must be preserved")
	assert.Equal(t, before.Priority, after.Priority, "priority must be preserved")
}

func TestHandleUpdateItem_Size_MutualExclusion(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	id := seedTask(t, ws)

	req := mcplib.CallToolRequest{}
	req.Params.Name = "backlogit_update_item"
	req.Params.Arguments = map[string]any{"id": id, "size": "M", "status": "done"}

	result, err := s.handleUpdateItem(ctx, req)
	require.NoError(t, err)
	require.True(t, result.IsError, "combining size with other updates must be a validation error")
}

func TestHandleUpdateItem_Size_InvalidEnum(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	id := seedTask(t, ws)

	req := mcplib.CallToolRequest{}
	req.Params.Name = "backlogit_update_item"
	req.Params.Arguments = map[string]any{"id": id, "size": "XXL"}

	result, err := s.handleUpdateItem(ctx, req)
	require.NoError(t, err)
	require.True(t, result.IsError, "out-of-enum size must be an error result")

	// An out-of-enum size is user-correctable and MUST surface as
	// validation_failed (422), not an opaque internal (500) error, so callers
	// can handle it deterministically.
	require.NotEmpty(t, result.Content)
	text, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok, "expected text content, got %T", result.Content[0])
	var resp struct {
		Error string `json:"error"`
	}
	require.NoError(t, json.Unmarshal([]byte(text.Text), &resp))
	assert.Equal(t, "validation_failed", resp.Error, "invalid size must map to validation_failed")
}
