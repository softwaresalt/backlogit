package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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
	ws := setupShipmentWorkspace(t)
	journal := newReturnBlockedCrashJournal(t, ws, "partial rollback")
	require.NoError(t, writeReturnBlockedJournalRecord(ws, journal))
	require.NoError(t, persistArtifact(context.Background(), ws, journal.TargetShipment, false))

	require.NoError(t, recoverPendingShipmentOperations(context.Background(), ws))
	assertURArtifactEqual(t, journal.Shipment, loadURCanonicalArtifact(t, ws, journal.Shipment.ID))
	assertURArtifactEqual(t, journal.Item, loadURCanonicalArtifact(t, ws, journal.Item.ID))
	require.Zero(t, countReturnBlockedEvidence(t, ws, journal))
	_, err := os.Stat(returnBlockedJournalPath(ws.RootPath, journal.Shipment.ID, journal.Item.ID))
	require.ErrorIs(t, err, os.ErrNotExist)
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
