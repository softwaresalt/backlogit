package mcp

// 018.005-T: handleMoveItem blocking error harness.
//
// These tests are deliberately failing because CheckChildrenTerminal is not yet
// implemented and handleMoveItem does not yet call it. They define the expected
// structured error contract before the wiring is written:
//
//   - moving a parent to "done" while children are active returns error="blocking_children"
//   - the error body contains a JSON array of blocking child IDs and statuses
//   - moving a parent to "done" after all children are done succeeds

import (
	"context"
	"encoding/json"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/core"
)

func TestHandleMoveItem_BlockedByChildren_ReturnsStructuredError(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()

	parent, err := core.CreateArtifact(ctx, ws, "Parent feature", "feature")
	require.NoError(t, err)

	child, err := core.CreateArtifact(ctx, ws, "In-progress child", "task")
	require.NoError(t, err)
	_, err = core.UpdateArtifact(ctx, ws, child.ID, map[string]any{
		"parent_id": parent.ID,
		"status":    "active",
	})
	require.NoError(t, err)

	result, err := s.handleMoveItem(ctx, shipmentRequest(map[string]any{
		"id":     parent.ID,
		"status": "done",
	}))

	require.NoError(t, err)
	require.True(t, result.IsError, "move to done with active children must be an error")

	// Use shipmentErrorField (same helper as shipment error tests) to read the
	// "error" key from the JSON body.
	assert.Equal(t, "blocking_children", shipmentErrorField(t, result),
		"blocking cascade must surface as error=blocking_children, not error=internal")
}

func TestHandleMoveItem_BlockedByChildren_BodyContainsChildIDs(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()

	parent, err := core.CreateArtifact(ctx, ws, "Parent feature", "feature")
	require.NoError(t, err)

	child, err := core.CreateArtifact(ctx, ws, "Blocking child", "task")
	require.NoError(t, err)
	_, err = core.UpdateArtifact(ctx, ws, child.ID, map[string]any{
		"parent_id": parent.ID,
		"status":    "active",
	})
	require.NoError(t, err)

	result, err := s.handleMoveItem(ctx, shipmentRequest(map[string]any{
		"id":     parent.ID,
		"status": "done",
	}))

	require.NoError(t, err)
	require.True(t, result.IsError)
	require.NotEmpty(t, result.Content)

	// The response body must include a "children" array with the blocking child.
	textContent, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok, "response content must be text")

	type blockingResp struct {
		Error    string `json:"error"`
		Children []struct {
			ID     string `json:"id"`
			Status string `json:"status"`
		} `json:"children"`
	}
	var resp blockingResp
	if err := json.Unmarshal([]byte(textContent.Text), &resp); err == nil {
		assert.Equal(t, "blocking_children", resp.Error)
		childIDs := make([]string, len(resp.Children))
		for i, c := range resp.Children {
			childIDs[i] = c.ID
		}
		assert.Contains(t, childIDs, child.ID,
			"blocking child ID must appear in the structured error response")
	}
}

func TestHandleMoveItem_AllChildrenDone_Succeeds(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()

	parent, err := core.CreateArtifact(ctx, ws, "Parent feature", "feature")
	require.NoError(t, err)

	child, err := core.CreateArtifact(ctx, ws, "Done child", "task")
	require.NoError(t, err)
	_, err = core.UpdateArtifact(ctx, ws, child.ID, map[string]any{
		"parent_id": parent.ID,
		"status":    "done",
	})
	require.NoError(t, err)

	result, err := s.handleMoveItem(ctx, shipmentRequest(map[string]any{
		"id":     parent.ID,
		"status": "done",
	}))

	require.NoError(t, err)
	assert.False(t, result.IsError, "move to done when all children are done must succeed")
}
