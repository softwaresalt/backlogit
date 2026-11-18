package db_test

import (
	"context"
	"database/sql"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/db"
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
	assert.Equal(t, 30000, busyTimeout, "busy_timeout must be 30000 ms")
}

// TestOpen_PragmasApplied verifies that Open sets journal_mode=WAL,
// foreign_keys=1, and busy_timeout=5000 via DSN query parameters.
func TestOpen_PragmasApplied(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "pragma-all.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close()) })

	assertConnectionPragmas(t, database)
}

func TestOpen_ForeignKeysEnforced(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "pragma-test.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close()) })

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
	t.Cleanup(func() { require.NoError(t, database.Close()) })

	var mode string
	require.NoError(t, database.QueryRow(`PRAGMA journal_mode`).Scan(&mode))
	assert.Equal(t, "wal", mode)
}

func TestOpen_BusyTimeout(t *testing.T) {
	database, err := db.Open(filepath.Join(t.TempDir(), "busy-timeout.db"))
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, database.Close()) })

	var timeout int
	require.NoError(t, database.QueryRow(`PRAGMA busy_timeout`).Scan(&timeout))
	assert.Equal(t, 30000, timeout)
}

// TestOpen_MultipleOpenSameFile verifies that two independent *sql.DB handles
// opened against the same file each honour journal_mode, foreign_keys, and
// busy_timeout.  DSN-based PRAGMAs must be applied per connection, not once
// globally on the file.
func TestOpen_MultipleOpenSameFile(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "multi.db")

	first, err := db.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, first.Close()) })

	second, err := db.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, second.Close()) })

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
	t.Cleanup(func() { require.NoError(t, first.Close()) })

	second, err := db.Open(dbPath)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, second.Close()) })

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
	t.Cleanup(func() { require.NoError(t, database.Close()) })
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

func TestU4_OpenPragmaOrderAndPooledSchemaInitialization(t *testing.T) {
	t.Run("pragma_order_busy_timeout_before_wal_and_foreign_keys", func(t *testing.T) {
		_, thisFile, _, ok := runtime.Caller(0)
		require.True(t, ok, "resolve connection_test.go path")
		connPath := filepath.Join(filepath.Dir(thisFile), "connection.go")

		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, connPath, nil, 0)
		require.NoError(t, err)

		var openDecl *ast.FuncDecl
		for _, decl := range file.Decls {
			if fn, isFn := decl.(*ast.FuncDecl); isFn && fn.Name.Name == "Open" {
				openDecl = fn
				break
			}
		}
		require.NotNil(t, openDecl, "Open function must exist in connection.go")

		var pragmas []string
		ast.Inspect(openDecl.Body, func(n ast.Node) bool {
			call, isCall := n.(*ast.CallExpr)
			if !isCall || len(call.Args) != 2 {
				return true
			}
			sel, isSel := call.Fun.(*ast.SelectorExpr)
			if !isSel || sel.Sel.Name != "Add" {
				return true
			}
			recv, isIdent := sel.X.(*ast.Ident)
			if !isIdent || recv.Name != "q" {
				return true
			}
			keyLit, isKeyLit := call.Args[0].(*ast.BasicLit)
			valLit, isValLit := call.Args[1].(*ast.BasicLit)
			if !isKeyLit || !isValLit || keyLit.Kind != token.STRING || valLit.Kind != token.STRING {
				return true
			}
			key, kErr := strconv.Unquote(keyLit.Value)
			val, vErr := strconv.Unquote(valLit.Value)
			if kErr == nil && vErr == nil && key == "_pragma" {
				pragmas = append(pragmas, val)
			}
			return true
		})

		indexOf := func(target string) int {
			for i, p := range pragmas {
				if p == target {
					return i
				}
			}
			return -1
		}

		busyIdx := indexOf("busy_timeout(30000)")
		walIdx := indexOf("journal_mode(WAL)")
		fkIdx := indexOf("foreign_keys(1)")
		require.NotEqual(t, -1, busyIdx, "Open must add _pragma=busy_timeout(30000); got %v", pragmas)
		require.NotEqual(t, -1, walIdx, "Open must add _pragma=journal_mode(WAL); got %v", pragmas)
		require.NotEqual(t, -1, fkIdx, "Open must add _pragma=foreign_keys(1); got %v", pragmas)
		require.Less(t, busyIdx, walIdx, "_pragma=busy_timeout(30000) must precede _pragma=journal_mode(WAL); got %v", pragmas)
		require.Less(t, busyIdx, fkIdx, "_pragma=busy_timeout(30000) must precede _pragma=foreign_keys(1); got %v", pragmas)
	})

	t.Run("open_initializes_persistent_wal_header_on_disk", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "wal-header.db")
		database, err := db.Open(dbPath)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, database.Close()) })

		hdr, err := os.ReadFile(dbPath)
		require.NoError(t, err)
		require.GreaterOrEqual(t, len(hdr), 100, "db.Open must initialize SQLite Page 1 on disk")
		assert.Equal(t, byte(2), hdr[18], "SQLite header byte 18 (write version) must be 2 for WAL")
		assert.Equal(t, byte(2), hdr[19], "SQLite header byte 19 (read version) must be 2 for WAL")
	})

	t.Run("pooled_connections_under_active_begin_immediate", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "pooled-wal.db")
		database, err := db.Open(dbPath)
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, database.Close()) })
		require.NoError(t, db.EnsureSchema(database))

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		conn1, err := database.Conn(ctx)
		require.NoError(t, err)
		txActive := false
		t.Cleanup(func() {
			if txActive {
				_, rollbackErr := conn1.ExecContext(context.Background(), `ROLLBACK`)
				assert.NoError(t, rollbackErr)
			}
			require.NoError(t, conn1.Close())
		})

		_, err = conn1.ExecContext(ctx, `BEGIN IMMEDIATE`)
		require.NoError(t, err)
		txActive = true

		_, err = conn1.ExecContext(ctx,
			`INSERT INTO commit_links (item_id, commit_sha, message, author) VALUES (?, ?, ?, ?)`,
			"216.005-T", "sha-conn1", "initial", "tester",
		)
		require.NoError(t, err)

		const workers = 4
		errs := make([]error, workers)
		modes := make([]string, workers)
		startWrites := make(chan struct{})
		var readsDone sync.WaitGroup
		var writesDone sync.WaitGroup
		readsDone.Add(workers)
		writesDone.Add(workers)

		for i := 0; i < workers; i++ {
			go func(workerID int) {
				defer writesDone.Done()

				var mode string
				if qErr := database.QueryRowContext(ctx, `PRAGMA journal_mode`).Scan(&mode); qErr != nil {
					readsDone.Done()
					errs[workerID] = fmt.Errorf("worker %d pragma journal_mode: %w", workerID, qErr)
					return
				}
				modes[workerID] = mode

				var count int
				if qErr := database.QueryRowContext(ctx, `SELECT COUNT(*) FROM commit_links`).Scan(&count); qErr != nil {
					readsDone.Done()
					errs[workerID] = fmt.Errorf("worker %d select commit_links: %w", workerID, qErr)
					return
				}
				readsDone.Done()

				select {
				case <-startWrites:
				case <-ctx.Done():
					errs[workerID] = fmt.Errorf("worker %d wait startWrites: %w", workerID, ctx.Err())
					return
				}

				if _, execErr := database.ExecContext(ctx,
					`INSERT INTO commit_links (item_id, commit_sha, message, author) VALUES (?, ?, ?, ?)`,
					"216.005-T", fmt.Sprintf("sha-worker-%d", workerID), "pooled", "worker",
				); execErr != nil {
					errs[workerID] = fmt.Errorf("worker %d insert commit_links: %w", workerID, execErr)
					return
				}
			}(i)
		}

		readsDone.Wait()
		close(startWrites)
		time.Sleep(50 * time.Millisecond)

		_, err = conn1.ExecContext(ctx, `COMMIT`)
		require.NoError(t, err)
		txActive = false

		writesDone.Wait()

		for i := 0; i < workers; i++ {
			require.NoError(t, errs[i])
			assert.Equal(t, "wal", modes[i])
		}

		var total int
		require.NoError(t, database.QueryRowContext(ctx, `SELECT COUNT(*) FROM commit_links`).Scan(&total))
		assert.Equal(t, 1+workers, total)
	})

	t.Run("concurrent_first_open_on_new_database", func(t *testing.T) {
		dbPath := filepath.Join(t.TempDir(), "concurrent-first-open.db")
		const workers = 8
		handles := make([]*sql.DB, workers)
		errs := make([]error, workers)
		modes := make([]string, workers)

		ready := make(chan struct{})
		var wg sync.WaitGroup
		wg.Add(workers)
		for i := 0; i < workers; i++ {
			go func(idx int) {
				defer wg.Done()
				<-ready
				h, err := db.Open(dbPath)
				if err != nil {
					errs[idx] = fmt.Errorf("worker %d open: %w", idx, err)
					return
				}
				handles[idx] = h

				var mode string
				if qErr := h.QueryRow(`PRAGMA journal_mode`).Scan(&mode); qErr != nil {
					errs[idx] = fmt.Errorf("worker %d pragma journal_mode: %w", idx, qErr)
					return
				}
				modes[idx] = mode
			}(i)
		}

		close(ready)
		wg.Wait()

		for i := 0; i < workers; i++ {
			if handles[i] != nil {
				assert.NoError(t, handles[i].Close(), "worker %d close", i)
			}
		}
		for i := 0; i < workers; i++ {
			require.NoError(t, errs[i], "worker %d", i)
			assert.Equal(t, "wal", modes[i], "worker %d journal_mode", i)
		}
	})
}
