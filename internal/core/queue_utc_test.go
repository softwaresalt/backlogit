package core_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
)

// TestBulkUpdateStatus_EmitsUTCUpdatedAt proves the bulk status-update restamp
// writes updated_at in canonical UTC even under a non-UTC local zone.
func TestBulkUpdateStatus_EmitsUTCUpdatedAt(t *testing.T) {
	withNonUTCLocal(t)
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feat, err := core.CreateArtifact(ctx, ws, "Bulk feature", "feature")
	require.NoError(t, err)
	task, err := core.CreateArtifact(ctx, ws, "Bulk task", "task", core.WithParent(feat.ID))
	require.NoError(t, err)

	res, err := core.BulkUpdateStatus(ctx, ws.DB, ws, []string{task.ID}, "active")
	require.NoError(t, err)
	require.Empty(t, res.Failed)

	assertFrontmatterUTC(t, readArtifactContent(t, ctx, ws, task.ID), "updated_at")
}

// TestMoveInQueue_EmitsUTCUpdatedAt proves the queue-reorder restamp writes
// updated_at in canonical UTC.
func TestMoveInQueue_EmitsUTCUpdatedAt(t *testing.T) {
	withNonUTCLocal(t)
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feat, err := core.CreateArtifact(ctx, ws, "Queue feature", "feature")
	require.NoError(t, err)
	_, err = core.CreateArtifact(ctx, ws, "Queue task 1", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	_, err = core.CreateArtifact(ctx, ws, "Queue task 2", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	t3, err := core.CreateArtifact(ctx, ws, "Queue task 3", "task", core.WithParent(feat.ID))
	require.NoError(t, err)

	filter := &core.QueueFilter{Statuses: []string{"queued"}}
	require.NoError(t, core.MoveInQueue(ctx, ws, t3.ID, 1, filter))

	assertFrontmatterUTC(t, readArtifactContent(t, ctx, ws, t3.ID), "updated_at")
}
