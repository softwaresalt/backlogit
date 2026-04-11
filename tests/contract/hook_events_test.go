package contract_test

// 027.006-T: Poll and Ack MCP tools — contract tests.
// These tests verify tool registration, schema, and handler behavior.
// Handler tests will fail (panic) until production implementations are in place.

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

func setupHookMCPServer(t *testing.T) *mcpinternal.Server {
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

// parseToolResultJSON parses the JSON content from a CallToolResult.
func parseToolResultJSON(t *testing.T, result *mcplib.CallToolResult) map[string]any {
	t.Helper()
	require.NotEmpty(t, result.Content, "result must have content")
	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok, "expected TextContent in result")
	var data map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &data))
	return data
}


// --- Registration contract tests (must pass as part of harness) ---

func TestPollHookEvents_ToolExists(t *testing.T) {
	s := setupHookMCPServer(t)
	tools := s.ListTools()

	found := false
	for _, tool := range tools {
		if tool == "backlogit_poll_hook_events" {
			found = true
			break
		}
	}
	assert.True(t, found, "backlogit_poll_hook_events must be registered")
}

func TestAckHookEvents_ToolExists(t *testing.T) {
	s := setupHookMCPServer(t)
	tools := s.ListTools()

	found := false
	for _, tool := range tools {
		if tool == "backlogit_ack_hook_events" {
			found = true
			break
		}
	}
	assert.True(t, found, "backlogit_ack_hook_events must be registered")
}

// --- Handler contract tests (will fail until implementation is complete) ---

func TestPollHookEvents_Handler_ReturnsResult(t *testing.T) {
	s := setupHookMCPServer(t)

	result, err := callToolForTest(t, s, "backlogit_poll_hook_events", map[string]any{
		"consumer_id": "stage",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError, "poll must not return an error for empty queue")
}

func TestPollHookEvents_Handler_ResponseHasEventsField(t *testing.T) {
	s := setupHookMCPServer(t)

	result, err := callToolForTest(t, s, "backlogit_poll_hook_events", map[string]any{
		"consumer_id": "ship",
	})
	require.NoError(t, err)
	require.NotNil(t, result)

	data := parseToolResultJSON(t, result)
	_, hasEvents := data["events"]
	_, hasDerived := data["derived_signals"]
	assert.True(t, hasEvents, "response must contain 'events' field")
	assert.True(t, hasDerived, "response must contain 'derived_signals' field")
}

func TestPollHookEvents_Handler_MissingConsumerID_ReturnsError(t *testing.T) {
	s := setupHookMCPServer(t)

	result, err := callToolForTest(t, s, "backlogit_poll_hook_events", map[string]any{})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "missing consumer_id must return an error")
}

func TestAckHookEvents_Handler_AcksCheckpoint(t *testing.T) {
	s := setupHookMCPServer(t)

	result, err := callToolForTest(t, s, "backlogit_ack_hook_events", map[string]any{
		"consumer_id": "stage",
		"seq":         float64(1),
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError, "ack must succeed for valid parameters")
}

func TestAckHookEvents_Handler_MissingConsumerID_ReturnsError(t *testing.T) {
	s := setupHookMCPServer(t)

	result, err := callToolForTest(t, s, "backlogit_ack_hook_events", map[string]any{
		"seq": float64(1),
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "missing consumer_id must return an error")
}

func TestAckHookEvents_Handler_MissingSeq_ReturnsError(t *testing.T) {
	s := setupHookMCPServer(t)

	result, err := callToolForTest(t, s, "backlogit_ack_hook_events", map[string]any{
		"consumer_id": "stage",
	})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "missing seq must return an error")
}
