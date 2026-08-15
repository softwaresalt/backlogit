package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/models"
)

func TestPersistArtifactWithGuardChecksBeforeFirstWrite(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feature, err := CreateArtifact(ctx, ws, "CAS feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "CAS task", "task", WithParent(feature.ID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "CAS shipment", []string{task.ID})
	require.NoError(t, err)

	for _, artifact := range []*models.Artifact{feature, task, shipment} {
		var sequence []string
		originalWriter := persistArtifactWriteFn
		persistArtifactWriteFn = func(artifact *models.Artifact, filePath string, durable bool) error {
			sequence = append(sequence, "write")
			return originalWriter(artifact, filePath, durable)
		}
		guardErr := errors.New("CAS refused")
		err := persistArtifactWithGuard(ctx, ws, artifact, false, func(context.Context) error {
			sequence = append(sequence, "guard")
			return guardErr
		})
		persistArtifactWriteFn = originalWriter

		require.ErrorIs(t, err, guardErr)
		assert.Equal(t, []string{"guard"}, sequence, "artifact %s must not write after guard refusal", artifact.ID)
	}
}

func TestPersistArtifactWithGuardRejectsRelocationBeforeDirectoryCreation(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feature, err := CreateArtifact(ctx, ws, "CAS relocation feature", "feature")
	require.NoError(t, err)
	feature.Status = models.StatusDone
	archiveDir := filepath.Join(WorkspaceStorageRoot(ws.RootPath), "archive")
	require.NoError(t, os.RemoveAll(archiveDir))

	var sequence []string
	originalWriter := persistArtifactWriteFn
	persistArtifactWriteFn = func(artifact *models.Artifact, filePath string, durable bool) error {
		sequence = append(sequence, "write")
		return originalWriter(artifact, filePath, durable)
	}
	t.Cleanup(func() { persistArtifactWriteFn = originalWriter })

	guardErr := errors.New("CAS refused")
	err = persistArtifactWithGuard(ctx, ws, feature, true, func(context.Context) error {
		sequence = append(sequence, "guard")
		return guardErr
	})

	require.ErrorIs(t, err, guardErr)
	assert.Equal(t, []string{"guard"}, sequence)
	_, statErr := os.Stat(archiveDir)
	assert.ErrorIs(t, statErr, os.ErrNotExist, "guard refusal must not create the relocation target")
}

func TestPersistArtifactWithGuardAllowsWriteAfterCheck(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feature, err := CreateArtifact(ctx, ws, "CAS success feature", "feature")
	require.NoError(t, err)
	sequence := []string{}
	originalWriter := persistArtifactWriteFn
	persistArtifactWriteFn = func(artifact *models.Artifact, filePath string, durable bool) error {
		sequence = append(sequence, "write")
		return originalWriter(artifact, filePath, durable)
	}
	t.Cleanup(func() { persistArtifactWriteFn = originalWriter })

	err = persistArtifactWithGuard(ctx, ws, feature, false, func(context.Context) error {
		sequence = append(sequence, "guard")
		return nil
	})

	require.NoError(t, err)
	assert.Equal(t, []string{"guard", "write"}, sequence)
}
