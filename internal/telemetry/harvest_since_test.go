package telemetry_test

// Harness for 031.002-T: Date-Filtered Harvest (--since flag).
//
// All tests will FAIL until the parser extracts timestamps and
// parseLogFiles applies the Since filter.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/db"
	"github.com/backlogit/backlogit/internal/telemetry"
)

// ---- Parser timestamp extraction --------------------------------------------

func TestCopilotCLIParser_ExtractsTimestampFromLogLine(t *testing.T) {
	// The parser must populate TelemetryEvent.Timestamp from the log-line
	// prefix (e.g. "2026-04-09T00:00:02.000Z").
	logLine := `2026-04-09T12:34:56.789Z [telemetry] {"event":"cli.model_call","session_id":"s1","request_id":"r1","model":"gpt-5.1","prompt_tokens_count":100,"completion_tokens_count":50,"total_tokens_count":150,"cached_tokens_count":0,"duration_ms":100}` + "\n"

	parser := telemetry.NewCopilotCLIParser()
	var events []telemetry.TelemetryEvent
	require.NoError(t, parser.Parse(strings.NewReader(logLine), func(e telemetry.TelemetryEvent) error {
		events = append(events, e)
		return nil
	}))

	require.Len(t, events, 1)
	expected := time.Date(2026, 4, 9, 12, 34, 56, 789000000, time.UTC)
	assert.Equal(t, expected, events[0].Timestamp,
		"parser must extract the log-line timestamp and populate TelemetryEvent.Timestamp")
}

func TestCopilotCLIParser_MalformedTimestampPrefix_AssignsZeroTime(t *testing.T) {
	// Lines without a parseable ISO 8601 prefix must yield zero-value timestamps.
	// Zero-value events are always included when --since is active (safe default).
	logLine := `no-timestamp [telemetry] {"event":"cli.model_call","session_id":"s2","request_id":"r2","model":"gpt-5.1","prompt_tokens_count":50,"completion_tokens_count":20,"total_tokens_count":70,"cached_tokens_count":0,"duration_ms":50}` + "\n"

	parser := telemetry.NewCopilotCLIParser()
	var events []telemetry.TelemetryEvent
	require.NoError(t, parser.Parse(strings.NewReader(logLine), func(e telemetry.TelemetryEvent) error {
		events = append(events, e)
		return nil
	}))

	require.Len(t, events, 1)
	assert.True(t, events[0].Timestamp.IsZero(),
		"malformed timestamp prefix must produce a zero-value Timestamp")
}

// ---- --since filter in HarvestTelemetry ------------------------------------

const sinceFilterLog = `2026-04-09T00:00:01.000Z [info] startup
2026-04-09T00:00:02.000Z [telemetry] {"event":"cli.model_call","session_id":"sess-old","request_id":"req-old","model":"gpt-5.1","prompt_tokens_count":100,"completion_tokens_count":50,"total_tokens_count":150,"cached_tokens_count":0,"duration_ms":100}
2026-04-11T00:00:02.000Z [telemetry] {"event":"cli.model_call","session_id":"sess-new","request_id":"req-new","model":"gpt-5.1","prompt_tokens_count":200,"completion_tokens_count":80,"total_tokens_count":280,"cached_tokens_count":0,"duration_ms":200}
`

func setupSinceWorkspace(t *testing.T) (workspacePath, copilotPath string) {
	t.Helper()
	workspacePath, copilotPath = setupTelemetryHarvestWorkspace(t)
	logsDir := filepath.Join(copilotPath, "logs")
	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "since-test.log"), []byte(sinceFilterLog), 0o644))
	return workspacePath, copilotPath
}

func TestHarvestTelemetry_Since_ExcludesOlderEvents(t *testing.T) {
	workspacePath, copilotPath := setupSinceWorkspace(t)

	sqliteDB, err := db.Open(filepath.Join(workspacePath, ".backlogit", "index.db"))
	require.NoError(t, err)
	defer sqliteDB.Close()

	// Only events on or after 2026-04-10 should be included.
	cutoff := time.Date(2026, 4, 10, 0, 0, 0, 0, time.UTC)
	result, err := telemetry.HarvestTelemetry(
		context.Background(), workspacePath, copilotPath, sqliteDB,
		telemetry.HarvestOptions{Since: &cutoff},
	)
	require.NoError(t, err)
	assert.Equal(t, 1, result.SessionsHarvested,
		"--since filter must exclude sessions whose events all predate the cutoff")
}

func TestHarvestTelemetry_Since_FullHarvestWhenAllEventsNewer(t *testing.T) {
	workspacePath, copilotPath := setupSinceWorkspace(t)

	sqliteDB, err := db.Open(filepath.Join(workspacePath, ".backlogit", "index.db"))
	require.NoError(t, err)
	defer sqliteDB.Close()

	// Cutoff before all events — both sessions must be included.
	cutoff := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	result, err := telemetry.HarvestTelemetry(
		context.Background(), workspacePath, copilotPath, sqliteDB,
		telemetry.HarvestOptions{Since: &cutoff},
	)
	require.NoError(t, err)
	assert.Equal(t, 2, result.SessionsHarvested,
		"--since with past cutoff must include all sessions")
}

func TestHarvestTelemetry_Since_FutureDate_ReturnsZeroSessions(t *testing.T) {
	workspacePath, copilotPath := setupSinceWorkspace(t)

	sqliteDB, err := db.Open(filepath.Join(workspacePath, ".backlogit", "index.db"))
	require.NoError(t, err)
	defer sqliteDB.Close()

	future := time.Now().Add(365 * 24 * time.Hour)
	result, err := telemetry.HarvestTelemetry(
		context.Background(), workspacePath, copilotPath, sqliteDB,
		telemetry.HarvestOptions{Since: &future},
	)
	require.NoError(t, err)
	assert.Equal(t, 0, result.SessionsHarvested,
		"--since in the future must return 0 sessions (not an error)")
}

func TestHarvestTelemetry_ZeroTimestampEvents_AlwaysIncluded(t *testing.T) {
	// Events that lack a parseable timestamp (zero Timestamp) must always be
	// included when --since is active — we never silently drop events we can't date.
	workspacePath, copilotPath := setupTelemetryHarvestWorkspace(t)
	logsDir := filepath.Join(copilotPath, "logs")

	// Log line without a timestamp prefix.
	noTSLog := `no-timestamp [telemetry] {"event":"cli.model_call","session_id":"sess-no-ts","request_id":"req-no-ts","model":"gpt-5.1","prompt_tokens_count":50,"completion_tokens_count":20,"total_tokens_count":70,"cached_tokens_count":0,"duration_ms":50}
`
	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "no-ts.log"), []byte(noTSLog), 0o644))

	sqliteDB, err := db.Open(filepath.Join(workspacePath, ".backlogit", "index.db"))
	require.NoError(t, err)
	defer sqliteDB.Close()

	// Use a recent cutoff that would normally exclude old events.
	cutoff := time.Now().Add(-time.Hour)
	result, err := telemetry.HarvestTelemetry(
		context.Background(), workspacePath, copilotPath, sqliteDB,
		telemetry.HarvestOptions{Since: &cutoff},
	)
	require.NoError(t, err)
	assert.Equal(t, 1, result.SessionsHarvested,
		"events with zero timestamp must always be included regardless of --since")
}
