package core_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/db"
	corerrors "github.com/backlogit/backlogit/internal/errors"
	"github.com/backlogit/backlogit/internal/models"
)

func TestFetchStash_ReturnsEmptyOnFreshWorkspace(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	result, err := core.FetchStash(ctx, ws, core.FetchStashOptions{})
	require.NoError(t, err)
	assert.Empty(t, result.Entries)
}

func TestAddAndHarvestStashEntry(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	entry, err := core.AddStashEntry(ctx, ws, "feature", "high", "Explore audit trail")
	require.NoError(t, err)
	require.NotEmpty(t, entry.ID)
	assert.Equal(t, "high", entry.Priority)

	result, err := core.HarvestStashEntry(ctx, ws, core.HarvestStashOptions{
		StashID:      entry.ID,
		ArtifactType: "feature",
		Description:  "Seeded from stash",
		Status:       "queued",
	})
	require.NoError(t, err)
	assert.Equal(t, entry.ID, result.Entry.ID)
	require.IsType(t, &models.Artifact{}, result.Artifact)

	artifact := result.Artifact.(*models.Artifact)
	require.NotNil(t, artifact.CustomFields)
	assert.Equal(t, "stash.jsonl", artifact.CustomFields["source_stash_path"])

	remaining, err := core.FetchStash(ctx, ws, core.FetchStashOptions{})
	require.NoError(t, err)
	assert.Empty(t, remaining.Entries)
}

func TestLinkDeliberationToStashEntry_ReturnsLinkedDeliberation(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	deliberation, err := core.CreateArtifact(ctx, ws, "Audit split follow-up", "deliberation",
		core.WithPriority("high"),
		core.WithDescription("## Problem Frame\n\n<!-- BEGIN:problem-frame -->\nCapture the trade-offs.\n<!-- END:problem-frame -->"),
	)
	require.NoError(t, err)
	require.NoError(t, db.UpsertItem(ctx, ws.DB, deliberation))

	entry, err := core.AddStashEntry(ctx, ws, "feature", "high", "Split audit dashboard")
	require.NoError(t, err)

	linked, err := core.LinkDeliberationToStashEntry(ctx, ws, entry.ID, deliberation.ID)
	require.NoError(t, err)
	assert.Equal(t, deliberation.ID, linked.DeliberationID)
	require.NotNil(t, linked.Deliberation)
	assert.Equal(t, deliberation.ID, linked.Deliberation.ID)

	fetched, err := core.FetchStash(ctx, ws, core.FetchStashOptions{})
	require.NoError(t, err)
	require.Len(t, fetched.Entries, 1)
	assert.Equal(t, deliberation.ID, fetched.Entries[0].DeliberationID)
	require.NotNil(t, fetched.Entries[0].Deliberation)
}

func TestAddStashEntry_SetsCreatedAt(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	before := time.Now().UTC()
	entry, err := core.AddStashEntry(ctx, ws, "task", "medium", "Check created_at is set")
	require.NoError(t, err)
	after := time.Now().UTC()

	require.NotNil(t, entry.CreatedAt, "CreatedAt must be non-nil")
	assert.True(t, !entry.CreatedAt.Before(before), "CreatedAt must be >= before")
	assert.True(t, !entry.CreatedAt.After(after), "CreatedAt must be <= after")
}

func TestFetchStash_KindFilter(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	_, err := core.AddStashEntry(ctx, ws, "feature", "high", "Feature item")
	require.NoError(t, err)
	_, err = core.AddStashEntry(ctx, ws, "task", "medium", "Task item")
	require.NoError(t, err)
	_, err = core.AddStashEntry(ctx, ws, "bug", "critical", "Bug item")
	require.NoError(t, err)

	result, err := core.FetchStash(ctx, ws, core.FetchStashOptions{Kind: "feature"})
	require.NoError(t, err)
	require.Len(t, result.Entries, 1)
	assert.Equal(t, "feature", result.Entries[0].Kind)

	result, err = core.FetchStash(ctx, ws, core.FetchStashOptions{Kind: "task"})
	require.NoError(t, err)
	require.Len(t, result.Entries, 1)
	assert.Equal(t, "task", result.Entries[0].Kind)
}

func TestFetchStash_InvalidKindFilter(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	_, err := core.FetchStash(ctx, ws, core.FetchStashOptions{Kind: "invalid-kind"})
	require.Error(t, err)
}

func TestRemoveStashEntry_RemovesAndReturns(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	added, err := core.AddStashEntry(ctx, ws, "task", "high", "Item to remove")
	require.NoError(t, err)

	removed, err := core.RemoveStashEntry(ctx, ws, added.ID)
	require.NoError(t, err)
	require.NotNil(t, removed)
	assert.Equal(t, added.ID, removed.ID)

	result, err := core.FetchStash(ctx, ws, core.FetchStashOptions{})
	require.NoError(t, err)
	assert.Empty(t, result.Entries)
}

func TestRemoveStashEntry_NotFound(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	_, err := core.RemoveStashEntry(ctx, ws, "DEADBEEF")
	require.Error(t, err)
	assert.ErrorIs(t, err, corerrors.ErrNotFound)
}

func TestEditStashEntry_UpdatesFields(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	added, err := core.AddStashEntry(ctx, ws, "task", "medium", "Original text")
	require.NoError(t, err)

	updated, err := core.EditStashEntry(ctx, ws, added.ID, core.EditStashOptions{
		Text:     "Updated text",
		Kind:     "feature",
		Priority: "high",
	})
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "Updated text", updated.Text)
	assert.Equal(t, "feature", updated.Kind)
	assert.Equal(t, "high", updated.Priority)
}

func TestEditStashEntry_PartialUpdate(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	added, err := core.AddStashEntry(ctx, ws, "bug", "low", "Bug description")
	require.NoError(t, err)

	updated, err := core.EditStashEntry(ctx, ws, added.ID, core.EditStashOptions{
		Priority: "critical",
	})
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "Bug description", updated.Text, "text must be unchanged")
	assert.Equal(t, "bug", updated.Kind, "kind must be unchanged")
	assert.Equal(t, "critical", updated.Priority)
}

func TestEditStashEntry_NotFound(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	_, err := core.EditStashEntry(ctx, ws, "DEADBEEF", core.EditStashOptions{Text: "new"})
	require.Error(t, err)
	assert.ErrorIs(t, err, corerrors.ErrNotFound)
}
