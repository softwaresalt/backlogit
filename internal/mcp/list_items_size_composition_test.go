package mcp

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

// TestListItemsExposesSizeComposition asserts the MCP list_items tool attaches
// the computed-on-read size_composition rollup to each aggregate (feature) item
// and omits it from non-aggregate (task) items, at parity with the CLI
// `list --json` surface. Both transports route through the shared core list
// shaper so they cannot drift (117-F / 60336CC0).
func TestListItemsExposesSizeComposition(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()

	feature := &models.Artifact{
		ID:           "970-F",
		Title:        "List projection feature",
		Status:       models.StatusActive,
		ArtifactType: "feature",
	}
	require.NoError(t, db.UpsertItem(ctx, ws.DB, feature))
	require.NoError(t, db.UpsertItem(ctx, ws.DB, &models.Artifact{
		ID:           "970.001-T",
		Title:        "Sized child task",
		Status:       models.StatusActive,
		ArtifactType: "task",
		ParentID:     feature.ID,
		CustomFields: map[string]any{"size": "L", "size_source": "agent"},
	}))

	result, err := s.handleListItems(ctx, contractRequest(map[string]any{}))
	require.NoError(t, err)
	text := resultTextForHarness(t, result)
	require.False(t, result.IsError, text)

	var rows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &rows), "list_items must return a JSON array: %s", text)

	foundFeature, foundTask := false, false
	for _, r := range rows {
		switch r["id"] {
		case feature.ID:
			foundFeature = true
			assert.Contains(t, r, "size_composition", "feature list_items row must carry size_composition")
		case "970.001-T":
			foundTask = true
			_, hasComp := r["size_composition"]
			assert.False(t, hasComp, "task list_items row must NOT carry size_composition")
		}
	}
	assert.True(t, foundFeature, "seeded feature not present in list_items: %s", text)
	assert.True(t, foundTask, "seeded task not present in list_items: %s", text)
}
