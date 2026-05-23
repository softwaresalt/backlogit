package core_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

func TestQueryQueue_DependencyLookupFailureDoesNotSilentlyHideItems(t *testing.T) {
	ws := setupQueueWorkspace(t)
	ctx := context.Background()

	_, err := ws.DB.ExecContext(ctx, `DROP TABLE item_deps`)
	require.NoError(t, err)

	_, err = core.QueryQueue(ctx, ws.DB, &core.QueueFilter{Statuses: []string{"queued"}})
	require.Error(t, err, "dependency lookup failures must be explicit")
	assert.Contains(t, err.Error(), "item_deps", "dependency lookup failures should mention the missing table")
}

func TestQueryQueue_BlocksExpectedDependencyTypes(t *testing.T) {
	testCases := []struct {
		name    string
		depType string
	}{
		{
			name:    "blocks",
			depType: "blocks",
		},
		{
			name:    "parent_of",
			depType: "parent_of",
		},
		{
			name:    "relates_to",
			depType: "relates_to",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			ws := setupQueueWorkspace(t)
			ctx := context.Background()

			require.NoError(t, db.UpsertDependency(ctx, ws.DB, "T001", "T002", tc.depType))

			view, err := core.QueryQueue(ctx, ws.DB, &core.QueueFilter{Statuses: []string{"queued"}})
			require.NoError(t, err)

			assert.NotContains(t, queueIDs(view.Items), "T001",
				"queued items with unresolved %s dependencies must stay blocked", tc.depType)
			assert.Contains(t, queueIDs(view.Items), "B001",
				"independent queued items must remain visible when %s blocks another item", tc.depType)
		})
	}
}

func queueIDs(items []*models.Artifact) []string {
	ids := make([]string, 0, len(items))
	for _, item := range items {
		ids = append(ids, item.ID)
	}
	return ids
}
