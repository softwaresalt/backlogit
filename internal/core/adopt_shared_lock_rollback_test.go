package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// holdActiveStashLock takes the lock exactly as HarvestStashEntry and
// ArchiveStashEntry do (the active stash file's sidecar).
func holdActiveStashLock(t *testing.T, ws *Workspace) func() {
	t.Helper()
	path := StashFilePath(ws.RootPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	unlock, err := lockStashFile(path)
	require.NoError(t, err)
	released := false
	release := func() {
		if !released {
			released = true
			_ = unlock()
		}
	}
	t.Cleanup(release)
	return release
}

func requireBlockedUntilReleased(t *testing.T, release func(), done <-chan error) {
	t.Helper()
	select {
	case err := <-done:
		t.Fatalf("operation finished while the active stash lock was held (err=%v)", err)
	case <-time.After(300 * time.Millisecond):
	}
	release()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("operation did not proceed after the active stash lock was released")
	}
}

func TestU3_AdoptArchiveRewriteSharesActiveStashLock(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	archive := filepath.Join(ws.StorageRoot, "archive")
	require.NoError(t, os.MkdirAll(archive, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(archive, stashArchiveFileName),
		[]byte(`{"id":"S1","priority":"high","kind":"bug","text":"t","archived_at":"2026-01-01T00:00:00Z","reason":"harvested","harvested_artifact_id":"001.001-T"}`+"\n"), 0o644))

	release := holdActiveStashLock(t, ws)
	done := make(chan error, 1)
	go func() {
		txn, err := propagateAdoptToStashArchive(ws, "001.001-T", "002.001-T")
		if txn != nil {
			txn.Close()
		}
		done <- err
	}()
	requireBlockedUntilReleased(t, release, done)
}

func TestU3_CorrectStashProvenanceSharesActiveStashLock(t *testing.T) {
	ws, _, _ := u2StashFixture(t)
	release := holdActiveStashLock(t, ws)
	done := make(chan error, 1)
	go func() {
		_, err := CorrectStashProvenance(context.Background(), ws, u2ForceReq())
		done <- err
	}()
	requireBlockedUntilReleased(t, release, done)
}

func TestU3_AdoptLogRenameFailureRestoresCrossReferences(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat1, err := CreateArtifact(ctx, ws, "Origin feature", "feature")
	require.NoError(t, err)
	feat2, err := CreateArtifact(ctx, ws, "Destination feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Task to adopt", "task", WithParent(feat1.ID))
	require.NoError(t, err)
	dep, err := CreateArtifact(ctx, ws, "Dependent task", "task",
		WithParent(feat1.ID), WithDependencies([]string{task.ID}))
	require.NoError(t, err)
	ship, err := CreateShipment(ctx, ws, "Manifest shipment", []string{task.ID})
	require.NoError(t, err)

	taskPath, err := FindArtifactPath(ctx, ws, task.ID)
	require.NoError(t, err)
	depPath, err := FindArtifactPath(ctx, ws, dep.ID)
	require.NoError(t, err)
	shipPath, err := FindArtifactPath(ctx, ws, ship.ID)
	require.NoError(t, err)
	taskBefore, err := os.ReadFile(taskPath)
	require.NoError(t, err)
	depBefore, err := os.ReadFile(depPath)
	require.NoError(t, err)
	shipBefore, err := os.ReadFile(shipPath)
	require.NoError(t, err)

	// Ensure an item log exists so the rename step runs, then fail it.
	logPath := filepath.Join(WorkspaceLogsRoot(ws.RootPath), task.ID+".jsonl")
	require.NoError(t, os.MkdirAll(filepath.Dir(logPath), 0o755))
	require.NoError(t, os.WriteFile(logPath, []byte("{}\n"), 0o644))

	orig := adoptRenameLogFn
	renameErr := errors.New("injected log rename failure")
	adoptRenameLogFn = func(_, _ string) error { return renameErr }
	t.Cleanup(func() { adoptRenameLogFn = orig })

	_, adoptErr := AdoptItem(ctx, ws, task.ID, feat2.ID)
	require.Error(t, adoptErr)
	assert.ErrorIs(t, adoptErr, renameErr)

	for name, c := range map[string]struct {
		path   string
		before []byte
	}{"adopted item": {taskPath, taskBefore}, "dependent": {depPath, depBefore}, "shipment manifest": {shipPath, shipBefore}} {
		after, readErr := os.ReadFile(c.path)
		require.NoError(t, readErr, name)
		assert.Equal(t, string(c.before), string(after), "%s must be restored byte-for-byte", name)
	}
	got, err := FindArtifactPath(ctx, ws, task.ID)
	require.NoError(t, err, "item must still be discoverable by its old ID")
	assert.Equal(t, taskPath, got)
}
