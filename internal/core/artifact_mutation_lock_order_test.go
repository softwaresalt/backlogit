package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

func TestU172_LockOrder_ArchiveItemDoesNotHoldArtifactLockWhileWaitingForItemLog(t *testing.T) {
	ctx := context.Background()
	ws := setupShipmentWorkspace(t)
	item, err := CreateArtifact(ctx, ws, "lock order archive item", "feature")
	require.NoError(t, err)

	logsDir := WorkspaceLogsRoot(ws.RootPath)
	cLockedCtx, unlockItemLog, err := events.LockItemLogCrossProcess(ctx, WorkspaceLocksRoot(ws.RootPath), logsDir, item.ID)
	require.NoError(t, err)
	itemLogLocked := true
	t.Cleanup(func() {
		if itemLogLocked {
			unlockItemLog()
		}
	})

	// 200.007-T supersedes the original 172.002-T observation order. ArchiveItem
	// now takes C for its archive set before B, so while the test holds C it must
	// be waiting for C holding no B, and must never reach B. The B hook is
	// therefore a negative probe: firing means ArchiveItem took B ahead of C.
	archiveReachedArtifactLock := make(chan struct{})
	previousHook := artifactMutationLockBarrierHook
	var hookOnce sync.Once
	artifactMutationLockBarrierHook = func(artifactID string) {
		if artifactID == item.ID {
			hookOnce.Do(func() { close(archiveReachedArtifactLock) })
		}
	}
	t.Cleanup(func() { artifactMutationLockBarrierHook = previousHook })

	// Installed after the test holds C and before ArchiveItem starts, so the
	// package-level hook is never written while another goroutine reads it.
	// The C hook fires only once a caller passes the in-process item-log mutex
	// this test holds, so it must stay silent until the test releases C. That
	// proves the writer below ran while ArchiveItem was still waiting for C.
	var archiveCAttempts atomic.Int32
	restoreItemLogHook := events.SetItemLogLockBarrierHookForTest(func(itemID string) {
		if itemID == item.ID {
			archiveCAttempts.Add(1)
		}
	})
	t.Cleanup(restoreItemLogHook)

	// The lifecycle hook reports when ArchiveItem reaches the item-log lock
	// step (after pending recovery and shipment discovery have completed), so
	// the negative probe and writer below run while ArchiveItem is actively
	// contending on C.
	archiveAttemptingItemLog := make(chan struct{})
	var itemLogAttemptOnce sync.Once
	archiveCtx := context.WithValue(ctx, shipmentLifecycleGlobalLockHookContextKey{}, func(phase string) {
		if phase == "attempt_item_log" {
			itemLogAttemptOnce.Do(func() { close(archiveAttemptingItemLog) })
		}
	})

	archiveDone := make(chan error, 1)
	archiveReturned := false
	go func() {
		_, archiveErr := ArchiveItem(archiveCtx, ws.DB, ws, item.ID)
		archiveDone <- archiveErr
	}()
	// Registered after the hook-restore cleanups so it runs before them: on an
	// early t.Fatal it releases C and drains ArchiveItem, so the goroutine never
	// reads a hook while a cleanup restores it and never outlives the workspace.
	t.Cleanup(func() {
		if archiveReturned {
			return
		}
		if itemLogLocked {
			unlockItemLog()
			itemLogLocked = false
		}
		select {
		case <-archiveDone:
		case <-time.After(u172ObservationWait):
			t.Errorf("ArchiveItem goroutine did not exit during cleanup")
		}
	})

	select {
	case <-archiveAttemptingItemLog:
	case <-time.After(u172ObservationWait):
		t.Fatal("ArchiveItem never reached the item-log lock step")
	}
	select {
	case <-archiveReachedArtifactLock:
		t.Fatal("ArchiveItem acquired the artifact mutation lock B while another writer held C: it must take C before B")
	case <-time.After(50 * time.Millisecond):
	}

	// The writer try-locks B, so a B held by ArchiveItem while it waits for C
	// surfaces as ErrTaskBusy until the deadline instead of a timing guess.
	writerErr := runU172CBeforeBWriter(cLockedCtx, ws, item.ID, u172ObservationWait)
	require.Zero(t, archiveCAttempts.Load(), "ArchiveItem acquired the item-log file lock while the test still held C")

	unlockItemLog()
	itemLogLocked = false

	select {
	case archiveErr := <-archiveDone:
		archiveReturned = true
		require.NoError(t, archiveErr)
	case <-time.After(u172ObservationWait):
		t.Fatal("ArchiveItem did not finish after the item-log lock was released")
	}

	require.NoErrorf(t, writerErr, "C-before-B writer could not acquire B while ArchiveItem waited for the item-log lock; ArchiveItem still holds B while acquiring C")
	require.Positive(t, archiveCAttempts.Load(), "ArchiveItem never attempted the item-log lock after the test released C")

	require.Contains(t, readU172ItemLogEventTypes(t, ws, item.ID), "archived")
	updated := readArtifactForTest(t, ctx, ws, item.ID)
	require.Equal(t, "c-before-b writer completed", updated.Title)
}

func runU172CBeforeBWriter(cLockedCtx context.Context, ws *Workspace, itemID string, wait time.Duration) error {
	deadline := time.Now().Add(wait)
	for {
		updated, err := findArtifact(cLockedCtx, ws, itemID)
		if err != nil {
			return err
		}
		updated.Title = "c-before-b writer completed"
		updated.UpdatedAt = models.NowUTC()
		err = persistArtifact(cLockedCtx, ws, updated, false)
		if err == nil || !errors.Is(err, ErrTaskBusy) || time.Now().After(deadline) {
			return err
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// u172ObservationWait bounds how long a test polls for an expected state. It is
// deliberately decoupled from defaultGateLockBoundedWait: nothing contends the
// locks while the test observes, so this only absorbs slow I/O on loaded hosts.
const u172ObservationWait = 30 * time.Second

func readU172ItemLogEventTypes(t *testing.T, ws *Workspace, itemID string) []string {
	t.Helper()
	data, err := os.ReadFile(events.LogPathForItem(WorkspaceLogsRoot(ws.RootPath), itemID))
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	types := make([]string, 0, len(lines))
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var event events.Event
		require.NoError(t, json.Unmarshal([]byte(line), &event))
		types = append(types, event.EventType)
	}
	return types
}

func TestU7_ArchiveGovernanceEmitsDeterministicMembershipAndItemLogAttemptStages(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feature, err := CreateArtifact(ctx, ws, "u7 archive governance feature", "feature")
	require.NoError(t, err)
	member, err := CreateArtifact(ctx, ws, "u7 archive governance member", "task", WithParent(feature.ID))
	require.NoError(t, err)
	_, err = UpdateArtifact(ctx, ws, member.ID, map[string]any{"status": string(models.StatusActive)})
	require.NoError(t, err)
	_, err = UpdateArtifact(ctx, ws, member.ID, map[string]any{"status": string(models.StatusDone)})
	require.NoError(t, err)
	_, err = CreateShipment(ctx, ws, "u7 archive governance shipment", []string{member.ID})
	require.NoError(t, err)

	logsDir := WorkspaceLogsRoot(ws.RootPath)
	var mu sync.Mutex
	var stages []string
	observedCtx := context.WithValue(ctx, shipmentLifecycleGlobalLockHookContextKey{}, func(stage string) {
		mu.Lock()
		stages = append(stages, stage)
		mu.Unlock()
		if stage == "attempt_item_log" {
			require.False(t, events.ItemLogCrossProcessLockHeld(ctx, logsDir, member.ID),
				"item-log lock C must not be held before attempt_item_log")
			unlockB, lockErr := lockArtifactMutation(ctx, ws, member.ID)
			require.NoError(t, lockErr, "artifact mutation lock B must not be held at attempt_item_log")
			require.NoError(t, unlockB())
		}
	})

	_, err = ArchiveItem(observedCtx, ws.DB, ws, member.ID)
	require.NoError(t, err)

	mu.Lock()
	recorded := append([]string(nil), stages...)
	mu.Unlock()

	acquiredIdx := slices.Index(recorded, "acquired")
	membershipIdx := slices.Index(recorded, "attempt_membership")
	itemLogIdx := slices.Index(recorded, "attempt_item_log")

	require.NotEqualf(t, -1, acquiredIdx, "expected 'acquired' stage in %v", recorded)
	require.NotEqualf(t, -1, membershipIdx, "expected 'attempt_membership' stage in %v", recorded)
	require.NotEqualf(t, -1, itemLogIdx, "expected 'attempt_item_log' stage in %v", recorded)
	require.Lessf(t, acquiredIdx, membershipIdx, "expected 'acquired' before 'attempt_membership' in %v", recorded)
	require.Lessf(t, membershipIdx, itemLogIdx, "expected 'attempt_membership' before 'attempt_item_log' in %v", recorded)
}
