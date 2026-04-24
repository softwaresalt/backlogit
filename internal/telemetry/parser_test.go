package telemetry_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/telemetry"
)

// Captured representative log samples for contract testing.
const sampleLog = `
2026-04-09T00:00:01.000Z [info] some unrelated debug line
2026-04-09T00:00:02.000Z [telemetry] {"event":"cli.model_call","session_id":"sess-001","request_id":"req-001","model":"claude-sonnet-4","prompt_tokens_count":1000,"completion_tokens_count":500,"total_tokens_count":1500,"cached_tokens_count":200,"duration_ms":1200}
2026-04-09T00:00:03.000Z [telemetry] {"event":"cli.tool_call","session_id":"sess-001","model_call_id":"req-001","tool_name":"backlogit_create_item","result_type":"text","duration_ms":45}
2026-04-09T00:00:04.000Z [telemetry] {"event":"cli.tool_call","session_id":"sess-001","model_call_id":"req-001","tool_name":"engram-query_memory","result_type":"text","duration_ms":30}
2026-04-09T00:00:05.000Z [telemetry] {"event":"cli.model_call","session_id":"sess-001","request_id":"req-002","model":"claude-sonnet-4","prompt_tokens_count":2000,"completion_tokens_count":800,"total_tokens_count":2800,"cached_tokens_count":500,"duration_ms":2100}
2026-04-09T00:00:06.000Z [telemetry] {"event":"cli.tool_call","session_id":"sess-001","model_call_id":"req-002","tool_name":"backlogit_get_item","result_type":"text","duration_ms":20}
2026-04-09T00:00:07.000Z [telemetry] {"event":"cli.model_call","session_id":"sess-002","request_id":"req-003","model":"gpt-5.1","prompt_tokens_count":500,"completion_tokens_count":100,"total_tokens_count":600,"cached_tokens_count":0,"duration_ms":400}
MALFORMED LINE {{{not json
2026-04-09T00:00:08.000Z [telemetry] {"event":"cli.tool_call","session_id":"sess-002","model_call_id":"req-003","tool_name":"view","result_type":"text","duration_ms":5}
2026-04-09T00:00:09.000Z [telemetry] {"event":"cli.tool_call","session_id":"sess-002","model_call_id":"req-003","tool_name":"github-create_pull_request","result_type":"text","duration_ms":800}
`

func TestCopilotCLIParser_ParseSampleLog(t *testing.T) {
	parser := telemetry.NewCopilotCLIParser()
	var events []telemetry.TelemetryEvent
	err := parser.Parse(strings.NewReader(sampleLog), func(e telemetry.TelemetryEvent) error {
		events = append(events, e)
		return nil
	})
	require.NoError(t, err)

	var modelCalls, toolCalls int
	for _, e := range events {
		switch e.Kind {
		case telemetry.EventKindModelCall:
			modelCalls++
		case telemetry.EventKindToolCall:
			toolCalls++
		}
	}

	assert.Equal(t, 3, modelCalls, "expected 3 model call events")
	assert.Equal(t, 5, toolCalls, "expected 5 tool call events")
}

func TestCopilotCLIParser_EmptyInput(t *testing.T) {
	parser := telemetry.NewCopilotCLIParser()
	var count int
	err := parser.Parse(strings.NewReader(""), func(_ telemetry.TelemetryEvent) error {
		count++
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 0, count, "empty input should emit no events")
}

func TestCopilotCLIParser_MalformedLinesSkipped(t *testing.T) {
	// A file with only malformed lines should not return an error;
	// it should produce zero events.
	input := "MALFORMED LINE\n{{{broken\nnot-json-at-all"
	parser := telemetry.NewCopilotCLIParser()
	var count int
	err := parser.Parse(strings.NewReader(input), func(_ telemetry.TelemetryEvent) error {
		count++
		return nil
	})
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestCopilotCLIParser_ModelCallFields(t *testing.T) {
	input := `2026-04-09T00:00:01.000Z [telemetry] {"event":"cli.model_call","session_id":"s1","request_id":"r1","model":"claude-sonnet-4","prompt_tokens_count":100,"completion_tokens_count":50,"total_tokens_count":150,"cached_tokens_count":10,"duration_ms":300}`
	parser := telemetry.NewCopilotCLIParser()
	var events []telemetry.TelemetryEvent
	require.NoError(t, parser.Parse(strings.NewReader(input), func(e telemetry.TelemetryEvent) error {
		events = append(events, e)
		return nil
	}))
	require.Len(t, events, 1)
	require.Equal(t, telemetry.EventKindModelCall, events[0].Kind)
	require.NotNil(t, events[0].ModelCall)
	mc := events[0].ModelCall
	assert.Equal(t, "s1", mc.SessionID)
	assert.Equal(t, "r1", mc.RequestID)
	assert.Equal(t, "claude-sonnet-4", mc.Model)
	assert.Equal(t, 100, mc.PromptTokens)
	assert.Equal(t, 50, mc.CompletionTokens)
	assert.Equal(t, 150, mc.TotalTokens)
	assert.Equal(t, 10, mc.CachedTokens)
	assert.Equal(t, 300, mc.DurationMs)
}

func TestCopilotCLIParser_ToolCallFields(t *testing.T) {
	input := `2026-04-09T00:00:01.000Z [telemetry] {"event":"cli.tool_call","session_id":"s1","model_call_id":"r1","tool_name":"backlogit_get_item","result_type":"text","duration_ms":42}`
	parser := telemetry.NewCopilotCLIParser()
	var events []telemetry.TelemetryEvent
	require.NoError(t, parser.Parse(strings.NewReader(input), func(e telemetry.TelemetryEvent) error {
		events = append(events, e)
		return nil
	}))
	require.Len(t, events, 1)
	require.Equal(t, telemetry.EventKindToolCall, events[0].Kind)
	require.NotNil(t, events[0].ToolCall)
	tc := events[0].ToolCall
	assert.Equal(t, "s1", tc.SessionID)
	assert.Equal(t, "r1", tc.ModelCallID)
	assert.Equal(t, "backlogit_get_item", tc.ToolName)
	assert.Equal(t, "text", tc.ResultType)
	assert.Equal(t, 42, tc.DurationMs)
}

// New-format test fixtures: multi-line [Telemetry] format.
const sampleNewFormatLog = `2026-04-23T10:00:00.000Z [INFO] some unrelated debug line
2026-04-23T10:00:01.000Z [INFO] [Telemetry] cli.model_call:
{
  "api_id": "api-001",
  "model": "claude-opus-4.6",
  "prompt_tokens_count": 52514,
  "completion_tokens_count": 1200,
  "total_tokens_count": 53714,
  "cached_tokens_count": 40000,
  "duration_ms": 15000,
  "request_id": "req-legacy-001",
  "session_id": "sess-new-001",
  "features": {
    "SOME_FLAG": "true"
  }
}
2026-04-23T10:00:02.000Z [INFO] [Telemetry] cli.tool_call:
{
  "tool_name": "backlogit_create_item",
  "tool_call_id": "call_abc123",
  "result_type": "SUCCESS",
  "duration_ms": 85,
  "model_call_id": "api-001",
  "session_id": "sess-new-001",
  "features": {
    "SOME_FLAG": "true"
  }
}
2026-04-23T10:00:03.000Z [INFO] [Telemetry] cli.telemetry:
{
  "kind": "should_be_skipped",
  "properties": {}
}
2026-04-23T10:00:04.000Z [INFO] [Telemetry] cli.restricted_telemetry:
{
  "kind": "also_skipped"
}
2026-04-23T10:00:05.000Z [INFO] [Telemetry] cli.model_call:
{
  "api_id": "api-002",
  "model": "gpt-5.4",
  "prompt_tokens_count": 1000,
  "completion_tokens_count": 200,
  "total_tokens_count": 1200,
  "cached_tokens_count": 0,
  "duration_ms": 3000,
  "request_id": "req-legacy-002",
  "session_id": "sess-new-002",
  "features": {}
}
`

func TestCopilotCLIParser_NewFormatModelCallAndToolCall(t *testing.T) {
	parser := telemetry.NewCopilotCLIParser()
	var events []telemetry.TelemetryEvent
	err := parser.Parse(strings.NewReader(sampleNewFormatLog), func(e telemetry.TelemetryEvent) error {
		events = append(events, e)
		return nil
	})
	require.NoError(t, err)

	var modelCalls, toolCalls int
	for _, e := range events {
		switch e.Kind {
		case telemetry.EventKindModelCall:
			modelCalls++
		case telemetry.EventKindToolCall:
			toolCalls++
		}
	}
	assert.Equal(t, 2, modelCalls, "expected 2 model call events (cli.telemetry/restricted skipped)")
	assert.Equal(t, 1, toolCalls, "expected 1 tool call event")
}

func TestCopilotCLIParser_NewFormatModelCallFields(t *testing.T) {
	parser := telemetry.NewCopilotCLIParser()
	var events []telemetry.TelemetryEvent
	require.NoError(t, parser.Parse(strings.NewReader(sampleNewFormatLog), func(e telemetry.TelemetryEvent) error {
		events = append(events, e)
		return nil
	}))

	// First event is the model call.
	require.NotEmpty(t, events)
	e := events[0]
	require.Equal(t, telemetry.EventKindModelCall, e.Kind)
	require.NotNil(t, e.ModelCall)
	mc := e.ModelCall
	assert.Equal(t, "sess-new-001", mc.SessionID)
	// api_id should be used as RequestID for tool call correlation.
	assert.Equal(t, "api-001", mc.RequestID)
	assert.Equal(t, "claude-opus-4.6", mc.Model)
	assert.Equal(t, 52514, mc.PromptTokens)
	assert.Equal(t, 1200, mc.CompletionTokens)
	assert.Equal(t, 53714, mc.TotalTokens)
	assert.Equal(t, 40000, mc.CachedTokens)
	assert.Equal(t, 15000, mc.DurationMs)
	// Timestamp should be extracted from the log-line prefix.
	assert.False(t, e.Timestamp.IsZero(), "timestamp should be parsed from log prefix")
}

func TestCopilotCLIParser_NewFormatToolCallCorrelation(t *testing.T) {
	parser := telemetry.NewCopilotCLIParser()
	var events []telemetry.TelemetryEvent
	require.NoError(t, parser.Parse(strings.NewReader(sampleNewFormatLog), func(e telemetry.TelemetryEvent) error {
		events = append(events, e)
		return nil
	}))

	// Find the tool call event.
	var tc *telemetry.ToolCall
	for _, e := range events {
		if e.Kind == telemetry.EventKindToolCall {
			tc = e.ToolCall
			break
		}
	}
	require.NotNil(t, tc)
	assert.Equal(t, "sess-new-001", tc.SessionID)
	assert.Equal(t, "api-001", tc.ModelCallID, "tool_call model_call_id should match model_call api_id")
	assert.Equal(t, "backlogit_create_item", tc.ToolName)
	assert.Equal(t, "SUCCESS", tc.ResultType)
	assert.Equal(t, 85, tc.DurationMs)
}

func TestCopilotCLIParser_MixedOldAndNewFormat(t *testing.T) {
	input := `2026-04-09T00:00:01.000Z [telemetry] {"event":"cli.model_call","session_id":"s-old","request_id":"r-old","model":"gpt-5.1","prompt_tokens_count":100,"completion_tokens_count":50,"total_tokens_count":150,"cached_tokens_count":0,"duration_ms":200}
2026-04-23T10:00:01.000Z [INFO] [Telemetry] cli.model_call:
{
  "api_id": "api-new",
  "model": "claude-opus-4.6",
  "prompt_tokens_count": 500,
  "completion_tokens_count": 100,
  "total_tokens_count": 600,
  "cached_tokens_count": 200,
  "duration_ms": 1000,
  "session_id": "s-new",
  "features": {}
}
2026-04-09T00:00:02.000Z [telemetry] {"event":"cli.tool_call","session_id":"s-old","model_call_id":"r-old","tool_name":"view","result_type":"text","duration_ms":5}
`
	parser := telemetry.NewCopilotCLIParser()
	var events []telemetry.TelemetryEvent
	require.NoError(t, parser.Parse(strings.NewReader(input), func(e telemetry.TelemetryEvent) error {
		events = append(events, e)
		return nil
	}))

	require.Len(t, events, 3, "should parse both old and new format events")

	// Old-format model call.
	assert.Equal(t, "s-old", events[0].ModelCall.SessionID)
	assert.Equal(t, "r-old", events[0].ModelCall.RequestID)

	// New-format model call.
	assert.Equal(t, "s-new", events[1].ModelCall.SessionID)
	assert.Equal(t, "api-new", events[1].ModelCall.RequestID)

	// Old-format tool call.
	assert.Equal(t, "s-old", events[2].ToolCall.SessionID)
}

func TestCopilotCLIParser_OldFormatBackwardCompat(t *testing.T) {
	// Verify that the original sample log (old format) still parses correctly.
	parser := telemetry.NewCopilotCLIParser()
	var events []telemetry.TelemetryEvent
	err := parser.Parse(strings.NewReader(sampleLog), func(e telemetry.TelemetryEvent) error {
		events = append(events, e)
		return nil
	})
	require.NoError(t, err)

	var modelCalls, toolCalls int
	for _, e := range events {
		switch e.Kind {
		case telemetry.EventKindModelCall:
			modelCalls++
		case telemetry.EventKindToolCall:
			toolCalls++
		}
	}
	assert.Equal(t, 3, modelCalls, "old format: expected 3 model call events")
	assert.Equal(t, 5, toolCalls, "old format: expected 5 tool call events")
}
