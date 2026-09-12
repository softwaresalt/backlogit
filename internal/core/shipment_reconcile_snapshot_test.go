package core

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

func TestShipmentReconcileSnapshot_RoundTripRestoresFullRowAndFile(t *testing.T) {
	ctx := context.Background()
	ws := setupShipmentWorkspace(t)
	const shipmentID = "167.016-roundtrip-S"
	const extensionColumn = "snapshot_restore_extension"

	addShipmentReconcileSnapshotExtensionColumn(t, ws, extensionColumn)
	insertShipmentReconcileSnapshotArtifact(t, ws, shipmentReconcileSnapshotArtifact(shipmentID))
	_, err := ws.DB.ExecContext(ctx, `UPDATE items SET snapshot_restore_extension = ? WHERE id = ?`, "original-extension", shipmentID)
	require.NoError(t, err)
	require.NoError(t, writeShipmentReconcileArchiveFile(ctx, ws, shipmentID, []byte("original archive bytes")))

	baseline, err := snapshotShipmentReconcile(ctx, ws, shipmentID)
	require.NoError(t, err)
	require.True(t, baseline.RowPresent)
	require.Equal(t, "original-extension", baseline.Row[extensionColumn])
	require.Equal(t, []byte("original archive bytes"), baseline.FileBytes)

	_, err = ws.DB.ExecContext(ctx, `UPDATE items SET title = ?, status = ?, snapshot_restore_extension = ?, labels = ? WHERE id = ?`,
		"mutated shipment title", "active", "mutated-extension", `["mutated"]`, shipmentID)
	require.NoError(t, err)
	require.NoError(t, writeShipmentReconcileArchiveFile(ctx, ws, shipmentID, []byte("mutated archive bytes")))

	require.NoError(t, restoreShipmentReconcile(ctx, ws, baseline))

	restored, err := snapshotShipmentReconcile(ctx, ws, shipmentID)
	require.NoError(t, err)
	assert.Equal(t, baseline.FileBytes, restored.FileBytes)
	assert.True(t, restored.RowPresent)
	assert.Equal(t, baseline.Row, restored.Row)

	archivePath := filepath.Join(workspaceStorageRoot(ws), shipmentReconcileArchiveDirName, shipmentID+".md")
	got, err := os.ReadFile(archivePath)
	require.NoError(t, err)
	assert.Equal(t, baseline.FileBytes, got)
}

func TestShipmentReconcileSnapshot_RestoreDeletesCreatedRowWhenSnapshotHadNoRow(t *testing.T) {
	ctx := context.Background()
	ws := setupShipmentWorkspace(t)
	const shipmentID = "167.016-absent-row-S"

	snapshot, err := snapshotShipmentReconcile(ctx, ws, shipmentID)
	require.NoError(t, err)
	assert.Nil(t, snapshot.FileBytes)
	assert.False(t, snapshot.RowPresent)
	assert.Nil(t, snapshot.Row)

	insertShipmentReconcileSnapshotArtifact(t, ws, shipmentReconcileSnapshotArtifact(shipmentID))
	assert.Equal(t, 1, shipmentReconcileSnapshotCount(t, ws, `SELECT COUNT(*) FROM items WHERE id = ?`, shipmentID))

	require.NoError(t, restoreShipmentReconcile(ctx, ws, snapshot))
	assert.Equal(t, 0, shipmentReconcileSnapshotCount(t, ws, `SELECT COUNT(*) FROM items WHERE id = ?`, shipmentID))
}

func TestShipmentReconcileSnapshot_RestoreDeleteDoesNotCascadeSatelliteRows(t *testing.T) {
	ctx := context.Background()
	ws := setupShipmentWorkspace(t)
	const shipmentID = "167.016-satellites-S"

	snapshot, err := snapshotShipmentReconcile(ctx, ws, shipmentID)
	require.NoError(t, err)
	require.False(t, snapshot.RowPresent)

	seedShipmentReconcileSnapshotSatelliteRows(t, ws, shipmentID)
	insertShipmentReconcileSnapshotArtifact(t, ws, shipmentReconcileSnapshotArtifact(shipmentID))

	assert.Equal(t, 1, shipmentReconcileSnapshotCount(t, ws, `SELECT COUNT(*) FROM items WHERE id = ?`, shipmentID))
	assert.Equal(t, 1, shipmentReconcileSnapshotCount(t, ws, `SELECT COUNT(*) FROM item_logs WHERE item_id = ?`, shipmentID))
	assert.Equal(t, 1, shipmentReconcileSnapshotCount(t, ws, `SELECT COUNT(*) FROM item_log_entries WHERE item_id = ?`, shipmentID))
	assert.Equal(t, 1, shipmentReconcileSnapshotCount(t, ws, `SELECT COUNT(*) FROM item_deps WHERE item_id = ?`, shipmentID))
	assert.Equal(t, 1, shipmentReconcileSnapshotCount(t, ws, `SELECT COUNT(*) FROM item_links WHERE source_id = ?`, shipmentID))
	assert.Equal(t, 1, shipmentReconcileSnapshotCount(t, ws, `SELECT COUNT(*) FROM stash_links WHERE item_id = ?`, shipmentID))
	assert.Equal(t, 1, shipmentReconcileSnapshotCount(t, ws, `SELECT COUNT(*) FROM commit_links WHERE item_id = ?`, shipmentID))

	require.NoError(t, restoreShipmentReconcile(ctx, ws, snapshot))

	assert.Equal(t, 0, shipmentReconcileSnapshotCount(t, ws, `SELECT COUNT(*) FROM items WHERE id = ?`, shipmentID))
	assert.Equal(t, 1, shipmentReconcileSnapshotCount(t, ws, `SELECT COUNT(*) FROM item_logs WHERE item_id = ?`, shipmentID))
	assert.Equal(t, 1, shipmentReconcileSnapshotCount(t, ws, `SELECT COUNT(*) FROM item_log_entries WHERE item_id = ?`, shipmentID))
	assert.Equal(t, 1, shipmentReconcileSnapshotCount(t, ws, `SELECT COUNT(*) FROM item_deps WHERE item_id = ?`, shipmentID))
	assert.Equal(t, 1, shipmentReconcileSnapshotCount(t, ws, `SELECT COUNT(*) FROM item_links WHERE source_id = ?`, shipmentID))
	assert.Equal(t, 1, shipmentReconcileSnapshotCount(t, ws, `SELECT COUNT(*) FROM stash_links WHERE item_id = ?`, shipmentID))
	assert.Equal(t, 1, shipmentReconcileSnapshotCount(t, ws, `SELECT COUNT(*) FROM commit_links WHERE item_id = ?`, shipmentID))
}

func TestShipmentReconcileSnapshot_RestoreReturnsIndeterminateOnPostRenameFailure(t *testing.T) {
	ctx := context.Background()
	ws := setupShipmentWorkspace(t)
	const shipmentID = "167.016-indeterminate-S"
	const extensionColumn = "snapshot_restore_indeterminate"

	addShipmentReconcileSnapshotExtensionColumn(t, ws, extensionColumn)
	insertShipmentReconcileSnapshotArtifact(t, ws, shipmentReconcileSnapshotArtifact(shipmentID))
	_, err := ws.DB.ExecContext(ctx, `UPDATE items SET snapshot_restore_indeterminate = ? WHERE id = ?`, "baseline-extension", shipmentID)
	require.NoError(t, err)
	require.NoError(t, writeShipmentReconcileArchiveFile(ctx, ws, shipmentID, []byte("baseline archive")))

	snapshot, err := snapshotShipmentReconcile(ctx, ws, shipmentID)
	require.NoError(t, err)

	_, err = ws.DB.ExecContext(ctx, `UPDATE items SET title = ?, snapshot_restore_indeterminate = ? WHERE id = ?`, "mutated", "mutated-extension", shipmentID)
	require.NoError(t, err)
	require.NoError(t, writeShipmentReconcileArchiveFile(ctx, ws, shipmentID, []byte("mutated archive")))

	err = restoreShipmentReconcileWithSeams(ctx, ws, snapshot, shipmentReconcileFSSeams{
		dirSyncEnabled: true,
		syncFile:       func(f *os.File) error { return f.Sync() },
		syncDir:        func(*os.File) error { return errors.New("simulated directory fsync failure") },
	})
	require.Error(t, err)
	assert.True(t, blerrors.IsWriteIndeterminate(err), "post-rename writer failures must surface ErrWriteIndeterminate, got: %v", err)

	restored, snapshotErr := snapshotShipmentReconcile(ctx, ws, shipmentID)
	require.NoError(t, snapshotErr)
	assert.Equal(t, snapshot.FileBytes, restored.FileBytes)
	assert.True(t, restored.RowPresent)
	assert.Equal(t, snapshot.Row, restored.Row)
}

func TestShipmentReconcileSnapshot_SnapshotAllowsMissingArchiveFile(t *testing.T) {
	ctx := context.Background()
	ws := setupShipmentWorkspace(t)
	const shipmentID = "167.016-missing-file-S"

	insertShipmentReconcileSnapshotArtifact(t, ws, shipmentReconcileSnapshotArtifact(shipmentID))

	snapshot, err := snapshotShipmentReconcile(ctx, ws, shipmentID)
	require.NoError(t, err)
	assert.Nil(t, snapshot.FileBytes, "missing archive files should snapshot as nil bytes, not an error")
	assert.True(t, snapshot.RowPresent)
	require.NotNil(t, snapshot.Row)
	assert.Equal(t, shipmentID, snapshot.Row["id"])
}

func shipmentReconcileSnapshotArtifact(id string) *models.Artifact {
	createdAt := time.Date(2026, 9, 12, 12, 0, 0, 123456000, time.UTC)
	updatedAt := createdAt.Add(2 * time.Minute)
	return &models.Artifact{
		ID:            id,
		Title:         "Shipment reconcile snapshot artifact",
		Status:        models.StatusActive,
		ArtifactType:  "shipment",
		Priority:      "high",
		Description:   "167.016-T snapshot test artifact",
		CreatedAt:     createdAt,
		UpdatedAt:     updatedAt,
		AssignedTo:    "copilot",
		Owner:         "ship",
		Labels:        []string{"snapshot", "rollback"},
		Dependencies:  []models.DependencyEdge{{ID: "167-F", Type: "blocks"}},
		References:    []string{"docs/design-docs/governed-operation-parity.md"},
		Commit:        "abc123def",
		Level:         1,
		HierarchyPath: "167",
		CustomFields: map[string]any{
			"reconcile_marker": "baseline",
		},
	}
}

func insertShipmentReconcileSnapshotArtifact(t *testing.T, ws *Workspace, artifact *models.Artifact) {
	t.Helper()
	require.NoError(t, bldb.UpsertItem(context.Background(), ws.DB, artifact))
}

func addShipmentReconcileSnapshotExtensionColumn(t *testing.T, ws *Workspace, column string) {
	t.Helper()
	_, err := ws.DB.ExecContext(context.Background(), `ALTER TABLE items ADD COLUMN `+column+` TEXT`)
	require.NoError(t, err)
}

func shipmentReconcileSnapshotCount(t *testing.T, ws *Workspace, query string, args ...any) int {
	t.Helper()
	var count int
	require.NoError(t, ws.DB.QueryRowContext(context.Background(), query, args...).Scan(&count))
	return count
}

func seedShipmentReconcileSnapshotSatelliteRows(t *testing.T, ws *Workspace, shipmentID string) {
	t.Helper()
	ctx := context.Background()
	const otherID = "167.016-related-S"
	linkedAt := time.Date(2026, 9, 12, 12, 30, 0, 0, time.UTC).Format(time.RFC3339Nano)
	updatedAt := time.Date(2026, 9, 12, 12, 31, 0, 0, time.UTC).Format(time.RFC3339Nano)
	timestamp := time.Date(2026, 9, 12, 12, 32, 0, 0, time.UTC).Format(time.RFC3339Nano)

	_, err := ws.DB.ExecContext(ctx, `INSERT INTO item_logs (item_id, log_path, updated_at) VALUES (?, ?, ?)`, shipmentID, "logs/"+shipmentID+".jsonl", updatedAt)
	require.NoError(t, err)
	_, err = ws.DB.ExecContext(ctx, `INSERT INTO item_log_entries (item_id, log_path, timestamp, actor, event_type, content, delta_json) VALUES (?, ?, ?, ?, ?, ?, ?)`, shipmentID, "logs/"+shipmentID+".jsonl", timestamp, "copilot", "shipment_reconciled", "content", `{}`)
	require.NoError(t, err)
	_, err = ws.DB.ExecContext(ctx, `INSERT INTO item_deps (item_id, depends_on, dep_type) VALUES (?, ?, ?)`, shipmentID, otherID, "blocks")
	require.NoError(t, err)
	_, err = ws.DB.ExecContext(ctx, `INSERT INTO item_links (source_id, target_id, link_type) VALUES (?, ?, ?)`, shipmentID, otherID, "informs")
	require.NoError(t, err)
	_, err = ws.DB.ExecContext(ctx, `INSERT INTO stash_links (stash_id, item_id, linked_at) VALUES (?, ?, ?)`, "STASH-167016", shipmentID, linkedAt)
	require.NoError(t, err)
	_, err = ws.DB.ExecContext(ctx, `INSERT INTO commit_links (item_id, commit_sha, message, author) VALUES (?, ?, ?, ?)`, shipmentID, "deadbeef", "feat: preserve satellites", "copilot")
	require.NoError(t, err)
}
