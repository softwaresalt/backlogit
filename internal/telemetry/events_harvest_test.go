package telemetry

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testSessionID = "test-session-abc"

// buildEventsJSONL constructs an events.jsonl stream from a slice of raw JSON lines.
func buildEventsJSONL(lines []string) string {
	return strings.Join(lines, "\n") + "\n"
}

func TestParseEventFacts_ToolCalls(t *testing.T) {
	raw := buildEventsJSONL([]string{
		`{"type":"session.start","timestamp":"2026-05-01T10:00:00Z","data":{"sessionId":"test-session-abc","startTime":"2026-05-01T10:00:00Z","context":{"branch":"main","repository":"owner/repo"}}}`,
		`{"type":"tool.execution_start","timestamp":"2026-05-01T10:00:01Z","data":{"toolCallId":"tc-001","toolName":"powershell","mcpServerName":"","mcpToolName":"","turnId":"turn-1"}}`,
		`{"type":"tool.execution_complete","timestamp":"2026-05-01T10:00:02Z","data":{"toolCallId":"tc-001","model":"claude-sonnet","turnId":"turn-1","success":true}}`,
		`{"type":"tool.execution_start","timestamp":"2026-05-01T10:00:03Z","data":{"toolCallId":"tc-002","toolName":"backlogit-backlogit_move_item","mcpServerName":"backlogit","mcpToolName":"backlogit_move_item","turnId":"turn-1"}}`,
		`{"type":"tool.execution_complete","timestamp":"2026-05-01T10:00:04Z","data":{"toolCallId":"tc-002","model":"claude-sonnet","turnId":"turn-1","success":false}}`,
	})

	facts, sessionFact, err := ParseEventFacts(strings.NewReader(raw), testSessionID)
	require.NoError(t, err)
	require.Len(t, facts, 2)
	assert.Nil(t, sessionFact, "no shutdown event — session fact should be nil")

	// First fact: built-in powershell tool
	tc1 := facts[0]
	assert.Equal(t, testSessionID, tc1.SessionID)
	assert.Equal(t, "powershell", tc1.ToolName)
	assert.Equal(t, "", tc1.ServerName)
	assert.True(t, tc1.IsBuiltin)
	assert.Equal(t, "claude-sonnet", tc1.Model)
	assert.True(t, tc1.Success)
	assert.InDelta(t, 1000, tc1.DurationMs, 50, "duration should be ~1000ms")
	assert.Equal(t, "main", tc1.Branch)
	assert.Equal(t, "owner/repo", tc1.Repository)

	// Second fact: MCP tool — tool name should be the short mcpToolName
	tc2 := facts[1]
	assert.Equal(t, "backlogit_move_item", tc2.ToolName, "MCP tool should use short mcpToolName")
	assert.Equal(t, "backlogit", tc2.ServerName)
	assert.False(t, tc2.IsBuiltin)
	assert.False(t, tc2.Success)
}

func TestParseEventFacts_SessionFact(t *testing.T) {
	raw := buildEventsJSONL([]string{
		`{"type":"session.start","timestamp":"2026-05-01T10:00:00Z","data":{"sessionId":"test-session-abc","startTime":"2026-05-01T10:00:00Z","context":{"branch":"feat/x","repository":"owner/repo"}}}`,
		`{"type":"tool.execution_start","timestamp":"2026-05-01T10:00:01Z","data":{"toolCallId":"tc-001","toolName":"view","mcpServerName":"","mcpToolName":"","turnId":"t1"}}`,
		`{"type":"tool.execution_complete","timestamp":"2026-05-01T10:00:02Z","data":{"toolCallId":"tc-001","model":"claude-haiku","turnId":"t1","success":true}}`,
		`{"type":"session.shutdown","timestamp":"2026-05-01T10:05:00Z","data":{"totalApiDurationMs":300000,"totalPremiumRequests":2,"currentTokens":12000,"systemTokens":5000,"conversationTokens":4000,"toolDefinitionsTokens":3000,"modelMetrics":{"claude-haiku":{"requests":{"count":10,"cost":1},"usage":{"inputTokens":50000,"outputTokens":10000,"cacheReadTokens":20000,"cacheWriteTokens":5000,"reasoningTokens":0}}}}}`,
	})

	facts, sessionFact, err := ParseEventFacts(strings.NewReader(raw), testSessionID)
	require.NoError(t, err)
	require.Len(t, facts, 1)
	require.NotNil(t, sessionFact)

	assert.Equal(t, testSessionID, sessionFact.SessionID)
	assert.Equal(t, "feat/x", sessionFact.Branch)
	assert.Equal(t, "owner/repo", sessionFact.Repository)
	assert.Equal(t, 12000, sessionFact.CurrentTokens)
	assert.Equal(t, 5000, sessionFact.SystemTokens)
	assert.Equal(t, 4000, sessionFact.ConversationTokens)
	assert.Equal(t, 3000, sessionFact.ToolDefinitionsTokens)
	assert.Equal(t, int64(300000), sessionFact.TotalApiDurationMs)
	assert.Equal(t, 1, sessionFact.ToolCallCount, "tool call count should reflect matched pairs")

	mm, ok := sessionFact.ModelMetrics["claude-haiku"]
	require.True(t, ok)
	assert.Equal(t, 10, mm.RequestCount)
	assert.Equal(t, 50000, mm.InputTokens)
	assert.Equal(t, 10000, mm.OutputTokens)
	assert.Equal(t, 20000, mm.CacheReadTokens)
}

func TestParseEventFacts_UnmatchedStart(t *testing.T) {
	// A start without a complete should not produce a fact (active/in-flight call).
	raw := buildEventsJSONL([]string{
		`{"type":"session.start","timestamp":"2026-05-01T10:00:00Z","data":{"sessionId":"test-session-abc","startTime":"2026-05-01T10:00:00Z","context":{"branch":"main","repository":"owner/repo"}}}`,
		`{"type":"tool.execution_start","timestamp":"2026-05-01T10:00:01Z","data":{"toolCallId":"tc-orphan","toolName":"grep","mcpServerName":"","mcpToolName":"","turnId":"t1"}}`,
	})

	facts, _, err := ParseEventFacts(strings.NewReader(raw), testSessionID)
	require.NoError(t, err)
	assert.Empty(t, facts, "unmatched start should not produce a fact")
}

func TestParseEventFacts_MalformedLines(t *testing.T) {
	// Malformed lines should be skipped without error.
	raw := buildEventsJSONL([]string{
		`{"type":"session.start","timestamp":"2026-05-01T10:00:00Z","data":{"sessionId":"test-session-abc","startTime":"2026-05-01T10:00:00Z","context":{"branch":"main","repository":"owner/repo"}}}`,
		`NOT VALID JSON`,
		`{"type":"tool.execution_start","timestamp":"2026-05-01T10:00:01Z","data":{"toolCallId":"tc-001","toolName":"view","mcpServerName":"","mcpToolName":"","turnId":"t1"}}`,
		`{"type":"tool.execution_complete","timestamp":"2026-05-01T10:00:02Z","data":{"toolCallId":"tc-001","model":"claude-sonnet","turnId":"t1","success":true}}`,
	})

	facts, _, err := ParseEventFacts(strings.NewReader(raw), testSessionID)
	require.NoError(t, err)
	require.Len(t, facts, 1)
	assert.Equal(t, "view", facts[0].ToolName)
}

func TestHarvestEventsFacts_IncrementalSkip(t *testing.T) {
	// Sessions already in ProcessedEventSessions should be skipped.
	dir := t.TempDir()
	copilotDir := filepath.Join(dir, ".copilot")
	sessionDir := filepath.Join(copilotDir, "session-state", "session-complete")
	require.NoError(t, os.MkdirAll(sessionDir, 0o755))

	eventsContent := buildEventsJSONL([]string{
		`{"type":"session.start","timestamp":"2026-05-01T10:00:00Z","data":{"sessionId":"session-complete","startTime":"2026-05-01T10:00:00Z","context":{"branch":"main","repository":"owner/repo"}}}`,
		`{"type":"tool.execution_start","timestamp":"2026-05-01T10:00:01Z","data":{"toolCallId":"tc-001","toolName":"view","mcpServerName":"","mcpToolName":"","turnId":"t1"}}`,
		`{"type":"tool.execution_complete","timestamp":"2026-05-01T10:00:02Z","data":{"toolCallId":"tc-001","model":"claude-sonnet","turnId":"t1","success":true}}`,
		`{"type":"session.shutdown","timestamp":"2026-05-01T10:05:00Z","data":{"totalApiDurationMs":0,"totalPremiumRequests":0,"currentTokens":0,"systemTokens":1000,"conversationTokens":500,"toolDefinitionsTokens":200,"modelMetrics":{}}}`,
	})
	require.NoError(t, os.WriteFile(filepath.Join(sessionDir, "events.jsonl"), []byte(eventsContent), 0o644))

	cp := &HarvestCheckpoint{
		FileOffsets:            make(map[string]int64),
		ProcessedEventSessions: map[string]bool{"session-complete": true},
	}

	tcAdded, sfAdded, err := HarvestEventsFacts(copilotDir, dir, cp, time.Now(), false)
	require.NoError(t, err)
	assert.Equal(t, 0, tcAdded, "already-processed session should produce 0 tool call facts")
	assert.Equal(t, 0, sfAdded, "already-processed session should produce 0 session facts")
}

func TestHarvestEventsFacts_NewSession(t *testing.T) {
	dir := t.TempDir()
	copilotDir := filepath.Join(dir, ".copilot")
	sessionDir := filepath.Join(copilotDir, "session-state", "session-new")
	require.NoError(t, os.MkdirAll(sessionDir, 0o755))

	eventsContent := buildEventsJSONL([]string{
		`{"type":"session.start","timestamp":"2026-05-01T10:00:00Z","data":{"sessionId":"session-new","startTime":"2026-05-01T10:00:00Z","context":{"branch":"main","repository":"owner/repo"}}}`,
		`{"type":"tool.execution_start","timestamp":"2026-05-01T10:00:01Z","data":{"toolCallId":"tc-001","toolName":"grep","mcpServerName":"","mcpToolName":"","turnId":"t1"}}`,
		`{"type":"tool.execution_complete","timestamp":"2026-05-01T10:00:02Z","data":{"toolCallId":"tc-001","model":"claude-sonnet","turnId":"t1","success":true}}`,
		`{"type":"tool.execution_start","timestamp":"2026-05-01T10:00:03Z","data":{"toolCallId":"tc-002","toolName":"backlogit-backlogit_get_item","mcpServerName":"backlogit","mcpToolName":"backlogit_get_item","turnId":"t1"}}`,
		`{"type":"tool.execution_complete","timestamp":"2026-05-01T10:00:04Z","data":{"toolCallId":"tc-002","model":"claude-sonnet","turnId":"t1","success":true}}`,
		`{"type":"session.shutdown","timestamp":"2026-05-01T10:05:00Z","data":{"totalApiDurationMs":60000,"totalPremiumRequests":0,"currentTokens":2000,"systemTokens":1000,"conversationTokens":500,"toolDefinitionsTokens":200,"modelMetrics":{"claude-sonnet":{"requests":{"count":2,"cost":0},"usage":{"inputTokens":5000,"outputTokens":1000,"cacheReadTokens":3000,"cacheWriteTokens":500,"reasoningTokens":0}}}}}`,
	})
	require.NoError(t, os.WriteFile(filepath.Join(sessionDir, "events.jsonl"), []byte(eventsContent), 0o644))

	cp := &HarvestCheckpoint{
		FileOffsets:            make(map[string]int64),
		ProcessedEventSessions: make(map[string]bool),
	}

	tcAdded, sfAdded, err := HarvestEventsFacts(copilotDir, dir, cp, time.Now(), false)
	require.NoError(t, err)
	assert.Equal(t, 2, tcAdded)
	assert.Equal(t, 1, sfAdded)
	assert.True(t, cp.ProcessedEventSessions["session-new"], "completed session should be marked processed")

	// Verify fact files exist and have content
	telDir := filepath.Join(dir, ".backlogit", "telemetry")
	tcFacts, err := readToolCallFacts(filepath.Join(telDir, "tool-calls.jsonl"))
	require.NoError(t, err)
	require.Len(t, tcFacts, 2)
	// MCP tool should use short name
	var mcpFact *ToolCallFact
	for i := range tcFacts {
		if tcFacts[i].ServerName == "backlogit" {
			mcpFact = &tcFacts[i]
		}
	}
	require.NotNil(t, mcpFact)
	assert.Equal(t, "backlogit_get_item", mcpFact.ToolName)

	sfFacts, err := readSessionFacts(filepath.Join(telDir, "session-facts.jsonl"))
	require.NoError(t, err)
	require.Len(t, sfFacts, 1)
	assert.Equal(t, "session-new", sfFacts[0].SessionID)
	assert.Equal(t, 2, sfFacts[0].ToolCallCount)
}

func TestHarvestEventsFacts_ForceClears(t *testing.T) {
	dir := t.TempDir()
	copilotDir := filepath.Join(dir, ".copilot")
	sessionDir := filepath.Join(copilotDir, "session-state", "session-force")
	require.NoError(t, os.MkdirAll(sessionDir, 0o755))

	eventsContent := buildEventsJSONL([]string{
		`{"type":"session.start","timestamp":"2026-05-01T10:00:00Z","data":{"sessionId":"session-force","startTime":"2026-05-01T10:00:00Z","context":{"branch":"main","repository":"owner/repo"}}}`,
		`{"type":"tool.execution_start","timestamp":"2026-05-01T10:00:01Z","data":{"toolCallId":"tc-001","toolName":"view","mcpServerName":"","mcpToolName":"","turnId":"t1"}}`,
		`{"type":"tool.execution_complete","timestamp":"2026-05-01T10:00:02Z","data":{"toolCallId":"tc-001","model":"claude-sonnet","turnId":"t1","success":true}}`,
		`{"type":"session.shutdown","timestamp":"2026-05-01T10:05:00Z","data":{"totalApiDurationMs":0,"totalPremiumRequests":0,"currentTokens":0,"systemTokens":100,"conversationTokens":50,"toolDefinitionsTokens":20,"modelMetrics":{}}}`,
	})
	require.NoError(t, os.WriteFile(filepath.Join(sessionDir, "events.jsonl"), []byte(eventsContent), 0o644))

	cp := &HarvestCheckpoint{
		FileOffsets:            make(map[string]int64),
		ProcessedEventSessions: map[string]bool{"session-force": true},
	}

	// Force should clear the skip list and re-process
	tcAdded, sfAdded, err := HarvestEventsFacts(copilotDir, dir, cp, time.Now(), true)
	require.NoError(t, err)
	assert.Equal(t, 1, tcAdded, "force should re-process previously-completed session")
	assert.Equal(t, 1, sfAdded)
}
