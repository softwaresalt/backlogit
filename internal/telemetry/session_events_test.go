package telemetry_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/telemetry"
)

const sampleSessionEvents = `{"event_type":"session.start","session_id":"sess-001","timestamp":"2026-04-09T00:00:00Z"}
{"event_type":"tool.execution_complete","toolCallId":"tc-001","model":"claude-sonnet-4","timestamp":"2026-04-09T00:01:00Z"}
{"event_type":"session.compaction_complete","preCompactionTokens":50000,"compactionTokensUsed":{"input":5000,"output":2000,"cachedInput":1000},"timestamp":"2026-04-09T00:10:00Z"}
{"event_type":"session.compaction_complete","preCompactionTokens":80000,"compactionTokensUsed":{"input":8000,"output":3000,"cachedInput":2000},"timestamp":"2026-04-09T00:20:00Z"}
`

func TestParseSessionEvents_ExtractsCompactions(t *testing.T) {
	events, err := telemetry.ParseSessionEvents(strings.NewReader(sampleSessionEvents))
	require.NoError(t, err)
	assert.Len(t, events, 2, "expected 2 compaction events")
	assert.Equal(t, 50000, events[0].PreCompactionTokens)
	assert.Equal(t, 5000, events[0].InputTokens)
	assert.Equal(t, 2000, events[0].OutputTokens)
	assert.Equal(t, 1000, events[0].CachedInputTokens)
	assert.Equal(t, 80000, events[1].PreCompactionTokens)
}

func TestParseSessionEvents_EmptyInput(t *testing.T) {
	events, err := telemetry.ParseSessionEvents(strings.NewReader(""))
	require.NoError(t, err)
	assert.Empty(t, events, "empty input should yield no compaction events")
}

func TestParseSessionEvents_NoCompactions(t *testing.T) {
	input := `{"event_type":"session.start","session_id":"sess-001","timestamp":"2026-04-09T00:00:00Z"}
{"event_type":"tool.execution_complete","toolCallId":"tc-001","timestamp":"2026-04-09T00:01:00Z"}
`
	events, err := telemetry.ParseSessionEvents(strings.NewReader(input))
	require.NoError(t, err)
	assert.Empty(t, events, "missing compaction events should return empty slice, not error")
}

func TestParseSessionEvents_MalformedLinesSkipped(t *testing.T) {
	input := `{"event_type":"session.compaction_complete","preCompactionTokens":10000,"compactionTokensUsed":{"input":1000,"output":500,"cachedInput":200},"timestamp":"2026-04-09T00:10:00Z"}
NOT JSON AT ALL
{"event_type":"session.compaction_complete","preCompactionTokens":20000,"compactionTokensUsed":{"input":2000,"output":1000,"cachedInput":400},"timestamp":"2026-04-09T00:20:00Z"}
`
	events, err := telemetry.ParseSessionEvents(strings.NewReader(input))
	require.NoError(t, err)
	assert.Len(t, events, 2, "malformed lines should be skipped, not cause failure")
}
