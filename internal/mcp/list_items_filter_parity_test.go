package mcp

// 122.001-T: Regression tests for priority/owner filter params in handleListItems.
//
// These tests verify that handleListItems correctly applies priority and owner
// filters when they are present in the MCP request arguments, and that omitting
// them preserves the existing unfiltered behavior. They serve as the permanent
// regression guard for the CLI/MCP filter parity established in 122.001-T.

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

// seedListFilterItems seeds two features with distinct priority/owner values for
// use by the handleListItems filter tests.
func seedListFilterItems(t *testing.T, ws *core.Workspace) (highID, lowID string) {
	t.Helper()
	ctx := context.Background()

	high := &models.Artifact{
		ID:           "LFP-001-F",
		Title:        "High priority feature",
		Status:       models.StatusQueued,
		ArtifactType: "feature",
		Priority:     "high",
		Owner:        "alice",
	}
	low := &models.Artifact{
		ID:           "LFP-002-F",
		Title:        "Low priority feature",
		Status:       models.StatusQueued,
		ArtifactType: "feature",
		Priority:     "low",
		Owner:        "bob",
	}
	require.NoError(t, db.UpsertItem(ctx, ws.DB, high))
	require.NoError(t, db.UpsertItem(ctx, ws.DB, low))
	return high.ID, low.ID
}

// listItemIDs unmarshals the JSON array text from a handleListItems result and
// returns the set of "id" values.
func listItemIDs(t *testing.T, text string) map[string]bool {
	t.Helper()
	var rows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(text), &rows), "list_items must return a JSON array; got: %s", text)
	ids := make(map[string]bool, len(rows))
	for _, r := range rows {
		if id, ok := r["id"].(string); ok {
			ids[id] = true
		}
	}
	return ids
}

// TestHandleListItems_FilterByPriority asserts that passing priority="high" in
// the request arguments returns only items with that priority. Before wiring the
// param this test must FAIL because priority is ignored and both items are
// returned.
func TestHandleListItems_FilterByPriority(t *testing.T) {
	s, ws := setupBugFixServer(t)
	highID, lowID := seedListFilterItems(t, ws)
	ctx := context.Background()

	result, err := s.handleListItems(ctx, contractRequest(map[string]any{
		"priority": "high",
	}))
	require.NoError(t, err)
	text := resultTextForHarness(t, result)
	require.False(t, result.IsError, text)

	ids := listItemIDs(t, text)

	assert.True(t, ids[highID], "priority=high filter must include the high-priority item")
	assert.False(t, ids[lowID], "priority=high filter must exclude the low-priority item")
}

// TestHandleListItems_FilterByOwner asserts that passing owner="alice" in the
// request arguments returns only items owned by alice. Before wiring the param
// this test must FAIL because owner is ignored and both items are returned.
func TestHandleListItems_FilterByOwner(t *testing.T) {
	s, ws := setupBugFixServer(t)
	highID, lowID := seedListFilterItems(t, ws)
	ctx := context.Background()

	result, err := s.handleListItems(ctx, contractRequest(map[string]any{
		"owner": "alice",
	}))
	require.NoError(t, err)
	text := resultTextForHarness(t, result)
	require.False(t, result.IsError, text)

	ids := listItemIDs(t, text)

	assert.True(t, ids[highID], "owner=alice filter must include alice's item")
	assert.False(t, ids[lowID], "owner=alice filter must exclude bob's item")
}

// TestHandleListItems_OmittedParamsPreserveBehavior asserts that omitting
// priority and owner still returns all seeded items, preserving existing behavior.
func TestHandleListItems_OmittedParamsPreserveBehavior(t *testing.T) {
	s, ws := setupBugFixServer(t)
	highID, lowID := seedListFilterItems(t, ws)
	ctx := context.Background()

	result, err := s.handleListItems(ctx, contractRequest(map[string]any{}))
	require.NoError(t, err)
	text := resultTextForHarness(t, result)
	require.False(t, result.IsError, text)

	ids := listItemIDs(t, text)

	assert.True(t, ids[highID], "omitting priority/owner must include the high-priority item")
	assert.True(t, ids[lowID], "omitting priority/owner must include the low-priority item")
}
