package db_test

// TestDSNPragmaSpike verifies which DSN formats modernc.org/sqlite v1.34.0
// accepts on Windows, and whether _pragma query parameters are applied.
//
// This spike is intentionally verbose: each sub-test captures a distinct
// hypothesis so the results can drive the production Open() implementation.
// The spike file can be removed once the implementation decision is recorded.

import (
	"database/sql"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// openRaw opens a SQLite database via the named driver with the exact DSN
// provided. It does NOT call Ping; callers must verify liveness themselves.
func openRaw(dsn string) (*sql.DB, error) {
	return sql.Open("sqlite", dsn)
}

// queryPragma reads a single PRAGMA string value from db.
func queryPragma(db *sql.DB, pragma string) (string, error) {
	var val string
	err := db.QueryRow(fmt.Sprintf("PRAGMA %s", pragma)).Scan(&val)
	return val, err
}

// queryPragmaInt reads a single PRAGMA integer value from db.
func queryPragmaInt(db *sql.DB, pragma string) (int, error) {
	var val int
	err := db.QueryRow(fmt.Sprintf("PRAGMA %s", pragma)).Scan(&val)
	return val, err
}

// TestDSNPragmaSpike covers five DSN strategies in order of preference.
// Each sub-test is independent; failures are reported but do not stop others.
func TestDSNPragmaSpike(t *testing.T) {
	dir := t.TempDir()

	// ── Helper: assert WAL + FK + busy_timeout via PRAGMA queries ──────────
	assertPragmas := func(t *testing.T, db *sql.DB) {
		t.Helper()

		jm, err := queryPragma(db, "journal_mode")
		if err != nil {
			t.Errorf("PRAGMA journal_mode: %v", err)
		} else if jm != "wal" {
			t.Errorf("journal_mode: want wal, got %q", jm)
		} else {
			t.Logf("✓ journal_mode = %q", jm)
		}

		fk, err := queryPragmaInt(db, "foreign_keys")
		if err != nil {
			t.Errorf("PRAGMA foreign_keys: %v", err)
		} else if fk != 1 {
			t.Errorf("foreign_keys: want 1, got %d", fk)
		} else {
			t.Logf("✓ foreign_keys = %d", fk)
		}

		bt, err := queryPragmaInt(db, "busy_timeout")
		if err != nil {
			t.Errorf("PRAGMA busy_timeout: %v", err)
		} else if bt != 5000 {
			t.Errorf("busy_timeout: want 5000, got %d", bt)
		} else {
			t.Logf("✓ busy_timeout = %d", bt)
		}
	}

	// ── Strategy 1: Plain Windows path (no URI, no pragmas) ─────────────────
	// Hypothesis: the driver accepts a raw file path on Windows.
	t.Run("plain_path_opens", func(t *testing.T) {
		dbPath := filepath.Join(dir, "plain.db")
		db, err := openRaw(dbPath)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		defer db.Close()
		if err := db.Ping(); err != nil {
			t.Fatalf("ping: %v", err)
		}
		t.Log("✓ plain Windows path accepted by driver")
	})

	// ── Strategy 2: file:/// URI (triple-slash) with _pragma params ─────────
	// Hypothesis: file:///C:/path?_pragma=journal_mode(WAL) works on Windows.
	t.Run("file_triple_slash_with_pragma", func(t *testing.T) {
		dbPath := filepath.Join(dir, "triple.db")
		slashPath := filepath.ToSlash(dbPath)
		if !strings.HasPrefix(slashPath, "/") {
			slashPath = "/" + slashPath // "C:/..." → "/C:/..."
		}
		q := url.Values{}
		q.Add("_pragma", "journal_mode(WAL)")
		q.Add("_pragma", "foreign_keys(1)")
		q.Add("_pragma", "busy_timeout(5000)")
		dsn := fmt.Sprintf("file://%s?%s", slashPath, q.Encode())
		t.Logf("DSN: %s", dsn)

		db, err := openRaw(dsn)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		defer db.Close()
		if err := db.Ping(); err != nil {
			t.Fatalf("ping: %v", err)
		}
		t.Log("✓ file:// URI opened successfully")
		assertPragmas(t, db)
	})

	// ── Strategy 3: file: URI with url.URL builder (current code, fixed) ───
	// Hypothesis: url.URL{Scheme:"file", Path:"/C:/..."} produces file:///C:/...
	t.Run("url_builder_fixed_leading_slash", func(t *testing.T) {
		dbPath := filepath.Join(dir, "urlbuilder.db")
		slashPath := filepath.ToSlash(dbPath)
		if !strings.HasPrefix(slashPath, "/") {
			slashPath = "/" + slashPath
		}
		q := url.Values{}
		q.Add("_pragma", "journal_mode(WAL)")
		q.Add("_pragma", "foreign_keys(1)")
		q.Add("_pragma", "busy_timeout(5000)")
		dsn := (&url.URL{
			Scheme:   "file",
			Path:     slashPath,
			RawQuery: q.Encode(),
		}).String()
		t.Logf("DSN: %s", dsn)

		db, err := openRaw(dsn)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		defer db.Close()
		if err := db.Ping(); err != nil {
			t.Fatalf("ping: %v", err)
		}
		t.Log("✓ url.URL builder with fixed path opened successfully")
		assertPragmas(t, db)
	})

	// ── Strategy 4: _pragma=key=value (equals syntax) ───────────────────────
	// Some SQLite URI drivers accept _pragma=journal_mode=WAL rather than
	// journal_mode(WAL). Test both syntaxes.
	t.Run("pragma_equals_syntax", func(t *testing.T) {
		dbPath := filepath.Join(dir, "eqsyntax.db")
		slashPath := filepath.ToSlash(dbPath)
		if !strings.HasPrefix(slashPath, "/") {
			slashPath = "/" + slashPath
		}
		// url.Values would percent-encode '=', so build RawQuery manually.
		dsn := fmt.Sprintf("file://%s?_pragma=journal_mode%%3DWAL&_pragma=foreign_keys%%3D1&_pragma=busy_timeout%%3D5000", slashPath)
		t.Logf("DSN: %s", dsn)

		db, err := openRaw(dsn)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		defer db.Close()
		if err := db.Ping(); err != nil {
			t.Fatalf("ping: %v", err)
		}
		t.Log("✓ equals-syntax pragma DSN opened successfully")
		assertPragmas(t, db)
	})

	// ── Strategy 5: Post-open Exec PRAGMAs (fallback baseline) ─────────────
	// This is always correct; it verifies the fallback mechanism is sound.
	t.Run("post_open_exec_pragmas", func(t *testing.T) {
		dbPath := filepath.Join(dir, "exec.db")
		db, err := openRaw(dbPath)
		if err != nil {
			t.Fatalf("open: %v", err)
		}
		defer db.Close()
		db.SetMaxOpenConns(1) // Ensure one connection so pragmas persist.
		for _, p := range []string{
			"PRAGMA journal_mode=WAL",
			"PRAGMA foreign_keys=ON",
			"PRAGMA busy_timeout=5000",
		} {
			if _, err := db.Exec(p); err != nil {
				t.Fatalf("exec pragma %q: %v", p, err)
			}
		}
		assertPragmas(t, db)
		t.Log("✓ post-open Exec approach works (fallback baseline)")
	})
}
