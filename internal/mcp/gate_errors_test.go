package mcp

import (
	"encoding/json"
	"errors"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// resultBody decodes an MCP tool result's text content into a generic map.
func resultBody(t *testing.T, result *mcplib.CallToolResult) map[string]any {
	t.Helper()
	require.NotEmpty(t, result.Content, "result content must not be empty")
	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok, "expected TextContent, got %T", result.Content[0])
	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &body))
	return body
}

// TestGateErrorResult_BlockedFamily verifies each block-family outcome maps to a
// distinct error_type and carries the requested status + redirect metadata.
func TestGateErrorResult_BlockedFamily(t *testing.T) {
	cases := []struct {
		outcome      string
		newStatus    string
		stateChanged bool
		wantErrType  string
		wantNextMenu bool
	}{
		{"blocked", "active", false, "gate_blocked", true},
		{"requeued", "queued", true, "gate_requeued", false},
		{"escalated", "blocked", true, "gate_escalated", false},
	}
	for _, tc := range cases {
		be := &corerrors.GateBlockedError{
			ItemID:       "001.001-T",
			OldStatus:    "active",
			NewStatus:    tc.newStatus,
			Outcome:      tc.outcome,
			StateChanged: tc.stateChanged,
			ReportJSON:   []byte(`{"passed":false}`),
			Repeated:     &corerrors.GateRepeatedFailure{Count: 3, Threshold: 3, Reached: true, Action: "block"},
		}
		result, handled := gateErrorResult(be, "done")
		require.True(t, handled, "outcome %q must be handled", tc.outcome)
		require.True(t, result.IsError, "outcome %q must be an error result", tc.outcome)
		body := resultBody(t, result)
		require.Equal(t, tc.wantErrType, body["error"], "outcome %q error_type", tc.outcome)
		require.Equal(t, "done", body["requested_status"])
		require.Equal(t, tc.newStatus, body["new_status"])
		require.Equal(t, tc.stateChanged, body["state_changed"])
		require.Equal(t, false, body["retryable"])
		require.NotNil(t, body["repeated_failure"])
		require.NotNil(t, body["gate_report"], "raw gate report must be preserved for machine callers")
		_, hasMenu := body["allowed_next_actions"]
		require.Equal(t, tc.wantNextMenu, hasMenu, "outcome %q next-action menu presence", tc.outcome)
	}
}

// TestGateErrorResult_ClassFamily verifies the non-block classes map to distinct
// error_types with correct retryable flags and retry_after_ms passthrough.
func TestGateErrorResult_ClassFamily(t *testing.T) {
	cases := []struct {
		class         string
		retryAfterMs  int
		wantErrType   string
		wantRetryable bool
		wantRetryKey  bool
	}{
		{"setup", 0, "gate_setup", false, false},
		{"config", 0, "gate_config", false, false},
		{"timeout", 1500, "gate_timeout", true, true},
		{"in_progress", 2000, "gate_in_progress", true, true},
	}
	for _, tc := range cases {
		ge := &corerrors.GateError{Class: tc.class, ItemID: "001.001-T", Message: "gate " + tc.class, RetryAfterMs: tc.retryAfterMs}
		result, handled := gateErrorResult(ge, "done")
		require.True(t, handled, "class %q must be handled", tc.class)
		require.True(t, result.IsError)
		body := resultBody(t, result)
		require.Equal(t, tc.wantErrType, body["error"], "class %q error_type", tc.class)
		require.Equal(t, tc.wantRetryable, body["retryable"], "class %q retryable", tc.class)
		_, hasRetryAfter := body["retry_after_ms"]
		require.Equal(t, tc.wantRetryKey, hasRetryAfter, "class %q retry_after_ms presence", tc.class)
	}
}

// TestGateErrorResult_NonGate verifies non-gate errors are not handled so the
// caller falls back to domainError.
func TestGateErrorResult_NonGate(t *testing.T) {
	_, handled := gateErrorResult(errors.New("boom"), "done")
	require.False(t, handled)
	_, handled = gateErrorResult(corerrors.ErrValidation, "done")
	require.False(t, handled)
}

// TestGatePassResult_EnvelopesArtifact verifies a passing gated completion nests
// a gate envelope alongside the artifact fields.
func TestGatePassResult_EnvelopesArtifact(t *testing.T) {
	artifact := map[string]any{"id": "001.001-T", "status": "done", "title": "x"}
	oc := &core.GateOutcome{
		ItemID:         "001.001-T",
		OldStatus:      "active",
		NewStatus:      "done",
		Outcome:        "passed",
		StateChanged:   true,
		HeadSHA:        "abc123",
		GateReportHash: "sha256:deadbeef",
	}
	result, err := gatePassResult(artifact, oc)
	require.NoError(t, err)
	require.False(t, result.IsError)
	body := resultBody(t, result)
	require.Equal(t, "001.001-T", body["id"], "artifact fields must be preserved at top level")
	gate, ok := body["gate"].(map[string]any)
	require.True(t, ok, "gate envelope must be present")
	require.Equal(t, "passed", gate["outcome"])
	require.Equal(t, "abc123", gate["head_sha"])
	require.Equal(t, "sha256:deadbeef", gate["gate_report_hash"])
}
