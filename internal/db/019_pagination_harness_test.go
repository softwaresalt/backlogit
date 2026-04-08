package db_test

// 019.001-T: Pagination support for QueryItems (list_items MCP tool).

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/db"
	"github.com/backlogit/backlogit/internal/models"
)

func TestQueryItems_Limit(t *testing.T) {
	// 019.001-T: QueryItems with Limit=2 returns at most 2 items.
	database := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	for i, title := range []string{"Alpha", "Beta", "Gamma", "Delta", "Epsilon"} {
		a := &models.Artifact{
			ID:           "019A" + string(rune('A'+i)),
			Title:        title,
			Status:       models.StatusQueued,
			ArtifactType: "task",
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		require.NoError(t, db.UpsertItem(ctx, database, a))
	}

	items, err := db.QueryItems(ctx, database, db.QueryFilters{Limit: 2})
	require.NoError(t, err)
	assert.Len(t, items, 2, "Limit=2 should return exactly 2 items")
}

func TestQueryItems_Offset(t *testing.T) {
	// 019.001-T: QueryItems with Offset skips the leading rows, enabling pagination.
	database := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	for i, title := range []string{"Alpha", "Beta", "Gamma", "Delta", "Epsilon"} {
		a := &models.Artifact{
			ID:           "019B" + string(rune('A'+i)),
			Title:        title,
			Status:       models.StatusQueued,
			ArtifactType: "task",
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		require.NoError(t, db.UpsertItem(ctx, database, a))
	}

	// First page
	page1, err := db.QueryItems(ctx, database, db.QueryFilters{Limit: 2, Offset: 0})
	require.NoError(t, err)
	require.Len(t, page1, 2)

	// Second page must not overlap with first
	page2, err := db.QueryItems(ctx, database, db.QueryFilters{Limit: 2, Offset: 2})
	require.NoError(t, err)
	require.Len(t, page2, 2)

	ids1 := map[string]bool{page1[0].ID: true, page1[1].ID: true}
	for _, a := range page2 {
		assert.False(t, ids1[a.ID], "page 2 item %s should not appear on page 1", a.ID)
	}
}

func TestQueryItems_ZeroLimit_ReturnsAll(t *testing.T) {
	// 019.001-T: Limit=0 (default) means no limit; all matching items are returned.
	database := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	for i, title := range []string{"Alpha", "Beta", "Gamma"} {
		a := &models.Artifact{
			ID:           "019C" + string(rune('A'+i)),
			Title:        title,
			Status:       models.StatusQueued,
			ArtifactType: "task",
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		require.NoError(t, db.UpsertItem(ctx, database, a))
	}

	items, err := db.QueryItems(ctx, database, db.QueryFilters{Limit: 0})
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(items), 3, "Limit=0 should return all items")
}
