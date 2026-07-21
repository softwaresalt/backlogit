package core

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

// SizeCompositionKey is the read-surface JSON field under which the
// computed-on-read size rollup is attached. Both the CLI and MCP transports
// consume this constant so the two surfaces cannot drift on the field name
// (108-F / 114-F parity).
const SizeCompositionKey = "size_composition"

// IsSizeCompositionAggregate reports whether an artifact type carries a
// computed-on-read size rollup on read surfaces. Size estimation is task-only,
// so only the rollup-parent types (feature, shipment) are projectable. The CLI
// and MCP read surfaces share this predicate so they cannot drift on which
// types expose the rollup.
func IsSizeCompositionAggregate(artifactType string) bool {
	return artifactType == "feature" || artifactType == "shipment"
}

// SizeCompositionMember describes one canonical member included in a size rollup.
type SizeCompositionMember struct {
	ID           string `json:"id"`
	ArtifactType string `json:"artifact_type"`
	Size         string `json:"size,omitempty"`
}

// SizeCompositionResult is the computed-on-read size rollup for a feature or shipment.
type SizeCompositionResult struct {
	Histogram      map[string]int          `json:"histogram"`
	Unsized        int                     `json:"unsized"`
	Members        []SizeCompositionMember `json:"members"`
	Skipped        []string                `json:"skipped,omitempty"`
	RulesetVersion *string                 `json:"ruleset_version"`
}

// SizeComposition computes the never-persisted size rollup for a feature or
// shipment (108-F SE-4). Size estimation is task-only, so the rollup counts only
// TASK members: feature membership is its direct task children by parent_id, and
// shipment membership is the explicit custom_fields.items manifest with each
// feature member expanded into its child tasks (the feature itself, a rollup
// parent, is never counted). Members are de-duplicated so a manifest listing a
// feature and its explicit child tasks counts each once. An existing task member
// with no size increments Unsized; an unresolved manifest id is warn-skipped
// (counted in neither Histogram nor Unsized). The result is computed on read and
// never written to disk. ruleset_version is always null until a canonical ruleset
// is owned.
func SizeComposition(ctx context.Context, ws *Workspace, artifact *models.Artifact) (*SizeCompositionResult, error) {
	if artifact == nil {
		return nil, fmt.Errorf("size composition: artifact is required")
	}
	if ws == nil {
		return nil, fmt.Errorf("size composition: workspace is required")
	}
	result := &SizeCompositionResult{
		Histogram: map[string]int{},
		Members:   []SizeCompositionMember{},
		Skipped:   []string{},
	}

	memberIDs, err := compositionMemberIDs(ctx, ws, artifact)
	if err != nil {
		return nil, fmt.Errorf("size composition members for %s: %w", artifact.ID, err)
	}
	unique := uniqueNonEmptyStrings(memberIDs)
	// 114-F / 47ED88ED: resolve all members from the SQLite index in a single
	// batched query instead of a per-member filesystem WalkDir. The read
	// surfaces already operate off the index, and the index includes archived
	// artifacts, so index resolution is both faster and consistent.
	resolved, err := resolveMembersFromIndex(ctx, ws, unique)
	if err != nil {
		return nil, fmt.Errorf("resolve size composition members for %s: %w", artifact.ID, err)
	}
	for _, id := range unique {
		member, ok := resolved[id]
		if !ok {
			// Unresolved manifest id: warn + skip; not counted in the histogram
			// or unsized.
			slog.WarnContext(ctx, "size composition: skipping unresolved member", "member_id", id)
			result.Skipped = append(result.Skipped, id)
			continue
		}
		size, _ := member.CustomFields["size"].(string)
		result.Members = append(result.Members, SizeCompositionMember{
			ID:           member.ID,
			ArtifactType: member.ArtifactType,
			Size:         size,
		})
		if size == "" {
			result.Unsized++
		} else {
			result.Histogram[size]++
		}
	}
	return result, nil
}

// resolveMembersFromIndex batch-resolves member artifacts from the SQLite index,
// guarding a nil workspace/DB (in which case no members resolve, consistent with
// childIDsByParent). It underpins the index-backed size rollup (114-F).
func resolveMembersFromIndex(ctx context.Context, ws *Workspace, ids []string) (map[string]*models.Artifact, error) {
	if ws == nil || ws.DB == nil {
		return map[string]*models.Artifact{}, nil
	}
	return db.GetItemsByIDs(ctx, ws.DB, ids)
}

// compositionMemberIDs resolves the canonical member IDs for a size rollup. Size
// estimation is task-only, so this yields only task members: for a feature, its
// direct task children by parent_id; for a shipment, the explicit manifest with
// directly-listed tasks kept and each feature member expanded into its child
// tasks (rollup-parent types such as the feature itself are excluded).
func compositionMemberIDs(ctx context.Context, ws *Workspace, artifact *models.Artifact) ([]string, error) {
	switch artifact.ArtifactType {
	case "feature":
		return childIDsByParent(ctx, ws, artifact.ID)
	case "shipment":
		manifest := NormalizeShipmentItems(artifact)
		// 114-F / 47ED88ED: batch-resolve manifest member types from the index
		// in one query instead of a per-member filesystem WalkDir.
		resolved, err := resolveMembersFromIndex(ctx, ws, manifest)
		if err != nil {
			return nil, fmt.Errorf("resolve shipment manifest: %w", err)
		}
		var ids []string
		for _, memberID := range manifest {
			member, ok := resolved[memberID]
			if !ok {
				// Dangling manifest id: keep it so the main loop records it as
				// skipped (warn + counted in neither histogram nor unsized).
				ids = append(ids, memberID)
				continue
			}
			switch member.ArtifactType {
			case "task":
				// A directly-listed task is a sizable member.
				ids = append(ids, memberID)
			case "feature":
				// A feature is a rollup parent, not a sizable unit: expand it into
				// its child tasks and do NOT count the feature itself.
				childIDs, cerr := childIDsByParent(ctx, ws, memberID)
				if cerr != nil {
					return nil, cerr
				}
				ids = append(ids, childIDs...)
			default:
				// Any other manifest member type (subtask, review, ...) is not a
				// sizable unit and is excluded from the rollup.
			}
		}
		return ids, nil
	default:
		return nil, fmt.Errorf("size composition: unsupported artifact type %q", artifact.ArtifactType)
	}
}

// childIDsByParent returns the IDs of the direct TASK children (the only sizable
// unit) of parentID from the SQLite index. Non-task children (e.g. review) are
// excluded so they never skew the rollup as spurious unsized members.
func childIDsByParent(ctx context.Context, ws *Workspace, parentID string) ([]string, error) {
	if ws == nil || ws.DB == nil {
		return nil, nil
	}
	rows, err := ws.DB.QueryContext(ctx, `SELECT id FROM items WHERE parent_id = ? AND artifact_type = 'task' ORDER BY id`, parentID)
	if err != nil {
		return nil, fmt.Errorf("query children of %s: %w", parentID, err)
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan child id of %s: %w", parentID, err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate children of %s: %w", parentID, err)
	}
	return ids, nil
}

// AttachSizeComposition marshals v into a generic map and attaches the
// computed-on-read size rollup under SizeCompositionKey without mutating or
// persisting v. It is shared by the CLI `get --json` and MCP get_item /
// get_shipment read surfaces so the projection shape cannot drift.
func AttachSizeComposition(v any, composition *SizeCompositionResult) (map[string]any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal for size composition: %w", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal for size composition: %w", err)
	}
	payload[SizeCompositionKey] = composition
	return payload, nil
}

// ShipmentViewWithComposition builds the read-only shipment view (carrying the
// render-time covering_feature projection) and attaches the computed-on-read
// size_composition rollup. On any rollup or projection failure it logs and
// returns the plain view so a rollup error never fails the surface. It is shared
// by the CLI `shipment get` and MCP get_shipment read surfaces so the two cannot
// drift (114-F parity).
func ShipmentViewWithComposition(ctx context.Context, ws *Workspace, shipment *models.Artifact) any {
	view := NewShipmentView(ctx, ws, shipment)
	if shipment == nil {
		return view
	}
	composition, err := SizeComposition(ctx, ws, shipment)
	if err != nil {
		slog.WarnContext(ctx, "size composition: shipment left without rollup", "id", shipment.ID, "error", err)
		return view
	}
	if composition == nil {
		return view
	}
	payload, perr := AttachSizeComposition(view, composition)
	if perr != nil {
		slog.WarnContext(ctx, "size composition: shipment projection failed", "id", shipment.ID, "error", perr)
		return view
	}
	return payload
}

// ShipmentViewsWithComposition maps ShipmentViewWithComposition over shipments,
// preserving order. It is shared by the CLI `shipment list` and MCP
// list_shipments read surfaces so the two cannot drift (114-F parity).
func ShipmentViewsWithComposition(ctx context.Context, ws *Workspace, shipments []*models.Artifact) []any {
	out := make([]any, len(shipments))
	for i, shipment := range shipments {
		out[i] = ShipmentViewWithComposition(ctx, ws, shipment)
	}
	return out
}

// aggregate (feature/shipment) item map, matching the typed artifact slice by
// index so order is preserved. Non-aggregate types are left unprojected; a
// rollup failure is logged and that item is left without a rollup rather than
// failing the whole response. It is shared by the CLI `queue view --json` and
// MCP get_queue read surfaces so the two surfaces cannot drift.
func InjectSizeComposition(ctx context.Context, ws *Workspace, artifacts []*models.Artifact, itemMaps []any) {
	for i, art := range artifacts {
		if i >= len(itemMaps) || art == nil {
			continue
		}
		if !IsSizeCompositionAggregate(art.ArtifactType) {
			continue
		}
		im, ok := itemMaps[i].(map[string]any)
		if !ok {
			continue
		}
		composition, err := SizeComposition(ctx, ws, art)
		if err != nil {
			slog.WarnContext(ctx, "size composition: item left without rollup", "id", art.ID, "error", err)
			continue
		}
		if composition != nil {
			im[SizeCompositionKey] = composition
		}
	}
}
