package core_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/db"
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
