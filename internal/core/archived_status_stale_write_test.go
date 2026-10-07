package core

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/db"
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

func TestU172_RemoveArtifactLinkGuardTreatsMissingArtifactAsConflict(t *testing.T) {
	ctx := context.Background()
	ws := setupShipmentWorkspace(t)
	source, err := CreateArtifact(ctx, ws, "missing guard source", "feature")
	require.NoError(t, err)
	require.NoError(t, setArchivedStatusForTest(ctx, ws, source.ID, string(models.StatusActive)))
	path, err := FindArtifactPath(ctx, ws, source.ID)
	require.NoError(t, err)
	require.NoError(t, os.Remove(path))

	err = guardArchivedStatusUnchangedSince(ws, source.ID, string(models.StatusActive))(ctx)

	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrShipmentConflict)
}

func TestU172_RemoveArtifactLinkPreservesDBOnlyPolicyAndNoOpBranch(t *testing.T) {
	ctx := context.Background()

	t.Run("guarded path strips database-only links", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		source, err := CreateArtifact(ctx, ws, "db-only strip source", "feature")
		require.NoError(t, err)
		explicitTarget, err := CreateArtifact(ctx, ws, "explicit target", "feature")
		require.NoError(t, err)
		dbOnlyTarget, err := CreateArtifact(ctx, ws, "db-only target", "feature")
		require.NoError(t, err)
		require.NoError(t, AddArtifactLink(ctx, ws, source.ID, explicitTarget.ID, "related_to"))
		require.NoError(t, db.AddLink(ctx, ws.DB, source.ID, dbOnlyTarget.ID, "informs"))

		require.NoError(t, RemoveArtifactLink(ctx, ws, source.ID, explicitTarget.ID, "related_to"))

		path, err := FindArtifactPath(ctx, ws, source.ID)
		require.NoError(t, err)
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		assert.False(t, strings.Contains(string(data), dbOnlyTarget.ID), "database-only link must not be merged into Markdown")
	})

	t.Run("early database-only branch stays unguarded", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		source, err := CreateArtifact(ctx, ws, "early branch source", "feature")
		require.NoError(t, err)
		target, err := CreateArtifact(ctx, ws, "early branch target", "feature")
		require.NoError(t, err)
		require.NoError(t, db.AddLink(ctx, ws.DB, source.ID, target.ID, "informs"))
		called := false
		previous := persistArtifactPreLockHook
		persistArtifactPreLockHook = func(string) { called = true }
		t.Cleanup(func() { persistArtifactPreLockHook = previous })

		require.NoError(t, RemoveArtifactLink(ctx, ws, source.ID, target.ID, "informs"))

		assert.False(t, called, "early db-only removal must not enter guarded persist")
		links, err := db.GetLinks(ctx, ws.DB, source.ID)
		require.NoError(t, err)
		assert.Empty(t, links)
	})
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
