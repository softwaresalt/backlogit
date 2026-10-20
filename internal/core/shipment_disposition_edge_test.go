package core

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// TestGovernedShipmentDispositionEdge verifies the governed-disposition marker
// opens only queued -> abandoned, with one atomic provenance write (B-U6).
func TestGovernedShipmentDispositionEdge(t *testing.T) {
	newQueued := func(t *testing.T) (*Workspace, string, string) {
		t.Helper()
		ws := setupShipmentWorkspace(t)
		ctx := context.Background()
		shipment, err := CreateShipment(ctx, ws, "Disposition edge shipment", nil)
		require.NoError(t, err)
		path, err := FindArtifactPath(ctx, ws, shipment.ID)
		require.NoError(t, err)
		return ws, shipment.ID, path
	}
	withMarker := func(p shipmentDispositionProvenance) context.Context {
		return context.WithValue(context.Background(), governedShipmentDispositionContextKey{}, p)
	}
	statusChangedCount := func(t *testing.T, ws *Workspace, id string) int {
		t.Helper()
		count := 0
		for _, event := range readUREvents(t, ws, id) {
			if event.EventType == "shipment_status_changed" && event.Delta["status"] == "abandoned" {
				count++
			}
		}
		return count
	}

	t.Run("marker_moves_queued_to_abandoned_with_provenance", func(t *testing.T) {
		ws, id, _ := newQueued(t)
		err := MoveShipmentStatus(withMarker(shipmentDispositionProvenance{
			Reason: "superseded", By: "operator", AuthorizationRef: "DL-1", SupersededBy: "002-S",
		}), ws, id, ShipmentAbandoned)
		require.NoError(t, err)

		got := loadURCanonicalArtifact(t, ws, id)
		require.Equal(t, models.StatusAbandoned, got.Status)
		require.Equal(t, "superseded", got.CustomFields["disposition_reason"])
		require.Equal(t, "operator", got.CustomFields["disposition_by"])
		require.Equal(t, "DL-1", got.CustomFields["disposition_authorization_ref"])
		require.Equal(t, "002-S", got.CustomFields["superseded_by"])
		require.Equal(t, 1, statusChangedCount(t, ws, id))
	})

	t.Run("empty_superseded_by_omits_key", func(t *testing.T) {
		ws, id, _ := newQueued(t)
		err := MoveShipmentStatus(withMarker(shipmentDispositionProvenance{
			Reason: "superseded", By: "operator", AuthorizationRef: "DL-1",
		}), ws, id, ShipmentAbandoned)
		require.NoError(t, err)

		got := loadURCanonicalArtifact(t, ws, id)
		require.Equal(t, models.StatusAbandoned, got.Status)
		require.Equal(t, "superseded", got.CustomFields["disposition_reason"])
		_, present := got.CustomFields["superseded_by"]
		require.False(t, present, "superseded_by must be absent when empty")
	})

	t.Run("marker_opens_no_other_edge", func(t *testing.T) {
		ws, id, path := newQueued(t)
		marker := withMarker(shipmentDispositionProvenance{
			Reason: "r", By: "b", AuthorizationRef: "a",
		})
		before, err := os.ReadFile(path)
		require.NoError(t, err)

		require.Error(t, MoveShipmentStatus(marker, ws, id, ShipmentShipped))
		after, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, before, after)

		ws2 := setupShipmentWorkspace(t)
		fixture := newURBlockedActiveFixture(t, ws2)
		err = MoveShipmentStatus(marker, ws2, fixture.shipment.ID, ShipmentQueued)
		require.Error(t, err, "active to queued must stay refused with the marker present")
		require.Equal(t, models.StatusActive, loadURCanonicalArtifact(t, ws2, fixture.shipment.ID).Status)
	})

	t.Run("incomplete_provenance_is_validation_error", func(t *testing.T) {
		cases := []struct {
			name string
			p    shipmentDispositionProvenance
		}{
			{"empty_reason", shipmentDispositionProvenance{By: "b", AuthorizationRef: "a"}},
			{"empty_by", shipmentDispositionProvenance{Reason: "r", AuthorizationRef: "a"}},
			{"empty_authorization_ref", shipmentDispositionProvenance{Reason: "r", By: "b"}},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				ws, id, path := newQueued(t)
				before, err := os.ReadFile(path)
				require.NoError(t, err)

				err = MoveShipmentStatus(withMarker(tc.p), ws, id, ShipmentAbandoned)
				require.ErrorIs(t, err, blerrors.ErrValidation)

				after, err := os.ReadFile(path)
				require.NoError(t, err)
				require.Equal(t, before, after)
			})
		}
	})
}
