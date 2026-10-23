package telemetry

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	_ "modernc.org/sqlite" // SQLite driver registration
)

var errInjectedClose = errors.New("injected close failure")

// seedSessionStoreForCloseTest creates a minimal session-store.db with one session.
func seedSessionStoreForCloseTest(t *testing.T) string {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "session-store.db")
	db, err := sql.Open("sqlite", dbPath)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE sessions (id TEXT PRIMARY KEY, cwd TEXT, repository TEXT, branch TEXT, created_at TEXT)`)
	require.NoError(t, err)
	_, err = db.Exec(`INSERT INTO sessions (id, cwd, repository, branch, created_at) VALUES ('sess-close', '/w', 'o/r', 'b', '2026-01-01T00:00:00Z')`)
	require.NoError(t, err)
	require.NoError(t, db.Close())
	return dbPath
}

// TestReadSessionStore_PropagatesDBCloseError forces the database handle close to
// fail and requires the failure to be joined into the error while the populated
// map is still returned.
func TestReadSessionStore_PropagatesDBCloseError(t *testing.T) {
	dbPath := seedSessionStoreForCloseTest(t)
	orig := closeSessionDB
	t.Cleanup(func() { closeSessionDB = orig })
	closeSessionDB = func(db *sql.DB) error {
		require.NoError(t, db.Close()) // release the real handle
		return errInjectedClose
	}

	metas, err := ReadSessionStore(dbPath)
	require.Error(t, err)
	assert.ErrorIs(t, err, errInjectedClose)
	assert.Contains(t, err.Error(), "close session-store.db")
	assert.Contains(t, metas, "sess-close", "populated map must still be returned with the close error")
}

// TestReadSessionStore_PropagatesRowsCloseError forces the rows close to fail.
func TestReadSessionStore_PropagatesRowsCloseError(t *testing.T) {
	dbPath := seedSessionStoreForCloseTest(t)
	orig := closeSessionRows
	t.Cleanup(func() { closeSessionRows = orig })
	closeSessionRows = func(rows *sql.Rows) error {
		require.NoError(t, rows.Close())
		return errInjectedClose
	}

	metas, err := ReadSessionStore(dbPath)
	require.Error(t, err)
	assert.ErrorIs(t, err, errInjectedClose)
	assert.Contains(t, err.Error(), "close session-store rows")
	assert.Contains(t, metas, "sess-close")
}

// TestReadSessionStore_JoinsBothCloseErrors proves the two close failures are
// both preserved rather than the later one overwriting the earlier one.
func TestReadSessionStore_JoinsBothCloseErrors(t *testing.T) {
	dbPath := seedSessionStoreForCloseTest(t)
	origDB, origRows := closeSessionDB, closeSessionRows
	t.Cleanup(func() { closeSessionDB, closeSessionRows = origDB, origRows })
	closeSessionDB = func(db *sql.DB) error {
		require.NoError(t, db.Close())
		return errInjectedClose
	}
	closeSessionRows = func(rows *sql.Rows) error {
		require.NoError(t, rows.Close())
		return errInjectedClose
	}

	_, err := ReadSessionStore(dbPath)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "close session-store.db")
	assert.Contains(t, err.Error(), "close session-store rows")
}

// TestHarvestEventsFacts_PropagatesFactFileCloseErrors forces both fact-file
// closes to fail and requires both failures in the returned error while the
// harvested counts are preserved.
func TestHarvestEventsFacts_PropagatesFactFileCloseErrors(t *testing.T) {
	dir := t.TempDir()
	copilotDir := filepath.Join(dir, ".copilot")
	workspacePath := filepath.Join(dir, ".backlogit")
	sessionDir := filepath.Join(copilotDir, "session-state", "session-close")
	require.NoError(t, os.MkdirAll(sessionDir, 0o755))
	require.NoError(t, os.MkdirAll(workspacePath, 0o755))

	events := buildEventsJSONL([]string{
		`{"type":"session.start","timestamp":"2026-05-01T10:00:00Z","data":{"sessionId":"session-close","startTime":"2026-05-01T10:00:00Z","context":{"branch":"main","repository":"owner/repo"}}}`,
		`{"type":"tool.execution_start","timestamp":"2026-05-01T10:00:01Z","data":{"toolCallId":"tc-001","toolName":"view","mcpServerName":"","mcpToolName":"","turnId":"t1"}}`,
		`{"type":"tool.execution_complete","timestamp":"2026-05-01T10:00:02Z","data":{"toolCallId":"tc-001","model":"claude-sonnet","turnId":"t1","success":true}}`,
		`{"type":"session.shutdown","timestamp":"2026-05-01T10:05:00Z","data":{"totalApiDurationMs":0,"totalPremiumRequests":0,"currentTokens":0,"systemTokens":1000,"conversationTokens":500,"toolDefinitionsTokens":200,"modelMetrics":{}}}`,
	})
	require.NoError(t, os.WriteFile(filepath.Join(sessionDir, "events.jsonl"), []byte(events), 0o644))

	orig := closeFactFile
	t.Cleanup(func() { closeFactFile = orig })
	closeFactFile = func(f *os.File) error {
		require.NoError(t, f.Close()) // release the real handle
		return errInjectedClose
	}

	cp := &HarvestCheckpoint{FileOffsets: make(map[string]int64)}
	tcAdded, sfAdded, err := HarvestEventsFacts(copilotDir, workspacePath, cp, time.Now(), false)
	require.Error(t, err)
	assert.ErrorIs(t, err, errInjectedClose)
	assert.Contains(t, err.Error(), "close tool-calls.jsonl")
	assert.Contains(t, err.Error(), "close session-facts.jsonl")
	assert.Equal(t, 1, tcAdded, "counts must still be reported alongside the close error")
	assert.Equal(t, 1, sfAdded)
}
