package mcp

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	bldb "github.com/softwaresalt/backlogit/internal/db"
)

const u3DryRunJournalName = "shipment-operation-0123456789abcdef0123456789abcdef.json"

func u3WritePendingJournal(t *testing.T, ws *core.Workspace, operation, shipmentID, target, related string) string {
	t.Helper()
	ctx := context.Background()
	shipment, err := core.GetShipment(ctx, ws, shipmentID)
	require.NoError(t, err)
	members := make([]any, 0)
	for _, memberID := range core.NormalizeShipmentItems(shipment) {
		member, memberErr := bldb.GetItem(ctx, ws.DB, memberID)
		require.NoError(t, memberErr)
		members = append(members, member)
	}
	preimage := map[string]any{"shipment": shipment, "members": members}
	if related != "" {
		other, relErr := core.GetShipment(ctx, ws, related)
		require.NoError(t, relErr)
		preimage["related"] = []any{other}
	}
	data, err := json.Marshal(map[string]any{
		"schema_version":  "shipment-operation/v1",
		"correlation_id":  "0123456789abcdef0123456789abcdef",
		"phase":           "intent",
		"operation":       operation,
		"recovery_policy": "rollback",
		"shipment_id":     shipmentID,
		"target":          target,
		"preimage":        preimage,
	})
	require.NoError(t, err)
	opsDir := filepath.Join(ws.StorageRoot, "ops")
	require.NoError(t, os.MkdirAll(opsDir, 0o755))
	path := filepath.Join(opsDir, u3DryRunJournalName)
	require.NoError(t, os.WriteFile(path, data, 0o644))
	return path
}

func u3SnapshotStorage(t *testing.T, storageRoot string) map[string]string {
	t.Helper()
	snap := map[string]string{}
	require.NoError(t, filepath.WalkDir(storageRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(storageRoot, path)
		if relErr != nil {
			return relErr
		}
		if d.IsDir() {
			if rel == "logs" || rel == ".locks" {
				return fs.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(path, ".md") || strings.HasPrefix(rel, "ops"+string(filepath.Separator)) {
			data, readErr := os.ReadFile(path)
			if readErr != nil {
				return readErr
			}
			snap[rel] = string(data)
		}
		return nil
	}))
	return snap
}

func TestU3_ManifestHandlers_DryRunPendingJournalSurvivesLazyServer(t *testing.T) {
	ctx := context.Background()
	for _, name := range []string{"remove", "move", "reorder"} {
		t.Run(name, func(t *testing.T) {
			_, ws := setupShipmentErrorServer(t)
			feat, err := core.CreateArtifact(ctx, ws, "U3 feature", "feature")
			require.NoError(t, err)
			task, err := core.CreateArtifact(ctx, ws, "U3 task", "task", core.WithParent(feat.ID))
			require.NoError(t, err)
			shipA, err := core.CreateShipment(ctx, ws, "U3 A", []string{feat.ID, task.ID})
			require.NoError(t, err)
			shipB, err := core.CreateShipment(ctx, ws, "U3 B", nil)
			require.NoError(t, err)

			// Lazy server: no workspace is initialized until a handler needs one.
			lazy := NewServerForRoot(ws.RootPath)
			require.Nil(t, lazy.Workspace)

			var path string
			var call func() (*mcplib.CallToolResult, error)
			switch name {
			case "remove":
				path = u3WritePendingJournal(t, ws, "remove_item", shipA.ID, task.ID, "")
				call = func() (*mcplib.CallToolResult, error) {
					return lazy.handleRemoveFromShipment(ctx, shipmentRequest(map[string]any{
						"shipment_id": shipA.ID, "item_id": task.ID, "dry_run": true}))
				}
			case "move":
				path = u3WritePendingJournal(t, ws, "move_item", shipA.ID, shipB.ID, shipB.ID)
				call = func() (*mcplib.CallToolResult, error) {
					return lazy.handleMoveShipmentItem(ctx, shipmentRequest(map[string]any{
						"item_id": task.ID, "from_shipment_id": shipA.ID, "to_shipment_id": shipB.ID,
						"dry_run": true}))
				}
			default:
				path = u3WritePendingJournal(t, ws, "reorder_items", shipA.ID, "reorder", "")
				call = func() (*mcplib.CallToolResult, error) {
					return lazy.handleReorderShipment(ctx, shipmentRequest(map[string]any{
						"shipment_id": shipA.ID, "auto": true, "dry_run": true}))
				}
			}
			before := u3SnapshotStorage(t, ws.StorageRoot)

			result, err := call()
			require.NoError(t, err)
			assert.Equal(t, "conflict", shipmentErrorField(t, result))
			text, ok := result.Content[0].(mcplib.TextContent)
			require.True(t, ok)
			assert.Contains(t, text.Text, "unfinished shipment recovery journal")

			assert.Nil(t, lazy.Workspace, "dry-run must not initialize (and recover) the server workspace")
			require.FileExists(t, path, "dry-run must not recover or delete the journal")
			assert.Equal(t, before, u3SnapshotStorage(t, ws.StorageRoot),
				"dry-run must leave artifacts and journals untouched")
		})
	}
}

func TestU3_ManifestWorkspace_DryRunConcurrentFirstUse(t *testing.T) {
	ctx := context.Background()
	_, ws := setupShipmentErrorServer(t)
	lazy := NewServerForRoot(ws.RootPath)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			got, release, result := lazy.manifestWorkspace(ctx, true)
			defer release()
			assert.Nil(t, result)
			assert.NotNil(t, got)
		}()
		go func() {
			defer wg.Done()
			got, err := lazy.ensureWorkspace(ctx)
			assert.NoError(t, err)
			assert.NotNil(t, got)
		}()
	}
	wg.Wait()
	require.NotNil(t, lazy.Workspace)
	require.NoError(t, lazy.Workspace.Close())
}
