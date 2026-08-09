package core

// U2 (130.002-T) regression tests: explicit ErrWriteIndeterminate reconciliation
// for AddDependency / RemoveDependency callers.
//
// AddDependency and RemoveDependency call persistArtifact(..., relocate=false).
// On the relocate=false path resolveArtifactPersistPaths returns (currentPath,
// currentPath, nil) immediately, so both the cross-directory source-dir fsync
// block (gated on srcDir != dstDir) and mkdirAllDurable are skipped. The only
// durable write on this path is WriteArtifactFileWithOptions, reached via
// persistArtifactWriteFn. Tests override this seam to inject
// ErrWriteIndeterminate without requiring a real fsync failure.
//
// Must not run with t.Parallel: tests swap the persistArtifactWriteFn package-global
// seam read on the production write path.

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// edgeExistsInternal is the white-box variant of edgeExists for internal tests.
func edgeExistsInternal(t *testing.T, ws *Workspace, itemID, dependsOn string) bool {
	t.Helper()
	edges, err := bldb.GetDependencies(context.Background(), ws.DB, itemID)
	require.NoError(t, err)
	for _, e := range edges {
		if e.DependsOn == dependsOn {
			return true
		}
	}
	return false
}

// TestPersistArtifact_PropagatesErrWriteIndeterminate is the precondition assertion:
// persistArtifact must propagate blerrors.ErrWriteIndeterminate when
// persistArtifactWriteFn returns it, so U2 reconciliation does not silently regress
// if that cross-file contract changes.
func TestPersistArtifact_PropagatesErrWriteIndeterminate(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "Feat", "feature")
	require.NoError(t, err)
	artifact, err := CreateArtifact(ctx, ws, "Source task", "task", WithParent(feat.ID))
	require.NoError(t, err)

	origFn := persistArtifactWriteFn
	persistArtifactWriteFn = func(a *models.Artifact, filePath string, durable bool) error {
		return fmt.Errorf("parent fsync failed: %w", blerrors.ErrWriteIndeterminate)
	}
	t.Cleanup(func() { persistArtifactWriteFn = origFn })

	err = persistArtifact(ctx, ws, artifact, false)
	require.Error(t, err)
	assert.True(t, blerrors.IsWriteIndeterminate(err),
		"persistArtifact must propagate ErrWriteIndeterminate from the write seam")
}

// TestAddDependency_IndeterminatePersist_EdgeNotRolledBack is the U2 regression for
// AddDependency: when the frontmatter persist returns ErrWriteIndeterminate (rename
// committed, parent fsync uncertain), the DB edge must NOT be rolled back — rolling it
// back would diverge the live index from the Markdown source of truth.
func TestAddDependency_IndeterminatePersist_EdgeNotRolledBack(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "Dep feat", "feature")
	require.NoError(t, err)
	source, err := CreateArtifact(ctx, ws, "Source", "task", WithParent(feat.ID))
	require.NoError(t, err)
	target, err := CreateArtifact(ctx, ws, "Target", "task", WithParent(feat.ID))
	require.NoError(t, err)

	// Insert the DB edge first so it exists before persistArtifact is reached.
	// (AddDependency inserts via db.AddDependencyChecked before calling persistArtifact.)
	origFn := persistArtifactWriteFn
	persistArtifactWriteFn = func(a *models.Artifact, filePath string, durable bool) error {
		return fmt.Errorf("parent fsync failed: %w", blerrors.ErrWriteIndeterminate)
	}
	t.Cleanup(func() { persistArtifactWriteFn = origFn })

	err = AddDependency(ctx, ws, source.ID, target.ID, "blocks")
	require.Error(t, err)
	assert.True(t, blerrors.IsWriteIndeterminate(err),
		"AddDependency must surface ErrWriteIndeterminate from indeterminate persist")

	// DB edge must NOT be rolled back: the MD write likely persisted.
	assert.True(t, edgeExistsInternal(t, ws, source.ID, target.ID),
		"DB edge must be retained when persist is indeterminate (not rolled back)")
}

// TestRemoveDependency_IndeterminatePersist_EdgeNotRolledBack is the U2 regression for
// RemoveDependency: when the frontmatter persist returns ErrWriteIndeterminate, the
// cache edge deletion must NOT be rolled back.
func TestRemoveDependency_IndeterminatePersist_EdgeNotRolledBack(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "Dep feat", "feature")
	require.NoError(t, err)
	source, err := CreateArtifact(ctx, ws, "Source", "task", WithParent(feat.ID))
	require.NoError(t, err)
	target, err := CreateArtifact(ctx, ws, "Target", "task", WithParent(feat.ID))
	require.NoError(t, err)

	// Establish the edge cleanly first.
	require.NoError(t, AddDependency(ctx, ws, source.ID, target.ID, "blocks"))
	require.True(t, edgeExistsInternal(t, ws, source.ID, target.ID))

	origFn := persistArtifactWriteFn
	persistArtifactWriteFn = func(a *models.Artifact, filePath string, durable bool) error {
		return fmt.Errorf("parent fsync failed: %w", blerrors.ErrWriteIndeterminate)
	}
	t.Cleanup(func() { persistArtifactWriteFn = origFn })

	err = RemoveDependency(ctx, ws, source.ID, target.ID)
	require.Error(t, err)
	assert.True(t, blerrors.IsWriteIndeterminate(err),
		"RemoveDependency must surface ErrWriteIndeterminate from indeterminate persist")

	// Edge was deleted from DB (RemoveDependency calls db.DeleteDependency first).
	// On indeterminate persist, the deletion must NOT be rolled back.
	assert.False(t, edgeExistsInternal(t, ws, source.ID, target.ID),
		"DB edge must remain deleted when persist is indeterminate (do not restore cache edge)")
}

// TestAddDependency_NotAppliedPersist_EdgeRolledBack guards the unchanged behavior:
// when the persist fails with ErrWriteNotApplied (file untouched), the DB edge must
// be rolled back to keep MD and the live index consistent.
func TestAddDependency_NotAppliedPersist_EdgeRolledBack(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "Dep feat", "feature")
	require.NoError(t, err)
	source, err := CreateArtifact(ctx, ws, "Source", "task", WithParent(feat.ID))
	require.NoError(t, err)
	target, err := CreateArtifact(ctx, ws, "Target", "task", WithParent(feat.ID))
	require.NoError(t, err)

	origFn := persistArtifactWriteFn
	persistArtifactWriteFn = func(a *models.Artifact, filePath string, durable bool) error {
		return fmt.Errorf("pre-rename write failed: %w", blerrors.ErrWriteNotApplied)
	}
	t.Cleanup(func() { persistArtifactWriteFn = origFn })

	err = AddDependency(ctx, ws, source.ID, target.ID, "blocks")
	require.Error(t, err)
	assert.False(t, blerrors.IsWriteIndeterminate(err), "not-applied error must not be classified as indeterminate")

	// DB edge must be rolled back: the file was untouched.
	assert.False(t, edgeExistsInternal(t, ws, source.ID, target.ID),
		"DB edge must be rolled back when persist is not-applied (file untouched)")
}

// TestAddDependency_IndeterminatePersist_ItemsRowReconciled verifies that on an
// indeterminate persist, db.UpsertItem is called to reconcile the stale
// items.dependencies column, so DB-fast-path mutations see the correct dep list.
func TestAddDependency_IndeterminatePersist_ItemsRowReconciled(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "Dep feat", "feature")
	require.NoError(t, err)
	source, err := CreateArtifact(ctx, ws, "Source", "task", WithParent(feat.ID))
	require.NoError(t, err)
	target, err := CreateArtifact(ctx, ws, "Target", "task", WithParent(feat.ID))
	require.NoError(t, err)

	origFn := persistArtifactWriteFn
	persistArtifactWriteFn = func(a *models.Artifact, filePath string, durable bool) error {
		return fmt.Errorf("parent fsync failed: %w", blerrors.ErrWriteIndeterminate)
	}
	t.Cleanup(func() { persistArtifactWriteFn = origFn })

	err = AddDependency(ctx, ws, source.ID, target.ID, "blocks")
	require.Error(t, err)
	assert.True(t, blerrors.IsWriteIndeterminate(err))

	// items.dependencies column must be reconciled via db.UpsertItem.
	row, getErr := bldb.GetItem(ctx, ws.DB, source.ID)
	require.NoError(t, getErr)
	func() {
		found := false
		for _, dep := range row.Dependencies {
			if dep.ID == target.ID {
				found = true
				break
			}
		}
		assert.True(t, found, "items.dependencies must be reconciled after indeterminate persist (UpsertItem called)")
	}()
}

// TestRemoveDependency_IndeterminatePersist_ItemsRowReconciled verifies that on an
// indeterminate persist, db.UpsertItem is called to reconcile the stale
// items.dependencies column so a subsequent Rehydrate does not restore the removed edge.
func TestRemoveDependency_IndeterminatePersist_ItemsRowReconciled(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "Dep feat", "feature")
	require.NoError(t, err)
	source, err := CreateArtifact(ctx, ws, "Source", "task", WithParent(feat.ID))
	require.NoError(t, err)
	target, err := CreateArtifact(ctx, ws, "Target", "task", WithParent(feat.ID))
	require.NoError(t, err)

	// Establish the edge cleanly first.
	require.NoError(t, AddDependency(ctx, ws, source.ID, target.ID, "blocks"))

	origFn := persistArtifactWriteFn
	persistArtifactWriteFn = func(a *models.Artifact, filePath string, durable bool) error {
		return fmt.Errorf("parent fsync failed: %w", blerrors.ErrWriteIndeterminate)
	}
	t.Cleanup(func() { persistArtifactWriteFn = origFn })

	err = RemoveDependency(ctx, ws, source.ID, target.ID)
	require.Error(t, err)
	assert.True(t, blerrors.IsWriteIndeterminate(err))

	// items.dependencies must not contain the removed dep after reconciliation.
	row, getErr := bldb.GetItem(ctx, ws.DB, source.ID)
	require.NoError(t, getErr)
	func() {
		found := false
		for _, dep := range row.Dependencies {
			if dep.ID == target.ID {
				found = true
				break
			}
		}
		assert.False(t, found, "items.dependencies must be reconciled after indeterminate persist (dep removed)")
	}()
}
