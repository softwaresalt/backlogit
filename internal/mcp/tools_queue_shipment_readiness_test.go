package mcp

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

func seedMCPArchivedPredecessor(t *testing.T, ws *core.Workspace, id, archivedStatus string) {
	t.Helper()
	dir := filepath.Join(core.WorkspaceStorageRoot(ws.RootPath), "archive")
	require.NoError(t, os.MkdirAll(dir, 0o755))
	body := "---\nid: " + id + "\nartifact_type: shipment\ntitle: Fixture " + id + "\nstatus: archived\narchived_status: " + archivedStatus + "\n---\n\nfixture body\n"
	require.NoError(t, os.WriteFile(filepath.Join(dir, id+".md"), []byte(body), 0o644))
	require.NoError(t, db.UpsertItem(context.Background(), ws.DB, &models.Artifact{ID: id, Title: "Predecessor " + id, Status: models.StatusArchived, ArtifactType: "shipment"}))
}

func seedMCPQueuedShipmentBehind(t *testing.T, ws *core.Workspace, id, predID string) {
	t.Helper()
	ctx := context.Background()
	require.NoError(t, db.UpsertItem(ctx, ws.DB, &models.Artifact{ID: id, Title: "Dependent " + id, Status: models.StatusQueued, ArtifactType: "shipment"}))
	require.NoError(t, db.UpsertDependency(ctx, ws.DB, id, predID, "blocks"))
}

// TestQueueShipmentReadiness_MCPGetQueueSplitsOnArchivedProvenance verifies
// backlogit_get_queue includes a shipment behind an archived shipped
// predecessor and excludes one behind an archived abandoned predecessor
// (A-U9 / 188.005-T).
func TestQueueShipmentReadiness_MCPGetQueueSplitsOnArchivedProvenance(t *testing.T) {
	s, ws := setupBugFixServer(t)
	seedMCPArchivedPredecessor(t, ws, "950-S", "shipped")
	seedMCPArchivedPredecessor(t, ws, "951-S", "abandoned")
	seedMCPQueuedShipmentBehind(t, ws, "960-S", "950-S")
	seedMCPQueuedShipmentBehind(t, ws, "961-S", "951-S")

	result, err := s.handleGetQueue(context.Background(), contractRequest(map[string]any{"type": "shipment", "status": "queued"}))
	require.NoError(t, err)
	text := resultTextForHarness(t, result)
	require.False(t, result.IsError, text)

	var view struct {
		Items []struct {
			ID string `json:"id"`
		} `json:"items"`
	}
	require.NoError(t, json.Unmarshal([]byte(text), &view), text)
	ids := make([]string, 0, len(view.Items))
	for _, item := range view.Items {
		ids = append(ids, item.ID)
	}
	assert.Contains(t, ids, "960-S", "shipment behind archived shipped predecessor must be included")
	assert.NotContains(t, ids, "961-S", "shipment behind archived abandoned predecessor must be excluded")
}
