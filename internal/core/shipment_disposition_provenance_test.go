package core

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/models"
)

// TestShipmentDispositionProvenance proves the disposition provenance survives
// later writes and an archive, asserted after a reload from Markdown
// (B-U12 / 192.005-T).
func TestShipmentDispositionProvenance(t *testing.T) {
	ctx := context.Background()

	disposed := func(t *testing.T) (*disposeFixture, string) {
		t.Helper()
		f := newDisposeFixture(t)
		authorizationRef := f.deliberation(t, "done", f.shipmentID)
		_, err := DisposeQueuedShipment(ctx, f.ws, f.shipmentID, f.opts(authorizationRef))
		require.NoError(t, err)
		return f, authorizationRef
	}
	requireProvenance := func(t *testing.T, got *models.Artifact, f *disposeFixture, authorizationRef string) {
		t.Helper()
		require.Equal(t, "superseded by a newer plan", got.CustomFields["disposition_reason"])
		require.Equal(t, "operator", got.CustomFields["disposition_by"])
		require.Equal(t, authorizationRef, got.CustomFields["disposition_authorization_ref"])
		require.Equal(t, f.otherID, got.CustomFields["superseded_by"])
	}

	t.Run("survives_unrelated_update", func(t *testing.T) {
		f, authorizationRef := disposed(t)

		_, err := UpdateArtifact(ctx, f.ws, f.shipmentID, map[string]any{"title": "Renamed after disposition"})
		require.NoError(t, err)

		got := loadURCanonicalArtifact(t, f.ws, f.shipmentID)
		require.Equal(t, "Renamed after disposition", got.Title)
		require.Equal(t, models.StatusAbandoned, got.Status)
		requireProvenance(t, got, f, authorizationRef)
	})

	t.Run("survives_archive", func(t *testing.T) {
		f, authorizationRef := disposed(t)

		_, err := ArchiveItem(ctx, f.ws.DB, f.ws, f.shipmentID)
		require.NoError(t, err)

		got := loadURCanonicalArtifact(t, f.ws, f.shipmentID)
		require.Equal(t, models.StatusArchived, got.Status)
		require.Equal(t, "abandoned", got.ArchivedStatus)
		requireProvenance(t, got, f, authorizationRef)

		path, err := FindArtifactPath(ctx, f.ws, f.shipmentID)
		require.NoError(t, err)
		raw, err := os.ReadFile(path)
		require.NoError(t, err)
		fm, _, err := models.ParseFrontmatter(string(raw))
		require.NoError(t, err)
		require.Equal(t, "abandoned", fm["archived_status"])
		custom, ok := fm["custom_fields"].(map[string]any)
		require.True(t, ok, "custom_fields must be present in the archived file")
		for _, key := range dispositionProvenanceKeys {
			require.Contains(t, custom, key)
		}
	})

	t.Run("never_counts_as_shipped_predecessor", func(t *testing.T) {
		f, _ := disposed(t)

		shipped, reason, err := ShipmentPredecessorShipped(ctx, f.ws, f.shipmentID)
		require.NoError(t, err)
		require.False(t, shipped, "live disposed shipment, reason %q", reason)

		_, err = ArchiveItem(ctx, f.ws.DB, f.ws, f.shipmentID)
		require.NoError(t, err)
		shipped, reason, err = ShipmentPredecessorShipped(ctx, f.ws, f.shipmentID)
		require.NoError(t, err)
		require.False(t, shipped, "archived disposed shipment, reason %q", reason)
	})
}
