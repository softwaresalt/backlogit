package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
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

	archiveAttemptedArtifactLock := make(chan struct{})
	previousHook := artifactMutationLockBarrierHook
	var hookOnce sync.Once
	artifactMutationLockBarrierHook = func(artifactID string) {
		if artifactID == item.ID {
			hookOnce.Do(func() { close(archiveAttemptedArtifactLock) })
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

	archiveDone := make(chan error, 1)
	archiveReturned := false
	go func() {
		_, archiveErr := ArchiveItem(ctx, ws.DB, ws, item.ID)
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
	case <-archiveAttemptedArtifactLock:
	case <-time.After(u172ObservationWait):
		t.Fatal("ArchiveItem never attempted the artifact mutation lock")
	}
	waitForU172ArtifactStatus(t, ctx, ws, item.ID, models.StatusArchived)

	// The writer try-locks B, so a B held by ArchiveItem while it waits for C
	// surfaces as ErrTaskBusy until the deadline instead of a timing guess.
	// Retrying only absorbs the short window between the archived status
	// becoming visible and ArchiveItem releasing B.
	writerErr := runU172CBeforeBWriter(cLockedCtx, ws, item.ID, defaultGateLockBoundedWait)
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

func waitForU172ArtifactStatus(t *testing.T, ctx context.Context, ws *Workspace, itemID string, status models.ArtifactStatus) {
	t.Helper()
	deadline := time.Now().Add(u172ObservationWait)
	for time.Now().Before(deadline) {
		artifact, err := findArtifact(ctx, ws, itemID)
		if err == nil && artifact.Status == status {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	artifact := readArtifactForTest(t, ctx, ws, itemID)
	require.Equal(t, status, artifact.Status)
}

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
