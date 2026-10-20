package core

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

const disposeRecoveryCorrelationID = "00112233445566778899aabbccddeeff"

// disposeRecoveryFixture holds a queued shipment (the dispose subject), a
// second queued shipment that later lifecycle calls operate on, and the
// hand-written dispose journal.
type disposeRecoveryFixture struct {
	ws          *Workspace
	shipmentID  string
	otherID     string
	featureID   string
	memberIDs   []string
	preimage    *models.Artifact
	journal     shipmentLifecycleJournal
	journalPath string
	reason      string
}

func newDisposeRecoveryFixture(t *testing.T) *disposeRecoveryFixture {
	t.Helper()
	ctx := context.Background()
	ws := setupShipmentWorkspace(t)
	feature, err := CreateArtifact(ctx, ws, "Dispose recovery feature", "feature")
	require.NoError(t, err)
	memberA, err := CreateArtifact(ctx, ws, "Dispose recovery member A", "task", WithParent(feature.ID))
	require.NoError(t, err)
	memberB, err := CreateArtifact(ctx, ws, "Dispose recovery member B", "task", WithParent(feature.ID))
	require.NoError(t, err)
	otherMember, err := CreateArtifact(ctx, ws, "Dispose recovery other member", "task", WithParent(feature.ID))
	require.NoError(t, err)

	shipment, err := CreateShipment(ctx, ws, "Dispose recovery shipment", []string{memberA.ID, memberB.ID})
	require.NoError(t, err)
	other, err := CreateShipment(ctx, ws, "Dispose recovery other shipment", []string{otherMember.ID})
	require.NoError(t, err)
	require.Equal(t, models.StatusQueued, shipment.Status)

	preimage, err := loadArtifact(ctx, ws, shipment.ID)
	require.NoError(t, err)
	memberIDs := NormalizeShipmentItems(preimage)
	members := make([]*models.Artifact, 0, len(memberIDs))
	for _, id := range memberIDs {
		member, loadErr := loadArtifact(ctx, ws, id)
		require.NoError(t, loadErr)
		members = append(members, cloneArtifact(member))
	}

	f := &disposeRecoveryFixture{
		ws:         ws,
		shipmentID: shipment.ID,
		otherID:    other.ID,
		featureID:  feature.ID,
		memberIDs:  memberIDs,
		preimage:   cloneArtifact(preimage),
		reason:     "superseded by newer plan",
	}
	f.journal = shipmentLifecycleJournal{
		SchemaVersion:  "shipment-operation/v1",
		CorrelationID:  disposeRecoveryCorrelationID,
		Phase:          "intent",
		Operation:      "dispose",
		RecoveryPolicy: "rollback",
		ShipmentID:     shipment.ID,
		Target:         string(ShipmentAbandoned),
		Reason:         f.reason,
		Preimage: shipmentLifecyclePreimage{
			Shipment: cloneArtifact(preimage),
			Members:  members,
		},
	}
	return f
}

func (f *disposeRecoveryFixture) writeJournal(t *testing.T, phase string) {
	t.Helper()
	f.journal.Phase = phase
	path, err := writeShipmentLifecycleJournalForWorkspace(
		f.ws, shipmentLifecycleJournalName(f.journal.CorrelationID), f.journal)
	require.NoError(t, err)
	f.journalPath = path
}

// applyDisposition performs the real single governed write (B-U7 marker) with
// the journal correlation ID so the status event is tagged like production.
func (f *disposeRecoveryFixture) applyDisposition(t *testing.T) {
	t.Helper()
	ctx := withShipmentOperation(context.Background(), f.journal.CorrelationID)
	ctx = context.WithValue(ctx, governedShipmentDispositionContextKey{}, shipmentDispositionProvenance{
		Reason: f.reason, By: "operator", AuthorizationRef: "DL-1", SupersededBy: f.otherID,
	})
	require.NoError(t, MoveShipmentStatus(ctx, f.ws, f.shipmentID, ShipmentAbandoned))
}

func (f *disposeRecoveryFixture) fileBytes(t *testing.T, id string) []byte {
	t.Helper()
	path, err := FindArtifactPath(context.Background(), f.ws, id)
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return data
}

func (f *disposeRecoveryFixture) abandonedStatusEvents(t *testing.T) int {
	t.Helper()
	count := 0
	for _, event := range readUREvents(t, f.ws, f.shipmentID) {
		if event.EventType == "shipment_status_changed" && event.Delta["status"] == "abandoned" &&
			event.Delta[shipmentOperationDeltaKey] == f.journal.CorrelationID {
			count++
		}
	}
	return count
}

func (f *disposeRecoveryFixture) requireJournalGone(t *testing.T) {
	t.Helper()
	_, err := os.Stat(f.journalPath)
	require.True(t, os.IsNotExist(err), "journal must be removed, stat err: %v", err)
}

func TestDisposeJournalRecovery(t *testing.T) {
	ctx := context.Background()

	t.Run("intent_before_write_queued_preimage_cleans_up_without_change", func(t *testing.T) {
		f := newDisposeRecoveryFixture(t)
		f.writeJournal(t, "intent")
		before := map[string][]byte{f.shipmentID: f.fileBytes(t, f.shipmentID)}
		for _, id := range f.memberIDs {
			before[id] = f.fileBytes(t, id)
		}

		require.NoError(t, recoverPendingShipmentOperations(ctx, f.ws))

		for id, want := range before {
			require.Equal(t, want, f.fileBytes(t, id), "artifact %s must be unchanged", id)
		}
		got := loadURCanonicalArtifact(t, f.ws, f.shipmentID)
		require.Equal(t, models.StatusQueued, got.Status)
		require.NotContains(t, got.CustomFields, "disposition_reason")
		require.Equal(t, 0, f.abandonedStatusEvents(t))
		f.requireJournalGone(t)
	})

	t.Run("intent_after_write_complete_disposition_is_kept", func(t *testing.T) {
		f := newDisposeRecoveryFixture(t)
		// The journal is written after the governed write only to build the
		// crash state: a live intent journal would trigger the recovery barrier
		// inside MoveShipmentStatus.
		f.applyDisposition(t)
		f.writeJournal(t, "intent")
		require.Equal(t, 1, f.abandonedStatusEvents(t))
		memberBytes := map[string][]byte{}
		for _, id := range f.memberIDs {
			memberBytes[id] = f.fileBytes(t, id)
		}

		require.NoError(t, recoverPendingShipmentOperations(ctx, f.ws))

		got := loadURCanonicalArtifact(t, f.ws, f.shipmentID)
		require.Equal(t, models.StatusAbandoned, got.Status, "G-D: complete disposition must not be rolled back")
		require.Equal(t, f.reason, got.CustomFields["disposition_reason"])
		require.Equal(t, "operator", got.CustomFields["disposition_by"])
		require.Equal(t, "DL-1", got.CustomFields["disposition_authorization_ref"])
		require.Equal(t, f.otherID, got.CustomFields["superseded_by"])
		require.Equal(t, 1, f.abandonedStatusEvents(t), "G-D: the status event is kept")
		for id, want := range memberBytes {
			require.Equal(t, want, f.fileBytes(t, id), "member %s must be unchanged", id)
		}
		f.requireJournalGone(t)

		// A second pass is a no-op.
		require.NoError(t, recoverPendingShipmentOperations(ctx, f.ws))
		require.Equal(t, models.StatusAbandoned, loadURCanonicalArtifact(t, f.ws, f.shipmentID).Status)
	})

	t.Run("intent_after_write_with_db_only_link_disposition_is_kept", func(t *testing.T) {
		f := newDisposeRecoveryFixture(t)
		// The journal preimage (unchanged production shape) has no link; the
		// governed write then materializes the DB-only link into Markdown.
		require.NoError(t, bldb.AddLink(ctx, f.ws.DB, f.shipmentID, f.featureID, "related_to"))
		f.applyDisposition(t)
		require.NotEmpty(t, loadURCanonicalArtifact(t, f.ws, f.shipmentID).Links)
		require.Empty(t, f.journal.Preimage.Shipment.Links)
		f.writeJournal(t, "intent")

		require.NoError(t, recoverPendingShipmentOperations(ctx, f.ws))

		got := loadURCanonicalArtifact(t, f.ws, f.shipmentID)
		require.Equal(t, models.StatusAbandoned, got.Status)
		f.requireJournalGone(t)
	})

	t.Run("db_only_link_does_not_mask_unrelated_change", func(t *testing.T) {
		f := newDisposeRecoveryFixture(t)
		require.NoError(t, bldb.AddLink(ctx, f.ws.DB, f.shipmentID, f.featureID, "related_to"))
		f.applyDisposition(t)
		changed := cloneArtifact(loadURCanonicalArtifact(t, f.ws, f.shipmentID))
		changed.Title = "unrelated edit"
		forceURArtifactFixture(t, f.ws, changed)
		f.writeJournal(t, "intent")

		err := recoverPendingShipmentOperations(ctx, f.ws)

		require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
	})

	t.Run("kept_disposition_repairs_stale_queued_index", func(t *testing.T) {
		f := newDisposeRecoveryFixture(t)
		f.applyDisposition(t)
		// Simulate a crash after the Markdown write but before the index upsert.
		require.NoError(t, bldb.UpsertItem(ctx, f.ws.DB, f.preimage))
		stale, err := bldb.GetItem(ctx, f.ws.DB, f.shipmentID)
		require.NoError(t, err)
		require.Equal(t, models.StatusQueued, stale.Status)
		f.writeJournal(t, "intent")
		before := f.fileBytes(t, f.shipmentID)

		require.NoError(t, recoverPendingShipmentOperations(ctx, f.ws))

		indexed, err := bldb.GetItem(ctx, f.ws.DB, f.shipmentID)
		require.NoError(t, err)
		require.Equal(t, models.StatusAbandoned, indexed.Status)
		got, err := GetShipment(ctx, f.ws, f.shipmentID)
		require.NoError(t, err)
		require.Equal(t, models.StatusAbandoned, got.Status)
		require.Equal(t, f.reason, got.CustomFields["disposition_reason"])
		require.Equal(t, before, f.fileBytes(t, f.shipmentID), "Markdown must not be rewritten")
		require.Equal(t, 1, f.abandonedStatusEvents(t))
		f.requireJournalGone(t)
	})

	t.Run("intent_foreign_state_fails_with_shipment_conflict", func(t *testing.T) {
		f := newDisposeRecoveryFixture(t)
		f.writeJournal(t, "intent")
		// Foreign state: queued but carrying disposition keys, which no
		// candidate (queued preimage, or complete abandoned) matches.
		foreign := cloneArtifact(f.preimage)
		if foreign.CustomFields == nil {
			foreign.CustomFields = map[string]any{}
		}
		foreign.CustomFields["disposition_reason"] = f.reason
		foreign.CustomFields["disposition_by"] = "operator"
		foreign.CustomFields["disposition_authorization_ref"] = "DL-1"
		governed := context.WithValue(ctx, artifactWriteEnvelopeContextKey{}, artifactWriteEnvelope{
			operation:                     "move_shipment_status",
			allowGovernedShipmentMutation: true,
		})
		require.NoError(t, persistArtifact(governed, f.ws, foreign, true))
		beforeBytes := f.fileBytes(t, f.shipmentID)

		err := recoverPendingShipmentOperations(ctx, f.ws)

		// G-C: assert the reachable output (sentinel plus shipment ID), not the
		// journal filename, which the existing CAS error does not emit.
		require.Error(t, err)
		require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
		require.Contains(t, err.Error(), f.shipmentID)
		require.Equal(t, beforeBytes, f.fileBytes(t, f.shipmentID), "foreign state must be left untouched")
		_, statErr := os.Stat(f.journalPath)
		require.NoError(t, statErr, "journal stays as operator-repair evidence")
	})

	t.Run("incomplete_abandoned_state_fails_with_shipment_conflict", func(t *testing.T) {
		f := newDisposeRecoveryFixture(t)
		f.reason = "different reason"
		f.applyDisposition(t)
		f.reason = "superseded by newer plan"
		f.journal.Reason = f.reason
		f.writeJournal(t, "intent")

		err := recoverPendingShipmentOperations(ctx, f.ws)

		require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
		require.Equal(t, models.StatusAbandoned, loadURCanonicalArtifact(t, f.ws, f.shipmentID).Status)
		_, statErr := os.Stat(f.journalPath)
		require.NoError(t, statErr)
	})

	t.Run("leftover_committed_journal_is_inert", func(t *testing.T) {
		f := newDisposeRecoveryFixture(t)
		f.applyDisposition(t)
		f.writeJournal(t, "committed")
		journalBytes, err := os.ReadFile(f.journalPath)
		require.NoError(t, err)
		shipmentBytes := f.fileBytes(t, f.shipmentID)

		records, err := loadShipmentOperationJournals(f.ws)
		require.NoError(t, err, "a committed dispose journal must load")
		require.Len(t, records, 1)

		require.NoError(t, recoverPendingShipmentOperations(ctx, f.ws))

		after, err := os.ReadFile(f.journalPath)
		require.NoError(t, err)
		require.Equal(t, journalBytes, after, "committed journal is left unchanged")
		require.Equal(t, shipmentBytes, f.fileBytes(t, f.shipmentID))
		require.Equal(t, models.StatusAbandoned, loadURCanonicalArtifact(t, f.ws, f.shipmentID).Status)

		claimed, err := ClaimShipment(ctx, f.ws, f.otherID)
		require.NoError(t, err)
		require.Equal(t, models.StatusActive, claimed.Status)
	})

	t.Run("leftover_intent_journal_does_not_block_later_operations", func(t *testing.T) {
		f := newDisposeRecoveryFixture(t)
		f.writeJournal(t, "intent")

		claimed, err := ClaimShipment(ctx, f.ws, f.otherID)
		require.NoError(t, err, "recovery resolves the intent journal first")
		require.Equal(t, models.StatusActive, claimed.Status)
		f.requireJournalGone(t)
		require.Equal(t, models.StatusQueued, loadURCanonicalArtifact(t, f.ws, f.shipmentID).Status)
	})

	t.Run("leftover_intent_journal_does_not_block_add_item", func(t *testing.T) {
		f := newDisposeRecoveryFixture(t)
		f.writeJournal(t, "intent")
		extra, err := CreateArtifact(ctx, f.ws, "Dispose recovery extra task", "task", WithParent(f.featureID))
		require.NoError(t, err)

		require.NoError(t, AddItemToShipment(ctx, f.ws, f.otherID, extra.ID))
		f.requireJournalGone(t)
	})
}
