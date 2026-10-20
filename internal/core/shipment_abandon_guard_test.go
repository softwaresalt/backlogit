package core

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// TestShipmentQueuedAbandonProtected verifies queued -> abandoned is refused
// on every generic shipment write path (B-U3).
func TestShipmentQueuedAbandonProtected(t *testing.T) {
	newQueued := func(t *testing.T) (*Workspace, string, string) {
		t.Helper()
		ws := setupShipmentWorkspace(t)
		ctx := context.Background()
		shipment, err := CreateShipment(ctx, ws, "Abandon guard shipment", nil)
		require.NoError(t, err)
		require.Equal(t, models.StatusQueued, shipment.Status)
		path, err := FindArtifactPath(ctx, ws, shipment.ID)
		require.NoError(t, err)
		return ws, shipment.ID, path
	}
	readFile := func(t *testing.T, path string) []byte {
		t.Helper()
		data, err := os.ReadFile(path)
		require.NoError(t, err)
		return data
	}

	t.Run("characterization_move_without_marker_and_queued_to_active", func(t *testing.T) {
		ws, id, path := newQueued(t)
		ctx := context.Background()
		before := readFile(t, path)

		err := MoveShipmentStatus(ctx, ws, id, ShipmentAbandoned)
		require.ErrorIs(t, err, blerrors.ErrShipmentConflict)

		_, err = UpdateArtifact(ctx, ws, id, map[string]any{"status": "active"})
		require.Error(t, err)
		require.Equal(t, before, readFile(t, path))
	})

	t.Run("update_artifact_refuses_abandoned", func(t *testing.T) {
		ws, id, path := newQueued(t)
		before := readFile(t, path)
		_, err := UpdateArtifact(context.Background(), ws, id, map[string]any{"status": "abandoned"})
		require.ErrorIs(t, err, blerrors.ErrShipmentBlockedRequiresEnvelope)
		require.Equal(t, before, readFile(t, path))
	})

	t.Run("update_artifact_with_gate_refuses_abandoned", func(t *testing.T) {
		ws, id, path := newQueued(t)
		before := readFile(t, path)
		_, _, err := UpdateArtifactWithGate(context.Background(), ws, id,
			map[string]any{"status": "abandoned"}, TransitionOptions{})
		require.ErrorIs(t, err, blerrors.ErrShipmentBlockedRequiresEnvelope)
		require.Equal(t, before, readFile(t, path))
	})

	t.Run("bulk_update_status_reports_failed", func(t *testing.T) {
		ws, id, path := newQueued(t)
		before := readFile(t, path)
		result, err := BulkUpdateStatus(context.Background(), nil, ws, []string{id}, "abandoned")
		require.NoError(t, err)
		require.Contains(t, result.Failed, id)
		require.Zero(t, result.Succeeded)
		require.Equal(t, before, readFile(t, path))
	})
}
