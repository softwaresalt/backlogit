package db

import (
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	_ "modernc.org/sqlite" // SQLite driver registration
)

// abs resolves dbPath to an absolute path so that the file: URI always
// references the intended location.  Without this, a relative path like
// ".backlogit/backlogit.db" gets a leading "/" prepended and becomes
// "/.backlogit/backlogit.db", which on Windows resolves to the drive root
// (e.g. D:\.backlogit\backlogit.db) instead of the working directory.
func abs(dbPath string) (string, error) {
	p, err := filepath.Abs(dbPath)
	if err != nil {
		return "", fmt.Errorf("resolve absolute path: %w", err)
	}
	return p, nil
}

// Open returns a configured *sql.DB backed by the SQLite file at dbPath.
//
// DSN options are injected via query parameters so the driver applies them on
// every new physical connection opened from the pool:
//
//   - _pragma=journal_mode(WAL)   — WAL journal enables concurrent readers alongside writers.
//   - _pragma=foreign_keys(1)     — Enforces FK constraints (SQLite disables them by default).
//   - _pragma=busy_timeout(30000) — Waits up to 30 s before returning SQLITE_BUSY; gives
//     multi-process MCP workloads enough headroom to serialise writes.
//   - _txlock=immediate           — All transactions use BEGIN IMMEDIATE, acquiring the
//     write lock at transaction start rather than on the first write. This eliminates
//     lock-upgrade conflicts that produce SQLITE_BUSY mid-transaction.
//
// Pool sizing: SetMaxOpenConns(4) / SetMaxIdleConns(4).
// WAL mode supports multiple simultaneous readers so a pool > 1 is safe and
// beneficial when the MCP server and CLI share a process.  4 is a conservative
// ceiling for a single-user tool; raising it further would add contention on the
// write lock without meaningful throughput gain.  (F-19)
func Open(dbPath string) (*sql.DB, error) {
	// Resolve to absolute so that the file: URI always references the
	// intended location regardless of the caller's working directory.
	absPath, err := abs(dbPath)
	if err != nil {
		return nil, err
	}

	// Build a canonical file: URI accepted by modernc.org/sqlite v1.34.0.
	//
	// filepath.ToSlash on Windows converts "C:\path\db" → "C:/path/db".
	// A path that does not start with "/" is interpreted as *relative* by the
	// SQLite URI parser, which would create a file in the current working
	// directory instead of the intended location.  Prepend "/" to produce the
	// triple-slash form "file:///C:/path/db" that SQLite treats as absolute on
	// all platforms.
	slashPath := filepath.ToSlash(absPath)
	if !strings.HasPrefix(slashPath, "/") {
		slashPath = "/" + slashPath
	}

	q := url.Values{}
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "foreign_keys(1)")
	q.Add("_pragma", "busy_timeout(30000)")
	q.Set("_txlock", "immediate")

	dsn := (&url.URL{
		Scheme:   "file",
		Path:     slashPath,
		RawQuery: q.Encode(),
	}).String()

	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// WAL allows concurrent readers; match idle to open so the pool does not
	// discard connections that are immediately reacquired under load.
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return db, nil
}
