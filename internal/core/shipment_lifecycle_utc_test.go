package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/models"
)

// TestAttachCommitToItems_EmitsUTCUpdatedAt proves the commit-link restamp
// writes updated_at in canonical UTC even under a non-UTC local zone
// (site: shipment_lifecycle.go attachCommitToItems).
func TestAttachCommitToItems_EmitsUTCUpdatedAt(t *testing.T) {
	withNonUTCLocalWB(t)
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "Commit link feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Commit link task", "task", WithParent(feat.ID))
	require.NoError(t, err)

	err = attachCommitToItems(ctx, ws, []string{task.ID}, &CommitMetadata{SHA: "deadbeefcafe"})
	require.NoError(t, err)

	assertFrontmatterUTCWB(t, readArtifactContentWB(t, ctx, ws, task.ID), "updated_at")
}

// TestSetArtifactStatus_EmitsUTCUpdatedAtWithCascade proves both the direct
// status restamp and the parent cascade restamp write updated_at in canonical
// UTC (sites: shipment_lifecycle.go setArtifactStatus + cascadePersistedParentStatuses).
func TestSetArtifactStatus_EmitsUTCUpdatedAtWithCascade(t *testing.T) {
	withNonUTCLocalWB(t)
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "Cascade feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Cascade task", "task", WithParent(feat.ID))
	require.NoError(t, err)

	_, err = setArtifactStatus(ctx, ws, task.ID, models.StatusActive, "utc cascade test")
	require.NoError(t, err)

	// Direct restamp (setArtifactStatus).
	assertFrontmatterUTCWB(t, readArtifactContentWB(t, ctx, ws, task.ID), "updated_at")
	// Parent cascade restamp (cascadePersistedParentStatuses).
	assertFrontmatterUTCWB(t, readArtifactContentWB(t, ctx, ws, feat.ID), "updated_at")
}
