package core_test

// 019.005-T: Orphan detection filter for get_queue (QueueFilter.OrphansOnly).

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/db"
	"github.com/backlogit/backlogit/internal/models"
)

func setup019QueueWorkspace(t *testing.T) *core.Workspace {
	t.Helper()
	ws := setupQueueWorkspace(t)
	ctx := context.Background()
	now := time.Now()

	// Orphan: ID contains "." (implies a parent) but parent_id is empty
	orphan := &models.Artifact{
		ID:           "019.901-T",
		Title:        "Orphaned task",
		Status:       models.StatusQueued,
		ArtifactType: "task",
		ParentID:     "", // no parent
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	// Non-orphan: has a parent_id
	parented := &models.Artifact{
		ID:           "019.902-T",
		Title:        "Parented task",
		Status:       models.StatusQueued,
		ArtifactType: "task",
		ParentID:     "019-F",
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	require.NoError(t, db.UpsertItem(ctx, ws.DB, orphan))
	require.NoError(t, db.UpsertItem(ctx, ws.DB, parented))
	return ws
}

func TestQueryQueue_OrphansOnly_ReturnsOnlyOrphans(t *testing.T) {
	// 019.005-T: OrphansOnly=true returns only items whose ID implies a parent
	// (contains ".") but whose parent_id is empty.
	ws := setup019QueueWorkspace(t)
	ctx := context.Background()

	filter := &core.QueueFilter{OrphansOnly: true}
	view, err := core.QueryQueue(ctx, ws.DB, filter)
	require.NoError(t, err)

	for _, item := range view.Items {
		assert.Empty(t, item.ParentID, "orphan filter must return only items with no parent_id")
		assert.Contains(t, item.ID, ".", "orphan filter must only return items whose ID implies a parent")
	}

	// The orphaned task must be present
	ids := make([]string, 0, len(view.Items))
	for _, item := range view.Items {
		ids = append(ids, item.ID)
	}
	assert.Contains(t, ids, "019.901-T")
}

func TestQueryQueue_OrphansOnly_ExcludesParentedItems(t *testing.T) {
	// 019.005-T: OrphansOnly=true must exclude items that have a valid parent_id.
	ws := setup019QueueWorkspace(t)
	ctx := context.Background()

	filter := &core.QueueFilter{OrphansOnly: true}
	view, err := core.QueryQueue(ctx, ws.DB, filter)
	require.NoError(t, err)

	for _, item := range view.Items {
		assert.NotEqual(t, "019.902-T", item.ID, "parented item must not appear in orphan results")
	}
}

func TestQueryQueue_DefaultFilter_IncludesOrphansWithAnnotation(t *testing.T) {
	// 019.005-T: With OrphansOnly=false (default), orphaned items are still
	// returned but annotated with is_orphan=true in CustomFields.
	ws := setup019QueueWorkspace(t)
	ctx := context.Background()

	filter := &core.QueueFilter{Statuses: []string{"queued"}}
	view, err := core.QueryQueue(ctx, ws.DB, filter)
	require.NoError(t, err)

	var foundOrphan bool
	for _, item := range view.Items {
		if item.ID == "019.901-T" {
			foundOrphan = true
			assert.Equal(t, true, item.CustomFields["is_orphan"],
				"orphan must be annotated with is_orphan=true")
		}
	}
	assert.True(t, foundOrphan, "orphaned item must appear in default queue results")
}
