package core

import (
	"context"
	"errors"
	"fmt"
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

func TestU172_Race_CascadeArchiveAndChildAssociateCommitConverge(t *testing.T) {
	ctx := context.Background()
	ws := setupShipmentWorkspace(t)
	parent, err := CreateArtifact(ctx, ws, "cascade race parent", "feature")
	require.NoError(t, err)
	child, err := CreateArtifact(ctx, ws, "cascade race child", "task", WithParent(parent.ID))
	require.NoError(t, err)
	ew := NewWorkspaceEventWriter(ws, WorkspaceLogsRoot(ws.RootPath))

	results := u20AwaitWithDeadline(t, 10*time.Second, map[string]func() error{
		"cascade_archive_parent": func() error {
			record, archiveErr := ArchiveItem(ctx, ws.DB, ws, parent.ID, WithCascade(true))
			if archiveErr == nil && len(record.FailedItems) > 0 {
				return fmt.Errorf("cascade failed items: %+v", record.FailedItems)
			}
			return archiveErr
		},
		"associate_commit_child": func() error {
			return AssociateCommit(ctx, ws, ew, child.ID, strings.Repeat("e", 40), "cascade concurrent commit", "u172-tester")
		},
	})

	var commitErr error
	for _, result := range results {
		requireU172NoLockOrderRaceFailure(t, result)
		if result.name == "cascade_archive_parent" {
			require.NoError(t, result.err)
		}
		if result.name == "associate_commit_child" {
			commitErr = result.err
		}
	}
	u20AssertArtifactCoherent(t, ws, parent.ID)
	u20AssertArtifactCoherent(t, ws, child.ID)
	require.Contains(t, readU172ItemLogEventTypes(t, ws, parent.ID), "archived")
	childEvents := readU172ItemLogEventTypes(t, ws, child.ID)
	require.Contains(t, childEvents, "archived")
	if commitErr == nil {
		require.Contains(t, childEvents, "commit_tracked")
	}
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
	if errors.Is(result.err, ErrTaskBusy) {
		// AssociateCommit takes the artifact mutation lock with TryLock by design,
		// so a concurrent archive/reconcile on the same item may legitimately make
		// it refuse cleanly. That refusal must be a not-applied partial with no
		// side effects; bounded-wait writers must never surface ErrTaskBusy.
		require.Truef(t, strings.HasPrefix(result.name, "associate_commit"), "%s returned artifact lock contention: %v", result.name, result.err)
		var partialErr *blerrors.MutationPartialError
		require.Truef(t, errors.As(result.err, &partialErr), "%s try-lock refusal must be a mutation partial: %v", result.name, result.err)
		require.Equalf(t, "not-applied", partialErr.Class, "%s try-lock refusal must be not-applied: %v", result.name, result.err)
	}
	require.NotContains(t, result.err.Error(), "item log lock", "%s returned item-log lock contention: %v", result.name, result.err)
}
