package mcp

import (
	"context"
	"encoding/json"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
)

func u3Body(t *testing.T, result *mcplib.CallToolResult) map[string]any {
	t.Helper()
	require.False(t, result.IsError, "unexpected error result: %+v", result.Content)
	text, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(text.Text), &body))
	return body
}

func TestU3_ManifestHandlers_RequiredArgumentValidation(t *testing.T) {
	s, _ := setupShipmentErrorServer(t)
	ctx := context.Background()

	tests := []struct {
		name    string
		handler func(context.Context, mcplib.CallToolRequest) (*mcplib.CallToolResult, error)
		args    map[string]any
	}{
		{"remove missing shipment_id", s.handleRemoveFromShipment, map[string]any{"item_id": "1-T"}},
		{"remove missing item_id", s.handleRemoveFromShipment, map[string]any{"shipment_id": "1-S"}},
		{"move missing item_id", s.handleMoveShipmentItem,
			map[string]any{"from_shipment_id": "1-S", "to_shipment_id": "2-S"}},
		{"move missing from", s.handleMoveShipmentItem,
			map[string]any{"item_id": "1-T", "to_shipment_id": "2-S"}},
		{"move missing to", s.handleMoveShipmentItem,
			map[string]any{"item_id": "1-T", "from_shipment_id": "1-S"}},
		{"reorder missing shipment_id", s.handleReorderShipment, map[string]any{"auto": true}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tc.handler(ctx, shipmentRequest(tc.args))
			require.NoError(t, err)
			assert.Equal(t, "validation_failed", shipmentErrorField(t, result))
		})
	}
}

func TestU3_ManifestHandlers_RoundTrip(t *testing.T) {
	s, ws := setupShipmentErrorServer(t)
	ctx := context.Background()

	feat, err := core.CreateArtifact(ctx, ws, "U3 feature", "feature")
	require.NoError(t, err)
	t1, err := core.CreateArtifact(ctx, ws, "U3 task 1", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	t2, err := core.CreateArtifact(ctx, ws, "U3 task 2", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	shipA, err := core.CreateShipment(ctx, ws, "U3 A", []string{feat.ID, t1.ID, t2.ID})
	require.NoError(t, err)
	shipB, err := core.CreateShipment(ctx, ws, "U3 B", nil)
	require.NoError(t, err)

	// Reorder dry-run leaves the manifest untouched.
	result, err := s.handleReorderShipment(ctx, shipmentRequest(map[string]any{
		"shipment_id": shipA.ID, "auto": true, "dry_run": true,
	}))
	require.NoError(t, err)
	assert.Equal(t, true, u3Body(t, result)["dry_run"])

	// Move live.
	result, err = s.handleMoveShipmentItem(ctx, shipmentRequest(map[string]any{
		"item_id": t2.ID, "from_shipment_id": shipA.ID, "to_shipment_id": shipB.ID,
		"reason": "split scope",
	}))
	require.NoError(t, err)
	body := u3Body(t, result)
	assert.Equal(t, t2.ID, body["item_id"])
	moved, err := core.GetShipment(ctx, ws, shipB.ID)
	require.NoError(t, err)
	assert.Contains(t, core.NormalizeShipmentItems(moved), t2.ID)

	// Remove live.
	result, err = s.handleRemoveFromShipment(ctx, shipmentRequest(map[string]any{
		"shipment_id": shipB.ID, "item_id": t2.ID, "reason": "descope",
	}))
	require.NoError(t, err)
	u3Body(t, result)
	after, err := core.GetShipment(ctx, ws, shipB.ID)
	require.NoError(t, err)
	assert.NotContains(t, core.NormalizeShipmentItems(after), t2.ID)

	// Force without a reason is refused.
	result, err = s.handleRemoveFromShipment(ctx, shipmentRequest(map[string]any{
		"shipment_id": shipA.ID, "item_id": t1.ID, "force": true,
	}))
	require.NoError(t, err)
	assert.True(t, result.IsError)
}
