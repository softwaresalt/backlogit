package db_test

// 019.006-T: Duplicate artifact detection (FindDuplicates).

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/db"
	"github.com/backlogit/backlogit/internal/models"
)

func TestFindDuplicates_ReturnsGroupsWithSameTitle(t *testing.T) {
	// 019.006-T: FindDuplicates groups artifacts with identical normalized titles.
	database := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	items := []*models.Artifact{
		{ID: "019D001", Title: "Fix login bug", Status: models.StatusQueued, ArtifactType: "task", CreatedAt: now, UpdatedAt: now},
		{ID: "019D002", Title: "Fix Login Bug", Status: models.StatusQueued, ArtifactType: "task", CreatedAt: now, UpdatedAt: now},
		{ID: "019D003", Title: "fix login bug", Status: models.StatusQueued, ArtifactType: "task", CreatedAt: now, UpdatedAt: now},
		{ID: "019D004", Title: "Unique title here", Status: models.StatusQueued, ArtifactType: "task", CreatedAt: now, UpdatedAt: now},
	}
	for _, a := range items {
		require.NoError(t, db.UpsertItem(ctx, database, a))
	}

	groups, err := db.FindDuplicates(ctx, database)
	require.NoError(t, err)

	// Should find one duplicate group (the three "fix login bug" variants)
	require.Len(t, groups, 1, "expected exactly one duplicate group")
	assert.Equal(t, 3, groups[0].Count)
	assert.Contains(t, groups[0].IDs, "019D001")
	assert.Contains(t, groups[0].IDs, "019D002")
	assert.Contains(t, groups[0].IDs, "019D003")
	assert.NotContains(t, groups[0].IDs, "019D004", "unique title must not appear in duplicate groups")
}

func TestFindDuplicates_ReturnsEmptyOnNoDuplicates(t *testing.T) {
	// 019.006-T: FindDuplicates returns an empty slice when all titles are unique.
	database := setupTestDB(t)
	ctx := context.Background()
	now := time.Now()

	for i, title := range []string{"Alpha task", "Beta task", "Gamma task"} {
		a := &models.Artifact{
			ID:           "019U" + string(rune('A'+i)),
			Title:        title,
			Status:       models.StatusQueued,
			ArtifactType: "task",
			CreatedAt:    now,
			UpdatedAt:    now,
		}
		require.NoError(t, db.UpsertItem(ctx, database, a))
	}

	groups, err := db.FindDuplicates(ctx, database)
	require.NoError(t, err)
	assert.Empty(t, groups, "no duplicate groups expected when all titles are unique")
}

func TestFindDuplicates_EmptyDatabase(t *testing.T) {
	// 019.006-T: FindDuplicates on an empty database returns an empty slice without error.
	database := setupTestDB(t)
	ctx := context.Background()

	groups, err := db.FindDuplicates(ctx, database)
	require.NoError(t, err)
	assert.Empty(t, groups)
}
