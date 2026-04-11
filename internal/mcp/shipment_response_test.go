package mcp

// 026.012-T: Shipment response shape normalization tests.
//
// These tests verify that handleListShipments and handleGetShipment produce
// structurally identical per-shipment responses, satisfying the contract that
// callers can use the same decode path for both endpoints.

import (
	"context"
	"encoding/json"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/core"
	"github.com/backlogit/backlogit/internal/models"
)

// shipmentFromResult decodes a non-error CallToolResult into a
// map[string]any for field-level comparison.
func shipmentFromResult(t *testing.T, result *mcplib.CallToolResult) map[string]any {
	t.Helper()
	require.False(t, result.IsError, "expected success result, got error")
	require.NotEmpty(t, result.Content)
	text, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok, "expected text content, got %T", result.Content[0])
	var out map[string]any
	require.NoError(t, json.Unmarshal([]byte(text.Text), &out))
	return out
}

// shipmentsFromResult decodes a list result into []map[string]any.
func shipmentsFromResult(t *testing.T, result *mcplib.CallToolResult) []map[string]any {
	t.Helper()
	require.False(t, result.IsError, "expected success result, got error")
	require.NotEmpty(t, result.Content)
	text, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok, "expected text content, got %T", result.Content[0])
	var out []map[string]any
	require.NoError(t, json.Unmarshal([]byte(text.Text), &out))
	return out
}

// itemsField extracts custom_fields.items from a decoded shipment map.
func itemsField(t *testing.T, shipmentMap map[string]any) []string {
	t.Helper()
	cf, ok := shipmentMap["custom_fields"].(map[string]any)
	require.True(t, ok, "custom_fields must be present and be an object")
	raw, ok := cf["items"]
	require.True(t, ok, "custom_fields.items must be present")
	// JSON unmarshal always produces []interface{} for arrays.
	rawSlice, ok := raw.([]interface{})
	require.True(t, ok, "items must be a JSON array, got %T", raw)
	out := make([]string, len(rawSlice))
	for i, v := range rawSlice {
		s, ok := v.(string)
		require.True(t, ok, "each item must be a string, got %T at index %d", v, i)
		out[i] = s
	}
	return out
}

// TestListShipments_SameShapeAsGetShipment verifies that listing a shipment and
// getting it individually produce identical values for the fields that matter:
// id, title, status, artifact_type, and custom_fields.items.
func TestListShipments_SameShapeAsGetShipment(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()

	feat, err := core.CreateArtifact(ctx, ws, "Normalisation feature", "feature")
	require.NoError(t, err)
	task1, err := core.CreateArtifact(ctx, ws, "Task alpha", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	task2, err := core.CreateArtifact(ctx, ws, "Task beta", "task", core.WithParent(feat.ID))
	require.NoError(t, err)

	shipment, err := core.CreateShipment(ctx, ws, "Shape test shipment", []string{task1.ID, task2.ID})
	require.NoError(t, err)

	// Fetch via handleGetShipment.
	getResult, err := s.handleGetShipment(ctx, shipmentRequest(map[string]any{
		"id": shipment.ID,
	}))
	require.NoError(t, err)
	getMap := shipmentFromResult(t, getResult)

	// Fetch via handleListShipments.
	listResult, err := s.handleListShipments(ctx, shipmentRequest(map[string]any{}))
	require.NoError(t, err)
	listSlice := shipmentsFromResult(t, listResult)

	// Find the shipment in the list by ID.
	var listMap map[string]any
	for _, m := range listSlice {
		if m["id"] == shipment.ID {
			listMap = m
			break
		}
	}
	require.NotNil(t, listMap, "shipment %s must appear in list result", shipment.ID)

	// Compare key scalar fields.
	for _, key := range []string{"id", "title", "status", "artifact_type"} {
		assert.Equal(t, getMap[key], listMap[key],
			"field %q must be identical between get and list shapes", key)
	}

	// Compare custom_fields.items — both must be []string with same elements.
	getItems := itemsField(t, getMap)
	listItems := itemsField(t, listMap)
	assert.ElementsMatch(t, getItems, listItems,
		"custom_fields.items must contain the same item IDs in both get and list")
}

// TestListShipments_EmptyItems_NeverNull verifies that a shipment with no items
// still produces custom_fields.items = [] (never null or absent).
func TestListShipments_EmptyItems_NeverNull(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()

	_, err := core.CreateShipment(ctx, ws, "Empty shipment", nil)
	require.NoError(t, err)

	listResult, err := s.handleListShipments(ctx, shipmentRequest(map[string]any{}))
	require.NoError(t, err)
	listSlice := shipmentsFromResult(t, listResult)
	require.NotEmpty(t, listSlice)

	for _, m := range listSlice {
		cf, ok := m["custom_fields"].(map[string]any)
		require.True(t, ok, "custom_fields must be present")
		items := cf["items"]
		require.NotNil(t, items, "custom_fields.items must not be null")
		_, isSlice := items.([]interface{})
		assert.True(t, isSlice, "custom_fields.items must be an array, got %T", items)
	}
}

// TestNormalizeShipmentItems_AllCases unit-tests normalizeShipmentItems directly
// to verify all three source-type branches produce []string.
func TestNormalizeShipmentItems_AllCases(t *testing.T) {
	tests := []struct {
		name  string
		input any    // value to set in custom_fields["items"], or nil for absent key
		want  []string
	}{
		{name: "nil custom_fields map", input: nil, want: []string{}},
		{name: "[]string already typed", input: []string{"a", "b"}, want: []string{"a", "b"}},
		{name: "[]any with strings", input: []any{"x", "y"}, want: []string{"x", "y"}},
		{name: "[]any with non-string element", input: []any{"ok", 42}, want: []string{"ok"}},
		{name: "unknown type falls back", input: 123, want: []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			art := buildTestShipmentArtifact(tt.input)
			normalizeShipmentItems(art)
			got := art.CustomFields["items"]
			assert.Equal(t, tt.want, got,
				"normalizeShipmentItems produced unexpected items for input %v (type %T)", tt.input, tt.input)
		})
	}
}

// buildTestShipmentArtifact constructs a minimal models.Artifact with
// custom_fields["items"] set to raw. When raw is nil the CustomFields map
// itself is left nil to test the nil-init path.
func buildTestShipmentArtifact(raw any) *models.Artifact {
	a := &models.Artifact{ArtifactType: "shipment"}
	if raw == nil {
		// CustomFields is nil — tests the initialisation branch.
		return a
	}
	a.CustomFields = map[string]any{"items": raw}
	return a
}
