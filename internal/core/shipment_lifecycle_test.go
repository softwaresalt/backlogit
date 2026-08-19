package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// 143.012-T (Unit 12): colocated regressions for honest compensation and the
// ShipShipment defer-registration swap. Both were written and observed failing
// before their corresponding change.
//
// MUST NOT call t.Parallel(): these tests override the package globals
// persistArtifactWriteFn.

// Regression 1: the defer ordering is DISCRIMINATING. A test that only asserts
// "a post-closure archiveItems failure still restores the non-member covering
// feature" passes identically before and after the swap and is therefore not a
// regression test for this change. This test instead observes, at the moment
// the fallback performs its restore write, whether the artifact mutation lock
// is STILL HELD. Before the swap LIFO released the locks first, so the probe
// would have acquired the lock; after the swap the probe must be refused.
func TestShipShipment_NonMemberFallbackRunsBeforeArtifactLockRelease(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feature, err := CreateArtifact(ctx, ws, "Defer order covering feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

	taskOne, err := CreateArtifact(ctx, ws, "Defer order task one", "task", WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, taskOne))

	taskTwo, err := CreateArtifact(ctx, ws, "Defer order task two", "task", WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, taskTwo))

	shipment, err := CreateShipment(ctx, ws, "Defer order shipment", []string{taskOne.ID, taskTwo.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	// Fail a POST-CLOSURE step so the deferred fallback (not the in-line
	// success-path call, and not the in-closure rollback) is the code path that
	// performs the restore. attachCommitToItems runs after the locked closure
	// returns and re-persists each artifact with the commit stamped, which makes
	// its writes uniquely identifiable.
	const closingSHA = "deadbeefdeadbeefdeadbeefdeadbeefdeadbeef"
	commitFailure := errors.New("injected post-closure commit association failure")

	preShipFeature, err := loadArtifact(ctx, ws, feature.ID)
	require.NoError(t, err)
	preShipStatus := preShipFeature.Status

	origFn := persistArtifactWriteFn
	fallbackObservedLockHeld := -1
	persistArtifactWriteFn = func(a *models.Artifact, filePath string, durable bool) error {
		if a.Commit == closingSHA {
			return commitFailure
		}
		// The restore write is uniquely identifiable: it is the only write that
		// puts the covering feature BACK to its pre-ship status.
		if a.ID == feature.ID && a.Status == preShipStatus && fallbackObservedLockHeld < 0 {
			// Probe with a FRESH context so the reentrancy short-circuit in
			// lockArtifactMutation (which keys on ctx ownership markers) cannot
			// mask a genuinely released lock.
			release, lockErr := lockArtifactMutation(context.Background(), ws, feature.ID)
			if lockErr != nil {
				fallbackObservedLockHeld = 1
			} else {
				fallbackObservedLockHeld = 0
				_ = release()
			}
		}
		return origFn(a, filePath, durable)
	}
	t.Cleanup(func() { persistArtifactWriteFn = origFn })

	result, err := ShipShipment(ctx, ws, shipment.ID, &CommitMetadata{SHA: closingSHA, Message: "closing", Author: "ship"})
	require.Error(t, err, "the injected post-closure failure must abort the ship")
	assert.Nil(t, result)
	assert.ErrorIs(t, err, commitFailure)

	require.NotEqual(t, -1, fallbackObservedLockHeld,
		"the non-member fallback must have performed its restore write")
	assert.Equal(t, 1, fallbackObservedLockHeld,
		"the non-member fallback must run BEFORE releaseArtifactLocks, with the artifact lock still held")

	restoredFeature, findErr := findArtifact(ctx, ws, feature.ID)
	require.NoError(t, findErr)
	assert.NotEqual(t, models.StatusDone, restoredFeature.Status)
	assert.NotEqual(t, models.StatusArchived, restoredFeature.Status)
}

// Regression 2: every per-item failure source in the compensation loop yields
// partially-compensated with the offending ID named, and no rollback path
// returns early while an item-log lock is held.
func TestRestoreShipArtifactsDetailed_PromotesEveryPerItemFailure(t *testing.T) {
	cases := []struct {
		name string
		// arm returns the ID expected to be un-restorable after the fixture is
		// sabotaged.
		arm func(t *testing.T, ws *Workspace, logsDir, id string)
	}{
		{
			name: "lock_acquisition_failure",
			arm: func(t *testing.T, ws *Workspace, logsDir, id string) {
				plantLockSidecarDirectory(t, logsDir, id)
			},
		},
		{
			name: "artifact_file_restore_failure",
			arm: func(t *testing.T, ws *Workspace, logsDir, id string) {
				// Replace the artifact's restore target with a directory so
				// restoreSnapshot cannot write it back.
				path, err := FindArtifactPath(context.Background(), ws, id)
				require.NoError(t, err)
				require.NoError(t, os.Remove(path))
				require.NoError(t, os.MkdirAll(path, 0o755))
				t.Cleanup(func() { _ = os.RemoveAll(path) })
			},
		},
	}

	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			ws := setupShipmentWorkspace(t)
			ctx := context.Background()

			feature, err := CreateArtifact(ctx, ws, "Promotion covering feature", "feature")
			require.NoError(t, err)
			require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

			task, err := CreateArtifact(ctx, ws, "Promotion task", "task", WithParent(feature.ID))
			require.NoError(t, err)
			require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))

			snapshots, err := snapshotShipArtifacts(ctx, ws, []string{feature.ID, task.ID})
			require.NoError(t, err)

			logsDir := WorkspaceLogsRoot(ws.RootPath)
			testCase.arm(t, ws, logsDir, task.ID)

			unrestored, restoreErr := restoreShipArtifactsDetailed(ctx, ws, snapshots)
			require.Error(t, restoreErr, "the sabotaged item must surface a compensation failure")
			require.Contains(t, unrestored, task.ID,
				"the un-restorable item must be named, never silently skipped")
			assert.NotContains(t, unrestored, feature.ID,
				"an item that restored successfully must not be reported as un-restored")

			// No lock leak: a subsequent acquisition of the healthy item's log
			// lock must still succeed immediately.
			featureCtx, unlock, lockErr := acquireItemLogWithBudget(ctx, logsDir, feature.ID,
				&shipRestoreBudget{deadline: alreadyElapsedDeadline(), attempts: 0})
			require.NoError(t, lockErr, "the compensation loop must not leak the item-log mutex")
			require.NotNil(t, unlock)
			unlock()
			_ = featureCtx
		})
	}
}

// Regression 3: an un-restorable release-scope item promotes the governed
// shipped-event refusal to partially-compensated and names the ID, and the
// result is carried on the structured MutationPartialError.
func TestClassifyShippedEventAppendFailure_PromotesPartialCompensation(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feature, err := CreateArtifact(ctx, ws, "Partial promotion feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

	task, err := CreateArtifact(ctx, ws, "Partial promotion task", "task", WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))

	snapshots, err := snapshotShipArtifacts(ctx, ws, []string{feature.ID, task.ID})
	require.NoError(t, err)

	logsDir := WorkspaceLogsRoot(ws.RootPath)
	plantLockSidecarDirectory(t, logsDir, task.ID)

	appendErr := &shipmentEventAppendError{
		shipmentID: "001-S",
		cause:      fmt.Errorf("open item log: %w: %w", blerrors.ErrWriteNotApplied, errors.New("boom")),
	}
	outcome := classifyShippedEventAppendFailure(ctx, ws, "001-S", appendErr, snapshots, nil)

	var partial *blerrors.MutationPartialError
	require.ErrorAs(t, outcome.err, &partial)
	assert.Equal(t, "not-applied", partial.Class)
	assert.Equal(t, "partially-compensated", partial.CompensationState)
	assert.Equal(t, blerrors.StepShippedEventAppend, partial.FailedStep)
	assert.Contains(t, outcome.err.Error(), task.ID,
		"the un-restored ID must be named in the returned error")
	assert.True(t, outcome.rollbackAttempted)

	_ = filepath.Base(logsDir)
}

// alreadyElapsedDeadline returns a deadline in the past so the budget grants no
// retries, proving the healthy item's lock is free on the FIRST attempt.
func alreadyElapsedDeadline() time.Time {
	return time.Now().Add(-time.Second)
}
