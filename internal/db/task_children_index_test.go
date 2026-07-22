package db_test

import (
	"context"
	"database/sql"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

// explainPlan returns the concatenated EXPLAIN QUERY PLAN detail rows for a
// query. EXPLAIN QUERY PLAN yields four columns (id, parent, notused, detail);
// modernc.org/sqlite returns the integer columns as int64.
func explainPlan(t *testing.T, database *sql.DB, query string, args ...any) string {
	t.Helper()
	rows, err := database.QueryContext(context.Background(), "EXPLAIN QUERY PLAN "+query, args...)
	require.NoError(t, err)
	defer rows.Close()

	var plan strings.Builder
	for rows.Next() {
		var id, parent, notused int64
		var detail string
		require.NoError(t, rows.Scan(&id, &parent, &notused, &detail))
		plan.WriteString(detail)
		plan.WriteString("\n")
	}
	require.NoError(t, rows.Err())
	return plan.String()
}

// batchChildrenQuery mirrors the chunk query shape built by
// GetTaskChildrenByParentIDs for the given parent-id placeholders.
func batchChildrenQuery(placeholders string) string {
	return "SELECT " + db.SelectCols + " FROM items WHERE parent_id IN (" + placeholders +
		") AND artifact_type = 'task' ORDER BY parent_id, id"
}

// TestTaskChildrenQueryPlans asserts the batched task-children query is served
// by the composite idx_items_parent_type_id and that the narrower
// idx_items_parent is still the planner's preference for pure parent_id lookups
// while both indexes exist. That preference is not proof of non-redundancy (the
// composite could serve the bare lookup via its leading column); idx_items_parent
// is retained as a conservative default (118.001-T / 0FA55F47).
func TestTaskChildrenQueryPlans(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	// Two parents, each with task children (out-of-order IDs) plus a subtask
	// that the artifact_type filter must exclude.
	seed := []*models.Artifact{
		{ID: "F1", Title: "F1", Status: models.StatusActive, ArtifactType: "feature"},
		{ID: "F2", Title: "F2", Status: models.StatusActive, ArtifactType: "feature"},
		{ID: "F1.002-T", Title: "b", Status: models.StatusActive, ArtifactType: "task", ParentID: "F1"},
		{ID: "F1.001-T", Title: "a", Status: models.StatusActive, ArtifactType: "task", ParentID: "F1"},
		{ID: "F2.001-T", Title: "c", Status: models.StatusActive, ArtifactType: "task", ParentID: "F2"},
		{ID: "F1.003-ST", Title: "s", Status: models.StatusActive, ArtifactType: "subtask", ParentID: "F1"},
	}
	for _, a := range seed {
		require.NoError(t, db.UpsertItem(ctx, database, a))
	}

	t.Run("single-parent batch uses composite index and elides the sort", func(t *testing.T) {
		plan := explainPlan(t, database, batchChildrenQuery("?"), "F1")
		assert.Contains(t, plan, "idx_items_parent_type_id",
			"single-parent batch should use the composite index:\n%s", plan)
		// With one parent value the equality lets the index supply the ordering,
		// so no temporary b-tree is materialized for ORDER BY.
		assert.NotContains(t, plan, "USE TEMP B-TREE",
			"composite index should eliminate the ORDER BY sort:\n%s", plan)
	})

	t.Run("multi-parent batch uses composite index", func(t *testing.T) {
		// The real GetTaskChildrenByParentIDs chunk shape is a multi-value IN.
		// Assert index selection only: whether the multi-value IN also elides the
		// sort is planner-version-dependent, so it is not asserted here.
		plan := explainPlan(t, database, batchChildrenQuery("?,?"), "F1", "F2")
		assert.Contains(t, plan, "idx_items_parent_type_id",
			"multi-parent batch should use the composite index:\n%s", plan)
	})

	t.Run("parent-only lookup keeps the narrower idx_items_parent", func(t *testing.T) {
		// The planner prefers the narrower index for a bare parent_id equality
		// (e.g. the ListItems parent filter) while both indexes exist. This
		// preference is why idx_items_parent is retained as a conservative
		// default; it is not proof of non-redundancy, since the composite could
		// also serve this lookup via its leading parent_id column.
		plan := explainPlan(t, database, "SELECT "+db.SelectCols+" FROM items WHERE parent_id = ?", "F1")
		assert.Contains(t, plan, "idx_items_parent (",
			"parent-only lookup should use the narrow idx_items_parent:\n%s", plan)
		assert.NotContains(t, plan, "idx_items_parent_type_id",
			"parent-only lookup should not use the composite index:\n%s", plan)
	})
}
