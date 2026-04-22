package events_test

// Harness for 040.001-T: Add fsync to hook event queue writes.
// Harness for 040.004-T: Fix hook queue stale-lock TOCTOU race.
//
// RED tests:
//   - TestHookEventWriter_StaleLockRecovery_PreExistingRecoveringFile: FAILS
//     with the current implementation because the current stale-lock recovery
//     does not pre-reap a leftover .recovering file. The new rename-based
//     algorithm removes it as part of the recovery sequence.
//
// Contract tests (currently passing, establish regression boundary):
//   - TestHookEventWriter_AppendHookEvent_DurableWrite
//   - TestHookEventWriter_StaleLockRecovery_SingleWriter
//   - TestHookEventWriter_FreshLock_NotRemoved
//   - TestHookEventWriter_StaleLockRecovery_ConcurrentRecovery

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/events"
)

// TestHookEventWriter_AppendHookEvent_DurableWrite confirms that a written
// event survives a re-read from the queue file (contract boundary for 040.001-T).
func TestHookEventWriter_AppendHookEvent_DurableWrite(t *testing.T) {
	dir := t.TempDir()
	w := events.NewHookEventWriter(dir)
	ctx := context.Background()

	_, err := w.AppendHookEvent(ctx, events.HookEvent{
		EventType: events.HookEventBlockedStale,
		Payload:   map[string]any{"item_id": "040.001-T"},
	})
	require.NoError(t, err)

	stored, err := w.ReadHookEvents(ctx)
	require.NoError(t, err)
	require.Len(t, stored, 1)
	assert.Equal(t, events.HookEventBlockedStale, stored[0].EventType)
	assert.Equal(t, "040.001-T", stored[0].Payload["item_id"])
}

// TestHookEventWriter_StaleLockRecovery_SingleWriter confirms that a stale
// lock file (old mtime) left by a crashed process does not permanently block
// new writers.
func TestHookEventWriter_StaleLockRecovery_SingleWriter(t *testing.T) {
	dir := t.TempDir()
	w := events.NewHookEventWriter(dir)
	ctx := context.Background()

	queuePath := filepath.Join(dir, "hooks_queue.jsonl")
	lockPath := queuePath + ".lock"
	require.NoError(t, os.WriteFile(lockPath, []byte("stale"), 0o644))
	staleTime := time.Now().Add(-2 * time.Minute)
	require.NoError(t, os.Chtimes(lockPath, staleTime, staleTime))

	_, err := w.AppendHookEvent(ctx, events.HookEvent{EventType: events.HookEventBlockedStale})
	require.NoError(t, err, "stale lock recovery must succeed")

	_, statErr := os.Stat(lockPath)
	assert.True(t, os.IsNotExist(statErr), "lock file must be removed after stale recovery")
}

// TestHookEventWriter_FreshLock_NotRemoved confirms that a fresh lock held by
// another concurrent process is not removed and the write fails clearly.
func TestHookEventWriter_FreshLock_NotRemoved(t *testing.T) {
	dir := t.TempDir()
	w := events.NewHookEventWriter(dir)
	ctx := context.Background()

	queuePath := filepath.Join(dir, "hooks_queue.jsonl")
	lockPath := queuePath + ".lock"
	require.NoError(t, os.WriteFile(lockPath, []byte("held"), 0o644))
	// Lock has a current mtime — well within the stale TTL.

	_, err := w.AppendHookEvent(ctx, events.HookEvent{EventType: events.HookEventBlockedStale})
	require.Error(t, err, "write must fail when another process holds a fresh lock")

	_, statErr := os.Stat(lockPath)
	assert.NoError(t, statErr, "fresh lock must not be removed by a failing writer")
}

// TestHookEventWriter_StaleLockRecovery_PreExistingRecoveringFile verifies
// that a leftover .recovering file from a prior crashed recovery attempt is
// removed (pre-reaped) before the new rename-based recovery proceeds.
//
// RED: The current implementation uses os.Remove(lockPath) then O_CREATE|O_EXCL
// retry. It never creates or cleans up a .recovering file. When this test runs
// against the current code, the .recovering file persists after the call —
// failing the final assertion. The rename-based TOCTOU fix pre-reaps it.
func TestHookEventWriter_StaleLockRecovery_PreExistingRecoveringFile(t *testing.T) {
	dir := t.TempDir()
	w := events.NewHookEventWriter(dir)
	ctx := context.Background()

	queuePath := filepath.Join(dir, "hooks_queue.jsonl")
	lockPath := queuePath + ".lock"
	recoveringPath := lockPath + ".recovering"

	// Stale lock left by a crashed process.
	require.NoError(t, os.WriteFile(lockPath, []byte("stale"), 0o644))
	staleTime := time.Now().Add(-2 * time.Minute)
	require.NoError(t, os.Chtimes(lockPath, staleTime, staleTime))

	// Orphaned .recovering file left by a process that crashed mid-recovery.
	require.NoError(t, os.WriteFile(recoveringPath, []byte("orphan"), 0o644))

	_, err := w.AppendHookEvent(ctx, events.HookEvent{EventType: events.HookEventBlockedStale})
	require.NoError(t, err,
		"stale lock recovery must succeed even when a .recovering file already exists")

	// The .recovering file must be cleaned up by the pre-reap step.
	_, statErr := os.Stat(recoveringPath)
	assert.True(t, os.IsNotExist(statErr),
		".recovering file must be removed during stale lock recovery (pre-reap)")
}

// TestHookEventWriter_StaleLockRecovery_ConcurrentRecovery verifies that when
// multiple goroutines simultaneously detect and attempt to recover the same
// stale lock, the rename-based atomic recovery ensures all events are written
// with unique sequence numbers (no interleaving or duplication).
func TestHookEventWriter_StaleLockRecovery_ConcurrentRecovery(t *testing.T) {
	dir := t.TempDir()
	w := events.NewHookEventWriter(dir)
	ctx := context.Background()

	queuePath := filepath.Join(dir, "hooks_queue.jsonl")
	lockPath := queuePath + ".lock"

	// Pre-create a stale lock so all goroutines hit the recovery path.
	require.NoError(t, os.WriteFile(lockPath, []byte("stale"), 0o644))
	staleTime := time.Now().Add(-2 * time.Minute)
	require.NoError(t, os.Chtimes(lockPath, staleTime, staleTime))

	const n = 5
	type result struct {
		seq int64
		err error
	}
	results := make(chan result, n)

	for range n {
		go func() {
			seq, err := w.AppendHookEvent(ctx, events.HookEvent{
				EventType: events.HookEventBlockedStale,
			})
			results <- result{seq, err}
		}()
	}

	seqs := make(map[int64]bool)
	var successes int
	for range n {
		r := <-results
		if r.err == nil {
			successes++
			assert.False(t, seqs[r.seq], "duplicate sequence number %d", r.seq)
			seqs[r.seq] = true
		}
	}
	assert.GreaterOrEqual(t, successes, 1, "at least one concurrent recovery must succeed")
	assert.Equal(t, successes, len(seqs), "all successful writes must have unique sequence numbers")
}
