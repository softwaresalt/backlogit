package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/models"
)

func TestDiagnosticToolsRemainAvailableWithPoisonedLifecycleJournal(t *testing.T) {
	root, shipmentID, snapshotRef := newPoisonedNormalizationWorkspace(t)
	server := NewServerForRoot(root)

	t.Run("doctor inspects poison", func(t *testing.T) {
		request := mcplib.CallToolRequest{}
		request.Params.Arguments = map[string]any{
			"check_orphans":    false,
			"check_duplicates": false,
		}
		result, err := server.InvokeTool(context.Background(), "backlogit_doctor", request)
		require.NoError(t, err)
		require.False(t, result.IsError)
		text, ok := result.Content[0].(mcplib.TextContent)
		require.True(t, ok)
		assert.Contains(t, text.Text, "invalid_shipment_lifecycle_journal")
	})

	t.Run("normalizer repairs target without weakening ordinary initialization", func(t *testing.T) {
		request := mcplib.CallToolRequest{}
		request.Params.Arguments = map[string]any{
			"id":           shipmentID,
			"snapshot_ref": snapshotRef,
			"by":           "diagnostic-test",
		}
		result, err := server.InvokeTool(context.Background(), "backlogit_normalize_blocked_shipment", request)
		require.NoError(t, err)
		require.False(t, result.IsError)

		ordinary, ordinaryErr := core.NewWorkspace(context.Background(), root)
		require.Error(t, ordinaryErr, "the unrelated poison journal must remain fail-closed for ordinary operations")
		require.Nil(t, ordinary)
	})
}

func newPoisonedNormalizationWorkspace(t *testing.T) (string, string, string) {
	t.Helper()

	ctx := context.Background()
	root := t.TempDir()
	storageRoot := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(storageRoot, 0o755))
	require.NoError(t, config.WriteDefaults(storageRoot))

	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	feature, err := core.CreateArtifact(ctx, ws, "Diagnostic feature", "feature")
	require.NoError(t, err)
	member, err := core.CreateArtifact(
		ctx,
		ws,
		"Diagnostic member",
		"task",
		core.WithParent(feature.ID),
		core.WithStatus(string(models.StatusActive)),
	)
	require.NoError(t, err)
	shipment, err := core.CreateShipment(ctx, ws, "Diagnostic shipment", []string{member.ID})
	require.NoError(t, err)
	_, err = core.ClaimShipment(ctx, ws, shipment.ID)
	require.NoError(t, err)
	_, err = core.BlockShipment(ctx, ws, shipment.ID, core.BlockOptions{
		Reason:    "repair me",
		BlockedBy: "diagnostic-test",
	})
	require.NoError(t, err)

	snapshotRef := filepath.ToSlash(filepath.Join("snapshots", "blocked.json"))
	snapshotPath := filepath.Join(storageRoot, filepath.FromSlash(snapshotRef))
	require.NoError(t, os.MkdirAll(filepath.Dir(snapshotPath), 0o755))
	snapshot := core.ShipmentBlockedSnapshot{
		SchemaVersion: core.ShipmentBlockedSnapshotSchemaVersion,
		ShipmentID:    shipment.ID,
		Target:        core.ShipmentBlocked,
		BlockedReason: "repair me",
		BlockedAt:     time.Now().UTC().Format(time.RFC3339),
		BlockedBy:     "diagnostic-test",
		Members:       map[string]string{member.ID: string(models.StatusActive)},
	}
	payload, err := json.Marshal(snapshot)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(snapshotPath, payload, 0o644))
	require.NoError(t, ws.Close())

	poisonPath := filepath.Join(storageRoot, "ops", "unrelated-poison.json")
	require.NoError(t, os.WriteFile(poisonPath, []byte(`{"schema_version":`), 0o644))
	ordinary, ordinaryErr := core.NewWorkspace(ctx, root)
	require.Error(t, ordinaryErr)
	require.Nil(t, ordinary)

	return root, shipment.ID, snapshotRef
}
