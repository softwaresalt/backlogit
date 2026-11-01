package db

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/binary"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"
)

var snapshotSequence atomic.Uint64

var errSnapshotChanged = errors.New("database snapshot changed or is invalid")

func openReadOnlySnapshot(path string) (result *sql.DB, resultErr error) {
	source, err := openSnapshotSource(path)
	if err != nil {
		return nil, err
	}
	defer func() { resultErr = errors.Join(resultErr, source.close()) }()
	// Reject unsafe leaves before allocating any snapshot cache.
	if _, _, err := source.read(source.name); err != nil {
		return nil, err
	}
	if _, err := source.readWAL(); err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	dir, err := os.MkdirTemp("", "backlogit-ro-*")
	if err != nil {
		return nil, fmt.Errorf("create read-only snapshot directory: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(dir); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("remove read-only snapshot directory: %w", err))
			if result != nil {
				resultErr = errors.Join(resultErr, result.Close())
				result = nil
			}
		}
	}()
	copyPath := filepath.Join(dir, "snapshot.db")
	var lastErr error
	fastRetries := 0
	for ctx.Err() == nil {
		base, wal, err := source.bracket()
		if err != nil && !errors.Is(err, errSnapshotChanged) {
			return nil, err
		}
		if err == nil {
			wal, err = committedSnapshotWAL(base, wal)
		}
		if err == nil {
			result, err = materializeReadOnlySnapshot(ctx, copyPath, base, wal)
			if err == nil {
				return result, nil
			}
		}
		lastErr = err
		if errors.Is(err, errSnapshotChanged) && fastRetries < 64 {
			fastRetries++
			runtime.Gosched()
			continue
		}
		timer := time.NewTimer(time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
		case <-timer.C:
		}
	}
	return nil, fmt.Errorf("acquire read-only database snapshot: %w", errors.Join(ctx.Err(), lastErr))
}

func (s *snapshotSource) readWAL() ([]byte, error) {
	wal, _, err := s.read(s.name + "-wal")
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read snapshot WAL: %w", err)
	}
	return wal, nil
}

// readSnapshotBracket surrounds both database observations with WAL observations.
// The outer WAL reads also detect checkpoint/reset generations crossing the
// requested db1/wal1/db2/wal2 bracket. Equality and SQLite validation are evidence
// for a stable image under cooperating SQLite writers, not an unconditional
// proof against ABA replacement or arbitrary external filesystem mutation.
func readSnapshotBracket(path string) ([]byte, []byte, error) {
	source, err := openSnapshotSource(path)
	if err != nil {
		return nil, nil, err
	}
	defer source.close() //nolint:errcheck
	return source.bracket()
}

func (s *snapshotSource) bracket() (base, wal []byte, resultErr error) {
	dbFile, err := s.openChecked(s.name)
	if err != nil {
		return nil, nil, fmt.Errorf("open snapshot database: %w", err)
	}
	defer func() { resultErr = errors.Join(resultErr, dbFile.Close()) }()
	walName := s.name + "-wal"
	walFile, err := s.openChecked(walName)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, nil, fmt.Errorf("open snapshot WAL: %w", err)
	}
	if walFile != nil {
		defer func() { resultErr = errors.Join(resultErr, walFile.Close()) }()
	}
	readWAL := func() ([]byte, error) {
		if walFile == nil {
			probe, err := s.open(walName)
			if err != nil {
				if errors.Is(err, os.ErrNotExist) {
					return nil, nil
				}
				return nil, fmt.Errorf("probe snapshot WAL: %w", err)
			}
			return nil, errors.Join(fmt.Errorf("snapshot WAL appeared: %w", errSnapshotChanged), probe.Close())
		}
		data, _, err := readCheckedSnapshotFile(walFile)
		if err != nil {
			return nil, fmt.Errorf("read snapshot WAL: %w", err)
		}
		return data, nil
	}
	wal0, err := readWAL()
	if err != nil {
		return nil, nil, err
	}
	base1, stat1, err := readCheckedSnapshotFile(dbFile)
	if err != nil {
		return nil, nil, fmt.Errorf("read snapshot database: %w", err)
	}
	wal1, err := readWAL()
	if err != nil {
		return nil, nil, err
	}
	base2, stat2, err := readCheckedSnapshotFile(dbFile)
	if err != nil {
		return nil, nil, fmt.Errorf("reread snapshot database: %w", err)
	}
	wal2, err := readWAL()
	if err != nil {
		return nil, nil, err
	}
	if !os.SameFile(stat1, stat2) || !stat1.ModTime().Equal(stat2.ModTime()) ||
		!bytes.Equal(base1, base2) || !bytes.Equal(wal0, wal1) || !bytes.Equal(wal1, wal2) {
		return nil, nil, fmt.Errorf("read snapshot bracket: %w", errSnapshotChanged)
	}
	if err := s.check(dbFile, s.name); err != nil {
		return nil, nil, fmt.Errorf("check snapshot database: %w", errors.Join(errSnapshotChanged, err))
	}
	if walFile != nil {
		if err := s.check(walFile, walName); err != nil {
			return nil, nil, fmt.Errorf("check snapshot WAL: %w", errors.Join(errSnapshotChanged, err))
		}
	}
	return base2, wal2, nil
}

// committedSnapshotWAL validates the header and frame checksum chain explicitly:
// SQLite itself may silently ignore a malformed WAL and quick_check the stale
// base successfully. A reset can leave old-generation frames at the end, and a
// writer can leave uncommitted frames; copy only the validated committed prefix.
func committedSnapshotWAL(base, wal []byte) ([]byte, error) {
	if len(wal) == 0 {
		return nil, nil
	}
	invalid := func() ([]byte, error) {
		return nil, fmt.Errorf("validate snapshot WAL: %w", errSnapshotChanged)
	}
	if len(base) < 100 || len(wal) < 32 {
		return invalid()
	}
	magic := binary.BigEndian.Uint32(wal[:4])
	if magic != 0x377f0682 && magic != 0x377f0683 {
		return invalid()
	}
	if binary.BigEndian.Uint32(wal[4:8]) != 3007000 {
		return invalid()
	}
	pageSize := int(binary.BigEndian.Uint32(wal[8:12]))
	basePageSize := int(binary.BigEndian.Uint16(base[16:18]))
	if basePageSize == 1 {
		basePageSize = 65536
	}
	if pageSize < 512 || pageSize > 65536 || pageSize&(pageSize-1) != 0 || pageSize != basePageSize {
		return invalid()
	}
	var order binary.ByteOrder = binary.LittleEndian
	if magic == 0x377f0683 {
		order = binary.BigEndian
	}
	checksum := func(data []byte, s1, s2 uint32) (uint32, uint32) {
		for i := 0; i < len(data); i += 8 {
			s1 += order.Uint32(data[i:i+4]) + s2
			s2 += order.Uint32(data[i+4:i+8]) + s1
		}
		return s1, s2
	}
	s1, s2 := checksum(wal[:24], 0, 0)
	if s1 != binary.BigEndian.Uint32(wal[24:28]) || s2 != binary.BigEndian.Uint32(wal[28:32]) {
		return invalid()
	}
	committedEnd := 0
	frameSize := 24 + pageSize
	hasCurrentGenerationCommit := func(start int) bool {
		// A salt mismatch may itself be corruption. Inspect every remaining
		// aligned full frame, not just the contiguous same-salt segment.
		for tail := start; tail+frameSize <= len(wal); tail += frameSize {
			header := wal[tail : tail+24]
			if bytes.Equal(header[8:16], wal[16:24]) && binary.BigEndian.Uint32(header[4:8]) != 0 {
				return true
			}
		}
		return false
	}
	for pos := 32; pos+frameSize <= len(wal); pos += frameSize {
		frame := wal[pos : pos+frameSize]
		if !bytes.Equal(frame[8:16], wal[16:24]) {
			if pos == 32 || hasCurrentGenerationCommit(pos) {
				// A reset with no current-generation frames is transient;
				// a later current-generation commit makes the tail ambiguous.
				// Retry rather than silently dropping an unverifiable WAL.
				return invalid()
			}
			break // Old generation left behind by WAL restart.
		}
		if binary.BigEndian.Uint32(frame[:4]) == 0 {
			return invalid()
		}
		s1, s2 = checksum(frame[:8], s1, s2)
		s1, s2 = checksum(frame[24:], s1, s2)
		if s1 != binary.BigEndian.Uint32(frame[16:20]) || s2 != binary.BigEndian.Uint32(frame[20:24]) {
			// Rollback/reuse and interrupted writes can leave full,
			// checksum-invalid uncommitted frames after a valid commit.
			// Never drop a potentially committed transaction: a commit
			// marker anywhere in this generation's remaining full frames
			// makes corruption ambiguous, so fail closed instead.
			if committedEnd == 0 || hasCurrentGenerationCommit(pos) {
				return invalid()
			}
			break
		}
		if binary.BigEndian.Uint32(frame[4:8]) != 0 {
			committedEnd = pos + frameSize
		}
	}
	if committedEnd == 0 {
		return nil, nil // No committed transaction in this generation.
	}
	return wal[:committedEnd], nil
}

func materializeReadOnlySnapshot(ctx context.Context, path string, base, wal []byte) (result *sql.DB, resultErr error) {
	// Close each attempt before removing its sidecars; never reuse an old WAL
	// when the next bracket observed an empty or absent WAL.
	for _, suffix := range []string{"-wal", "-shm", ""} {
		if err := os.Remove(path + suffix); err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("remove previous snapshot image: %w", err)
		}
	}
	if err := os.WriteFile(path, base, 0o600); err != nil {
		return nil, fmt.Errorf("write snapshot database: %w", err)
	}
	if len(wal) > 0 {
		if err := os.WriteFile(path+"-wal", wal, 0o600); err != nil {
			return nil, fmt.Errorf("write snapshot WAL: %w", err)
		}
	}
	source, err := Open(path)
	if err != nil {
		return nil, fmt.Errorf("open copied database: %w", err)
	}
	source.SetMaxOpenConns(1)
	defer func() {
		if err := source.Close(); err != nil {
			resultErr = errors.Join(resultErr, fmt.Errorf("close copied database: %w", err))
			if result != nil {
				resultErr = errors.Join(resultErr, result.Close())
				result = nil
			}
		}
	}()
	var check string
	if err := source.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&check); err != nil {
		return nil, fmt.Errorf("check copied database: %w", err)
	}
	if check != "ok" {
		return nil, fmt.Errorf("copied database quick_check %q: %w", check, errSnapshotChanged)
	}
	var busy, pages, checkpointed int
	if err := source.QueryRowContext(ctx, "PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &pages, &checkpointed); err != nil {
		return nil, fmt.Errorf("checkpoint copied database: %w", err)
	}
	if busy != 0 {
		return nil, fmt.Errorf("checkpoint copied database busy: %w", errSnapshotChanged)
	}
	uri := fmt.Sprintf("file:backlogit-ro-%d-%d?mode=memory&cache=shared", os.Getpid(), snapshotSequence.Add(1))
	target, err := sql.Open("sqlite", uri+"&_pragma=query_only(1)")
	if err != nil {
		return nil, fmt.Errorf("open memory snapshot: %w", err)
	}
	target.SetMaxOpenConns(1)
	target.SetMaxIdleConns(1) // The idle connection owns the in-memory lifetime.
	fail := func(err error) (*sql.DB, error) {
		return nil, errors.Join(err, target.Close())
	}
	if err := target.PingContext(ctx); err != nil {
		return fail(fmt.Errorf("pin memory snapshot: %w", err))
	}
	if _, err := source.ExecContext(ctx, "VACUUM INTO ?", uri); err != nil {
		return fail(fmt.Errorf("vacuum copied database into memory: %w", err))
	}
	if err := target.QueryRowContext(ctx, "PRAGMA quick_check").Scan(&check); err != nil {
		return fail(fmt.Errorf("check memory snapshot: %w", err))
	}
	if check != "ok" {
		return fail(fmt.Errorf("memory snapshot quick_check %q: %w", check, errSnapshotChanged))
	}
	return target, nil
}
