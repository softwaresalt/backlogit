package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// TestU172_ReconcileArchivedLifecycleSetStepDetectsConcurrentStatusChange
// injects a same-item status write between UnarchiveItem and the
// reconciliation set step (inside the set step's B critical section, before
// its read) and asserts the compare-and-swap rejects the stale write instead
// of clobbering the concurrent writer or re-archiving over it (172.003-T).
func TestU172_ReconcileArchivedLifecycleSetStepDetectsConcurrentStatusChange(t *testing.T) {
	ctx := context.Background()
	ws := setupShipmentWorkspace(t)
	item, err := CreateArtifact(ctx, ws, "reconcile set-step cas", "feature")
	require.NoError(t, err)
	_, err = ArchiveItem(ctx, ws.DB, ws, item.ID)
	require.NoError(t, err)

	archived := readArtifactForTest(t, ctx, ws, item.ID)
	require.Equal(t, models.StatusArchived, archived.Status)
	archivedStatus := archived.ArchivedStatus
	require.NotEmpty(t, archivedStatus)
	const injectedStatus = "blocked"
	require.NotEqual(t, injectedStatus, archivedStatus)

	queueDir := filepath.Join(workspaceStorageRoot(ws), "queue")
	var (
		injectOnce sync.Once
		injected   bool
		injectErr  error
	)
	previousHook := artifactMutationLockBarrierHook
	artifactMutationLockBarrierHook = func(artifactID string) {
		if artifactID != item.ID {
			return
		}
		path, findErr := FindArtifactPath(ctx, ws, item.ID)
		if findErr != nil || filepath.Clean(filepath.Dir(path)) != filepath.Clean(queueDir) {
			// UnarchiveItem's own B acquisition fires while the file is still archived.
			return
		}
		injectOnce.Do(func() {
			raw, readErr := os.ReadFile(path)
			if readErr != nil {
				injectErr = readErr
				return
			}
			fm, body, parseErr := models.ParseFrontmatter(string(raw))
			if parseErr != nil {
				injectErr = parseErr
				return
			}
			fm["status"] = injectedStatus
			injectErr = os.WriteFile(path, []byte(models.SerializeFrontmatter(fm, body)), 0o644)
			injected = injectErr == nil
		})
	}
	t.Cleanup(func() { artifactMutationLockBarrierHook = previousHook })

	result, err := ReconcileArchivedLifecycle(ctx, ws.DB, ws, ReconciliationRequest{
		ItemIDs:      []string{item.ID},
		TargetStatus: "done",
		Reason:       "set-step cas",
		Actor:        "test-actor",
	})

	require.NoError(t, injectErr)
	require.True(t, injected, "barrier hook never observed the unarchived item at the set step")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, blerrors.ErrShipmentConflict), "expected ErrShipmentConflict, got %v", err)

	path, err := FindArtifactPath(ctx, ws, item.ID)
	require.NoError(t, err)
	assert.Equal(t, filepath.Clean(queueDir), filepath.Clean(filepath.Dir(path)),
		"conflicting reconcile must not re-archive over the concurrent writer")
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	fm, _, err := models.ParseFrontmatter(string(raw))
	require.NoError(t, err)
	assert.Equal(t, injectedStatus, fm["status"], "concurrent writer's status must survive")
	if cf, ok := fm["custom_fields"].(map[string]any); ok {
		assert.NotContains(t, cf, "reconciled_at", "stale reconciliation metadata must not be written")
	}
	assert.Contains(t, readU172ItemLogEventTypes(t, ws, item.ID), "lifecycle_reconciliation_conflict",
		"a CAS refusal must leave a durable conflict audit event")
}

// TestU172_ReconcileArchivedLifecycleUnarchiveStepDetectsConcurrentArchivedStatusChange
// injects an archived_status write inside UnarchiveItem's own B critical
// section (after reconciliation's unlocked read) and asserts the unarchive
// compare-and-swap refuses, leaving the archived item exactly as the
// concurrent writer left it (172.003-T).
func TestU172_ReconcileArchivedLifecycleUnarchiveStepDetectsConcurrentArchivedStatusChange(t *testing.T) {
	ctx := context.Background()
	ws := setupShipmentWorkspace(t)
	item, err := CreateArtifact(ctx, ws, "reconcile unarchive-step cas", "feature")
	require.NoError(t, err)
	_, err = ArchiveItem(ctx, ws.DB, ws, item.ID)
	require.NoError(t, err)

	archived := readArtifactForTest(t, ctx, ws, item.ID)
	require.Equal(t, models.StatusArchived, archived.Status)
	const injectedArchivedStatus = "blocked"
	require.NotEqual(t, injectedArchivedStatus, archived.ArchivedStatus)

	archiveDir := filepath.Join(workspaceStorageRoot(ws), "archive")
	var (
		injectOnce sync.Once
		injected   bool
		injectErr  error
	)
	previousHook := artifactMutationLockBarrierHook
	artifactMutationLockBarrierHook = func(artifactID string) {
		if artifactID != item.ID {
			return
		}
		path, findErr := FindArtifactPath(ctx, ws, item.ID)
		if findErr != nil || filepath.Clean(filepath.Dir(path)) != filepath.Clean(archiveDir) {
			return
		}
		injectOnce.Do(func() {
			raw, readErr := os.ReadFile(path)
			if readErr != nil {
				injectErr = readErr
				return
			}
			fm, body, parseErr := models.ParseFrontmatter(string(raw))
			if parseErr != nil {
				injectErr = parseErr
				return
			}
			fm["archived_status"] = injectedArchivedStatus
			injectErr = os.WriteFile(path, []byte(models.SerializeFrontmatter(fm, body)), 0o644)
			injected = injectErr == nil
		})
	}
	t.Cleanup(func() { artifactMutationLockBarrierHook = previousHook })

	result, err := ReconcileArchivedLifecycle(ctx, ws.DB, ws, ReconciliationRequest{
		ItemIDs:      []string{item.ID},
		TargetStatus: "done",
		Reason:       "unarchive-step cas",
		Actor:        "test-actor",
	})

	require.NoError(t, injectErr)
	require.True(t, injected, "barrier hook never observed the archived item at the unarchive step")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, blerrors.ErrShipmentConflict), "expected ErrShipmentConflict, got %v", err)

	path, err := FindArtifactPath(ctx, ws, item.ID)
	require.NoError(t, err)
	assert.Equal(t, filepath.Clean(archiveDir), filepath.Clean(filepath.Dir(path)),
		"refused unarchive must leave the item in the archive")
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	fm, _, err := models.ParseFrontmatter(string(raw))
	require.NoError(t, err)
	assert.Equal(t, string(models.StatusArchived), fm["status"])
	assert.Equal(t, injectedArchivedStatus, fm["archived_status"], "concurrent writer's archived_status must survive")
	assert.Contains(t, readU172ItemLogEventTypes(t, ws, item.ID), "lifecycle_reconciliation_conflict",
		"a CAS refusal must leave a durable conflict audit event")
}

// TestU172_UnarchiveExpectingRefusesVanishedItemAsConflict pins that the
// unarchive compare-and-swap reports an item that disappeared between the
// unlocked read and B as a conflict while keeping ErrNotFound in the chain,
// so reconciliation records a conflict audit event (172.003-T).
func TestU172_UnarchiveExpectingRefusesVanishedItemAsConflict(t *testing.T) {
	ctx := context.Background()
	ws := setupShipmentWorkspace(t)
	item, err := CreateArtifact(ctx, ws, "unarchive vanished", "feature")
	require.NoError(t, err)
	_, err = ArchiveItem(ctx, ws.DB, ws, item.ID)
	require.NoError(t, err)
	archived := readArtifactForTest(t, ctx, ws, item.ID)

	previousHook := artifactMutationLockBarrierHook
	var removeErr error
	artifactMutationLockBarrierHook = func(artifactID string) {
		if artifactID != item.ID {
			return
		}
		path, findErr := FindArtifactPath(ctx, ws, item.ID)
		if findErr != nil {
			return
		}
		removeErr = os.Remove(path)
	}
	t.Cleanup(func() { artifactMutationLockBarrierHook = previousHook })

	err = unarchiveItemExpecting(ctx, ws.DB, ws, item.ID, archived.ArchivedStatus)
	require.NoError(t, removeErr)
	require.Error(t, err)
	assert.True(t, errors.Is(err, blerrors.ErrShipmentConflict), "expected ErrShipmentConflict, got %v", err)
	assert.True(t, errors.Is(err, blerrors.ErrNotFound), "expected ErrNotFound to stay in the chain, got %v", err)

	t.Run("plain unarchive keeps lookup error", func(t *testing.T) {
		err := UnarchiveItem(ctx, ws.DB, ws, "998-F")
		require.Error(t, err)
		assert.False(t, errors.Is(err, blerrors.ErrShipmentConflict), "plain unarchive must not report a conflict, got %v", err)
	})
}

// TestU172_SetItemStatusAndMetaRefusesMissingOrReArchivedItem pins the
// set-step compare-and-swap's existence and location guards (172.003-T).
func TestU172_SetItemStatusAndMetaRefusesMissingOrReArchivedItem(t *testing.T) {
	ctx := context.Background()
	ws := setupShipmentWorkspace(t)

	t.Run("not found", func(t *testing.T) {
		err := setItemStatusAndMeta(ctx, ws.DB, ws, "999-F", "queued", "done", map[string]any{})
		require.Error(t, err)
		assert.True(t, errors.Is(err, blerrors.ErrShipmentConflict), "expected ErrShipmentConflict, got %v", err)
		assert.True(t, errors.Is(err, blerrors.ErrNotFound), "expected ErrNotFound to stay in the chain, got %v", err)
	})

	t.Run("re-archived", func(t *testing.T) {
		item, err := CreateArtifact(ctx, ws, "set-step re-archived", "feature")
		require.NoError(t, err)
		_, err = ArchiveItem(ctx, ws.DB, ws, item.ID)
		require.NoError(t, err)
		before := readArtifactForTest(t, ctx, ws, item.ID)

		err = setItemStatusAndMeta(ctx, ws.DB, ws, item.ID, string(models.StatusArchived), "done", map[string]any{"reconciled_at": "x"})
		require.Error(t, err)
		assert.True(t, errors.Is(err, blerrors.ErrShipmentConflict), "expected ErrShipmentConflict, got %v", err)

		after := readArtifactForTest(t, ctx, ws, item.ID)
		assert.Equal(t, before.Status, after.Status)
		assert.Equal(t, before.ArchivedStatus, after.ArchivedStatus)
	})
}
