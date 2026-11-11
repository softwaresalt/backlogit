package core

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/softwaresalt/backlogit/internal/canonical"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

const (
	reconstituteMaxPlanBytes = 1 << 20
	reconstitutePrevToken    = "prev"
	reconstituteDagRootLabel = "dag-root"
)

var reconstituteLabelPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*$`)

func reconstituteValidationError(format string, args ...any) error {
	return fmt.Errorf("reconstitute: %s: %w", fmt.Sprintf(format, args...), blerrors.ErrValidation)
}

// UnmarshalYAML accepts a scalar existing-feature ID ("195-F"), an
// {id: "195-F"} mapping, or a {create: {...}} mapping.
func (f *ReconstituteFeatureSpec) UnmarshalYAML(node *yaml.Node) error {
	for node.Kind == yaml.AliasNode && node.Alias != nil {
		node = node.Alias
	}
	*f = ReconstituteFeatureSpec{}
	switch node.Kind {
	case yaml.ScalarNode:
		var id string
		if err := node.Decode(&id); err != nil {
			return fmt.Errorf("decode feature id: %w", err)
		}
		f.ID = strings.TrimSpace(id)
		return nil
	case yaml.MappingNode:
		for i := 0; i+1 < len(node.Content); i += 2 {
			key, value := node.Content[i], node.Content[i+1]
			switch key.Value {
			case "id":
				if err := value.Decode(&f.ID); err != nil {
					return fmt.Errorf("decode feature id: %w", err)
				}
				f.ID = strings.TrimSpace(f.ID)
			case "create":
				if err := rejectUnknownCreateFeatureKeys(value); err != nil {
					return err
				}
				var create ReconstituteCreateFeatureSpec
				if err := value.Decode(&create); err != nil {
					return fmt.Errorf("decode feature create: %w", err)
				}
				f.Create = &create
			default:
				return fmt.Errorf("line %d: unknown feature field %q", key.Line, key.Value)
			}
		}
		return nil
	default:
		return fmt.Errorf("line %d: feature must be an ID or a mapping", node.Line)
	}
}

func rejectUnknownCreateFeatureKeys(node *yaml.Node) error {
	for node.Kind == yaml.AliasNode && node.Alias != nil {
		node = node.Alias
	}
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("line %d: feature create must be a mapping", node.Line)
	}
	for i := 0; i+1 < len(node.Content); i += 2 {
		switch node.Content[i].Value {
		case "title", "priority", "description", "related_to":
		default:
			return fmt.Errorf("line %d: unknown feature create field %q", node.Content[i].Line, node.Content[i].Value)
		}
	}
	return nil
}

func rejectDuplicateYAMLKeys(node *yaml.Node) error {
	switch node.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, child := range node.Content {
			if err := rejectDuplicateYAMLKeys(child); err != nil {
				return err
			}
		}
	case yaml.MappingNode:
		seen := make(map[string]struct{}, len(node.Content)/2)
		for i := 0; i+1 < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Kind == yaml.ScalarNode {
				if _, duplicate := seen[key.Value]; duplicate {
					return fmt.Errorf("line %d: duplicate mapping key %q", key.Line, key.Value)
				}
				seen[key.Value] = struct{}{}
			}
			if err := rejectDuplicateYAMLKeys(node.Content[i+1]); err != nil {
				return err
			}
		}
	}
	return nil
}

// readReconstitutePlanFile reads a plan file that resolves inside the real
// workspace root. Every component below the root is checked with Lstat so an
// intermediate or leaf symlink / reparse point is refused.
func readReconstitutePlanFile(ws *Workspace, planPath string) ([]byte, error) {
	realRoot, err := filepath.EvalSymlinks(ws.RootPath)
	if err != nil {
		return nil, reconstituteValidationError("resolve workspace root: %v", err)
	}
	absRoot, err := filepath.Abs(ws.RootPath)
	if err != nil {
		return nil, reconstituteValidationError("resolve workspace root: %v", err)
	}
	candidate := planPath
	if !filepath.IsAbs(candidate) {
		candidate = filepath.Join(absRoot, candidate)
	}
	candidate = filepath.Clean(candidate)

	rel := ""
	for _, base := range []string{realRoot, absRoot} {
		if pathContained(base, candidate) {
			rel, err = filepath.Rel(base, candidate)
			if err != nil {
				return nil, reconstituteValidationError("resolve plan path: %v", err)
			}
			break
		}
	}
	if rel == "" || rel == "." {
		return nil, reconstituteValidationError("plan path %q is outside the workspace root", planPath)
	}

	current := realRoot
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, statErr := os.Lstat(current)
		if statErr != nil {
			return nil, reconstituteValidationError("inspect plan path component %q: %v", current, statErr)
		}
		link, linkErr := IsSymlinkOrReparsePoint(info, current)
		if linkErr != nil {
			return nil, reconstituteValidationError("inspect plan path component %q: %v", current, linkErr)
		}
		if link {
			return nil, reconstituteValidationError("plan path component %q is a symlink or reparse point", current)
		}
	}
	resolved, err := filepath.EvalSymlinks(current)
	if err != nil {
		return nil, reconstituteValidationError("resolve plan path: %v", err)
	}
	if !pathContained(realRoot, resolved) {
		return nil, reconstituteValidationError("plan path %q resolves outside the workspace root", planPath)
	}
	info, err := os.Stat(resolved)
	if err != nil {
		return nil, reconstituteValidationError("stat plan file: %v", err)
	}
	if !info.Mode().IsRegular() || info.Size() > reconstituteMaxPlanBytes {
		return nil, reconstituteValidationError("plan file must be a regular file of at most %d bytes", reconstituteMaxPlanBytes)
	}
	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, reconstituteValidationError("read plan file: %v", err)
	}
	return data, nil
}

// loadAndValidateReconstitutePlan parses the plan from opts with strict YAML
// rules (known fields only, no duplicate keys) and validates its shape.
func loadAndValidateReconstitutePlan(ws *Workspace, opts ReconstituteOptions) (*ReconstitutePlan, string, error) {
	hasPath, hasYAML := strings.TrimSpace(opts.PlanPath) != "", strings.TrimSpace(opts.PlanYAML) != ""
	if hasPath == hasYAML {
		return nil, "", reconstituteValidationError("exactly one of plan_path or plan_yaml is required")
	}
	var data []byte
	if hasPath {
		var err error
		if data, err = readReconstitutePlanFile(ws, opts.PlanPath); err != nil {
			return nil, "", fmt.Errorf("reconstitute: load plan: %w", err)
		}
	} else {
		if len(opts.PlanYAML) > reconstituteMaxPlanBytes {
			return nil, "", reconstituteValidationError("plan_yaml exceeds %d bytes", reconstituteMaxPlanBytes)
		}
		data = []byte(opts.PlanYAML)
	}

	var doc yaml.Node
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	if err := decoder.Decode(&doc); err != nil {
		return nil, "", fmt.Errorf("reconstitute: parse plan: %w: %w", blerrors.ErrValidation, err)
	}
	if err := decoder.Decode(&yaml.Node{}); !errors.Is(err, io.EOF) {
		if err != nil {
			return nil, "", fmt.Errorf("reconstitute: plan must contain exactly one YAML document: %w: %w", blerrors.ErrValidation, err)
		}
		return nil, "", reconstituteValidationError("plan must contain exactly one YAML document")
	}
	if err := rejectDuplicateYAMLKeys(&doc); err != nil {
		return nil, "", fmt.Errorf("reconstitute: parse plan: %w: %w", blerrors.ErrValidation, err)
	}
	var plan ReconstitutePlan
	strict := yaml.NewDecoder(bytes.NewReader(data))
	strict.KnownFields(true)
	if err := strict.Decode(&plan); err != nil {
		return nil, "", fmt.Errorf("reconstitute: parse plan: %w: %w", blerrors.ErrValidation, err)
	}
	if len(plan.SourceShipments) == 0 {
		return nil, "", reconstituteValidationError("source_shipments must not be empty")
	}
	if len(plan.Shipments) == 0 {
		return nil, "", reconstituteValidationError("shipments must not be empty")
	}
	seenSources := make(map[string]struct{}, len(plan.SourceShipments))
	for _, raw := range plan.SourceShipments {
		id := strings.TrimSpace(raw)
		if id == "" {
			return nil, "", reconstituteValidationError("source_shipments contains an empty ID")
		}
		if !validReconstituteID(id) || strings.EqualFold(id, shipmentLifecycleGlobalLockID) {
			return nil, "", reconstituteValidationError("source shipment ID %q is invalid", id)
		}
		foldKey := strings.ToLower(id)
		if _, duplicate := seenSources[foldKey]; duplicate {
			return nil, "", reconstituteValidationError("source shipment %s is listed more than once", id)
		}
		seenSources[foldKey] = struct{}{}
	}
	return &plan, canonical.HashBytes(data), nil
}

type reconstituteTarget struct {
	spec            ReconstituteShipmentSpec
	key             string
	existing        *models.Artifact
	title           string
	featureKey      string
	existingFeature *models.Artifact
	items           []string
	blocksOn        []string
	adopt           []string
}

func (t *reconstituteTarget) status() models.ArtifactStatus {
	if t.existing != nil {
		return t.existing.Status
	}
	return models.StatusQueued
}

type reconstituteMemberMove struct {
	sourceID string
	updated  *models.Artifact
	changed  bool
	requeued bool
}

type reconstitutePlanState struct {
	plan        *ReconstitutePlan
	digest      string
	opts        ReconstituteOptions
	sources     map[string]*models.Artifact
	targets     []*reconstituteTarget
	byKey       map[string]*reconstituteTarget
	arts        map[string]*models.Artifact
	moves       map[string]*reconstituteMemberMove
	unassigned  []string
	preds       map[string][]string // shipment key -> final blocks predecessor keys
	edgeAdds    map[string][]string
	edgeRemoves map[string][]string
	rewireItems []reconstituteItemRewire
	dagRoots    []string
	manifests   map[string][]string // source shipment ID -> original manifest

	outsideQueuedCount int
}

type reconstituteItemRewire struct {
	item     string
	blocksOn []string
}

type reconstituteWorld struct {
	refs         map[string][]artifactRef
	storageRoot  string
	outsideShips map[string]*models.Artifact // non-terminal shipments outside the plan
	outsideOwner map[string]string           // member item -> outside shipment ID
}

func (st *reconstitutePlanState) readArtifact(ctx context.Context, ws *Workspace, id string) (*models.Artifact, error) {
	if art, ok := st.arts[id]; ok {
		return art, nil
	}
	art, err := findArtifact(ctx, ws, id)
	if err != nil {
		if errors.Is(err, blerrors.ErrNotFound) {
			return nil, reconstituteValidationError("artifact %s does not exist", id)
		}
		return nil, fmt.Errorf("reconstitute: load artifact %s: %w", id, err)
	}
	st.arts[id] = art
	return art, nil
}

// planReconstitution validates the plan against the on-disk workspace without
// writing anything. It is shared by the dry-run preview and the live path.
func planReconstitution(ctx context.Context, ws *Workspace, opts ReconstituteOptions) (*reconstitutePlanState, error) {
	if err := validateShipmentManifestOptions(ShipmentManifestOptions{Force: opts.Force, Reason: opts.Reason}); err != nil {
		return nil, err
	}
	plan, digest, err := loadAndValidateReconstitutePlan(ws, opts)
	if err != nil {
		return nil, err
	}
	st := &reconstitutePlanState{
		plan: plan, digest: digest, opts: opts,
		sources:     map[string]*models.Artifact{},
		byKey:       map[string]*reconstituteTarget{},
		arts:        map[string]*models.Artifact{},
		moves:       map[string]*reconstituteMemberMove{},
		preds:       map[string][]string{},
		edgeAdds:    map[string][]string{},
		edgeRemoves: map[string][]string{},
		manifests:   map[string][]string{},
	}

	if err := st.loadSources(ctx, ws); err != nil {
		return nil, err
	}
	if err := st.buildTargets(ctx, ws); err != nil {
		return nil, err
	}
	world, err := st.loadWorld(ws)
	if err != nil {
		return nil, err
	}
	if err := st.validatePartition(ctx, ws, world); err != nil {
		return nil, err
	}
	if err := st.resolveGraph(ctx, ws, world); err != nil {
		return nil, err
	}
	if err := st.validateFeatureSplit(ctx, ws, world); err != nil {
		return nil, err
	}
	if err := st.validateSequencing(); err != nil {
		return nil, err
	}
	if err := st.requireSourcesReused(); err != nil {
		return nil, err
	}
	if err := st.planMoves(ctx, ws); err != nil {
		return nil, err
	}
	return st, nil
}

func (st *reconstitutePlanState) loadSources(ctx context.Context, ws *Workspace) error {
	guardOpts := ShipmentManifestOptions{Force: st.opts.Force, Reason: st.opts.Reason}
	for _, id := range st.plan.SourceShipments {
		id = strings.TrimSpace(id)
		if id == "" {
			return reconstituteValidationError("source_shipments contains an empty ID")
		}
		if _, duplicate := st.sources[id]; duplicate {
			return reconstituteValidationError("source shipment %s is listed more than once", id)
		}
		shipment, err := readShipmentFromDiskReadOnly(ws, id)
		if err != nil {
			return err
		}
		if err := validateShipmentManifestQueueGuard(shipment, guardOpts, "reconstitute"); err != nil {
			return err
		}
		st.sources[id] = shipment
		st.manifests[id] = NormalizeShipmentItems(shipment)
	}
	return nil
}

func (st *reconstitutePlanState) buildTargets(ctx context.Context, ws *Workspace) error {
	labels := map[string]struct{}{}
	for index, spec := range st.plan.Shipments {
		target := &reconstituteTarget{spec: spec, title: strings.TrimSpace(spec.Title)}
		spec.ID, spec.Label = strings.TrimSpace(spec.ID), strings.TrimSpace(spec.Label)
		target.spec = spec
		if spec.Label != "" {
			if !reconstituteLabelPattern.MatchString(spec.Label) || spec.Label == reconstitutePrevToken {
				return reconstituteValidationError("shipment label %q is invalid", spec.Label)
			}
			if _, duplicate := labels[spec.Label]; duplicate {
				return reconstituteValidationError("shipment label %q is used more than once", spec.Label)
			}
			labels[spec.Label] = struct{}{}
		}
		if spec.ID != "" {
			source, isSource := st.sources[spec.ID]
			if !isSource {
				return reconstituteValidationError("shipment id %s must be listed in source_shipments", spec.ID)
			}
			if _, duplicate := st.byKey[spec.ID]; duplicate {
				return reconstituteValidationError("shipment %s is declared more than once", spec.ID)
			}
			target.existing, target.key = source, spec.ID
			if target.title == "" {
				target.title = source.Title
			}
		} else {
			target.key = "new:" + strconv.Itoa(index)
			if target.title == "" {
				target.title = spec.Label
			}
			if target.title == "" {
				target.title = "Reconstituted shipment " + strconv.Itoa(index+1)
			}
		}

		hasID, hasCreate := spec.Feature.ID != "", spec.Feature.Create != nil
		if hasID == hasCreate {
			return reconstituteValidationError("shipment %d must set exactly one of feature id or feature create", index+1)
		}
		if hasID {
			feature, err := st.readArtifact(ctx, ws, spec.Feature.ID)
			if err != nil {
				return err
			}
			if feature.ArtifactType != "feature" {
				return reconstituteValidationError("covering feature %s is a %s, not a feature", feature.ID, feature.ArtifactType)
			}
			target.featureKey, target.existingFeature = feature.ID, feature
		} else {
			create := spec.Feature.Create
			if strings.TrimSpace(create.Title) == "" {
				return reconstituteValidationError("shipment %d feature create requires a title", index+1)
			}
			if related := strings.TrimSpace(create.RelatedTo); related != "" {
				if _, err := st.readArtifact(ctx, ws, related); err != nil {
					return err
				}
			}
			target.featureKey = "new-feature:" + strconv.Itoa(index)
		}

		if len(spec.Items) == 0 {
			return reconstituteValidationError("shipment %d must list at least one item", index+1)
		}
		seen := map[string]struct{}{}
		for _, raw := range spec.Items {
			itemID := strings.TrimSpace(raw)
			if itemID == "" {
				return reconstituteValidationError("shipment %d lists an empty item ID", index+1)
			}
			if _, duplicate := seen[itemID]; duplicate {
				return reconstituteValidationError("item %s is listed more than once in shipment %d", itemID, index+1)
			}
			seen[itemID] = struct{}{}
			item, err := st.readArtifact(ctx, ws, itemID)
			if err != nil {
				return err
			}
			if item.ArtifactType == "shipment" || item.ArtifactType == "feature" {
				return reconstituteValidationError("item %s is a %s; list covering features under feature", itemID, item.ArtifactType)
			}
			target.items = append(target.items, itemID)
		}
		st.targets = append(st.targets, target)
		st.byKey[target.key] = target
	}
	return nil
}

func (st *reconstitutePlanState) loadWorld(ws *Workspace) (*reconstituteWorld, error) {
	refs, err := scanCanonicalArtifacts(ws)
	if err != nil {
		return nil, fmt.Errorf("reconstitute: scan workspace: %w", err)
	}
	world := &reconstituteWorld{
		refs:         refs,
		storageRoot:  workspaceStorageRoot(ws),
		outsideShips: map[string]*models.Artifact{},
		outsideOwner: map[string]string{},
	}
	for id, group := range refs {
		ref := group[0]
		if ref.artifactType != "shipment" || pathUnderArchive(world.storageRoot, ref.path) ||
			!shipmentStatusBlocksAssignment(models.ArtifactStatus(ref.status)) {
			continue
		}
		if _, isSource := st.sources[id]; isSource {
			continue
		}
		shipment, _, parseErr := parseFile(ref.path)
		if parseErr != nil {
			return nil, fmt.Errorf("reconstitute: parse shipment %s: %w", id, parseErr)
		}
		world.outsideShips[id] = shipment
		if shipment.Status == models.StatusQueued {
			st.outsideQueuedCount++
		}
		for _, member := range NormalizeShipmentItems(shipment) {
			world.outsideOwner[member] = id
		}
	}
	return world, nil
}

func (st *reconstitutePlanState) validatePartition(ctx context.Context, ws *Workspace, world *reconstituteWorld) error {
	assigned := map[string]string{}
	assign := func(id, key string) error {
		if previous, duplicate := assigned[id]; duplicate {
			return reconstituteValidationError("item %s is assigned to more than one shipment (%s and %s)", id, previous, key)
		}
		if owner, taken := world.outsideOwner[id]; taken {
			return fmt.Errorf("reconstitute: item %s already belongs to shipment %s outside source_shipments: %w: %w",
				id, owner, blerrors.ErrItemAlreadyAssigned, blerrors.ErrValidation)
		}
		assigned[id] = key
		return nil
	}
	for _, target := range st.targets {
		if target.existingFeature != nil {
			if err := assign(target.featureKey, target.key); err != nil {
				return err
			}
		}
		for _, itemID := range target.items {
			if err := assign(itemID, target.key); err != nil {
				return err
			}
		}
	}

	sourceMembers := map[string]struct{}{}
	for _, sourceID := range st.plan.SourceShipments {
		for _, member := range st.manifests[strings.TrimSpace(sourceID)] {
			sourceMembers[member] = struct{}{}
		}
	}
	for _, raw := range st.plan.UnassignedItems {
		itemID := strings.TrimSpace(raw)
		if _, member := sourceMembers[itemID]; !member {
			return reconstituteValidationError("unassigned item %s is not a member of any source shipment", itemID)
		}
		if key, duplicate := assigned[itemID]; duplicate {
			return reconstituteValidationError("item %s is both unassigned and assigned to %s", itemID, key)
		}
		st.unassigned = append(st.unassigned, itemID)
	}
	for _, sourceID := range st.plan.SourceShipments {
		for _, member := range st.manifests[strings.TrimSpace(sourceID)] {
			if _, ok := assigned[member]; ok || containsString(st.unassigned, member) {
				continue
			}
			art, err := st.readArtifact(ctx, ws, member)
			if err != nil {
				return err
			}
			if art.ArtifactType == "feature" {
				continue
			}
			return reconstituteValidationError("source member %s of shipment %s is not assigned to any shipment or listed in unassigned_items",
				member, sourceID)
		}
	}
	return nil
}

func (st *reconstitutePlanState) resolveShipmentToken(token string, index int, world *reconstituteWorld) (string, error) {
	token = strings.TrimSpace(token)
	if token == "" {
		return "", reconstituteValidationError("empty shipment reference")
	}
	if token == reconstitutePrevToken {
		if index <= 0 {
			return "", reconstituteValidationError("%q is not valid on the first shipment", reconstitutePrevToken)
		}
		return st.targets[index-1].key, nil
	}
	for _, target := range st.targets {
		if target.spec.Label == token || (target.existing != nil && target.existing.ID == token) {
			return target.key, nil
		}
	}
	if _, outside := world.outsideShips[token]; outside {
		return token, nil
	}
	if group := world.refs[token]; len(group) > 0 && group[0].artifactType == "shipment" {
		return token, nil
	}
	return "", reconstituteValidationError("unknown shipment reference %q", token)
}

func appendUniqueString(values []string, value string) []string {
	if containsString(values, value) {
		return values
	}
	return append(values, value)
}

func (st *reconstitutePlanState) shipmentPreds(world *reconstituteWorld, shipment *models.Artifact) []string {
	preds := []string{}
	for _, dep := range shipment.Dependencies {
		if !isBlocksDependency(dep) {
			continue
		}
		if group := world.refs[dep.ID]; len(group) > 0 && group[0].artifactType == "shipment" {
			preds = appendUniqueString(preds, dep.ID)
		}
	}
	return preds
}

func (st *reconstitutePlanState) resolveGraph(ctx context.Context, ws *Workspace, world *reconstituteWorld) error {
	for _, target := range st.targets {
		if target.existing != nil {
			st.preds[target.key] = st.shipmentPreds(world, target.existing)
		} else {
			st.preds[target.key] = []string{}
		}
	}
	for id, shipment := range world.outsideShips {
		st.preds[id] = st.shipmentPreds(world, shipment)
	}
	for index, target := range st.targets {
		for _, token := range target.spec.BlocksOn {
			key, err := st.resolveShipmentToken(token, index, world)
			if err != nil {
				return err
			}
			if key == target.key {
				return reconstituteValidationError("shipment %s cannot block on itself", target.key)
			}
			target.blocksOn = appendUniqueString(target.blocksOn, key)
			if !containsString(st.preds[target.key], key) {
				st.preds[target.key] = append(st.preds[target.key], key)
				st.edgeAdds[target.key] = append(st.edgeAdds[target.key], key)
			}
		}
	}

	for _, rewire := range st.plan.Rewire {
		subject := strings.TrimSpace(rewire.Item)
		if subject == "" {
			return reconstituteValidationError("rewire item must not be empty")
		}
		if key, err := st.resolveShipmentToken(subject, 0, world); err == nil {
			var desired []string
			for _, token := range rewire.BlocksOn {
				pred, err := st.resolveShipmentToken(token, 0, world)
				if err != nil {
					return err
				}
				if pred == key {
					return reconstituteValidationError("shipment %s cannot block on itself", key)
				}
				desired = appendUniqueString(desired, pred)
			}
			for _, pred := range st.preds[key] {
				if !containsString(desired, pred) {
					st.edgeRemoves[key] = append(st.edgeRemoves[key], pred)
				}
			}
			for _, pred := range desired {
				if !containsString(st.preds[key], pred) {
					st.edgeAdds[key] = appendUniqueString(st.edgeAdds[key], pred)
				}
			}
			st.preds[key] = append([]string{}, desired...)
			continue
		}
		if _, err := st.readArtifact(ctx, ws, subject); err != nil {
			return err
		}
		desired := make([]string, 0, len(rewire.BlocksOn))
		for _, raw := range rewire.BlocksOn {
			dep := strings.TrimSpace(raw)
			if dep == subject {
				return reconstituteValidationError("item %s cannot block on itself", subject)
			}
			if _, err := st.readArtifact(ctx, ws, dep); err != nil {
				return err
			}
			desired = append(desired, dep)
		}
		st.rewireItems = append(st.rewireItems, reconstituteItemRewire{item: subject, blocksOn: desired})
	}

	if err := reconstituteDetectCycle(st.preds); err != nil {
		return err
	}
	return st.validateForwardEdges(ctx, ws, world)
}

func reconstituteDetectCycle(preds map[string][]string) error {
	const (
		visiting = 1
		done     = 2
	)
	state := map[string]int{}
	var visit func(node string) error
	visit = func(node string) error {
		switch state[node] {
		case visiting:
			return reconstituteValidationError("shipment dependency cycle through %s", node)
		case done:
			return nil
		}
		state[node] = visiting
		for _, pred := range preds[node] {
			if err := visit(pred); err != nil {
				return err
			}
		}
		state[node] = done
		return nil
	}
	nodes := make([]string, 0, len(preds))
	for node := range preds {
		nodes = append(nodes, node)
	}
	sort.Strings(nodes)
	for _, node := range nodes {
		if err := visit(node); err != nil {
			return err
		}
	}
	return nil
}

func reconstituteReaches(preds map[string][]string, from, to string) bool {
	seen := map[string]bool{}
	stack := []string{from}
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, pred := range preds[current] {
			if pred == to {
				return true
			}
			if !seen[pred] {
				seen[pred] = true
				stack = append(stack, pred)
			}
		}
	}
	return false
}

// membership maps every member of every post-plan non-terminal shipment to its shipment key.
func (st *reconstitutePlanState) membership(world *reconstituteWorld) map[string]string {
	owner := map[string]string{}
	for _, target := range st.targets {
		if target.existingFeature != nil {
			owner[target.featureKey] = target.key
		}
		for _, item := range target.items {
			owner[item] = target.key
		}
	}
	for item, shipment := range world.outsideOwner {
		owner[item] = shipment
	}
	return owner
}

func (st *reconstitutePlanState) validateForwardEdges(ctx context.Context, ws *Workspace, world *reconstituteWorld) error {
	owner := st.membership(world)
	rewired := map[string][]string{}
	for _, rewire := range st.rewireItems {
		rewired[rewire.item] = rewire.blocksOn
	}
	items := make([]string, 0, len(owner))
	for item := range owner {
		items = append(items, item)
	}
	sort.Strings(items)
	for _, item := range items {
		art, err := st.readArtifact(ctx, ws, item)
		if err != nil {
			if errors.Is(err, blerrors.ErrValidation) {
				continue
			}
			return err
		}
		deps := rewired[item]
		if _, isRewired := rewired[item]; !isRewired {
			deps = nil
			for _, dep := range art.Dependencies {
				if isBlocksDependency(dep) {
					deps = append(deps, dep.ID)
				}
			}
		}
		for _, dep := range deps {
			ownerOfDep, member := owner[dep]
			if !member || ownerOfDep == owner[item] {
				continue
			}
			if reconstituteReaches(st.preds, ownerOfDep, owner[item]) {
				return reconstituteValidationError(
					"forward_shipment_dependency: item %s in shipment %s depends on %s in shipment %s, which depends on %s",
					item, owner[item], dep, ownerOfDep, owner[item])
			}
		}
	}
	return nil
}

func (st *reconstitutePlanState) validateFeatureSplit(ctx context.Context, ws *Workspace, world *reconstituteWorld) error {
	owner := st.membership(world)
	effectiveParent := map[string]string{}
	for _, target := range st.targets {
		for _, item := range target.items {
			art := st.arts[item]
			parent := art.ParentID
			if parent != target.featureKey && st.isAdoptable(ctx, ws, art) {
				parent = target.featureKey
				target.adopt = append(target.adopt, item)
			}
			effectiveParent[item] = parent
		}
	}
	owningFeature := func(id string) string {
		current := id
		for hops := 0; hops < 16; hops++ {
			if parent, ok := effectiveParent[current]; ok {
				if parent == "" {
					return ""
				}
				current = parent
				continue
			}
			if strings.HasPrefix(current, "new-feature:") {
				return current
			}
			group := world.refs[current]
			if len(group) == 0 {
				return ""
			}
			if group[0].artifactType == "feature" {
				return current
			}
			if group[0].parentID == "" {
				return ""
			}
			current = group[0].parentID
		}
		return ""
	}
	featureShipments := map[string]map[string]struct{}{}
	for item, shipment := range owner {
		feature := owningFeature(item)
		if feature == "" {
			continue
		}
		if featureShipments[feature] == nil {
			featureShipments[feature] = map[string]struct{}{}
		}
		featureShipments[feature][shipment] = struct{}{}
	}
	features := make([]string, 0, len(featureShipments))
	for feature := range featureShipments {
		features = append(features, feature)
	}
	sort.Strings(features)
	for _, feature := range features {
		if len(featureShipments[feature]) > 1 {
			return reconstituteValidationError("%s: members of feature %s would be split across shipments",
				FindingSplitFeatureAcrossShipments, feature)
		}
	}
	return nil
}

// isAdoptable reports whether an item sits directly under a feature (or is an
// orphan) and can therefore be re-parented under a covering feature.
func (st *reconstitutePlanState) isAdoptable(ctx context.Context, ws *Workspace, item *models.Artifact) bool {
	if item.ParentID == "" {
		return true
	}
	parent, err := st.readArtifact(ctx, ws, item.ParentID)
	if err != nil {
		return true
	}
	return parent.ArtifactType == "feature"
}

func (st *reconstitutePlanState) validateSequencing() error {
	hasOutgoing := func(key string) bool { return len(st.preds[key]) > 0 }
	queued := st.outsideQueuedCount
	for _, target := range st.targets {
		if target.status() == models.StatusQueued {
			queued++
		}
	}
	if queued < 2 {
		return nil
	}
	var roots []string
	for _, target := range st.targets {
		if target.status() != models.StatusQueued || hasOutgoing(target.key) {
			continue
		}
		roots = append(roots, target.key)
	}
	unlabelled := 0
	for _, key := range roots {
		target := st.byKey[key]
		explicit := target.spec.Label == reconstituteDagRootLabel ||
			(target.existing != nil && containsString(target.existing.Labels, reconstituteDagRootLabel))
		if !explicit {
			unlabelled++
		}
	}
	if unlabelled > 1 {
		return reconstituteValidationError("unsequenced queued shipments %s: add blocks_on edges or label a single root %q",
			strings.Join(roots, ", "), reconstituteDagRootLabel)
	}
	st.dagRoots = roots
	return nil
}

func (st *reconstitutePlanState) requireSourcesReused() error {
	for _, sourceID := range st.plan.SourceShipments {
		sourceID = strings.TrimSpace(sourceID)
		if _, reused := st.byKey[sourceID]; !reused {
			return reconstituteValidationError("source shipment %s must be reused by a target shipment (set id: %s)", sourceID, sourceID)
		}
	}
	return nil
}

func (st *reconstitutePlanState) planMoves(ctx context.Context, ws *Workspace) error {
	destination := map[string]*reconstituteTarget{}
	for _, target := range st.targets {
		for _, item := range target.items {
			destination[item] = target
		}
	}
	for _, sourceID := range st.plan.SourceShipments {
		sourceID = strings.TrimSpace(sourceID)
		for _, member := range st.manifests[sourceID] {
			art, err := st.readArtifact(ctx, ws, member)
			if err != nil {
				return err
			}
			if art.ArtifactType == "feature" {
				continue
			}
			dest := destination[member]
			if dest != nil && dest.existing != nil && dest.existing.ID == sourceID {
				continue
			}
			if err := requireUnstartedClaimedMember(ctx, ws, art, sourceID,
				fmt.Sprintf("reconstitute item %s out of shipment %s", member, sourceID)); err != nil {
				return err
			}
			var destArtifact *models.Artifact
			if dest != nil {
				destArtifact = dest.existing
				if destArtifact == nil {
					destArtifact = &models.Artifact{ID: dest.key, ArtifactType: "shipment", Status: models.StatusQueued}
				}
			}
			updated, changed, requeued := planMemberDetach(art, sourceID, destArtifact)
			st.moves[member] = &reconstituteMemberMove{sourceID: sourceID, updated: updated, changed: changed, requeued: requeued}
		}
	}
	return nil
}

// bumpTypedID returns id with its last typed ordinal advanced by n.
func bumpTypedID(id string, prefix, suffix string, n int) string {
	segment := lastIDSegment(id)
	ordinal, ok := typedSegmentOrdinal(segment, prefix, suffix)
	if !ok {
		return id
	}
	return id[:len(id)-len(segment)] + formatTypedSegment(prefix, suffix, ordinal+n)
}

// predictIDs previews the IDs the next count creations of artifactType under
// parentID would receive, without writing anything.
func predictIDs(ctx context.Context, ws *Workspace, artifactType, parentID string, titles []string) ([]string, error) {
	typeCfg, ok := ws.Config.ArtifactTypes[artifactType]
	if !ok || typeCfg == nil {
		return nil, reconstituteValidationError("unknown artifact type %s", artifactType)
	}
	ids := make([]string, 0, len(titles))
	if ws.Config.QueueLayout != nil {
		if _, levelErr := LevelForType(ws.Config.QueueLayout, artifactType); levelErr == nil {
			base, err := allocateTypedHierarchicalID(ctx, ws, parentID, artifactType, typeCfg, ws.Config.QueueLayout)
			if err != nil {
				return nil, fmt.Errorf("reconstitute: predict %s id: %w", artifactType, err)
			}
			for index := range titles {
				ids = append(ids, bumpTypedID(base, typeCfg.Prefix, typeCfg.Suffix, index))
			}
			return ids, nil
		}
	}
	next, err := NextID(ctx, ws.DB, artifactType, typeCfg)
	if err != nil {
		return nil, fmt.Errorf("reconstitute: predict %s id: %w", artifactType, err)
	}
	if diskMax, diskErr := MaxDiskArtifactSequence(ws, typeCfg); diskErr != nil {
		return nil, fmt.Errorf("reconstitute: predict %s id: %w", artifactType, diskErr)
	} else if diskMax >= next {
		next = diskMax + 1
	}
	for index, title := range titles {
		ids = append(ids, ResolveName(typeCfg, title, next+index, ws.Config.MaxSlugLength))
	}
	return ids, nil
}

// applyReconstituteRewiresToGraph overlays the plan's item rewires onto the
// artifacts used to order a manifest, so ordering reflects the dependencies the
// plan will commit rather than the ones on disk. Rewired artifacts are cloned;
// the caller's originals are never mutated. mapID translates plan IDs to final
// (post-adoption) IDs.
func applyReconstituteRewiresToGraph(arts map[string]*models.Artifact, rewires []reconstituteItemRewire, mapID func(string) string) {
	for _, rewire := range rewires {
		subject := mapID(rewire.item)
		art, ok := arts[subject]
		if !ok {
			continue
		}
		updated := cloneArtifact(art)
		kept := make([]models.DependencyEdge, 0, len(updated.Dependencies)+len(rewire.blocksOn))
		for _, dep := range updated.Dependencies {
			if !isBlocksDependency(dep) {
				kept = append(kept, dep)
			}
		}
		for _, dep := range rewire.blocksOn {
			kept = append(kept, models.DependencyEdge{ID: mapID(dep), Type: "blocks"})
		}
		updated.Dependencies = kept
		arts[subject] = updated
	}
}

// orderReconstituteManifest orders a target manifest: the covering feature
// leads, followed by the items in dependency order (auto) or as listed.
func orderReconstituteManifest(featureID string, itemIDs []string, arts map[string]*models.Artifact, auto bool) ([]string, error) {
	members := append([]string{featureID}, itemIDs...)
	graph, err := buildReorderGraph(members, arts)
	if err != nil {
		return nil, err
	}
	if auto {
		return graph.autoOrder()
	}
	if err := graph.validateExplicit(members); err != nil {
		return nil, err
	}
	return members, nil
}

func (t *reconstituteTarget) autoOrder() bool {
	return !t.spec.ExplicitOrder && (t.spec.AutoOrder == nil || *t.spec.AutoOrder)
}

func reconstituteMapIDs(ids []string, idMap map[string]string) []string {
	mapped := make([]string, 0, len(ids))
	for _, id := range ids {
		if newID, ok := idMap[id]; ok {
			id = newID
		}
		mapped = append(mapped, id)
	}
	return mapped
}

func newReconstituteResult(st *reconstitutePlanState) *ReconstituteShipmentResult {
	sources := make([]string, 0, len(st.plan.SourceShipments))
	for _, id := range st.plan.SourceShipments {
		sources = append(sources, strings.TrimSpace(id))
	}
	return &ReconstituteShipmentResult{
		Force: st.opts.Force, Reason: strings.TrimSpace(st.opts.Reason),
		PlanDigest:        st.digest,
		SourceShipments:   sources,
		CreatedFeatures:   []ReconstituteCreatedArtifact{},
		CreatedShipments:  []ReconstituteCreatedArtifact{},
		TargetShipments:   []string{},
		LabelMap:          map[string]string{},
		AdoptedIDMap:      map[string]string{},
		BeforeManifests:   map[string][]string{},
		AfterManifests:    map[string][]string{},
		DependencyChanges: []ReconstituteDependencyChange{},
	}
}

// projectReconstitution previews the outcome of a validated plan with predicted IDs.
func projectReconstitution(ctx context.Context, ws *Workspace, st *reconstitutePlanState) (*ReconstituteShipmentResult, error) {
	result := newReconstituteResult(st)
	result.Status, result.DryRun = "dry_run", true
	for id, manifest := range st.manifests {
		result.BeforeManifests[id] = append([]string{}, manifest...)
	}

	idOf := map[string]string{}
	var featureTitles, shipmentTitles []string
	var featureTargets, shipmentTargets []*reconstituteTarget
	for _, target := range st.targets {
		idOf[target.key] = target.key
		if target.existing == nil {
			shipmentTargets = append(shipmentTargets, target)
			shipmentTitles = append(shipmentTitles, target.title)
		}
		idOf[target.featureKey] = target.featureKey
		if target.existingFeature == nil {
			featureTargets = append(featureTargets, target)
			featureTitles = append(featureTitles, target.spec.Feature.Create.Title)
		}
	}
	if len(featureTargets) > 0 {
		ids, err := predictIDs(ctx, ws, "feature", "", featureTitles)
		if err != nil {
			return nil, err
		}
		for index, target := range featureTargets {
			idOf[target.featureKey] = ids[index]
			result.CreatedFeatures = append(result.CreatedFeatures, ReconstituteCreatedArtifact{
				ID: ids[index], Label: target.spec.Label, Title: target.spec.Feature.Create.Title,
			})
		}
	}
	if len(shipmentTargets) > 0 {
		ids, err := predictIDs(ctx, ws, "shipment", "", shipmentTitles)
		if err != nil {
			return nil, err
		}
		for index, target := range shipmentTargets {
			idOf[target.key] = ids[index]
			result.CreatedShipments = append(result.CreatedShipments, ReconstituteCreatedArtifact{
				ID: ids[index], Label: target.spec.Label, Title: target.title,
			})
		}
	}

	itemCfg := func(item *models.Artifact) (prefix, suffix string) {
		if cfg := ws.Config.ArtifactTypes[item.ArtifactType]; cfg != nil {
			return cfg.Prefix, cfg.Suffix
		}
		return "", ""
	}
	perParent := map[string]int{}
	for _, target := range st.targets {
		if len(target.adopt) == 0 || target.spec.KeepID {
			continue
		}
		parentID := idOf[target.featureKey]
		if target.existingFeature != nil {
			for _, item := range target.adopt {
				predicted, err := predictIDs(ctx, ws, st.arts[item].ArtifactType, parentID, make([]string, perParent[parentID]+1))
				if err != nil {
					return nil, err
				}
				result.AdoptedIDMap[item] = predicted[perParent[parentID]]
				perParent[parentID]++
			}
			continue
		}
		numeric, err := numericParentPath(parentID, ws.Config.QueueLayout)
		if err != nil {
			return nil, fmt.Errorf("reconstitute: predict adopted ids: %w", err)
		}
		for _, item := range target.adopt {
			prefix, suffix := itemCfg(st.arts[item])
			perParent[parentID]++
			result.AdoptedIDMap[item] = numeric + "." + formatTypedSegment(prefix, suffix, perParent[parentID])
		}
	}

	simulated := map[string]*models.Artifact{}
	simulate := func(id string) *models.Artifact {
		if art, ok := simulated[id]; ok {
			return art
		}
		art := cloneArtifact(st.arts[id])
		art.ID = reconstituteMapIDs([]string{id}, result.AdoptedIDMap)[0]
		for index, dep := range art.Dependencies {
			art.Dependencies[index].ID = reconstituteMapIDs([]string{dep.ID}, result.AdoptedIDMap)[0]
		}
		simulated[id] = art
		return art
	}
	for _, target := range st.targets {
		featureID := idOf[target.featureKey]
		arts := map[string]*models.Artifact{}
		if target.existingFeature != nil {
			arts[featureID] = target.existingFeature
		} else {
			arts[featureID] = &models.Artifact{ID: featureID, ArtifactType: "feature"}
		}
		itemIDs := make([]string, 0, len(target.items))
		for _, item := range target.items {
			art := simulate(item)
			if containsString(target.adopt, item) {
				art.ParentID = featureID
			}
			arts[art.ID] = art
			itemIDs = append(itemIDs, art.ID)
		}
		applyReconstituteRewiresToGraph(arts, st.rewireItems, func(id string) string {
			return reconstituteMapIDs([]string{id}, result.AdoptedIDMap)[0]
		})
		order, err := orderReconstituteManifest(featureID, itemIDs, arts, target.autoOrder())
		if err != nil {
			return nil, err
		}
		id := idOf[target.key]
		result.TargetShipments = append(result.TargetShipments, id)
		result.AfterManifests[id] = order
		if target.spec.Label != "" {
			result.LabelMap[target.spec.Label] = id
		}
	}
	result.DependencyChanges = st.dependencyChanges(func(key string) string {
		if id, ok := idOf[key]; ok {
			return id
		}
		return key
	}, result.AdoptedIDMap)
	return result, nil
}

// dependencyChanges lists the shipment and item dependency edges the plan adds or removes.
func (st *reconstitutePlanState) dependencyChanges(idOf func(string) string, adopted map[string]string) []ReconstituteDependencyChange {
	changes := []ReconstituteDependencyChange{}
	keys := make([]string, 0, len(st.edgeAdds)+len(st.edgeRemoves))
	seen := map[string]bool{}
	for _, target := range st.targets {
		if !seen[target.key] && (len(st.edgeAdds[target.key]) > 0 || len(st.edgeRemoves[target.key]) > 0) {
			seen[target.key] = true
			keys = append(keys, target.key)
		}
	}
	for key := range st.edgeAdds {
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	for key := range st.edgeRemoves {
		if !seen[key] {
			seen[key] = true
			keys = append(keys, key)
		}
	}
	for _, key := range keys {
		change := ReconstituteDependencyChange{Item: idOf(key), Added: []string{}, Removed: []string{}}
		for _, add := range st.edgeAdds[key] {
			change.Added = append(change.Added, idOf(add))
		}
		for _, remove := range st.edgeRemoves[key] {
			change.Removed = append(change.Removed, idOf(remove))
		}
		changes = append(changes, change)
	}
	for _, rewire := range st.rewireItems {
		change := ReconstituteDependencyChange{
			Item:  reconstituteMapIDs([]string{rewire.item}, adopted)[0],
			Added: []string{}, Removed: []string{},
		}
		current := map[string]bool{}
		art := st.arts[rewire.item]
		for _, dep := range art.Dependencies {
			if isBlocksDependency(dep) {
				current[dep.ID] = true
				if !containsString(rewire.blocksOn, dep.ID) {
					change.Removed = append(change.Removed, reconstituteMapIDs([]string{dep.ID}, adopted)[0])
				}
			}
		}
		for _, dep := range rewire.blocksOn {
			if !current[dep] {
				change.Added = append(change.Added, reconstituteMapIDs([]string{dep}, adopted)[0])
			}
		}
		if len(change.Added) > 0 || len(change.Removed) > 0 {
			changes = append(changes, change)
		}
	}
	return changes
}
