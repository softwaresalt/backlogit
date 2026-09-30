package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
)

func TestClaimMarkerReadSurface_MCP(t *testing.T) {
	t.Run("RED_mcp_recipe_reads_marker", func(t *testing.T) {
		server, shipmentID, memberID := setupClaimMarkerReadSurfaceMCP(t, true)
		ctx := context.Background()

		listRequest := mcplib.CallToolRequest{}
		listRequest.Params.Arguments = map[string]any{"status": "active"}
		listResult, err := server.InvokeTool(ctx, "backlogit_list_shipments", listRequest)
		require.NoError(t, err)
		listedShipments := shipmentsFromResult(t, listResult)
		require.Len(t, listedShipments, 1, "active shipment list must contain the claimed shipment")
		assert.Equal(t, shipmentID, listedShipments[0]["id"], "active shipment list must identify the claimed shipment")
		assert.Contains(t, itemsField(t, listedShipments[0]), memberID, "active shipment list must contain the claimed member")

		getRequest := mcplib.CallToolRequest{}
		getRequest.Params.Arguments = map[string]any{"id": memberID}
		itemResult, err := server.InvokeTool(ctx, "backlogit_get_item", getRequest)
		require.NoError(t, err)
		item := extractResultJSON(t, itemResult)
		itemFields, _ := item["custom_fields"].(map[string]any)
		marker, present := itemFields["scheduler_baseline_claim"]
		assert.True(t, present, "MCP get_item custom_fields.scheduler_baseline_claim must be present")
		assert.Equal(t, shipmentID, marker, "MCP get_item must expose the claiming shipment ID")
	})

	t.Run("Characterization_empty_active_list_and_organic_marker_absent", func(t *testing.T) {
		server, _, memberID := setupClaimMarkerReadSurfaceMCP(t, false)
		ctx := context.Background()

		listRequest := mcplib.CallToolRequest{}
		listRequest.Params.Arguments = map[string]any{"status": "active"}
		listResult, err := server.InvokeTool(ctx, "backlogit_list_shipments", listRequest)
		require.NoError(t, err)
		shipments := shipmentsFromResult(t, listResult)
		assert.NotNil(t, shipments, "empty active shipment list must be an array, not null")
		assert.Empty(t, shipments, "shipment list must be empty before a claim")

		getRequest := mcplib.CallToolRequest{}
		getRequest.Params.Arguments = map[string]any{"id": memberID}
		itemResult, err := server.InvokeTool(ctx, "backlogit_get_item", getRequest)
		require.NoError(t, err)
		item := extractResultJSON(t, itemResult)
		itemFields, _ := item["custom_fields"].(map[string]any)
		_, markerPresent := itemFields["scheduler_baseline_claim"]
		assert.False(t, markerPresent, "an organic-active item must not carry scheduler_baseline_claim")
	})
}

func setupClaimMarkerReadSurfaceMCP(t *testing.T, claimShipment bool) (*Server, string, string) {
	t.Helper()
	root := t.TempDir()
	storageRoot := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(storageRoot, 0o755))
	require.NoError(t, config.WriteDefaults(storageRoot))

	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() {
		require.NoError(t, ws.Close())
	})

	feature, err := core.CreateArtifact(ctx, ws, "MCP claim marker feature", "feature")
	require.NoError(t, err)
	member, err := core.CreateArtifact(ctx, ws, "MCP claim marker member", "task", core.WithParent(feature.ID))
	require.NoError(t, err)
	updates := map[string]any{"custom_fields": map[string]any{"fixture": "preserved"}}
	if !claimShipment {
		updates["status"] = "active"
	}
	_, err = core.UpdateArtifact(ctx, ws, member.ID, updates)
	require.NoError(t, err)

	shipmentID := ""
	if claimShipment {
		shipment, createErr := core.CreateShipment(ctx, ws, "MCP claim marker shipment", []string{member.ID})
		require.NoError(t, createErr)
		shipmentID = shipment.ID
		_, err = core.ClaimShipment(ctx, ws, shipmentID)
		require.NoError(t, err)
	}
	return NewServer(ws), shipmentID, member.ID
}
