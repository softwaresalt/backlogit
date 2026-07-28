package events

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

func durableEvent(id string) Event {
	return Event{Actor: "agent", ItemID: id, EventType: "state_change", Delta: map[string]any{"k": "v"}}
}

// TestNewEventWriter_DefaultsDurableOff asserts the back-compat constructor
// leaves durability off.
func TestNewEventWriter_DefaultsDurableOff(t *testing.T) {
	t.Parallel()
	w := NewEventWriter(t.TempDir())
	assert.False(t, w.durable, "durable must default off for NewEventWriter(logsDir)")
}

// TestWithDurableWrites_EnablesDurable asserts the construction option flips the
// immutable durable flag.
func TestWithDurableWrites_EnablesDurable(t *testing.T) {
	t.Parallel()
	w := NewEventWriter(t.TempDir(), WithDurableWrites(true))
	assert.True(t, w.durable, "WithDurableWrites(true) must enable durable appends")
}

// TestAppendEvent_DurableFsyncsFile asserts a durable append fsyncs the log file.
func TestAppendEvent_DurableFsyncsFile(t *testing.T) {
	dir := t.TempDir()
	w := NewEventWriter(dir, WithDurableWrites(true))
	fileSyncs := 0
	w.fsyncFileImpl = func(f *os.File) error { fileSyncs++; return f.Sync() }
	w.dirSyncEnabled = false // isolate the file-sync assertion

	require.NoError(t, w.AppendEvent(context.Background(), durableEvent("T1")))
	assert.Equal(t, 1, fileSyncs, "durable append must fsync the log file exactly once")
	assert.FileExists(t, filepath.Join(dir, "T1.jsonl"))
}

// TestAppendEvent_DurableFsyncsParentDirPOSIX asserts the parent logs dir is
// fsynced on a durable append when directory syncing is enabled (POSIX).
func TestAppendEvent_DurableFsyncsParentDirPOSIX(t *testing.T) {
	dir := t.TempDir()
	w := NewEventWriter(dir, WithDurableWrites(true))
	w.dirSyncEnabled = true
	var synced []string
	w.fsyncDirImpl = func(p string) error { synced = append(synced, p); return nil }

	require.NoError(t, w.AppendEvent(context.Background(), durableEvent("T1")))
	assert.Contains(t, synced, dir, "durable append must fsync the parent logs dir on POSIX")
}

// TestAppendEvent_DurableWindowsSkipsDirFsync asserts the Windows best-effort
// dirent behavior: no directory fsync is attempted when dir syncing is disabled.
func TestAppendEvent_DurableWindowsSkipsDirFsync(t *testing.T) {
	dir := t.TempDir()
	w := NewEventWriter(dir, WithDurableWrites(true))
	w.dirSyncEnabled = false // models Windows (no directory-handle flush)
	dirSyncCalled := false
	w.fsyncDirImpl = func(string) error { dirSyncCalled = true; return nil }

	require.NoError(t, w.AppendEvent(context.Background(), durableEvent("T1")))
	assert.False(t, dirSyncCalled, "Windows dirent durability is best-effort; no dir fsync attempted")
}

// TestAppendEvent_DirFsyncFailureIsIndeterminate asserts a post-write dir fsync
// failure surfaces ErrWriteIndeterminate while the append is already visible.
func TestAppendEvent_DirFsyncFailureIsIndeterminate(t *testing.T) {
	dir := t.TempDir()
	w := NewEventWriter(dir, WithDurableWrites(true))
	w.dirSyncEnabled = true
	w.fsyncDirImpl = func(string) error { return errors.New("simulated dir fsync failure") }

	err := w.AppendEvent(context.Background(), durableEvent("T1"))
	require.Error(t, err)
	assert.True(t, blerrors.IsWriteIndeterminate(err),
		"a post-write dir fsync failure must be indeterminate (append not atomic)")
	assert.FileExists(t, filepath.Join(dir, "T1.jsonl"),
		"the append is already visible even though the dir fsync failed")
}

// TestAppendEvent_FileFsyncFailureIsIndeterminate asserts a post-write file
// fsync failure surfaces ErrWriteIndeterminate.
func TestAppendEvent_FileFsyncFailureIsIndeterminate(t *testing.T) {
	dir := t.TempDir()
	w := NewEventWriter(dir, WithDurableWrites(true))
	w.dirSyncEnabled = false
	w.fsyncFileImpl = func(*os.File) error { return errors.New("simulated file fsync failure") }

	err := w.AppendEvent(context.Background(), durableEvent("T1"))
	require.Error(t, err)
	assert.True(t, blerrors.IsWriteIndeterminate(err),
		"a post-write file fsync failure must be indeterminate")
}

// TestAppendEvent_DurableOnFreshTreeCreatesAndSyncsAncestors asserts a durable
// append into a not-yet-existent logs subtree creates and fsyncs the new
// ancestors level-by-level, then writes the event.
func TestAppendEvent_DurableOnFreshTreeCreatesAndSyncsAncestors(t *testing.T) {
	base := t.TempDir()
	logs := filepath.Join(base, "a", "b", "logs")
	w := NewEventWriter(logs, WithDurableWrites(true))
	w.dirSyncEnabled = true
	var synced []string
	w.fsyncDirImpl = func(p string) error { synced = append(synced, p); return nil }

	require.NoError(t, w.AppendEvent(context.Background(), durableEvent("T1")))
	assert.FileExists(t, filepath.Join(logs, "T1.jsonl"))
	// Each newly created ancestor's parent is fsynced so the new dirent is durable.
	assert.Contains(t, synced, filepath.Join(base, "a", "b"), "parent of new logs dir must be fsynced")
}

// TestAppendEvent_DurableOffUnchanged asserts the durable-off fast path still
// writes without invoking any fsync seam.
func TestAppendEvent_DurableOffUnchanged(t *testing.T) {
	dir := t.TempDir()
	w := NewEventWriter(dir)
	w.fsyncFileImpl = func(*os.File) error { t.Fatal("durable-off must not fsync the file"); return nil }
	w.fsyncDirImpl = func(string) error { t.Fatal("durable-off must not fsync the dir"); return nil }

	require.NoError(t, w.AppendEvent(context.Background(), durableEvent("T1")))
	assert.FileExists(t, filepath.Join(dir, "T1.jsonl"))
}
