package telemetry

import (
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"os"

	_ "modernc.org/sqlite" // SQLite driver registration
)

// ReadSessionStore opens the session-store.db SQLite database at dbPath
// (read-only) and returns a map of sessionID → SessionMeta with branch,
// repository, work directory, and timing data. Returns an empty map (not an
// error) when the database is missing or inaccessible — graceful fallback per
// Plan Review F3.
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
		if closeErr := db.Close(); closeErr != nil {
			retErr = errors.Join(retErr, fmt.Errorf("close session-store.db: %w", closeErr))
		}
	}()

	rows, err := db.Query(`SELECT id, COALESCE(repository,''), COALESCE(branch,''), COALESCE(cwd,''), COALESCE(created_at,'') FROM sessions`)
	if err != nil {
		return nil, fmt.Errorf("query session-store: %w", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
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
