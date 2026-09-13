package core

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// rewriteShipmentReconcileRawManifestItems overwrites the archived shipment's
// RAW custom_fields.items frontmatter value with items (an arbitrary,
// possibly-malformed slice), round-tripping through the same
// WriteArtifactFileWithOptions/SerializeFrontmatter path production writers
// use, so the reload under test sees exactly the YAML shape a corrupt/legacy
// manifest would produce (mixed scalar types, blanks, duplicates).
func rewriteShipmentReconcileRawManifestItems(t *testing.T, ws *Workspace, shipmentID string, items []any) {
	t.Helper()
	ctx := context.Background()
	shipment, err := findArtifact(ctx, ws, shipmentID)
	require.NoError(t, err)
	if shipment.CustomFields == nil {
		shipment.CustomFields = map[string]any{}
	}
	shipment.CustomFields["items"] = items
	path, err := FindArtifactPath(ctx, ws, shipmentID)
	require.NoError(t, err)
	require.NoError(t, WriteArtifactFileWithOptions(shipment, path, false))
}

// TestValidateShipmentReconcilePreconditions_RawManifestMustBeUniqueNonBlankStrings
// covers PR #440 review round 2, finding 4: the RAW `items` manifest value
// must be validated as a well-formed array of unique, non-blank strings
// BEFORE any normalization/dropping (NormalizeShipmentItems +
// uniqueNonEmptyStrings) runs. A malformed/corrupt legacy manifest —
// containing a non-string element, a blank/whitespace-only string, or a
// duplicate — must fail closed rather than be silently reduced to its
// surviving valid entries.
func TestValidateShipmentReconcilePreconditions_RawManifestMustBeUniqueNonBlankStrings(t *testing.T) {
	cases := []struct {
		name  string
		items []any
	}{
		{name: "non_string_element", items: []any{"done-member", 42}},
		{name: "blank_element", items: []any{"done-member", "   "}},
		{name: "duplicate_element", items: []any{"done-member", "done-member"}},
		{name: "mixed_corruption_from_finding", items: []any{"done-member", 42, "", "done-member"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ws := setupShipmentWorkspace(t)
			ctx := context.Background()
			memberID := createShipmentReconcileMemberWithStatus(t, ws, models.StatusDone)
			shipment := createArchivedShipmentReconcileFixture(t, ws, []string{memberID})
			items := make([]any, 0, len(tc.items))
			for _, item := range tc.items {
				if s, ok := item.(string); ok && s == "done-member" {
					items = append(items, memberID)
					continue
				}
				items = append(items, item)
			}
			rewriteShipmentReconcileRawManifestItems(t, ws, shipment.ID, items)
			req := validShipmentReconcilePreconditionRequest(shipment.ID)
			digest := mustShipmentReconcileRequestIdentityDigest(t, req)

			_, err := validateShipmentReconcilePreconditions(ctx, ws, req, digest)
			require.Error(t, err, "a malformed raw manifest must fail closed, not be silently repaired to its surviving valid entries")
			assert.ErrorIs(t, err, blerrors.ErrValidation)
		})
	}
}

// TestValidateShipmentReconcilePreconditions_WellFormedRawManifestStillAccepted
// is the positive control: a raw manifest that is already a well-formed,
// unique, non-blank string array continues to validate cleanly (the new
// raw-manifest guard must not reject legitimate input).
func TestValidateShipmentReconcilePreconditions_WellFormedRawManifestStillAccepted(t *testing.T) {
	ws := setupShipmentWorkspace(t)
	ctx := context.Background()
	memberID := createShipmentReconcileMemberWithStatus(t, ws, models.StatusDone)
	shipment := createArchivedShipmentReconcileFixture(t, ws, []string{memberID})
	rewriteShipmentReconcileRawManifestItems(t, ws, shipment.ID, []any{memberID})
	req := validShipmentReconcilePreconditionRequest(shipment.ID)
	digest := mustShipmentReconcileRequestIdentityDigest(t, req)

	reloaded, err := validateShipmentReconcilePreconditions(ctx, ws, req, digest)
	require.NoError(t, err)
	require.NotNil(t, reloaded)
}

// TestReconcileShipmentToShipped_RawManifestValidationRunsInPhaseA proves the
// raw-manifest guard is wired into the real transaction entry point
// (reconcileShipmentToShippedImpl's Phase A), not just the standalone
// precondition gate: a malformed raw manifest must be rejected before Phase
// C's evidence gathering ever runs (it fails even against a request whose
// merge_sha/closure evidence are not backed by any real git history).
func TestReconcileShipmentToShipped_RawManifestValidationRunsInPhaseA(t *testing.T) {
	ws, shipmentID, memberID := u20ReconcileFixture(t)
	rewriteShipmentReconcileRawManifestItems(t, ws, shipmentID, []any{memberID, 7, "", memberID})
	req := validShipmentReconcilePreconditionRequest(shipmentID)

	_, err := ReconcileShipmentToShipped(context.Background(), ws, req)
	require.Error(t, err, "a malformed raw manifest must fail closed in Phase A, before evidence gathering")
	assert.ErrorIs(t, err, blerrors.ErrValidation)
}
