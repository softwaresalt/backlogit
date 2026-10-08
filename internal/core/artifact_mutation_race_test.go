package core

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

func TestU172_Race_ArchiveItemAndAssociateCommitSameItemConverge(t *testing.T) {
	ctx := context.Background()
	ws := setupShipmentWorkspace(t)
	item, err := CreateArtifact(ctx, ws, "archive commit race item", "feature")
	require.NoError(t, err)
	ew := NewWorkspaceEventWriter(ws, WorkspaceLogsRoot(ws.RootPath))

	results := u20AwaitWithDeadline(t, 10*time.Second, map[string]func() error{
		"archive_item": func() error {
			_, archiveErr := ArchiveItem(ctx, ws.DB, ws, item.ID)
			return archiveErr
		},
		"associate_commit": func() error {
			return AssociateCommit(ctx, ws, ew, item.ID, strings.Repeat("c", 40), "concurrent commit", "u172-tester")
		},
	})

	for _, result := range results {
		requireU172NoLockOrderRaceFailure(t, result)
	}
	u20AssertArtifactCoherent(t, ws, item.ID)
}

func TestU172_Race_ReconcileArchivedLifecycleBatchAndSameItemWriterConverge(t *testing.T) {
	ctx := context.Background()
	ws := setupShipmentWorkspace(t)
	first, err := CreateArtifact(ctx, ws, "batch reconcile first", "feature")
	require.NoError(t, err)
	second, err := CreateArtifact(ctx, ws, "batch reconcile second", "feature")
	require.NoError(t, err)
	_, err = ArchiveItem(ctx, ws.DB, ws, first.ID)
	require.NoError(t, err)
	_, err = ArchiveItem(ctx, ws.DB, ws, second.ID)
	require.NoError(t, err)
	ew := NewWorkspaceEventWriter(ws, WorkspaceLogsRoot(ws.RootPath))

	results := u20AwaitWithDeadline(t, 10*time.Second, map[string]func() error{
		"reconcile_archived_lifecycle": func() error {
			_, reconcileErr := ReconcileArchivedLifecycle(ctx, ws.DB, ws, ReconciliationRequest{
				ItemIDs:      []string{first.ID, second.ID},
				TargetStatus: string(models.StatusDone),
				Reason:       "race regression",
				Actor:        "u172-tester",
			})
			return reconcileErr
		},
		"associate_commit": func() error {
			return AssociateCommit(ctx, ws, ew, first.ID, strings.Repeat("d", 40), "batch concurrent commit", "u172-tester")
		},
	})

	for _, result := range results {
		requireU172NoLockOrderRaceFailure(t, result)
	}
	u20AssertArtifactCoherent(t, ws, first.ID)
	u20AssertArtifactCoherent(t, ws, second.ID)
}

func requireU172NoLockOrderRaceFailure(t *testing.T, result u20GuardedResult) {
	t.Helper()
	require.Falsef(t, result.panicked, "%s panicked: %v", result.name, result.panicVal)
	if result.err == nil {
		return
	}
	require.Falsef(t, errors.Is(result.err, blerrors.ErrGateInProgress), "%s returned lock contention: %v", result.name, result.err)
	require.Falsef(t, errors.Is(result.err, ErrShipmentReconcileLockBusy), "%s returned reconcile lock contention: %v", result.name, result.err)
	require.NotContains(t, result.err.Error(), "item log lock", "%s returned item-log lock contention: %v", result.name, result.err)
}
