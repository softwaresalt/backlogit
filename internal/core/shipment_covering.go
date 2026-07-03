package core

import (
	"context"
	"errors"
	"log/slog"
	"strings"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// CoveringFeature is the read-only, render-time projection of the root feature a
// shipment delivers. It is derived from the shipment manifest and never
// persisted. It is a named value object (rather than two positional strings) so
// the shape can grow without call-site churn.
type CoveringFeature struct {
	ID    string `json:"id"`
	Title string `json:"title"`
}

// ShipmentView is a read-only response envelope that embeds the full shipment
// artifact and adds a derived covering-feature field. The CoveringFeature
// pointer is nil — and omitted from JSON via omitempty — when the shipment has
// no covering feature.
//
// The derived data is a top-level sibling of the promoted artifact fields and is
// NEVER written into the persisted custom_fields map, so it cannot round-trip
// back through any write path (the retroactive manifest mutation the B8FF7590
// determination forbids). Both the CLI and MCP surfaces marshal this one type so
// field name and omit semantics have a single source of truth.
type ShipmentView struct {
	*models.Artifact
	CoveringFeature *CoveringFeature `json:"covering_feature,omitempty"`
}

// DeriveCoveringFeature resolves the root covering feature for a shipment from
// its manifest (custom_fields.items), returning (feature, true) on success or
// (zero-value, false) when the shipment has no resolvable root feature.
//
// It is a PURE READ. Member IDs come from NormalizeShipmentItems (the f015 read-edge
// normalizer) and are resolved with bldb.GetItem — never loadArtifact, which
// upserts the index on a cache miss (a DB write). The input shipment is not
// mutated and nothing is persisted.
//
// The covering feature is the first manifest item, in parent-first order, that
// is a feature with a dotless root ID. Items that resolve to ErrNotFound are
// skipped defensively; any other lookup error is logged (slog.WarnContext) and
// skipped rather than silently swallowed or failing the whole render.
func DeriveCoveringFeature(ctx context.Context, ws *Workspace, shipment *models.Artifact) (CoveringFeature, bool) {
	// A nil workspace, an unseeded index handle (ws.DB == nil), or a nil
	// shipment cannot resolve a covering feature. Guarding ws.DB here keeps the
	// derivation panic-safe for Workspace values constructed without a DB (e.g.
	// Workspace{} in tests or future call sites): bldb.GetItem dereferences the
	// *sql.DB directly, so a nil handle must fall back to the safe (ok=false)
	// branch rather than panic.
	if ws == nil || ws.DB == nil || shipment == nil {
		return CoveringFeature{}, false
	}
	for _, itemID := range NormalizeShipmentItems(shipment) {
		item, err := bldb.GetItem(ctx, ws.DB, itemID)
		if err != nil {
			if errors.Is(err, blerrors.ErrNotFound) {
				slog.DebugContext(ctx, "covering-feature: manifest item not found",
					"shipment_id", shipment.ID, "item_id", itemID)
				continue
			}
			slog.WarnContext(ctx, "covering-feature: manifest item lookup failed",
				"shipment_id", shipment.ID, "item_id", itemID, "error", err)
			continue
		}
		if isRootCoveringFeature(item) {
			return CoveringFeature{ID: item.ID, Title: item.Title}, true
		}
	}
	return CoveringFeature{}, false
}

// isRootCoveringFeature reports whether the resolved artifact is a root covering
// feature: type "feature" with a dotless root ID. The dotless-ID predicate is
// the primary, frontmatter-level-independent signal — nested features always
// carry a dotted ID (e.g. 013.001-F), root features never do (e.g. 058-F).
// Level==1 corroborates in the live index but is intentionally not gated on:
// archived-artifact frontmatter may omit level, and gating on it would drop a
// legitimately archived covering feature.
func isRootCoveringFeature(item *models.Artifact) bool {
	if item == nil {
		return false
	}
	if item.ArtifactType != "feature" {
		return false
	}
	return !strings.Contains(item.ID, ".")
}

// NewShipmentView builds a read-only response envelope for a single shipment,
// deriving the covering feature at render time. The shipment artifact is
// embedded unchanged; the covering-feature pointer is nil (and omitted) when the
// shipment has no covering feature. This is the single shared shaper consumed by
// both the CLI and MCP surfaces.
func NewShipmentView(ctx context.Context, ws *Workspace, shipment *models.Artifact) ShipmentView {
	view := ShipmentView{Artifact: shipment}
	if cf, ok := DeriveCoveringFeature(ctx, ws, shipment); ok {
		derived := cf
		view.CoveringFeature = &derived
	}
	return view
}

// NewShipmentViews builds read-only response envelopes for a slice of shipments,
// preserving order, so list surfaces share identical shape with get.
func NewShipmentViews(ctx context.Context, ws *Workspace, shipments []*models.Artifact) []ShipmentView {
	views := make([]ShipmentView, len(shipments))
	for i, shipment := range shipments {
		views[i] = NewShipmentView(ctx, ws, shipment)
	}
	return views
}
