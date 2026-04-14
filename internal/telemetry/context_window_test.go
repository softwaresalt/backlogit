package telemetry_test

// Harness for 031.003-T: Context Window Consumption Tracking.
//
// All tests will FAIL until context_window.go is fully implemented.

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/telemetry"
)

// ---- ContextLimitForModel ---------------------------------------------------

func TestContextLimitForModel_KnownModel_ReturnsCorrectLimit(t *testing.T) {
	tests := []struct {
		model string
		want  int
	}{
		{"claude-sonnet-4", 200000},
		{"claude-sonnet-4.5", 200000},
		{"gpt-4.1", 1000000},
		{"gpt-4.1-mini", 500000},
		{"o4-mini", 200000},
	}
	for _, tt := range tests {
		t.Run(tt.model, func(t *testing.T) {
			got := telemetry.ContextLimitForModel(tt.model)
			assert.Equal(t, tt.want, got, "incorrect context limit for model %q", tt.model)
		})
	}
}

func TestContextLimitForModel_UnknownModel_ReturnsDefault(t *testing.T) {
	got := telemetry.ContextLimitForModel("unknown-model-xyz")
	assert.Equal(t, telemetry.DefaultContextLimit, got,
		"unknown model must return DefaultContextLimit")
}

// ---- ComputeContextMetrics --------------------------------------------------

func TestComputeContextMetrics_EmptyModelCalls_ReturnsNil(t *testing.T) {
	// No model calls → nil metrics, not division by zero.
	result := telemetry.ComputeContextMetrics(nil, nil)
	assert.Nil(t, result, "no model calls must produce nil ContextWindowMetrics")
}

func TestComputeContextMetrics_SingleKnownModel_CorrectUtilization(t *testing.T) {
	calls := []telemetry.ModelCall{
		{
			Model:        "claude-sonnet-4",
			PromptTokens: 100000,
			TotalTokens:  120000,
		},
	}
	metrics := telemetry.ComputeContextMetrics(calls, nil)
	require.NotNil(t, metrics)

	assert.Equal(t, 200000, metrics.MaxContextTokens)
	assert.Equal(t, 100000, metrics.PeakPromptTokens)
	assert.InDelta(t, 0.5, metrics.PeakUtilization, 0.001,
		"PeakUtilization = 100000 / 200000 = 0.5")
	assert.Equal(t, 100000, metrics.RemainingCapacity,
		"RemainingCapacity = 200000 - 100000")
	assert.InDelta(t, 120000.0, metrics.DepletionRate, 0.001,
		"DepletionRate = 120000 total_tokens / 1 model_call")
}

func TestComputeContextMetrics_MultipleModelCalls_TracksPeakAcrossCalls(t *testing.T) {
	calls := []telemetry.ModelCall{
		{Model: "claude-sonnet-4", PromptTokens: 50000, TotalTokens: 60000},
		{Model: "claude-sonnet-4", PromptTokens: 150000, TotalTokens: 180000}, // peak
		{Model: "claude-sonnet-4", PromptTokens: 80000, TotalTokens: 95000},
	}
	metrics := telemetry.ComputeContextMetrics(calls, nil)
	require.NotNil(t, metrics)

	assert.Equal(t, 150000, metrics.PeakPromptTokens)
	assert.InDelta(t, 0.75, metrics.PeakUtilization, 0.001,
		"PeakUtilization = 150000 / 200000 = 0.75")
	totalTokens := 60000 + 180000 + 95000
	expectedDepletionRate := float64(totalTokens) / 3.0
	assert.InDelta(t, expectedDepletionRate, metrics.DepletionRate, 0.001)
}

func TestComputeContextMetrics_UnknownModel_UsesDefaultLimit(t *testing.T) {
	calls := []telemetry.ModelCall{
		{Model: "future-model-xyz", PromptTokens: 50000, TotalTokens: 60000},
	}
	metrics := telemetry.ComputeContextMetrics(calls, nil)
	require.NotNil(t, metrics)

	assert.Equal(t, telemetry.DefaultContextLimit, metrics.MaxContextTokens,
		"unknown model must use DefaultContextLimit for calculations")
}

func TestComputeContextMetrics_CompactionEventsAreCounted(t *testing.T) {
	calls := []telemetry.ModelCall{
		{Model: "claude-sonnet-4", PromptTokens: 10000, TotalTokens: 12000},
	}
	compactions := []telemetry.CompactionEvent{
		{Timestamp: "2026-04-09T01:00:00Z", PreCompactionTokens: 180000},
		{Timestamp: "2026-04-09T02:00:00Z", PreCompactionTokens: 190000},
	}
	metrics := telemetry.ComputeContextMetrics(calls, compactions)
	require.NotNil(t, metrics)

	assert.Equal(t, 2, metrics.CompactionCount)
}

// ---- SessionSummary has ContextWindow field ---------------------------------

func TestSessionSummary_HasContextWindowField(t *testing.T) {
	// Compile-time assertion: SessionSummary must have a ContextWindow field of
	// type *ContextWindowMetrics.
	var s telemetry.SessionSummary
	s.ContextWindow = &telemetry.ContextWindowMetrics{
		PeakUtilization: 0.5,
	}
	assert.NotNil(t, s.ContextWindow)
}

// ---- End-to-end: harvested record includes context window fields -----------

func TestHarvestTelemetry_ProducesContextWindowMetrics(t *testing.T) {
	// After a full harvest with a known model, telemetry-sessions.jsonl must
	// contain context window fields for sessions with model calls.
	workspacePath, copilotPath := setupTelemetryHarvestWorkspace(t)
	writeSampleProcessLog(t, filepath.Join(copilotPath, "logs"))

	sqliteDB, err := db.Open(filepath.Join(workspacePath, ".backlogit", "index.db"))
	require.NoError(t, err)
	defer sqliteDB.Close()

	_, err = telemetry.HarvestTelemetry(
		context.Background(), workspacePath, copilotPath, sqliteDB,
		telemetry.HarvestOptions{},
	)
	require.NoError(t, err)

	// telemetry_sessions table must have non-null peak_utilization for
	// sessions that used claude-sonnet-4 (a known model with a defined limit).
	row := sqliteDB.QueryRow(
		`SELECT peak_utilization FROM telemetry_sessions WHERE session_id = 'sess-h1'`,
	)
	var peakUtil *float64
	require.NoError(t, row.Scan(&peakUtil))
	require.NotNil(t, peakUtil,
		"telemetry_sessions.peak_utilization must be non-null for sessions with known models")
	assert.Greater(t, *peakUtil, 0.0)
}
