package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

// seedArchivedPredecessor writes the Markdown provenance and index row for an
// archived shipment predecessor.
func seedArchivedPredecessor(t *testing.T, ws *Workspace, id, archivedStatus string) {
	t.Helper()
	ctx := context.Background()
	writeReadinessFixture(t, ws, "archive", id, readinessShipmentMarkdown(id, "status: archived\narchived_status: "+archivedStatus+"\n"))
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, &models.Artifact{ID: id, Title: "Predecessor " + id, Status: models.StatusArchived, ArtifactType: "shipment"}))
}

func seedQueuedShipmentBehind(t *testing.T, ws *Workspace, id, predID string) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, &models.Artifact{ID: id, Title: "Dependent " + id, Status: models.StatusQueued, ArtifactType: "shipment"}))
	require.NoError(t, bldb.UpsertDependency(ctx, ws.DB, id, predID, "blocks"))
}

// TestQueryQueueForWorkspace pins the workspace entry point that resolves
// shipment predecessor readiness from Markdown provenance (A-U7 / 188.003-T).
func TestQueryQueueForWorkspace(t *testing.T) {
	filter := func() *QueueFilter {
		return &QueueFilter{Types: []string{"shipment"}, Statuses: []string{"queued"}}
	}

	t.Run("archived shipped predecessor included and abandoned excluded", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		seedArchivedPredecessor(t, ws, "950-S", "shipped")
		seedArchivedPredecessor(t, ws, "951-S", "abandoned")
		seedQueuedShipmentBehind(t, ws, "960-S", "950-S")
		seedQueuedShipmentBehind(t, ws, "961-S", "951-S")

		view, err := QueryQueueForWorkspace(context.Background(), ws, filter())
		require.NoError(t, err)
		require.NotNil(t, view)
		assert.True(t, queueContains(view, "960-S"), "shipment behind archived shipped predecessor must be included")
		assert.False(t, queueContains(view, "961-S"), "shipment behind archived abandoned predecessor must be excluded")
	})

	t.Run("MoveInQueue succeeds behind archived shipped predecessor", func(t *testing.T) {
		ctx := context.Background()
		ws := setupShipmentWorkspace(t)
		_, err := CreateShipment(ctx, ws, "Other queued shipment", nil)
		require.NoError(t, err)
		dependent, err := CreateShipment(ctx, ws, "Dependent shipment", nil)
		require.NoError(t, err)
		seedArchivedPredecessor(t, ws, "950-S", "shipped")
		require.NoError(t, bldb.UpsertDependency(ctx, ws.DB, dependent.ID, "950-S", "blocks"))

		require.NoError(t, MoveInQueue(ctx, ws, dependent.ID, 1, filter()))
	})

	t.Run("predicate infrastructure error propagates wrapped", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		seedArchivedPredecessor(t, ws, "950-S", "shipped")
		seedQueuedShipmentBehind(t, ws, "960-S", "950-S")

		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		view, err := QueryQueueForWorkspace(ctx, ws, filter())
		require.Error(t, err)
		assert.Nil(t, view)
		assert.ErrorIs(t, err, context.Canceled)
	})
}
