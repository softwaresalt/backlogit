package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// seedCoveringArtifact upserts a minimal valid artifact into the workspace
// index so covering-feature derivation can resolve it via bldb.GetItem.
func seedCoveringArtifact(t *testing.T, ws *Workspace, id, title, artifactType string, level int) {
	t.Helper()
	ctx := context.Background()
	art := &models.Artifact{
		ID:           id,
		Title:        title,
		Status:       models.StatusActive,
		ArtifactType: artifactType,
		Level:        level,
	}
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, art))
}

// shipmentWithItems builds an in-memory (unpersisted) shipment artifact whose
// custom_fields.items references the given IDs as []any — the shape SQLite JSON
// round-trips produce (f015). Deriving the covering feature resolves those IDs
// against the index; the shipment itself is never persisted.
func shipmentWithItems(id string, itemIDs ...string) *models.Artifact {
	items := make([]any, len(itemIDs))
	for i, v := range itemIDs {
		items[i] = v
	}
	return &models.Artifact{
		ID:           id,
		Title:        "Shipment " + id,
		Status:       models.StatusActive,
		ArtifactType: "shipment",
		CustomFields: map[string]any{"items": items},
	}
}

// Unit 1 scenario 1: single-feature manifest returns that feature's ID + title.
func TestDeriveCoveringFeature_SingleFeature(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	seedCoveringArtifact(t, ws, "100-F", "Root feature", "feature", 1)

	shipment := shipmentWithItems("100-S", "100-F")
	beforeLen := len(shipment.CustomFields)

	cf, ok := DeriveCoveringFeature(ctx, ws, shipment)

	require.True(t, ok)
	assert.Equal(t, "100-F", cf.ID)
	assert.Equal(t, "Root feature", cf.Title)

	// The derivation must not mutate the input shipment nor inject derived keys
	// into the persisted-looking custom_fields map (R3 / write-path bypass guard).
	assert.Equal(t, beforeLen, len(shipment.CustomFields))
	_, injected := shipment.CustomFields["covering_feature"]
	assert.False(t, injected, "derivation must not inject covering_feature into custom_fields")
}

// Unit 1 scenario 2: manifest with a root feature and a nested (dotted-ID)
// feature returns the root covering feature, not the nested one — even when the
// nested feature is listed first.
func TestDeriveCoveringFeature_RootAndNested_ReturnsRoot(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	seedCoveringArtifact(t, ws, "100-F", "Root feature", "feature", 1)
	seedCoveringArtifact(t, ws, "100.001-F", "Nested feature", "feature", 2)

	shipment := shipmentWithItems("100-S", "100.001-F", "100-F")

	cf, ok := DeriveCoveringFeature(ctx, ws, shipment)

	require.True(t, ok)
	assert.Equal(t, "100-F", cf.ID, "must select the dotless root feature, not the nested one")
	assert.Equal(t, "Root feature", cf.Title)
}

// Unit 1 scenario 3: a tasks-only manifest yields (zero, false).
func TestDeriveCoveringFeature_ZeroFeatures(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	seedCoveringArtifact(t, ws, "200.001-T", "Task only", "task", 2)

	shipment := shipmentWithItems("200-S", "200.001-T")

	cf, ok := DeriveCoveringFeature(ctx, ws, shipment)

	assert.False(t, ok)
	assert.Equal(t, CoveringFeature{}, cf)
}

// Unit 1 scenario 4: an item that resolves to ErrNotFound is skipped defensively
// via GetItem; the valid covering feature is still resolved, and no upsert occurs
// (pure read — GetItem must not add the missing ID to the index).
func TestDeriveCoveringFeature_MissingItem_SkippedNoUpsert(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	seedCoveringArtifact(t, ws, "300-F", "Root feature", "feature", 1)

	_, err := bldb.GetItem(ctx, ws.DB, "999-MISSING")
	require.ErrorIs(t, err, blerrors.ErrNotFound, "precondition: missing ID absent from index")

	shipment := shipmentWithItems("300-S", "999-MISSING", "300-F")

	cf, ok := DeriveCoveringFeature(ctx, ws, shipment)
	require.True(t, ok)
	assert.Equal(t, "300-F", cf.ID)

	_, err = bldb.GetItem(ctx, ws.DB, "999-MISSING")
	require.ErrorIs(t, err, blerrors.ErrNotFound, "derivation must not upsert missing items (pure read)")
}

// Unit 1 scenario 5 (acceptance c): nil / empty shipment yields (zero, false)
// without panicking.
func TestDeriveCoveringFeature_NilAndEmpty(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()

	cf, ok := DeriveCoveringFeature(ctx, ws, nil)
	assert.False(t, ok)
	assert.Equal(t, CoveringFeature{}, cf)

	empty := &models.Artifact{ID: "400-S", Title: "Empty", Status: models.StatusActive, ArtifactType: "shipment"}
	cf, ok = DeriveCoveringFeature(ctx, ws, empty)
	assert.False(t, ok)
	assert.Equal(t, CoveringFeature{}, cf)
}

// Unit 1 scenario 5 (defensive): a non-nil Workspace with a nil DB handle yields
// (zero, false) without panicking. bldb.GetItem dereferences ws.DB directly, so
// the derivation must fall back to the safe branch for Workspace values
// constructed without a DB (e.g. Workspace{} in tests or future call sites).
func TestDeriveCoveringFeature_NilDB(t *testing.T) {
	ctx := context.Background()
	shipment := shipmentWithItems("410-S", "410-F")

	assert.NotPanics(t, func() {
		cf, ok := DeriveCoveringFeature(ctx, &Workspace{}, shipment)
		assert.False(t, ok)
		assert.Equal(t, CoveringFeature{}, cf)
	})
}

// NewShipmentView embeds the shipment and attaches the derived covering feature
// as a top-level pointer field, omitted when absent.
func TestNewShipmentView_ShapesEnvelope(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	seedCoveringArtifact(t, ws, "500-F", "Covered feature", "feature", 1)

	withFeature := shipmentWithItems("500-S", "500-F")
	view := NewShipmentView(ctx, ws, withFeature)
	require.NotNil(t, view.CoveringFeature)
	assert.Equal(t, "500-F", view.CoveringFeature.ID)
	assert.Equal(t, "Covered feature", view.CoveringFeature.Title)
	assert.Same(t, withFeature, view.Artifact)

	noFeature := shipmentWithItems("501-S", "501.001-T")
	view = NewShipmentView(ctx, ws, noFeature)
	assert.Nil(t, view.CoveringFeature, "covering feature pointer must be nil when absent")
}
