package db

import (
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // SQLite driver registration
)

// Open returns a configured SQLite connection in WAL mode.
//
// Worker: Implement SQLite connection with WAL mode, foreign keys, and busy timeout.
func Open(dbPath string) (*sql.DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			db.Close()
			return nil, fmt.Errorf("set pragma %q: %w", p, err)
		}
	}
	return db, nil
}
