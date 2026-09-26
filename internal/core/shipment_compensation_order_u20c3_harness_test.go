package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

func TestU20C3_CompensationEventFailureLeavesRecoverableIntent(t *testing.T) {
	tests := []struct {
		name        string
		operation   string
		injectedErr error
	}{
		{
			name:        "block",
			operation:   "block",
			injectedErr: errors.New("u20c3 injected landed write failure"),
		},
		{
			name:        "unblock",
			operation:   "unblock",
			injectedErr: errors.New("u20c3 injected landed write failure"),
		},
		{
			name:        "error_shape_wrapping_shipment_conflict",
			operation:   "block",
			injectedErr: fmt.Errorf("u20c3 injected landed write failure: %w", blerrors.ErrShipmentConflict),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			root, ws := setupUR3Workspace(t)
			t.Cleanup(func() { require.NoError(t, ws.Close()) })
			fixture := newURBlockedActiveFixture(t, ws)

			var operationShipment *models.Artifact
			if tt.operation == "unblock" {
				var err error
				operationShipment, err = BlockShipment(ctx, ws, fixture.shipment.ID, BlockOptions{
					Reason:    "waiting for compensation-order recovery",
					BlockedBy: "U20C3",
				})
				require.NoError(t, err)
			} else {
				operationShipment = fixture.shipment
			}

			preimageShipment := cloneArtifact(loadURCanonicalArtifact(t, ws, operationShipment.ID))
			memberIDs := NormalizeShipmentItems(preimageShipment)
			require.NotEmpty(t, memberIDs)
			preimageMembers := make([]*models.Artifact, 0, len(memberIDs))
			activeMemberCount := 0
			for _, memberID := range memberIDs {
				member := cloneArtifact(loadURCanonicalArtifact(t, ws, memberID))
				preimageMembers = append(preimageMembers, member)
				if member.Status == models.StatusActive {
					activeMemberCount++
				}
			}
			if tt.operation == "block" {
				require.Greater(t, activeMemberCount, 0, "block fixture must have at least one active member")
			} else {
				_, err := validatePersistedBlockedShipmentEnvelope(ctx, ws, preimageShipment)
				require.NoError(t, err, "unblock preimage must have a valid blocked envelope")
			}

			targetStatus := models.StatusBlocked
			if tt.operation == "unblock" {
				targetStatus = models.StatusQueued
			}
			eventLogPath := events.LogPathForItem(WorkspaceLogsRoot(root), preimageShipment.ID)
			realWrite := persistArtifactWriteFn
			armed := false
			restoreObstruction := func() {}
			persistArtifactWriteFn = func(artifact *models.Artifact, path string, durable bool) error {
				if !armed &&
					artifact.ID == preimageShipment.ID &&
					artifact.ArtifactType == "shipment" &&
					artifact.Status == targetStatus {
					if err := realWrite(artifact, path, durable); err != nil {
						return err
					}
					armed = true
					restoreObstruction = armU20C3FilesystemObstruction(t, eventLogPath, true)
					return tt.injectedErr
				}
				return realWrite(artifact, path, durable)
			}
			t.Cleanup(func() { persistArtifactWriteFn = realWrite })

			var operationErr error
			if tt.operation == "block" {
				_, operationErr = BlockShipment(ctx, ws, preimageShipment.ID, BlockOptions{
					Reason:    "waiting for compensation-order recovery",
					BlockedBy: "U20C3",
				})
			} else {
				_, operationErr = UnblockShipment(ctx, ws, preimageShipment.ID, UnblockOptions{
					Target:      ShipmentQueued,
					Confirm:     true,
					UnblockedBy: "U20C3",
				})
			}

			assert.True(t, armed, "the first governed shipment write in the target status must arm the obstruction")
			assert.Error(t, operationErr)
			if operationErr != nil {
				assert.True(t, blerrors.IsWriteIndeterminate(operationErr))
				assert.Contains(t, operationErr.Error(), tt.injectedErr.Error())
				assert.Contains(t, operationErr.Error(), "append compensation status evidence")
				assert.False(t, errors.Is(operationErr, blerrors.ErrShipmentConflict))
				assert.False(t, errors.Is(operationErr, blerrors.ErrValidation))
				assert.False(t, errors.Is(operationErr, blerrors.ErrWriteNotApplied))
				assert.False(t, errors.Is(operationErr, blerrors.ErrNotFound))
				var partialErr *blerrors.MutationPartialError
				assert.False(t, errors.As(operationErr, &partialErr))
			}

			records, err := loadShipmentOperationJournals(ws)
			require.NoError(t, err)
			var journalPath string
			var journal shipmentLifecycleJournal
			for _, record := range records {
				if record.kind == shipmentLifecycleJournalKind &&
					record.lifecycle.ShipmentID == preimageShipment.ID &&
					record.lifecycle.Operation == tt.operation {
					journalPath = record.path
					journal = record.lifecycle
					break
				}
			}
			require.NotEmpty(t, journalPath, "failed lifecycle operation must retain its journal")
			assertURArtifactEqual(t, preimageShipment, journal.Preimage.Shipment)
			require.Len(t, journal.Preimage.Members, len(preimageMembers))
			preimageMembersByID := make(map[string]*models.Artifact, len(journal.Preimage.Members))
			for _, member := range journal.Preimage.Members {
				preimageMembersByID[member.ID] = member
			}
			for _, member := range preimageMembers {
				assertURArtifactEqual(t, member, preimageMembersByID[member.ID])
			}
			assert.Equal(t, "intent", journal.Phase, "failed status evidence must leave the durable intent")

			shipmentAfterFailure := cloneArtifact(loadURCanonicalArtifact(t, ws, preimageShipment.ID))
			assertURArtifactEqual(t, preimageShipment, shipmentAfterFailure)
			for _, member := range preimageMembers {
				current := cloneArtifact(loadURCanonicalArtifact(t, ws, member.ID))
				assertURArtifactEqual(t, member, current)
			}

			restoreObstruction()
			if tt.operation == "unblock" {
				_, validationErr := validatePersistedBlockedShipmentEnvelope(ctx, ws, shipmentAfterFailure)
				assert.NoError(t, validationErr, "failed unblock compensation must restore a valid blocked envelope")
			}

			recoveryErr := recoverPendingShipmentOperations(ctx, ws)
			assert.NoError(t, recoveryErr, "recovery must resolve the retained compensation intent")

			journalData, err := os.ReadFile(journalPath)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(journalData, &journal))
			assert.Equal(t, "compensated", journal.Phase)
			shipmentAfterRecovery := cloneArtifact(loadURCanonicalArtifact(t, ws, preimageShipment.ID))
			assertURArtifactEqual(t, preimageShipment, shipmentAfterRecovery)
			for _, member := range preimageMembers {
				assertURArtifactEqual(t, member, loadURCanonicalArtifact(t, ws, member.ID))
			}

			itemEvents := readUREvents(t, ws, preimageShipment.ID)
			terminalCount := 0
			appliedCount := 0
			lastAppliedTarget := ""
			appliedIndex := -1
			terminalIndex := -1
			for index, event := range itemEvents {
				if exactEventCorrelationUR(event) != journal.CorrelationID ||
					eventLifecycleOperationUR(event) != tt.operation {
					continue
				}
				if event.EventType == "shipment_status_changed" && eventPhaseUR(event) == "applied" {
					appliedCount++
					lastAppliedTarget = eventTargetUR(event)
					appliedIndex = index
				}
				if event.EventType == "shipment_lifecycle" && eventPhaseUR(event) == "compensated" {
					terminalCount++
					terminalIndex = index
				}
			}
			assert.Equal(t, 1, terminalCount, "exactly one correlated terminal compensated event must exist")
			assert.Equal(t, 1, appliedCount, "exactly one correlated compensated status event must exist")
			assert.Equal(t, string(preimageShipment.Status), lastAppliedTarget,
				"the last applied status target must be the shipment preimage")
			assert.GreaterOrEqual(t, appliedIndex, 0)
			assert.GreaterOrEqual(t, terminalIndex, 0)
			assert.Less(t, appliedIndex, terminalIndex, "compensated status evidence must precede its terminal event")

			report, doctorErr := Doctor(ctx, ws, &DoctorOptions{
				CheckOrphans:    false,
				CheckDuplicates: false,
			})
			assert.NoError(t, doctorErr)
			if report != nil {
				assert.False(t, hasUR3DoctorFinding(report, preimageShipment.ID, FindingTornShipmentLifecycleIntent))
				assert.False(t, hasUR3DoctorFinding(
					report,
					preimageShipment.ID,
					FindingConflictingShipmentLifecycleEvidence,
				))
			}
		})
	}
}

func TestU20C3_CompensationJournalFailureAfterTerminalEvidenceRecovers(t *testing.T) {
	tests := []struct {
		name        string
		operation   string
		injectedErr error
	}{
		{
			name:        "block",
			operation:   "block",
			injectedErr: errors.New("u20c3 injected landed write failure"),
		},
		{
			name:        "unblock",
			operation:   "unblock",
			injectedErr: errors.New("u20c3 injected landed write failure"),
		},
		{
			name:        "error_shape_wrapping_shipment_conflict",
			operation:   "block",
			injectedErr: fmt.Errorf("u20c3 injected landed write failure: %w", blerrors.ErrShipmentConflict),
		},
		{
			name:        "error_shape_wrapping_write_not_applied",
			operation:   "block",
			injectedErr: fmt.Errorf("u20c3 injected landed write failure: %w", blerrors.ErrWriteNotApplied),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			root, ws := setupUR3Workspace(t)
			t.Cleanup(func() { require.NoError(t, ws.Close()) })
			fixture := newURBlockedActiveFixture(t, ws)

			var operationShipment *models.Artifact
			if tt.operation == "unblock" {
				var err error
				operationShipment, err = BlockShipment(ctx, ws, fixture.shipment.ID, BlockOptions{
					Reason:    "waiting for compensation-order recovery",
					BlockedBy: "U20C3",
				})
				require.NoError(t, err)
			} else {
				operationShipment = fixture.shipment
			}

			preimageShipment := cloneArtifact(loadURCanonicalArtifact(t, ws, operationShipment.ID))
			memberIDs := NormalizeShipmentItems(preimageShipment)
			require.NotEmpty(t, memberIDs)
			preimageMembers := make([]*models.Artifact, 0, len(memberIDs))
			activeMemberCount := 0
			for _, memberID := range memberIDs {
				member := cloneArtifact(loadURCanonicalArtifact(t, ws, memberID))
				preimageMembers = append(preimageMembers, member)
				if member.Status == models.StatusActive {
					activeMemberCount++
				}
			}
			if tt.operation == "block" {
				require.Greater(t, activeMemberCount, 0, "block fixture must have at least one active member")
			} else {
				_, err := validatePersistedBlockedShipmentEnvelope(ctx, ws, preimageShipment)
				require.NoError(t, err, "unblock preimage must have a valid blocked envelope")
			}

			targetStatus := models.StatusBlocked
			if tt.operation == "unblock" {
				targetStatus = models.StatusQueued
			}
			opsRoot := shipmentOpsRoot(root)
			realWrite := persistArtifactWriteFn
			armed := false
			restoreObstruction := func() {}
			persistArtifactWriteFn = func(artifact *models.Artifact, path string, durable bool) error {
				if !armed &&
					artifact.ID == preimageShipment.ID &&
					artifact.ArtifactType == "shipment" &&
					artifact.Status == targetStatus {
					if err := realWrite(artifact, path, durable); err != nil {
						return err
					}
					armed = true
					restoreObstruction = armU20C3FilesystemObstruction(t, opsRoot, false)
					return tt.injectedErr
				}
				return realWrite(artifact, path, durable)
			}
			t.Cleanup(func() { persistArtifactWriteFn = realWrite })

			var operationErr error
			if tt.operation == "block" {
				_, operationErr = BlockShipment(ctx, ws, preimageShipment.ID, BlockOptions{
					Reason:    "waiting for compensation-order recovery",
					BlockedBy: "U20C3",
				})
			} else {
				_, operationErr = UnblockShipment(ctx, ws, preimageShipment.ID, UnblockOptions{
					Target:      ShipmentQueued,
					Confirm:     true,
					UnblockedBy: "U20C3",
				})
			}

			assert.True(t, armed, "the first governed shipment write in the target status must arm the obstruction")
			assert.Error(t, operationErr)
			if operationErr != nil {
				assert.True(t, blerrors.IsWriteIndeterminate(operationErr))
				assert.Contains(t, operationErr.Error(), tt.injectedErr.Error())
				assert.Contains(t, operationErr.Error(), "persist compensation journal")
				assert.False(t, errors.Is(operationErr, blerrors.ErrShipmentConflict))
				assert.False(t, errors.Is(operationErr, blerrors.ErrValidation))
				assert.False(t, errors.Is(operationErr, blerrors.ErrWriteNotApplied))
				assert.False(t, errors.Is(operationErr, blerrors.ErrNotFound))
				var partialErr *blerrors.MutationPartialError
				assert.False(t, errors.As(operationErr, &partialErr))
			}

			shipmentAfterFailure := cloneArtifact(loadURCanonicalArtifact(t, ws, preimageShipment.ID))
			membersAfterFailure := make([]*models.Artifact, 0, len(preimageMembers))
			for _, member := range preimageMembers {
				membersAfterFailure = append(
					membersAfterFailure,
					cloneArtifact(loadURCanonicalArtifact(t, ws, member.ID)),
				)
			}
			var blockedEnvelopeErr error
			if tt.operation == "unblock" {
				_, blockedEnvelopeErr = validatePersistedBlockedShipmentEnvelope(ctx, ws, shipmentAfterFailure)
			}
			eventsBeforeRecovery := readUREvents(t, ws, preimageShipment.ID)

			restoreObstruction()
			records, err := loadShipmentOperationJournals(ws)
			require.NoError(t, err)
			var journalPath string
			var journal shipmentLifecycleJournal
			for _, record := range records {
				if record.kind == shipmentLifecycleJournalKind &&
					record.lifecycle.ShipmentID == preimageShipment.ID &&
					record.lifecycle.Operation == tt.operation {
					journalPath = record.path
					journal = record.lifecycle
					break
				}
			}
			require.NotEmpty(t, journalPath, "failed lifecycle operation must retain its journal")
			assertURArtifactEqual(t, preimageShipment, journal.Preimage.Shipment)
			require.Len(t, journal.Preimage.Members, len(preimageMembers))
			preimageMembersByID := make(map[string]*models.Artifact, len(journal.Preimage.Members))
			for _, member := range journal.Preimage.Members {
				preimageMembersByID[member.ID] = member
			}
			for _, member := range preimageMembers {
				assertURArtifactEqual(t, member, preimageMembersByID[member.ID])
			}
			assert.Equal(t, "intent", journal.Phase, "failed journal persistence must retain the intent")

			assertURArtifactEqual(t, preimageShipment, shipmentAfterFailure)
			for index, member := range preimageMembers {
				assertURArtifactEqual(t, member, membersAfterFailure[index])
			}
			if tt.operation == "unblock" {
				assert.NoError(t, blockedEnvelopeErr, "failed unblock compensation must restore a valid blocked envelope")
			}

			terminalCount := 0
			appliedCount := 0
			appliedIndex := -1
			terminalIndex := -1
			lastAppliedTarget := ""
			for index, event := range eventsBeforeRecovery {
				if exactEventCorrelationUR(event) != journal.CorrelationID ||
					eventLifecycleOperationUR(event) != tt.operation {
					continue
				}
				if event.EventType == "shipment_status_changed" && eventPhaseUR(event) == "applied" {
					appliedCount++
					appliedIndex = index
					lastAppliedTarget = eventTargetUR(event)
				}
				if event.EventType == "shipment_lifecycle" && eventPhaseUR(event) == "compensated" {
					terminalCount++
					terminalIndex = index
				}
			}
			assert.Equal(t, 1, appliedCount, "compensated applied status evidence must be present before recovery")
			assert.Equal(t, 1, terminalCount, "exactly one terminal compensated event must be present before recovery")
			assert.Equal(t, string(preimageShipment.Status), lastAppliedTarget)
			assert.GreaterOrEqual(t, appliedIndex, 0)
			assert.GreaterOrEqual(t, terminalIndex, 0)
			assert.Less(t, appliedIndex, terminalIndex,
				"compensated status evidence must precede the terminal compensated event")

			recoveryErr := recoverPendingShipmentOperations(ctx, ws)
			assert.NoError(t, recoveryErr, "recovery must finalize the terminal compensated evidence")

			journalData, err := os.ReadFile(journalPath)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(journalData, &journal))
			assert.Equal(t, "compensated", journal.Phase)
			assertURArtifactEqual(t, preimageShipment, loadURCanonicalArtifact(t, ws, preimageShipment.ID))
			for _, member := range preimageMembers {
				assertURArtifactEqual(t, member, loadURCanonicalArtifact(t, ws, member.ID))
			}

			eventsAfterRecovery := readUREvents(t, ws, preimageShipment.ID)
			terminalCount = 0
			appliedCount = 0
			appliedIndex = -1
			terminalIndex = -1
			for index, event := range eventsAfterRecovery {
				if exactEventCorrelationUR(event) != journal.CorrelationID ||
					eventLifecycleOperationUR(event) != tt.operation {
					continue
				}
				if event.EventType == "shipment_status_changed" && eventPhaseUR(event) == "applied" {
					appliedCount++
					appliedIndex = index
				}
				if event.EventType == "shipment_lifecycle" && eventPhaseUR(event) == "compensated" {
					terminalCount++
					terminalIndex = index
				}
			}
			assert.Equal(t, 1, appliedCount, "recovery must not duplicate compensated status evidence")
			assert.Equal(t, 1, terminalCount, "recovery must not duplicate the terminal compensated event")
			assert.GreaterOrEqual(t, appliedIndex, 0)
			assert.GreaterOrEqual(t, terminalIndex, 0)
			assert.Less(t, appliedIndex, terminalIndex)

			report, doctorErr := Doctor(ctx, ws, &DoctorOptions{
				CheckOrphans:    false,
				CheckDuplicates: false,
			})
			assert.NoError(t, doctorErr)
			if report != nil {
				assert.False(t, hasUR3DoctorFinding(report, preimageShipment.ID, FindingTornShipmentLifecycleIntent))
				assert.False(t, hasUR3DoctorFinding(
					report,
					preimageShipment.ID,
					FindingConflictingShipmentLifecycleEvidence,
				))
			}
		})
	}
}

func armU20C3FilesystemObstruction(t *testing.T, path string, plantDirectory bool) func() {
	t.Helper()

	_, err := os.Lstat(path)
	require.NoError(t, err, "obstruction target must exist before it is displaced")
	asideDir, err := os.MkdirTemp(filepath.Dir(path), ".u20c3-displaced-")
	require.NoError(t, err)
	displacedPath := filepath.Join(asideDir, filepath.Base(path))

	var renameErr error
	for attempt := 0; attempt < 5; attempt++ {
		renameErr = os.Rename(path, displacedPath)
		if renameErr == nil {
			break
		}
		if attempt < 4 {
			time.Sleep(10 * time.Millisecond)
		}
	}
	require.NoError(t, renameErr, "bounded rename retries must move the original obstruction target aside")

	var restoreOnce sync.Once
	planted := false
	restore := func() {
		restoreOnce.Do(func() {
			if planted {
				plantedInfo, statErr := os.Lstat(path)
				require.NoError(t, statErr, "the planted obstruction must still occupy its target")
				if plantDirectory {
					require.True(t, plantedInfo.IsDir(), "only the planted directory may be removed")
					entries, readErr := os.ReadDir(path)
					require.NoError(t, readErr)
					require.Empty(t, entries, "do not recursively remove anything added to the planted directory")
				} else {
					require.True(t, plantedInfo.Mode().IsRegular(), "only the planted regular file may be removed")
					content, readErr := os.ReadFile(path)
					require.NoError(t, readErr)
					require.Equal(t, []byte("u20c3 planted obstruction"), content,
						"only the helper's planted file may be removed")
				}
				require.NoError(t, os.Remove(path), "remove only the planted obstruction entry")
			}
			require.NoError(t, os.Rename(displacedPath, path), "restore the original obstruction target")
			require.NoError(t, os.Remove(asideDir), "remove the now-empty displacement directory")
		})
	}
	t.Cleanup(restore)

	if plantDirectory {
		err = os.Mkdir(path, 0o755)
		if err == nil {
			planted = true
		}
		require.NoError(t, err, "plant the directory obstruction")
	} else {
		file, openErr := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		require.NoError(t, openErr, "plant the regular-file obstruction")
		planted = true
		_, writeErr := file.Write([]byte("u20c3 planted obstruction"))
		closeErr := file.Close()
		require.NoError(t, writeErr)
		require.NoError(t, closeErr)
	}

	plantedInfo, err := os.Lstat(path)
	require.NoError(t, err, "verify planted obstruction with Lstat")
	if plantDirectory {
		require.True(t, plantedInfo.IsDir(), "event log obstruction must be a directory")
	} else {
		require.True(t, plantedInfo.Mode().IsRegular(), "shipment-operation obstruction must be a regular file")
	}
	return restore
}
