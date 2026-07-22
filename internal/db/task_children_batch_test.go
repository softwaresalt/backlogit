package db_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

// TestGetTaskChildrenByParentIDs_GroupsByParent asserts the batched children
// resolver returns each parent's direct TASK children grouped by parent ID,
// ordered by ID, excludes non-task children, and ignores empty/duplicate parents.
// It is the batched counterpart to the per-aggregate childIDsByParent lookup
// behind the size-composition rollup (117-F / A6A1B47E).
func TestGetTaskChildrenByParentIDs_GroupsByParent(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	// F1 has two task children (out-of-order IDs to prove ordering) and one
	// non-task child that must be excluded.
	require.NoError(t, db.UpsertItem(ctx, database, &models.Artifact{ID: "F1", Title: "Feature 1", Status: models.StatusActive, ArtifactType: "feature"}))
	require.NoError(t, db.UpsertItem(ctx, database, &models.Artifact{ID: "F1.002-T", Title: "T b", Status: models.StatusActive, ArtifactType: "task", ParentID: "F1"}))
	require.NoError(t, db.UpsertItem(ctx, database, &models.Artifact{ID: "F1.001-T", Title: "T a", Status: models.StatusActive, ArtifactType: "task", ParentID: "F1"}))
	require.NoError(t, db.UpsertItem(ctx, database, &models.Artifact{ID: "F1.003-ST", Title: "Sub", Status: models.StatusActive, ArtifactType: "subtask", ParentID: "F1"}))

	// F2 has one task child.
	require.NoError(t, db.UpsertItem(ctx, database, &models.Artifact{ID: "F2", Title: "Feature 2", Status: models.StatusActive, ArtifactType: "feature"}))
	require.NoError(t, db.UpsertItem(ctx, database, &models.Artifact{ID: "F2.001-T", Title: "T c", Status: models.StatusActive, ArtifactType: "task", ParentID: "F2"}))

	// F3 has no task children.
	require.NoError(t, db.UpsertItem(ctx, database, &models.Artifact{ID: "F3", Title: "Feature 3", Status: models.StatusActive, ArtifactType: "feature"}))

	// Duplicate and empty parent IDs must be tolerated.
	got, err := db.GetTaskChildrenByParentIDs(ctx, database, []string{"F1", "F2", "F3", "", "F1"})
	require.NoError(t, err)

	require.Contains(t, got, "F1")
	f1IDs := make([]string, len(got["F1"]))
	for i, c := range got["F1"] {
		f1IDs[i] = c.ID
	}
	assert.Equal(t, []string{"F1.001-T", "F1.002-T"}, f1IDs, "task children ordered by ID, non-task excluded")

	require.Contains(t, got, "F2")
	require.Len(t, got["F2"], 1)
	assert.Equal(t, "F2.001-T", got["F2"][0].ID)

	assert.NotContains(t, got, "F3", "a parent with no task children is absent from the map")
	assert.NotContains(t, got, "", "empty parent id is ignored")
}

// TestGetTaskChildrenByParentIDs_EmptyInputs asserts empty and nil inputs return
// an empty map without error (a miss is never an error).
func TestGetTaskChildrenByParentIDs_EmptyInputs(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	got, err := db.GetTaskChildrenByParentIDs(ctx, database, nil)
	require.NoError(t, err)
	assert.Empty(t, got)

	got, err = db.GetTaskChildrenByParentIDs(ctx, nil, []string{"F1"})
	require.NoError(t, err)
	assert.Empty(t, got)
}
