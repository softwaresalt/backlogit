package core

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// captureDisposeIntentJournal returns the intent journal exactly as
// DisposeQueuedShipment persisted it. The journal is read inside the
// after-write hook, where it is still on disk.
func captureDisposeIntentJournal(t *testing.T, f *disposeFixture, hookErr error) (shipmentLifecycleJournal, error) {
	t.Helper()
	var captured shipmentLifecycleJournal
	found := false
	setDisposeHook(t, func() error {
		records, err := loadShipmentOperationJournals(f.ws)
		require.NoError(t, err)
		require.Len(t, records, 1)
		captured = records[0].lifecycle
		found = true
		return hookErr
	})
	authorizationRef := f.deliberation(t, "done", f.shipmentID)
	_, err := DisposeQueuedShipment(context.Background(), f.ws, f.shipmentID, f.opts(authorizationRef))
	require.True(t, found, "intent journal was never observed")
	return captured, err
}

// TestDisposeJournalUnnormalizedPreimage proves (192-F) that the journal keeps
// the shipment as loaded, so recovery of an absent or null items field does
// not fail the preimage comparison with ErrShipmentConflict.
func TestDisposeJournalUnnormalizedPreimage(t *testing.T) {
	ctx := context.Background()
	shapes := []struct {
		name  string
		apply func(*models.Artifact)
	}{
		{"absent_items", func(a *models.Artifact) { delete(a.CustomFields, "items") }},
		{"null_items", func(a *models.Artifact) { a.CustomFields["items"] = nil }},
	}
	for _, shape := range shapes {
		prepare := func(t *testing.T) *disposeFixture {
			t.Helper()
			f := newDisposeFixture(t)
			forced := cloneArtifact(loadURCanonicalArtifact(t, f.ws, f.shipmentID))
			shape.apply(forced)
			forceURArtifactFixture(t, f.ws, forced)
			return f
		}

		t.Run(shape.name+"/pre_write_recovery_keeps_queued", func(t *testing.T) {
			f := prepare(t)
			before := f.fileBytes(t, f.shipmentID)
			journal, err := captureDisposeIntentJournal(t, f, errors.New("injected pre-commit failure"))
			require.Error(t, err)
			f.requireUnchanged(t, f.shipmentID, before)
			if raw, ok := journal.Preimage.Shipment.CustomFields["items"]; ok {
				require.Nil(t, raw, "journal must not insert a normalized items list")
			}

			journal.Phase = "intent"
			_, err = writeShipmentLifecycleJournalForWorkspace(
				f.ws, shipmentLifecycleJournalName(journal.CorrelationID), journal)
			require.NoError(t, err)

			require.NoError(t, recoverPendingShipmentOperations(ctx, f.ws))
			f.requireUnchanged(t, f.shipmentID, before)
			require.Equal(t, models.StatusQueued, loadURCanonicalArtifact(t, f.ws, f.shipmentID).Status)
			f.requireNoJournals(t)

			claimed, err := ClaimShipment(ctx, f.ws, f.otherID)
			require.NoError(t, err)
			require.Equal(t, models.StatusActive, claimed.Status)
		})

		t.Run(shape.name+"/post_write_recovery_keeps_abandoned", func(t *testing.T) {
			f := prepare(t)
			journal, err := captureDisposeIntentJournal(t, f, nil)
			require.NoError(t, err)
			require.Equal(t, models.StatusAbandoned, loadURCanonicalArtifact(t, f.ws, f.shipmentID).Status)
			if raw, ok := journal.Preimage.Shipment.CustomFields["items"]; ok {
				require.Nil(t, raw, "journal must not insert a normalized items list")
			}

			journal.Phase = "intent"
			_, err = writeShipmentLifecycleJournalForWorkspace(
				f.ws, shipmentLifecycleJournalName(journal.CorrelationID), journal)
			require.NoError(t, err)

			err = recoverPendingShipmentOperations(ctx, f.ws)
			require.NotErrorIs(t, err, blerrors.ErrShipmentConflict)
			require.NoError(t, err)
			require.Equal(t, models.StatusAbandoned, loadURCanonicalArtifact(t, f.ws, f.shipmentID).Status)
			f.requireNoJournals(t)
		})
	}
}
