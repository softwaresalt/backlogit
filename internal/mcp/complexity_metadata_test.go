package mcp

import (
	"context"
	"database/sql"
	"encoding/json"
	"testing"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

func TestMCPUpdateComplexity_PersistsAndClears(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	feature, err := core.CreateArtifact(ctx, ws, "MCP complexity feature", "feature")
	require.NoError(t, err)
	task, err := core.CreateArtifact(ctx, ws, "MCP complexity task", "task", core.WithParent(feature.ID))
	require.NoError(t, err)

	result, err := s.handleUpdateItem(ctx, contractRequest(map[string]any{
		"id":         task.ID,
		"complexity": "high",
	}))
	require.NoError(t, err)
	require.False(t, result.IsError, resultTextForHarness(t, result))
	data := extractResultJSON(t, result)
	cf, ok := data["custom_fields"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "high", cf["complexity"])

	clear, err := s.handleUpdateItem(ctx, contractRequest(map[string]any{
		"id":         task.ID,
		"complexity": "",
	}))
	require.NoError(t, err)
	require.False(t, clear.IsError, resultTextForHarness(t, clear))
	var projected sql.NullString
	require.NoError(t, ws.DB.QueryRowContext(ctx, `SELECT complexity FROM items WHERE id = ?`, task.ID).Scan(&projected))
	assert.False(t, projected.Valid)
}

func TestMCPUpdateComplexity_InvalidAndMixedValidation(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	feature, err := core.CreateArtifact(ctx, ws, "MCP complexity validation feature", "feature")
	require.NoError(t, err)
	task, err := core.CreateArtifact(ctx, ws, "MCP complexity validation task", "task", core.WithParent(feature.ID))
	require.NoError(t, err)

	invalid, err := s.handleUpdateItem(ctx, contractRequest(map[string]any{
		"id":         task.ID,
		"complexity": "extreme",
	}))
	require.NoError(t, err)
	assert.Equal(t, "validation_failed", contractErrorType(t, invalid))
	assert.Contains(t, resultTextForHarness(t, invalid), "trivial")

	mixedGeneric, err := s.handleUpdateItem(ctx, contractRequest(map[string]any{
		"id":         task.ID,
		"complexity": "high",
		"status":     "done",
	}))
	require.NoError(t, err)
	assert.Equal(t, "validation_failed", contractErrorType(t, mixedGeneric))

	mixedSize, err := s.handleUpdateItem(ctx, contractRequest(map[string]any{
		"id":         task.ID,
		"complexity": "high",
		"size":       "M",
	}))
	require.NoError(t, err)
	assert.Equal(t, "validation_failed", contractErrorType(t, mixedSize))
}

func TestMCPListComplexityFilter(t *testing.T) {
	s, ws := setupBugFixServer(t)
	ctx := context.Background()
	require.NoError(t, db.UpsertItem(ctx, ws.DB, mcpComplexityArtifact("962.001-T", "high")))
	require.NoError(t, db.UpsertItem(ctx, ws.DB, mcpComplexityArtifact("962.002-T", "low")))

	result, err := s.handleListItems(ctx, contractRequest(map[string]any{"complexity": "high"}))
	require.NoError(t, err)
	require.False(t, result.IsError, resultTextForHarness(t, result))
	items := extractResultJSONArray(t, result)
	require.Len(t, items, 1)
	assert.Equal(t, "962.001-T", items[0]["id"])

	invalid, err := s.handleListItems(ctx, contractRequest(map[string]any{"complexity": "bogus"}))
	require.NoError(t, err)
	assert.Equal(t, "validation_failed", contractErrorType(t, invalid))
}

func TestMCPGetWITMetadata_ExposesComplexity(t *testing.T) {
	s, _ := setupBugFixServer(t)
	ctx := context.Background()

	result, err := s.handleGetWITMetadata(ctx, contractRequest(map[string]any{"type": "task"}))
	require.NoError(t, err)
	require.False(t, result.IsError, resultTextForHarness(t, result))
	data := extractResultJSON(t, result)
	fields, ok := data["fields"].(map[string]any)
	require.True(t, ok)
	complexity, ok := fields["complexity"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "enum", complexity["type"])
	assert.Equal(t, []any{"trivial", "low", "medium", "high"}, complexity["values"])
}

func mcpComplexityArtifact(id, complexity string) *models.Artifact {
	now := time.Date(2026, 7, 30, 20, 0, 0, 0, time.UTC)
	return &models.Artifact{
		ID:           id,
		Title:        "MCP complexity list task",
		Status:       models.StatusQueued,
		ArtifactType: "task",
		Priority:     "medium",
		CustomFields: map[string]any{"complexity": complexity},
		CreatedAt:    now,
		UpdatedAt:    now,
	}
}

func extractResultJSONArray(t *testing.T, result *mcplib.CallToolResult) []map[string]any {
	t.Helper()
	var rows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(resultTextForHarness(t, result)), &rows))
	return rows
}
