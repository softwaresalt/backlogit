package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

// u1Fixture holds a workspace with two queued shipments:
//
//	shipA = [feat, task1, task2]
//	shipB = [feat2, task3]
type u1Fixture struct {
	ws                *Workspace
	feat, feat2       *models.Artifact
	task1, task2      *models.Artifact
	task3             *models.Artifact
	shipA, shipB      *models.Artifact
	ctx               context.Context
	originalPersistFn func(*models.Artifact, string, bool) error
}

func newU1Fixture(t *testing.T) *u1Fixture {
	t.Helper()
	ctx := context.Background()
	ws := setupShipmentWorkspace(t)

	feat, err := CreateArtifact(ctx, ws, "U1 feature", "feature")
	require.NoError(t, err)
	task1, err := CreateArtifact(ctx, ws, "U1 task 1", "task", WithParent(feat.ID))
	require.NoError(t, err)
	task2, err := CreateArtifact(ctx, ws, "U1 task 2", "task", WithParent(feat.ID))
	require.NoError(t, err)
	feat2, err := CreateArtifact(ctx, ws, "U1 other feature", "feature")
	require.NoError(t, err)
	task3, err := CreateArtifact(ctx, ws, "U1 task 3", "task", WithParent(feat2.ID))
	require.NoError(t, err)

	shipA, err := CreateShipment(ctx, ws, "U1 ship A", []string{feat.ID, task1.ID, task2.ID})
	require.NoError(t, err)
	shipB, err := CreateShipment(ctx, ws, "U1 ship B", []string{feat2.ID, task3.ID})
	require.NoError(t, err)

	fx := &u1Fixture{
		ws: ws, ctx: ctx,
		feat: feat, feat2: feat2, task1: task1, task2: task2, task3: task3,
		shipA: shipA, shipB: shipB,
		originalPersistFn: persistArtifactWriteFn,
	}
	t.Cleanup(func() {
		persistArtifactWriteFn = fx.originalPersistFn
		ws.shipmentEventAppend = nil
		ws.removeShipmentOperationJournal = nil
	})
	return fx
}

func (fx *u1Fixture) claimA(t *testing.T) {
	t.Helper()
	_, err := ClaimShipment(fx.ctx, fx.ws, fx.shipA.ID)
	require.NoError(t, err)
	member := u1Disk(t, fx.ws, fx.task1.ID)
	require.Equal(t, models.StatusActive, member.Status)
	require.Equal(t, fx.shipA.ID, member.CustomFields[schedulerBaselineClaimKey])
}

func (fx *u1Fixture) blockA(t *testing.T) {
	t.Helper()
	fx.claimA(t)
	_, err := BlockShipment(fx.ctx, fx.ws, fx.shipA.ID, BlockOptions{Reason: "u1 block"})
	require.NoError(t, err)
	require.Equal(t, models.StatusBlocked, u1Disk(t, fx.ws, fx.shipA.ID).Status)
}

func (fx *u1Fixture) snapshot(t *testing.T) urAggregateSnapshot {
	t.Helper()
	return snapshotURGovernedState(t, fx.ws, []string{
		fx.shipA.ID, fx.shipB.ID, fx.feat.ID, fx.feat2.ID, fx.task1.ID, fx.task2.ID, fx.task3.ID,
	})
}

func u1Disk(t *testing.T, ws *Workspace, id string) *models.Artifact {
	t.Helper()
	artifact, err := findArtifact(context.Background(), ws, id)
	require.NoError(t, err)
	return artifact
}

func u1DB(t *testing.T, ws *Workspace, id string) *models.Artifact {
	t.Helper()
	artifact, err := bldb.GetItem(context.Background(), ws.DB, id)
	require.NoError(t, err)
	return artifact
}

func u1RequireManifest(t *testing.T, ws *Workspace, shipmentID string, want []string) {
	t.Helper()
	require.Equal(t, want, NormalizeShipmentItems(u1Disk(t, ws, shipmentID)), "markdown manifest of %s", shipmentID)
	require.Equal(t, want, NormalizeShipmentItems(u1DB(t, ws, shipmentID)), "sqlite manifest of %s", shipmentID)
}

func u1RequireMember(t *testing.T, ws *Workspace, id string, status models.ArtifactStatus, claim string) {
	t.Helper()
	for label, artifact := range map[string]*models.Artifact{"markdown": u1Disk(t, ws, id), "sqlite": u1DB(t, ws, id)} {
		require.Equal(t, status, artifact.Status, "%s status of %s", label, id)
		got, _ := artifact.CustomFields[schedulerBaselineClaimKey].(string)
		require.Equal(t, claim, got, "%s claim marker of %s", label, id)
	}
}

func u1Events(t *testing.T, ws *Workspace, itemID, eventType string) []events.Event {
	t.Helper()
	all, err := events.ReadAllEvents(context.Background(), WorkspaceLogsRoot(ws.RootPath), itemID)
	require.NoError(t, err)
	var matched []events.Event
	for _, event := range all {
		if event.EventType == eventType {
			matched = append(matched, event)
		}
	}
	return matched
}

func u1Journals(t *testing.T, ws *Workspace) []shipmentOperationJournalRecord {
	t.Helper()
	records, validationErrs, err := inspectShipmentOperationJournalsReadOnly(ws)
	require.NoError(t, err)
	require.Empty(t, validationErrs)
	return records
}

// u1StorageFiles lists every non-database file in the storage root so
// zero-write previews can prove no lock file or journal was created.
func u1StorageFiles(t *testing.T, ws *Workspace) map[string]string {
	t.Helper()
	root := workspaceStorageRoot(ws)
	return snapshotURFileTree(t, root, root, func(path string) bool {
		lower := strings.ToLower(path)
		for _, suffix := range []string{".db", ".db-wal", ".db-shm", ".sqlite", ".sqlite-wal", ".sqlite-shm"} {
			if strings.HasSuffix(lower, suffix) {
				return false
			}
		}
		return true
	})
}

// u1WriteJournal persists a manually built manifest-operation journal, as a
// crashed operation would have left behind.
func u1WriteJournal(
	t *testing.T,
	ws *Workspace,
	operation, phase string,
	shipment *models.Artifact,
	related *models.Artifact,
	target string,
) (string, shipmentLifecycleJournal) {
	t.Helper()
	var raw [16]byte
	_, err := rand.Read(raw[:])
	require.NoError(t, err)
	correlationID := hex.EncodeToString(raw[:])

	shipmentPreimage := cloneArtifact(u1Disk(t, ws, shipment.ID))
	normalizeShipmentArtifact(shipmentPreimage)
	preimage := shipmentLifecyclePreimage{Shipment: shipmentPreimage}
	for _, memberID := range NormalizeShipmentItems(shipmentPreimage) {
		preimage.Members = append(preimage.Members, cloneArtifact(u1Disk(t, ws, memberID)))
	}
	if related != nil {
		preimage.Related = []*models.Artifact{cloneArtifact(u1Disk(t, ws, related.ID))}
	}
	journal := shipmentLifecycleJournal{
		SchemaVersion:  "shipment-operation/v1",
		CorrelationID:  correlationID,
		Phase:          phase,
		Operation:      operation,
		RecoveryPolicy: "rollback",
		ShipmentID:     shipment.ID,
		Target:         target,
		Preimage:       preimage,
	}
	path, err := writeShipmentLifecycleJournalForWorkspace(ws, shipmentLifecycleJournalName(correlationID), journal)
	require.NoError(t, err)
	return path, journal
}

func u1Barrier(t *testing.T, fx *u1Fixture) {
	t.Helper()
	_, unlock, err := lockShipmentLifecycleGlobal(fx.ctx, fx.ws)
	require.NoError(t, err)
	require.NoError(t, unlock())
}

func u1FailPersistOnce(fx *u1Fixture, artifactID string, injected error) *int {
	calls := 0
	original := fx.originalPersistFn
	persistArtifactWriteFn = func(artifact *models.Artifact, filePath string, durable bool) error {
		if artifact.ID == artifactID {
			calls++
			if calls == 1 {
				return injected
			}
		}
		return original(artifact, filePath, durable)
	}
	return &calls
}

func u1FailPersistAlways(fx *u1Fixture, artifactID string, injected error) {
	original := fx.originalPersistFn
	persistArtifactWriteFn = func(artifact *models.Artifact, filePath string, durable bool) error {
		if artifact.ID == artifactID {
			return injected
		}
		return original(artifact, filePath, durable)
	}
}

func TestU1_RemoveFromShipment_QueuedAndForceGuards(t *testing.T) {
	t.Run("dry_run_preview_is_zero_write", func(t *testing.T) {
		fx := newU1Fixture(t)
		before := fx.snapshot(t)
		files := u1StorageFiles(t, fx.ws)

		result, err := RemoveFromShipment(fx.ctx, fx.ws, fx.shipA.ID, fx.task1.ID, ShipmentManifestOptions{DryRun: true})
		require.NoError(t, err)
		require.Equal(t, "dry_run", result.Status)
		require.True(t, result.DryRun)
		require.Equal(t, []string{fx.feat.ID, fx.task1.ID, fx.task2.ID}, result.ItemsBefore)
		require.Equal(t, []string{fx.feat.ID, fx.task2.ID}, result.ItemsAfter)
		requireURAggregateUnchanged(t, fx.ws, before)
		require.Equal(t, files, u1StorageFiles(t, fx.ws), "preview must not create lock files, journals, or events")
		require.Empty(t, u1Events(t, fx.ws, fx.shipA.ID, EventShipmentItemRemoved))
	})

	t.Run("dry_run_refuses_pending_journal_without_recovery", func(t *testing.T) {
		fx := newU1Fixture(t)
		path, _ := u1WriteJournal(t, fx.ws, shipmentOperationRemoveItem, shipmentOperationPhaseIntent,
			fx.shipA, nil, fx.task1.ID)
		before := fx.snapshot(t)

		_, err := RemoveFromShipment(fx.ctx, fx.ws, fx.shipA.ID, fx.task1.ID, ShipmentManifestOptions{DryRun: true})
		require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
		requireURAggregateUnchanged(t, fx.ws, before)
		require.FileExists(t, path, "preview must not run recovery or touch the journal")
	})

	t.Run("dry_run_sqlite_cache_miss_reads_markdown_without_writes", func(t *testing.T) {
		fx := newU1Fixture(t)
		for _, id := range []string{fx.shipA.ID, fx.task1.ID} {
			_, err := fx.ws.DB.ExecContext(fx.ctx, `DELETE FROM items WHERE id = ?`, id)
			require.NoError(t, err)
		}
		before := fx.snapshot(t)
		files := u1StorageFiles(t, fx.ws)

		result, err := RemoveFromShipment(fx.ctx, fx.ws, fx.shipA.ID, fx.task1.ID, ShipmentManifestOptions{DryRun: true})
		require.NoError(t, err)
		require.Equal(t, []string{fx.feat.ID, fx.task2.ID}, result.ItemsAfter)
		requireURAggregateUnchanged(t, fx.ws, before)
		require.Equal(t, files, u1StorageFiles(t, fx.ws))
		_, dbErr := bldb.GetItem(fx.ctx, fx.ws.DB, fx.shipA.ID)
		require.ErrorIs(t, dbErr, blerrors.ErrNotFound, "preview must not repopulate the SQLite cache")
	})

	t.Run("live_queued_removal_keeps_member_queued_and_audits", func(t *testing.T) {
		fx := newU1Fixture(t)

		result, err := RemoveFromShipment(fx.ctx, fx.ws, fx.shipA.ID, fx.task1.ID, ShipmentManifestOptions{})
		require.NoError(t, err)
		require.Equal(t, "removed", result.Status)
		require.False(t, result.DryRun)
		require.Equal(t, []string{fx.feat.ID, fx.task1.ID, fx.task2.ID}, result.ItemsBefore)
		require.Equal(t, []string{fx.feat.ID, fx.task2.ID}, result.ItemsAfter)

		u1RequireManifest(t, fx.ws, fx.shipA.ID, []string{fx.feat.ID, fx.task2.ID})
		u1RequireMember(t, fx.ws, fx.task1.ID, models.StatusQueued, "")
		path, err := FindArtifactPath(fx.ctx, fx.ws, fx.task1.ID)
		require.NoError(t, err)
		require.Equal(t, "queue", filepath.Base(filepath.Dir(path)))

		removed := u1Events(t, fx.ws, fx.shipA.ID, EventShipmentItemRemoved)
		require.Len(t, removed, 1)
		require.Equal(t, fx.task1.ID, removed[0].Delta["item_id"])
		require.Equal(t, "committed", removed[0].Delta["phase"])
		require.NotEmpty(t, removed[0].Delta["correlation_id"])
		var indexed int
		require.NoError(t, fx.ws.DB.QueryRowContext(fx.ctx,
			`SELECT COUNT(*) FROM item_log_entries WHERE item_id = ? AND event_type = ?`,
			fx.shipA.ID, EventShipmentItemRemoved).Scan(&indexed))
		require.Equal(t, 1, indexed, "event must be indexed in SQLite")
		require.Empty(t, u1Journals(t, fx.ws), "journal must be cleaned up")
	})

	t.Run("item_not_in_shipment_is_not_found", func(t *testing.T) {
		fx := newU1Fixture(t)
		_, err := RemoveFromShipment(fx.ctx, fx.ws, fx.shipA.ID, fx.task3.ID, ShipmentManifestOptions{})
		require.ErrorIs(t, err, blerrors.ErrNotFound)
	})

	t.Run("empty_arguments_are_validation_errors", func(t *testing.T) {
		fx := newU1Fixture(t)
		_, err := RemoveFromShipment(fx.ctx, fx.ws, "", fx.task1.ID, ShipmentManifestOptions{})
		require.ErrorIs(t, err, blerrors.ErrValidation)
		_, err = RemoveFromShipment(fx.ctx, fx.ws, fx.shipA.ID, "", ShipmentManifestOptions{})
		require.ErrorIs(t, err, blerrors.ErrValidation)
	})

	t.Run("active_shipment_requires_force_and_reason_and_requeues_claimed_member", func(t *testing.T) {
		fx := newU1Fixture(t)
		fx.claimA(t)
		before := fx.snapshot(t)

		_, err := RemoveFromShipment(fx.ctx, fx.ws, fx.shipA.ID, fx.task1.ID, ShipmentManifestOptions{})
		require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
		_, err = RemoveFromShipment(fx.ctx, fx.ws, fx.shipA.ID, fx.task1.ID, ShipmentManifestOptions{Force: true, Reason: "   "})
		require.ErrorIs(t, err, blerrors.ErrValidation)
		requireURAggregateUnchanged(t, fx.ws, before)

		result, err := RemoveFromShipment(fx.ctx, fx.ws, fx.shipA.ID, fx.task1.ID,
			ShipmentManifestOptions{Force: true, Reason: "operator descope"})
		require.NoError(t, err)
		require.Equal(t, "removed", result.Status)
		require.True(t, result.Force)

		u1RequireManifest(t, fx.ws, fx.shipA.ID, []string{fx.feat.ID, fx.task2.ID})
		u1RequireMember(t, fx.ws, fx.task1.ID, models.StatusQueued, "")
		u1RequireMember(t, fx.ws, fx.task2.ID, models.StatusActive, fx.shipA.ID)

		removed := u1Events(t, fx.ws, fx.shipA.ID, EventShipmentItemRemoved)
		require.Len(t, removed, 1)
		require.Equal(t, true, removed[0].Delta["force"])
		require.Equal(t, "operator descope", removed[0].Delta["reason"])

		var requeued *events.Event
		for _, event := range u1Events(t, fx.ws, fx.task1.ID, "status_changed") {
			if event.Delta["from"] == "active" && event.Delta["to"] == "queued" {
				event := event
				requeued = &event
			}
		}
		require.NotNil(t, requeued, "member must record a status_changed active -> queued event")
		require.Equal(t, removed[0].Delta["correlation_id"], requeued.Delta["correlation_id"])
		require.Empty(t, u1Journals(t, fx.ws))
	})

	t.Run("started_claimed_member_is_refused", func(t *testing.T) {
		fx := newU1Fixture(t)
		fx.claimA(t)
		require.NoError(t, AppendComment(fx.ctx, fx.ws, nil, fx.task1.ID, "tester", "WORK_STARTED: begin", ""))
		before := fx.snapshot(t)

		opts := ShipmentManifestOptions{Force: true, Reason: "operator descope"}
		_, err := RemoveFromShipment(fx.ctx, fx.ws, fx.shipA.ID, fx.task1.ID, opts)
		require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
		opts.DryRun = true
		_, err = RemoveFromShipment(fx.ctx, fx.ws, fx.shipA.ID, fx.task1.ID, opts)
		require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
		requireURAggregateUnchanged(t, fx.ws, before)
	})

	t.Run("active_member_without_source_claim_marker_is_refused", func(t *testing.T) {
		fx := newU1Fixture(t)
		fx.claimA(t)
		member := u1Disk(t, fx.ws, fx.task1.ID)
		member.CustomFields[schedulerBaselineClaimKey] = fx.shipB.ID
		require.NoError(t, persistArtifact(fx.ctx, fx.ws, member, false))
		before := fx.snapshot(t)

		_, err := RemoveFromShipment(fx.ctx, fx.ws, fx.shipA.ID, fx.task1.ID,
			ShipmentManifestOptions{Force: true, Reason: "operator descope"})
		require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
		requireURAggregateUnchanged(t, fx.ws, before)
	})

	t.Run("blocked_shipment_is_refused_even_with_force", func(t *testing.T) {
		fx := newU1Fixture(t)
		fx.blockA(t)
		before := fx.snapshot(t)

		for _, dryRun := range []bool{false, true} {
			_, err := RemoveFromShipment(fx.ctx, fx.ws, fx.shipA.ID, fx.task1.ID,
				ShipmentManifestOptions{Force: true, Reason: "override", DryRun: dryRun})
			require.ErrorIs(t, err, blerrors.ErrShipmentConflict, "dry_run=%v", dryRun)
		}
		requireURAggregateUnchanged(t, fx.ws, before)
	})

	t.Run("root_covering_feature_removal_refused_while_dependents_remain", func(t *testing.T) {
		fx := newU1Fixture(t)
		before := fx.snapshot(t)

		_, err := RemoveFromShipment(fx.ctx, fx.ws, fx.shipA.ID, fx.feat.ID, ShipmentManifestOptions{})
		require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
		_, err = RemoveFromShipment(fx.ctx, fx.ws, fx.shipA.ID, fx.feat.ID, ShipmentManifestOptions{DryRun: true})
		require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
		requireURAggregateUnchanged(t, fx.ws, before)

		for _, id := range []string{fx.task1.ID, fx.task2.ID, fx.feat.ID} {
			_, err = RemoveFromShipment(fx.ctx, fx.ws, fx.shipA.ID, id, ShipmentManifestOptions{})
			require.NoError(t, err, "removing %s once no dependents remain", id)
		}
		u1RequireManifest(t, fx.ws, fx.shipA.ID, []string{})
	})

	t.Run("crash_between_writes_intent_journal_recovered_by_barrier", func(t *testing.T) {
		fx := newU1Fixture(t)
		fx.claimA(t)
		before := fx.snapshot(t)

		_, journal := u1WriteJournal(t, fx.ws, shipmentOperationRemoveItem, shipmentOperationPhaseIntent,
			fx.shipA, nil, fx.task1.ID)
		// Simulate the crash window: member requeued, manifest not yet updated.
		member := u1Disk(t, fx.ws, fx.task1.ID)
		member.Status = models.StatusQueued
		delete(member.CustomFields, schedulerBaselineClaimKey)
		require.NoError(t, persistArtifact(fx.ctx, fx.ws, member, true))
		shipment := u1Disk(t, fx.ws, fx.shipA.ID)
		shipment.CustomFields["items"] = []string{fx.feat.ID, fx.task2.ID}
		require.NoError(t, persistArtifact(fx.ctx, fx.ws, shipment, false))
		require.Len(t, u1Journals(t, fx.ws), 1, "validator must accept the remove_item intent journal")
		require.Equal(t, journal.CorrelationID, u1Journals(t, fx.ws)[0].lifecycle.CorrelationID)

		u1Barrier(t, fx)

		requireP1C6DurableStateRestored(t, fx.ws, before)
		u1RequireMember(t, fx.ws, fx.task1.ID, models.StatusActive, fx.shipA.ID)
		u1RequireManifest(t, fx.ws, fx.shipA.ID, []string{fx.feat.ID, fx.task1.ID, fx.task2.ID})
	})

	t.Run("left_behind_committed_journal_is_finalized_without_rollback", func(t *testing.T) {
		fx := newU1Fixture(t)
		fx.claimA(t)
		fx.ws.removeShipmentOperationJournal = func(string) error { return errors.New("injected cleanup failure") }

		_, err := RemoveFromShipment(fx.ctx, fx.ws, fx.shipA.ID, fx.task1.ID,
			ShipmentManifestOptions{Force: true, Reason: "operator descope"})
		require.NoError(t, err, "a journal-cleanup failure must not fail the completed operation")
		records := u1Journals(t, fx.ws)
		require.Len(t, records, 1)
		require.Equal(t, shipmentOperationPhaseCommitted, records[0].lifecycle.Phase)
		require.Equal(t, shipmentOperationRemoveItem, records[0].lifecycle.Operation)
		require.Equal(t, shipmentOperationRecoveryPolicyRollback, records[0].lifecycle.RecoveryPolicy)
		require.Equal(t, fx.shipA.ID, records[0].lifecycle.ShipmentID)
		require.Equal(t, fx.task1.ID, records[0].lifecycle.Target)
		var preimageMember *models.Artifact
		for _, member := range records[0].lifecycle.Preimage.Members {
			if member.ID == fx.task1.ID {
				preimageMember = member
			}
		}
		require.NotNil(t, preimageMember)
		require.Equal(t, models.StatusActive, preimageMember.Status)
		require.Equal(t, fx.shipA.ID, preimageMember.CustomFields[schedulerBaselineClaimKey])

		fx.ws.removeShipmentOperationJournal = nil
		u1Barrier(t, fx)

		require.Empty(t, u1Journals(t, fx.ws))
		u1RequireManifest(t, fx.ws, fx.shipA.ID, []string{fx.feat.ID, fx.task2.ID})
		u1RequireMember(t, fx.ws, fx.task1.ID, models.StatusQueued, "")
		require.Len(t, u1Events(t, fx.ws, fx.shipA.ID, EventShipmentItemRemoved), 1)
	})

	t.Run("write_indeterminate_retains_intent_journal_for_barrier_recovery", func(t *testing.T) {
		fx := newU1Fixture(t)
		fx.claimA(t)
		before := fx.snapshot(t)
		u1FailPersistAlways(fx, fx.shipA.ID, fmt.Errorf("injected: %w", blerrors.ErrWriteIndeterminate))

		_, err := RemoveFromShipment(fx.ctx, fx.ws, fx.shipA.ID, fx.task1.ID,
			ShipmentManifestOptions{Force: true, Reason: "operator descope"})
		require.ErrorIs(t, err, blerrors.ErrWriteIndeterminate)

		records := u1Journals(t, fx.ws)
		require.Len(t, records, 1, "indeterminate outcome must retain the journal")
		require.Equal(t, shipmentOperationPhaseIntent, records[0].lifecycle.Phase)
		u1RequireMember(t, fx.ws, fx.task1.ID, models.StatusQueued, "") // no in-memory compensation

		persistArtifactWriteFn = fx.originalPersistFn
		u1Barrier(t, fx)

		requireP1C6DurableStateRestored(t, fx.ws, before)
		u1RequireMember(t, fx.ws, fx.task1.ID, models.StatusActive, fx.shipA.ID)
		u1RequireManifest(t, fx.ws, fx.shipA.ID, []string{fx.feat.ID, fx.task1.ID, fx.task2.ID})
	})

	t.Run("write_not_applied_rolls_back_manifest_and_member", func(t *testing.T) {
		fx := newU1Fixture(t)
		fx.claimA(t)
		before := fx.snapshot(t)
		calls := u1FailPersistOnce(fx, fx.shipA.ID, fmt.Errorf("injected: %w", blerrors.ErrWriteNotApplied))

		_, err := RemoveFromShipment(fx.ctx, fx.ws, fx.shipA.ID, fx.task1.ID,
			ShipmentManifestOptions{Force: true, Reason: "operator descope"})
		require.Error(t, err)
		require.False(t, errors.Is(err, blerrors.ErrWriteIndeterminate))
		require.GreaterOrEqual(t, *calls, 1)

		requireP1C6DurableStateRestored(t, fx.ws, before)
		u1RequireMember(t, fx.ws, fx.task1.ID, models.StatusActive, fx.shipA.ID)
		u1RequireManifest(t, fx.ws, fx.shipA.ID, []string{fx.feat.ID, fx.task1.ID, fx.task2.ID})
		require.Empty(t, u1Journals(t, fx.ws), "journal must be removed after rollback")
	})
}

func TestU1_MoveShipmentItem_AtomicTransferAndGuards(t *testing.T) {
	forceOpts := ShipmentManifestOptions{Force: true, Reason: "rebalance claimed member"}

	t.Run("dry_run_preview_is_zero_write", func(t *testing.T) {
		fx := newU1Fixture(t)
		before := fx.snapshot(t)
		files := u1StorageFiles(t, fx.ws)

		result, err := MoveShipmentItem(fx.ctx, fx.ws, fx.task1.ID, fx.shipA.ID, fx.shipB.ID,
			ShipmentManifestOptions{DryRun: true})
		require.NoError(t, err)
		require.Equal(t, "dry_run", result.Status)
		require.True(t, result.DryRun)
		require.Equal(t, []string{fx.feat.ID, fx.task1.ID, fx.task2.ID}, result.FromBefore)
		require.Equal(t, []string{fx.feat.ID, fx.task2.ID}, result.FromAfter)
		require.Equal(t, []string{fx.feat2.ID, fx.task3.ID}, result.ToBefore)
		require.Equal(t, []string{fx.feat2.ID, fx.task3.ID, fx.task1.ID}, result.ToAfter)
		requireURAggregateUnchanged(t, fx.ws, before)
		require.Equal(t, files, u1StorageFiles(t, fx.ws))
	})

	t.Run("dry_run_refuses_pending_journal_without_recovery", func(t *testing.T) {
		fx := newU1Fixture(t)
		path, _ := u1WriteJournal(t, fx.ws, shipmentOperationMoveItem, shipmentOperationPhaseIntent,
			fx.shipA, fx.shipB, fx.shipB.ID)
		before := fx.snapshot(t)

		_, err := MoveShipmentItem(fx.ctx, fx.ws, fx.task1.ID, fx.shipA.ID, fx.shipB.ID,
			ShipmentManifestOptions{DryRun: true})
		require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
		requireURAggregateUnchanged(t, fx.ws, before)
		require.FileExists(t, path)
	})

	t.Run("dry_run_sqlite_cache_miss_reads_markdown_without_writes", func(t *testing.T) {
		fx := newU1Fixture(t)
		for _, id := range []string{fx.shipA.ID, fx.shipB.ID, fx.task1.ID} {
			_, err := fx.ws.DB.ExecContext(fx.ctx, `DELETE FROM items WHERE id = ?`, id)
			require.NoError(t, err)
		}
		before := fx.snapshot(t)
		files := u1StorageFiles(t, fx.ws)

		_, err := MoveShipmentItem(fx.ctx, fx.ws, fx.task1.ID, fx.shipA.ID, fx.shipB.ID,
			ShipmentManifestOptions{DryRun: true})
		require.NoError(t, err)
		requireURAggregateUnchanged(t, fx.ws, before)
		require.Equal(t, files, u1StorageFiles(t, fx.ws))
	})

	t.Run("live_move_transfers_item_and_audits_both_shipments", func(t *testing.T) {
		fx := newU1Fixture(t)

		result, err := MoveShipmentItem(fx.ctx, fx.ws, fx.task1.ID, fx.shipA.ID, fx.shipB.ID,
			ShipmentManifestOptions{Reason: "rebalance scope"})
		require.NoError(t, err)
		require.Equal(t, "moved", result.Status)
		u1RequireManifest(t, fx.ws, fx.shipA.ID, []string{fx.feat.ID, fx.task2.ID})
		u1RequireManifest(t, fx.ws, fx.shipB.ID, []string{fx.feat2.ID, fx.task3.ID, fx.task1.ID})
		u1RequireMember(t, fx.ws, fx.task1.ID, models.StatusQueued, "")

		fromEvents := u1Events(t, fx.ws, fx.shipA.ID, EventShipmentItemMoved)
		toEvents := u1Events(t, fx.ws, fx.shipB.ID, EventShipmentItemMoved)
		require.Len(t, fromEvents, 1)
		require.Len(t, toEvents, 1)
		for _, event := range []events.Event{fromEvents[0], toEvents[0]} {
			require.Equal(t, fx.task1.ID, event.Delta["item_id"])
			require.Equal(t, fx.shipA.ID, event.Delta["from_shipment_id"])
			require.Equal(t, fx.shipB.ID, event.Delta["to_shipment_id"])
			require.Equal(t, "rebalance scope", event.Delta["reason"])
			require.Equal(t, "committed", event.Delta["phase"])
		}
		require.NotEmpty(t, fromEvents[0].Delta["correlation_id"])
		require.Equal(t, fromEvents[0].Delta["correlation_id"], toEvents[0].Delta["correlation_id"])
		require.Empty(t, u1Journals(t, fx.ws))
	})

	t.Run("argument_and_status_guards", func(t *testing.T) {
		fx := newU1Fixture(t)
		_, err := MoveShipmentItem(fx.ctx, fx.ws, fx.task1.ID, fx.shipA.ID, fx.shipA.ID, ShipmentManifestOptions{})
		require.ErrorIs(t, err, blerrors.ErrValidation)
		_, err = MoveShipmentItem(fx.ctx, fx.ws, fx.task3.ID, fx.shipA.ID, fx.shipB.ID, ShipmentManifestOptions{})
		require.ErrorIs(t, err, blerrors.ErrNotFound)

		fx.claimA(t)
		before := fx.snapshot(t)
		_, err = MoveShipmentItem(fx.ctx, fx.ws, fx.task1.ID, fx.shipA.ID, fx.shipB.ID, ShipmentManifestOptions{})
		require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
		_, err = MoveShipmentItem(fx.ctx, fx.ws, fx.task1.ID, fx.shipA.ID, fx.shipB.ID,
			ShipmentManifestOptions{Force: true, Reason: " "})
		require.ErrorIs(t, err, blerrors.ErrValidation)
		_, err = MoveShipmentItem(fx.ctx, fx.ws, fx.task3.ID, fx.shipB.ID, fx.shipA.ID, ShipmentManifestOptions{})
		require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
		requireURAggregateUnchanged(t, fx.ws, before)
	})

	t.Run("item_assigned_to_third_shipment_is_refused", func(t *testing.T) {
		fx := newU1Fixture(t)
		other, err := CreateArtifact(fx.ctx, fx.ws, "U1 other", "task", WithParent(fx.feat2.ID))
		require.NoError(t, err)
		shipC, err := CreateShipment(fx.ctx, fx.ws, "U1 ship C", []string{other.ID})
		require.NoError(t, err)
		// Hand-assign task3 (a shipB member) to shipC's manifest to model drift.
		shipment := u1Disk(t, fx.ws, shipC.ID)
		shipment.CustomFields["items"] = []string{other.ID, fx.task3.ID}
		require.NoError(t, persistArtifact(fx.ctx, fx.ws, shipment, false))
		before := fx.snapshot(t)

		_, err = MoveShipmentItem(fx.ctx, fx.ws, fx.task3.ID, fx.shipB.ID, fx.shipA.ID, ShipmentManifestOptions{})
		require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
		requireURAggregateUnchanged(t, fx.ws, before)
	})

	t.Run("claimed_member_transfers_to_queued_shipment_then_claims_cleanly", func(t *testing.T) {
		fx := newU1Fixture(t)
		fx.claimA(t)

		result, err := MoveShipmentItem(fx.ctx, fx.ws, fx.task1.ID, fx.shipA.ID, fx.shipB.ID, forceOpts)
		require.NoError(t, err)
		require.Equal(t, "moved", result.Status)
		u1RequireManifest(t, fx.ws, fx.shipA.ID, []string{fx.feat.ID, fx.task2.ID})
		u1RequireManifest(t, fx.ws, fx.shipB.ID, []string{fx.feat2.ID, fx.task3.ID, fx.task1.ID})
		u1RequireMember(t, fx.ws, fx.task1.ID, models.StatusQueued, "")

		var requeued bool
		for _, event := range u1Events(t, fx.ws, fx.task1.ID, "status_changed") {
			if event.Delta["from"] == "active" && event.Delta["to"] == "queued" {
				requeued = true
			}
		}
		require.True(t, requeued, "member must record a status_changed active -> queued event")

		// Free the single active slot so the destination can be claimed.
		governed := context.WithValue(fx.ctx, artifactWriteEnvelopeContextKey{},
			artifactWriteEnvelope{allowGovernedShipmentMutation: true})
		source := u1Disk(t, fx.ws, fx.shipA.ID)
		source.Status = models.StatusQueued
		require.NoError(t, persistArtifact(governed, fx.ws, source, true))

		_, err = ClaimShipment(fx.ctx, fx.ws, fx.shipB.ID)
		require.NoError(t, err)
		u1RequireMember(t, fx.ws, fx.task1.ID, models.StatusActive, fx.shipB.ID)
		u1RequireMember(t, fx.ws, fx.task3.ID, models.StatusActive, fx.shipB.ID)
	})

	t.Run("started_or_unclaimed_active_member_is_refused", func(t *testing.T) {
		fx := newU1Fixture(t)
		fx.claimA(t)
		require.NoError(t, AppendComment(fx.ctx, fx.ws, nil, fx.task1.ID, "tester", "WORK_STARTED: begin", ""))
		before := fx.snapshot(t)

		_, err := MoveShipmentItem(fx.ctx, fx.ws, fx.task1.ID, fx.shipA.ID, fx.shipB.ID, forceOpts)
		require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
		dry := forceOpts
		dry.DryRun = true
		_, err = MoveShipmentItem(fx.ctx, fx.ws, fx.task1.ID, fx.shipA.ID, fx.shipB.ID, dry)
		require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
		requireURAggregateUnchanged(t, fx.ws, before)

		// A member lacking the source claim marker is refused as well.
		member := u1Disk(t, fx.ws, fx.task2.ID)
		member.CustomFields[schedulerBaselineClaimKey] = fx.shipB.ID
		require.NoError(t, persistArtifact(fx.ctx, fx.ws, member, false))
		before = fx.snapshot(t)
		_, err = MoveShipmentItem(fx.ctx, fx.ws, fx.task2.ID, fx.shipA.ID, fx.shipB.ID, forceOpts)
		require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
		requireURAggregateUnchanged(t, fx.ws, before)
	})

	t.Run("blocked_shipment_is_refused_even_with_force", func(t *testing.T) {
		fx := newU1Fixture(t)
		fx.blockA(t)
		before := fx.snapshot(t)

		for _, dryRun := range []bool{false, true} {
			opts := ShipmentManifestOptions{Force: true, Reason: "override", DryRun: dryRun}
			_, err := MoveShipmentItem(fx.ctx, fx.ws, fx.task1.ID, fx.shipA.ID, fx.shipB.ID, opts)
			require.ErrorIs(t, err, blerrors.ErrShipmentConflict, "blocked source dry_run=%v", dryRun)
			_, err = MoveShipmentItem(fx.ctx, fx.ws, fx.task3.ID, fx.shipB.ID, fx.shipA.ID, opts)
			require.ErrorIs(t, err, blerrors.ErrShipmentConflict, "blocked target dry_run=%v", dryRun)
		}
		requireURAggregateUnchanged(t, fx.ws, before)
	})

	t.Run("crash_between_writes_intent_journal_recovered_by_barrier", func(t *testing.T) {
		fx := newU1Fixture(t)
		fx.claimA(t)
		before := fx.snapshot(t)

		_, journal := u1WriteJournal(t, fx.ws, shipmentOperationMoveItem, shipmentOperationPhaseIntent,
			fx.shipA, fx.shipB, fx.shipB.ID)
		member := u1Disk(t, fx.ws, fx.task1.ID)
		member.Status = models.StatusQueued
		delete(member.CustomFields, schedulerBaselineClaimKey)
		require.NoError(t, persistArtifact(fx.ctx, fx.ws, member, true))
		source := u1Disk(t, fx.ws, fx.shipA.ID)
		source.CustomFields["items"] = []string{fx.feat.ID, fx.task2.ID}
		require.NoError(t, persistArtifact(fx.ctx, fx.ws, source, false))
		target := u1Disk(t, fx.ws, fx.shipB.ID)
		target.CustomFields["items"] = []string{fx.feat2.ID, fx.task3.ID, fx.task1.ID}
		require.NoError(t, persistArtifact(fx.ctx, fx.ws, target, false))
		records := u1Journals(t, fx.ws)
		require.Len(t, records, 1, "validator must accept the move_item intent journal")
		require.Equal(t, journal.CorrelationID, records[0].lifecycle.CorrelationID)

		u1Barrier(t, fx)

		requireP1C6DurableStateRestored(t, fx.ws, before)
		u1RequireMember(t, fx.ws, fx.task1.ID, models.StatusActive, fx.shipA.ID)
		u1RequireManifest(t, fx.ws, fx.shipA.ID, []string{fx.feat.ID, fx.task1.ID, fx.task2.ID})
		u1RequireManifest(t, fx.ws, fx.shipB.ID, []string{fx.feat2.ID, fx.task3.ID})
	})

	t.Run("left_behind_committed_journal_is_finalized_without_rollback", func(t *testing.T) {
		fx := newU1Fixture(t)
		fx.claimA(t)
		fx.ws.removeShipmentOperationJournal = func(string) error { return errors.New("injected cleanup failure") }

		_, err := MoveShipmentItem(fx.ctx, fx.ws, fx.task1.ID, fx.shipA.ID, fx.shipB.ID, forceOpts)
		require.NoError(t, err)
		records := u1Journals(t, fx.ws)
		require.Len(t, records, 1)
		require.Equal(t, shipmentOperationPhaseCommitted, records[0].lifecycle.Phase)
		require.Equal(t, shipmentOperationMoveItem, records[0].lifecycle.Operation)
		require.Equal(t, fx.shipA.ID, records[0].lifecycle.ShipmentID)
		require.Equal(t, fx.shipB.ID, records[0].lifecycle.Target)
		require.Len(t, records[0].lifecycle.Preimage.Related, 1)
		require.Equal(t, fx.shipB.ID, records[0].lifecycle.Preimage.Related[0].ID)

		fx.ws.removeShipmentOperationJournal = nil
		u1Barrier(t, fx)

		require.Empty(t, u1Journals(t, fx.ws))
		u1RequireManifest(t, fx.ws, fx.shipA.ID, []string{fx.feat.ID, fx.task2.ID})
		u1RequireManifest(t, fx.ws, fx.shipB.ID, []string{fx.feat2.ID, fx.task3.ID, fx.task1.ID})
		u1RequireMember(t, fx.ws, fx.task1.ID, models.StatusQueued, "")
		require.Len(t, u1Events(t, fx.ws, fx.shipB.ID, EventShipmentItemMoved), 1)
	})

	t.Run("write_indeterminate_retains_intent_journal_for_barrier_recovery", func(t *testing.T) {
		fx := newU1Fixture(t)
		fx.claimA(t)
		before := fx.snapshot(t)
		u1FailPersistAlways(fx, fx.shipB.ID, fmt.Errorf("injected: %w", blerrors.ErrWriteIndeterminate))

		_, err := MoveShipmentItem(fx.ctx, fx.ws, fx.task1.ID, fx.shipA.ID, fx.shipB.ID, forceOpts)
		require.ErrorIs(t, err, blerrors.ErrWriteIndeterminate)

		records := u1Journals(t, fx.ws)
		require.Len(t, records, 1)
		require.Equal(t, shipmentOperationPhaseIntent, records[0].lifecycle.Phase)

		persistArtifactWriteFn = fx.originalPersistFn
		u1Barrier(t, fx)

		requireP1C6DurableStateRestored(t, fx.ws, before)
		u1RequireMember(t, fx.ws, fx.task1.ID, models.StatusActive, fx.shipA.ID)
		u1RequireManifest(t, fx.ws, fx.shipA.ID, []string{fx.feat.ID, fx.task1.ID, fx.task2.ID})
		u1RequireManifest(t, fx.ws, fx.shipB.ID, []string{fx.feat2.ID, fx.task3.ID})
	})

	t.Run("second_append_not_applied_rolls_back_and_appends_aborted_event", func(t *testing.T) {
		fx := newU1Fixture(t)
		fx.claimA(t)
		var correlationID string
		fx.ws.shipmentEventAppend = func(ctx context.Context, ws *Workspace, itemID, eventType string, delta map[string]any) error {
			if eventType == EventShipmentItemMoved && itemID == fx.shipB.ID {
				correlationID = shipmentOperationID(ctx)
				return fmt.Errorf("injected second append: %w", blerrors.ErrWriteNotApplied)
			}
			return appendShipmentEventErr(ctx, ws, itemID, eventType, delta)
		}

		_, err := MoveShipmentItem(fx.ctx, fx.ws, fx.task1.ID, fx.shipA.ID, fx.shipB.ID, forceOpts)
		require.Error(t, err)
		require.False(t, errors.Is(err, blerrors.ErrWriteIndeterminate))
		require.NotEmpty(t, correlationID)

		u1RequireMember(t, fx.ws, fx.task1.ID, models.StatusActive, fx.shipA.ID)
		u1RequireManifest(t, fx.ws, fx.shipA.ID, []string{fx.feat.ID, fx.task1.ID, fx.task2.ID})
		u1RequireManifest(t, fx.ws, fx.shipB.ID, []string{fx.feat2.ID, fx.task3.ID})
		require.Empty(t, u1Journals(t, fx.ws), "journal must be removed after rollback")

		var aborted []events.Event
		for _, event := range u1Events(t, fx.ws, fx.shipA.ID, EventShipmentItemMoved) {
			if event.Delta["phase"] == "aborted" {
				aborted = append(aborted, event)
			}
			require.NotEqual(t, "committed", event.Delta["phase"], "rolled-back move must not leave a committed event")
		}
		require.Len(t, aborted, 1)
		require.Equal(t, correlationID, aborted[0].Delta["correlation_id"])
		require.Empty(t, u1Events(t, fx.ws, fx.shipB.ID, EventShipmentItemMoved))
	})
}

// u2Create creates an artifact and wires hard blocks dependencies
// (itemID depends_on each dependsOn entry).
func u2Create(
	t *testing.T,
	ws *Workspace,
	title, artifactType, parentID string,
	dependsOn ...*models.Artifact,
) *models.Artifact {
	t.Helper()
	ctx := context.Background()
	var opts []Option
	if parentID != "" {
		opts = append(opts, WithParent(parentID))
	}
	artifact, err := CreateArtifact(ctx, ws, title, artifactType, opts...)
	require.NoError(t, err)
	for _, dep := range dependsOn {
		require.NoError(t, AddDependency(ctx, ws, artifact.ID, dep.ID, "blocks"))
	}
	return artifact
}

func u2IDs(items ...*models.Artifact) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}

func u2Ship(t *testing.T, ws *Workspace, title string, items ...*models.Artifact) *models.Artifact {
	t.Helper()
	shipment, err := CreateShipment(context.Background(), ws, title, u2IDs(items...))
	require.NoError(t, err)
	require.Equal(t, u2IDs(items...), NormalizeShipmentItems(u1Disk(t, ws, shipment.ID)),
		"fixture must keep the requested manifest order")
	return shipment
}

func u2Snapshot(t *testing.T, ws *Workspace, shipment *models.Artifact, items ...*models.Artifact) urAggregateSnapshot {
	t.Helper()
	return snapshotURGovernedState(t, ws, append(u2IDs(items...), shipment.ID))
}

func TestU2_ReorderShipment_ExplicitAndAutoTopological(t *testing.T) {
	ctx := context.Background()
	type u2Scrambled struct {
		ws                                      *Workspace
		feat, task1, task2, task3, sub11, sub21 *models.Artifact
		ship                                    *models.Artifact
		scrambled, ordered                      []string
		all                                     []*models.Artifact
	}
	newFixture := func(t *testing.T) *u2Scrambled {
		t.Helper()
		ws := setupShipmentWorkspace(t)
		fx := &u2Scrambled{ws: ws}
		fx.feat = u2Create(t, ws, "U2 feature", "feature", "")
		fx.task1 = u2Create(t, ws, "U2 task 1", "task", fx.feat.ID)
		fx.task2 = u2Create(t, ws, "U2 task 2", "task", fx.feat.ID, fx.task1)
		fx.task3 = u2Create(t, ws, "U2 task 3", "task", fx.feat.ID, fx.task2)
		fx.sub11 = u2Create(t, ws, "U2 subtask 1.1", "subtask", fx.task1.ID)
		fx.sub21 = u2Create(t, ws, "U2 subtask 2.1", "subtask", fx.task2.ID)
		fx.all = []*models.Artifact{fx.sub21, fx.task3, fx.task1, fx.feat, fx.task2, fx.sub11}
		fx.ship = u2Ship(t, ws, "U2 ship", fx.all...)
		fx.scrambled = u2IDs(fx.all...)
		fx.ordered = u2IDs(fx.feat, fx.task1, fx.sub11, fx.task2, fx.sub21, fx.task3)
		return fx
	}

	t.Run("auto_dry_run_is_zero_write", func(t *testing.T) {
		fx := newFixture(t)
		before := u2Snapshot(t, fx.ws, fx.ship, fx.all...)
		files := u1StorageFiles(t, fx.ws)

		result, err := ReorderShipment(ctx, fx.ws, fx.ship.ID, ShipmentManifestOptions{Auto: true, DryRun: true})
		require.NoError(t, err)
		require.Equal(t, "dry_run", result.Status)
		require.Equal(t, "auto", result.Mode)
		require.True(t, result.DryRun)
		require.Equal(t, fx.scrambled, result.ItemsBefore)
		require.Equal(t, fx.ordered, result.ItemsAfter)
		requireURAggregateUnchanged(t, fx.ws, before)
		require.Equal(t, files, u1StorageFiles(t, fx.ws))
		require.Empty(t, u1Events(t, fx.ws, fx.ship.ID, EventShipmentReordered))
	})

	t.Run("dry_run_refuses_pending_journal_without_recovery", func(t *testing.T) {
		fx := newFixture(t)
		path, _ := u1WriteJournal(t, fx.ws, shipmentOperationRemoveItem, shipmentOperationPhaseIntent,
			fx.ship, nil, fx.task1.ID)
		before := u2Snapshot(t, fx.ws, fx.ship, fx.all...)

		_, err := ReorderShipment(ctx, fx.ws, fx.ship.ID, ShipmentManifestOptions{Auto: true, DryRun: true})
		require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
		requireURAggregateUnchanged(t, fx.ws, before)
		require.FileExists(t, path)
	})

	t.Run("dry_run_sqlite_cache_miss_reads_markdown_without_writes", func(t *testing.T) {
		fx := newFixture(t)
		for _, id := range []string{fx.ship.ID, fx.task1.ID, fx.sub21.ID} {
			_, err := fx.ws.DB.ExecContext(ctx, `DELETE FROM items WHERE id = ?`, id)
			require.NoError(t, err)
		}
		before := u2Snapshot(t, fx.ws, fx.ship, fx.all...)
		files := u1StorageFiles(t, fx.ws)

		result, err := ReorderShipment(ctx, fx.ws, fx.ship.ID, ShipmentManifestOptions{Auto: true, DryRun: true})
		require.NoError(t, err)
		require.Equal(t, fx.ordered, result.ItemsAfter)
		requireURAggregateUnchanged(t, fx.ws, before)
		require.Equal(t, files, u1StorageFiles(t, fx.ws))
		_, dbErr := bldb.GetItem(ctx, fx.ws.DB, fx.ship.ID)
		require.ErrorIs(t, dbErr, blerrors.ErrNotFound)
	})

	t.Run("live_auto_persists_and_audits", func(t *testing.T) {
		fx := newFixture(t)
		result, err := ReorderShipment(ctx, fx.ws, fx.ship.ID, ShipmentManifestOptions{Auto: true})
		require.NoError(t, err)
		require.Equal(t, "reordered", result.Status)
		require.Equal(t, "auto", result.Mode)
		require.False(t, result.DryRun)
		require.Equal(t, fx.ordered, result.ItemsAfter)
		u1RequireManifest(t, fx.ws, fx.ship.ID, fx.ordered)

		reordered := u1Events(t, fx.ws, fx.ship.ID, EventShipmentReordered)
		require.Len(t, reordered, 1)
		require.Equal(t, "auto", reordered[0].Delta["mode"])
		require.Equal(t, "committed", reordered[0].Delta["phase"])
		require.NotEmpty(t, reordered[0].Delta["correlation_id"])
		require.NotNil(t, reordered[0].Delta["items_before"])
		require.NotNil(t, reordered[0].Delta["items_after"])
		var indexed int
		require.NoError(t, fx.ws.DB.QueryRowContext(ctx,
			`SELECT COUNT(*) FROM item_log_entries WHERE item_id = ? AND event_type = ?`,
			fx.ship.ID, EventShipmentReordered).Scan(&indexed))
		require.Equal(t, 1, indexed)
		require.Empty(t, u1Journals(t, fx.ws))

		again, err := ReorderShipment(ctx, fx.ws, fx.ship.ID, ShipmentManifestOptions{Auto: true, DryRun: true})
		require.NoError(t, err)
		require.Equal(t, fx.ordered, again.ItemsAfter, "auto must be idempotent")
	})

	t.Run("explicit_permutation_and_rejections", func(t *testing.T) {
		fx := newFixture(t)
		bad := map[string][]string{
			"omits_member":        u2IDs(fx.feat, fx.task1, fx.sub11, fx.task2, fx.sub21),
			"foreign_id":          append(u2IDs(fx.feat, fx.task1, fx.sub11, fx.task2, fx.sub21), "999-T"),
			"duplicate":           u2IDs(fx.feat, fx.task1, fx.task1, fx.task2, fx.sub21, fx.task3),
			"non_root_before_it":  u2IDs(fx.task1, fx.feat, fx.sub11, fx.task2, fx.sub21, fx.task3),
			"violates_blocks":     u2IDs(fx.feat, fx.task2, fx.task1, fx.sub11, fx.sub21, fx.task3),
			"violates_parent":     u2IDs(fx.feat, fx.sub11, fx.task1, fx.task2, fx.sub21, fx.task3),
			"violates_chain_tail": u2IDs(fx.feat, fx.task1, fx.sub11, fx.task3, fx.task2, fx.sub21),
		}
		before := u2Snapshot(t, fx.ws, fx.ship, fx.all...)
		for name, items := range bad {
			for _, dryRun := range []bool{true, false} {
				_, err := ReorderShipment(ctx, fx.ws, fx.ship.ID, ShipmentManifestOptions{Items: items, DryRun: dryRun})
				require.ErrorIs(t, err, blerrors.ErrValidation, "%s dry_run=%v", name, dryRun)
			}
		}
		requireURAggregateUnchanged(t, fx.ws, before)

		valid := u2IDs(fx.feat, fx.task1, fx.task2, fx.sub21, fx.sub11, fx.task3)
		result, err := ReorderShipment(ctx, fx.ws, fx.ship.ID, ShipmentManifestOptions{Items: valid})
		require.NoError(t, err)
		require.Equal(t, "explicit", result.Mode)
		require.Equal(t, "reordered", result.Status)
		u1RequireManifest(t, fx.ws, fx.ship.ID, valid)
		reordered := u1Events(t, fx.ws, fx.ship.ID, EventShipmentReordered)
		require.Len(t, reordered, 1)
		require.Equal(t, "explicit", reordered[0].Delta["mode"])
	})

	t.Run("mode_selection_is_mutually_exclusive", func(t *testing.T) {
		fx := newFixture(t)
		before := u2Snapshot(t, fx.ws, fx.ship, fx.all...)
		_, err := ReorderShipment(ctx, fx.ws, fx.ship.ID, ShipmentManifestOptions{})
		require.ErrorIs(t, err, blerrors.ErrValidation)
		_, err = ReorderShipment(ctx, fx.ws, fx.ship.ID, ShipmentManifestOptions{Auto: true, Items: fx.ordered})
		require.ErrorIs(t, err, blerrors.ErrValidation)
		_, err = ReorderShipment(ctx, fx.ws, "", ShipmentManifestOptions{Auto: true})
		require.ErrorIs(t, err, blerrors.ErrValidation)
		requireURAggregateUnchanged(t, fx.ws, before)
	})

	t.Run("active_shipment_requires_force_and_reason", func(t *testing.T) {
		fx := newU1Fixture(t)
		fx.claimA(t)
		want := []string{fx.feat.ID, fx.task2.ID, fx.task1.ID}
		before := fx.snapshot(t)

		_, err := ReorderShipment(fx.ctx, fx.ws, fx.shipA.ID, ShipmentManifestOptions{Items: want})
		require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
		_, err = ReorderShipment(fx.ctx, fx.ws, fx.shipA.ID, ShipmentManifestOptions{Items: want, Force: true})
		require.ErrorIs(t, err, blerrors.ErrValidation)
		requireURAggregateUnchanged(t, fx.ws, before)

		result, err := ReorderShipment(fx.ctx, fx.ws, fx.shipA.ID,
			ShipmentManifestOptions{Items: want, Force: true, Reason: "fix manifest order"})
		require.NoError(t, err)
		require.True(t, result.Force)
		u1RequireManifest(t, fx.ws, fx.shipA.ID, want)
		reordered := u1Events(t, fx.ws, fx.shipA.ID, EventShipmentReordered)
		require.Len(t, reordered, 1)
		require.Equal(t, true, reordered[0].Delta["force"])
		require.Equal(t, "fix manifest order", reordered[0].Delta["reason"])
	})

	t.Run("blocked_shipment_is_refused_even_with_force", func(t *testing.T) {
		fx := newU1Fixture(t)
		fx.blockA(t)
		before := fx.snapshot(t)
		for _, dryRun := range []bool{false, true} {
			_, err := ReorderShipment(fx.ctx, fx.ws, fx.shipA.ID, ShipmentManifestOptions{
				Auto: true, Force: true, Reason: "fix manifest order", DryRun: dryRun,
			})
			require.ErrorIs(t, err, blerrors.ErrShipmentConflict, "dry_run=%v", dryRun)
		}
		requireURAggregateUnchanged(t, fx.ws, before)
	})

	t.Run("crash_intent_journal_is_rolled_back_by_barrier", func(t *testing.T) {
		fx := newU1Fixture(t)
		_, journal := u1WriteJournal(t, fx.ws, shipmentOperationReorderItems, shipmentOperationPhaseIntent,
			fx.shipA, nil, "reorder")
		torn := u1Disk(t, fx.ws, fx.shipA.ID)
		torn.CustomFields["items"] = []string{fx.feat.ID, fx.task2.ID, fx.task1.ID}
		require.NoError(t, persistArtifact(fx.ctx, fx.ws, torn, false))
		_ = journal

		u1Barrier(t, fx)
		u1RequireManifest(t, fx.ws, fx.shipA.ID, []string{fx.feat.ID, fx.task1.ID, fx.task2.ID})
		require.Empty(t, u1Journals(t, fx.ws))
	})
}

func TestU2_ReorderShipment_NestedFeatureAndFlatRegressions(t *testing.T) {
	ctx := context.Background()

	t.Run("nested_feature_precedes_its_task", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		root := u2Create(t, ws, "U2 root feature", "feature", "")
		nested := u2Create(t, ws, "U2 nested feature", "feature", root.ID)
		require.Contains(t, nested.ID, ".")
		task := u2Create(t, ws, "U2 nested task", "task", nested.ID)
		ship := u2Ship(t, ws, "U2 nested ship", root, task, nested)

		result, err := ReorderShipment(ctx, ws, ship.ID, ShipmentManifestOptions{Auto: true})
		require.NoError(t, err)
		require.Equal(t, u2IDs(root, nested, task), result.ItemsAfter)
		u1RequireManifest(t, ws, ship.ID, u2IDs(root, nested, task))
	})

	t.Run("flat_manifest_honors_subtask_blocks_edge_with_absent_parent", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		parent := u2Create(t, ws, "U2 absent parent", "task", "")
		sub := u2Create(t, ws, "U2 flat subtask", "subtask", parent.ID)
		task := u2Create(t, ws, "U2 flat task", "task", "", sub)
		ship := u2Ship(t, ws, "U2 flat ship", task, sub)

		result, err := ReorderShipment(ctx, ws, ship.ID, ShipmentManifestOptions{Auto: true})
		require.NoError(t, err)
		require.Equal(t, u2IDs(sub, task), result.ItemsAfter)
	})

	t.Run("hard_blocks_cycle_among_members_is_refused", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		a := u2Create(t, ws, "U2 cycle a", "task", "")
		b := u2Create(t, ws, "U2 cycle b", "task", "", a)
		// Inject the back edge directly in frontmatter; AddDependency would
		// reject it.
		onDisk := u1Disk(t, ws, a.ID)
		onDisk.Dependencies = append(onDisk.Dependencies, models.DependencyEdge{ID: b.ID, Type: "blocks"})
		require.NoError(t, persistArtifact(ctx, ws, onDisk, false))
		ship := u2Ship(t, ws, "U2 cycle ship", a, b)

		_, err := ReorderShipment(ctx, ws, ship.ID, ShipmentManifestOptions{Auto: true})
		require.ErrorIs(t, err, blerrors.ErrValidation)
		_, err = ReorderShipment(ctx, ws, ship.ID, ShipmentManifestOptions{Items: u2IDs(a, b), DryRun: true})
		require.ErrorIs(t, err, blerrors.ErrValidation)
	})
}

func TestU2_ReorderShipment_CrossParentCycleAvoidanceAndIdempotence(t *testing.T) {
	ctx := context.Background()
	ws := setupShipmentWorkspace(t)
	feat := u2Create(t, ws, "U2 cross F", "feature", "")
	// Create b and a first so A and B can declare the hard dependencies.
	parentA := u2Create(t, ws, "U2 cross A", "task", feat.ID)
	parentB := u2Create(t, ws, "U2 cross B", "task", feat.ID)
	childA := u2Create(t, ws, "U2 cross a", "subtask", parentA.ID)
	childB := u2Create(t, ws, "U2 cross b", "subtask", parentB.ID)
	require.NoError(t, AddDependency(ctx, ws, parentA.ID, childB.ID, "blocks"))
	require.NoError(t, AddDependency(ctx, ws, parentB.ID, childA.ID, "blocks"))
	ship := u2Ship(t, ws, "U2 cross ship", feat, parentA, parentB, childA, childB)
	want := u2IDs(feat, childB, parentA, childA, parentB)

	dry, err := ReorderShipment(ctx, ws, ship.ID, ShipmentManifestOptions{Auto: true, DryRun: true})
	require.NoError(t, err)
	require.Equal(t, want, dry.ItemsAfter)

	for run := 1; run <= 2; run++ {
		result, err := ReorderShipment(ctx, ws, ship.ID, ShipmentManifestOptions{Auto: true})
		require.NoError(t, err, "auto run %d", run)
		require.Equal(t, want, result.ItemsAfter, "auto run %d", run)
		u1RequireManifest(t, ws, ship.ID, want)
	}

	// The persisted auto order is accepted verbatim as an explicit order.
	explicit, err := ReorderShipment(ctx, ws, ship.ID, ShipmentManifestOptions{Items: want})
	require.NoError(t, err)
	require.Equal(t, want, explicit.ItemsAfter)

	// The skipped B -> b parent edge is not enforced, but A -> a is.
	_, err = ReorderShipment(ctx, ws, ship.ID, ShipmentManifestOptions{
		Items: u2IDs(feat, childB, childA, parentA, parentB), DryRun: true,
	})
	require.ErrorIs(t, err, blerrors.ErrValidation)
}

func TestU2_ReorderShipment_RootFeatureDependencies(t *testing.T) {
	ctx := context.Background()

	t.Run("root_depending_on_non_root_member_is_refused", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		task := u2Create(t, ws, "U2 prereq task", "task", "")
		root := u2Create(t, ws, "U2 dependent root", "feature", "", task)
		ship := u2Ship(t, ws, "U2 root dep ship", task, root)

		_, err := ReorderShipment(ctx, ws, ship.ID, ShipmentManifestOptions{Auto: true})
		require.ErrorIs(t, err, blerrors.ErrValidation)
		_, err = ReorderShipment(ctx, ws, ship.ID, ShipmentManifestOptions{Items: u2IDs(root, task), DryRun: true})
		require.ErrorIs(t, err, blerrors.ErrValidation)
		u1RequireManifest(t, ws, ship.ID, u2IDs(task, root))
	})

	t.Run("roots_are_topologically_ordered_and_enforced", func(t *testing.T) {
		ws := setupShipmentWorkspace(t)
		base := u2Create(t, ws, "U2 base root", "feature", "")
		top := u2Create(t, ws, "U2 top root", "feature", "", base)
		task := u2Create(t, ws, "U2 root task", "task", base.ID)
		ship := u2Ship(t, ws, "U2 two roots ship", task, top, base)

		result, err := ReorderShipment(ctx, ws, ship.ID, ShipmentManifestOptions{Auto: true})
		require.NoError(t, err)
		require.Equal(t, u2IDs(base, top, task), result.ItemsAfter)

		_, err = ReorderShipment(ctx, ws, ship.ID, ShipmentManifestOptions{Items: u2IDs(top, base, task), DryRun: true})
		require.ErrorIs(t, err, blerrors.ErrValidation)
		_, err = ReorderShipment(ctx, ws, ship.ID, ShipmentManifestOptions{Items: u2IDs(base, task, top), DryRun: true})
		require.ErrorIs(t, err, blerrors.ErrValidation, "every root must lead the manifest")
		ok, err := ReorderShipment(ctx, ws, ship.ID, ShipmentManifestOptions{Items: u2IDs(base, top, task), DryRun: true})
		require.NoError(t, err)
		require.Equal(t, u2IDs(base, top, task), ok.ItemsAfter)
	})
}
