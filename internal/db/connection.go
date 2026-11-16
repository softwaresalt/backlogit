package db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
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
//   - _pragma=busy_timeout(30000) — Waits up to 30 s before returning SQLITE_BUSY; must
//     precede journal_mode(WAL) so pooled connections wait for any active writer
//     before checking or transitioning journal mode.
//   - _pragma=journal_mode(WAL)   — WAL journal enables concurrent readers alongside writers.
//   - _pragma=foreign_keys(1)     — Enforces FK constraints (SQLite disables them by default).
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
	if strings.TrimSpace(dbPath) == "" {
		return nil, fmt.Errorf("open database: empty database path")
	}

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
	q.Add("_pragma", "busy_timeout(30000)")
	q.Add("_pragma", "journal_mode(WAL)")
	q.Add("_pragma", "foreign_keys(1)")
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
		if closeErr := db.Close(); closeErr != nil {
			return nil, fmt.Errorf("ping database: %w", errors.Join(err, fmt.Errorf("close database after ping failure: %w", closeErr)))
		}
		return nil, fmt.Errorf("ping database: %w", err)
	}

	if err := ensureWALHeaderPersisted(context.Background(), db, absPath); err != nil {
		if closeErr := db.Close(); closeErr != nil {
			return nil, fmt.Errorf("initialize WAL database: %w", errors.Join(err, fmt.Errorf("close database after WAL initialization failure: %w", closeErr)))
		}
		return nil, fmt.Errorf("initialize WAL database: %w", err)
	}
	return db, nil
}

// ensureWALHeaderPersisted verifies that the connection is operating in WAL
// journal mode and materializes the 100-byte SQLite Page 1 WAL header
// (bytes 18..19 == 2, 2) on disk when the database file is empty (0 bytes) so
// subsequent pooled connections never open in delete mode against a 0-byte file.
func ensureWALHeaderPersisted(ctx context.Context, db *sql.DB, absPath string) (err error) {
	conn, err := db.Conn(ctx)
	if err != nil {
		return fmt.Errorf("acquire connection for WAL verification: %w", err)
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close WAL verification connection: %w", closeErr))
		}
	}()

	var journalMode string
	if err := conn.QueryRowContext(ctx, `PRAGMA journal_mode`).Scan(&journalMode); err != nil {
		return fmt.Errorf("query journal_mode: %w", err)
	}
	if !strings.EqualFold(journalMode, "wal") {
		return fmt.Errorf("verify WAL journal_mode: got %q, want %q", journalMode, "wal")
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return fmt.Errorf("stat database file: %w", err)
	}
	if info.Size() == 0 {
		if _, err := conn.ExecContext(ctx, `PRAGMA user_version = 0`); err != nil {
			return fmt.Errorf("initialize WAL database header: %w", err)
		}
		var busy, logFrames, checkpointed int
		if err := conn.QueryRowContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`).Scan(&busy, &logFrames, &checkpointed); err != nil {
			return fmt.Errorf("checkpoint WAL database header: %w", err)
		}
		if busy != 0 {
			return fmt.Errorf("checkpoint WAL database header: checkpoint busy (%d)", busy)
		}
		info, err = os.Stat(absPath)
		if err != nil {
			return fmt.Errorf("stat database file after WAL initialization: %w", err)
		}
	}

	if info.Size() < 100 {
		return fmt.Errorf("verify WAL header size: got %d bytes, want >= 100", info.Size())
	}
	if err := verifyWALHeaderBytes(absPath); err != nil {
		return err
	}
	return nil
}

func verifyWALHeaderBytes(absPath string) (err error) {
	f, err := os.Open(absPath)
	if err != nil {
		return fmt.Errorf("open database file for WAL header check: %w", err)
	}
	defer func() {
		if closeErr := f.Close(); closeErr != nil {
			err = errors.Join(err, fmt.Errorf("close database file after WAL header check: %w", closeErr))
		}
	}()

	var hdr [100]byte
	if _, err := io.ReadFull(f, hdr[:]); err != nil {
		return fmt.Errorf("read SQLite Page 1 header: %w", err)
	}
	if hdr[18] != 2 || hdr[19] != 2 {
		return fmt.Errorf("verify WAL header bytes 18..19: got [%d, %d], want [2, 2]", hdr[18], hdr[19])
	}
	return nil
}

// OpenReadOnly returns a query-only in-memory snapshot of dbPath, including
// committed WAL data. It never SQLite-opens the source or creates source
// sidecars. Snapshot acquisition retries changing/invalid images for up to 30s.
// The caller must bind dbPath's directory to its validated storage root. All
// source ancestors and leaves must be real, unlinked files/directories; this
// function does not bless arbitrary symlink targets as trusted roots.
func OpenReadOnly(dbPath string) (*sql.DB, error) {
	absPath, err := abs(dbPath)
	if err != nil {
		return nil, err
	}
	return openReadOnlySnapshot(absPath)
}
