package mcp

import (
	"context"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

func requireNoSizingTODOMCP(t *testing.T, resultText string) {
	t.Helper()
	if strings.Contains(resultText, "TODO: implement 108-F size estimation") {
		t.Fatalf("%s", resultText)
	}
}

func resultTextForHarness(t *testing.T, result *mcplib.CallToolResult) string {
	t.Helper()
	require.NotNil(t, result)
	require.NotEmpty(t, result.Content)
	text, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok, "expected text content, got %T", result.Content[0])
	return text.Text
}

func TestSE5MCPUpdateSizeProvenanceFieldsHarness(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	feature, err := core.CreateArtifact(ctx, ws, "MCP size feature", "feature")
	require.NoError(t, err)
	task, err := core.CreateArtifact(ctx, ws, "MCP size task", "task", core.WithParent(feature.ID))
	require.NoError(t, err)

	result, err := s.handleUpdateItem(ctx, contractRequest(map[string]any{
		"id":                   task.ID,
		"size":                 "M",
		"size_source":          "agent",
		"size_ruleset_version": "ruleset-alpha",
	}))
	require.NoError(t, err)
	text := resultTextForHarness(t, result)
	requireNoSizingTODOMCP(t, text)
	require.False(t, result.IsError, text)
}

func TestSE5MCPRejectsHumanMasqueradeHarness(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	feature, err := core.CreateArtifact(ctx, ws, "MCP mask feature", "feature")
	require.NoError(t, err)
	task, err := core.CreateArtifact(ctx, ws, "MCP mask task", "task", core.WithParent(feature.ID))
	require.NoError(t, err)

	result, err := s.handleUpdateItem(ctx, contractRequest(map[string]any{
		"id":          task.ID,
		"size":        "S",
		"size_source": "human",
	}))
	require.NoError(t, err)
	text := resultTextForHarness(t, result)
	requireNoSizingTODOMCP(t, text)
	if result.IsError {
		assert.Equal(t, "validation_failed", contractErrorType(t, result))
		return
	}
	data := extractResultJSON(t, result)
	customFields, ok := data["custom_fields"].(map[string]any)
	require.True(t, ok)
	assert.NotEqual(t, "human", customFields["size_source"])
}

func TestSE6MCPReadProjectionSizeCompositionHarness(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	feature := &models.Artifact{
		ID:           "960-F",
		Title:        "Read projection feature",
		Status:       models.StatusActive,
		ArtifactType: "feature",
		CustomFields: map[string]any{
			"size":                 "L",
			"size_source":          "agent",
			"size_ruleset_version": "ruleset-alpha",
		},
	}
	require.NoError(t, db.UpsertItem(ctx, ws.DB, feature))

	result, err := s.handleGetItem(ctx, contractRequest(map[string]any{"id": feature.ID}))
	require.NoError(t, err)
	text := resultTextForHarness(t, result)
	requireNoSizingTODOMCP(t, text)
	require.False(t, result.IsError, text)
	data := extractResultJSON(t, result)
	customFields, ok := data["custom_fields"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "L", customFields["size"])
	assert.Contains(t, data, "size_composition")

	// SE-6: get_shipment must also expose the never-persisted composition rollup.
	shipment := &models.Artifact{
		ID:           "961-S",
		Title:        "Read projection shipment",
		Status:       models.StatusActive,
		ArtifactType: "shipment",
		CustomFields: map[string]any{
			"items": []any{feature.ID},
		},
	}
	require.NoError(t, db.UpsertItem(ctx, ws.DB, shipment))
	shipResult, err := s.handleGetShipment(ctx, contractRequest(map[string]any{"id": shipment.ID}))
	require.NoError(t, err)
	shipText := resultTextForHarness(t, shipResult)
	requireNoSizingTODOMCP(t, shipText)
	require.False(t, shipResult.IsError, shipText)
	shipData := extractResultJSON(t, shipResult)
	assert.Contains(t, shipData, "size_composition")

	// SE-6: get_queue must project the composition onto feature/shipment items.
	queueResult, err := s.handleGetQueue(ctx, contractRequest(map[string]any{"type": "feature"}))
	require.NoError(t, err)
	queueText := resultTextForHarness(t, queueResult)
	requireNoSizingTODOMCP(t, queueText)
	require.False(t, queueResult.IsError, queueText)
	queueData := extractResultJSON(t, queueResult)
	queueItems, ok := queueData["items"].([]any)
	require.True(t, ok, queueText)
	require.NotEmpty(t, queueItems)
	foundFeature := false
	for _, it := range queueItems {
		im, ok := it.(map[string]any)
		if !ok {
			continue
		}
		if im["id"] == feature.ID {
			assert.Contains(t, im, "size_composition")
			foundFeature = true
		}
	}
	assert.True(t, foundFeature, "seeded feature not present in queue items: %s", queueText)
}
