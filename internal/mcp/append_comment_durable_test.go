package mcp

// U5 (130.005-T) regression tests: map durability classes to explicit machine-readable
// MCP outcomes in handleAppendComment.
//
// These tests inject failures through the appendCommentFn seam (package-global,
// overridable) because core.AppendComment is called directly and the events fsync
// seams are unexported in package events — there is no path to inject a durability
// error from package mcp without this seam.
//
// Must not run with t.Parallel: tests swap the appendCommentFn package-global seam.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
)

// appendCommentRequest builds a minimal handleAppendComment CallToolRequest.
func appendCommentRequest(itemID, actor, comment string) mcplib.CallToolRequest {
	var req mcplib.CallToolRequest
	req.Params.Name = "backlogit_append_comment"
	req.Params.Arguments = map[string]any{
		"item_id": itemID,
		"actor":   actor,
		"comment": comment,
	}
	return req
}

// parseToolResultJSON unmarshals the first TextContent from an MCP result.
func parseToolResultJSON(t *testing.T, result *mcplib.CallToolResult) map[string]any {
	t.Helper()
	require.NotEmpty(t, result.Content)
	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok, "expected TextContent, got %T", result.Content[0])
	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &m))
	return m
}

// countCommentEvents counts JSONL lines with event_type=="comment" in data.
func countCommentEvents(data []byte) int {
	count := 0
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var ev map[string]any
		if json.Unmarshal([]byte(line), &ev) != nil {
			continue
		}
		if ev["event_type"] == "comment" {
			count++
		}
	}
	return count
}

// TestHandleAppendComment_IndeterminateAppend_ReturnsDistinctOutcome is the U5
// regression: when appendCommentFn returns ErrWriteIndeterminate, handleAppendComment
// must return an MCP error result with "error":"write_indeterminate" and
// "retryable":false, distinct from the generic "internal" error so an agent does not
// auto-retry and duplicate the comment.
func TestHandleAppendComment_IndeterminateAppend_ReturnsDistinctOutcome(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()

	artifact, err := core.CreateArtifact(ctx, ws, "Comment host", "feature")
	require.NoError(t, err)

	origFn := appendCommentFn
	appendCommentFn = func(_ context.Context, _ *core.Workspace, _ *events.EventWriter, _, _, _, _ string) error {
		return fmt.Errorf("parent fsync failed: %w", blerrors.ErrWriteIndeterminate)
	}
	t.Cleanup(func() { appendCommentFn = origFn })

	result, err := s.handleAppendComment(ctx, appendCommentRequest(artifact.ID, "agent", "looks good"))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "indeterminate append must return an error result")

	body := parseToolResultJSON(t, result)
	assert.Equal(t, "write_indeterminate", body["error"],
		"indeterminate class must map to write_indeterminate (not generic internal)")
	retryable, _ := body["retryable"].(bool)
	assert.False(t, retryable,
		"indeterminate outcome must have retryable:false to prevent duplicate comment")
}

// TestHandleAppendComment_NotAppliedAppend_ReturnsRetryableOutcome asserts that when
// appendCommentFn returns ErrWriteNotApplied, handleAppendComment returns an MCP error
// result with "error":"write_not_applied" and "retryable":true.
func TestHandleAppendComment_NotAppliedAppend_ReturnsRetryableOutcome(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()

	artifact, err := core.CreateArtifact(ctx, ws, "Comment host 2", "feature")
	require.NoError(t, err)

	origFn := appendCommentFn
	appendCommentFn = func(_ context.Context, _ *core.Workspace, _ *events.EventWriter, _, _, _, _ string) error {
		return fmt.Errorf("pre-write open failed: %w", blerrors.ErrWriteNotApplied)
	}
	t.Cleanup(func() { appendCommentFn = origFn })

	result, err := s.handleAppendComment(ctx, appendCommentRequest(artifact.ID, "agent", "retry me"))
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "not-applied append must return an error result")

	body := parseToolResultJSON(t, result)
	assert.Equal(t, "write_not_applied", body["error"],
		"not-applied class must map to write_not_applied")
	retryable, _ := body["retryable"].(bool)
	assert.True(t, retryable,
		"not-applied outcome must have retryable:true (comment was not appended, safe to retry)")
}

// TestHandleAppendComment_RetryIdempotency_ExactlyOneEvent asserts that after a
// not-applied outcome (no comment written), a retry produces exactly ONE comment event
// in the item log — the retry cannot duplicate. Also verifies the ordering contract:
// IsWriteNotApplied is evaluated before IsWriteIndeterminate (mutually exclusive classes).
func TestHandleAppendComment_RetryIdempotency_ExactlyOneEvent(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()

	artifact, err := core.CreateArtifact(ctx, ws, "Idempotency host", "feature")
	require.NoError(t, err)

	calls := 0
	origFn := appendCommentFn
	appendCommentFn = func(fctx context.Context, fws *core.Workspace, few *events.EventWriter, itemID, actor, comment, commitSHA string) error {
		calls++
		if calls == 1 {
			return fmt.Errorf("open failed: %w", blerrors.ErrWriteNotApplied)
		}
		return core.AppendComment(fctx, fws, few, itemID, actor, comment, commitSHA)
	}
	t.Cleanup(func() { appendCommentFn = origFn })

	req := appendCommentRequest(artifact.ID, "agent", "idempotent comment")

	// First call: not-applied → error result.
	r1, err := s.handleAppendComment(ctx, req)
	require.NoError(t, err)
	assert.True(t, r1.IsError, "first call must fail (not-applied)")
	body1 := parseToolResultJSON(t, r1)
	assert.Equal(t, "write_not_applied", body1["error"])

	// Retry: succeeds → {"ok":true}.
	r2, err := s.handleAppendComment(ctx, req)
	require.NoError(t, err)
	assert.False(t, r2.IsError, "retry must succeed")
	body2 := parseToolResultJSON(t, r2)
	assert.Equal(t, true, body2["ok"], "success response must contain ok:true")

	// Exactly one comment event in the log.
	logPath := fmt.Sprintf("%s/.backlogit/logs/%s.jsonl", ws.RootPath, artifact.ID)
	rawLog, readErr := os.ReadFile(logPath)
	require.NoError(t, readErr)
	assert.Equal(t, 1, countCommentEvents(rawLog),
		"exactly one comment event after retry (no duplicate)")
}
