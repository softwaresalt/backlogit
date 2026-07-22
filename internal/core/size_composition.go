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
//
// Staleness window: the rollup is computed from the SQLite index, which is a
// disposable cache rebuilt from the Markdown source of truth on sync. Between an
// out-of-band on-disk mutation and the next index sync/rehydrate, a rollup can
// reflect slightly stale sizes or membership. This is an accepted, best-effort
// read contract (the read surfaces already tolerate it); callers that need
// guaranteed freshness must sync the index first. No freshness/version API is
// exposed here by design (YAGNI) until a concrete consumer requires one.
func SizeComposition(ctx context.Context, ws *Workspace, artifact *models.Artifact) (*SizeCompositionResult, error) {
	if artifact == nil {
		return nil, fmt.Errorf("size composition: artifact is required")
	}
	if ws == nil {
		return nil, fmt.Errorf("size composition: workspace is required")
	}

	deps := indexMemberDeps(ws)
	memberIDs, err := compositionMemberIDs(ctx, artifact, deps)
	if err != nil {
		return nil, fmt.Errorf("size composition members for %s: %w", artifact.ID, err)
	}
	unique := uniqueNonEmptyStrings(memberIDs)
	// 114-F / 47ED88ED: resolve all members from the SQLite index in a single
	// batched query instead of a per-member filesystem WalkDir. The read
	// surfaces already operate off the index, and the index includes archived
	// artifacts, so index resolution is both faster and consistent.
	resolved, err := deps.resolve(ctx, unique)
	if err != nil {
		return nil, fmt.Errorf("resolve size composition members for %s: %w", artifact.ID, err)
	}
	return rollupFromMembers(ctx, unique, resolved), nil
}

// SizeCompositions computes the never-persisted size rollup for many aggregates
// (feature/shipment) in a bounded, constant number of batched index round-trips
// instead of one SizeComposition call per aggregate, returning a map keyed by
// aggregate ID. Non-aggregate artifacts and nil entries are skipped (absent from
// the map) and duplicate aggregate IDs are computed once. Every rollup is
// byte-identical to the per-artifact SizeComposition because both share
// compositionMemberIDs and rollupFromMembers over the same index rows, so this
// batched shaper can back the grouped queue/list read surfaces without changing
// output (117-F / A6A1B47E). Like SizeComposition, resolution is computed on read
// off the SQLite index and is never written to disk, and shares the same
// index-staleness read contract documented on SizeComposition.
func SizeCompositions(ctx context.Context, ws *Workspace, artifacts []*models.Artifact) (map[string]*SizeCompositionResult, error) {
	out := make(map[string]*SizeCompositionResult)
	if ws == nil {
		return out, nil
	}

	// Collect the aggregate artifacts, de-duplicated by ID (first occurrence
	// wins) so a member listed under several groups is computed once.
	seen := make(map[string]struct{}, len(artifacts))
	aggregates := make([]*models.Artifact, 0, len(artifacts))
	for _, a := range artifacts {
		if a == nil || !IsSizeCompositionAggregate(a.ArtifactType) {
			continue
		}
		if _, ok := seen[a.ID]; ok {
			continue
		}
		seen[a.ID] = struct{}{}
		aggregates = append(aggregates, a)
	}
	if len(aggregates) == 0 {
		return out, nil
	}

	// Prefetch phase: a bounded, constant number of batched index queries that
	// remove the per-aggregate query fan-out (the N+1 the single path incurs).

	// 1) Resolve every shipment manifest member once to learn its type.
	manifestUnion := make([]string, 0)
	for _, a := range aggregates {
		if a.ArtifactType == "shipment" {
			manifestUnion = append(manifestUnion, NormalizeShipmentItems(a)...)
		}
	}
	manifestResolved, err := resolveMembersFromIndex(ctx, ws, uniqueNonEmptyStrings(manifestUnion))
	if err != nil {
		return nil, fmt.Errorf("batch resolve shipment manifests: %w", err)
	}

	// 2) Resolve the task children of every feature parent once: input features
	//    plus any feature referenced by a shipment manifest (expanded into its
	//    tasks, never counted itself).
	parentIDs := make([]string, 0, len(aggregates))
	for _, a := range aggregates {
		if a.ArtifactType == "feature" {
			parentIDs = append(parentIDs, a.ID)
		}
	}
	for _, m := range manifestResolved {
		if m.ArtifactType == "feature" {
			parentIDs = append(parentIDs, m.ID)
		}
	}
	childrenMap, err := db.GetTaskChildrenByParentIDs(ctx, ws.DB, uniqueNonEmptyStrings(parentIDs))
	if err != nil {
		return nil, fmt.Errorf("batch resolve task children: %w", err)
	}

	// 3) Combined resolution map for the final rollup: manifest members plus
	//    every prefetched child artifact.
	batchResolved := make(map[string]*models.Artifact, len(manifestResolved))
	for id, a := range manifestResolved {
		batchResolved[id] = a
	}
	for _, children := range childrenMap {
		for _, c := range children {
			batchResolved[c.ID] = c
		}
	}

	// Compute phase: pure projection over the prefetched maps, no further index
	// round-trips.
	deps := prefetchedMemberDeps(childrenMap, batchResolved)
	for _, a := range aggregates {
		memberIDs, err := compositionMemberIDs(ctx, a, deps)
		if err != nil {
			return nil, fmt.Errorf("size composition members for %s: %w", a.ID, err)
		}
		unique := uniqueNonEmptyStrings(memberIDs)
		out[a.ID] = rollupFromMembers(ctx, unique, batchResolved)
	}
	return out, nil
}

// memberDeps abstracts the two index round-trips a size rollup needs — resolving
// a set of member IDs and listing a parent's task-child IDs — so the per-artifact
// and batched paths share compositionMemberIDs. The single path binds these to
// direct index queries; the batched path binds them to prefetched-map reads,
// guaranteeing identical output (117-F / A6A1B47E).
type memberDeps struct {
	childIDs func(ctx context.Context, parentID string) ([]string, error)
	resolve  func(ctx context.Context, ids []string) (map[string]*models.Artifact, error)
}

// indexMemberDeps binds memberDeps to direct SQLite-index queries for the
// per-artifact SizeComposition path.
func indexMemberDeps(ws *Workspace) memberDeps {
	return memberDeps{
		childIDs: func(ctx context.Context, parentID string) ([]string, error) {
			return childIDsByParent(ctx, ws, parentID)
		},
		resolve: func(ctx context.Context, ids []string) (map[string]*models.Artifact, error) {
			return resolveMembersFromIndex(ctx, ws, ids)
		},
	}
}

// prefetchedMemberDeps binds memberDeps to already-prefetched maps for the
// batched SizeCompositions path: child IDs come from childrenMap (ordered by ID,
// matching childIDsByParent) and member resolution is a subset lookup of the
// combined resolved map.
func prefetchedMemberDeps(childrenMap map[string][]*models.Artifact, resolved map[string]*models.Artifact) memberDeps {
	return memberDeps{
		childIDs: func(_ context.Context, parentID string) ([]string, error) {
			children := childrenMap[parentID]
			ids := make([]string, len(children))
			for i, c := range children {
				ids[i] = c.ID
			}
			return ids, nil
		},
		resolve: func(_ context.Context, ids []string) (map[string]*models.Artifact, error) {
			out := make(map[string]*models.Artifact, len(ids))
			for _, id := range ids {
				if a, ok := resolved[id]; ok {
					out[id] = a
				}
			}
			return out, nil
		},
	}
}

// rollupFromMembers builds the size rollup from an ordered, de-duplicated set of
// member IDs and their resolved artifacts. An id absent from resolved is
// warn-skipped (counted in neither Histogram nor Unsized); a resolved member with
// no size increments Unsized. It is the shared tail of both the per-artifact and
// batched rollup paths so the two cannot drift.
func rollupFromMembers(ctx context.Context, orderedUnique []string, resolved map[string]*models.Artifact) *SizeCompositionResult {
	result := &SizeCompositionResult{
		Histogram: map[string]int{},
		Members:   []SizeCompositionMember{},
		Skipped:   []string{},
	}
	for _, id := range orderedUnique {
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
	return result
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
// tasks (rollup-parent types such as the feature itself are excluded). Index
// access is routed through deps so the per-artifact and batched paths share this
// resolution logic and cannot drift.
func compositionMemberIDs(ctx context.Context, artifact *models.Artifact, deps memberDeps) ([]string, error) {
	switch artifact.ArtifactType {
	case "feature":
		return deps.childIDs(ctx, artifact.ID)
	case "shipment":
		manifest := NormalizeShipmentItems(artifact)
		// 114-F / 47ED88ED: batch-resolve manifest member types from the index
		// in one query instead of a per-member filesystem WalkDir.
		resolved, err := deps.resolve(ctx, manifest)
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
				childIDs, cerr := deps.childIDs(ctx, memberID)
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
// get_shipment read surfaces so the projection shape cannot drift. A payload
// that marshals to a JSON null (an untyped nil or a typed nil pointer) or to a
// non-object is rejected with an error rather than panicking on assignment to a
// nil map.
func AttachSizeComposition(v any, composition *SizeCompositionResult) (map[string]any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("marshal for size composition: %w", err)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("unmarshal for size composition: %w", err)
	}
	if payload == nil {
		return nil, fmt.Errorf("size composition: cannot attach rollup to a nil or non-object payload")
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

// InjectSizeCompositionFromMap projects precomputed rollups onto each aggregate
// (feature/shipment) item map, matching the typed artifact slice by index so
// order is preserved. Non-aggregate types and artifacts absent from comps are
// left unprojected. Callers compute rollups once via SizeCompositions and inject
// them onto several views (e.g. a flat queue and each of its groups) without
// recomputing (117-F / A6A1B47E).
func InjectSizeCompositionFromMap(artifacts []*models.Artifact, itemMaps []any, comps map[string]*SizeCompositionResult) {
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
		if comp, ok := comps[art.ID]; ok && comp != nil {
			im[SizeCompositionKey] = comp
		}
	}
}

// ListWithSizeComposition projects the computed-on-read size rollup onto the
// aggregate (feature/shipment) artifacts in a flat list and returns a slice ready
// for JSON encoding: each aggregate becomes a map carrying size_composition and
// every non-aggregate stays the raw *models.Artifact (so it carries no rollup).
// Order is preserved. Rollups are computed once via SizeCompositions; on a batch
// error it warns and returns the raw artifacts so the list surface degrades
// rather than failing. It is shared by the CLI `list --json` and MCP list_items
// read surfaces so the two cannot drift (117-F / 60336CC0).
func ListWithSizeComposition(ctx context.Context, ws *Workspace, artifacts []*models.Artifact) []any {
	out := make([]any, len(artifacts))
	comps, err := SizeCompositions(ctx, ws, artifacts)
	if err != nil {
		slog.WarnContext(ctx, "size composition: list left without rollups", "error", err)
		for i, art := range artifacts {
			out[i] = art
		}
		return out
	}
	for i, art := range artifacts {
		if art == nil {
			out[i] = art
			continue
		}
		comp, ok := comps[art.ID]
		if !ok || comp == nil {
			out[i] = art
			continue
		}
		payload, perr := AttachSizeComposition(art, comp)
		if perr != nil {
			slog.WarnContext(ctx, "size composition: list projection failed", "id", art.ID, "error", perr)
			out[i] = art
			continue
		}
		out[i] = payload
	}
	return out
}
