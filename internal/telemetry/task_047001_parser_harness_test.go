package telemetry_test

// Harness for 047.001-T: Fix bufio.Scanner token-too-long handling in telemetry parser.
//
// These tests FAIL against the current implementation because bufio.Scanner has a hard
// 1MB token limit. Feeding a line longer than 1MB causes scanner.Err() to return
// bufio.ErrTooLong, which aborts the entire parse.
//
// Once parser.go replaces bufio.NewScanner with bufio.NewReader + ReadString('\n'),
// all oversized-line tests should pass. Once ValidateSessionSummary is implemented
// in validate.go, the zero-token session tests should pass.
//
// Harness commands:
//
//	go test ./internal/telemetry/ -run TestParser_Oversized -v
//	go test ./internal/telemetry/ -run TestParser_ZeroToken -v

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/telemetry"
)

// buildOversizedNonTelemetryLine returns a single log line whose total length
// exceeds sizeBytes. The line contains no telemetry marker, so the parser should
// skip it entirely with no error.
func buildOversizedNonTelemetryLine(sizeBytes int) string {
	return "2026-04-25T00:00:01.000Z [info] " + strings.Repeat("x", sizeBytes) + "\n"
}

// TestParser_OversizedNonTelemetryLine_DoesNotError asserts that a single line
// exceeding 1MB that carries no telemetry marker is silently discarded without
// returning an error.
//
// CURRENTLY FAILS: bufio.Scanner returns bufio.ErrTooLong for lines > 1MB.
func TestParser_OversizedNonTelemetryLine_DoesNotError(t *testing.T) {
	input := buildOversizedNonTelemetryLine(2 * 1024 * 1024) // 2MB non-telemetry line
	parser := telemetry.NewCopilotCLIParser()
	var count int
	err := parser.Parse(strings.NewReader(input), func(_ telemetry.TelemetryEvent) error {
		count++
		return nil
	})
	require.NoError(t, err, "oversized non-telemetry line must not return an error")
	assert.Equal(t, 0, count, "oversized non-telemetry line must emit no events")
}

// TestParser_OversizedLine_SubsequentEventsPreserved asserts that valid telemetry
// events appearing after an oversized non-telemetry line are still parsed and emitted.
//
// CURRENTLY FAILS: bufio.Scanner aborts on ErrTooLong and never reads subsequent lines.
func TestParser_OversizedLine_SubsequentEventsPreserved(t *testing.T) {
	validBefore := "2026-04-25T00:00:00.000Z [telemetry] " +
		`{"event":"cli.model_call","session_id":"before","request_id":"r-before","model":"claude-sonnet-4",` +
		`"prompt_tokens_count":100,"completion_tokens_count":50,"total_tokens_count":150,` +
		`"cached_tokens_count":0,"duration_ms":200}` + "\n"
	oversized := buildOversizedNonTelemetryLine(2 * 1024 * 1024)
	validAfter := "2026-04-25T00:00:02.000Z [telemetry] " +
		`{"event":"cli.tool_call","session_id":"after","model_call_id":"r-after",` +
		`"tool_name":"view","result_type":"text","duration_ms":5}` + "\n"

	parser := telemetry.NewCopilotCLIParser()
	var events []telemetry.TelemetryEvent
	err := parser.Parse(strings.NewReader(validBefore+oversized+validAfter), func(e telemetry.TelemetryEvent) error {
		events = append(events, e)
		return nil
	})
	require.NoError(t, err, "parser must not error when an oversized line appears mid-stream")
	require.Len(t, events, 2, "valid events before and after the oversized line must both be emitted")
	assert.Equal(t, telemetry.EventKindModelCall, events[0].Kind)
	assert.Equal(t, telemetry.EventKindToolCall, events[1].Kind)
}

// TestParser_OversizedLineBeforeEvent_EventStillParsed asserts that a large
// non-telemetry line appearing before a valid model_call event does not prevent
// that event from being parsed correctly.
//
// CURRENTLY FAILS: bufio.Scanner aborts on ErrTooLong before reaching the valid event.
func TestParser_OversizedLineBeforeEvent_EventStillParsed(t *testing.T) {
	oversized := buildOversizedNonTelemetryLine(2 * 1024 * 1024)
	validEvent := "2026-04-25T00:00:01.000Z [telemetry] " +
		`{"event":"cli.model_call","session_id":"s-valid","request_id":"r-valid","model":"claude-sonnet-4",` +
		`"prompt_tokens_count":500,"completion_tokens_count":200,"total_tokens_count":700,` +
		`"cached_tokens_count":0,"duration_ms":1000}` + "\n"

	parser := telemetry.NewCopilotCLIParser()
	var events []telemetry.TelemetryEvent
	err := parser.Parse(strings.NewReader(oversized+validEvent), func(e telemetry.TelemetryEvent) error {
		events = append(events, e)
		return nil
	})
	require.NoError(t, err, "oversized prefix line must not abort the parse")
	require.Len(t, events, 1, "exactly one valid event must be emitted after the oversized line")
	require.NotNil(t, events[0].ModelCall)
	assert.Equal(t, 700, events[0].ModelCall.TotalTokens,
		"parsed model call must report the correct non-zero token count")
}

// TestParser_ZeroTokenSession_Rejected asserts that ValidateSessionSummary returns
// false for a session where TotalTokens==0 and ToolCalls>0. Such sessions are
// produced when oversized log entries are dropped, leaving tool calls without
// any model-call token attribution.
//
// CURRENTLY FAILS: ValidateSessionSummary panics "not implemented: session validation".
func TestParser_ZeroTokenSession_Rejected(t *testing.T) {
	s := telemetry.SessionSummary{
		SessionID:   "sess-zero-tokens",
		TotalTokens: 0,
		ToolCalls:   3,
		ModelCalls:  0,
	}
	assert.False(t, telemetry.ValidateSessionSummary(s),
		"session with TotalTokens==0 and ToolCalls>0 must be rejected as a partial session")
}

// TestParser_ZeroTokenZeroToolSession_Accepted asserts that a completely empty
// session (TotalTokens==0 and ToolCalls==0) passes validation. It represents a
// session with no recorded activity, which is valid (not a partial session).
//
// CURRENTLY FAILS: ValidateSessionSummary panics "not implemented: session validation".
func TestParser_ZeroTokenZeroToolSession_Accepted(t *testing.T) {
	s := telemetry.SessionSummary{
		SessionID:   "sess-empty",
		TotalTokens: 0,
		ToolCalls:   0,
		ModelCalls:  0,
	}
	assert.True(t, telemetry.ValidateSessionSummary(s),
		"empty session (TotalTokens==0 and ToolCalls==0) is not a partial session and must be accepted")
}

// TestParser_ValidSession_Accepted asserts that a normal session with tokens and
// tool calls passes validation.
//
// CURRENTLY FAILS: ValidateSessionSummary panics "not implemented: session validation".
func TestParser_ValidSession_Accepted(t *testing.T) {
	s := telemetry.SessionSummary{
		SessionID:   "sess-normal",
		TotalTokens: 1500,
		ToolCalls:   5,
		ModelCalls:  2,
	}
	assert.True(t, telemetry.ValidateSessionSummary(s),
		"normal session with tokens and tool calls must be accepted")
}
