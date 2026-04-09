package telemetry_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/telemetry"
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
