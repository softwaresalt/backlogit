package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/cli"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

const claimMarkerHarnessKey = "scheduler_baseline_claim"

func TestU0b_ClaimMarkerReadSurface(t *testing.T) {
	t.Run("cli_frontmatter_projection_contains_claim_marker", func(t *testing.T) {
		root := setupCLIWorkspace(t)
		ctx := context.Background()
		ws, err := core.NewWorkspace(ctx, root)
		require.NoError(t, err)

		feature, err := core.CreateArtifact(ctx, ws, "CLI marker feature", "feature")
		require.NoError(t, err)
		require.NoError(t, db.UpsertItem(ctx, ws.DB, feature))
		item, err := core.CreateArtifact(ctx, ws, "CLI marker task", "task", core.WithParent(feature.ID))
		require.NoError(t, err)
		require.NoError(t, db.UpsertItem(ctx, ws.DB, item))
		shipment, err := core.CreateShipment(ctx, ws, "CLI marker shipment", []string{item.ID})
		require.NoError(t, err)
		_, err = core.ClaimShipment(ctx, ws, shipment.ID)
		require.NoError(t, err)
		ws.Close()

		payload := executeGetJSON(t, root, item.ID)
		customFields, _ := payload["custom_fields"].(map[string]any)
		assert.Equal(t, shipment.ID, customFields[claimMarkerHarnessKey],
			"CLI get --format json must expose custom_fields.%s with the activating shipment ID",
			claimMarkerHarnessKey)
	})

	t.Run("organic_active_item_omits_claim_marker", func(t *testing.T) {
		root := setupCLIWorkspace(t)
		ctx := context.Background()
		ws, err := core.NewWorkspace(ctx, root)
		require.NoError(t, err)

		item, err := core.CreateArtifact(ctx, ws, "Organic active feature", "feature")
		require.NoError(t, err)
		_, err = core.UpdateArtifact(ctx, ws, item.ID, map[string]any{
			"status": string(models.StatusActive),
		})
		require.NoError(t, err)
		ws.Close()

		payload := executeGetJSON(t, root, item.ID)
		customFields, _ := payload["custom_fields"].(map[string]any)
		assert.NotContains(t, customFields, claimMarkerHarnessKey,
			"organic-active items must omit custom_fields.%s", claimMarkerHarnessKey)
	})
}

func executeGetJSON(t *testing.T, root, itemID string) map[string]any {
	t.Helper()

	cmd := cli.NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "get", itemID, "--format", "json"})
	require.NoError(t, cmd.Execute())

	var payload map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &payload))
	return payload
}
