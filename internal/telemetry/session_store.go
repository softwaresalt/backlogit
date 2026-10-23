package telemetry

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"

	_ "modernc.org/sqlite" // SQLite driver registration
)

// closeSessionDB and closeSessionRows are test seams for forcing close failures.
var (
	closeSessionDB   = func(db *sql.DB) error { return db.Close() }
	closeSessionRows = func(rows *sql.Rows) error { return rows.Close() }
)

// ReadSessionStore opens the session-store.db SQLite database at dbPath
// (read-only) and returns a map of sessionID → SessionMeta with branch,
// repository, work directory, and timing data. Returns an empty map (not an
// error) when the database is missing or inaccessible — graceful fallback per
// Plan Review F3.
// A failure while closing the read-only handles is joined into the returned
// error alongside any populated map; callers must treat the map as valid data.
func ReadSessionStore(dbPath string) (result map[string]SessionMeta, retErr error) {
	result = make(map[string]SessionMeta)
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		slog.Debug("session-store.db not found, skipping", "path", dbPath)
		return result, nil
	}
	db, err := sql.Open("sqlite", "file:"+dbPath+"?mode=ro")
	if err != nil {
		slog.Warn("failed to open session-store.db", "path", dbPath, "err", err)
		return result, nil
	}
	defer func() {
		if closeErr := closeSessionDB(db); closeErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("close session-store.db: %w", closeErr))
		}
	}()

	rows, err := db.Query(`SELECT id, COALESCE(repository,''), COALESCE(branch,''), COALESCE(cwd,''), COALESCE(created_at,'') FROM sessions`)
	if err != nil {
		return nil, fmt.Errorf("query session-store: %w", err)
	}
	defer func() {
		if closeErr := closeSessionRows(rows); closeErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("close session-store rows: %w", closeErr))
		}
	}()

	for rows.Next() {
		var meta SessionMeta
		if err := rows.Scan(&meta.SessionID, &meta.Repository, &meta.Branch, &meta.WorkDir, &meta.StartedAt); err != nil {
			slog.Warn("failed to scan session row", "err", err)
			continue
		}
		result[meta.SessionID] = meta
	}
	return result, rows.Err()
}
