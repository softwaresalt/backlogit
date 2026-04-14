package db_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"

	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/telemetry"
)

func setupTelemetryDB(t *testing.T) *sql.DB {
	t.Helper()
	sqliteDB, err := db.Open(filepath.Join(t.TempDir(), "test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { sqliteDB.Close() })
	return sqliteDB
}

func TestEnsureTelemetrySchema_CreatesTablesIdempotently(t *testing.T) {
	sqliteDB := setupTelemetryDB(t)

	// First call should succeed.
	require.NoError(t, db.EnsureTelemetrySchema(sqliteDB))

	// Second call should also succeed (idempotent).
	require.NoError(t, db.EnsureTelemetrySchema(sqliteDB))

	// Tables should exist and be SELECTable.
	rows1, err := sqliteDB.Query("SELECT session_id, total_tokens FROM telemetry_sessions LIMIT 0")
	require.NoError(t, err, "telemetry_sessions table should exist after EnsureTelemetrySchema")
	rows1.Close()

	rows2, err := sqliteDB.Query("SELECT session_id, server_name, tool_name, call_count FROM telemetry_tool_usage LIMIT 0")
	require.NoError(t, err, "telemetry_tool_usage table should exist after EnsureTelemetrySchema")
	rows2.Close()
}

func TestRehydrateTelemetry_IsIdempotent(t *testing.T) {
	sqliteDB := setupTelemetryDB(t)
	require.NoError(t, db.EnsureTelemetrySchema(sqliteDB))

	// Write sample telemetry-sessions.jsonl into a temp workspace.
	workspacePath := t.TempDir()
	backlogitDir := filepath.Join(workspacePath, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	writeSampleTelemetryJSONL(t, backlogitDir)

	ctx := context.Background()
	require.NoError(t, db.RehydrateTelemetry(ctx, workspacePath, sqliteDB))
	countAfterFirst := countTelemetrySessions(t, sqliteDB)

	require.NoError(t, db.RehydrateTelemetry(ctx, workspacePath, sqliteDB))
	countAfterSecond := countTelemetrySessions(t, sqliteDB)

	assert.Equal(t, countAfterFirst, countAfterSecond, "re-rehydration should produce identical row count")
	assert.Greater(t, countAfterFirst, 0, "rehydration should insert at least one session row")
}

func TestRehydrateTelemetry_CompositeKeyPreventsDuplicates(t *testing.T) {
	sqliteDB := setupTelemetryDB(t)
	require.NoError(t, db.EnsureTelemetrySchema(sqliteDB))

	workspacePath := t.TempDir()
	backlogitDir := filepath.Join(workspacePath, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	writeSampleTelemetryJSONL(t, backlogitDir)

	ctx := context.Background()
	require.NoError(t, db.RehydrateTelemetry(ctx, workspacePath, sqliteDB))

	var count int
	err := sqliteDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM telemetry_tool_usage WHERE session_id='sess-t1' AND server_name='backlogit' AND tool_name='backlogit_create_item'`,
	).Scan(&count)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "composite PK should prevent duplicates on re-rehydrate")
}

func TestTelemetryTablesQueryableViaGate(t *testing.T) {
	// After rehydration, backlogit_query_sql should be able to SELECT from telemetry tables.
	sqliteDB := setupTelemetryDB(t)
	require.NoError(t, db.EnsureTelemetrySchema(sqliteDB))
	require.NoError(t, db.EnsureSchema(sqliteDB))

	rows, err := sqliteDB.Query("SELECT session_id, total_tokens FROM telemetry_sessions LIMIT 10")
	require.NoError(t, err)
	defer rows.Close()
	assert.NoError(t, rows.Err())
}

// writeSampleTelemetryJSONL writes a minimal telemetry-sessions.jsonl fixture.
func writeSampleTelemetryJSONL(t *testing.T, backlogitDir string) {
	t.Helper()
	content := `{"record_type":"session_summary","harvested_at":"2026-04-09T00:00:00Z","session_id":"sess-t1","branch":"main","repository":"test/repo","total_tokens":1500,"prompt_tokens":1000,"completion_tokens":500,"cached_tokens":200,"model_calls":1,"tool_calls":2,"tokens_by_model":{"claude-sonnet-4":1500},"tool_calls_by_server":{"backlogit":2},"completed_tasks":[],"tokens_per_task":null,"compaction_count":0}
{"record_type":"tool_usage","harvested_at":"2026-04-09T00:00:00Z","session_id":"sess-t1","server_name":"backlogit","tool_name":"backlogit_create_item","call_count":1,"total_duration_ms":45}
{"record_type":"tool_usage","harvested_at":"2026-04-09T00:00:00Z","session_id":"sess-t1","server_name":"engram","tool_name":"engram-query_memory","call_count":1,"total_duration_ms":30}
`
	require.NoError(t, os.WriteFile(
		filepath.Join(backlogitDir, "telemetry-sessions.jsonl"),
		[]byte(content), 0o644,
	))
}

func countTelemetrySessions(t *testing.T, sqliteDB *sql.DB) int {
	t.Helper()
	var count int
	require.NoError(t, sqliteDB.QueryRow("SELECT COUNT(*) FROM telemetry_sessions").Scan(&count))
	return count
}

// Ensure telemetry package is imported for shared types in tests.
var _ = telemetry.SessionSummaryRecord{}
