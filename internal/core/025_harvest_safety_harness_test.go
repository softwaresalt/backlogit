package core_test

// 025.015-T (Unit 3): Enforce hierarchy constraints in HarvestStashEntry and fix
// the stash data-loss ordering bug (P0 finding F-003).
//
// Red tests (fail until Units 2 and/or 3 are implemented):
//   - TestHarvestStashEntry_RejectsTaskWithoutParent (no hierarchy check in CreateArtifact → harvest succeeds when it should fail)
//   - TestHarvestStashEntry_PreservesStashOnCreateFailure (stash removed before CreateArtifact → data loss on failure)
//
// Green tests (pass with current code):
//   - TestHarvestStashEntry_SucceedsWithParent

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/models"
)

// TestHarvestStashEntry_RejectsTaskWithoutParent asserts that harvesting a task-kind
// stash entry without providing a parent_id returns an error and leaves the stash entry
// intact (not consumed).
//
// RED until Unit 2 adds hierarchy enforcement to CreateArtifact (so the harvest fails),
// AND Unit 3 fixes the ordering so the stash is not removed before CreateArtifact succeeds.
func TestHarvestStashEntry_RejectsTaskWithoutParent(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	entry, err := core.AddStashEntry(ctx, ws, "task", "medium", "Task needing a parent feature")
	require.NoError(t, err)

	// Attempt to harvest as task with no parent_id
	_, harvestErr := core.HarvestStashEntry(ctx, ws, core.HarvestStashOptions{
		StashID:      entry.ID,
		ArtifactType: "task",
		// No ParentID — must fail at hierarchy validation
	})

	require.Error(t, harvestErr, "harvesting a task-kind stash entry without parent_id must return an error")

	// Stash entry must remain intact — not consumed on failure
	remaining, fetchErr := core.FetchStash(ctx, ws, core.FetchStashOptions{})
	require.NoError(t, fetchErr)
	require.Len(t, remaining.Entries, 1, "stash entry must not be consumed when harvest fails validation")
	assert.Equal(t, entry.ID, remaining.Entries[0].ID)
}

// TestHarvestStashEntry_SucceedsWithParent asserts that harvesting a task-kind stash
// entry with a valid parent_id succeeds and removes the stash entry.
// GREEN with current code and after Units 2 and 3.
func TestHarvestStashEntry_SucceedsWithParent(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feature, err := core.CreateArtifact(ctx, ws, "Parent feature for harvest", "feature")
	require.NoError(t, err)

	entry, err := core.AddStashEntry(ctx, ws, "task", "high", "Task to harvest under feature")
	require.NoError(t, err)

	result, err := core.HarvestStashEntry(ctx, ws, core.HarvestStashOptions{
		StashID:      entry.ID,
		ArtifactType: "task",
		ParentID:     feature.ID,
	})

	require.NoError(t, err, "harvesting a task-kind stash entry with a valid parent_id must succeed")
	require.NotNil(t, result)
	assert.Equal(t, entry.ID, result.Entry.ID)
	artifact, ok := result.Artifact.(*models.Artifact)
	require.True(t, ok, "HarvestedStashResult.Artifact must be *models.Artifact")
	assert.Equal(t, feature.ID, artifact.ParentID, "harvested task must carry the provided parent_id")

	// Stash entry must be consumed on success
	remaining, fetchErr := core.FetchStash(ctx, ws, core.FetchStashOptions{})
	require.NoError(t, fetchErr)
	assert.Empty(t, remaining.Entries, "stash entry must be consumed after successful harvest")
}

// TestHarvestStashEntry_PreservesStashOnCreateFailure asserts that if CreateArtifact
// fails for any reason, the stash JSONL file still contains the original entry.
// This tests the P0 data-loss ordering bug (F-003): the current implementation
// removes the stash entry BEFORE CreateArtifact runs, so a failed harvest consumes
// the stash entry permanently.
//
// RED until Unit 3 reorders the operations: validate first, mutate stash only on success.
func TestHarvestStashEntry_PreservesStashOnCreateFailure(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	entry, err := core.AddStashEntry(ctx, ws, "task", "high", "Entry that must survive failed harvest")
	require.NoError(t, err)

	// Use an artifact type that does not exist — CreateArtifact will fail with
	// "unknown artifact type". This exercises the path where stash removal has
	// already happened (current bug) vs. only happening after success (fixed behaviour).
	_, harvestErr := core.HarvestStashEntry(ctx, ws, core.HarvestStashOptions{
		StashID:      entry.ID,
		ArtifactType: "nonexistent_type",
	})

	require.Error(t, harvestErr, "harvesting with unknown artifact type must return an error")

	// Key assertion: stash entry must still be present after the failed harvest.
	remaining, fetchErr := core.FetchStash(ctx, ws, core.FetchStashOptions{})
	require.NoError(t, fetchErr)
	require.Len(t, remaining.Entries, 1,
		"stash entry must not be lost when CreateArtifact fails — P0 data-loss bug (F-003)")
	assert.Equal(t, entry.ID, remaining.Entries[0].ID)
}
