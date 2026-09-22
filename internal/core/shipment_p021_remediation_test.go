package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

func TestP021ClaimSerialization_MembershipAndGenericWritersUseGlobalLock(t *testing.T) {
	t.Run("membership_writer_waits_and_deduplicates", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		ctx := context.Background()
		feature, err := CreateArtifact(ctx, ws, "P021 membership feature", "feature")
		require.NoError(t, err)
		member, err := CreateArtifact(ctx, ws, "P021 membership member", "task", WithParent(feature.ID))
		require.NoError(t, err)
		shipment, err := CreateShipment(ctx, ws, "P021 membership shipment", nil)
		require.NoError(t, err)

		unlock, err := lockShipmentMembership(ctx, ws, shipmentLifecycleGlobalLockID)
		require.NoError(t, err)
		result := make(chan error, 1)
		observedCtx, attempted, acquired := p021ObserveGlobalLock(ctx)
		go func() {
			result <- AddItemToShipment(observedCtx, ws, shipment.ID, member.ID)
		}()
		<-attempted

		var early error
		completedWhileLocked := false
		select {
		case <-acquired:
			t.Fatal("membership writer acquired the workspace-global lifecycle lock while its competitor held it")
		case early = <-result:
			completedWhileLocked = true
		case <-time.After(150 * time.Millisecond):
		}
		require.NoError(t, unlock())
		if !completedWhileLocked {
			select {
			case early = <-result:
			case <-time.After(defaultGateLockBoundedWait + 2*time.Second):
				t.Fatal("membership writer did not finish after the global lifecycle lock was released")
			}
		}

		require.False(t, completedWhileLocked, "membership writer bypassed the workspace-global lifecycle lock")
		require.NoError(t, early)
		require.NoError(t, AddItemToShipment(ctx, ws, shipment.ID, member.ID))
		got, err := GetShipment(ctx, ws, shipment.ID)
		require.NoError(t, err)
		require.Equal(t, []string{member.ID}, NormalizeShipmentItems(got),
			"serialized retries must not leave duplicate or partial membership")
		require.Equal(t, models.StatusQueued, loadURCanonicalArtifact(t, ws, member.ID).Status)
	})

	writers := []struct {
		name  string
		setup func(*testing.T, *Workspace) func(context.Context) error
	}{
		{
			name: "generic_update",
			setup: func(t *testing.T, ws *Workspace) func(context.Context) error {
				t.Helper()
				shipment, err := CreateShipment(context.Background(), ws, "P021 generic update", nil)
				require.NoError(t, err)
				return func(ctx context.Context) error {
					_, updateErr := UpdateArtifact(ctx, ws, shipment.ID, map[string]any{
						"title": "P021 generic update completed",
					})
					return updateErr
				}
			},
		},
		{
			name: "bulk_update",
			setup: func(t *testing.T, ws *Workspace) func(context.Context) error {
				t.Helper()
				feature, err := CreateArtifact(context.Background(), ws, "P021 bulk feature", "feature")
				require.NoError(t, err)
				member, err := CreateArtifact(
					context.Background(),
					ws,
					"P021 bulk member",
					"task",
					WithParent(feature.ID),
				)
				require.NoError(t, err)
				return func(ctx context.Context) error {
					result, bulkErr := BulkUpdateStatus(
						ctx,
						ws.DB,
						ws,
						[]string{member.ID},
						string(models.StatusActive),
					)
					if bulkErr != nil {
						return bulkErr
					}
					if len(result.Failed) != 0 || result.Succeeded != 1 {
						return errors.New("bulk update did not complete exactly once")
					}
					return nil
				}
			},
		},
		{
			name: "cascade_update",
			setup: func(t *testing.T, ws *Workspace) func(context.Context) error {
				t.Helper()
				feature, err := CreateArtifact(context.Background(), ws, "P021 cascade feature", "feature")
				require.NoError(t, err)
				member, err := CreateArtifact(
					context.Background(),
					ws,
					"P021 cascade member",
					"task",
					WithParent(feature.ID),
					WithStatus(string(models.StatusActive)),
				)
				require.NoError(t, err)
				return func(ctx context.Context) error {
					return cascadePersistedParentStatuses(ctx, ws, member.ID)
				}
			},
		},
	}

	for _, writer := range writers {
		t.Run(writer.name+"_waits", func(t *testing.T) {
			ws := setupShipmentWorkspace(t)
			action := writer.setup(t, ws)
			ctx := context.Background()
			unlock, err := lockShipmentMembership(ctx, ws, shipmentLifecycleGlobalLockID)
			require.NoError(t, err)

			result := make(chan error, 1)
			observedCtx, attempted, acquired := p021ObserveGlobalLock(ctx)
			go func() {
				result <- action(observedCtx)
			}()
			<-attempted

			var actionErr error
			completedWhileLocked := false
			select {
			case <-acquired:
				t.Fatalf("%s acquired the workspace-global lifecycle lock while its competitor held it", writer.name)
			case actionErr = <-result:
				completedWhileLocked = true
			case <-time.After(150 * time.Millisecond):
			}
			require.NoError(t, unlock())
			if !completedWhileLocked {
				select {
				case actionErr = <-result:
				case <-time.After(defaultGateLockBoundedWait + 2*time.Second):
					t.Fatalf("%s did not finish after the global lifecycle lock was released", writer.name)
				}
			}

			require.False(t, completedWhileLocked, "%s bypassed the workspace-global lifecycle lock", writer.name)
			require.NoError(t, actionErr)
		})
	}
}

func TestP1C4_BlockedShipmentMembershipIsImmutable(t *testing.T) {
	tests := []struct {
		name string
		act  func(context.Context, *Workspace, urBlockedFixture, *models.Artifact) error
	}{
		{
			name: "AddItemToShipment",
			act: func(ctx context.Context, ws *Workspace, fixture urBlockedFixture, candidate *models.Artifact) error {
				return AddItemToShipment(ctx, ws, fixture.shipment.ID, candidate.ID)
			},
		},
		{
			name: "ReturnBlockedItem",
			act: func(ctx context.Context, ws *Workspace, fixture urBlockedFixture, _ *models.Artifact) error {
				return ReturnBlockedItem(ctx, ws, fixture.shipment.ID, fixture.members[0].ID, "must remain sealed")
			},
		},
		{
			name: "UpdateArtifact_custom_fields",
			act: func(ctx context.Context, ws *Workspace, fixture urBlockedFixture, _ *models.Artifact) error {
				_, err := UpdateArtifact(ctx, ws, fixture.shipment.ID, map[string]any{
					"custom_fields": map[string]any{
						"items": []string{fixture.members[0].ID},
					},
				})
				return err
			},
		},
		{
			name: "private_generic_persist",
			act: func(ctx context.Context, ws *Workspace, fixture urBlockedFixture, _ *models.Artifact) error {
				blocked := cloneArtifact(loadURCanonicalArtifact(t, ws, fixture.shipment.ID))
				blocked.CustomFields["items"] = []string{fixture.members[0].ID}
				blocked.UpdatedAt = models.NowUTC()
				return persistArtifact(ctx, ws, blocked, false)
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ws := setupShipmentWorkspace(t)
			ctx := context.Background()
			fixture := newURBlockedActiveFixture(t, ws)
			feature, err := CreateArtifact(ctx, ws, "C4 candidate feature", "feature")
			require.NoError(t, err)
			candidate, err := CreateArtifact(ctx, ws, "C4 candidate member", "task", WithParent(feature.ID))
			require.NoError(t, err)
			_, err = BlockShipment(ctx, ws, fixture.shipment.ID, BlockOptions{
				Reason:    "C4 sealed aggregate",
				BlockedBy: "C4 test",
			})
			require.NoError(t, err)
			before := snapshotURAggregate(t, ws, fixture.shipment.ID)

			err = test.act(ctx, ws, fixture, candidate)

			require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
			requireURAggregateUnchanged(t, ws, before)
		})
	}
}

func TestP1C6_ClaimCompensationDetachesCancellationAndRestoresExactly(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx, cancel := context.WithCancel(context.Background())
	feature, err := CreateArtifact(ctx, ws, "C6 claim feature", "feature")
	require.NoError(t, err)
	parent, err := CreateArtifact(ctx, ws, "C6 claim parent", "task", WithParent(feature.ID))
	require.NoError(t, err)
	member, err := CreateArtifact(ctx, ws, "C6 claim member", "subtask", WithParent(parent.ID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "C6 claim shipment", []string{member.ID})
	require.NoError(t, err)
	before := snapshotURGovernedState(t, ws, []string{shipment.ID, feature.ID, parent.ID, member.ID})

	originalWriter := persistArtifactWriteFn
	persistArtifactWriteFn = func(artifact *models.Artifact, filePath string, durable bool) error {
		if artifact.ID == feature.ID && artifact.Status == models.StatusActive {
			cancel()
			return context.Canceled
		}
		return originalWriter(artifact, filePath, durable)
	}
	t.Cleanup(func() { persistArtifactWriteFn = originalWriter })

	_, err = ClaimShipment(ctx, ws, shipment.ID)

	require.ErrorIs(t, err, context.Canceled)
	requireP1C6DurableStateRestored(t, ws, before)
}

func TestP1C6_ReturnBlockedCompensationDetachesCancellationAndRestoresExactly(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx, cancel := context.WithCancel(context.Background())
	feature, err := CreateArtifact(ctx, ws, "C6 return feature", "feature")
	require.NoError(t, err)
	member, err := CreateArtifact(ctx, ws, "C6 return member", "task", WithParent(feature.ID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "C6 return shipment", []string{member.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	before := snapshotURAggregate(t, ws, shipment.ID)

	originalWriter := persistArtifactWriteFn
	persistArtifactWriteFn = func(artifact *models.Artifact, filePath string, durable bool) error {
		if artifact.ID == member.ID && artifact.Status == models.StatusBlocked {
			cancel()
			return context.Canceled
		}
		return originalWriter(artifact, filePath, durable)
	}
	t.Cleanup(func() { persistArtifactWriteFn = originalWriter })

	err = ReturnBlockedItem(ctx, ws, shipment.ID, member.ID, "C6 injected cancellation")

	require.ErrorIs(t, err, context.Canceled)
	requireP1C6DurableStateRestored(t, ws, before)
}

func TestP1C6_ClaimCompensationFailureIsClassifiedAndRecoverable(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx, cancel := context.WithCancel(context.Background())
	feature, err := CreateArtifact(ctx, ws, "C6 claim recovery feature", "feature")
	require.NoError(t, err)
	parent, err := CreateArtifact(ctx, ws, "C6 claim recovery parent", "task", WithParent(feature.ID))
	require.NoError(t, err)
	member, err := CreateArtifact(ctx, ws, "C6 claim recovery member", "subtask", WithParent(parent.ID))
	require.NoError(t, err)
	appendItemEvent(ctx, ws, parent.ID, "c6_baseline", map[string]any{"preserve": true})
	shipment, err := CreateShipment(ctx, ws, "C6 claim recovery shipment", []string{member.ID})
	require.NoError(t, err)
	parentEventLogPath := events.LogPathForItem(WorkspaceLogsRoot(ws.RootPath), parent.ID)
	before := snapshotURGovernedState(t, ws, []string{shipment.ID, feature.ID, parent.ID, member.ID})

	originalWriter := persistArtifactWriteFn
	persistArtifactWriteFn = func(artifact *models.Artifact, filePath string, durable bool) error {
		if artifact.ID == feature.ID && artifact.Status == models.StatusActive {
			cancel()
			return context.Canceled
		}
		return originalWriter(artifact, filePath, durable)
	}
	t.Cleanup(func() { persistArtifactWriteFn = originalWriter })

	injectedRecoveryErr := errors.New("injected claim compensation failure")
	originalRestore := restoreShipmentSnapshotFn
	failedRestore := false
	restoreShipmentSnapshotFn = func(snapshot fileSnapshot) error {
		if !failedRestore && filepath.Clean(snapshot.Path) == filepath.Clean(parentEventLogPath) {
			failedRestore = true
			return injectedRecoveryErr
		}
		return originalRestore(snapshot)
	}
	t.Cleanup(func() { restoreShipmentSnapshotFn = originalRestore })

	_, err = ClaimShipment(ctx, ws, shipment.ID)

	var partial *blerrors.MutationPartialError
	require.ErrorAs(t, err, &partial)
	require.Equal(t, "double-fault", partial.Class)
	require.Equal(t, "partially-compensated", partial.CompensationState)
	require.ErrorIs(t, err, injectedRecoveryErr)
	requireClaimRecoveryIntent(t, ws, shipment.ID)

	persistArtifactWriteFn = originalWriter
	restoreShipmentSnapshotFn = originalRestore
	require.NoError(t, recoverPendingShipmentOperations(context.Background(), ws))
	requireP1C6DurableStateRestored(t, ws, before)
}

func TestP1C6_ReturnBlockedCompensationFailureIsClassifiedAndRecoverable(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx, cancel := context.WithCancel(context.Background())
	feature, err := CreateArtifact(ctx, ws, "C6 return recovery feature", "feature")
	require.NoError(t, err)
	member, err := CreateArtifact(ctx, ws, "C6 return recovery member", "task", WithParent(feature.ID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "C6 return recovery shipment", []string{member.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	before := snapshotURAggregate(t, ws, shipment.ID)

	injectedRecoveryErr := errors.New("injected return compensation failure")
	originalWriter := persistArtifactWriteFn
	forwardFailed := false
	persistArtifactWriteFn = func(artifact *models.Artifact, filePath string, durable bool) error {
		switch {
		case artifact.ID == member.ID && artifact.Status == models.StatusBlocked:
			forwardFailed = true
			cancel()
			return context.Canceled
		case forwardFailed && artifact.ID == shipment.ID:
			return injectedRecoveryErr
		default:
			return originalWriter(artifact, filePath, durable)
		}
	}
	t.Cleanup(func() { persistArtifactWriteFn = originalWriter })

	err = ReturnBlockedItem(ctx, ws, shipment.ID, member.ID, "C6 durable recovery")

	var partial *blerrors.MutationPartialError
	require.ErrorAs(t, err, &partial)
	require.Equal(t, "double-fault", partial.Class)
	require.Equal(t, "partially-compensated", partial.CompensationState)
	require.ErrorIs(t, err, injectedRecoveryErr)
	_, statErr := os.Stat(returnBlockedJournalPath(ws.RootPath, shipment.ID, member.ID))
	require.NoError(t, statErr, "failed compensation must retain its durable recovery journal")

	persistArtifactWriteFn = originalWriter
	require.NoError(t, recoverPendingShipmentOperations(context.Background(), ws))
	requireP1C6DurableStateRestored(t, ws, before)
}

func requireClaimRecoveryIntent(t *testing.T, ws *Workspace, shipmentID string) {
	t.Helper()

	records, err := loadShipmentOperationJournals(ws)
	require.NoError(t, err)
	for _, record := range records {
		if record.kind == shipmentLifecycleJournalKind &&
			record.lifecycle.Operation == "claim" &&
			record.lifecycle.ShipmentID == shipmentID &&
			record.lifecycle.Phase == "intent" {
			return
		}
	}
	require.FailNow(t, "claim recovery intent missing", "shipment %s has no durable claim intent", shipmentID)
}

func requireP1C6DurableStateRestored(t *testing.T, ws *Workspace, before urAggregateSnapshot) {
	t.Helper()

	after := snapshotURGovernedState(t, ws, before.ArtifactIDs)
	for id, want := range before.Artifacts {
		assertURArtifactEqual(t, want, after.Artifacts[id])
	}
	require.Equal(t, before.CanonicalFiles, after.CanonicalFiles)
	require.Equal(t, before.ArtifactLocations, after.ArtifactLocations)
	require.Equal(t, before.EventLogs, after.EventLogs)
	require.Equal(t, before.OperationJournals, after.OperationJournals)
}

func p021ObserveGlobalLock(ctx context.Context) (context.Context, <-chan struct{}, <-chan struct{}) {
	attempted := make(chan struct{}, 1)
	acquired := make(chan struct{}, 1)
	hook := func(phase string) {
		switch phase {
		case "attempt":
			attempted <- struct{}{}
		case "acquired":
			acquired <- struct{}{}
		}
	}
	return context.WithValue(ctx, shipmentLifecycleGlobalLockHookContextKey{}, hook), attempted, acquired
}

func TestP1C7_BlockedEnvelopeRequiresGovernedProvenance(t *testing.T) {
	t.Run("unblock_refuses_plausible_out_of_band_record", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		fixture := newURBlockedActiveFixture(t, ws)
		p021ForceOutOfBandBlockedShipment(t, ws, fixture)
		before := snapshotURAggregate(t, ws, fixture.shipment.ID)

		_, err := UnblockShipment(context.Background(), ws, fixture.shipment.ID, UnblockOptions{
			Target:      ShipmentActive,
			Confirm:     true,
			UnblockedBy: "C7 test",
		})
		require.ErrorIs(t, err, blerrors.ErrShipmentBlockedRequiresEnvelope)
		requireURAggregateUnchanged(t, ws, before)
	})

	t.Run("normalizer_establishes_provenance_before_unblock", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		fixture := newURBlockedActiveFixture(t, ws)
		p021ForceOutOfBandBlockedShipment(t, ws, fixture)
		snapshotRef := p021WriteBlockedSnapshot(t, ws, fixture.shipment.ID)

		normalized, err := NormalizeBlockedShipment(
			context.Background(),
			ws,
			fixture.shipment.ID,
			snapshotRef,
			"C7 normalizer",
		)
		require.NoError(t, err)
		require.Equal(t, models.StatusBlocked, normalized.Status)

		unblocked, err := UnblockShipment(context.Background(), ws, fixture.shipment.ID, UnblockOptions{
			Target:      ShipmentActive,
			Confirm:     true,
			UnblockedBy: "C7 test",
		})
		require.NoError(t, err)
		require.Equal(t, models.StatusActive, unblocked.Status)
	})

	t.Run("doctor_reports_plausible_out_of_band_record", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		fixture := newURBlockedActiveFixture(t, ws)
		p021ForceOutOfBandBlockedShipment(t, ws, fixture)

		report, err := Doctor(context.Background(), ws, &DoctorOptions{})
		require.NoError(t, err)
		require.Contains(t, report.Findings, DoctorFinding{
			Type:        FindingMalformedBlockedShipment,
			Severity:    DoctorSeverityError,
			Code:        FindingMalformedBlockedShipment,
			ArtifactID:  fixture.shipment.ID,
			Description: "blocked shipment \"" + fixture.shipment.ID + "\" lacks a canonical governed block/normalize envelope; run the blocked-shipment normalizer",
			Message:     "blocked shipment \"" + fixture.shipment.ID + "\" lacks a canonical governed block/normalize envelope; run the blocked-shipment normalizer",
		})
	})
}

func TestP1C7_BlockedEnvelopeRequiresCanonicalMetadata(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*models.Artifact)
	}{
		{
			name: "reason_must_match_evidence",
			mutate: func(shipment *models.Artifact) {
				shipment.CustomFields["blocked_reason"] = "tampered reason"
			},
		},
		{
			name: "blocked_at_must_be_rfc3339",
			mutate: func(shipment *models.Artifact) {
				shipment.CustomFields["blocked_at"] = "not-a-timestamp"
			},
		},
		{
			name: "blocked_by_must_be_present",
			mutate: func(shipment *models.Artifact) {
				delete(shipment.CustomFields, "blocked_by")
			},
		},
		{
			name: "branch_must_be_a_non_empty_string_when_present",
			mutate: func(shipment *models.Artifact) {
				shipment.CustomFields["branch"] = 7
			},
		},
		{
			name: "resume_reference_must_be_non_empty_when_present",
			mutate: func(shipment *models.Artifact) {
				shipment.CustomFields["resume_checkpoint_ref"] = ""
			},
		},
		{
			name: "snapshot_must_exactly_cover_manifest",
			mutate: func(shipment *models.Artifact) {
				snapshot := statusSnapshotUR(shipment.CustomFields["member_status_snapshot"])
				snapshot["999.999-T"] = string(models.StatusQueued)
				shipment.CustomFields["member_status_snapshot"] = snapshot
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ws := setupShipmentWorkspace(t)
			fixture := newURBlockedActiveFixture(t, ws)
			shipment := cloneArtifact(loadURCanonicalArtifact(t, ws, fixture.shipment.ID))
			shipment.CustomFields["branch"] = "feat/c7-envelope"
			forceURArtifactFixture(t, ws, shipment)
			_, err := BlockShipment(context.Background(), ws, fixture.shipment.ID, BlockOptions{
				Reason:              "canonical C7 blocker",
				BlockedBy:           "C7 actor",
				ResumeCheckpointRef: "c7-resume.json",
			})
			require.NoError(t, err)

			tampered := cloneArtifact(loadURCanonicalArtifact(t, ws, fixture.shipment.ID))
			test.mutate(tampered)
			tampered.UpdatedAt = models.NowUTC()
			forceURArtifactFixture(t, ws, tampered)
			before := snapshotURAggregate(t, ws, fixture.shipment.ID)

			_, err = UnblockShipment(context.Background(), ws, fixture.shipment.ID, UnblockOptions{
				Target:      ShipmentActive,
				Confirm:     true,
				UnblockedBy: "C7 test",
			})
			require.ErrorIs(t, err, blerrors.ErrShipmentBlockedRequiresEnvelope)
			requireURAggregateUnchanged(t, ws, before)
		})
	}
}

func TestP1C8_GenericShipmentCreateRejectsBlockedStatus(t *testing.T) {
	tests := []struct {
		name   string
		create func(context.Context, *Workspace) (*models.Artifact, error)
	}{
		{
			name: "generic_artifact_create",
			create: func(ctx context.Context, ws *Workspace) (*models.Artifact, error) {
				return CreateArtifact(ctx, ws, "C8 generic blocked", "shipment", WithStatus("blocked"))
			},
		},
		{
			name: "shipment_create",
			create: func(ctx context.Context, ws *Workspace) (*models.Artifact, error) {
				return CreateShipment(ctx, ws, "C8 blocked shipment", nil, WithStatus("blocked"))
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ws := setupShipmentWorkspace(t)

			_, err := test.create(context.Background(), ws)
			require.ErrorIs(t, err, blerrors.ErrShipmentBlockedRequiresEnvelope)
		})
	}
}

func TestP1C8_GenericShipmentCreatePreservesOrdinaryAllowedStatuses(t *testing.T) {
	tests := []struct {
		name   string
		status models.ArtifactStatus
	}{
		{name: "default_queued", status: models.StatusQueued},
		{name: "explicit_abandoned", status: models.StatusAbandoned},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			ws := setupShipmentWorkspace(t)
			opts := []Option{}
			if test.status != models.StatusQueued {
				opts = append(opts, WithStatus(string(test.status)))
			}

			created, err := CreateArtifact(
				context.Background(),
				ws,
				"C8 allowed "+test.name,
				"shipment",
				opts...,
			)
			require.NoError(t, err)
			require.Equal(t, test.status, created.Status)
		})
	}
}

func p021ForceOutOfBandBlockedShipment(t *testing.T, ws *Workspace, fixture urBlockedFixture) {
	t.Helper()

	shipment := cloneArtifact(loadURCanonicalArtifact(t, ws, fixture.shipment.ID))
	memberStatuses := make(map[string]string, len(fixture.members))
	for _, memberID := range NormalizeShipmentItems(shipment) {
		member := cloneArtifact(loadURCanonicalArtifact(t, ws, memberID))
		memberStatuses[memberID] = string(member.Status)
		if member.Status == models.StatusActive || member.Status == models.StatusReview {
			member.Status = models.StatusQueued
			member.UpdatedAt = models.NowUTC()
			forceURArtifactFixture(t, ws, member)
		}
	}

	shipment.Status = models.StatusBlocked
	shipment.UpdatedAt = models.NowUTC()
	if shipment.CustomFields == nil {
		shipment.CustomFields = map[string]any{}
	}
	shipment.CustomFields["blocked_reason"] = "plausible imported blocker"
	shipment.CustomFields["blocked_at"] = time.Now().UTC().Format(time.RFC3339)
	shipment.CustomFields["blocked_by"] = "out-of-band importer"
	shipment.CustomFields["branch"] = "feat/imported-blocked-record"
	shipment.CustomFields["resume_checkpoint_ref"] = "imported-resume.json"
	shipment.CustomFields["member_status_snapshot"] = memberStatuses
	forceURArtifactFixture(t, ws, shipment)
}

func TestP021RecoveryCAS_RefusesDriftAndRestoresExactPreimage(t *testing.T) {
	t.Run("rollback_refuses_member_drift_without_mutation", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		fixture := newURBlockedActiveFixture(t, ws)
		journal := p021LifecycleJournal(t, ws, "block", "rollback", fixture.shipment.ID, "")
		journalPath := p021WriteLifecycleJournal(t, ws, journal)

		drifted := cloneArtifact(journal.Preimage.Members[0])
		drifted.Status = models.StatusQueued
		drifted.Title = "post-crash member edit"
		drifted.UpdatedAt = models.NowUTC()
		forceURArtifactFixture(t, ws, drifted)
		before := snapshotURAggregate(t, ws, fixture.shipment.ID)

		err := recoverPendingShipmentOperations(context.Background(), ws)
		require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
		requireURAggregateUnchanged(t, ws, before)
		require.Equal(t, "intent", p021ReadLifecycleJournal(t, journalPath).Phase)
	})

	t.Run("unblock_rollback_refuses_shipment_drift_without_mutation", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		fixture := newURBlockedActiveFixture(t, ws)
		_, err := BlockShipment(context.Background(), ws, fixture.shipment.ID, BlockOptions{
			Reason:    "P021 blocked",
			BlockedBy: "P021",
		})
		require.NoError(t, err)
		journal := p021LifecycleJournal(t, ws, "unblock", "rollback", fixture.shipment.ID, "")
		journal.Target = string(ShipmentActive)
		journalPath := p021WriteLifecycleJournal(t, ws, journal)

		drifted := cloneArtifact(journal.Preimage.Shipment)
		drifted.Status = models.StatusActive
		drifted.Title = "post-crash shipment edit"
		delete(drifted.CustomFields, "blocked_reason")
		delete(drifted.CustomFields, "blocked_at")
		delete(drifted.CustomFields, "blocked_by")
		drifted.UpdatedAt = models.NowUTC()
		forceURArtifactFixture(t, ws, drifted)
		before := snapshotURAggregate(t, ws, fixture.shipment.ID)

		err = recoverPendingShipmentOperations(context.Background(), ws)
		require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
		requireURAggregateUnchanged(t, ws, before)
		require.Equal(t, "intent", p021ReadLifecycleJournal(t, journalPath).Phase)
	})

	t.Run("normalizer_roll_forward_refuses_member_drift_without_mutation", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		fixture := newURBlockedActiveFixture(t, ws)
		_, err := BlockShipment(context.Background(), ws, fixture.shipment.ID, BlockOptions{
			Reason:    "P021 normalize source",
			BlockedBy: "P021",
		})
		require.NoError(t, err)
		snapshotRef := p021WriteBlockedSnapshot(t, ws, fixture.shipment.ID)
		journal := p021LifecycleJournal(t, ws, "normalize", "roll_forward", fixture.shipment.ID, snapshotRef)
		journalPath := p021WriteLifecycleJournal(t, ws, journal)

		drifted := cloneArtifact(journal.Preimage.Members[0])
		drifted.Title = "post-crash normalizer edit"
		drifted.UpdatedAt = models.NowUTC()
		forceURArtifactFixture(t, ws, drifted)
		before := snapshotURAggregate(t, ws, fixture.shipment.ID)

		err = recoverPendingShipmentOperations(context.Background(), ws)
		require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
		requireURAggregateUnchanged(t, ws, before)
		require.Equal(t, "intent", p021ReadLifecycleJournal(t, journalPath).Phase)
	})

	t.Run("bootstrap_roll_forward_refuses_shipment_drift_without_mutation", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		fixture := newURBlockedActiveFixture(t, ws)
		snapshotRef := p021WriteBootstrapSnapshot(t, ws, fixture.shipment.ID)
		journal := p021LifecycleJournal(t, ws, "block", "roll_forward", fixture.shipment.ID, snapshotRef)
		journalPath := p021WriteLifecycleJournal(t, ws, journal)

		drifted := cloneArtifact(journal.Preimage.Shipment)
		drifted.Title = "post-intent bootstrap edit"
		drifted.UpdatedAt = models.NowUTC()
		forceURArtifactFixture(t, ws, drifted)
		before := snapshotURAggregate(t, ws, fixture.shipment.ID)

		err := recoverPendingShipmentOperations(context.Background(), ws)
		require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
		requireURAggregateUnchanged(t, ws, before)
		require.Equal(t, "intent", p021ReadLifecycleJournal(t, journalPath).Phase)
	})

	t.Run("rollback_restores_exact_preimage_when_cas_holds", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		fixture := newURBlockedActiveFixture(t, ws)
		journal := p021LifecycleJournal(t, ws, "block", "rollback", fixture.shipment.ID, "")
		journalPath := p021WriteLifecycleJournal(t, ws, journal)

		torn := cloneArtifact(journal.Preimage.Members[0])
		torn.Status = models.StatusQueued
		torn.UpdatedAt = models.NowUTC()
		forceURArtifactFixture(t, ws, torn)

		require.NoError(t, recoverPendingShipmentOperations(context.Background(), ws))
		assertURArtifactEqual(t, journal.Preimage.Shipment, loadURCanonicalArtifact(t, ws, fixture.shipment.ID))
		for _, member := range journal.Preimage.Members {
			assertURArtifactEqual(t, member, loadURCanonicalArtifact(t, ws, member.ID))
		}
		require.Equal(t, "compensated", p021ReadLifecycleJournal(t, journalPath).Phase)
	})
}

func p021LifecycleJournal(
	t *testing.T,
	ws *Workspace,
	operation string,
	recoveryPolicy string,
	shipmentID string,
	snapshotRef string,
) shipmentLifecycleJournal {
	t.Helper()

	shipment := cloneArtifact(loadURCanonicalArtifact(t, ws, shipmentID))
	memberIDs := NormalizeShipmentItems(shipment)
	members := make([]*models.Artifact, 0, len(memberIDs))
	for _, memberID := range memberIDs {
		members = append(members, cloneArtifact(loadURCanonicalArtifact(t, ws, memberID)))
	}
	return shipmentLifecycleJournal{
		SchemaVersion:  "shipment-operation/v1",
		CorrelationID:  "p021-" + operation,
		Phase:          "intent",
		Operation:      operation,
		RecoveryPolicy: recoveryPolicy,
		ShipmentID:     shipmentID,
		Target:         string(ShipmentBlocked),
		Reason:         "P021 recovery",
		BlockedBy:      "P021",
		SnapshotRef:    snapshotRef,
		Preimage: shipmentLifecyclePreimage{
			Shipment: shipment,
			Members:  members,
		},
	}
}

func p021WriteLifecycleJournal(t *testing.T, ws *Workspace, journal shipmentLifecycleJournal) string {
	t.Helper()

	journal.CorrelationID = p021CorrelationID(journal.Operation)
	path, err := writeShipmentLifecycleJournalForWorkspace(
		ws,
		shipmentLifecycleJournalName(journal.CorrelationID),
		journal,
	)
	require.NoError(t, err)
	return path
}

func p021CorrelationID(operation string) string {
	switch operation {
	case "block":
		return "11111111111111111111111111111111"
	case "unblock":
		return "22222222222222222222222222222222"
	case "normalize":
		return "33333333333333333333333333333333"
	default:
		return "ffffffffffffffffffffffffffffffff"
	}
}

func p021ReadLifecycleJournal(t *testing.T, path string) shipmentLifecycleJournal {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	var journal shipmentLifecycleJournal
	require.NoError(t, json.Unmarshal(data, &journal))
	return journal
}

func p021WriteBootstrapSnapshot(t *testing.T, ws *Workspace, shipmentID string) string {
	t.Helper()

	shipment := loadURCanonicalArtifact(t, ws, shipmentID)
	memberStatuses := make(map[string]string)
	for _, memberID := range NormalizeShipmentItems(shipment) {
		memberStatuses[memberID] = string(loadURCanonicalArtifact(t, ws, memberID).Status)
	}
	snapshot := ShipmentBlockedSnapshot{
		SchemaVersion: ShipmentBlockedSnapshotSchemaVersion,
		ShipmentID:    shipmentID,
		Target:        ShipmentBlocked,
		BlockedReason: "P021 bootstrap recovery",
		BlockedAt:     time.Now().UTC().Format(time.RFC3339),
		BlockedBy:     "P021",
		Members:       memberStatuses,
	}
	data, err := json.Marshal(snapshot)
	require.NoError(t, err)
	relativePath := filepath.Join("bootstrap", "p021-"+shipmentID+".json")
	absolutePath := filepath.Join(WorkspaceStorageRoot(ws.RootPath), relativePath)
	require.NoError(t, os.MkdirAll(filepath.Dir(absolutePath), 0o755))
	require.NoError(t, os.WriteFile(absolutePath, data, 0o644))
	return relativePath
}

func p021WriteBlockedSnapshot(t *testing.T, ws *Workspace, shipmentID string) string {
	t.Helper()

	shipment := loadURCanonicalArtifact(t, ws, shipmentID)
	memberStatuses := statusSnapshotUR(shipment.CustomFields["member_status_snapshot"])
	snapshot := ShipmentBlockedSnapshot{
		SchemaVersion: ShipmentBlockedSnapshotSchemaVersion,
		ShipmentID:    shipmentID,
		Target:        ShipmentBlocked,
		BlockedReason: "P021 recovered blocked shipment",
		BlockedAt:     time.Now().UTC().Format(time.RFC3339),
		BlockedBy:     "P021",
		Members:       memberStatuses,
	}
	data, err := json.Marshal(snapshot)
	require.NoError(t, err)
	relativePath := filepath.Join("snapshots", "p021-"+shipmentID+".json")
	absolutePath := filepath.Join(WorkspaceStorageRoot(ws.RootPath), relativePath)
	require.NoError(t, os.MkdirAll(filepath.Dir(absolutePath), 0o755))
	require.NoError(t, os.WriteFile(absolutePath, data, 0o644))
	return filepath.ToSlash(relativePath)
}
