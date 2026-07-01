package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTaskLock_AcquireReleaseRoundTrip(t *testing.T) {
	dir := t.TempDir()
	taskPath := filepath.Join(dir, "100.001-T.md")
	require.NoError(t, os.WriteFile(taskPath, []byte("---\nid: 100.001-T\n---\nbody"), 0o644))

	unlock, err := lockTaskFile(taskPath)
	require.NoError(t, err)
	require.NotNil(t, unlock)

	sidecar := taskLockSidecarPath(taskPath)
	_, statErr := os.Stat(sidecar)
	assert.NoError(t, statErr, "sidecar must exist while the lock is held")

	require.NoError(t, unlock())
	_, statErr = os.Stat(sidecar)
	assert.True(t, os.IsNotExist(statErr), "sidecar must be gone after release")

	// unlock is idempotent (safe under defer + explicit call).
	assert.NoError(t, unlock())
}

func TestTaskLock_BusyWhenSidecarPresentDoesNotBlock(t *testing.T) {
	dir := t.TempDir()
	taskPath := filepath.Join(dir, "100.002-T.md")

	// Pre-create a FRESH sidecar to simulate a concurrent holder in another
	// process. lockTaskFile must observe busy without blocking.
	sidecar := taskLockSidecarPath(taskPath)
	require.NoError(t, os.WriteFile(sidecar, []byte("held"), 0o644))

	done := make(chan struct{})
	var unlock func() error
	var err error
	go func() {
		unlock, err = lockTaskFile(taskPath)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("lockTaskFile blocked on a busy sidecar instead of returning promptly")
	}

	assert.Nil(t, unlock)
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrTaskBusy), "busy path must return ErrTaskBusy, got %v", err)
}

func TestTaskLock_StaleSidecarRecoveredWithWarn(t *testing.T) {
	dir := t.TempDir()
	taskPath := filepath.Join(dir, "100.003-T.md")

	// Age a leftover sidecar beyond the stale TTL (crash residue).
	sidecar := taskLockSidecarPath(taskPath)
	require.NoError(t, os.WriteFile(sidecar, []byte("stale"), 0o644))
	old := time.Now().Add(-2 * taskStaleLockTTL)
	require.NoError(t, os.Chtimes(sidecar, old, old))

	unlock, err := lockTaskFile(taskPath)
	require.NoError(t, err, "a stale sidecar must be reclaimed, not treated as busy")
	require.NotNil(t, unlock)

	// The reclaimed lock is now held by us.
	_, statErr := os.Stat(sidecar)
	assert.NoError(t, statErr)

	require.NoError(t, unlock())
}

// TestTaskLock_IOErrorNotClassifiedAsBusy is the regression guard for the
// exit-code contract: a sidecar-creation failure for a reason OTHER than
// "already exists" (here, a missing parent directory → ENOENT) must surface as
// an ordinary error, NOT ErrTaskBusy. Misclassifying it as busy would map a
// genuine IO fault to exit 4 instead of exit 3 and trigger misleading retries.
func TestTaskLock_IOErrorNotClassifiedAsBusy(t *testing.T) {
	dir := t.TempDir()
	// Parent directory intentionally does NOT exist, so O_CREATE|O_EXCL fails
	// with ENOENT (not EEXIST).
	taskPath := filepath.Join(dir, "no-such-dir", "100.004-T.md")

	unlock, err := lockTaskFile(taskPath)
	assert.Nil(t, unlock)
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrTaskBusy),
		"a non-EEXIST IO error must NOT be reported as busy, got %v", err)
}
