package core

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

type claimFlatArtifactSnapshot struct {
	fileContent []byte
	database    *models.Artifact
	eventLog    []byte
}

func TestClaimShipmentFlatScope_UnlistedAncestorIsNotLockedOrMutated(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	ancestor, err := CreateArtifact(ctx, ws, "claim flat ancestor", "feature")
	require.NoError(t, err)
	member, err := CreateArtifact(ctx, ws, "claim flat member", "task", WithParent(ancestor.ID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "claim flat shipment", []string{member.ID})
	require.NoError(t, err)
	before := snapshotClaimFlatArtifact(t, ws, ancestor.ID)

	probed := false
	ancestorLocked := false
	persistArtifactPreLockHook = func(id string) {
		if id != shipment.ID || probed {
			return
		}
		probed = true
		ancestorLocked = flatScopeLockProbe(t, ws, ancestor.ID)
	}
	t.Cleanup(func() { persistArtifactPreLockHook = nil })

	_, err = ClaimShipment(ctx, ws, shipment.ID)

	require.NoError(t, err)
	require.True(t, probed, "claim must probe while its explicit aggregate locks are held")
	require.False(t, ancestorLocked, "claim must not lock a hierarchy-derived non-member ancestor")
	requireClaimFlatArtifactUnchanged(t, ws, ancestor.ID, before)
	require.Equal(t, models.StatusActive, loadURCanonicalArtifact(t, ws, member.ID).Status)
}

func TestClaimShipmentFlatScope_CompensationDoesNotRestoreUnlistedAncestor(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	ancestor, err := CreateArtifact(ctx, ws, "claim compensation ancestor", "feature")
	require.NoError(t, err)
	member, err := CreateArtifact(ctx, ws, "claim compensation member", "task", WithParent(ancestor.ID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "claim compensation shipment", []string{member.ID})
	require.NoError(t, err)
	beforeAncestor := snapshotClaimFlatArtifact(t, ws, ancestor.ID)
	beforeAggregate := snapshotURGovernedState(t, ws, []string{shipment.ID, member.ID})

	injectedErr := errors.New("injected explicit-member activation failure")
	originalWriter := persistArtifactWriteFn
	persistArtifactWriteFn = func(artifact *models.Artifact, filePath string, durable bool) error {
		if artifact.ID == member.ID && artifact.Status == models.StatusActive {
			return injectedErr
		}
		return originalWriter(artifact, filePath, durable)
	}
	t.Cleanup(func() { persistArtifactWriteFn = originalWriter })

	_, err = ClaimShipment(ctx, ws, shipment.ID)

	require.ErrorIs(t, err, injectedErr)
	requireClaimFlatArtifactUnchanged(t, ws, ancestor.ID, beforeAncestor)
	requireP1C6DurableStateRestored(t, ws, beforeAggregate)
}

func TestClaimShipmentFlatScope_ExplicitFeatureMemberActivatesDirectly(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feature, err := CreateArtifact(ctx, ws, "claim listed feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "claim listed task", "task", WithParent(feature.ID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "claim listed feature shipment", []string{feature.ID, task.ID})
	require.NoError(t, err)

	_, err = ClaimShipment(ctx, ws, shipment.ID)

	require.NoError(t, err)
	require.Equal(t, models.StatusActive, loadURCanonicalArtifact(t, ws, feature.ID).Status)
	require.Equal(t, models.StatusActive, loadURCanonicalArtifact(t, ws, task.ID).Status)
}

func TestClaimShipmentFlatScope_RecoveryRejectsRelatedPreimage(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	ancestor, err := CreateArtifact(ctx, ws, "claim recovery unrelated ancestor", "feature")
	require.NoError(t, err)
	member, err := CreateArtifact(ctx, ws, "claim recovery member", "task", WithParent(ancestor.ID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "claim recovery shipment", []string{member.ID})
	require.NoError(t, err)

	journal := p021LifecycleJournal(t, ws, "claim", "rollback", shipment.ID, "")
	journal.Target = string(ShipmentActive)
	journal.Preimage.Related = []*models.Artifact{cloneArtifact(ancestor)}
	journalPath := p021WriteLifecycleJournal(t, ws, journal)
	before := snapshotURWorkspace(t, ws)

	lockPath, err := artifactMutationLockPath(ws, ancestor.ID)
	require.NoError(t, err)
	unlock, err := lockTaskFileWithHeartbeat(ctx, lockPath, defaultGateLockBoundedWait, defaultGateLockHeartbeat)
	require.NoError(t, err)
	locked := true
	t.Cleanup(func() {
		if locked {
			require.NoError(t, unlock())
		}
	})

	recoveryCtx, cancel := context.WithTimeout(ctx, 150*time.Millisecond)
	defer cancel()
	err = recoverPendingShipmentOperations(recoveryCtx, ws)

	require.ErrorIs(t, err, blerrors.ErrValidation)
	require.ErrorContains(t, err, journalPath)
	require.NoError(t, unlock())
	locked = false
	requireURAggregateUnchanged(t, ws, before)
	require.Equal(t, "intent", p021ReadLifecycleJournal(t, journalPath).Phase)
}

func snapshotClaimFlatArtifact(t *testing.T, ws *Workspace, artifactID string) claimFlatArtifactSnapshot {
	t.Helper()
	path, err := FindArtifactPath(context.Background(), ws, artifactID)
	require.NoError(t, err)
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	databaseArtifact, err := bldb.GetItem(context.Background(), ws.DB, artifactID)
	require.NoError(t, err)
	eventLog, err := os.ReadFile(events.LogPathForItem(WorkspaceLogsRoot(ws.RootPath), artifactID))
	if errors.Is(err, os.ErrNotExist) {
		eventLog = nil
	} else {
		require.NoError(t, err)
	}
	return claimFlatArtifactSnapshot{
		fileContent: content,
		database:    cloneArtifact(databaseArtifact),
		eventLog:    eventLog,
	}
}

func requireClaimFlatArtifactUnchanged(
	t *testing.T,
	ws *Workspace,
	artifactID string,
	before claimFlatArtifactSnapshot,
) {
	t.Helper()
	after := snapshotClaimFlatArtifact(t, ws, artifactID)
	require.Equal(t, before.fileContent, after.fileContent, "unlisted ancestor file bytes changed")
	require.Equal(t, artifactCodecViewUR(t, before.database), artifactCodecViewUR(t, after.database),
		"unlisted ancestor database projection changed")
	require.Equal(t, before.eventLog, after.eventLog, "unlisted ancestor event bytes changed")
}
