package core

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
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

	archiveDone := make(chan error, 1)
	go func() {
		_, archiveErr := ArchiveItem(ctx, ws.DB, ws, item.ID)
		archiveDone <- archiveErr
	}()

	select {
	case <-archiveAttemptedArtifactLock:
	case <-time.After(defaultGateLockBoundedWait):
		t.Fatal("ArchiveItem never attempted the artifact mutation lock")
	}
	waitForU172ArtifactStatus(t, ctx, ws, item.ID, models.StatusArchived)

	cBeforeBWriterDone := make(chan error, 1)
	go func() {
		updated, loadErr := findArtifact(cLockedCtx, ws, item.ID)
		if loadErr != nil {
			cBeforeBWriterDone <- loadErr
			return
		}
		updated.Title = "c-before-b writer completed"
		updated.UpdatedAt = models.NowUTC()
		cBeforeBWriterDone <- persistArtifact(cLockedCtx, ws, updated, false)
	}()

	var writerErr error
	writerCompleted := false
	select {
	case writerErr = <-cBeforeBWriterDone:
		writerCompleted = true
	case <-time.After(500 * time.Millisecond):
	}

	unlockItemLog()
	itemLogLocked = false

	select {
	case archiveErr := <-archiveDone:
		require.NoError(t, archiveErr)
	case <-time.After(defaultGateLockBoundedWait + 2*time.Second):
		t.Fatal("ArchiveItem did not finish after the item-log lock was released")
	}

	require.True(t, writerCompleted, "C-before-B writer blocked while ArchiveItem waited for the item-log lock; ArchiveItem still holds B while acquiring C")
	require.NoError(t, writerErr)

	require.Contains(t, readU172ItemLogEventTypes(t, ws, item.ID), "archived")
	updated := readArtifactForTest(t, ctx, ws, item.ID)
	require.Equal(t, "c-before-b writer completed", updated.Title)
}

func waitForU172ArtifactStatus(t *testing.T, ctx context.Context, ws *Workspace, itemID string, status models.ArtifactStatus) {
	t.Helper()
	deadline := time.Now().Add(defaultGateLockBoundedWait)
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
