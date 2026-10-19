package core

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/hooks"
	"github.com/softwaresalt/backlogit/internal/models"
)

// 200.008-T (CY-B B8): post-archive stash side effects are compare-and-swap
// guarded.
//
// ArchiveItem releases its governance locks and then runs post-archive hooks and
// archives the stash entries linked to the item. Those side effects used to run
// unconditionally, so a writer that changed the item between the archive commit
// and the stash state update left a stale side effect: the item's stash entries
// were archived although the item was no longer in the archived state the side
// effect was derived from.
//
// The concurrent writer is injected with the public post-archive hook point
// (hooks.HookArchiveItem, hooks.PhasePost). It runs after the archive commit and
// before the stash side effect, which is exactly the window under test, and it
// uses the real UnarchiveItem writer. No production hook is added.

func newStashLinkedArchiveFixture(t *testing.T) (ws *Workspace, itemID, stashID string) {
	t.Helper()
	ws = setupShipmentWorkspace(t)
	ctx := context.Background()
	item, err := CreateArtifact(ctx, ws, "archive side effect cas", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, item))
	entry, err := AddStashEntry(ctx, ws, "task", "medium", "stash entry linked to the archived item")
	require.NoError(t, err)
	require.NoError(t, bldb.LinkStashEntry(ctx, ws.DB, entry.ID, item.ID, time.Now().UTC()))
	return ws, item.ID, entry.ID
}

func stashEntryIsActive(t *testing.T, ws *Workspace, stashID string) bool {
	t.Helper()
	entries, err := readStashEntries(StashFilePath(ws.RootPath))
	require.NoError(t, err)
	for _, entry := range entries {
		if entry.ID == stashID {
			return true
		}
	}
	return false
}

func TestArchiveItem_PostArchiveStashSideEffectIsCASGuarded(t *testing.T) {
	t.Run("without_concurrent_writer_linked_stash_is_archived", func(t *testing.T) {
		ws, itemID, stashID := newStashLinkedArchiveFixture(t)

		_, err := ArchiveItem(context.Background(), ws.DB, ws, itemID)
		require.NoError(t, err)

		require.False(t, stashEntryIsActive(t, ws, stashID),
			"the linked stash entry must still be archived when nothing changed the item")
	})

	t.Run("concurrent_unarchive_between_commit_and_side_effect_refuses_stale_stash_archive", func(t *testing.T) {
		ws, itemID, stashID := newStashLinkedArchiveFixture(t)
		ws.HookRunner = hooks.NewHookRunner()
		var writerErr error
		ws.HookRunner.Register(hooks.HookArchiveItem, hooks.PhasePost, hooks.HookRegistration{
			Name:     "b8_concurrent_writer",
			Priority: 100,
			Fn: func(hookCtx context.Context, _ hooks.HookContext) error {
				// A concurrent writer restores the item after the archive committed.
				writerErr = UnarchiveItem(hookCtx, ws.DB, ws, itemID)
				return nil
			},
		})

		_, err := ArchiveItem(context.Background(), ws.DB, ws, itemID)
		require.NoError(t, err, "the archive itself committed; only the stale side effect is refused")
		require.NoError(t, writerErr, "the concurrent writer must restore the item")

		restored := readArtifactForTest(t, context.Background(), ws, itemID)
		require.NotEqual(t, models.StatusArchived, restored.Status, "the item must be back out of the archive")
		require.True(t, stashEntryIsActive(t, ws, stashID),
			"the stale post-archive side effect must be refused: the item is no longer in the archived state the side effect was derived from")
	})
}
