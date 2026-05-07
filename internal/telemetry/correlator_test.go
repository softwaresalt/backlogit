package telemetry_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/telemetry"
)

func TestCorrelate_GroupsEventsBySession(t *testing.T) {
	events := []telemetry.TelemetryEvent{
		{Kind: telemetry.EventKindModelCall, ModelCall: &telemetry.ModelCall{
			SessionID: "sess-001", RequestID: "req-001", Model: "claude-sonnet-4",
			PromptTokens: 1000, CompletionTokens: 500, TotalTokens: 1500, CachedTokens: 200,
		}},
		{Kind: telemetry.EventKindToolCall, ToolCall: &telemetry.ToolCall{
			SessionID: "sess-001", ModelCallID: "req-001", ToolName: "backlogit_create_item",
		}},
		{Kind: telemetry.EventKindToolCall, ToolCall: &telemetry.ToolCall{
			SessionID: "sess-001", ModelCallID: "req-001", ToolName: "engram-query_memory",
		}},
		{Kind: telemetry.EventKindModelCall, ModelCall: &telemetry.ModelCall{
			SessionID: "sess-002", RequestID: "req-002", Model: "gpt-5.1",
			PromptTokens: 200, CompletionTokens: 80, TotalTokens: 280, CachedTokens: 0,
		}},
		{Kind: telemetry.EventKindToolCall, ToolCall: &telemetry.ToolCall{
			SessionID: "sess-002", ModelCallID: "req-002", ToolName: "view",
		}},
	}
	metas := map[string]telemetry.SessionMeta{
		"sess-001": {SessionID: "sess-001", Branch: "feat/test", Repository: "test/repo"},
		"sess-002": {SessionID: "sess-002", Branch: "main", Repository: "test/repo"},
	}

	summaries, err := telemetry.Correlate(context.Background(), events, metas, t.TempDir(), nil)
	require.NoError(t, err)
	require.Len(t, summaries, 2)

	byID := make(map[string]telemetry.SessionSummary)
	for _, s := range summaries {
		byID[s.SessionID] = s
	}

	s1 := byID["sess-001"]
	assert.Equal(t, 1500, s1.TotalTokens)
	assert.Equal(t, 1, s1.ModelCalls)
	assert.Equal(t, 2, s1.ToolCalls)
	// TokensByServer stores proportional token allocation (not a set).
	// 2 calls split evenly: backlogit=750, engram=750.
	assert.Equal(t, 750, s1.TokensByServer["backlogit"])
	assert.Equal(t, 750, s1.TokensByServer["engram"])

	s2 := byID["sess-002"]
	assert.Equal(t, 280, s2.TotalTokens)
	assert.Equal(t, 1, s2.ModelCalls)
	assert.Equal(t, 1, s2.ToolCalls)
	// Single tool call → copilot_builtin receives 100% of tokens.
	assert.Equal(t, 280, s2.TokensByServer["copilot_builtin"])
}

func TestCorrelate_NoTaskCompletions_NilTokensPerTask(t *testing.T) {
	events := []telemetry.TelemetryEvent{
		{Kind: telemetry.EventKindModelCall, ModelCall: &telemetry.ModelCall{
			SessionID: "sess-noop", RequestID: "req-x", Model: "claude-sonnet-4",
			TotalTokens: 100,
		}},
	}
	metas := map[string]telemetry.SessionMeta{
		"sess-noop": {SessionID: "sess-noop"},
	}

	summaries, err := telemetry.Correlate(context.Background(), events, metas, t.TempDir(), nil)
	require.NoError(t, err)
	require.Len(t, summaries, 1)
	assert.Nil(t, summaries[0].TokensPerTask, "sessions with no task completions should report nil tokens_per_task")
}
