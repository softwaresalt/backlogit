package mcp

// 136-F/U11: backlogit_abandon_checkpoint and backlogit_quarantine_checkpoint
// MCP tool handler tests. Both tools require an explicit, non-empty operator
// parameter — it is never inferred on the MCP surface — and route through the
// shared core.AbandonCheckpoint / core.QuarantineCheckpoint verbs used by the
// CLI fallback.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func writeCheckpointFileMCP(t *testing.T, root, filename, body string) {
	t.Helper()
	dir := filepath.Join(root, ".backlogit", "checkpoints")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, filename), []byte(body), 0o644))
}

func TestHandleAbandonCheckpoint_HappyPath(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	writeCheckpointFileMCP(t, ws.RootPath, "checkpoint-mcp-abandon.json",
		`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active","created_at":"2026-08-01T00:00:00Z","updated_at":"2026-08-01T00:00:00Z"}`)

	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_abandon_checkpoint"
	request.Params.Arguments = map[string]any{
		"filename": "checkpoint-mcp-abandon.json",
		"reason":   "superseded",
		"operator": "tester@example.com",
	}

	result, err := s.handleAbandonCheckpoint(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.IsError, "abandon must succeed for a valid target")
	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &resp))
	assert.Equal(t, "abandoned", resp["disposition"])
}

func TestHandleAbandonCheckpoint_MissingOperator(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	writeCheckpointFileMCP(t, ws.RootPath, "checkpoint-mcp-abandon2.json",
		`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active","created_at":"2026-08-01T00:00:00Z","updated_at":"2026-08-01T00:00:00Z"}`)

	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_abandon_checkpoint"
	request.Params.Arguments = map[string]any{
		"filename": "checkpoint-mcp-abandon2.json",
		"reason":   "superseded",
	}

	result, err := s.handleAbandonCheckpoint(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "abandon must fail when operator is not supplied")
}

func TestHandleAbandonCheckpoint_MalformedNamesQuarantine(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	writeCheckpointFileMCP(t, ws.RootPath, "checkpoint-mcp-malformed.json", "not-json{")

	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_abandon_checkpoint"
	request.Params.Arguments = map[string]any{
		"filename": "checkpoint-mcp-malformed.json",
		"reason":   "x",
		"operator": "tester@example.com",
	}

	result, err := s.handleAbandonCheckpoint(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.IsError)
	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &resp))
	assert.Equal(t, "checkpoint_use_quarantine", resp["code"])
	assert.False(t, resp["retryable"].(bool))
}

func TestHandleQuarantineCheckpoint_HappyPath(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	writeCheckpointFileMCP(t, ws.RootPath, "checkpoint-mcp-quarantine.json", "not-json{")

	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_quarantine_checkpoint"
	request.Params.Arguments = map[string]any{
		"filename": "checkpoint-mcp-quarantine.json",
		"reason":   "corrupt",
		"operator": "tester@example.com",
	}

	result, err := s.handleQuarantineCheckpoint(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.IsError, "quarantine must succeed for a malformed target")
	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &resp))
	assert.Equal(t, "quarantined", resp["disposition"])

	destPath := filepath.Join(ws.RootPath, ".backlogit", "archive", "checkpoints", "checkpoint-mcp-quarantine.json")
	assert.FileExists(t, destPath)
}

func TestHandleQuarantineCheckpoint_ValidNamesAbandon(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	writeCheckpointFileMCP(t, ws.RootPath, "checkpoint-mcp-valid.json",
		`{"schema_version":1,"agent":"ship","session_id":"s1","phase":"build","status":"active","created_at":"2026-08-01T00:00:00Z","updated_at":"2026-08-01T00:00:00Z"}`)

	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_quarantine_checkpoint"
	request.Params.Arguments = map[string]any{
		"filename": "checkpoint-mcp-valid.json",
		"reason":   "x",
		"operator": "tester@example.com",
	}

	result, err := s.handleQuarantineCheckpoint(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, result.IsError)
	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	var resp map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &resp))
	assert.Equal(t, "checkpoint_use_abandon", resp["code"])
}

func TestHandleQuarantineCheckpoint_MissingOperator(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	writeCheckpointFileMCP(t, ws.RootPath, "checkpoint-mcp-quarantine2.json", "not-json{")

	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_quarantine_checkpoint"
	request.Params.Arguments = map[string]any{
		"filename": "checkpoint-mcp-quarantine2.json",
		"reason":   "corrupt",
	}

	result, err := s.handleQuarantineCheckpoint(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "quarantine must fail when operator is not supplied")
}

