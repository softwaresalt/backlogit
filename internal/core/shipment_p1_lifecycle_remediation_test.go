package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

func TestShipmentOperationJournalTempName(t *testing.T) {
	tests := []struct {
		name       string
		tempName   string
		wantTarget string
		wantOK     bool
	}{
		{
			name:       "lifecycle journal",
			tempName:   ".shipment-operation-shipment-operation-0123456789abcdef0123456789abcdef.json.tmp",
			wantTarget: "shipment-operation-0123456789abcdef0123456789abcdef.json",
			wantOK:     true,
		},
		{
			name:       "return blocked journal",
			tempName:   ".shipment-operation-return-blocked-155-S-155.001-T.json.tmp",
			wantTarget: "return-blocked-155-S-155.001-T.json",
			wantOK:     true,
		},
		{
			name:     "old random windows-style name is not attributable",
			tempName: ".shipment-operation-173946205.tmp",
		},
		{
			name:     "unknown target is not attributable",
			tempName: ".shipment-operation-unknown.json.tmp",
		},
		{
			name:     "arbitrary unknown entry",
			tempName: "unrelated.tmp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target, ok := shipmentOperationJournalTempTarget(tt.tempName)
			require.Equal(t, tt.wantOK, ok)
			require.Equal(t, tt.wantTarget, target)
		})
	}
}

func TestNewWorkspace_RemovesWriterTempResidueAndRecoversValidJournal(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feature, err := CreateArtifact(ctx, ws, "temp residue feature", "feature")
	require.NoError(t, err)
	item, err := CreateArtifact(ctx, ws, "temp residue item", "task", WithParent(feature.ID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "temp residue shipment", []string{item.ID})
	require.NoError(t, err)

	originalShipment := cloneArtifact(loadURCanonicalArtifact(t, ws, shipment.ID))
	originalItem := cloneArtifact(loadURCanonicalArtifact(t, ws, item.ID))
	require.NoError(t, writeReturnBlockedJournal(ws, originalShipment, originalItem))
	journalName := filepath.Base(returnBlockedJournalPath(ws.RootPath, shipment.ID, item.ID))
	tempName, err := shipmentOperationJournalTempName(journalName)
	require.NoError(t, err)
	tempPath := filepath.Join(shipmentOpsRoot(ws.RootPath), tempName)
	require.NoError(t, os.WriteFile(tempPath, []byte("crash-before-rename"), 0o600))

	tornShipment := cloneArtifact(originalShipment)
	tornShipment.CustomFields["items"] = removeString(NormalizeShipmentItems(tornShipment), item.ID)
	tornShipment.UpdatedAt = models.NowUTC()
	require.NoError(t, persistArtifact(ctx, ws, tornShipment, false))
	root := ws.RootPath
	require.NoError(t, ws.Close())

	reopened, err := NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reopened.Close()) })
	assertURArtifactEqual(t, originalShipment, loadURCanonicalArtifact(t, reopened, shipment.ID))
	assertURArtifactEqual(t, originalItem, loadURCanonicalArtifact(t, reopened, item.ID))
	_, err = os.Lstat(tempPath)
	require.ErrorIs(t, err, os.ErrNotExist)
}

func TestNewWorkspace_RejectsUnsafeWriterTempResidue(t *testing.T) {
	tests := []struct {
		name string
		seed func(*testing.T, string, string)
	}{
		{
			name: "non regular residue",
			seed: func(t *testing.T, _ string, tempPath string) {
				t.Helper()
				require.NoError(t, os.Mkdir(tempPath, 0o755))
			},
		},
		{
			name: "redirected residue",
			seed: func(t *testing.T, outsidePath, tempPath string) {
				t.Helper()
				if err := os.Symlink(outsidePath, tempPath); err != nil {
					t.Skipf("filesystem cannot create a file symlink: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			root := newShipmentOpsSecurityWorkspace(t)
			opsRoot := filepath.Join(root, ".backlogit", "ops")
			require.NoError(t, os.MkdirAll(opsRoot, 0o755))
			tempName, err := shipmentOperationJournalTempName(testShipmentOperationJournalName)
			require.NoError(t, err)
			tempPath := filepath.Join(opsRoot, tempName)
			outsidePath := filepath.Join(t.TempDir(), "outside")
			const outsideContent = "must remain untouched"
			require.NoError(t, os.WriteFile(outsidePath, []byte(outsideContent), 0o600))
			tt.seed(t, outsidePath, tempPath)

			ws, err := NewWorkspace(context.Background(), root)
			require.Error(t, err)
			require.Nil(t, ws)
			require.Equal(t, outsideContent, string(requireReadFile(t, outsidePath)))
		})
	}
}

func TestNormalizeBlockedShipmentForRecovery_RefusesExistingSameShipmentIntent(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	fixture := newURBlockedActiveFixture(t, ws)
	p021ForceOutOfBandBlockedShipment(t, ws, fixture)
	snapshotRef := p021WriteBlockedSnapshot(t, ws, fixture.shipment.ID)
	journal := p021LifecycleJournal(t, ws, "normalize", "roll_forward", fixture.shipment.ID, snapshotRef)
	p021WriteLifecycleJournal(t, ws, journal)
	opsRoot := shipmentOpsRoot(ws.RootPath)
	poisonPath := filepath.Join(opsRoot, "unrelated-poison.json")
	require.NoError(t, os.WriteFile(poisonPath, []byte(`{}`), 0o600))
	before := countShipmentLifecycleJournalFiles(t, opsRoot)

	_, err := NormalizeBlockedShipmentForRecovery(
		context.Background(),
		ws,
		fixture.shipment.ID,
		snapshotRef,
		"ownership test",
	)

	require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
	require.Equal(t, before, countShipmentLifecycleJournalFiles(t, opsRoot))
	_, statErr := os.Stat(poisonPath)
	require.NoError(t, statErr, "unrelated poison must remain available for diagnosis")
}

func TestNormalizeBlockedShipmentForRecovery_UnrelatedPoisonRemainsDiagnosable(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	fixture := newURBlockedActiveFixture(t, ws)
	p021ForceOutOfBandBlockedShipment(t, ws, fixture)
	snapshotRef := p021WriteBlockedSnapshot(t, ws, fixture.shipment.ID)
	poisonPath := filepath.Join(shipmentOpsRoot(ws.RootPath), "unrelated-poison.json")
	require.NoError(t, os.MkdirAll(filepath.Dir(poisonPath), 0o755))
	require.NoError(t, os.WriteFile(poisonPath, []byte(`{}`), 0o600))

	normalized, err := NormalizeBlockedShipmentForRecovery(
		context.Background(),
		ws,
		fixture.shipment.ID,
		snapshotRef,
		"diagnostic normalization",
	)

	require.NoError(t, err)
	require.Equal(t, models.StatusBlocked, normalized.Status)
	_, statErr := os.Stat(poisonPath)
	require.NoError(t, statErr)
	_, err = loadShipmentOperationJournals(ws)
	require.Error(t, err, "the poison entry must remain diagnosable")
}

func TestNormalizeBlockedShipmentForRecovery_RejectsMalformedCanonicalOwnershipJournal(t *testing.T) {
	tests := []struct {
		name          string
		correlationID string
		payload       []byte
		mutate        func(*shipmentLifecycleJournal)
	}{
		{
			name:          "semantic ownership is incomplete",
			correlationID: "44444444444444444444444444444444",
			payload: []byte(`{
  "schema_version": "shipment-operation/v1",
  "correlation_id": "44444444444444444444444444444444",
  "phase": "intent",
  "operation": "block",
  "recovery_policy": "rollback",
  "target": "blocked",
  "preimage": {}
}`),
		},
		{
			name:          "journal cannot be decoded",
			correlationID: "55555555555555555555555555555555",
			payload:       []byte(`{"schema_version":`),
		},
		{
			name:          "claim target contradicts producer tuple",
			correlationID: "66666666666666666666666666666666",
			mutate: func(journal *shipmentLifecycleJournal) {
				journal.Operation = "claim"
				journal.Target = string(ShipmentBlocked)
			},
		},
		{
			name:          "block rollback target contradicts producer tuple",
			correlationID: "77777777777777777777777777777777",
			mutate: func(journal *shipmentLifecycleJournal) {
				journal.Target = string(ShipmentActive)
			},
		},
		{
			name:          "block roll forward omits required snapshot",
			correlationID: "88888888888888888888888888888888",
			mutate: func(journal *shipmentLifecycleJournal) {
				journal.RecoveryPolicy = "roll_forward"
				journal.SnapshotRef = ""
			},
		},
		{
			name:          "unblock target contradicts producer tuple",
			correlationID: "99999999999999999999999999999999",
			mutate: func(journal *shipmentLifecycleJournal) {
				journal.Operation = "unblock"
				journal.Target = string(ShipmentBlocked)
			},
		},
		{
			name:          "normalize policy contradicts producer tuple",
			correlationID: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			mutate: func(journal *shipmentLifecycleJournal) {
				journal.Operation = "normalize"
				journal.RecoveryPolicy = "rollback"
				journal.SnapshotRef = "snapshots/semantic-tuple.json"
			},
		},
		{
			name:          "normalize roll forward omits required snapshot",
			correlationID: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			mutate: func(journal *shipmentLifecycleJournal) {
				journal.Operation = "normalize"
				journal.RecoveryPolicy = "roll_forward"
				journal.SnapshotRef = ""
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := setupShipmentWorkspace(t)
			fixture := newURBlockedActiveFixture(t, ws)
			p021ForceOutOfBandBlockedShipment(t, ws, fixture)
			snapshotRef := p021WriteBlockedSnapshot(t, ws, fixture.shipment.ID)
			payload := tt.payload
			if tt.mutate != nil {
				journal := p021LifecycleJournal(t, ws, "block", "rollback", fixture.shipment.ID, "")
				journal.CorrelationID = tt.correlationID
				tt.mutate(&journal)
				var marshalErr error
				payload, marshalErr = json.Marshal(journal)
				require.NoError(t, marshalErr)
			}
			journalPath := writeP1MalformedCanonicalLifecycleJournal(
				t,
				ws,
				tt.correlationID,
				payload,
			)
			before := snapshotURAggregate(t, ws, fixture.shipment.ID)

			_, err := NormalizeBlockedShipmentForRecovery(
				context.Background(),
				ws,
				fixture.shipment.ID,
				snapshotRef,
				"ownership validation",
			)

			require.ErrorIs(t, err, blerrors.ErrValidation)
			require.ErrorContains(t, err, filepath.Base(journalPath))
			requireURAggregateUnchanged(t, ws, before)

			report, doctorErr := Doctor(context.Background(), ws, &DoctorOptions{})
			require.NoError(t, doctorErr)
			require.True(t, hasUR3DoctorFinding(report, "ops", FindingInvalidShipmentLifecycleJournal))
		})
	}
}

func TestNormalizeBlockedShipmentForRecovery_ChecksOwnershipAfterGlobalLock(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	fixture := newURBlockedActiveFixture(t, ws)
	p021ForceOutOfBandBlockedShipment(t, ws, fixture)
	snapshotRef := p021WriteBlockedSnapshot(t, ws, fixture.shipment.ID)
	ctx := context.Background()
	unlock, err := lockShipmentMembership(ctx, ws, shipmentLifecycleGlobalLockID)
	require.NoError(t, err)

	started := make(chan struct{})
	result := make(chan error, 1)
	go func() {
		close(started)
		_, normalizeErr := NormalizeBlockedShipmentForRecovery(
			ctx,
			ws,
			fixture.shipment.ID,
			snapshotRef,
			"concurrent ownership",
		)
		result <- normalizeErr
	}()
	<-started
	select {
	case early := <-result:
		t.Fatalf("normalizer bypassed global lock: %v", early)
	case <-time.After(150 * time.Millisecond):
	}

	journal := p021LifecycleJournal(t, ws, "normalize", "roll_forward", fixture.shipment.ID, snapshotRef)
	p021WriteLifecycleJournal(t, ws, journal)
	require.NoError(t, unlock())

	select {
	case normalizeErr := <-result:
		require.ErrorIs(t, normalizeErr, blerrors.ErrShipmentConflict)
	case <-time.After(defaultGateLockBoundedWait + 2*time.Second):
		t.Fatal("normalizer did not finish after global lock release")
	}
	require.Equal(t, 1, countShipmentLifecycleJournalFiles(t, shipmentOpsRoot(ws.RootPath)))
}

func TestBlockShipment_CanonicalizesOptionalEnvelopeValuesBeforeWrites(t *testing.T) {
	tests := []struct {
		name           string
		branch         string
		blockedBy      string
		resumeRef      string
		wantBranch     string
		wantBlockedBy  string
		wantResumeRef  string
		wantEventActor string
	}{
		{
			name:           "trims_present_values",
			branch:         "  feat/canonical-block  ",
			blockedBy:      "  release operator  ",
			resumeRef:      "  checkpoints/resume.json  ",
			wantBranch:     "feat/canonical-block",
			wantBlockedBy:  "release operator",
			wantResumeRef:  "checkpoints/resume.json",
			wantEventActor: "release operator",
		},
		{
			name:           "omits_whitespace_only_values",
			branch:         " \t ",
			blockedBy:      "\n ",
			resumeRef:      " \r\n ",
			wantEventActor: "backlogit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := setupShipmentWorkspace(t)
			fixture := newURBlockedActiveFixture(t, ws)
			shipment := cloneArtifact(loadURCanonicalArtifact(t, ws, fixture.shipment.ID))
			shipment.CustomFields["branch"] = tt.branch
			forceURArtifactFixture(t, ws, shipment)

			blocked, err := BlockShipment(context.Background(), ws, fixture.shipment.ID, BlockOptions{
				Reason:              "canonical block",
				BlockedBy:           tt.blockedBy,
				ResumeCheckpointRef: tt.resumeRef,
			})
			require.NoError(t, err)
			requireP1OptionalEnvelopeValue(t, blocked.CustomFields, "branch", tt.wantBranch)
			requireP1OptionalEnvelopeValue(t, blocked.CustomFields, "blocked_by", tt.wantBlockedBy)
			requireP1OptionalEnvelopeValue(t, blocked.CustomFields, "resume_checkpoint_ref", tt.wantResumeRef)
			requireP1BlockedEnvelopeConsumersAccept(t, ws, blocked)
			requireP1LifecycleEvidenceCanonical(
				t,
				ws,
				blocked.ID,
				"block",
				tt.wantEventActor,
				tt.wantBranch,
				tt.wantBlockedBy,
				tt.wantResumeRef,
			)
			requireP1LifecycleJournalCanonical(
				t,
				ws,
				blocked.ID,
				"block",
				tt.wantBlockedBy,
				tt.wantResumeRef,
			)

			unblocked, err := UnblockShipment(context.Background(), ws, blocked.ID, UnblockOptions{
				Target:      ShipmentActive,
				Confirm:     true,
				UnblockedBy: "canonicalization test",
			})
			require.NoError(t, err)
			require.Equal(t, models.StatusActive, unblocked.Status)
		})
	}
}

func TestNormalizeBlockedShipment_CanonicalizesOptionalEnvelopeValuesBeforeWrites(t *testing.T) {
	tests := []struct {
		name          string
		branch        string
		blockedBy     string
		resumeRef     string
		wantBranch    string
		wantBlockedBy string
		wantResumeRef string
	}{
		{
			name:          "trims_present_values",
			branch:        "  feat/canonical-normalize  ",
			blockedBy:     "  snapshot operator  ",
			resumeRef:     "  checkpoints/normalize.json  ",
			wantBranch:    "feat/canonical-normalize",
			wantBlockedBy: "snapshot operator",
			wantResumeRef: "checkpoints/normalize.json",
		},
		{
			name:      "omits_whitespace_only_values",
			branch:    " \t ",
			blockedBy: "\n ",
			resumeRef: " \r\n ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := setupShipmentWorkspace(t)
			fixture := newURBlockedActiveFixture(t, ws)
			p021ForceOutOfBandBlockedShipment(t, ws, fixture)
			snapshotRef := writeP1BlockedSnapshot(
				t,
				ws,
				fixture.shipment.ID,
				tt.branch,
				tt.blockedBy,
				tt.resumeRef,
			)

			normalized, err := NormalizeBlockedShipment(
				context.Background(),
				ws,
				fixture.shipment.ID,
				snapshotRef,
				"  recovery operator  ",
			)
			require.NoError(t, err)
			requireP1OptionalEnvelopeValue(t, normalized.CustomFields, "branch", tt.wantBranch)
			requireP1OptionalEnvelopeValue(t, normalized.CustomFields, "blocked_by", tt.wantBlockedBy)
			requireP1OptionalEnvelopeValue(t, normalized.CustomFields, "resume_checkpoint_ref", tt.wantResumeRef)
			requireP1BlockedEnvelopeConsumersAccept(t, ws, normalized)
			requireP1LifecycleEvidenceCanonical(
				t,
				ws,
				normalized.ID,
				"normalize",
				"recovery operator",
				tt.wantBranch,
				tt.wantBlockedBy,
				tt.wantResumeRef,
			)
			requireP1LifecycleJournalCanonical(
				t,
				ws,
				normalized.ID,
				"normalize",
				"recovery operator",
				snapshotRef,
			)

			unblocked, err := UnblockShipment(context.Background(), ws, normalized.ID, UnblockOptions{
				Target:      ShipmentActive,
				Confirm:     true,
				UnblockedBy: "canonicalization test",
			})
			require.NoError(t, err)
			require.Equal(t, models.StatusActive, unblocked.Status)
		})
	}
}

func TestBlockRecovery_CanonicalizesAppliedTargetForCAS(t *testing.T) {
	tests := []struct {
		name      string
		branch    string
		blockedBy string
		resumeRef string
	}{
		{
			name:      "trims_present_values",
			branch:    "  feat/recovery-canonical  ",
			blockedBy: "  recovery operator  ",
			resumeRef: "  checkpoints/recovery.json  ",
		},
		{
			name:      "omits_whitespace_only_values",
			branch:    " \t ",
			blockedBy: "\n ",
			resumeRef: " \r\n ",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := setupShipmentWorkspace(t)
			fixture := newURBlockedActiveFixture(t, ws)
			shipment := cloneArtifact(loadURCanonicalArtifact(t, ws, fixture.shipment.ID))
			shipment.CustomFields["branch"] = tt.branch
			forceURArtifactFixture(t, ws, shipment)

			blocked, err := BlockShipment(context.Background(), ws, fixture.shipment.ID, BlockOptions{
				Reason:              "recover canonical applied target",
				BlockedBy:           tt.blockedBy,
				ResumeCheckpointRef: tt.resumeRef,
			})
			require.NoError(t, err)
			_, err = validatePersistedBlockedShipmentEnvelope(context.Background(), ws, blocked)
			require.NoError(t, err, "post-crash applied target must satisfy the shared envelope validator")

			records, err := loadShipmentOperationJournals(ws)
			require.NoError(t, err)
			var journalPath string
			var journal shipmentLifecycleJournal
			for _, record := range records {
				if record.kind == shipmentLifecycleJournalKind &&
					record.lifecycle.Operation == "block" &&
					record.lifecycle.ShipmentID == fixture.shipment.ID {
					journalPath = record.path
					journal = record.lifecycle
					break
				}
			}
			require.NotEmpty(t, journalPath)
			journal.Phase = "intent"
			_, err = writeShipmentLifecycleJournalForWorkspace(ws, filepath.Base(journalPath), journal)
			require.NoError(t, err)

			require.NoError(t, recoverPendingShipmentOperations(context.Background(), ws),
				"canonical post-crash applied target must satisfy recovery CAS")
			require.Equal(t, "committed", p021ReadLifecycleJournal(t, journalPath).Phase)
			assertURArtifactEqual(t, blocked, loadURCanonicalArtifact(t, ws, fixture.shipment.ID))
		})
	}
}

func TestLifecycleMutators_RunPendingRecoveryBarrierBeforeAggregateAccess(t *testing.T) {
	tests := []struct {
		name    string
		prepare func(*testing.T, *Workspace) func(context.Context) error
	}{
		{
			name: "ship",
			prepare: func(t *testing.T, ws *Workspace) func(context.Context) error {
				t.Helper()
				shipment, err := CreateShipment(context.Background(), ws, "barrier ship", nil)
				require.NoError(t, err)
				return func(ctx context.Context) error {
					_, shipErr := ShipShipment(ctx, ws, shipment.ID, nil)
					return shipErr
				}
			},
		},
		{
			name: "add_item",
			prepare: func(t *testing.T, ws *Workspace) func(context.Context) error {
				t.Helper()
				feature, err := CreateArtifact(context.Background(), ws, "barrier add feature", "feature")
				require.NoError(t, err)
				item, err := CreateArtifact(
					context.Background(),
					ws,
					"barrier add item",
					"task",
					WithParent(feature.ID),
				)
				require.NoError(t, err)
				shipment, err := CreateShipment(context.Background(), ws, "barrier add shipment", nil)
				require.NoError(t, err)
				return func(ctx context.Context) error {
					return AddItemToShipment(ctx, ws, shipment.ID, item.ID)
				}
			},
		},
		{
			name: "return_blocked",
			prepare: func(t *testing.T, ws *Workspace) func(context.Context) error {
				t.Helper()
				feature, err := CreateArtifact(context.Background(), ws, "barrier return feature", "feature")
				require.NoError(t, err)
				item, err := CreateArtifact(
					context.Background(),
					ws,
					"barrier return item",
					"task",
					WithParent(feature.ID),
				)
				require.NoError(t, err)
				shipment, err := CreateShipment(
					context.Background(),
					ws,
					"barrier return shipment",
					[]string{item.ID},
				)
				require.NoError(t, err)
				return func(ctx context.Context) error {
					return ReturnBlockedItem(ctx, ws, shipment.ID, item.ID, "barrier refusal")
				}
			},
		},
		{
			name: "archive",
			prepare: func(t *testing.T, ws *Workspace) func(context.Context) error {
				t.Helper()
				artifact, err := CreateArtifact(context.Background(), ws, "barrier archive", "feature")
				require.NoError(t, err)
				return func(ctx context.Context) error {
					_, archiveErr := ArchiveItem(ctx, ws.DB, ws, artifact.ID)
					return archiveErr
				}
			},
		},
		{
			name: "generic_update",
			prepare: func(t *testing.T, ws *Workspace) func(context.Context) error {
				t.Helper()
				artifact, err := CreateArtifact(context.Background(), ws, "barrier generic update", "feature")
				require.NoError(t, err)
				return func(ctx context.Context) error {
					_, updateErr := UpdateArtifact(ctx, ws, artifact.ID, map[string]any{
						"title": "barrier generic update attempted",
					})
					return updateErr
				}
			},
		},
		{
			name: "bulk_update",
			prepare: func(t *testing.T, ws *Workspace) func(context.Context) error {
				t.Helper()
				parent, err := CreateArtifact(context.Background(), ws, "barrier bulk parent", "feature")
				require.NoError(t, err)
				artifact, err := CreateArtifact(
					context.Background(),
					ws,
					"barrier bulk update",
					"task",
					WithParent(parent.ID),
				)
				require.NoError(t, err)
				return func(ctx context.Context) error {
					result, bulkErr := BulkUpdateStatus(
						ctx,
						ws.DB,
						ws,
						[]string{artifact.ID},
						string(models.StatusActive),
					)
					if bulkErr != nil {
						return bulkErr
					}
					if result.Succeeded != 1 || len(result.Failed) != 0 {
						return errors.New("bulk update did not complete exactly once")
					}
					return nil
				}
			},
		},
		{
			name: "cascade_update",
			prepare: func(t *testing.T, ws *Workspace) func(context.Context) error {
				t.Helper()
				parent, err := CreateArtifact(context.Background(), ws, "barrier cascade parent", "feature")
				require.NoError(t, err)
				child, err := CreateArtifact(
					context.Background(),
					ws,
					"barrier cascade child",
					"task",
					WithParent(parent.ID),
					WithStatus(string(models.StatusActive)),
				)
				require.NoError(t, err)
				return func(ctx context.Context) error {
					return cascadePersistedParentStatuses(ctx, ws, child.ID)
				}
			},
		},
		{
			name: "doctor_fix_orphans",
			prepare: func(t *testing.T, ws *Workspace) func(context.Context) error {
				t.Helper()
				feature, err := CreateArtifact(context.Background(), ws, "barrier doctor feature", "feature")
				require.NoError(t, err)
				orphan, err := CreateArtifact(
					context.Background(),
					ws,
					"barrier doctor orphan",
					"task",
					WithParent(feature.ID),
				)
				require.NoError(t, err)
				orphan = cloneArtifact(loadURCanonicalArtifact(t, ws, orphan.ID))
				orphan.ParentID = ""
				forceURArtifactFixture(t, ws, orphan)
				return func(ctx context.Context) error {
					_, doctorErr := Doctor(ctx, ws, &DoctorOptions{
						CheckOrphans: true,
						FixOrphans:   true,
					})
					return doctorErr
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := setupShipmentWorkspace(t)
			act := tt.prepare(t, ws)
			journalPath := writeP1MalformedCanonicalLifecycleJournal(
				t,
				ws,
				"44444444444444444444444444444444",
				[]byte(`{
  "schema_version": "shipment-operation/v1",
  "correlation_id": "44444444444444444444444444444444",
  "phase": "intent",
  "operation": "block",
  "recovery_policy": "rollback",
  "target": "blocked",
  "preimage": {}
}`),
			)
			before := snapshotURWorkspace(t, ws)
			ctx, _, acquired := p021ObserveGlobalLock(context.Background())

			err := act(ctx)

			require.ErrorIs(t, err, blerrors.ErrValidation)
			require.ErrorContains(t, err, filepath.Base(journalPath))
			select {
			case <-acquired:
			default:
				t.Fatal("mutator did not acquire the workspace-global lifecycle lock before recovery")
			}
			requireURAggregateUnchanged(t, ws, before)
		})
	}
}

func TestNormalizeBlockedShipment_RejectsWhitespaceActorBeforeWrites(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	fixture := newURBlockedActiveFixture(t, ws)
	p021ForceOutOfBandBlockedShipment(t, ws, fixture)
	snapshotRef := writeP1BlockedSnapshot(
		t,
		ws,
		fixture.shipment.ID,
		"feat/required-actor",
		"snapshot operator",
		"checkpoints/required-actor.json",
	)
	before := snapshotURAggregate(t, ws, fixture.shipment.ID)

	_, err := NormalizeBlockedShipment(
		context.Background(),
		ws,
		fixture.shipment.ID,
		snapshotRef,
		" \t ",
	)

	require.ErrorIs(t, err, blerrors.ErrValidation)
	requireURAggregateUnchanged(t, ws, before)
}

func requireP1OptionalEnvelopeValue(t *testing.T, fields map[string]any, key, want string) {
	t.Helper()

	got, found := fields[key]
	if want == "" {
		require.False(t, found, "%s must be absent when its normalized value is empty", key)
		return
	}
	require.True(t, found, "%s must be present", key)
	require.Equal(t, want, got)
}

func requireP1BlockedEnvelopeConsumersAccept(
	t *testing.T,
	ws *Workspace,
	shipment *models.Artifact,
) {
	t.Helper()

	_, err := validatePersistedBlockedShipmentEnvelope(context.Background(), ws, shipment)
	require.NoError(t, err, "shared blocked-envelope validator must accept successful output")
	report, err := Doctor(context.Background(), ws, &DoctorOptions{})
	require.NoError(t, err)
	for _, finding := range report.Findings {
		require.False(
			t,
			finding.Type == FindingMalformedBlockedShipment && finding.ArtifactID == shipment.ID,
			"doctor must accept successful output: %+v",
			finding,
		)
	}
}

func requireP1LifecycleEvidenceCanonical(
	t *testing.T,
	ws *Workspace,
	shipmentID string,
	operation string,
	wantActor string,
	wantBranch string,
	wantBlockedBy string,
	wantResumeRef string,
) {
	t.Helper()

	found := 0
	for _, event := range readUREvents(t, ws, shipmentID) {
		if event.Delta["operation"] != operation {
			continue
		}
		found++
		require.Equal(t, wantActor, event.Actor)
		requireP1OptionalEnvelopeValue(t, event.Delta, "branch", wantBranch)
		requireP1OptionalEnvelopeValue(t, event.Delta, "blocked_by", wantBlockedBy)
		requireP1OptionalEnvelopeValue(t, event.Delta, "resume_checkpoint_ref", wantResumeRef)
		if operation == "normalize" {
			require.Equal(t, wantActor, event.Delta["normalized_by"])
		}
	}
	require.Positive(t, found, "expected lifecycle evidence for %s", operation)
}

func requireP1LifecycleJournalCanonical(
	t *testing.T,
	ws *Workspace,
	shipmentID string,
	operation string,
	wantActor string,
	wantSnapshotRef string,
) {
	t.Helper()

	records, err := loadShipmentOperationJournals(ws)
	require.NoError(t, err)
	found := 0
	for _, record := range records {
		if record.kind != shipmentLifecycleJournalKind ||
			record.lifecycle.ShipmentID != shipmentID ||
			record.lifecycle.Operation != operation {
			continue
		}
		found++
		require.Equal(t, wantActor, record.lifecycle.BlockedBy)
		require.Equal(t, wantSnapshotRef, record.lifecycle.SnapshotRef)
	}
	require.Equal(t, 1, found, "expected one lifecycle journal for %s", operation)
}

func writeP1BlockedSnapshot(
	t *testing.T,
	ws *Workspace,
	shipmentID string,
	branch string,
	blockedBy string,
	resumeRef string,
) string {
	t.Helper()

	shipment := loadURCanonicalArtifact(t, ws, shipmentID)
	snapshot := ShipmentBlockedSnapshot{
		SchemaVersion:       ShipmentBlockedSnapshotSchemaVersion,
		ShipmentID:          shipmentID,
		Branch:              branch,
		Target:              ShipmentBlocked,
		BlockedReason:       "canonical normalization",
		BlockedAt:           time.Now().UTC().Format(time.RFC3339),
		BlockedBy:           blockedBy,
		ResumeCheckpointRef: resumeRef,
		Members:             statusSnapshotUR(shipment.CustomFields["member_status_snapshot"]),
	}
	data, err := json.Marshal(snapshot)
	require.NoError(t, err)
	relativePath := filepath.Join("snapshots", "p1-canonical-"+shipmentID+".json")
	absolutePath := filepath.Join(WorkspaceStorageRoot(ws.RootPath), relativePath)
	require.NoError(t, os.MkdirAll(filepath.Dir(absolutePath), 0o755))
	require.NoError(t, os.WriteFile(absolutePath, data, 0o644))
	return filepath.ToSlash(relativePath)
}

func writeP1MalformedCanonicalLifecycleJournal(
	t *testing.T,
	ws *Workspace,
	correlationID string,
	payload []byte,
) string {
	t.Helper()

	opsRoot := shipmentOpsRoot(ws.RootPath)
	require.NoError(t, os.MkdirAll(opsRoot, 0o755))
	path := filepath.Join(opsRoot, shipmentLifecycleJournalName(correlationID))
	require.NoError(t, os.WriteFile(path, payload, 0o600))
	return path
}

func TestReturnBlockedRecovery_TargetIntentConvergesWithoutUndo(t *testing.T) {
	tests := []struct {
		name         string
		seedEvidence bool
	}{
		{name: "crash before evidence"},
		{name: "crash after evidence", seedEvidence: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := setupShipmentWorkspace(t)
			journal := newReturnBlockedCrashJournal(t, ws, "target intent "+tt.name)
			require.NoError(t, writeReturnBlockedJournalRecord(ws, journal))
			require.NoError(t, persistArtifact(context.Background(), ws, journal.TargetShipment, false))
			require.NoError(t, persistArtifact(context.Background(), ws, journal.TargetItem, true))
			if tt.seedEvidence {
				require.NoError(t, appendReturnBlockedEvidence(context.Background(), ws, journal))
			}
			beforeEvidence := countReturnBlockedEvidence(t, ws, journal)
			root := ws.RootPath
			require.NoError(t, ws.Close())

			reopened, err := NewWorkspace(context.Background(), root)
			require.NoError(t, err)
			t.Cleanup(func() { require.NoError(t, reopened.Close()) })
			assertURArtifactEqual(t, journal.TargetShipment,
				loadURCanonicalArtifact(t, reopened, journal.Shipment.ID))
			assertURArtifactEqual(t, journal.TargetItem,
				loadURCanonicalArtifact(t, reopened, journal.Item.ID))
			require.Equal(t, 2, countReturnBlockedEvidence(t, reopened, journal))
			if tt.seedEvidence {
				require.Equal(t, beforeEvidence, countReturnBlockedEvidence(t, reopened, journal))
			}
			_, err = os.Stat(returnBlockedJournalPath(root, journal.Shipment.ID, journal.Item.ID))
			require.ErrorIs(t, err, os.ErrNotExist)

			require.NoError(t, recoverPendingShipmentOperations(context.Background(), reopened))
			require.Equal(t, 2, countReturnBlockedEvidence(t, reopened, journal),
				"recovery must remain idempotent after finalization")
		})
	}
}

func TestReturnBlockedRecovery_IntentPartialRollsBackExactly(t *testing.T) {
	tests := []struct {
		name string
		lock func(context.Context, *Workspace, returnBlockedJournal) (func() error, error)
	}{
		{
			name: "membership lock contention",
			lock: func(ctx context.Context, ws *Workspace, journal returnBlockedJournal) (func() error, error) {
				return lockShipmentMembership(ctx, ws, journal.Shipment.ID)
			},
		},
		{
			name: "artifact lock contention",
			lock: func(ctx context.Context, ws *Workspace, journal returnBlockedJournal) (func() error, error) {
				ids := []string{journal.Shipment.ID, journal.Item.ID}
				sort.Strings(ids)
				unlocks := make([]func() error, 0, len(ids))
				release := func() error {
					var errs []error
					for i := len(unlocks) - 1; i >= 0; i-- {
						errs = append(errs, unlocks[i]())
					}
					return errors.Join(errs...)
				}
				for _, id := range ids {
					lockPath, err := artifactMutationLockPath(ws, id)
					if err != nil {
						return nil, errors.Join(err, release())
					}
					unlock, err := lockTaskFileWithHeartbeat(
						ctx,
						lockPath,
						defaultGateLockBoundedWait,
						defaultGateLockHeartbeat,
					)
					if err != nil {
						return nil, errors.Join(err, release())
					}
					unlocks = append(unlocks, unlock)
				}
				return release, nil
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ws := setupShipmentWorkspace(t)
			journal := newReturnBlockedCrashJournal(t, ws, "partial rollback "+tt.name)
			require.NoError(t, writeReturnBlockedJournalRecord(ws, journal))
			require.NoError(t, persistArtifact(context.Background(), ws, journal.TargetShipment, false))

			unlock, err := tt.lock(context.Background(), ws, journal)
			require.NoError(t, err)
			locked := true
			t.Cleanup(func() {
				if locked {
					require.NoError(t, unlock())
				}
			})

			started := make(chan struct{})
			recovered := make(chan error, 1)
			go func() {
				close(started)
				recovered <- recoverPendingShipmentOperations(context.Background(), ws)
			}()
			<-started
			select {
			case earlyErr := <-recovered:
				t.Fatalf("return-blocked recovery bypassed %s: %v", tt.name, earlyErr)
			case <-time.After(150 * time.Millisecond):
			}

			require.NoError(t, unlock())
			locked = false
			select {
			case recoveryErr := <-recovered:
				require.NoError(t, recoveryErr)
			case <-time.After(defaultGateLockBoundedWait + 2*time.Second):
				t.Fatalf("return-blocked recovery did not finish after releasing %s", tt.name)
			}

			assertURArtifactEqual(t, journal.Shipment, loadURCanonicalArtifact(t, ws, journal.Shipment.ID))
			assertURArtifactEqual(t, journal.Item, loadURCanonicalArtifact(t, ws, journal.Item.ID))
			require.Zero(t, countReturnBlockedEvidence(t, ws, journal))
			_, err = os.Stat(returnBlockedJournalPath(ws.RootPath, journal.Shipment.ID, journal.Item.ID))
			require.ErrorIs(t, err, os.ErrNotExist)
		})
	}
}

func TestDoctor_DoesNotDeleteShipmentOperationTempEvidence(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	journalName := shipmentLifecycleJournalName("cccccccccccccccccccccccccccccccc")
	tempName, err := shipmentOperationJournalTempName(journalName)
	require.NoError(t, err)
	tempPath := filepath.Join(shipmentOpsRoot(ws.RootPath), tempName)
	require.NoError(t, os.MkdirAll(filepath.Dir(tempPath), 0o755))
	const evidence = "incomplete-writer-evidence"
	require.NoError(t, os.WriteFile(tempPath, []byte(evidence), 0o600))
	require.Equal(t, evidence, string(requireReadFile(t, tempPath)))
	_, _, err = inspectShipmentOperationJournalsReadOnly(ws)
	require.NoError(t, err)
	require.Equal(t, evidence, string(requireReadFile(t, tempPath)))

	_, err = Doctor(context.Background(), ws, &DoctorOptions{})

	require.NoError(t, err)
	require.Equal(t, evidence, string(requireReadFile(t, tempPath)))
}

func TestReturnBlockedRecovery_RefusesDriftWithoutMutation(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	journal := newReturnBlockedCrashJournal(t, ws, "drift refusal")
	require.NoError(t, writeReturnBlockedJournalRecord(ws, journal))
	require.NoError(t, persistArtifact(context.Background(), ws, journal.TargetShipment, false))
	driftedItem := cloneArtifact(journal.Item)
	driftedItem.Title = "concurrent edit"
	driftedItem.UpdatedAt = models.NowUTC()
	require.NoError(t, persistArtifact(context.Background(), ws, driftedItem, true))
	before := snapshotURAggregate(t, ws, journal.Shipment.ID)

	err := recoverPendingShipmentOperations(context.Background(), ws)

	require.ErrorIs(t, err, blerrors.ErrShipmentConflict)
	requireURAggregateUnchanged(t, ws, before)
	persisted := readReturnBlockedJournal(t, ws, journal.Shipment.ID, journal.Item.ID)
	require.Equal(t, "intent", persisted.Phase)
}

func TestReturnBlockedItem_CommittedJournalSurvivesCleanupFailure(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feature, err := CreateArtifact(ctx, ws, "cleanup failure feature", "feature")
	require.NoError(t, err)
	item, err := CreateArtifact(ctx, ws, "cleanup failure item", "task", WithParent(feature.ID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "cleanup failure shipment", []string{item.ID})
	require.NoError(t, err)
	injected := errors.New("injected cleanup failure")
	ws.removeShipmentOperationJournal = func(string) error { return injected }

	require.NoError(t, ReturnBlockedItem(ctx, ws, shipment.ID, item.ID, "cleanup failed after success"))
	journal := readReturnBlockedJournal(t, ws, shipment.ID, item.ID)
	require.Equal(t, "committed", journal.Phase)
	assertURArtifactEqual(t, journal.TargetShipment, loadURCanonicalArtifact(t, ws, shipment.ID))
	assertURArtifactEqual(t, journal.TargetItem, loadURCanonicalArtifact(t, ws, item.ID))
	require.Equal(t, 2, countReturnBlockedEvidence(t, ws, journal))

	ws.removeShipmentOperationJournal = nil
	root := ws.RootPath
	require.NoError(t, ws.Close())
	reopened, err := NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, reopened.Close()) })
	assertURArtifactEqual(t, journal.TargetShipment, loadURCanonicalArtifact(t, reopened, shipment.ID))
	assertURArtifactEqual(t, journal.TargetItem, loadURCanonicalArtifact(t, reopened, item.ID))
	require.Equal(t, 2, countReturnBlockedEvidence(t, reopened, journal))
	_, err = os.Stat(returnBlockedJournalPath(root, shipment.ID, item.ID))
	require.ErrorIs(t, err, os.ErrNotExist)
}

func newReturnBlockedCrashJournal(t *testing.T, ws *Workspace, reason string) returnBlockedJournal {
	t.Helper()

	ctx := context.Background()
	feature, err := CreateArtifact(ctx, ws, reason+" feature", "feature")
	require.NoError(t, err)
	item, err := CreateArtifact(ctx, ws, reason+" item", "task", WithParent(feature.ID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, reason+" shipment", []string{item.ID})
	require.NoError(t, err)
	preimageShipment := cloneArtifact(loadURCanonicalArtifact(t, ws, shipment.ID))
	preimageItem := cloneArtifact(loadURCanonicalArtifact(t, ws, item.ID))
	targetShipment := cloneArtifact(preimageShipment)
	targetShipment.CustomFields["items"] = removeString(NormalizeShipmentItems(targetShipment), item.ID)
	targetShipment.UpdatedAt = models.NowUTC()
	targetItem := cloneArtifact(preimageItem)
	targetItem.Status = models.StatusBlocked
	if targetItem.CustomFields == nil {
		targetItem.CustomFields = map[string]any{}
	}
	targetItem.CustomFields["blocked_reason"] = reason
	targetItem.UpdatedAt = models.NowUTC()

	journal, err := newReturnBlockedJournal(
		preimageShipment,
		preimageItem,
		targetShipment,
		targetItem,
		reason,
	)
	require.NoError(t, err)
	return journal
}

func countShipmentLifecycleJournalFiles(t *testing.T, opsRoot string) int {
	t.Helper()

	entries, err := os.ReadDir(opsRoot)
	require.NoError(t, err)
	count := 0
	for _, entry := range entries {
		if shipmentLifecycleJournalNamePattern.MatchString(entry.Name()) {
			count++
		}
	}
	return count
}

func readReturnBlockedJournal(t *testing.T, ws *Workspace, shipmentID, itemID string) returnBlockedJournal {
	t.Helper()

	data, err := os.ReadFile(returnBlockedJournalPath(ws.RootPath, shipmentID, itemID))
	require.NoError(t, err)
	var journal returnBlockedJournal
	require.NoError(t, decodeShipmentOperationJournal(data, &journal))
	return journal
}

func countReturnBlockedEvidence(t *testing.T, ws *Workspace, journal returnBlockedJournal) int {
	t.Helper()

	count := 0
	for _, itemID := range []string{journal.Shipment.ID, journal.Item.ID} {
		for _, event := range readUREvents(t, ws, itemID) {
			if isShipmentOperationEvent(event, journal.CorrelationID) {
				count++
			}
		}
	}
	return count
}
