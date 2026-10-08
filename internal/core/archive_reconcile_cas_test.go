package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// TestU172_ReconcileArchivedLifecycleSetStepDetectsConcurrentStatusChange
// injects a same-item status write between UnarchiveItem and the
// reconciliation set step (inside the set step's B critical section, before
// its read) and asserts the compare-and-swap rejects the stale write instead
// of clobbering the concurrent writer or re-archiving over it (172.003-T).
func TestU172_ReconcileArchivedLifecycleSetStepDetectsConcurrentStatusChange(t *testing.T) {
	ctx := context.Background()
	ws := setupShipmentWorkspace(t)
	item, err := CreateArtifact(ctx, ws, "reconcile set-step cas", "feature")
	require.NoError(t, err)
	_, err = ArchiveItem(ctx, ws.DB, ws, item.ID)
	require.NoError(t, err)

	archived := readArtifactForTest(t, ctx, ws, item.ID)
	require.Equal(t, models.StatusArchived, archived.Status)
	archivedStatus := archived.ArchivedStatus
	require.NotEmpty(t, archivedStatus)
	const injectedStatus = "blocked"
	require.NotEqual(t, injectedStatus, archivedStatus)

	queueDir := filepath.Join(workspaceStorageRoot(ws), "queue")
	var (
		injectOnce sync.Once
		injected   bool
		injectErr  error
	)
	previousHook := artifactMutationLockBarrierHook
	artifactMutationLockBarrierHook = func(artifactID string) {
		if artifactID != item.ID {
			return
		}
		path, findErr := FindArtifactPath(ctx, ws, item.ID)
		if findErr != nil || filepath.Clean(filepath.Dir(path)) != filepath.Clean(queueDir) {
			// UnarchiveItem's own B acquisition fires while the file is still archived.
			return
		}
		injectOnce.Do(func() {
			raw, readErr := os.ReadFile(path)
			if readErr != nil {
				injectErr = readErr
				return
			}
			fm, body, parseErr := models.ParseFrontmatter(string(raw))
			if parseErr != nil {
				injectErr = parseErr
				return
			}
			fm["status"] = injectedStatus
			injectErr = os.WriteFile(path, []byte(models.SerializeFrontmatter(fm, body)), 0o644)
			injected = injectErr == nil
		})
	}
	t.Cleanup(func() { artifactMutationLockBarrierHook = previousHook })

	result, err := ReconcileArchivedLifecycle(ctx, ws.DB, ws, ReconciliationRequest{
		ItemIDs:      []string{item.ID},
		TargetStatus: "done",
		Reason:       "set-step cas",
		Actor:        "test-actor",
	})

	require.NoError(t, injectErr)
	require.True(t, injected, "barrier hook never observed the unarchived item at the set step")
	require.Error(t, err)
	assert.Nil(t, result)
	assert.True(t, errors.Is(err, blerrors.ErrShipmentConflict), "expected ErrShipmentConflict, got %v", err)

	path, err := FindArtifactPath(ctx, ws, item.ID)
	require.NoError(t, err)
	assert.Equal(t, filepath.Clean(queueDir), filepath.Clean(filepath.Dir(path)),
		"conflicting reconcile must not re-archive over the concurrent writer")
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	fm, _, err := models.ParseFrontmatter(string(raw))
	require.NoError(t, err)
	assert.Equal(t, injectedStatus, fm["status"], "concurrent writer's status must survive")
	if cf, ok := fm["custom_fields"].(map[string]any); ok {
		assert.NotContains(t, cf, "reconciled_at", "stale reconciliation metadata must not be written")
	}
}
