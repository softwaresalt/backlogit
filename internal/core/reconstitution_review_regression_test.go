package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/stretchr/testify/require"
)

func TestReviewAssociateCommitPreheldCRequiresGlobal(t *testing.T) {
	for _, kind := range []string{"shipment", "member"} {
		t.Run(kind, func(t *testing.T) {
			fx := newReconstituteFixture(t)
			id := fx.source.ID
			if kind == "member" {
				id = fx.tasks[0].ID
			}

			path, err := FindArtifactPath(context.Background(), fx.ws, id)
			require.NoError(t, err)
			before, err := os.ReadFile(path)
			require.NoError(t, err)
			// Hold global independently, as an archive writer would while
			// waiting for C. A C-only call must not even attempt global.
			_, releaseGlobal, err := lockShipmentLifecycleGlobal(context.Background(), fx.ws)
			require.NoError(t, err)
			defer func() { require.NoError(t, releaseGlobal()) }()
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			ctx = context.WithValue(ctx, shipmentLifecycleGlobalLockHookContextKey{}, func(phase string) {
				t.Errorf("C-only caller attempted lifecycle global: %s", phase)
			})
			ctx, releaseC, err := events.LockItemLogCrossProcess(ctx,
				WorkspaceLocksRoot(fx.ws.RootPath), WorkspaceLogsRoot(fx.ws.RootPath), id)
			require.NoError(t, err)
			defer releaseC()
			writer := NewWorkspaceEventWriter(fx.ws, WorkspaceLogsRoot(fx.ws.RootPath))
			err = AssociateCommit(ctx, fx.ws, writer, id, strings.Repeat("b", 40), "", "")
			var orderErr *ShipmentLifecycleLockOrderError
			require.ErrorAs(t, err, &orderErr)
			after, err := os.ReadFile(path)
			require.NoError(t, err)
			require.Equal(t, before, after)
			var count int
			require.NoError(t, fx.ws.DB.QueryRow("SELECT COUNT(*) FROM commit_links WHERE item_id = ?", id).Scan(&count))
			require.Zero(t, count)
		})
	}
}

func TestReviewAssociateCommitPreheldGlobalAndC(t *testing.T) {
	fx := newReconstituteFixture(t)
	ctx, releaseGlobal, err := lockShipmentLifecycleGlobal(context.Background(), fx.ws)
	require.NoError(t, err)
	defer func() { require.NoError(t, releaseGlobal()) }()
	ctx, releaseC, err := events.LockItemLogCrossProcess(ctx,
		WorkspaceLocksRoot(fx.ws.RootPath), WorkspaceLogsRoot(fx.ws.RootPath), fx.source.ID)
	require.NoError(t, err)
	defer releaseC()
	writer := NewWorkspaceEventWriter(fx.ws, WorkspaceLogsRoot(fx.ws.RootPath))
	require.NoError(t, AssociateCommit(ctx, fx.ws, writer, fx.source.ID, strings.Repeat("c", 40), "", ""))
}

func TestReviewPersistMemberPreheldCDoesNotAcquireGlobal(t *testing.T) {
	// A nonmember classified by AssociateCommit can become a member before
	// persist rechecks membership. The final acquisition boundary must also
	// reject C -> global, rather than relying on the earlier classification.
	fx := newReconstituteFixture(t)
	_, releaseGlobal, err := lockShipmentLifecycleGlobal(context.Background(), fx.ws)
	require.NoError(t, err)
	defer func() { require.NoError(t, releaseGlobal()) }()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	ctx = context.WithValue(ctx, shipmentLifecycleGlobalLockHookContextKey{}, func(phase string) {
		t.Errorf("persist attempted global while holding C: %s", phase)
	})
	ctx, releaseC, err := events.LockItemLogCrossProcess(ctx,
		WorkspaceLocksRoot(fx.ws.RootPath), WorkspaceLogsRoot(fx.ws.RootPath), fx.tasks[0].ID)
	require.NoError(t, err)
	defer releaseC()
	err = persistArtifact(ctx, fx.ws, cloneArtifact(fx.tasks[0]), false)
	var orderErr *ShipmentLifecycleLockOrderError
	require.ErrorAs(t, err, &orderErr)
}

func TestReviewAssociateCommitReadsRecoveredManifest(t *testing.T) {
	ctx := context.Background()
	fx := newReconstituteFixture(t)
	path, err := FindArtifactPath(ctx, fx.ws, fx.source.ID)
	require.NoError(t, err)
	preimage, err := os.ReadFile(path)
	require.NoError(t, err)
	originalItems := NormalizeShipmentItems(fx.source)

	torn := cloneArtifact(fx.source)
	torn.CustomFields["items"] = removeString(originalItems, fx.tasks[0].ID)
	require.NoError(t, persistArtifact(ctx, fx.ws, torn, false))

	corr := "0000000000000000000000000000d001"
	journal := reconstituteRollbackJournal(fx, corr)
	relativePath, err := filepath.Rel(fx.ws.StorageRoot, path)
	require.NoError(t, err)
	journal.Reconstitute.Images = []reconstituteFileImage{{
		Path: filepath.ToSlash(relativePath), Exists: true, Content: preimage,
	}}
	_, err = writeShipmentLifecycleJournalForWorkspace(fx.ws, shipmentLifecycleJournalName(corr), journal)
	require.NoError(t, err)

	writer := NewWorkspaceEventWriter(fx.ws, WorkspaceLogsRoot(fx.ws.RootPath))
	require.NoError(t, AssociateCommit(ctx, fx.ws, writer, fx.source.ID, strings.Repeat("a", 40), "review", "test"))
	recovered, err := findArtifact(ctx, fx.ws, fx.source.ID)
	require.NoError(t, err)
	require.Equal(t, originalItems, NormalizeShipmentItems(recovered))
}

func TestReviewReconstitutionRecoveryWithAbandonedStashSidecar(t *testing.T) {
	ctx := context.Background()
	fx := newReconstituteFixture(t)
	corr := "0000000000000000000000000000d002"
	_, err := writeShipmentLifecycleJournalForWorkspace(fx.ws, shipmentLifecycleJournalName(corr),
		reconstituteRollbackJournal(fx, corr))
	require.NoError(t, err)
	lockPath := StashFilePath(fx.ws.RootPath) + ".lock"
	require.NoError(t, os.WriteFile(lockPath, nil, 0o644))
	old := time.Now().Add(-2 * staleLockTTL)
	require.NoError(t, os.Chtimes(lockPath, old, old))

	reopened, err := NewWorkspace(ctx, fx.ws.RootPath)
	require.NoError(t, err, "recovery should handle a sidecar left by a crashed reconstitution")
	t.Cleanup(func() { require.NoError(t, reopened.Close()) })
}

func reviewStorageTree(t *testing.T, root string) map[string]string {
	t.Helper()
	tree := make(map[string]string)
	require.NoError(t, filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if info.IsDir() {
			tree[rel] = "<directory>"
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		tree[rel] = string(content)
		return nil
	}))
	return tree
}

func TestReviewColdReadOnlyPreviewDoesNotChangeStorageTree(t *testing.T) {
	ctx := context.Background()
	fx := newReconstituteFixture(t)
	require.NoError(t, fx.ws.Close())
	dbPath := filepath.Join(fx.ws.StorageRoot, "backlogit.db")
	require.NoFileExists(t, dbPath+"-wal")
	require.NoFileExists(t, dbPath+"-shm")
	before := reviewStorageTree(t, fx.ws.StorageRoot)

	ro, err := NewReadOnlyIndexWorkspace(ctx, fx.ws.RootPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ro.Close() })
	_, err = ReconstituteShipments(ctx, ro, ReconstituteOptions{PlanYAML: fx.splitPlanYAML(), DryRun: true})
	require.NoError(t, err)
	after := reviewStorageTree(t, fx.ws.StorageRoot)
	require.True(t, len(before) == len(after), "storage entry count changed: before=%d after=%d", len(before), len(after))
	for path, content := range before {
		require.Contains(t, after, path)
		require.True(t, content == after[path], "storage entry changed: %s", path)
	}
}
