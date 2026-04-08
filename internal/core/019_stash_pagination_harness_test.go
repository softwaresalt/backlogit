package core_test

// 019.002-T: Pagination support for FetchStash (fetch_stash MCP tool).

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/core"
)

func TestFetchStash_Limit_ReturnsAtMostN(t *testing.T) {
	// 019.002-T: FetchStash with Limit=2 returns at most 2 entries even when more exist.
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	for _, text := range []string{"Idea one", "Idea two", "Idea three", "Idea four"} {
		_, err := core.AddStashEntry(ctx, ws, "task", "medium", text)
		require.NoError(t, err)
	}

	result, err := core.FetchStash(ctx, ws, core.FetchStashOptions{Limit: 2})
	require.NoError(t, err)
	assert.Len(t, result.Entries, 2, "Limit=2 must cap entries at 2")
}

func TestFetchStash_ZeroLimit_ReturnsAll(t *testing.T) {
	// 019.002-T: Limit=0 (default) returns all entries regardless of count.
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	for _, text := range []string{"Idea one", "Idea two", "Idea three"} {
		_, err := core.AddStashEntry(ctx, ws, "task", "medium", text)
		require.NoError(t, err)
	}

	result, err := core.FetchStash(ctx, ws, core.FetchStashOptions{Limit: 0})
	require.NoError(t, err)
	assert.Len(t, result.Entries, 3, "Limit=0 should return all 3 entries")
}

func TestFetchStash_LimitWithPriority_AppliedAfterFilter(t *testing.T) {
	// 019.002-T: Limit is applied after priority filter, not before.
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	// Add 3 high-priority and 2 medium-priority entries
	for i := 0; i < 3; i++ {
		_, err := core.AddStashEntry(ctx, ws, "task", "high", "High priority idea")
		require.NoError(t, err)
	}
	for i := 0; i < 2; i++ {
		_, err := core.AddStashEntry(ctx, ws, "task", "medium", "Medium priority idea")
		require.NoError(t, err)
	}

	result, err := core.FetchStash(ctx, ws, core.FetchStashOptions{Priority: "high", Limit: 2})
	require.NoError(t, err)
	assert.Len(t, result.Entries, 2, "Limit=2 with priority=high should return 2 high-priority entries")
	for _, e := range result.Entries {
		assert.Equal(t, "high", e.Priority)
	}
}
