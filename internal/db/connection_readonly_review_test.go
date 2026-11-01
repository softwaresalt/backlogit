package db

import (
	"context"
	"database/sql"
	"encoding/binary"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestReviewSnapshotWALCrashTail(t *testing.T) {
	writer, path := reviewReadOnlyWriter(t)
	_, err := writer.Exec("UPDATE pairs SET a=9, b=9")
	require.NoError(t, err)
	base, err := os.ReadFile(path)
	require.NoError(t, err)
	wal, err := os.ReadFile(path + "-wal")
	require.NoError(t, err)
	prefix, err := committedSnapshotWAL(base, wal)
	require.NoError(t, err)
	frameSize := 24 + int(binary.BigEndian.Uint32(wal[8:12]))
	for _, committed := range []bool{false, true} {
		t.Run(map[bool]string{false: "uncommitted-crash-tail", true: "corrupt-committed-tail"}[committed], func(t *testing.T) {
			tail := append([]byte(nil), wal[len(wal)-frameSize:]...)
			if !committed {
				binary.BigEndian.PutUint32(tail[4:8], 0)
			}

			tail[24] ^= 0xff // Full frame with invalid checksum.
			image := append(append([]byte(nil), wal...), tail...)
			valid, err := committedSnapshotWAL(base, image)
			if committed {
				require.ErrorIs(t, err, errSnapshotChanged)
			} else {
				require.NoError(t, err)
				require.Equal(t, prefix, valid)
				copyPath := filepath.Join(t.TempDir(), "crashed.db")
				// SQLite recovery independently establishes the same prefix.
				require.NoError(t, os.WriteFile(copyPath, base, 0o600))
				require.NoError(t, os.WriteFile(copyPath+"-wal", image, 0o600))
				recovered, err := Open(copyPath)
				require.NoError(t, err)
				defer func() { require.NoError(t, recovered.Close()) }()
				var a int
				require.NoError(t, recovered.QueryRow("SELECT a FROM pairs").Scan(&a))
				require.Equal(t, 9, a)
			}
		})
	}
}

func TestReviewSnapshotWALRollbackReuse(t *testing.T) {
	writer, path := reviewReadOnlyWriter(t)
	_, err := writer.Exec("PRAGMA cache_size=5; CREATE TABLE spill (id INTEGER PRIMARY KEY, data BLOB); PRAGMA wal_checkpoint(TRUNCATE)")
	require.NoError(t, err)
	tx, err := writer.Begin()
	require.NoError(t, err)
	for i := range 80 {
		_, err = tx.Exec("INSERT INTO spill VALUES (?, zeroblob(4000))", i)
		require.NoError(t, err)
	}
	spilled, err := os.ReadFile(path + "-wal")
	require.NoError(t, err)
	require.Greater(t, len(spilled), 32, "transaction must spill actual uncommitted WAL frames")
	require.NoError(t, tx.Rollback())
	_, err = writer.Exec("UPDATE pairs SET a=11, b=11")
	require.NoError(t, err)
	base, err := os.ReadFile(path)
	require.NoError(t, err)
	reused, err := os.ReadFile(path + "-wal")
	require.NoError(t, err)
	require.Equal(t, spilled[16:24], reused[16:24], "rollback reuse stays in the same WAL generation")
	prefix, err := committedSnapshotWAL(base, reused)
	require.NoError(t, err)
	require.Less(t, len(prefix), len(reused), "reused WAL has leftover full uncommitted frames")
	copyPath := filepath.Join(t.TempDir(), "rollback.db")
	require.NoError(t, os.WriteFile(copyPath, base, 0o600))
	require.NoError(t, os.WriteFile(copyPath+"-wal", reused, 0o600))
	// Independent SQLite recovery and the zero-write snapshot must agree.
	recovered, err := Open(copyPath)
	require.NoError(t, err)
	defer func() { require.NoError(t, recovered.Close()) }()
	snapshot, err := OpenReadOnly(path)
	require.NoError(t, err)
	defer func() { require.NoError(t, snapshot.Close()) }()
	for _, db := range []*sql.DB{recovered, snapshot} {
		var a, count int
		require.NoError(t, db.QueryRow("SELECT a FROM pairs").Scan(&a))
		require.Equal(t, 11, a)
		require.NoError(t, db.QueryRow("SELECT COUNT(*) FROM spill").Scan(&count))
		require.Zero(t, count)
	}
	after, err := os.ReadFile(path + "-wal")
	require.NoError(t, err)
	require.Equal(t, reused, after)
}

func TestReviewSnapshotWALCorruptTailWithLaterCommit(t *testing.T) {
	_, path := reviewReadOnlyWriter(t)
	base, err := os.ReadFile(path)
	require.NoError(t, err)
	wal, err := os.ReadFile(path + "-wal")
	require.NoError(t, err)
	frameSize := 24 + int(binary.BigEndian.Uint32(wal[8:12]))
	uncommitted := append([]byte(nil), wal[len(wal)-frameSize:]...)
	binary.BigEndian.PutUint32(uncommitted[4:8], 0)
	uncommitted[24] ^= 0xff
	image := append(append(append([]byte(nil), wal...), uncommitted...), wal[len(wal)-frameSize:]...)
	_, err = committedSnapshotWAL(base, image)
	require.ErrorIs(t, err, errSnapshotChanged, "a later commit marker must not be silently dropped")
}

func TestReviewVacuumIntoSharedMemory(t *testing.T) {
	source, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	source.SetMaxOpenConns(1)
	defer func() { require.NoError(t, source.Close()) }()
	_, err = source.Exec("CREATE TABLE probe (n INTEGER); INSERT INTO probe VALUES (42)")
	require.NoError(t, err)
	target, err := sql.Open("sqlite", "file:review-vacuum-probe?mode=memory&cache=shared")
	require.NoError(t, err)
	target.SetMaxOpenConns(1)
	defer func() { require.NoError(t, target.Close()) }()
	require.NoError(t, target.Ping())
	_, err = source.Exec("VACUUM INTO ?", "file:review-vacuum-probe?mode=memory&cache=shared")
	require.NoError(t, err)
	var n int
	require.NoError(t, target.QueryRow("SELECT n FROM probe").Scan(&n))
	require.Equal(t, 42, n)
}

func reviewReadOnlyWriter(t *testing.T) (*sql.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "source.db")
	writer, err := Open(path)
	require.NoError(t, err)
	writer.SetMaxOpenConns(1)
	t.Cleanup(func() { require.NoError(t, writer.Close()) })
	_, err = writer.Exec("PRAGMA wal_autocheckpoint=0; CREATE TABLE pairs (id INTEGER PRIMARY KEY, a INTEGER, b INTEGER); INSERT INTO pairs VALUES (1, 0, 0)")
	require.NoError(t, err)
	return writer, path
}

func TestReviewReadOnlyPreservesCommittedWAL(t *testing.T) {
	writer, path := reviewReadOnlyWriter(t)
	_, err := writer.Exec("UPDATE pairs SET a=7, b=7")
	require.NoError(t, err)
	beforeDB, err := os.ReadFile(path)
	require.NoError(t, err)
	beforeWAL, err := os.ReadFile(path + "-wal")
	require.NoError(t, err)
	entries, err := os.ReadDir(filepath.Dir(path))
	require.NoError(t, err)
	beforeSHM, err := os.ReadFile(path + "-shm")
	require.NoError(t, err)
	tempRoot := t.TempDir()
	t.Setenv("TMPDIR", tempRoot)
	t.Setenv("TMP", tempRoot)
	t.Setenv("TEMP", tempRoot)
	ro, err := OpenReadOnly(path)
	require.NoError(t, err)
	tempEntries, err := os.ReadDir(tempRoot)
	require.NoError(t, err)
	require.Empty(t, tempEntries, "temporary files must be gone before returning the memory handle")
	var a, b int
	require.NoError(t, ro.QueryRow("SELECT a, b FROM pairs").Scan(&a, &b))
	require.Equal(t, 7, a)
	require.Equal(t, a, b)
	_, err = ro.Exec("UPDATE pairs SET a=99")
	require.Error(t, err)
	require.NoError(t, ro.Close())
	afterDB, err := os.ReadFile(path)
	require.NoError(t, err)
	afterWAL, err := os.ReadFile(path + "-wal")
	require.NoError(t, err)
	afterEntries, err := os.ReadDir(filepath.Dir(path))
	require.NoError(t, err)
	afterSHM, err := os.ReadFile(path + "-shm")
	require.NoError(t, err)
	require.Equal(t, beforeDB, afterDB)
	require.Equal(t, beforeWAL, afterWAL)
	require.Equal(t, beforeSHM, afterSHM)
	require.Len(t, afterEntries, len(entries))
	for i, entry := range entries {
		require.Equal(t, entry.Name(), afterEntries[i].Name())
	}
}

func TestReviewReadOnlyRejectsInvalidWALInsteadOfSilentlyDroppingIt(t *testing.T) {
	writer, path := reviewReadOnlyWriter(t)
	_, err := writer.Exec("PRAGMA wal_checkpoint(TRUNCATE); UPDATE pairs SET a=8, b=8")
	require.NoError(t, err)
	base, err := os.ReadFile(path)
	require.NoError(t, err)
	wal, err := os.ReadFile(path + "-wal")
	require.NoError(t, err)
	require.Greater(t, len(wal), 32)
	wal[0] ^= 0xff
	copyPath := filepath.Join(t.TempDir(), "invalid.db")
	require.NoError(t, os.WriteFile(copyPath, base, 0o600))
	require.NoError(t, os.WriteFile(copyPath+"-wal", wal, 0o600))
	ro, err := OpenReadOnly(copyPath)
	if ro != nil {
		require.NoError(t, ro.Close())
	}
	require.Error(t, err, "SQLite can ignore invalid WAL and quick_check the stale base successfully")
}

func TestReviewSnapshotWALValidation(t *testing.T) {
	_, path := reviewReadOnlyWriter(t)
	base, err := os.ReadFile(path)
	require.NoError(t, err)
	wal, err := os.ReadFile(path + "-wal")
	require.NoError(t, err)
	for _, test := range []struct {
		name   string
		offset int
	}{
		{name: "header-checksum", offset: 24},
		{name: "first-frame-salt", offset: 40},
		{name: "frame-checksum", offset: 48},
	} {
		t.Run(test.name, func(t *testing.T) {
			invalid := append([]byte(nil), wal...)
			invalid[test.offset] ^= 0xff
			_, err := committedSnapshotWAL(base, invalid)
			require.ErrorIs(t, err, errSnapshotChanged)
		})
	}
	valid, err := committedSnapshotWAL(base, wal)
	require.NoError(t, err)
	require.NotEmpty(t, valid)
}

func TestReviewReadOnlyConcurrentWriterAndCheckpoint(t *testing.T) {
	writer, path := reviewReadOnlyWriter(t)
	ctx, cancel := context.WithCancel(context.Background())
	var wg sync.WaitGroup
	wg.Add(1)
	done := make(chan error, 1)
	go func() {
		defer wg.Done()
		for n := 1; ; n++ {
			if ctx.Err() != nil {
				done <- nil
				return
			}
			if _, err := writer.ExecContext(ctx,
				"INSERT INTO pairs VALUES (1, ?, ?) ON CONFLICT(id) DO UPDATE SET a=excluded.a, b=excluded.b", n, n); err != nil {
				if ctx.Err() != nil {
					done <- nil
				} else {
					done <- err
				}
				return
			}
			if n%7 == 0 {
				if _, err := writer.ExecContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)"); err != nil && ctx.Err() == nil {
					done <- err
					return
				}
			}
		}
	}()
	t.Cleanup(func() {
		cancel()
		wg.Wait()
		require.NoError(t, <-done)
	})
	for range 20 {
		ro, err := OpenReadOnly(path)
		require.NoError(t, err)
		var a, b int
		err = ro.QueryRow("SELECT a, b FROM pairs").Scan(&a, &b)
		closeErr := ro.Close()
		require.NoError(t, err)
		require.NoError(t, closeErr)
		require.Equal(t, a, b)
	}
}
