package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/models"
)

// TestWriteStatusDirect_EmitsUTCUpdatedAt proves the authoritative gate-redirect
// status write restamps updated_at in canonical UTC even under a non-UTC local
// zone (site: gate_transition.go writeStatusDirect).
func TestWriteStatusDirect_EmitsUTCUpdatedAt(t *testing.T) {
	withNonUTCLocalWB(t)
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "Gate redirect feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Gate redirect task", "task", WithParent(feat.ID))
	require.NoError(t, err)

	artifact, err := loadArtifact(ctx, ws, task.ID)
	require.NoError(t, err)
	_, err = ws.writeStatusDirect(ctx, artifact, string(models.StatusQueued), string(models.StatusActive))
	require.NoError(t, err)

	assertFrontmatterUTCWB(t, readArtifactContentWB(t, ctx, ws, task.ID), "updated_at")
}
