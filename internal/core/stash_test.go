package core_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/core"
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

	remaining, err := core.FetchStash(ctx, ws, core.FetchStashOptions{})
	require.NoError(t, err)
	assert.Empty(t, remaining.Entries)
}
