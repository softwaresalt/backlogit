package core

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

// 200.007-T (CY-B B7) acceptance criterion 2: a request for the item-log lock C
// while B is held, for an item whose C is not context-held, fails closed with an
// error instead of acquiring C. A context-held C is a reuse and succeeds.
//
// The item-log barrier (events.SetItemLogLockBarrierHookForTest) fires
// immediately before a C file lock is acquired, so a zero count proves C was
// never acquired. This test installs a package-global hook and must not call
// t.Parallel.
func TestLockItemLogsSorted_FailsClosedWhenBHeldAndCNotContextHeld(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	item, err := CreateArtifact(ctx, ws, "fail closed item", "feature")
	require.NoError(t, err)

	var cAcquisitions atomic.Int32
	restore := events.SetItemLogLockBarrierHookForTest(func(string) { cAcquisitions.Add(1) })
	t.Cleanup(restore)

	t.Run("c_requested_under_b_without_context_held_c_is_refused", func(t *testing.T) {
		cAcquisitions.Store(0)
		bCtx, releaseB, err := lockArtifactMutations(ctx, ws, []string{item.ID})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, releaseB()) })

		_, release, err := lockItemLogsSorted(bCtx, ws, []string{item.ID})
		require.ErrorIs(t, err, errItemLogLockUnderArtifactLocks)
		require.Nil(t, release)
		require.Zero(t, cAcquisitions.Load(), "C must not be acquired while B is held")
	})

	t.Run("nested_archive_under_b_without_context_held_c_fails_closed", func(t *testing.T) {
		cAcquisitions.Store(0)
		bCtx, releaseB, err := lockArtifactMutations(ctx, ws, []string{item.ID})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, releaseB()) })

		_, archiveErr := ArchiveItem(bCtx, ws.DB, ws, item.ID)
		require.ErrorIs(t, archiveErr, errItemLogLockUnderArtifactLocks)
		require.Zero(t, cAcquisitions.Load(), "C must not be acquired while B is held")
		require.NotEqual(t, models.StatusArchived, readArtifactForTest(t, ctx, ws, item.ID).Status,
			"a refused archive must leave the item untouched")
	})

	t.Run("context_held_c_is_reused_under_b", func(t *testing.T) {
		cCtx, releaseC, err := lockItemLogsSorted(ctx, ws, []string{item.ID})
		require.NoError(t, err)
		t.Cleanup(releaseC)
		cAcquisitions.Store(0)

		bCtx, releaseB, err := lockArtifactMutations(cCtx, ws, []string{item.ID})
		require.NoError(t, err)
		t.Cleanup(func() { require.NoError(t, releaseB()) })

		_, reuseRelease, err := lockItemLogsSorted(bCtx, ws, []string{item.ID})
		require.NoError(t, err)
		reuseRelease()
		require.Zero(t, cAcquisitions.Load(), "a context-held C is reused, never re-acquired")
	})
}
