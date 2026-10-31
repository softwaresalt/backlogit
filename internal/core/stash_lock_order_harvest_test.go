package core

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// runConcurrentWithWatchdog runs each fn in its own goroutine and fails the
// test when they have not all finished before the watchdog fires (deadlock).
func runConcurrentWithWatchdog(t *testing.T, fns ...func() error) []error {
	t.Helper()
	errs := make([]error, len(fns))
	var wg sync.WaitGroup
	for i, fn := range fns {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = fn()
		}()
	}
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatal("deadlock: concurrent stash-lock operations did not complete (lock-order inversion)")
	}
	return errs
}

func harvestFeatures(ctx context.Context, ws *Workspace, n int) error {
	for i := 0; i < n; i++ {
		entry, err := AddStashEntry(ctx, ws, "feature", "medium", "harvest candidate")
		if err != nil {
			return err
		}
		if _, err := HarvestStashEntry(ctx, ws, HarvestStashOptions{StashID: entry.ID, ArtifactType: "feature"}); err != nil {
			return err
		}
	}
	return nil
}

func TestLockOrder_HarvestConcurrentWithAdoptRenameDoesNotDeadlock(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feat1, err := CreateArtifact(ctx, ws, "Origin feature", "feature")
	require.NoError(t, err)
	feat2, err := CreateArtifact(ctx, ws, "Destination feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Task to adopt", "task", WithParent(feat1.ID))
	require.NoError(t, err)

	errs := runConcurrentWithWatchdog(t,
		func() error { return harvestFeatures(ctx, ws, 4) },
		func() error { _, adoptErr := AdoptItem(ctx, ws, task.ID, feat2.ID); return adoptErr },
	)
	for _, e := range errs {
		require.NoError(t, e)
	}
}

func TestLockOrder_HarvestConcurrentWithCorrectStashProvenanceDoesNotDeadlock(t *testing.T) {
	ws, _, _ := u2StashFixture(t)
	ctx := context.Background()

	errs := runConcurrentWithWatchdog(t,
		func() error { return harvestFeatures(ctx, ws, 4) },
		func() error { _, corrErr := CorrectStashProvenance(ctx, ws, u2ForceReq()); return corrErr },
	)
	for _, e := range errs {
		require.NoError(t, e)
	}
}
