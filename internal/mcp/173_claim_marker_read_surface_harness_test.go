package mcp

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

const claimMarkerHarnessKey = "scheduler_baseline_claim"

func TestU0b_ClaimMarkerReadSurface(t *testing.T) {
	t.Run("mcp_db_projection_contains_identical_claim_marker", func(t *testing.T) {
		server, ws := setupBugFixServer(t)
		ctx := context.Background()

		feature, err := core.CreateArtifact(ctx, ws, "MCP marker feature", "feature")
		require.NoError(t, err)
		require.NoError(t, db.UpsertItem(ctx, ws.DB, feature))
		item, err := core.CreateArtifact(ctx, ws, "MCP marker task", "task", core.WithParent(feature.ID))
		require.NoError(t, err)
		require.NoError(t, db.UpsertItem(ctx, ws.DB, item))
		shipment, err := core.CreateShipment(ctx, ws, "MCP marker shipment", []string{item.ID})
		require.NoError(t, err)
		_, err = core.ClaimShipment(ctx, ws, shipment.ID)
		require.NoError(t, err)

		result, err := server.handleGetItem(ctx, contractRequest(map[string]any{"id": item.ID}))
		require.NoError(t, err)
		require.False(t, result.IsError)
		payload := extractResultJSON(t, result)
		customFields, _ := payload["custom_fields"].(map[string]any)
		assert.Equal(t, shipment.ID, customFields[claimMarkerHarnessKey],
			"MCP get_item must expose custom_fields.%s with the activating shipment ID",
			claimMarkerHarnessKey)
	})

	t.Run("consumer_ignoring_custom_fields_is_unchanged", func(t *testing.T) {
		server, ws := setupBugFixServer(t)
		ctx := context.Background()

		item, err := core.CreateArtifact(ctx, ws, "MCP ignored marker feature", "feature")
		require.NoError(t, err)
		item.Status = models.StatusActive
		require.NoError(t, db.UpsertItem(ctx, ws.DB, item))

		result, err := server.handleGetItem(ctx, contractRequest(map[string]any{"id": item.ID}))
		require.NoError(t, err)
		require.False(t, result.IsError)
		payload := extractResultJSON(t, result)
		raw, err := json.Marshal(payload)
		require.NoError(t, err)

		var consumer struct {
			ID     string                `json:"id"`
			Status models.ArtifactStatus `json:"status"`
		}
		require.NoError(t, json.Unmarshal(raw, &consumer))
		assert.Equal(t, item.ID, consumer.ID)
		assert.Equal(t, models.StatusActive, consumer.Status)
	})
}
