package core

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

const (
	shipmentReorderModeAuto     = "auto"
	shipmentReorderModeExplicit = "explicit"
	shipmentReorderTarget       = "reorder"
)

// ReorderShipmentResult reports a ReorderShipment outcome or preview.
type ReorderShipmentResult struct {
	ShipmentID    string   `json:"shipment_id"`
	Mode          string   `json:"mode"`
	Status        string   `json:"status"`
	DryRun        bool     `json:"dry_run"`
	Force         bool     `json:"force,omitempty"`
	Reason        string   `json:"reason,omitempty"`
	CorrelationID string   `json:"correlation_id,omitempty"`
	ItemsBefore   []string `json:"items_before"`
	ItemsAfter    []string `json:"items_after"`
}

// reorderGraph is the two-phase dependency model shared by --auto and --items.
//
// Root covering features form the leading prefix; every other member is a
// vertex of G. G holds every hard intra-manifest blocks edge plus the default
// parent-before-child edges that do not close a cycle.
type reorderGraph struct {
	members   []string
	index     map[string]int
	parentOf  map[string]string
	roots     []string
	nonRoots  []string
	rootEdges map[string][]string
	edges     map[string][]string
}

func appendEdge(edges map[string][]string, from, to string) {
	if containsString(edges[from], to) {
		return
	}
	edges[from] = append(edges[from], to)
}

func edgeReachable(edges map[string][]string, from, to string) bool {
	seen := map[string]bool{from: true}
	stack := []string{from}
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if current == to {
			return true
		}
		for _, next := range edges[current] {
			if !seen[next] {
				seen[next] = true
				stack = append(stack, next)
			}
		}
	}
	return false
}

// topoOrder runs Kahn's algorithm over nodes (given in manifest order). pick
// chooses among the ready nodes (always in manifest order). ok is false when a
// cycle prevents emitting every node.
func topoOrder(
	nodes []string,
	edges map[string][]string,
	pick func(ready, emitted []string) string,
) (order []string, ok bool) {
	indegree := make(map[string]int, len(nodes))
	for _, node := range nodes {
		indegree[node] += 0
		for _, next := range edges[node] {
			indegree[next]++
		}
	}
	done := make(map[string]bool, len(nodes))
	order = make([]string, 0, len(nodes))
	for len(order) < len(nodes) {
		ready := make([]string, 0, len(nodes))
		for _, node := range nodes {
			if !done[node] && indegree[node] == 0 {
				ready = append(ready, node)
			}
		}
		if len(ready) == 0 {
			return order, false
		}
		chosen := pick(ready, order)
		done[chosen] = true
		order = append(order, chosen)
		for _, next := range edges[chosen] {
			indegree[next]--
		}
	}
	return order, true
}

func lowestIndexPick(ready, _ []string) string { return ready[0] }

func buildReorderGraph(members []string, arts map[string]*models.Artifact) (*reorderGraph, error) {
	graph := &reorderGraph{
		members:   members,
		index:     make(map[string]int, len(members)),
		parentOf:  make(map[string]string, len(members)),
		rootEdges: map[string][]string{},
		edges:     map[string][]string{},
	}
	isRoot := make(map[string]bool, len(members))
	for position, id := range members {
		graph.index[id] = position
		graph.parentOf[id] = arts[id].ParentID
		if isRootCoveringFeature(arts[id]) {
			isRoot[id] = true
			graph.roots = append(graph.roots, id)
		} else {
			graph.nonRoots = append(graph.nonRoots, id)
		}
	}

	// Phase 1: hard blocks edges across every member.
	for _, id := range members {
		for _, dep := range arts[id].Dependencies {
			if dep.Type != "" && dep.Type != "blocks" {
				continue
			}
			if _, member := graph.index[dep.ID]; !member {
				continue
			}
			if dep.ID == id {
				return nil, fmt.Errorf("item %s depends on itself: %w", id, blerrors.ErrValidation)
			}
			switch {
			case isRoot[id] && !isRoot[dep.ID]:
				return nil, fmt.Errorf("root covering feature %s depends on non-root member %s; roots must lead the manifest: %w",
					id, dep.ID, blerrors.ErrValidation)
			case isRoot[id]:
				appendEdge(graph.rootEdges, dep.ID, id)
			case isRoot[dep.ID]:
				// Roots always lead; the prerequisite is satisfied.
			default:
				appendEdge(graph.edges, dep.ID, id)
			}
		}
	}
	if _, ok := topoOrder(graph.roots, graph.rootEdges, lowestIndexPick); !ok {
		return nil, fmt.Errorf("hard blocks dependencies among root covering features form a cycle: %w", blerrors.ErrValidation)
	}
	if _, ok := topoOrder(graph.nonRoots, graph.edges, lowestIndexPick); !ok {
		return nil, fmt.Errorf("hard blocks dependencies among shipment members form a cycle: %w", blerrors.ErrValidation)
	}

	// Phase 2: admit parent-before-child edges in canonical (parent, child)
	// order unless the parent is already reachable from the child.
	type pair struct{ parent, child string }
	candidates := make([]pair, 0, len(graph.nonRoots))
	inG := make(map[string]bool, len(graph.nonRoots))
	for _, id := range graph.nonRoots {
		inG[id] = true
	}
	for _, id := range graph.nonRoots {
		if parent := graph.parentOf[id]; parent != "" && inG[parent] {
			candidates = append(candidates, pair{parent, id})
		}
	}
	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].parent != candidates[j].parent {
			return candidates[i].parent < candidates[j].parent
		}
		return candidates[i].child < candidates[j].child
	})
	for _, candidate := range candidates {
		if !edgeReachable(graph.edges, candidate.child, candidate.parent) {
			appendEdge(graph.edges, candidate.parent, candidate.child)
		}
	}
	return graph, nil
}

// autoOrder emits the topologically ordered manifest. Ready members whose
// parent is the most recently emitted member (or its nearest ancestor with a
// ready child) win; otherwise manifest order breaks ties.
func (g *reorderGraph) autoOrder() ([]string, error) {
	rootOrder, ok := topoOrder(g.roots, g.rootEdges, lowestIndexPick)
	if !ok {
		return nil, fmt.Errorf("root covering features form a cycle: %w", blerrors.ErrValidation)
	}
	rest, ok := topoOrder(g.nonRoots, g.edges, func(ready, emitted []string) string {
		if len(emitted) > 0 {
			current := emitted[len(emitted)-1]
			for depth := 0; current != "" && depth <= len(g.members); depth++ {
				for _, candidate := range ready {
					if g.parentOf[candidate] == current {
						return candidate
					}
				}
				current = g.parentOf[current]
			}
		}
		return ready[0]
	})
	if !ok {
		return nil, fmt.Errorf("shipment member dependency graph has a cycle: %w", blerrors.ErrValidation)
	}
	return append(rootOrder, rest...), nil
}

// validateExplicit checks a requested order against the manifest and G.
func (g *reorderGraph) validateExplicit(items []string) error {
	if len(items) != len(g.members) {
		return fmt.Errorf("items list has %d entries but the manifest has %d: %w",
			len(items), len(g.members), blerrors.ErrValidation)
	}
	position := make(map[string]int, len(items))
	for pos, id := range items {
		if _, member := g.index[id]; !member {
			return fmt.Errorf("item %q is not in the shipment manifest: %w", id, blerrors.ErrValidation)
		}
		if _, duplicate := position[id]; duplicate {
			return fmt.Errorf("item %q is listed more than once: %w", id, blerrors.ErrValidation)
		}
		position[id] = pos
	}
	for _, root := range g.roots {
		if position[root] >= len(g.roots) {
			return fmt.Errorf("root covering feature %s must lead the manifest: %w", root, blerrors.ErrValidation)
		}
	}
	for _, edges := range []map[string][]string{g.rootEdges, g.edges} {
		for from, targets := range edges {
			for _, to := range targets {
				if position[from] >= position[to] {
					return fmt.Errorf("item %s must precede %s: %w", from, to, blerrors.ErrValidation)
				}
			}
		}
	}
	return nil
}

type reorderPlan struct {
	mode   string
	before []string
	after  []string
}

func planReorder(
	ctx context.Context,
	ws *Workspace,
	shipment *models.Artifact,
	opts ShipmentManifestOptions,
) (*reorderPlan, error) {
	if err := validateShipmentManifestQueueGuard(shipment, opts, "reorder manifest"); err != nil {
		return nil, err
	}
	before := NormalizeShipmentItems(shipment)
	arts := make(map[string]*models.Artifact, len(before))
	for _, id := range before {
		member, err := findArtifact(ctx, ws, id)
		if err != nil {
			return nil, fmt.Errorf("load shipment member %s: %w", id, err)
		}
		arts[id] = member
	}
	graph, err := buildReorderGraph(before, arts)
	if err != nil {
		return nil, err
	}
	plan := &reorderPlan{before: before}
	if opts.Auto {
		plan.mode = shipmentReorderModeAuto
		if plan.after, err = graph.autoOrder(); err != nil {
			return nil, err
		}
		return plan, nil
	}
	plan.mode = shipmentReorderModeExplicit
	if err := graph.validateExplicit(opts.Items); err != nil {
		return nil, err
	}
	plan.after = append(make([]string, 0, len(opts.Items)), opts.Items...)
	return plan, nil
}

// ReorderShipment rewrites a shipment manifest order, either to an explicit
// permutation (opts.Items) or to a deterministic dependency-respecting order
// (opts.Auto). Queued shipments need no override, other non-blocked shipments
// need Force and Reason, and blocked shipments are always refused. The live
// path runs under a durable reorder_items journal.
func ReorderShipment(
	ctx context.Context,
	ws *Workspace,
	shipmentID string,
	opts ShipmentManifestOptions,
) (*ReorderShipmentResult, error) {
	if ws == nil {
		return nil, fmt.Errorf("reorder shipment workspace is nil: %w", blerrors.ErrValidation)
	}
	if strings.TrimSpace(shipmentID) == "" {
		return nil, fmt.Errorf("shipment id is required: %w", blerrors.ErrValidation)
	}
	if (len(opts.Items) > 0) == opts.Auto {
		return nil, fmt.Errorf("exactly one of items or auto is required: %w", blerrors.ErrValidation)
	}
	if err := validateShipmentManifestOptions(opts); err != nil {
		return nil, err
	}
	reason := strings.TrimSpace(opts.Reason)
	result := &ReorderShipmentResult{ShipmentID: shipmentID, DryRun: opts.DryRun, Force: opts.Force, Reason: reason}

	if opts.DryRun {
		if err := ensureNoPendingShipmentRecoveryReadOnly(ws); err != nil {
			return nil, err
		}
		shipment, err := readShipmentFromDiskReadOnly(ws, shipmentID)
		if err != nil {
			return nil, err
		}
		plan, err := planReorder(ctx, ws, shipment, opts)
		if err != nil {
			return nil, err
		}
		result.Status, result.Mode = "dry_run", plan.mode
		result.ItemsBefore, result.ItemsAfter = plan.before, plan.after
		return result, nil
	}

	lockedCtx, globalUnlock, err := lockShipmentLifecycleGlobal(ctx, ws)
	if err != nil {
		return nil, fmt.Errorf("reorder shipment %s: lock shipment lifecycle: %w", shipmentID, err)
	}
	defer func() {
		if unlockErr := globalUnlock(); unlockErr != nil {
			slog.WarnContext(ctx, "release shipment lifecycle lock after reorder", "shipment_id", shipmentID, "error", unlockErr)
		}
	}()
	ctx = lockedCtx

	membershipUnlock, err := lockShipmentMembership(ctx, ws, shipmentID)
	if err != nil {
		return nil, fmt.Errorf("lock shipment %s membership: %w", shipmentID, err)
	}
	defer func() {
		if unlockErr := membershipUnlock(); unlockErr != nil {
			slog.WarnContext(ctx, "release shipment membership lock", "shipment_id", shipmentID, "error", unlockErr)
		}
	}()

	if _, err := GetShipment(ctx, ws, shipmentID); err != nil {
		return nil, err
	}
	lockedCtx, artifactUnlock, err := lockArtifactMutations(ctx, ws, []string{shipmentID})
	if err != nil {
		return nil, fmt.Errorf("lock shipment %s reorder aggregate: %w", shipmentID, err)
	}
	defer func() {
		if unlockErr := artifactUnlock(); unlockErr != nil {
			slog.WarnContext(ctx, "release shipment reorder locks", "shipment_id", shipmentID, "error", unlockErr)
		}
	}()
	ctx = lockedCtx

	shipment, err := readShipmentFromDiskReadOnly(ws, shipmentID)
	if err != nil {
		return nil, err
	}
	plan, err := planReorder(ctx, ws, shipment, opts)
	if err != nil {
		return nil, err
	}
	memberPreimages, err := manifestMemberPreimages(ctx, ws, plan.before)
	if err != nil {
		return nil, err
	}
	correlationID, err := newShipmentManifestCorrelationID()
	if err != nil {
		return nil, err
	}
	updated := cloneArtifact(shipment)
	updated.CustomFields["items"] = plan.after
	updated.UpdatedAt = models.NowUTC()

	mutation := &manifestMutation{
		operation:     shipmentOperationReorderItems,
		eventType:     EventShipmentReordered,
		correlationID: correlationID,
		reason:        reason,
		journal: shipmentLifecycleJournal{
			SchemaVersion:  "shipment-operation/v1",
			CorrelationID:  correlationID,
			Phase:          shipmentOperationPhaseIntent,
			Operation:      shipmentOperationReorderItems,
			RecoveryPolicy: shipmentOperationRecoveryPolicyRollback,
			ShipmentID:     shipmentID,
			Target:         shipmentReorderTarget,
			Reason:         reason,
			Preimage: shipmentLifecyclePreimage{
				Shipment: cloneArtifact(shipment),
				Members:  memberPreimages,
			},
		},
		shipments:         []*models.Artifact{updated},
		shipmentPreimages: []*models.Artifact{cloneArtifact(shipment)},
		deltas: []map[string]any{{
			"shipment_id":  shipmentID,
			"mode":         plan.mode,
			"items_before": plan.before,
			"items_after":  plan.after,
		}},
		force: opts.Force,
	}
	if err := mutation.run(ctx, ws); err != nil {
		return nil, err
	}
	result.Status, result.Mode, result.CorrelationID = "reordered", plan.mode, correlationID
	result.ItemsBefore, result.ItemsAfter = plan.before, plan.after
	return result, nil
}
