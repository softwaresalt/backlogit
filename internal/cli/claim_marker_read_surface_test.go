package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	backlogitcli "github.com/softwaresalt/backlogit/internal/cli"
	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClaimMarkerReadSurface_CLI(t *testing.T) {
	t.Run("RED_cli_recipe_reads_marker", func(t *testing.T) {
		root, shipmentID, memberID := setupClaimMarkerReadSurfaceCLI(t, true)

		listOutput := runClaimMarkerCLI(t, root, "shipment", "list", "--status", "active", "--format", "json")
		var shipments []map[string]any
		require.NoError(t, json.Unmarshal([]byte(listOutput), &shipments), "shipment list must return JSON")
		require.Len(t, shipments, 1, "active shipment list must contain the claimed shipment")
		assert.Equal(t, shipmentID, shipments[0]["id"], "active shipment list must identify the claimed shipment")

		shipmentFields, ok := shipments[0]["custom_fields"].(map[string]any)
		require.True(t, ok, "active shipment list must expose custom_fields")
		items, ok := shipmentFields["items"].([]any)
		require.True(t, ok, "active shipment list custom_fields.items must be an array")
		assert.Contains(t, items, memberID, "active shipment list must contain the claimed member")

		itemOutput := runClaimMarkerCLI(t, root, "get", memberID, "--format", "json")
		var item map[string]any
		require.NoError(t, json.Unmarshal([]byte(itemOutput), &item), "item get must return JSON")
		itemFields, _ := item["custom_fields"].(map[string]any)
		marker, present := itemFields["scheduler_baseline_claim"]
		assert.True(t, present, "CLI get custom_fields.scheduler_baseline_claim must be present")
		assert.Equal(t, shipmentID, marker, "CLI get must expose the claiming shipment ID")
	})

	t.Run("Characterization_empty_active_list_and_organic_marker_absent", func(t *testing.T) {
		root, _, memberID := setupClaimMarkerReadSurfaceCLI(t, false)

		listOutput := runClaimMarkerCLI(t, root, "shipment", "list", "--status", "active", "--format", "json")
		var shipments []map[string]any
		require.NoError(t, json.Unmarshal([]byte(listOutput), &shipments), "empty shipment list must return JSON")
		assert.NotNil(t, shipments, "empty active shipment list must be an array, not null")
		assert.Empty(t, shipments, "shipment list must be empty before a claim")

		itemOutput := runClaimMarkerCLI(t, root, "get", memberID, "--format", "json")
		var item map[string]any
		require.NoError(t, json.Unmarshal([]byte(itemOutput), &item), "organic item get must return JSON")
		itemFields, ok := item["custom_fields"].(map[string]any)
		require.True(t, ok, "organic item get must expose custom_fields")
		fixture, fixturePresent := itemFields["fixture"]
		require.True(t, fixturePresent, "organic item get must expose the fixture custom field")
		assert.Equal(t, "preserved", fixture, "organic item get must preserve the fixture custom field")
		_, markerPresent := itemFields["scheduler_baseline_claim"]
		assert.False(t, markerPresent, "an organic-active item must not carry scheduler_baseline_claim")
	})
}

func setupClaimMarkerReadSurfaceCLI(t *testing.T, claimShipment bool) (string, string, string) {
	t.Helper()
	root := t.TempDir()
	storageRoot := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(storageRoot, 0o755))
	require.NoError(t, config.WriteDefaults(storageRoot))

	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	workspaceClosed := false
	t.Cleanup(func() {
		if !workspaceClosed {
			require.NoError(t, ws.Close())
		}
	})

	feature, err := core.CreateArtifact(ctx, ws, "CLI claim marker feature", "feature")
	require.NoError(t, err)
	member, err := core.CreateArtifact(ctx, ws, "CLI claim marker member", "task", core.WithParent(feature.ID))
	require.NoError(t, err)
	updates := map[string]any{"custom_fields": map[string]any{"fixture": "preserved"}}
	if !claimShipment {
		updates["status"] = "active"
	}
	_, err = core.UpdateArtifact(ctx, ws, member.ID, updates)
	require.NoError(t, err)

	shipmentID := ""
	if claimShipment {
		shipment, createErr := core.CreateShipment(ctx, ws, "CLI claim marker shipment", []string{member.ID})
		require.NoError(t, createErr)
		shipmentID = shipment.ID
		_, err = core.ClaimShipment(ctx, ws, shipmentID)
		require.NoError(t, err)
	}

	require.NoError(t, ws.Close())
	workspaceClosed = true
	return root, shipmentID, member.ID
}

func runClaimMarkerCLI(t *testing.T, root string, args ...string) string {
	t.Helper()
	output := new(bytes.Buffer)
	errorsOutput := new(bytes.Buffer)
	cmd := backlogitcli.NewRootCommand()
	cmd.SetOut(output)
	cmd.SetErr(errorsOutput)
	cmd.SetArgs(append([]string{"--cwd", root}, args...))
	require.NoError(t, cmd.Execute(), "CLI command failed: %s", errorsOutput.String())
	return output.String()
}
