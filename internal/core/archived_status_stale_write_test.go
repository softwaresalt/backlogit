package core

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

func TestU172_RemoveArtifactLinkRejectsArchivedStatusStaleWrite(t *testing.T) {
	ctx := context.Background()
	ws := setupShipmentWorkspace(t)
	source, err := CreateArtifact(ctx, ws, "stale remove source", "feature")
	require.NoError(t, err)
	target, err := CreateArtifact(ctx, ws, "stale remove target", "feature")
	require.NoError(t, err)
	require.NoError(t, AddArtifactLink(ctx, ws, source.ID, target.ID, "related_to"))
	require.NoError(t, setArchivedStatusForTest(ctx, ws, source.ID, string(models.StatusActive)))

	installArchivedStatusReconcileHook(t, ctx, ws, source.ID)

	err = RemoveArtifactLink(ctx, ws, source.ID, target.ID, "related_to")

	assert.ErrorIs(t, err, blerrors.ErrShipmentConflict)
	updated := readArtifactForTest(t, ctx, ws, source.ID)
	assert.Equal(t, string(ShipmentShipped), updated.ArchivedStatus)
}

func TestU172_BulkUpdateStatusReportsArchivedStatusStaleWrite(t *testing.T) {
	ctx := context.Background()
	ws := setupShipmentWorkspace(t)
	feature, err := CreateArtifact(ctx, ws, "stale bulk feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "stale bulk task", "task", WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, setArchivedStatusForTest(ctx, ws, task.ID, string(models.StatusActive)))

	installArchivedStatusReconcileHook(t, ctx, ws, task.ID)

	result, err := BulkUpdateStatus(ctx, ws.DB, ws, []string{task.ID}, string(models.StatusActive))

	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 0, result.Succeeded)
	assert.Contains(t, result.Failed, task.ID)
	updated := readArtifactForTest(t, ctx, ws, task.ID)
	assert.Equal(t, string(ShipmentShipped), updated.ArchivedStatus)
}

func setArchivedStatusForTest(ctx context.Context, ws *Workspace, id, archivedStatus string) error {
	artifact, err := findArtifact(ctx, ws, id)
	if err != nil {
		return err
	}
	previousStatus := artifact.Status
	artifact.Status = models.StatusArchived
	artifact.ArchivedFrom = ".backlogit/queue/" + artifact.ID + ".md"
	artifact.ArchivedStatus = archivedStatus
	artifact.UpdatedAt = models.NowUTC()
	return persistArtifact(ctx, ws, artifact, shouldRelocateOnStatusChange(previousStatus, artifact.Status))
}

func installArchivedStatusReconcileHook(t *testing.T, ctx context.Context, ws *Workspace, id string) {
	t.Helper()
	previous := persistArtifactPreLockHook
	ran := false
	persistArtifactPreLockHook = func(artifactID string) {
		if artifactID != id || ran {
			return
		}
		ran = true
		err := setArchivedStatusForTest(ctx, ws, id, string(ShipmentShipped))
		require.NoError(t, err)
	}
	t.Cleanup(func() { persistArtifactPreLockHook = previous })
}

func readArtifactForTest(t *testing.T, ctx context.Context, ws *Workspace, id string) *models.Artifact {
	t.Helper()
	artifact, err := findArtifact(ctx, ws, id)
	if errors.Is(err, blerrors.ErrNotFound) {
		require.NoError(t, err, "artifact %s should still exist", id)
	}
	require.NoError(t, err)
	return artifact
}
