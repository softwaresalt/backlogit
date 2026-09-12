package core

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// 167.019-T behavior harness: snapshot-before-lock artifact writers
// (AssociateCommit, AddDependency, AddArtifactLink — the task's own
// representative set) must not clobber a concurrent archived_status change
// with their own stale pre-lock snapshot.
//
// Contention is reproduced DETERMINISTICALLY (never via a sleep-based
// timing race): persistArtifactPreLockHook fires synchronously, in the same
// goroutine, immediately before persistArtifactWithLinkPolicyAndGuard
// acquires the artifact-mutation lock (B) — i.e. AFTER the writer under
// test has already done its own pre-lock findArtifact read, but BEFORE it
// can reach its own lock/guard step. The hook performs the "concurrent
// writer already committed" mutation right there (via a nested, safe
// persistArtifact call — lock B is not yet held at that point), so by the
// time the outer call proceeds to acquire lock B and run its guard, the
// on-disk archived_status has already diverged from what the writer
// captured before this hook ever ran.

// withConcurrentArchivedStatusMutation arranges for mutate to run exactly
// once, synchronously, the first time persistArtifactPreLockHook fires for
// itemID, then runs fn and returns its error. The hook is always cleared
// afterward.
func withConcurrentArchivedStatusMutation(t *testing.T, itemID string, mutate func(), fn func() error) error {
	t.Helper()
	fired := false
	persistArtifactPreLockHook = func(id string) {
		if id != itemID || fired {
			return
		}
		fired = true
		mutate()
	}
	t.Cleanup(func() { persistArtifactPreLockHook = nil })
	return fn()
}

func archiveArtifact(t *testing.T, ws *Workspace, id, archivedStatus string) {
	t.Helper()
	current, err := findArtifact(context.Background(), ws, id)
	require.NoError(t, err)
	current.ArchivedFrom = string(current.Status)
	current.Status = models.StatusArchived
	current.ArchivedStatus = archivedStatus
	require.NoError(t, persistArtifact(context.Background(), ws, current, false))
}

// TestGuardArchivedStatusUnchangedSince_UnitBehavior exercises the shared
// guard directly for both the reject and pass-through paths.
func TestGuardArchivedStatusUnchangedSince_UnitBehavior(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feature, err := CreateArtifact(ctx, ws, "167.019-T guard feature", "feature")
	require.NoError(t, err)

	t.Run("rejects_when_changed", func(t *testing.T) {
		archiveArtifact(t, ws, feature.ID, "shipped")

		guard := guardArchivedStatusUnchangedSince(ws, feature.ID, "")
		guardErr := guard(ctx)
		require.Error(t, guardErr)
		assert.True(t, errors.Is(guardErr, blerrors.ErrShipmentConflict))
	})

	t.Run("passes_when_unchanged", func(t *testing.T) {
		current, findErr := findArtifact(ctx, ws, feature.ID)
		require.NoError(t, findErr)
		guard := guardArchivedStatusUnchangedSince(ws, feature.ID, current.ArchivedStatus)
		require.NoError(t, guard(ctx))
	})
}

func TestAssociateCommit_DoesNotClobberConcurrentArchivedStatusChange(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	shipment, err := CreateArtifact(ctx, ws, "167.019-T AssociateCommit shipment", "shipment")
	require.NoError(t, err)
	ew := NewWorkspaceEventWriter(ws, WorkspaceLogsRoot(ws.RootPath))

	fnErr := withConcurrentArchivedStatusMutation(t, shipment.ID, func() {
		archiveArtifact(t, ws, shipment.ID, "shipped")
	}, func() error {
		return AssociateCommit(ctx, ws, ew, shipment.ID, "deadbeef", "msg", "author")
	})

	require.Error(t, fnErr, "AssociateCommit must not silently clobber a concurrent archived_status change")
	assert.True(t, errors.Is(fnErr, blerrors.ErrShipmentConflict))

	final, findErr := findArtifact(ctx, ws, shipment.ID)
	require.NoError(t, findErr)
	assert.Equal(t, "shipped", final.ArchivedStatus, "the concurrent writer's archived_status must survive, never be clobbered")
	assert.Empty(t, final.Commit, "the stale writer's own field change must not have been applied either")
}

func TestAddDependency_DoesNotClobberConcurrentArchivedStatusChange(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	shipment, err := CreateArtifact(ctx, ws, "167.019-T AddDependency shipment", "shipment")
	require.NoError(t, err)
	other, err := CreateArtifact(ctx, ws, "167.019-T AddDependency other", "feature")
	require.NoError(t, err)

	fnErr := withConcurrentArchivedStatusMutation(t, shipment.ID, func() {
		archiveArtifact(t, ws, shipment.ID, "shipped")
	}, func() error {
		return AddDependency(ctx, ws, shipment.ID, other.ID, "blocks")
	})

	require.Error(t, fnErr, "AddDependency must not silently clobber a concurrent archived_status change")
	assert.True(t, errors.Is(fnErr, blerrors.ErrShipmentConflict))

	final, findErr := findArtifact(ctx, ws, shipment.ID)
	require.NoError(t, findErr)
	assert.Equal(t, "shipped", final.ArchivedStatus)
	assert.Empty(t, final.Dependencies, "the stale writer's dependency edge must not have been applied either")
}

func TestAddArtifactLink_DoesNotClobberConcurrentArchivedStatusChange(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	shipment, err := CreateArtifact(ctx, ws, "167.019-T AddArtifactLink shipment", "shipment")
	require.NoError(t, err)
	other, err := CreateArtifact(ctx, ws, "167.019-T AddArtifactLink other", "feature")
	require.NoError(t, err)

	fnErr := withConcurrentArchivedStatusMutation(t, shipment.ID, func() {
		archiveArtifact(t, ws, shipment.ID, "shipped")
	}, func() error {
		return AddArtifactLink(ctx, ws, shipment.ID, other.ID, "related_to")
	})

	require.Error(t, fnErr, "AddArtifactLink must not silently clobber a concurrent archived_status change")
	assert.True(t, errors.Is(fnErr, blerrors.ErrShipmentConflict))

	final, findErr := findArtifact(ctx, ws, shipment.ID)
	require.NoError(t, findErr)
	assert.Equal(t, "shipped", final.ArchivedStatus)
	assert.Empty(t, final.Links, "the stale writer's link must not have been applied either")
}

// TestAssociateCommit_NoConcurrentChangeSucceedsNormally is the control
// case: without a concurrent archived_status change, the guard must not
// interfere with the normal write path.
func TestAssociateCommit_NoConcurrentChangeSucceedsNormally(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feature, err := CreateArtifact(ctx, ws, "167.019-T control feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "167.019-T control task", "task", WithParent(feature.ID))
	require.NoError(t, err)
	ew := NewWorkspaceEventWriter(ws, WorkspaceLogsRoot(ws.RootPath))

	require.NoError(t, AssociateCommit(ctx, ws, ew, task.ID, "cafebabe", "msg", "author"))

	final, findErr := findArtifact(ctx, ws, task.ID)
	require.NoError(t, findErr)
	assert.Equal(t, "cafebabe", final.Commit)
}
