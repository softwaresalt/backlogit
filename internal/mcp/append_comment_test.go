package mcp

// U4 (079.004-T) behavior-preservation: handleAppendComment is refactored to
// delegate to the shared core.AppendComment path used by the CLI `comment add`
// command. These characterization tests pin the observable behavior — success
// shape `{"ok":true}` and the persisted per-item JSONL "comment" event — so the
// extraction cannot silently change what append_comment writes.

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
)

func readItemLogEvents(t *testing.T, root, itemID string) []map[string]any {
	t.Helper()
	path := filepath.Join(root, ".backlogit", "logs", itemID+".jsonl")
	data, err := os.ReadFile(path)
	require.NoError(t, err, "item log file must exist after append_comment")
	out := make([]map[string]any, 0)
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var ev map[string]any
		require.NoError(t, json.Unmarshal([]byte(line), &ev))
		out = append(out, ev)
	}
	return out
}

// handleAppendComment persists a "comment" event to the item's JSONL log and
// returns `{"ok":true}`. This is the shared contract both surfaces must honor.
func TestHandleAppendComment_PersistsCommentEvent(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()

	artifact, err := core.CreateArtifact(ctx, ws, "Comment host", "feature")
	require.NoError(t, err)

	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_append_comment"
	request.Params.Arguments = map[string]any{
		"item_id": artifact.ID,
		"actor":   "reviewer",
		"comment": "looks good",
	}

	result, err := s.handleAppendComment(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.IsError, "append_comment must succeed for a valid item")
	require.NotEmpty(t, result.Content)
	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok, "expected text content, got %T", result.Content[0])
	assert.JSONEq(t, `{"ok":true}`, tc.Text)

	logs := readItemLogEvents(t, ws.RootPath, artifact.ID)
	require.NotEmpty(t, logs)
	last := logs[len(logs)-1]
	assert.Equal(t, "comment", last["event_type"])
	assert.Equal(t, "reviewer", last["actor"])
	delta, ok := last["delta"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "looks good", delta["comment"])
}

// Missing item_id is rejected before any write (validation-failed), preserving
// the pre-extraction guard.
func TestHandleAppendComment_MissingItemID_ValidationError(t *testing.T) {
	s, _ := setupBugFixServer(t)
	ctx := context.Background()

	request := mcplib.CallToolRequest{}
	request.Params.Name = "backlogit_append_comment"
	request.Params.Arguments = map[string]any{
		"actor":   "reviewer",
		"comment": "orphaned",
	}

	result, err := s.handleAppendComment(ctx, request)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "missing item_id must return a validation error result")
}
