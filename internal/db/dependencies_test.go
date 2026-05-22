package db_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

func setupDepsTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "deps_test.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)
	require.NoError(t, db.EnsureSchema(database))
	t.Cleanup(func() { database.Close() })

	// Seed test items
	ctx := context.Background()
	for _, a := range []*models.Artifact{
		{ID: "T001", Title: "Task 1", Status: models.StatusQueued, ArtifactType: "task"},
		{ID: "T002", Title: "Task 2", Status: models.StatusQueued, ArtifactType: "task"},
		{ID: "T003", Title: "Task 3", Status: models.StatusDone, ArtifactType: "task"},
	} {
		require.NoError(t, db.UpsertItem(ctx, database, a))
	}
	return database
}

func TestUpsertDependency(t *testing.T) {
	// Arrange
	database := setupDepsTestDB(t)
	ctx := context.Background()

	// Act
	err := db.UpsertDependency(ctx, database, "T001", "T003", "blocks")

	// Assert
	require.NoError(t, err)
}

func TestGetDependencies(t *testing.T) {
	// Arrange
	database := setupDepsTestDB(t)
	ctx := context.Background()
	require.NoError(t, db.UpsertDependency(ctx, database, "T001", "T003", "blocks"))

	// Act
	deps, err := db.GetDependencies(ctx, database, "T001")

	// Assert
	require.NoError(t, err)
	require.Len(t, deps, 1)
	assert.Equal(t, "T003", deps[0].DependsOn)
	assert.Equal(t, "blocks", deps[0].DepType)
}

func TestGetDependents(t *testing.T) {
	// Arrange
	database := setupDepsTestDB(t)
	ctx := context.Background()
	require.NoError(t, db.UpsertDependency(ctx, database, "T001", "T003", "blocks"))

	// Act
	deps, err := db.GetDependents(ctx, database, "T003")

	// Assert
	require.NoError(t, err)
	require.Len(t, deps, 1)
	assert.Equal(t, "T001", deps[0].ItemID)
}

func TestDeleteDependency(t *testing.T) {
	// Arrange
	database := setupDepsTestDB(t)
	ctx := context.Background()
	require.NoError(t, db.UpsertDependency(ctx, database, "T001", "T003", "blocks"))

	// Act
	err := db.DeleteDependency(ctx, database, "T001", "T003")

	// Assert
	require.NoError(t, err)
	deps, err := db.GetDependencies(ctx, database, "T001")
	require.NoError(t, err)
	assert.Empty(t, deps)
}

func TestDetectCycle(t *testing.T) {
	// Arrange
	database := setupDepsTestDB(t)
	ctx := context.Background()
	require.NoError(t, db.UpsertDependency(ctx, database, "T001", "T002", "blocks"))
	require.NoError(t, db.UpsertDependency(ctx, database, "T002", "T003", "blocks"))

	// Act — adding T003→T001 would create a cycle
	hasCycle, err := db.DetectCycle(ctx, database, "T003", "T001")

	// Assert
	require.NoError(t, err)
	assert.True(t, hasCycle, "should detect cycle: T001→T002→T003→T001")
}

func TestAddDependencyChecked_ValidatesTargetsBeforeInsert(t *testing.T) {
	testCases := []struct {
		name      string
		itemID    string
		dependsOn string
	}{
		{
			name:      "missing source item",
			itemID:    "T404",
			dependsOn: "T001",
		},
		{
			name:      "missing target item",
			itemID:    "T001",
			dependsOn: "T404",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			database := setupDepsTestDB(t)
			ctx := context.Background()

			err := db.AddDependencyChecked(ctx, database, tc.itemID, tc.dependsOn, "blocks")
			require.Error(t, err, "invalid dependency targets must return an error")

			var count int
			require.NoError(t, database.QueryRowContext(ctx,
				`SELECT COUNT(*) FROM item_deps WHERE item_id = ? AND depends_on = ?`,
				tc.itemID, tc.dependsOn,
			).Scan(&count))
			assert.Zero(t, count, "invalid dependency targets must not be persisted")
		})
	}
}
