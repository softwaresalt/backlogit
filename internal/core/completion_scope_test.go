package core

// 017.012-T: Completion-scope status reconciliation harness.
//
// These tests specify the behaviour of ReconcileCompletionScope, which
// enforces two invariants when work transitions to done:
//
//  1. Every queued or active descendant of a done parent is also marked done.
//  2. No item may remain done when it has descendants that are still queued
//     or active (a parent cannot be "done" ahead of its children without an
//     explicit reconciliation call).

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

// TestReconcileCompletionScope_MarksDoneChildrenWhenParentIsDone verifies
// that all queued or active children of a done parent are promoted to done.
func TestReconcileCompletionScope_MarksDoneChildrenWhenParentIsDone(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feature, err := CreateArtifact(ctx, ws, "Reconcile feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

	taskA, err := CreateArtifact(ctx, ws, "Task A", "task", WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, taskA))
	taskB, err := CreateArtifact(ctx, ws, "Task B", "task", WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, taskB))

	// Mark parent done but leave children queued — creating an inconsistency.
	feature.Status = models.StatusDone
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

	// Act: reconcile from the feature root.
	err = ReconcileCompletionScope(ctx, ws, feature.ID)

	// Assert: no error returned.
	require.NoError(t, err)

	// Assert: children are now done.
	dbA, err := bldb.GetItem(ctx, ws.DB, taskA.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusDone, dbA.Status, "Task A must be done after reconciliation")

	dbB, err := bldb.GetItem(ctx, ws.DB, taskB.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusDone, dbB.Status, "Task B must be done after reconciliation")
}

// TestReconcileCompletionScope_ChildrenAlreadyDoneIsNoOp verifies that
// reconciling a fully-done hierarchy produces no errors and leaves all items
// in their done state.
func TestReconcileCompletionScope_ChildrenAlreadyDoneIsNoOp(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feature, err := CreateArtifact(ctx, ws, "Complete feature", "feature")
	require.NoError(t, err)
	feature.Status = models.StatusDone
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

	task, err := CreateArtifact(ctx, ws, "Complete task", "task", WithParent(feature.ID))
	require.NoError(t, err)
	task.Status = models.StatusDone
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))

	err = ReconcileCompletionScope(ctx, ws, feature.ID)

	require.NoError(t, err)

	dbTask, _ := bldb.GetItem(ctx, ws.DB, task.ID)
	assert.Equal(t, models.StatusDone, dbTask.Status, "already-done task must remain done")
}

// TestReconcileCompletionScope_ErrorsWhenParentIsNotDone verifies that the
// function returns an error when called on a parent that is not yet done,
// preventing premature child promotion.
func TestReconcileCompletionScope_ErrorsWhenParentIsNotDone(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feature, err := CreateArtifact(ctx, ws, "In-progress feature", "feature")
	require.NoError(t, err)
	feature.Status = models.StatusActive
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

	err = ReconcileCompletionScope(ctx, ws, feature.ID)

	require.Error(t, err, "reconciling an active parent must return an error")
	assert.Contains(t, err.Error(), feature.ID)
}

// TestReconcileCompletionScope_IsRecursive verifies that reconciliation
// descends through nested child levels (subtasks under tasks under feature).
func TestReconcileCompletionScope_IsRecursive(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feature, err := CreateArtifact(ctx, ws, "Deep feature", "feature")
	require.NoError(t, err)
	feature.Status = models.StatusDone
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

	task, err := CreateArtifact(ctx, ws, "Deep task", "task", WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))

	subtask, err := CreateArtifact(ctx, ws, "Deep subtask", "subtask", WithParent(task.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, subtask))

	err = ReconcileCompletionScope(ctx, ws, feature.ID)
	require.NoError(t, err)

	dbTask, _ := bldb.GetItem(ctx, ws.DB, task.ID)
	assert.Equal(t, models.StatusDone, dbTask.Status, "nested task must be done")

	dbSub, _ := bldb.GetItem(ctx, ws.DB, subtask.ID)
	assert.Equal(t, models.StatusDone, dbSub.Status, "nested subtask must be done")
}

// TestReconcileCompletionScope_LeafItemWithNoChildrenSucceeds verifies that
// calling reconcile on a leaf artifact (no children) that is done is a no-op
// and does not error.
func TestReconcileCompletionScope_LeafItemWithNoChildrenSucceeds(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "Leaf feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feat))
	task, err := CreateArtifact(ctx, ws, "Leaf task", "task", WithParent(feat.ID))
	require.NoError(t, err)
	task.Status = models.StatusDone
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))

	err = ReconcileCompletionScope(ctx, ws, task.ID)
	require.NoError(t, err)
}

// TestReconcileCompletionScope_MissingItemReturnsNotFound verifies that
// calling reconcile with a non-existent artifact ID returns an error.
func TestReconcileCompletionScope_MissingItemReturnsNotFound(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	err := ReconcileCompletionScope(ctx, ws, "DOES-NOT-EXIST")
	require.Error(t, err)
}
