package core

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

const claimMarkerHarnessKey = "scheduler_baseline_claim"

func TestU0a_ClaimMarkerLifecycle(t *testing.T) {
	t.Run("claim_activation_sets_marker_on_each_activated_item", func(t *testing.T) {
		ctx := context.Background()
		ws := setupShipmentWorkspace(t)

		feature, err := CreateArtifact(ctx, ws, "Claim marker feature", "feature")
		require.NoError(t, err)
		require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))

		itemIDs := make([]string, 0, 2)
		for _, title := range []string{"Claim marker task A", "Claim marker task B"} {
			item, createErr := CreateArtifact(ctx, ws, title, "task", WithParent(feature.ID))
			require.NoError(t, createErr)
			require.NoError(t, bldb.UpsertItem(ctx, ws.DB, item))
			itemIDs = append(itemIDs, item.ID)
		}

		shipment, err := CreateShipment(ctx, ws, "Claim marker shipment", itemIDs)
		require.NoError(t, err)
		_, err = ClaimShipment(ctx, ws, shipment.ID)
		require.NoError(t, err)

		for _, itemID := range itemIDs {
			item, loadErr := loadArtifact(ctx, ws, itemID)
			require.NoError(t, loadErr)
			assert.Equal(t, shipment.ID, item.CustomFields[claimMarkerHarnessKey],
				"claim activation must set custom_fields.%s to activating shipment ID on %s",
				claimMarkerHarnessKey, itemID)
		}

		parent, err := loadArtifact(ctx, ws, feature.ID)
		require.NoError(t, err)
		assert.NotContains(t, parent.CustomFields, claimMarkerHarnessKey,
			"claim marker must not propagate to a parent through status cascade")
	})

	t.Run("non_claim_status_write_preserves_unrelated_frontmatter", func(t *testing.T) {
		ctx := context.Background()
		ws := setupShipmentWorkspace(t)

		item, err := CreateArtifact(ctx, ws, "Non-claim seam baseline", "feature")
		require.NoError(t, err)
		item.CustomFields = map[string]any{"existing_contract": "unchanged"}
		require.NoError(t, persistArtifact(ctx, ws, item, false))

		before := normalizedStatusIndependentFrontmatter(t, ctx, ws, item.ID)
		_, err = setArtifactStatus(ctx, ws, item.ID, models.StatusActive, "non-claim characterization")
		require.NoError(t, err)
		after := normalizedStatusIndependentFrontmatter(t, ctx, ws, item.ID)

		assert.Equal(t, before, after,
			"non-claim setArtifactStatus must preserve frontmatter outside status and updated_at")
	})

	t.Run("rollback_clears_marker_and_restores_nil_custom_fields", func(t *testing.T) {
		ctx := context.Background()
		ws := setupShipmentWorkspace(t)

		shipment, err := CreateShipment(ctx, ws, "Rollback marker shipment", nil)
		require.NoError(t, err)

		activatedIDs := make([]string, 0, 2)
		for _, title := range []string{"Rollback marker feature A", "Rollback marker feature B"} {
			item, createErr := CreateArtifact(ctx, ws, title, "feature")
			require.NoError(t, createErr)
			item.Status = models.StatusActive
			item.CustomFields = map[string]any{claimMarkerHarnessKey: shipment.ID}
			require.NoError(t, persistArtifact(ctx, ws, item, true))
			activatedIDs = append(activatedIDs, item.ID)
		}

		err = rollbackShipmentClaim(
			ctx,
			ws,
			shipment.ID,
			cloneArtifact(shipment),
			activatedIDs,
			errors.New("forced claim failure"),
		)
		require.Error(t, err)

		for _, itemID := range activatedIDs {
			item, loadErr := loadArtifact(ctx, ws, itemID)
			require.NoError(t, loadErr)
			assert.Equal(t, models.StatusQueued, item.Status)
			assert.Nil(t, item.CustomFields,
				"rollback must delete custom_fields.%s and restore nil custom_fields on %s",
				claimMarkerHarnessKey, itemID)
		}
	})
}

func normalizedStatusIndependentFrontmatter(
	t *testing.T,
	ctx context.Context,
	ws *Workspace,
	itemID string,
) []byte {
	t.Helper()

	path, err := FindArtifactPath(ctx, ws, itemID)
	require.NoError(t, err)
	raw, err := os.ReadFile(path)
	require.NoError(t, err)
	frontmatter, _, err := models.ParseFrontmatter(string(raw))
	require.NoError(t, err)
	delete(frontmatter, "status")
	delete(frontmatter, "updated_at")
	normalized, err := json.Marshal(frontmatter)
	require.NoError(t, err)
	return normalized
}
