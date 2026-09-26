package core

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

func TestU20C5_CommitJournalFailureAfterCommittedEvidenceDoesNotCompensate(t *testing.T) {
	tests := []struct {
		name       string
		op         string
		target     models.ArtifactStatus
		errorLabel string
	}{
		{
			name:       "block",
			op:         "block",
			target:     models.StatusBlocked,
			errorLabel: "persist block shipment commit",
		},
		{
			name:       "unblock_to_queued",
			op:         "unblock",
			target:     models.StatusQueued,
			errorLabel: "persist unblock shipment commit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			root, ws := setupUR3Workspace(t)
			t.Cleanup(func() { require.NoError(t, ws.Close()) })
			fixture := newURBlockedActiveFixture(t, ws)

			shipmentForOperation := fixture.shipment
			if tt.op == "unblock" {
				var err error
				shipmentForOperation, err = BlockShipment(ctx, ws, fixture.shipment.ID, BlockOptions{
					Reason:    "waiting for committed journal recovery",
					BlockedBy: "U20C5",
				})
				require.NoError(t, err)
				require.NotNil(t, shipmentForOperation)
			}

			shipmentID := shipmentForOperation.ID
			preimageShipment := cloneArtifact(loadURCanonicalArtifact(t, ws, shipmentID))
			if tt.op == "unblock" {
				_, err := validatePersistedBlockedShipmentEnvelope(ctx, ws, preimageShipment)
				require.NoError(t, err, "unblock must start from a canonical blocked envelope")
				for _, memberID := range NormalizeShipmentItems(preimageShipment) {
					member := loadURCanonicalArtifact(t, ws, memberID)
					require.Equal(t, models.StatusQueued, member.Status,
						"unblock to queued must not need a member restore write")
				}
			}

			baselineEvents := len(readUREvents(t, ws, shipmentID))
			realWrite := persistArtifactWriteFn
			armed := false
			restoreObstruction := func() {}
			persistArtifactWriteFn = func(artifact *models.Artifact, path string, durable bool) error {
				if !armed &&
					artifact.ID == shipmentID &&
					artifact.ArtifactType == "shipment" &&
					artifact.Status == tt.target {
					if err := realWrite(artifact, path, durable); err != nil {
						return err
					}
					armed = true
					restoreObstruction = armU20C3FilesystemObstruction(t, shipmentOpsRoot(root), false)
					return nil
				}
				return realWrite(artifact, path, durable)
			}
			t.Cleanup(func() { persistArtifactWriteFn = realWrite })

			var returnedArtifact *models.Artifact
			var operationErr error
			if tt.op == "block" {
				returnedArtifact, operationErr = BlockShipment(ctx, ws, shipmentID, BlockOptions{
					Reason:    "waiting for committed journal recovery",
					BlockedBy: "U20C5",
				})
			} else {
				returnedArtifact, operationErr = UnblockShipment(ctx, ws, shipmentID, UnblockOptions{
					Target:      ShipmentQueued,
					Confirm:     true,
					UnblockedBy: "U20C5",
				})
			}

			assert.True(t, armed,
				"the final governed shipment write must succeed before the shipment-operations obstruction is armed")
			assert.Nil(t, returnedArtifact, "an indeterminate commit-journal failure must return no artifact")
			require.Error(t, operationErr)
			assert.True(t, errors.Is(operationErr, blerrors.ErrWriteIndeterminate))
			assert.Contains(t, operationErr.Error(), tt.errorLabel)
			assert.False(t, errors.Is(operationErr, blerrors.ErrValidation))
			assert.False(t, errors.Is(operationErr, blerrors.ErrShipmentConflict))
			assert.False(t, errors.Is(operationErr, blerrors.ErrWriteNotApplied))
			assert.False(t, errors.Is(operationErr, blerrors.ErrNotFound))
			var partialErr *blerrors.MutationPartialError
			assert.False(t, errors.As(operationErr, &partialErr))

			shipmentAfterFailure := cloneArtifact(loadURCanonicalArtifact(t, ws, shipmentID))
			assert.Equal(t, tt.target, shipmentAfterFailure.Status,
				"the already-committed shipment state must remain persisted after the failed journal write")
			if tt.op == "block" {
				_, envelopeErr := validatePersistedBlockedShipmentEnvelope(ctx, ws, shipmentAfterFailure)
				assert.NoError(t, envelopeErr, "the committed block must retain a canonical envelope")
			}

			itemEvents := readUREvents(t, ws, shipmentID)
			require.Greater(t, len(itemEvents), baselineEvents,
				"the operation must append lifecycle evidence after the fixture baseline")
			var correlationID string
			for _, event := range itemEvents[baselineEvents:] {
				if event.EventType == "shipment_lifecycle" &&
					eventPhaseUR(event) == "intent" &&
					eventLifecycleOperationUR(event) == tt.op {
					correlationID = exactEventCorrelationUR(event)
					break
				}
			}
			require.NotEmpty(t, correlationID, "the failed operation must have a correlated lifecycle intent")

			committedEvents := 0
			compensatedLifecycleEvents := 0
			compensatedStatusEvents := 0
			for _, event := range itemEvents {
				if exactEventCorrelationUR(event) != correlationID ||
					eventLifecycleOperationUR(event) != tt.op {
					continue
				}
				if event.EventType == "shipment_lifecycle" {
					switch eventPhaseUR(event) {
					case "committed":
						committedEvents++
					case "compensated":
						compensatedLifecycleEvents++
					}
				}
				if event.EventType == "shipment_status_changed" {
					target, targetOK := event.Delta["target"].(string)
					status, statusOK := event.Delta["status"].(string)
					if eventPhaseUR(event) == "compensated" ||
						(targetOK && statusOK &&
							target == string(preimageShipment.Status) &&
							status == string(preimageShipment.Status)) {
						compensatedStatusEvents++
					}
				}
			}
			assert.GreaterOrEqual(t, committedEvents, 1,
				"a correlated committed lifecycle event must verify durable committed evidence before the journal failure")
			assert.Zero(t, compensatedLifecycleEvents,
				"durable committed evidence must never be followed by a compensated lifecycle event")
			assert.Zero(t, compensatedStatusEvents,
				"durable committed evidence must never be followed by compensating status evidence")

			restoreObstruction()
			records, err := loadShipmentOperationJournals(ws)
			require.NoError(t, err)
			var journal shipmentLifecycleJournal
			journalFound := false
			for _, record := range records {
				if record.kind == shipmentLifecycleJournalKind &&
					record.lifecycle.CorrelationID == correlationID {
					journal = record.lifecycle
					journalFound = true
					break
				}
			}
			require.True(t, journalFound, "the original lifecycle journal must remain available for recovery")
			assert.Equal(t, "intent", journal.Phase,
				"the failed journal commit must leave the on-disk journal at intent")

			require.NoError(t, recoverPendingShipmentOperations(ctx, ws),
				"recovery must finalize the journal from durable committed evidence")
			records, err = loadShipmentOperationJournals(ws)
			require.NoError(t, err)
			journalFound = false
			for _, record := range records {
				if record.kind == shipmentLifecycleJournalKind &&
					record.lifecycle.CorrelationID == correlationID {
					journal = record.lifecycle
					journalFound = true
					break
				}
			}
			require.True(t, journalFound, "recovery must retain the lifecycle journal")
			assert.Equal(t, "committed", journal.Phase)

			recoveredShipment := cloneArtifact(loadURCanonicalArtifact(t, ws, shipmentID))
			assert.Equal(t, tt.target, recoveredShipment.Status,
				"persisted shipment state must be re-read after recovery")
			if tt.op == "block" {
				_, envelopeErr := validatePersistedBlockedShipmentEnvelope(ctx, ws, recoveredShipment)
				require.NoError(t, envelopeErr, "recovered block must retain a canonical envelope")
			}

			report, doctorErr := Doctor(ctx, ws, &DoctorOptions{
				CheckOrphans:    false,
				CheckDuplicates: false,
			})
			require.NoError(t, doctorErr)
			require.NotNil(t, report)
			assert.Empty(t, report.Findings, "Doctor must report a clean workspace after recovery")
			assert.False(t, hasUR3DoctorFinding(
				report,
				shipmentID,
				FindingConflictingShipmentLifecycleEvidence,
			), "Doctor must not report conflicting lifecycle evidence after recovery")
		})
	}
}
