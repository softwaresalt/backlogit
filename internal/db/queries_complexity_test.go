package db_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

func TestQueryItems_ComplexityFilterMatchesTasks(t *testing.T) {
	ctx := context.Background()
	database := setupProjectionDB(t)
	require.NoError(t, db.UpsertItem(ctx, database, projectionArtifact("955.001-T", map[string]any{"complexity": "high"})))
	require.NoError(t, db.UpsertItem(ctx, database, projectionArtifact("955.002-T", map[string]any{"complexity": "low"})))

	items, err := db.QueryItems(ctx, database, db.QueryFilters{Complexity: "high"})
	require.NoError(t, err)

	require.Len(t, items, 1)
	assert.Equal(t, "955.001-T", items[0].ID)
}

func TestQueryItems_ComplexityFilterExcludesNonTasks(t *testing.T) {
	ctx := context.Background()
	database := setupProjectionDB(t)
	require.NoError(t, db.UpsertItem(ctx, database, projectionArtifact("955.003-T", map[string]any{"complexity": "high"})))

	feature := projectionArtifact("955-F", map[string]any{})
	feature.ArtifactType = "feature"
	feature.ParentID = ""
	feature.Level = 1
	feature.HierarchyPath = "955"
	require.NoError(t, db.UpsertItem(ctx, database, feature))
	_, err := database.ExecContext(ctx, `UPDATE items SET complexity = ? WHERE id = ?`, "high", feature.ID)
	require.NoError(t, err)

	items, err := db.QueryItems(ctx, database, db.QueryFilters{Complexity: "high"})
	require.NoError(t, err)

	require.Len(t, items, 1)
	assert.Equal(t, "955.003-T", items[0].ID)
	assert.Equal(t, "task", items[0].ArtifactType)
}

func TestQueryItems_BlankComplexityDoesNotConstrain(t *testing.T) {
	ctx := context.Background()
	database := setupProjectionDB(t)
	require.NoError(t, db.UpsertItem(ctx, database, projectionArtifact("956.001-T", map[string]any{"complexity": "medium"})))
	require.NoError(t, db.UpsertItem(ctx, database, projectionArtifact("956.002-T", map[string]any{"complexity": "low"})))

	items, err := db.QueryItems(ctx, database, db.QueryFilters{Complexity: ""})
	require.NoError(t, err)

	assert.Len(t, items, 2)
}

func TestQueryItems_ComplexityComposesWithOtherFilters(t *testing.T) {
	ctx := context.Background()
	database := setupProjectionDB(t)
	activeHigh := projectionArtifact("957.001-T", map[string]any{"complexity": "high"})
	activeHigh.Priority = "critical"
	doneHigh := projectionArtifact("957.002-T", map[string]any{"complexity": "high"})
	doneHigh.Status = models.StatusDone
	doneHigh.Priority = "critical"
	activeLow := projectionArtifact("957.003-T", map[string]any{"complexity": "low"})
	activeLow.Priority = "critical"
	require.NoError(t, db.UpsertItem(ctx, database, activeHigh))
	require.NoError(t, db.UpsertItem(ctx, database, doneHigh))
	require.NoError(t, db.UpsertItem(ctx, database, activeLow))

	items, err := db.QueryItems(ctx, database, db.QueryFilters{
		Status:     string(models.StatusActive),
		Type:       "task",
		Priority:   "critical",
		Complexity: "high",
	})
	require.NoError(t, err)

	require.Len(t, items, 1)
	assert.Equal(t, activeHigh.ID, items[0].ID)
}

func TestQueryItems_ComplexityFilterUsesBoundParameter(t *testing.T) {
	ctx := context.Background()
	database := setupProjectionDB(t)
	require.NoError(t, db.UpsertItem(ctx, database, projectionArtifact("958.001-T", map[string]any{"complexity": "high"})))
	require.NoError(t, db.UpsertItem(ctx, database, projectionArtifact("958.002-T", map[string]any{"complexity": "low"})))

	items, err := db.QueryItems(ctx, database, db.QueryFilters{Complexity: "high' OR '1'='1"})
	require.NoError(t, err)

	assert.Empty(t, items)
}
