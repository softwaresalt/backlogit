package telemetry_test

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"

	"github.com/backlogit/backlogit/internal/telemetry"
)

func TestReadSessionStore_MissingDB(t *testing.T) {
	// When session-store.db does not exist, ReadSessionStore returns an
	// empty map and no error (graceful fallback per plan).
	metas, err := telemetry.ReadSessionStore("/nonexistent/path/session-store.db")
	require.NoError(t, err)
	assert.Empty(t, metas, "missing DB should return empty map, not error")
}

func TestReadSessionStore_ReadsSessionMetadata(t *testing.T) {
	// Create a minimal session-store.db with the expected schema.
	dbPath := filepath.Join(t.TempDir(), "session-store.db")
	createMinimalSessionStoreDB(t, dbPath)

	metas, err := telemetry.ReadSessionStore(dbPath)
	require.NoError(t, err)
	require.Contains(t, metas, "sess-abc", "should find seeded session")

	meta := metas["sess-abc"]
	assert.Equal(t, "sess-abc", meta.SessionID)
	assert.Equal(t, "feat/test-branch", meta.Branch)
	assert.Equal(t, "backlogit/backlogit", meta.Repository)
}

// createMinimalSessionStoreDB seeds a SQLite file matching the Copilot CLI schema.
func createMinimalSessionStoreDB(t *testing.T, dbPath string) {
	t.Helper()
	db := openTestDB(t, dbPath)
	defer db.Close()

	_, err := db.Exec(`CREATE TABLE IF NOT EXISTS sessions (
		id         TEXT PRIMARY KEY,
		cwd        TEXT,
		repository TEXT,
		branch     TEXT,
		summary    TEXT,
		created_at TEXT,
		updated_at TEXT
	)`)
	require.NoError(t, err)

	_, err = db.Exec(`INSERT INTO sessions (id, cwd, repository, branch, created_at, updated_at)
		VALUES ('sess-abc', '/workspace', 'backlogit/backlogit', 'feat/test-branch', '2026-04-09T00:00:00Z', '2026-04-09T01:00:00Z')`)
	require.NoError(t, err)
}
