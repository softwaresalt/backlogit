package core

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

// 200.009-T review follow-up: a nested ArchiveItem must not run the linked-stash
// side effect while it still holds the inherited item-log locks (C).
//
// ArchiveStashEntry waits for the process-wide stashMu. CorrectStashProvenance
// holds stashMu and then requests C for the canonical delivery artifact. If a
// cascaded child archived the linked stash entries while the parent's C was held,
// the two uncancellable waits interlocked: the archive held C and waited for
// stashMu, the correction held stashMu and waited for C.
//
// The test forces that interleaving. It plays the provenance correction by
// holding stashMu for real and then requesting C for the archived child with the
// same appender CorrectStashProvenance uses. A fixed ArchiveItem has released C
// before it touches stashMu, so the request completes while stashMu is still held.
// A regression leaves C held, the request never completes, and the test fails
// after the timeout instead of hanging. The test mutates the process-wide
// stashMu and must not call t.Parallel.
func TestArchiveItem_CascadeStashSideEffectRunsAfterItemLogLocksReleased(t *testing.T) {
	const wait = 10 * time.Second
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	parent, err := CreateArtifact(ctx, ws, "stash deferral parent", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, parent))
	child, err := CreateArtifact(ctx, ws, "stash deferral child", "task", WithParent(parent.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, child))
	entry, err := AddStashEntry(ctx, ws, "task", "medium", "stash entry linked to the cascaded child")
	require.NoError(t, err)
	require.NoError(t, bldb.LinkStashEntry(ctx, ws.DB, entry.ID, child.ID, time.Now().UTC()))

	// The correction side: stashMu is held before it requests C.
	stashMu.Lock()
	var unlockOnce sync.Once
	releaseStash := func() { unlockOnce.Do(stashMu.Unlock) }
	t.Cleanup(releaseStash)

	archiveDone := make(chan error, 1)
	go func() {
		_, archiveErr := ArchiveItem(ctx, ws.DB, ws, parent.ID, WithCascade(true))
		archiveDone <- archiveErr
	}()

	// Wait until the child's archive committed: it still holds C (a fix has not
	// reached its release yet, a regression is about to block on stashMu).
	require.Eventually(t, func() bool {
		item, loadErr := loadArtifact(ctx, ws, child.ID)
		return loadErr == nil && item.Status == models.StatusArchived
	}, wait, 5*time.Millisecond, "the cascaded child must be archived")

	correctionDone := make(chan error, 1)
	go func() {
		correctionDone <- appendItemEventErr(ctx, ws, child.ID, "stash_provenance_corrected",
			map[string]any{"stash_id": entry.ID})
	}()

	var correctionErr error
	select {
	case correctionErr = <-correctionDone:
	case <-time.After(wait):
		releaseStash() // unwind the blocked goroutines before failing
		<-archiveDone
		t.Fatal("deadlock: the cascaded archive held the item-log lock C while waiting for stashMu held by the provenance correction")
	}
	require.NoError(t, correctionErr)

	releaseStash()
	select {
	case archiveErr := <-archiveDone:
		require.NoError(t, archiveErr)
	case <-time.After(wait):
		t.Fatal("the archive did not finish after stashMu was released")
	}
	require.False(t, stashEntryIsActive(t, ws, entry.ID),
		"the deferred linked-stash side effect must still run once the locks are released")
}
