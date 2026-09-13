package core

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

// TestSyncShipmentReconcileItemIndex_UpdatesRowFromArchiveContent (Copilot
// PR #440 review, finding 5) proves Phase D's SQLite index sync actually
// re-syncs the items row from the just-written archive Markdown content:
// after a governed reconciliation, the row must reflect the new title and
// custom_fields (carrying the idempotency-key resume marker) rather than
// remaining stale until the next full sync/reindex.
func TestSyncShipmentReconcileItemIndex_UpdatesRowFromArchiveContent(t *testing.T) {
	ctx := context.Background()
	ws := setupShipmentWorkspace(t)
	const shipmentID = "167-index-sync-S"

	original := &models.Artifact{
		ID:           shipmentID,
		Title:        "Original Title",
		Status:       models.StatusArchived,
		ArtifactType: "shipment",
		CustomFields: map[string]any{},
	}
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, original))

	frontmatter := map[string]any{
		"id":              shipmentID,
		"title":           "Updated Title",
		"status":          string(models.StatusArchived),
		"artifact_type":   "shipment",
		"archived_status": string(ShipmentShipped),
		"custom_fields": map[string]any{
			shipmentReconcileIdempotencyKeyField: "idem-index-sync-001",
		},
	}
	archiveContent := []byte(models.SerializeFrontmatter(frontmatter, "shipment body"))

	syncShipmentReconcileItemIndex(ctx, ws, shipmentID, archiveContent)

	var gotTitle, gotCustomFields string
	require.NoError(t, ws.DB.QueryRowContext(ctx, `SELECT title, custom_fields FROM items WHERE id = ?`, shipmentID).Scan(&gotTitle, &gotCustomFields))
	assert.Equal(t, "Updated Title", gotTitle, "the items row title must be re-synced from the new archive content")

	var customFields map[string]any
	require.NoError(t, json.Unmarshal([]byte(gotCustomFields), &customFields))
	assert.Equal(t, "idem-index-sync-001", customFields[shipmentReconcileIdempotencyKeyField],
		"the resume-marker idempotency key must be reflected in the synced custom_fields")
}

// TestSyncShipmentReconcileItemIndex_MalformedContentDoesNotPanicOrCorrupt
// proves the best-effort contract explicitly: a malformed archiveContent
// (that cannot be parsed into frontmatter/an artifact) must never panic and
// must never corrupt the existing row -- it is silently skipped (logged as
// a warning), mirroring appendShipmentReconcileEventImpl's own "index
// failure does not un-reconcile" precedent applied here to the write side.
func TestSyncShipmentReconcileItemIndex_MalformedContentDoesNotPanicOrCorrupt(t *testing.T) {
	ctx := context.Background()
	ws := setupShipmentWorkspace(t)
	const shipmentID = "167-index-sync-malformed-S"

	original := &models.Artifact{
		ID:           shipmentID,
		Title:        "Untouched Title",
		Status:       models.StatusArchived,
		ArtifactType: "shipment",
		CustomFields: map[string]any{},
	}
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, original))

	assert.NotPanics(t, func() {
		syncShipmentReconcileItemIndex(ctx, ws, shipmentID, []byte("---\nnot: [valid: yaml::\n---\nbody"))
	})

	var gotTitle string
	require.NoError(t, ws.DB.QueryRowContext(ctx, `SELECT title FROM items WHERE id = ?`, shipmentID).Scan(&gotTitle))
	assert.Equal(t, "Untouched Title", gotTitle, "a malformed archive content sync failure must leave the existing row untouched")
}
