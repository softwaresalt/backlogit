package mcp

// 015.011-T: Shipment error sentinel MCP classification harness.
//
// These tests verify that shipment-related MCP tool handlers return the correct
// error category in their JSON response body:
//
//   - ErrShipmentNotFound    → {"error":"not_found",  ...}
//   - ErrShipmentConflict    → {"error":"conflict",   ...}
//   - ErrItemAlreadyAssigned → {"error":"conflict",   ...}
//   - ErrCannotReturnItem    → {"error":"conflict",   ...}
//
// The intent is to prevent regression: shipment sentinels must never fall
// through to the {"error":"internal"} category.

import (
	"context"
	"encoding/json"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
)

// setupShipmentErrorServer builds a minimal workspace and returns an
// initialised Server ready for direct unexported handler calls.
func setupShipmentErrorServer(t *testing.T) (*Server, *core.Workspace) {
	t.Helper()
	// Reuse the setup helper already defined in section_bugs_test.go.
	return setupBugFixServer(t)
}

// shipmentErrorField parses the "error" key from an MCP tool result.
// The result must be an error result and its first content item must be text.
func shipmentErrorField(t *testing.T, result *mcplib.CallToolResult) string {
	t.Helper()
	require.True(t, result.IsError, "expected error result")
	require.NotEmpty(t, result.Content, "expected at least one content item")
	text, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok, "expected text content, got %T", result.Content[0])
	var resp struct {
		Error string `json:"error"`
	}
	require.NoError(t, json.Unmarshal([]byte(text.Text), &resp),
		"result must be valid JSON: %s", text.Text)
	return resp.Error
}

// shipmentRequest builds a CallToolRequest with a string arguments map.
func shipmentRequest(args map[string]any) mcplib.CallToolRequest {
	req := mcplib.CallToolRequest{}
	req.Params.Arguments = args
	return req
}

// TestGetShipment_NotFoundReturnsNotFoundError verifies that requesting a
// non-existent shipment yields error="not_found", not error="internal".
func TestGetShipment_NotFoundReturnsNotFoundError(t *testing.T) {
	s, _ := setupShipmentErrorServer(t)
	ctx := context.Background()

	result, err := s.handleGetShipment(ctx, shipmentRequest(map[string]any{
		"id": "nonexistent-999-S",
	}))
	require.NoError(t, err)
	assert.Equal(t, "not_found", shipmentErrorField(t, result),
		"missing shipment must surface as not_found, not internal")
}

// TestAddItemToShipment_AlreadyAssignedReturnsConflict verifies that adding an
// item already in another shipment yields error="conflict".
func TestAddItemToShipment_AlreadyAssignedReturnsConflict(t *testing.T) {
	s, ws := setupShipmentErrorServer(t)
	ctx := context.Background()

	feat, err := core.CreateArtifact(ctx, ws, "Assignment feature", "feature")
	require.NoError(t, err)
	task, err := core.CreateArtifact(ctx, ws, "Already assigned task", "task", core.WithParent(feat.ID))
	require.NoError(t, err)

	_, err = core.CreateShipment(ctx, ws, "Shipment 1", []string{task.ID})
	require.NoError(t, err)

	ship2, err := core.CreateShipment(ctx, ws, "Shipment 2", nil)
	require.NoError(t, err)

	result, err := s.handleAddToShipment(ctx, shipmentRequest(map[string]any{
		"shipment_id": ship2.ID,
		"item_id":     task.ID,
	}))
	require.NoError(t, err)
	assert.Equal(t, "conflict", shipmentErrorField(t, result),
		"double-assignment must surface as conflict, not internal")
}

// TestReturnBlockedItem_NotInShipmentReturnsConflict verifies that attempting
// to return an item not in the shipment yields error="conflict".
func TestReturnBlockedItem_NotInShipmentReturnsConflict(t *testing.T) {
	s, ws := setupShipmentErrorServer(t)
	ctx := context.Background()

	feat, err := core.CreateArtifact(ctx, ws, "Return feature", "feature")
	require.NoError(t, err)
	task, err := core.CreateArtifact(ctx, ws, "Standalone task", "task", core.WithParent(feat.ID))
	require.NoError(t, err)

	shipment, err := core.CreateShipment(ctx, ws, "Empty shipment", nil)
	require.NoError(t, err)
	_, err = core.ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	result, err := s.handleReturnBlocked(ctx, shipmentRequest(map[string]any{
		"shipment_id": shipment.ID,
		"item_id":     task.ID,
		"reason":      "not in shipment",
	}))
	require.NoError(t, err)
	assert.Equal(t, "conflict", shipmentErrorField(t, result),
		"cannot-return-item must surface as conflict, not internal")
}

// TestClaimShipment_AlreadyActiveReturnsConflict verifies that claiming an
// already-active shipment yields error="conflict".
func TestClaimShipment_AlreadyActiveReturnsConflict(t *testing.T) {
	s, ws := setupShipmentErrorServer(t)
	ctx := context.Background()

	shipment, err := core.CreateShipment(ctx, ws, "Active shipment", nil)
	require.NoError(t, err)
	_, err = core.ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)

	result, err := s.handleClaimShipment(ctx, shipmentRequest(map[string]any{
		"id": shipment.ID,
	}))
	require.NoError(t, err)
	assert.Equal(t, "conflict", shipmentErrorField(t, result),
		"double-claim must surface as conflict, not internal")
}
