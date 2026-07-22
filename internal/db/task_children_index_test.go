package db_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

// TestGetTaskChildrenByParentIDs_UsesCompositeIndex asserts the batched
// task-children query is served by idx_items_parent_type_id. The composite
// (parent_id, artifact_type, id) index satisfies both the parent_id +
// artifact_type equality filter and the ORDER BY parent_id, id, so the plan
// neither falls back to the parent-only idx_items_parent nor materializes a
// temp b-tree for sorting (118.001-T / 0FA55F47).
func TestGetTaskChildrenByParentIDs_UsesCompositeIndex(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	require.NoError(t, db.UpsertItem(ctx, database, &models.Artifact{ID: "F1", Title: "F", Status: models.StatusActive, ArtifactType: "feature"}))
	require.NoError(t, db.UpsertItem(ctx, database, &models.Artifact{ID: "F1.001-T", Title: "T", Status: models.StatusActive, ArtifactType: "task", ParentID: "F1"}))

	// Mirror the single-chunk query shape built by GetTaskChildrenByParentIDs.
	query := "EXPLAIN QUERY PLAN SELECT " + db.SelectCols +
		" FROM items WHERE parent_id IN (?) AND artifact_type = 'task' ORDER BY parent_id, id"
	rows, err := database.QueryContext(ctx, query, "F1")
	require.NoError(t, err)
	defer rows.Close()

	var plan strings.Builder
	for rows.Next() {
		var id, parent, notused int
		var detail string
		require.NoError(t, rows.Scan(&id, &parent, &notused, &detail))
		plan.WriteString(detail)
		plan.WriteString("\n")
	}
	require.NoError(t, rows.Err())

	got := plan.String()
	assert.Contains(t, got, "idx_items_parent_type_id",
		"query plan should use the composite index:\n%s", got)
	assert.NotContains(t, got, "USE TEMP B-TREE",
		"composite index should eliminate the ORDER BY sort:\n%s", got)
}
