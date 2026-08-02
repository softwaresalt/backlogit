package core

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/hooks"
	"github.com/softwaresalt/backlogit/internal/models"
)

// setupShipmentWorkspace creates a temp workspace directory with minimal config for
// shipment testing. Returns the Workspace pointer.
func setupShipmentWorkspace(t *testing.T) *Workspace {
	t.Helper()
	root := t.TempDir()
	ws := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(ws, 0o755))
	require.NoError(t, config.WriteDefaults(ws))

	ctx := context.Background()
	workspace, err := NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { workspace.Close() })
	return workspace
}

// T002 / ST012: Create a shipment and verify it exists with queued status.
func TestCreateShipment_Success(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feat, err := CreateArtifact(ctx, ws, "Shipment feature", "feature")
	require.NoError(t, err)
	taskOne, err := CreateArtifact(ctx, ws, "Shipment task 1", "task", WithParent(feat.ID))
	require.NoError(t, err)
	taskTwo, err := CreateArtifact(ctx, ws, "Shipment task 2", "task", WithParent(feat.ID))
	require.NoError(t, err)

	// Act
	shipment, err := CreateShipment(ctx, ws, "Sprint 1 delivery", []string{taskOne.ID, taskTwo.ID})

	// Assert
	require.NoError(t, err)
	require.NotNil(t, shipment)
	assert.Equal(t, "shipment", shipment.ArtifactType)
	assert.Equal(t, "queued", string(shipment.Status))
	assert.Contains(t, shipment.ID, "S")
	assert.Equal(t, "Sprint 1 delivery", shipment.Title)
}

// T002 / ST012: Reject creating a shipment with an item already assigned to an active shipment.
func TestCreateShipment_RejectsAlreadyAssignedItem(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feat, err := CreateArtifact(ctx, ws, "Assigned feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Assigned task", "task", WithParent(feat.ID))
	require.NoError(t, err)
	_, err = CreateShipment(ctx, ws, "Shipment 1", []string{task.ID})
	require.NoError(t, err)

	// Act
	_, err = CreateShipment(ctx, ws, "Shipment 2", []string{task.ID})

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrItemAlreadyAssigned)
}

// T002 / ST012: Move shipment from queued to active.
func TestMoveShipmentStatus_QueuedToActive(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	shipment, err := CreateShipment(ctx, ws, "Test shipment", nil)
	require.NoError(t, err)

	// Act
	err = MoveShipmentStatus(ctx, ws, shipment.ID, ShipmentActive)

	// Assert
	require.NoError(t, err)
	updated, err := GetShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	assert.Equal(t, "active", string(updated.Status))
}

// T002 / ST012: Claiming a shipment activates included work and rolls feature status up.
func TestClaimShipment_ActivatesIncludedScope(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feature, err := CreateArtifact(ctx, ws, "Claim lifecycle feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))
	task, err := CreateArtifact(ctx, ws, "Claim lifecycle task", "task", WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))
	shipment, err := CreateShipment(ctx, ws, "Claim lifecycle shipment", []string{task.ID})
	require.NoError(t, err)

	// Act
	claimed, err := ClaimShipment(ctx, ws, shipment.ID)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, claimed)
	assert.Equal(t, models.StatusActive, claimed.Status)

	updatedTask, err := loadArtifact(ctx, ws, task.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusActive, updatedTask.Status)

	updatedFeature, err := loadArtifact(ctx, ws, feature.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusActive, updatedFeature.Status)
}

// T002 / ST012: Move shipment from active to shipped.
func TestMoveShipmentStatus_ActiveToShipped(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	shipment, err := CreateShipment(ctx, ws, "Deliver shipment", nil)
	require.NoError(t, err)
	require.NoError(t, MoveShipmentStatus(ctx, ws, shipment.ID, ShipmentActive))

	// Act
	err = MoveShipmentStatus(ctx, ws, shipment.ID, ShipmentShipped)

	// Assert
	require.NoError(t, err)
	updated, err := GetShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	assert.Equal(t, "shipped", string(updated.Status))
}

// T002 / ST012: Shipping a release archives completed scope, returns untouched work
// to backlog, archives linked deliberation, and records the merge commit in logs.
func TestShipShipment_CleansReleasedFeatureScope(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	deliberation, err := CreateArtifact(ctx, ws, "Release cleanup deliberation", "deliberation")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, deliberation))

	feature, err := CreateArtifact(
		ctx,
		ws,
		"Release cleanup feature",
		"feature",
		WithDescription("Origin: "+deliberation.ID),
	)
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

	releasedTask, err := CreateArtifact(ctx, ws, "Released task", "task", WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, releasedTask))

	futureTask, err := CreateArtifact(ctx, ws, "Future task", "task", WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, futureTask))

	// Partial-feature manifest: only releasedTask is an explicit shipment
	// member. The covering feature and its linked deliberation are NOT listed,
	// so per the membership contract (133-F) they must stay open — this is the
	// 114-S partial-feature regression scenario.
	shipment, err := CreateShipment(ctx, ws, "Release cleanup shipment", []string{releasedTask.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	commit := &CommitMetadata{
		SHA:     "deadbeefcafebabe",
		Message: "merge: release cleanup feature",
		Author:  "tester@example.com",
	}

	// Act
	result, err := ShipShipment(ctx, ws, shipment.ID, commit)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, shipment.ID, result.ShipmentID)
	assert.Equal(t, string(ShipmentShipped), result.ShipmentStatus)
	assert.Contains(t, result.ArchivedIDs, shipment.ID)
	assert.Contains(t, result.ArchivedIDs, releasedTask.ID)
	assert.NotContains(t, result.ArchivedIDs, feature.ID,
		"non-member covering feature must not be archived on a partial-feature ship")
	assert.NotContains(t, result.ArchivedIDs, deliberation.ID,
		"linked deliberation of a non-member covering feature must not be archived")
	assert.Contains(t, result.ReturnedIDs, futureTask.ID)

	// The covering feature must remain open: not done, not archived, and its
	// file must still physically reside under .backlogit/queue/ — this is the
	// transitive parent-status rollup seam (completeReleaseScope marks
	// releasedTask done -> cascadePersistedParentStatuses sees all RECORDED
	// children done -> rolls the feature to done and relocates it). Checking
	// only ArchivedIDs absence would false-green here because the rollup
	// relocates the file without ever routing it through the archival
	// collector.
	openFeature, err := findArtifact(ctx, ws, feature.ID)
	require.NoError(t, err)
	assert.NotEqual(t, models.StatusDone, openFeature.Status, "non-member covering feature must not be marked done")
	assert.NotEqual(t, models.StatusArchived, openFeature.Status, "non-member covering feature must not be archived")
	featureQueuePath, pathErr := FindArtifactPath(ctx, ws, feature.ID)
	require.NoError(t, pathErr, "covering feature file must still be discoverable")
	assert.Equal(t, "queue", filepath.Base(filepath.Dir(featureQueuePath)),
		"covering feature file must remain under .backlogit/queue/, got %s", featureQueuePath)

	archivedReleasedTask, err := loadArtifact(ctx, ws, releasedTask.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusArchived, archivedReleasedTask.Status)

	queuedFutureTask, err := loadArtifact(ctx, ws, futureTask.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusQueued, queuedFutureTask.Status)
	assert.Empty(t, queuedFutureTask.ParentID, "returned item should have parent_id cleared")

	openDeliberation, err := findArtifact(ctx, ws, deliberation.ID)
	require.NoError(t, err)
	assert.NotEqual(t, models.StatusArchived, openDeliberation.Status,
		"linked deliberation of a non-member covering feature must not be archived")

	for _, itemID := range []string{shipment.ID, releasedTask.ID} {
		entries, logErr := bldb.ListItemLogEntries(ctx, ws.DB, itemID, 0)
		require.NoError(t, logErr)
		found := false
		for _, entry := range entries {
			if entry.EventType == "commit_tracked" {
				found = true
				assert.Equal(t, commit.SHA, entry.Delta["commit_sha"])
			}
		}
		assert.True(t, found, "expected commit_tracked entry for %s", itemID)
	}
}

// 133.003-T (Unit 1b): Feature-inclusive full-feature characterization — locks
// the baseline full-feature close path. When the covering feature IS an
// explicit shipment member, it legitimately closes and archives along with
// all of its terminal children. This must pass both before and after the
// Unit 2 membership-gating fix (baseline preservation guard).
func TestShipShipment_FeatureInclusiveManifestArchivesFeature(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feature, err := CreateArtifact(ctx, ws, "Full feature release", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

	taskOne, err := CreateArtifact(ctx, ws, "Full feature task one", "task", WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, taskOne))

	taskTwo, err := CreateArtifact(ctx, ws, "Full feature task two", "task", WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, taskTwo))

	// Feature-inclusive manifest: the feature and ALL of its children are
	// explicit members, so it is a genuine full-feature ship.
	shipment, err := CreateShipment(ctx, ws, "Full feature shipment", []string{feature.ID, taskOne.ID, taskTwo.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	// Act
	result, err := ShipShipment(ctx, ws, shipment.ID, nil)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.ArchivedIDs, feature.ID, "explicitly listed covering feature must be archived")
	assert.Contains(t, result.ArchivedIDs, taskOne.ID)
	assert.Contains(t, result.ArchivedIDs, taskTwo.ID)

	archivedFeature, err := loadArtifact(ctx, ws, feature.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusArchived, archivedFeature.Status)
}

// 133.003-T (Unit 1b): Body-planned-only regression — the covering feature's
// ONLY recorded children are the shipped tasks (no other siblings exist at
// all, harvested or otherwise). This is the exact 114-S hard case: the
// transitive parent-status rollup (completeReleaseScope -> setArtifactStatus
// -> cascadePersistedParentStatuses -> ComputeParentStatus) sees every
// RECORDED child done and rolls the non-member covering feature to done,
// relocating it out of .backlogit/queue/ BEFORE the direct status seam or the
// archival collector ever run. A membership gate on only those two seams is
// insufficient to catch this case; the rollup itself must be neutralized.
func TestShipShipment_BodyPlannedOnlyChildrenShipKeepsFeatureOpen(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feature, err := CreateArtifact(ctx, ws, "Body-planned covering feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

	// The feature's ONLY recorded children are the two tasks below; there is
	// no unharvested/nonterminal sibling to keep it open by descendant count.
	taskOne, err := CreateArtifact(ctx, ws, "Body-planned task one", "task", WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, taskOne))

	taskTwo, err := CreateArtifact(ctx, ws, "Body-planned task two", "task", WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, taskTwo))

	// Children-only manifest: the feature is NOT an explicit member, even
	// though shipping both tasks makes every RECORDED child of the feature
	// terminal (the exact condition that fires ComputeParentStatus's rollup).
	shipment, err := CreateShipment(ctx, ws, "Body-planned shipment", []string{taskOne.ID, taskTwo.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	// Act
	result, err := ShipShipment(ctx, ws, shipment.ID, nil)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.ArchivedIDs, taskOne.ID)
	assert.Contains(t, result.ArchivedIDs, taskTwo.ID)
	assert.NotContains(t, result.ArchivedIDs, feature.ID,
		"non-member covering feature must not be archived even when all its recorded children ship")

	// Must check BOTH status and physical file location: the rollup path
	// closes and relocates the feature WITHOUT ever routing it through the
	// archival collector, so ArchivedIDs absence alone false-greens.
	openFeature, err := findArtifact(ctx, ws, feature.ID)
	require.NoError(t, err)
	assert.NotEqual(t, models.StatusDone, openFeature.Status,
		"body-planned covering feature must not be rolled up to done")
	assert.NotEqual(t, models.StatusArchived, openFeature.Status,
		"body-planned covering feature must not be archived")
	featureQueuePath, pathErr := FindArtifactPath(ctx, ws, feature.ID)
	require.NoError(t, pathErr, "covering feature file must still be discoverable")
	assert.Equal(t, "queue", filepath.Base(filepath.Dir(featureQueuePath)),
		"covering feature file must remain under .backlogit/queue/, got %s", featureQueuePath)
}

// 133.004-T (Unit 2 failure-injection): the deferred restore in ShipShipment
// must fire even when a later step fails and ShipShipment returns an error.
// moveShipmentStatusWithTopLevel's persistArtifact call (marking the
// shipment itself "shipped") runs strictly after completeReleaseScope's
// rollup already relocated the non-member covering feature, but strictly
// before collectArchiveCandidateIDs/archiveItems ever run. Failing exactly
// that write exercises the defer's error-join path without ever reaching the
// archival collector, proving the restore is not merely a side effect of the
// happy path.
func TestShipShipment_RestoresNonMemberFeatureEvenWhenShipFailsAfterRollup(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feature, err := CreateArtifact(ctx, ws, "Body-planned covering feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

	// The feature's ONLY recorded children are the two tasks below, so
	// shipping both fires the exact ComputeParentStatus rollup condition.
	taskOne, err := CreateArtifact(ctx, ws, "Body-planned task one", "task", WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, taskOne))

	taskTwo, err := CreateArtifact(ctx, ws, "Body-planned task two", "task", WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, taskTwo))

	shipment, err := CreateShipment(ctx, ws, "Body-planned shipment", []string{taskOne.ID, taskTwo.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	// Fail only the write that persists the shipment artifact itself; let
	// every other artifact write (tasks, the covering feature, and its
	// restore) proceed through the real implementation.
	injectedErr := errors.New("injected shipment write failure")
	origFn := persistArtifactWriteFn
	persistArtifactWriteFn = func(a *models.Artifact, filePath string, durable bool) error {
		if a.ID == shipment.ID {
			return injectedErr
		}
		return origFn(a, filePath, durable)
	}
	t.Cleanup(func() { persistArtifactWriteFn = origFn })

	// Act
	result, err := ShipShipment(ctx, ws, shipment.ID, nil)

	// Assert
	require.Error(t, err, "ship shipment must surface the injected write failure")
	assert.ErrorIs(t, err, injectedErr)
	assert.Nil(t, result)

	// Even though ShipShipment failed partway through -- after the rollup
	// already marked the covering feature done and relocated it, but before
	// the archival collector ever ran -- the deferred restore must still have
	// fired and reverted the feature to its pre-ship status and location.
	restoredFeature, findErr := findArtifact(ctx, ws, feature.ID)
	require.NoError(t, findErr)
	assert.NotEqual(t, models.StatusDone, restoredFeature.Status,
		"non-member covering feature must be restored even when the ship fails after the rollup")
	assert.NotEqual(t, models.StatusArchived, restoredFeature.Status,
		"non-member covering feature must not be archived when the ship fails after the rollup")
	featureQueuePath, pathErr := FindArtifactPath(ctx, ws, feature.ID)
	require.NoError(t, pathErr, "covering feature file must still be discoverable after restore")
	assert.Equal(t, "queue", filepath.Base(filepath.Dir(featureQueuePath)),
		"covering feature file must be restored under .backlogit/queue/, got %s", featureQueuePath)
}

// 133.004-T (review-fix): a nested feature reached only through
// AdoptItem-based re-parenting (dotted ID, e.g. "002.001-F") must not be
// corrupted by the non-member restore mechanism when it is a genuine,
// fully-released descendant of an EXPLICIT-member covering feature. F is
// never itself listed in the manifest, so featureScopeRoots still discovers
// it (walking up from the explicitly listed grandchild task) and
// snapshotNonMemberFeatureStatuses records it as "non-member". But
// collectArchiveCandidateIDs's first loop sweeps F into the archive scope
// anyway because F is a genuine descendant of the explicit-member root G,
// and completeReleaseScope already marked it terminal. F is therefore
// legitimately archived via the real ArchiveItem pipeline (archived_from/
// archived_status stamped, moved under .backlogit/archive/) -- and the
// deferred restore must recognize that and leave it alone, never reverting
// an item that the same call already, correctly archived.
func TestShipShipment_LegitimatelyArchivesNestedFeatureDescendantOfMember(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	rootFeature, err := CreateArtifact(ctx, ws, "Root covering feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, rootFeature))

	rootTask, err := CreateArtifact(ctx, ws, "Root feature direct task", "task", WithParent(rootFeature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, rootTask))

	// Nested feature: created top-level (features can only be created as
	// roots per config's AllowedChildren), then re-parented under the root
	// feature via AdoptItem, which -- unlike CreateArtifact -- does not
	// enforce AllowedChildren. This mirrors how the repository's own archive
	// contains dotted-ID features (e.g. 013.001-F) that predate this fix.
	nestedFeatureSeed, err := CreateArtifact(ctx, ws, "Nested feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, nestedFeatureSeed))

	adoptResult, err := AdoptItem(ctx, ws, nestedFeatureSeed.ID, rootFeature.ID)
	require.NoError(t, err)
	nestedFeatureID := adoptResult.NewID
	if nestedFeatureID == "" {
		nestedFeatureID = nestedFeatureSeed.ID
	}

	nestedTask, err := CreateArtifact(ctx, ws, "Nested feature task", "task", WithParent(nestedFeatureID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, nestedTask))

	// Manifest lists the root feature, its direct task, and the NESTED
	// task -- but never the nested feature itself. featureScopeRoots still
	// discovers the nested feature by walking up from nestedTask's ancestry.
	shipment, err := CreateShipment(ctx, ws, "Full-tree shipment", []string{rootFeature.ID, rootTask.ID, nestedTask.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	// Act
	result, err := ShipShipment(ctx, ws, shipment.ID, nil)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Contains(t, result.ArchivedIDs, rootFeature.ID)
	assert.Contains(t, result.ArchivedIDs, rootTask.ID)
	assert.Contains(t, result.ArchivedIDs, nestedTask.ID)
	assert.Contains(t, result.ArchivedIDs, nestedFeatureID,
		"nested feature that is a genuine descendant of the explicit-member root must be archived, not skipped")

	// The critical regression check: the nested feature must remain archived
	// after ShipShipment's deferred restore runs. Restoring it here would mean
	// the restore mechanism reverted a legitimate, already-completed archival
	// of the SAME call -- corrupting archived_from/archived_status and
	// leaving the item in a confused, partially-reverted state.
	finalNested, err := loadArtifact(ctx, ws, nestedFeatureID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusArchived, finalNested.Status,
		"nested feature legitimately archived as part of the fully-released root must not be reverted by the non-member restore")

	nestedPath, pathErr := FindArtifactPath(ctx, ws, nestedFeatureID)
	require.NoError(t, pathErr, "nested feature file must still be discoverable")
	assert.Equal(t, "archive", filepath.Base(filepath.Dir(nestedPath)),
		"nested feature file must remain under .backlogit/archive/, got %s", nestedPath)
}

// TestArchiveItems_PreservesPriorSuccessesWhenLaterItemFails is a regression
// guard for a review-fix (PR #327 Copilot finding): archiveItems previously
// returned (nil, err) whenever a LATER item in its depth-sorted loop failed,
// discarding the IDs it had already archived earlier in the very same call.
// ShipShipment assigns that return value directly to archivedIDs before the
// error causes an early return, so its deferred restoreRolledUpNonMemberFeatures
// would run with an empty exclusion set and could revert a nested feature this
// exact call had just legitimately archived -- the precise corruption the
// archivedIDs exclusion (133.004-T review-fix) exists to prevent, just reached
// via a partial-failure path instead of the happy path. This test exercises
// archiveItems directly (rather than through the full ShipShipment cascade,
// where every candidate is already relocated into .backlogit/archive/ by the
// unguarded done-status persistArtifact relocate -- see
// config.defaults.go's status->"archive" directory rule -- before archiveItems
// ever runs, making ArchiveItem's distinct-occupant guard unreachable from
// that path). archiveItems must preserve every ID it archived before the
// failure.
func TestArchiveItems_PreservesPriorSuccessesWhenLaterItemFails(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	parent, err := CreateArtifact(ctx, ws, "Parent (fails to archive)", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, parent))

	child, err := CreateArtifact(ctx, ws, "Child (archived first)", "task", WithParent(parent.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, child))

	// Poison parent's archive destination with a distinct foreign occupant
	// (same id, different title) so ArchiveItem refuses it with
	// ErrArchiveDestinationOccupied. Neither item's status has changed from
	// its post-creation default, so both are still genuinely in queue/ --
	// unlike a ShipShipment-driven call, this reproduces a real distinct-path
	// collision rather than an already-relocated in-place recovery. child's
	// ID has one more dot than parent's (nested one level under it), so
	// depthSortedIDs always processes child BEFORE parent.
	archiveDir := filepath.Join(WorkspaceStorageRoot(ws.RootPath), "archive")
	require.NoError(t, os.MkdirAll(archiveDir, 0o755))
	foreignPath := filepath.Join(archiveDir, parent.ID+".md")
	foreign := "---\n" +
		"id: \"" + parent.ID + "\"\n" +
		"title: \"Pre-existing DIFFERENT archived item\"\n" +
		"status: archived\n" +
		"artifact_type: feature\n" +
		"level: 1\n" +
		"---\nForeign archived body.\n"
	require.NoError(t, os.WriteFile(foreignPath, []byte(foreign), 0o644))

	// Act
	archived, err := archiveItems(ctx, ws, []string{parent.ID, child.ID})

	// Assert
	require.Error(t, err, "archiving parent onto an occupied destination must fail")
	assert.True(t, errors.Is(err, blerrors.ErrArchiveDestinationOccupied),
		"failure must be the propagated archive-destination-occupied error; got: %v", err)
	assert.Contains(t, archived, child.ID,
		"archiveItems must preserve IDs it already archived before a later item's failure, not discard them")

	// child must genuinely be archived on disk despite the overall call
	// having failed on parent.
	finalChild, loadErr := loadArtifact(ctx, ws, child.ID)
	require.NoError(t, loadErr)
	assert.Equal(t, models.StatusArchived, finalChild.Status,
		"child archived before the later failure must remain archived")

	// The foreign occupant at parent's archive destination must be left
	// untouched -- confirming the failure was a clean refusal, not a partial
	// overwrite.
	foreignContent, readErr := os.ReadFile(foreignPath)
	require.NoError(t, readErr)
	assert.Contains(t, string(foreignContent), "Pre-existing DIFFERENT archived item")
}

// TestRestoreRolledUpNonMemberFeatures_SucceedsWithCanceledContext is a
// regression guard for a review-fix (PR #327 Copilot finding): ShipShipment's
// deferred cleanup is documented to "always attempt the revert, even if a
// later step in this function fails and returns early" (see the ShipShipment
// defer at the archivedIDs declaration), but restoreRolledUpNonMemberFeatures
// previously reused the caller's context unchanged. When ShipShipment's own
// context is canceled or its deadline expires -- exactly the condition under
// which the forward operation is most likely to have failed and triggered
// this cleanup -- the reused context makes the internal loadArtifact/
// setArtifactStatus calls fail immediately with context.Canceled /
// DeadlineExceeded, leaving the covering feature stranded at its unintended
// rolled-up status and archive-directory location despite the "always
// attempt" guarantee. This mirrors the established rollbackQueueMove
// precedent (internal/core/queue.go:365-372), which detaches from the
// caller's context via context.WithoutCancel for the exact same reason.
// restoreRolledUpNonMemberFeatures must do the same.
func TestRestoreRolledUpNonMemberFeatures_SucceedsWithCanceledContext(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	bg := context.Background()

	feature, err := CreateArtifact(bg, ws, "Rolled-up covering feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(bg, ws.DB, feature))

	// Simulate the unintended rollup completeReleaseScope's cascade performs:
	// the feature's status moves to done, which (per the registry's
	// status-based directory routing) also physically relocates its file
	// into .backlogit/archive/.
	_, err = setArtifactStatus(bg, ws, feature.ID, models.StatusDone, "simulated unintended rollup")
	require.NoError(t, err)

	snapshots := map[string]featureStatusSnapshot{
		feature.ID: {status: models.StatusQueued},
	}

	canceledCtx, cancel := context.WithCancel(bg)
	cancel()
	require.Error(t, canceledCtx.Err(), "test precondition: context must already be canceled")

	// Act: archivedIDs is nil, so the feature is not in the exclusion set --
	// it must be restored, and it must be restored EVEN THOUGH canceledCtx is
	// already done, exactly as ShipShipment's deferred cleanup requires.
	restoreErr := restoreRolledUpNonMemberFeatures(canceledCtx, ws, snapshots, nil)

	// Assert
	require.NoError(t, restoreErr,
		"restore must succeed even when the caller's context is already canceled")

	restored, findErr := findArtifact(bg, ws, feature.ID)
	require.NoError(t, findErr)
	assert.Equal(t, models.StatusQueued, restored.Status,
		"feature must be reverted to its pre-rollup status despite the canceled context")
	restoredPath, pathErr := FindArtifactPath(bg, ws, feature.ID)
	require.NoError(t, pathErr)
	assert.Equal(t, "queue", filepath.Base(filepath.Dir(restoredPath)),
		"feature file must be relocated back under .backlogit/queue/ despite the canceled context")
}

// TestRestoreRolledUpNonMemberFeatures_RestoresDeepestFirstRegardlessOfMapOrder
// is a regression guard for a review-fix (PR #327 Copilot finding):
// restoreRolledUpNonMemberFeatures iterated `snapshots` with a plain
// `for featureID, snapshot := range snapshots` loop. Go map iteration order
// is intentionally randomized, but setArtifactStatus unconditionally cascades
// every status change UP to the parent via cascadePersistedParentStatuses. If
// a parent feature's snapshot is restored before its child's, the child's
// later restore re-triggers the parent cascade (via ComputeParentStatus,
// which mirrors a single child's status onto its parent) and silently
// recomputes -- overwriting -- the parent's just-restored value, corrupting
// the parent's final status. Restoring children before their parents
// (deepest-first, exactly what depthSortedIDs already provides for the
// analogous ordering need in completeReleaseScope) avoids this because each
// parent's own restore is always the LAST write to touch it.
//
// A single parent/child pair would only reproduce this on roughly half of
// all runs -- Go's map iteration order for two keys is effectively a coin
// flip that does not repeat reliably across process runs -- so this test
// uses eight independent parent/child pairs in ONE combined snapshots map.
// Fixing the bug is the only way every pair lands correctly on every run;
// the unfixed code has only a ~1-in-256 chance of coincidentally iterating
// all eight pairs in the safe child-before-parent order.
func TestRestoreRolledUpNonMemberFeatures_RestoresDeepestFirstRegardlessOfMapOrder(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	bg := context.Background()

	const pairCount = 8
	type pair struct {
		parentID, childID string
	}
	pairs := make([]pair, 0, pairCount)
	snapshots := make(map[string]featureStatusSnapshot, pairCount*2)

	for i := 0; i < pairCount; i++ {
		parent, err := CreateArtifact(bg, ws, fmt.Sprintf("Parent %d", i), "feature")
		require.NoError(t, err)
		require.NoError(t, bldb.UpsertItem(bg, ws.DB, parent))

		childSeed, err := CreateArtifact(bg, ws, fmt.Sprintf("Child %d", i), "feature")
		require.NoError(t, err)
		require.NoError(t, bldb.UpsertItem(bg, ws.DB, childSeed))

		// Nest the child under the parent the same way the codebase's other
		// nested-feature tests do: create top-level, then re-parent via
		// AdoptItem (CreateArtifact enforces AllowedChildren and would
		// reject a feature-under-feature at creation time).
		adoptResult, err := AdoptItem(bg, ws, childSeed.ID, parent.ID)
		require.NoError(t, err)
		childID := adoptResult.NewID
		if childID == "" {
			childID = childSeed.ID
		}

		// Simulate the unintended rollup completeReleaseScope's cascade
		// performs: the child rolls up to done first, and because it is the
		// parent's ONLY recorded child, setArtifactStatus's own cascade also
		// forces the parent to done as a side effect.
		_, err = setArtifactStatus(bg, ws, childID, models.StatusDone, "simulated unintended rollup")
		require.NoError(t, err)

		parentReloaded, err := loadArtifact(bg, ws, parent.ID)
		require.NoError(t, err)
		require.Equal(t, models.StatusDone, parentReloaded.Status,
			"test precondition: pair %d parent must also roll up to done via the child's cascade", i)

		snapshots[parent.ID] = featureStatusSnapshot{status: models.StatusQueued}
		snapshots[childID] = featureStatusSnapshot{status: models.StatusActive}
		pairs = append(pairs, pair{parentID: parent.ID, childID: childID})
	}

	// Act
	restoreErr := restoreRolledUpNonMemberFeatures(bg, ws, snapshots, nil)

	// Assert
	require.NoError(t, restoreErr)
	for i, p := range pairs {
		restoredParent, err := loadArtifact(bg, ws, p.parentID)
		require.NoError(t, err)
		assert.Equal(t, models.StatusQueued, restoredParent.Status,
			"pair %d: parent must end up at its own pre-ship snapshot, not silently overwritten by a later child-triggered cascade", i)

		restoredChild, err := loadArtifact(bg, ws, p.childID)
		require.NoError(t, err)
		assert.Equal(t, models.StatusActive, restoredChild.Status,
			"pair %d: child must be restored to its own pre-ship snapshot", i)
	}
}

// 133.004-T (review-fix, PR #327 ordering finding): the non-member covering
// feature restore must complete on the successful path BEFORE
// VerifyPostShipConsistency and the post-ship hooks run, not merely by the
// time the function returns. Before this fix, restoreRolledUpNonMemberFeatures
// ran only inside ShipShipment's deferred cleanup, which -- like every Go
// defer -- executes during return unwinding, i.e. strictly AFTER
// VerifyPostShipConsistency and ws.HookRunner.FirePost have already run to
// completion as ordinary in-line statements. A post-ship hook (an external
// webhook or custom integration) could therefore observe -- and act on -- the
// covering feature in its transient, incorrectly-rolled-up done/archived
// state, even though the ship ultimately succeeds and the feature is
// reverted moments later. This test registers a synchronous post-ship hook
// probe and asserts it observes the ALREADY-RESTORED feature.
func TestShipShipment_RestoresNonMemberFeatureBeforePostShipHooksObserveIt(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	ws.HookRunner = hooks.NewHookRunner()

	feature, err := CreateArtifact(ctx, ws, "Body-planned covering feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

	// The feature's ONLY recorded children are the two tasks below, so
	// shipping both fires the exact ComputeParentStatus rollup condition.
	taskOne, err := CreateArtifact(ctx, ws, "Body-planned task one", "task", WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, taskOne))

	taskTwo, err := CreateArtifact(ctx, ws, "Body-planned task two", "task", WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, taskTwo))

	shipment, err := CreateShipment(ctx, ws, "Body-planned shipment", []string{taskOne.ID, taskTwo.ID})
	require.NoError(t, err)
	_, err = ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	var (
		hookFired      bool
		observedStatus models.ArtifactStatus
		observeErr     error
	)
	ws.HookRunner.Register(hooks.HookShipShipment, hooks.PhasePost, hooks.HookRegistration{
		Name:     "probe_covering_feature_state",
		Priority: 100,
		Fn: func(hookCtx context.Context, hc hooks.HookContext) error {
			hookFired = true
			snapshot, findErr := findArtifact(hookCtx, ws, feature.ID)
			if findErr != nil {
				observeErr = findErr
				return nil
			}
			observedStatus = snapshot.Status
			return nil
		},
	})

	// Act
	result, err := ShipShipment(ctx, ws, shipment.ID, nil)

	// Assert
	require.NoError(t, err)
	require.NotNil(t, result)
	require.True(t, hookFired, "test precondition: the post-ship hook probe must have fired")
	require.NoError(t, observeErr)
	assert.NotEqual(t, models.StatusDone, observedStatus,
		"post-ship hook must observe the covering feature already restored, not the transient rolled-up done status")
	assert.NotEqual(t, models.StatusArchived, observedStatus,
		"post-ship hook must observe the covering feature already restored, not archived")
}

// T002 / ST012: Reject invalid status transition (queued -> shipped).
func TestMoveShipmentStatus_InvalidTransition(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	shipment, err := CreateShipment(ctx, ws, "Bad transition", nil)
	require.NoError(t, err)

	// Act
	err = MoveShipmentStatus(ctx, ws, shipment.ID, ShipmentShipped)

	// Assert
	require.Error(t, err, "queued -> shipped should be rejected")
}

// T002 / ST013: Add an item to a shipment.
func TestAddItemToShipment_Success(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	shipment, err := CreateShipment(ctx, ws, "With items", nil)
	require.NoError(t, err)

	// Create a task artifact for the item
	feat, err := CreateArtifact(ctx, ws, "Shipment feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Test task", "task", WithParent(feat.ID))
	require.NoError(t, err)

	// Act
	err = AddItemToShipment(ctx, ws, shipment.ID, task.ID)

	// Assert
	require.NoError(t, err)
	updated, err := GetShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	items, ok := updated.CustomFields["items"].([]string)
	require.True(t, ok, "shipment must have items in custom fields")
	assert.Contains(t, items, task.ID)
}

// T002 / ST013: Reject adding an item already in another shipment.
func TestAddItemToShipment_AlreadyAssigned(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	s1, err := CreateShipment(ctx, ws, "Shipment 1", nil)
	require.NoError(t, err)
	s2, err := CreateShipment(ctx, ws, "Shipment 2", nil)
	require.NoError(t, err)

	feat, err := CreateArtifact(ctx, ws, "Contested feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Contested task", "task", WithParent(feat.ID))
	require.NoError(t, err)

	require.NoError(t, AddItemToShipment(ctx, ws, s1.ID, task.ID))

	// Act
	err = AddItemToShipment(ctx, ws, s2.ID, task.ID)

	// Assert
	require.Error(t, err, "item already in s1 must not be added to s2")
	_ = s2 // prevent unused
}

// T002 / ST013: Reject adding a missing item to a shipment.
func TestAddItemToShipment_MissingItem(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	shipment, err := CreateShipment(ctx, ws, "With items", nil)
	require.NoError(t, err)

	// Act
	err = AddItemToShipment(ctx, ws, shipment.ID, "T999")

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrNotFound)
}

// T002 / ST013: Allow reassigning an item after its previous shipment is shipped.
func TestAddItemToShipment_AllowsItemAfterShippedShipment(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feat, err := CreateArtifact(ctx, ws, "Reusable feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Reusable task", "task", WithParent(feat.ID))
	require.NoError(t, err)
	firstShipment, err := CreateShipment(ctx, ws, "Shipment 1", nil)
	require.NoError(t, err)
	secondShipment, err := CreateShipment(ctx, ws, "Shipment 2", nil)
	require.NoError(t, err)
	require.NoError(t, AddItemToShipment(ctx, ws, firstShipment.ID, task.ID))
	require.NoError(t, MoveShipmentStatus(ctx, ws, firstShipment.ID, ShipmentActive))
	require.NoError(t, MoveShipmentStatus(ctx, ws, firstShipment.ID, ShipmentShipped))

	// Act
	err = AddItemToShipment(ctx, ws, secondShipment.ID, task.ID)

	// Assert
	require.NoError(t, err)
	updated, err := GetShipment(ctx, ws, secondShipment.ID)
	require.NoError(t, err)
	assert.Contains(t, NormalizeShipmentItems(updated), task.ID)
}

// T002 / ST013: Reject adding an item to a shipped shipment.
func TestAddItemToShipment_RejectsTerminalShipment(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feat, err := CreateArtifact(ctx, ws, "Terminal feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Terminal shipment task", "task", WithParent(feat.ID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "Terminal shipment", nil)
	require.NoError(t, err)
	require.NoError(t, MoveShipmentStatus(ctx, ws, shipment.ID, ShipmentActive))
	require.NoError(t, MoveShipmentStatus(ctx, ws, shipment.ID, ShipmentShipped))

	// Act
	err = AddItemToShipment(ctx, ws, shipment.ID, task.ID)

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrShipmentConflict)
}

// T002 / ST013: Allow reassigning an item after its previous shipment is archived.
func TestAddItemToShipment_AllowsItemAfterArchivedShipment(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feat, err := CreateArtifact(ctx, ws, "Archived reusable feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Archived reusable task", "task", WithParent(feat.ID))
	require.NoError(t, err)
	firstShipment, err := CreateShipment(ctx, ws, "Archived shipment 1", nil)
	require.NoError(t, err)
	secondShipment, err := CreateShipment(ctx, ws, "Archived shipment 2", nil)
	require.NoError(t, err)
	require.NoError(t, AddItemToShipment(ctx, ws, firstShipment.ID, task.ID))
	_, err = ArchiveItem(ctx, ws.DB, ws, firstShipment.ID)
	require.NoError(t, err)

	// Act
	err = AddItemToShipment(ctx, ws, secondShipment.ID, task.ID)

	// Assert
	require.NoError(t, err)
	updated, err := GetShipment(ctx, ws, secondShipment.ID)
	require.NoError(t, err)
	assert.Contains(t, NormalizeShipmentItems(updated), task.ID)
}

// T002 / ST013: Return a blocked item from shipment.
func TestReturnBlockedItem_Success(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	shipment, err := CreateShipment(ctx, ws, "Blocked test", nil)
	require.NoError(t, err)

	feat, err := CreateArtifact(ctx, ws, "Blockable feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Blockable task", "task", WithParent(feat.ID))
	require.NoError(t, err)
	require.NoError(t, AddItemToShipment(ctx, ws, shipment.ID, task.ID))

	// Act
	err = ReturnBlockedItem(ctx, ws, shipment.ID, task.ID, "dependency not ready")

	// Assert
	require.NoError(t, err)

	// Verify item is no longer in shipment
	updated, err := GetShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	if items, ok := updated.CustomFields["items"].([]string); ok {
		assert.NotContains(t, items, task.ID)
	}
}

// T002 / ST013: Reject returning an item not in the shipment.
func TestReturnBlockedItem_NotInShipment(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	shipment, err := CreateShipment(ctx, ws, "Not in shipment", nil)
	require.NoError(t, err)

	// Act
	err = ReturnBlockedItem(ctx, ws, shipment.ID, "T999", "fake reason")

	// Assert
	require.Error(t, err, "returning an item not in the shipment must fail")
}

// T002 / ST013: Reject returning a blocked item from an archived shipment.
func TestReturnBlockedItem_RejectsTerminalShipment(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	shipment, err := CreateShipment(ctx, ws, "Archived return shipment", nil)
	require.NoError(t, err)
	feat, err := CreateArtifact(ctx, ws, "Archived return feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Archived return task", "task", WithParent(feat.ID))
	require.NoError(t, err)
	require.NoError(t, AddItemToShipment(ctx, ws, shipment.ID, task.ID))
	_, err = ArchiveItem(ctx, ws.DB, ws, shipment.ID)
	require.NoError(t, err)

	// Act
	err = ReturnBlockedItem(ctx, ws, shipment.ID, task.ID, "archived shipment")

	// Assert
	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrShipmentConflict)
}

// T002 / ST013: Roll back shipment changes when the item update fails.
func TestPersistReturnedBlockedArtifacts_RollsBackOnItemFailure(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	shipment, err := CreateShipment(ctx, ws, "Rollback shipment", nil)
	require.NoError(t, err)
	feat, err := CreateArtifact(ctx, ws, "Rollback feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Rollback task", "task", WithParent(feat.ID))
	require.NoError(t, err)
	require.NoError(t, AddItemToShipment(ctx, ws, shipment.ID, task.ID))

	currentShipment, err := GetShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	currentItem, err := loadArtifact(ctx, ws, task.ID)
	require.NoError(t, err)

	originalShipment := cloneArtifact(currentShipment)
	originalItem := cloneArtifact(currentItem)

	currentShipment.CustomFields["items"] = removeString(NormalizeShipmentItems(currentShipment), task.ID)
	currentShipment.UpdatedAt = time.Now()
	currentItem.Status = models.StatusBlocked
	currentItem.Title = ""
	currentItem.UpdatedAt = time.Now()

	// Act
	rolledBack, err := persistReturnedBlockedArtifacts(ctx, ws, originalShipment, currentShipment, originalItem, currentItem)

	// Assert
	require.Error(t, err)
	assert.True(t, rolledBack)
	restoredShipment, loadShipmentErr := GetShipment(ctx, ws, shipment.ID)
	require.NoError(t, loadShipmentErr)
	assert.Contains(t, NormalizeShipmentItems(restoredShipment), task.ID)
	restoredItem, loadItemErr := loadArtifact(ctx, ws, task.ID)
	require.NoError(t, loadItemErr)
	assert.Equal(t, models.StatusQueued, restoredItem.Status)
}

// T002 / ST013: Restore the Markdown file when DB upsert fails after file write.
func TestPersistArtifact_RestoresFileOnUpsertFailure(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	shipment, err := CreateShipment(ctx, ws, "File restore shipment", nil)
	require.NoError(t, err)
	currentPath, err := FindArtifactPath(ctx, ws, shipment.ID)
	require.NoError(t, err)
	originalContent, err := os.ReadFile(currentPath)
	require.NoError(t, err)
	shipment.Title = "Broken after file write"
	shipment.UpdatedAt = time.Now()
	require.NoError(t, ws.DB.Close())

	// Act
	err = persistArtifact(ctx, ws, shipment, false)

	// Assert
	require.Error(t, err)
	restoredContent, readErr := os.ReadFile(currentPath)
	require.NoError(t, readErr)
	assert.Equal(t, string(originalContent), string(restoredContent))
}

// T002 / ST013: Recover a journaled blocked-item return on workspace reopen.
func TestNewWorkspace_RecoversPendingReturnBlockedJournal(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	shipment, err := CreateShipment(ctx, ws, "Recovered shipment", nil)
	require.NoError(t, err)
	feat, err := CreateArtifact(ctx, ws, "Recovered feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Recovered task", "task", WithParent(feat.ID))
	require.NoError(t, err)
	require.NoError(t, AddItemToShipment(ctx, ws, shipment.ID, task.ID))
	originalShipment, err := GetShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	originalItem, err := loadArtifact(ctx, ws, task.ID)
	require.NoError(t, err)
	updatedShipment := cloneArtifact(originalShipment)
	updatedShipment.CustomFields["items"] = removeString(NormalizeShipmentItems(updatedShipment), task.ID)
	updatedShipment.UpdatedAt = time.Now()
	require.NoError(t, writeReturnBlockedJournal(ws.RootPath, originalShipment, originalItem))
	require.NoError(t, persistArtifact(ctx, ws, updatedShipment, false))
	rootPath := ws.RootPath
	require.NoError(t, ws.Close())

	// Act
	reopened, err := NewWorkspace(ctx, rootPath)
	require.NoError(t, err)
	defer reopened.Close()

	// Assert
	recoveredShipment, err := GetShipment(ctx, reopened, shipment.ID)
	require.NoError(t, err)
	assert.Contains(t, NormalizeShipmentItems(recoveredShipment), task.ID)
	recoveredItem, err := loadArtifact(ctx, reopened, task.ID)
	require.NoError(t, err)
	assert.Equal(t, models.StatusQueued, recoveredItem.Status)
	_, statErr := os.Stat(returnBlockedJournalPath(rootPath, shipment.ID, task.ID))
	assert.ErrorIs(t, statErr, os.ErrNotExist)
}

// T002 / ST014: Verify shipment survives rehydration cycle.
func TestShipment_RehydrationConsistency(t *testing.T) {
	// Arrange
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	feat, err := CreateArtifact(ctx, ws, "Rehydration feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Rehydration task", "task", WithParent(feat.ID))
	require.NoError(t, err)
	shipment, err := CreateShipment(ctx, ws, "Rehydration test", []string{task.ID})
	require.NoError(t, err)

	// Force rehydration by closing and reopening workspace
	ws.Close()
	ws2, err := NewWorkspace(ctx, filepath.Dir(ws.RootPath))
	require.NoError(t, err)
	defer ws2.Close()

	// Act
	recovered, err := GetShipment(ctx, ws2, shipment.ID)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, shipment.ID, recovered.ID)
	assert.Equal(t, shipment.Title, recovered.Title)
}

func TestAdoptItem_Success(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feature, err := CreateArtifact(ctx, ws, "Original feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

	task, err := CreateArtifact(ctx, ws, "Orphan candidate", "task", WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))

	// Simulate orphaning: clear parent_id.
	require.NoError(t, clearParentID(ctx, ws, task.ID))
	orphaned, err := loadArtifact(ctx, ws, task.ID)
	require.NoError(t, err)
	assert.True(t, IsOrphan(orphaned), "task should be orphaned after clearParentID")

	newFeature, err := CreateArtifact(ctx, ws, "New feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, newFeature))

	// Act
	result, err := AdoptItem(ctx, ws, task.ID, newFeature.ID)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, task.ID, result.ItemID)
	assert.Equal(t, newFeature.ID, result.NewParentID)
	assert.True(t, result.IsOrphan, "should report item was orphaned")
	assert.NotEmpty(t, result.OriginFeature, "should capture origin feature from ID prefix")

	// After adoption with ID rewrite, look up by the new ID.
	lookupID := result.NewID
	if lookupID == "" {
		lookupID = task.ID
	}
	adopted, err := loadArtifact(ctx, ws, lookupID)
	require.NoError(t, err)
	assert.Equal(t, newFeature.ID, adopted.ParentID)
	assert.Equal(t, feature.ID, adopted.CustomFields["origin_feature"])
	assert.False(t, IsOrphan(adopted), "adopted item should no longer be orphan")

	// Verify old ID is no longer in the index when ID was rewritten.
	if result.NewID != "" && result.NewID != task.ID {
		_, oldErr := bldb.GetItem(ctx, ws.DB, task.ID)
		assert.Error(t, oldErr, "old ID should be removed from index after adoption")
	}
}

func TestAdoptItem_RejectsArchivedItem(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feature, err := CreateArtifact(ctx, ws, "Feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

	task, err := CreateArtifact(ctx, ws, "Archived task", "task", WithParent(feature.ID))
	require.NoError(t, err)
	task.Status = models.StatusArchived
	task.ArchivedFrom = "queue"
	task.ArchivedStatus = "done"
	task.UpdatedAt = time.Now()
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))
	require.NoError(t, WriteArtifactFile(task, findArtifactPathDirect(ws, task.ID)))

	_, err = AdoptItem(ctx, ws, task.ID, feature.ID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "archived")
}

func TestAdoptItem_RejectsMissingParent(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	feat, err := CreateArtifact(ctx, ws, "Parent feature", "feature")
	require.NoError(t, err)
	task, err := CreateArtifact(ctx, ws, "Lonely task", "task", WithParent(feat.ID))
	require.NoError(t, err)

	_, err = AdoptItem(ctx, ws, task.ID, "NONEXISTENT")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "NONEXISTENT")
}

func TestIsOrphan(t *testing.T) {
	tests := []struct {
		name     string
		id       string
		parentID string
		want     bool
	}{
		{"orphaned hierarchical ID", "015.009-T", "", true},
		{"parented hierarchical ID", "015.009-T", "015-F", false},
		{"top-level ID no parent", "001-T", "", false},
		{"deep orphan", "015.001.003-ST", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			a := &models.Artifact{ID: tt.id, ParentID: tt.parentID}
			assert.Equal(t, tt.want, IsOrphan(a))
		})
	}
}

func TestExtractIDPrefix(t *testing.T) {
	tests := []struct {
		id   string
		want string
	}{
		{"015.009-T", "015"},
		{"015.001.003-ST", "015.001"},
		{"001-T", ""},
		{"015-F", ""},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			assert.Equal(t, tt.want, extractIDPrefix(tt.id))
		})
	}
}

// findArtifactPathDirect is a test helper to locate an artifact file.
func findArtifactPathDirect(ws *Workspace, id string) string {
	ctx := context.Background()
	p, err := FindArtifactPath(ctx, ws, id)
	if err != nil {
		return ""
	}
	return p
}

// --- 134.001-T: Failing tests for settable shipment priority (U1 red harness) ---

// TestCreateShipmentWithPriority_PersistsFrontmatter verifies that CreateShipment
// with WithPriority persists the priority to the returned artifact, Markdown
// frontmatter, and the SQLite index.
//
// Red harness: fails until U2 threads WithPriority through CreateShipment.
func TestCreateShipmentWithPriority_PersistsFrontmatter(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	shipment, err := CreateShipment(ctx, ws, "High priority shipment", nil, WithPriority("high"))
	require.NoError(t, err)
	require.NotNil(t, shipment)

	// Assert: returned artifact carries the priority.
	assert.Equal(t, "high", shipment.Priority,
		"CreateShipment with WithPriority must set priority on the returned artifact")

	// Assert: priority was written to Markdown frontmatter (create path uses
	// os.WriteFile directly, not the persistArtifactWriteFn seam, so we assert
	// the file rather than injecting a write seam).
	path, err := FindArtifactPath(ctx, ws, shipment.ID)
	require.NoError(t, err)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "priority: high",
		"priority must be written to the Markdown frontmatter by CreateShipment")

	// Assert: priority is present in the SQLite index after create.
	indexed, err := loadArtifact(ctx, ws, shipment.ID)
	require.NoError(t, err)
	assert.Equal(t, "high", indexed.Priority,
		"priority must be present in the SQLite index after CreateShipment")
}

// TestCreateShipmentWithPriority_QueueSortOrdersByPriority verifies that the queue
// view orders queued shipments by priority when SortBy is "priority". N
// independent pairs defeat Go map-iteration nondeterminism in fixture creation.
// Shipments are created in REVERSE priority order so their sequential IDs would
// produce the WRONG ordering under a pure id-ASC sort, ensuring the test only
// passes when the priority column is actually persisted and used.
//
// Red harness: fails until U2 threads WithPriority through CreateShipment so
// the shipments carry distinct priorities for the queue ordering to use.
func TestCreateShipmentWithPriority_QueueSortOrdersByPriority(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	// Create shipments in REVERSE priority order so their sequential IDs
	// (low→medium→high→critical) would sort lowest-first under a plain id-ASC
	// tie-break. The test only passes once the priority column drives the order.
	empty, err := CreateShipment(ctx, ws, "Unprioritized shipment", nil)
	require.NoError(t, err)
	low, err := CreateShipment(ctx, ws, "Low shipment", nil, WithPriority("low"))
	require.NoError(t, err)
	medium, err := CreateShipment(ctx, ws, "Medium shipment", nil, WithPriority("medium"))
	require.NoError(t, err)
	high, err := CreateShipment(ctx, ws, "High shipment", nil, WithPriority("high"))
	require.NoError(t, err)
	critical, err := CreateShipment(ctx, ws, "Critical shipment", nil, WithPriority("critical"))
	require.NoError(t, err)

	filter := &QueueFilter{
		Types:    []string{"shipment"},
		Statuses: []string{"queued"},
		SortBy:   "priority",
	}
	view, err := QueryQueue(ctx, ws.DB, filter)
	require.NoError(t, err)
	require.GreaterOrEqual(t, view.TotalCount, 5,
		"queue must contain all created shipments")

	idPos := func(id string) int {
		for i, item := range view.Items {
			if item.ID == id {
				return i
			}
		}
		return -1
	}

	// Guard: each expected ID must be present in the result set.
	for _, id := range []string{critical.ID, high.ID, medium.ID, low.ID, empty.ID} {
		require.NotEqual(t, -1, idPos(id),
			"shipment %s must be present in the queue view before comparing ordering", id)
	}

	// Each independent pair: higher priority must appear before lower.
	assert.Less(t, idPos(critical.ID), idPos(high.ID),
		"critical must sort before high")
	assert.Less(t, idPos(high.ID), idPos(medium.ID),
		"high must sort before medium")
	assert.Less(t, idPos(medium.ID), idPos(low.ID),
		"medium must sort before low")
	assert.Less(t, idPos(low.ID), idPos(empty.ID),
		"low must sort before empty-priority")
}

// TestCreateShipmentWithPriority_EmptyPriorityLastAndDeterministic verifies that
// shipments with empty priority sort last, and that two empty-priority shipments
// tie-break deterministically by id ASC.
//
// Red harness: partially fails until U2 persists priority (low must come before
// empty-priority shipments, but without U2 all priorities are empty).
func TestCreateShipmentWithPriority_EmptyPriorityLastAndDeterministic(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	s1, err := CreateShipment(ctx, ws, "Unprioritized first", nil)
	require.NoError(t, err)
	s2, err := CreateShipment(ctx, ws, "Unprioritized second", nil)
	require.NoError(t, err)
	lowShipment, err := CreateShipment(ctx, ws, "Low priority shipment", nil, WithPriority("low"))
	require.NoError(t, err)

	filter := &QueueFilter{
		Types:    []string{"shipment"},
		Statuses: []string{"queued"},
		SortBy:   "priority",
	}
	view, err := QueryQueue(ctx, ws.DB, filter)
	require.NoError(t, err)

	idPos := func(id string) int {
		for i, item := range view.Items {
			if item.ID == id {
				return i
			}
		}
		return -1
	}

	// low must precede both empty-priority shipments.
	assert.Less(t, idPos(lowShipment.ID), idPos(s1.ID),
		"low-priority shipment must sort before empty-priority s1")
	assert.Less(t, idPos(lowShipment.ID), idPos(s2.ID),
		"low-priority shipment must sort before empty-priority s2")

	// s1 and s2 both have empty priority; the tie-break is id ASC.
	if s1.ID < s2.ID {
		assert.Less(t, idPos(s1.ID), idPos(s2.ID),
			"empty-priority tie-break must be id ASC: %s before %s", s1.ID, s2.ID)
	} else {
		assert.Less(t, idPos(s2.ID), idPos(s1.ID),
			"empty-priority tie-break must be id ASC: %s before %s", s2.ID, s1.ID)
	}
}
