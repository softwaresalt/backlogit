package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

// TestSizeCompositionResolvesFeatureMembersFromIndex proves the feature rollup
// resolves members from the SQLite index rather than a per-member filesystem
// WalkDir (114-F / 47ED88ED). A member present only in the index (no Markdown
// file on disk) must still be counted; the previous filesystem resolver would
// have warn-skipped it.
func TestSizeCompositionResolvesFeatureMembersFromIndex(t *testing.T) {
	ws, _ := newSizeEstimationHarnessWorkspace(t)
	ctx := context.Background()
	feature := &models.Artifact{
		ID: "970-F", Title: "Index-only feature", Status: models.StatusActive, ArtifactType: "feature",
	}
	require.NoError(t, db.UpsertItem(ctx, ws.DB, feature))
	require.NoError(t, db.UpsertItem(ctx, ws.DB, &models.Artifact{
		ID: "970.001-T", Title: "Index-only sized child", Status: models.StatusActive,
		ArtifactType: "task", ParentID: "970-F", CustomFields: map[string]any{"size": "M"},
	}))

	result, err := SizeComposition(ctx, ws, feature)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.Histogram["M"], "index-only sized child must be counted")
	assert.Len(t, result.Members, 1, "one resolved task member")
	assert.Empty(t, result.Skipped, "no member should be skipped when present in the index")
}

// TestSizeCompositionResolvesShipmentManifestFromIndex proves the shipment
// manifest member-type resolution (task vs feature expansion) is also
// index-backed, not a per-member filesystem WalkDir.
func TestSizeCompositionResolvesShipmentManifestFromIndex(t *testing.T) {
	ws, _ := newSizeEstimationHarnessWorkspace(t)
	ctx := context.Background()
	require.NoError(t, db.UpsertItem(ctx, ws.DB, &models.Artifact{
		ID: "971-F", Title: "f", Status: models.StatusActive, ArtifactType: "feature",
	}))
	require.NoError(t, db.UpsertItem(ctx, ws.DB, &models.Artifact{
		ID: "971.001-T", Title: "t", Status: models.StatusActive, ArtifactType: "task",
		ParentID: "971-F", CustomFields: map[string]any{"size": "L"},
	}))
	shipment := &models.Artifact{
		ID: "971-S", Title: "s", Status: models.StatusActive, ArtifactType: "shipment",
		CustomFields: map[string]any{"items": []any{"971-F"}},
	}
	require.NoError(t, db.UpsertItem(ctx, ws.DB, shipment))

	result, err := SizeComposition(ctx, ws, shipment)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.Equal(t, 1, result.Histogram["L"], "expanded child task counted from index")
	assert.Len(t, result.Members, 1)
	assert.Empty(t, result.Skipped)
}

// TestGetItemsByIDsBatchResolvesAndOmitsMissing verifies the batch index
// resolver returns found artifacts keyed by ID and simply omits IDs with no
// indexed row (a miss is not an error).
func TestGetItemsByIDsBatchResolvesAndOmitsMissing(t *testing.T) {
	ws, _ := newSizeEstimationHarnessWorkspace(t)
	ctx := context.Background()
	require.NoError(t, db.UpsertItem(ctx, ws.DB, &models.Artifact{
		ID: "972.001-T", Title: "a", Status: models.StatusActive, ArtifactType: "task",
		CustomFields: map[string]any{"size": "S"},
	}))
	require.NoError(t, db.UpsertItem(ctx, ws.DB, &models.Artifact{
		ID: "972.002-T", Title: "b", Status: models.StatusActive, ArtifactType: "task",
	}))

	resolved, err := db.GetItemsByIDs(ctx, ws.DB, []string{"972.001-T", "972.002-T", "999.404-T", ""})
	require.NoError(t, err)
	require.Len(t, resolved, 2, "only the two indexed IDs resolve; missing and empty are omitted")
	require.Contains(t, resolved, "972.001-T")
	assert.Equal(t, "S", resolved["972.001-T"].CustomFields["size"])
	require.Contains(t, resolved, "972.002-T")
	assert.NotContains(t, resolved, "999.404-T")
}
