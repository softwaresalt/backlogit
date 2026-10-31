package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	bldb "github.com/softwaresalt/backlogit/internal/db"
)

func TestU3_ReconstituteDryRun_ColdServerUsesReadOnlyIndex(t *testing.T) {
	ctx := context.Background()
	_, ws := setupShipmentErrorServer(t)
	feat, err := core.CreateArtifact(ctx, ws, "Cold feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feat))
	task, err := core.CreateArtifact(ctx, ws, "Cold task", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))
	source, err := core.CreateShipment(ctx, ws, "Cold source", []string{task.ID})
	require.NoError(t, err)

	// Cold server: no workspace has been initialized, so the preview must open
	// its own non-recovering workspace that can still read the index.
	cold := NewServerForRoot(ws.RootPath)
	require.Nil(t, cold.Workspace)
	before := u3SnapshotStorage(t, ws.StorageRoot)

	plan := fmt.Sprintf(`source_shipments: [%s]
shipments:
  - id: %s
    label: only
    feature: {create: {title: Cold covering feature}}
    items: [%s]
`, source.ID, source.ID, task.ID)
	result, err := cold.handleReconstituteShipment(ctx, shipmentRequest(map[string]any{
		"plan_yaml": plan, "dry_run": true}))
	require.NoError(t, err)
	require.False(t, result.IsError, "cold-server preview must not fail: %+v", result.Content)
	text, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	var preview core.ReconstituteShipmentResult
	require.NoError(t, json.Unmarshal([]byte(text.Text), &preview))
	assert.True(t, preview.DryRun)
	require.Len(t, preview.CreatedFeatures, 1)
	assert.NotEmpty(t, preview.CreatedFeatures[0].ID, "created feature ID must be predicted from the index")

	assert.Nil(t, cold.Workspace, "dry-run must not initialize the server workspace")
	assert.Equal(t, before, u3SnapshotStorage(t, ws.StorageRoot), "dry-run must not write")
}
