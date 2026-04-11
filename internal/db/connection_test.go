package db_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/db"
)

// assertConnectionPragmas is a table-driven helper that queries the three
// PRAGMAs Open() is required to set and asserts their expected values.
func assertConnectionPragmas(t *testing.T, database *sql.DB) {
	t.Helper()

	var journalMode string
	require.NoError(t, database.QueryRow(`PRAGMA journal_mode`).Scan(&journalMode))
	assert.Equal(t, "wal", journalMode, "journal_mode must be WAL")

	var foreignKeys int
	require.NoError(t, database.QueryRow(`PRAGMA foreign_keys`).Scan(&foreignKeys))
	assert.Equal(t, 1, foreignKeys, "foreign_keys must be enabled")

	var busyTimeout int
	require.NoError(t, database.QueryRow(`PRAGMA busy_timeout`).Scan(&busyTimeout))
	assert.Equal(t, 5000, busyTimeout, "busy_timeout must be 5000 ms")
}

// TestOpen_PragmasApplied verifies that Open sets journal_mode=WAL,
// foreign_keys=1, and busy_timeout=5000 via DSN query parameters.
func TestOpen_PragmasApplied(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "pragma-all.db"))
	require.NoError(t, err)
	t.Cleanup(func() { database.Close() })

	assertConnectionPragmas(t, database)
}

func TestOpen_ForeignKeysEnforced(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "pragma-test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { database.Close() })

	_, err = database.Exec(`
		CREATE TABLE parent (id TEXT PRIMARY KEY);
		CREATE TABLE child (
			id TEXT PRIMARY KEY,
			parent_id TEXT NOT NULL,
			FOREIGN KEY(parent_id) REFERENCES parent(id)
		);
	`)
	require.NoError(t, err)

	_, err = database.Exec(`INSERT INTO child (id, parent_id) VALUES ('child-1', 'missing-parent')`)

	require.Error(t, err)
}

func TestOpen_WALMode(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "wal-test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { database.Close() })

	var mode string
	require.NoError(t, database.QueryRow(`PRAGMA journal_mode`).Scan(&mode))
	assert.Equal(t, "wal", mode)
}

func TestOpen_BusyTimeout(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "busy-timeout.db"))
	require.NoError(t, err)
	t.Cleanup(func() { database.Close() })

	var timeout int
	require.NoError(t, database.QueryRow(`PRAGMA busy_timeout`).Scan(&timeout))
	assert.Equal(t, 5000, timeout)
}

// TestOpen_MultipleOpenSameFile verifies that two independent *sql.DB handles
// opened against the same file each honour journal_mode, foreign_keys, and
// busy_timeout.  DSN-based PRAGMAs must be applied per connection, not once
// globally on the file.
func TestOpen_MultipleOpenSameFile(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "multi.db")

	first, err := db.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { first.Close() })

	second, err := db.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { second.Close() })

	t.Run("first_handle", func(t *testing.T) {
		assertConnectionPragmas(t, first)
	})
	t.Run("second_handle", func(t *testing.T) {
		assertConnectionPragmas(t, second)
	})
}

func TestOpen_MultipleOpenSameFile_InheritsSettings(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "shared.db")

	first, err := db.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { first.Close() })

	second, err := db.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { second.Close() })

	_, err = first.Exec(`
		CREATE TABLE parent (id TEXT PRIMARY KEY);
		CREATE TABLE child (
			id TEXT PRIMARY KEY,
			parent_id TEXT NOT NULL,
			FOREIGN KEY(parent_id) REFERENCES parent(id)
		);
	`)
	require.NoError(t, err)

	for _, database := range []*sql.DB{first, second} {
		_, execErr := database.Exec(`INSERT INTO child (id, parent_id) VALUES (?, ?)`, filepath.Base(dbPath), "missing-parent")
		require.Error(t, execErr)
	}
}

func TestOpen_InvalidPath_ReturnsError(t *testing.T) {
	invalidPath := filepath.Join(t.TempDir(), "missing", "nested", "db.sqlite")

	database, err := db.Open(invalidPath)

	require.Error(t, err)
	assert.Nil(t, database)
}

func TestOpen_RehydrationPreservesPragmas(t *testing.T) {
	root := t.TempDir()
	backlogRoot := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogRoot, 0o755))

	database, err := db.Open(filepath.Join(backlogRoot, "backlogit.db"))
	require.NoError(t, err)
	t.Cleanup(func() { database.Close() })
	require.NoError(t, db.EnsureSchema(database))

	ctx := context.Background()
	_, err = database.ExecContext(ctx, `
		CREATE TABLE parent (id TEXT PRIMARY KEY);
		CREATE TABLE child (
			id TEXT PRIMARY KEY,
			parent_id TEXT NOT NULL,
			FOREIGN KEY(parent_id) REFERENCES parent(id)
		);
	`)
	require.NoError(t, err)

	_, err = db.Rehydrate(ctx, backlogRoot, database)
	require.NoError(t, err)

	// Verify all three PRAGMAs still hold after schema creation and rehydration.
	assertConnectionPragmas(t, database)

	// Verify FK enforcement is live (not just reported as enabled).
	_, err = database.ExecContext(ctx, `INSERT INTO child (id, parent_id) VALUES ('child-1', 'missing-parent')`)
	require.Error(t, err)
}

