package core

import (
	"fmt"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/softwaresalt/backlogit/internal/models"
)

// manifestShipment is a non-terminal shipment with its parsed manifest state.
type manifestShipment struct {
	id       string
	status   string
	items    []string
	labels   []string
	blocksOn []string // blocks-type dependency IDs
}

func isBlocksDependency(edge models.DependencyEdge) bool {
	return edge.Type == "" || edge.Type == "blocks"
}

func pathUnderArchive(path string) bool {
	parts := strings.Split(filepath.ToSlash(path), "/")
	// The final element is the file name; any directory named archive marks it.
	for _, p := range parts[:len(parts)-1] {
		if p == "archive" {
			return true
		}
	}
	return false
}

// detectShipmentManifestFindings inspects non-terminal shipments (queued,
// active, blocked; never under archive/) for manifest integrity problems:
// dangling members, duplicate membership, features split across shipments,
// forward shipment dependencies, and (advisory) unsequenced queued shipments.
// It is read-only.
func detectShipmentManifestFindings(refs map[string][]artifactRef, ids []string) ([]DoctorFinding, error) {
	shipments := make(map[string]*manifestShipment)
	shipmentOrder := make([]string, 0)
	for _, id := range ids {
		group := refs[id]
		if len(group) == 0 {
			continue
		}
		ref := group[0]
		if ref.artifactType != "shipment" || pathUnderArchive(ref.path) {
			continue
		}
		switch ref.status {
		case string(ShipmentQueued), string(ShipmentActive), string(ShipmentBlocked):
		default:
			continue
		}
		artifact, _, err := parseFile(ref.path)
		if err != nil {
			return nil, fmt.Errorf("parse shipment %s: %w", id, err)
		}
		ms := &manifestShipment{
			id:     id,
			status: ref.status,
			items:  NormalizeShipmentItems(artifact),
			labels: artifact.Labels,
		}
		for _, dep := range artifact.Dependencies {
			if isBlocksDependency(dep) {
				ms.blocksOn = append(ms.blocksOn, dep.ID)
			}
		}
		shipments[id] = ms
		shipmentOrder = append(shipmentOrder, id)
	}

	findings := make([]DoctorFinding, 0)
	membership := make(map[string][]string) // item -> shipments
	for _, sid := range shipmentOrder {
		ms := shipments[sid]
		seen := make(map[string]struct{}, len(ms.items))
		for _, item := range ms.items {
			if _, dup := seen[item]; dup {
				continue
			}
			seen[item] = struct{}{}
			membership[item] = append(membership[item], sid)
			if len(refs[item]) == 0 {
				findings = append(findings, newDoctorErrorFinding(FindingDanglingShipmentMember, sid,
					fmt.Sprintf("shipment %q lists member %q that exists in neither queue/ nor archive/", sid, item)))
			}
		}
	}

	items := make([]string, 0, len(membership))
	for item := range membership {
		items = append(items, item)
	}
	sort.Strings(items)
	for _, item := range items {
		if sids := membership[item]; len(sids) > 1 {
			findings = append(findings, newDoctorErrorFinding(FindingDuplicateShipmentMembership, item,
				fmt.Sprintf("item %q appears in more than one non-terminal shipment: %s", item, strings.Join(sids, ", "))))
		}
	}

	// Split features: group members by owning feature.
	featureShipments := make(map[string]map[string]struct{})
	for _, item := range items {
		feature := owningFeatureID(refs, item)
		if feature == "" {
			continue
		}
		if featureShipments[feature] == nil {
			featureShipments[feature] = make(map[string]struct{})
		}
		for _, sid := range membership[item] {
			featureShipments[feature][sid] = struct{}{}
		}
	}
	features := make([]string, 0, len(featureShipments))
	for feature := range featureShipments {
		features = append(features, feature)
	}
	sort.Strings(features)
	for _, feature := range features {
		set := featureShipments[feature]
		if len(set) < 2 {
			continue
		}
		sids := make([]string, 0, len(set))
		for sid := range set {
			sids = append(sids, sid)
		}
		sort.Strings(sids)
		findings = append(findings, newDoctorErrorFinding(FindingSplitFeatureAcrossShipments, feature,
			fmt.Sprintf("members of feature %q are split across non-terminal shipments: %s", feature, strings.Join(sids, ", "))))
	}

	forward, err := detectForwardShipmentDependencies(refs, shipments, shipmentOrder, membership)
	if err != nil {
		return nil, err
	}
	findings = append(findings, forward...)
	findings = append(findings, detectUnsequencedQueuedShipments(refs, shipments, shipmentOrder)...)

	sort.SliceStable(findings, func(i, j int) bool {
		if findings[i].Type != findings[j].Type {
			return findings[i].Type < findings[j].Type
		}
		return findings[i].ArtifactID < findings[j].ArtifactID
	})
	return findings, nil
}

// owningFeatureID resolves the feature that owns id: a feature owns itself;
// tasks and subtasks resolve through their parent_id chain.
func owningFeatureID(refs map[string][]artifactRef, id string) string {
	current := id
	for hops := 0; hops < 16; hops++ {
		group := refs[current]
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

// shipmentReaches reports whether from transitively depends on to through the
// shipment-level blocks DAG.
func shipmentReaches(shipments map[string]*manifestShipment, from, to string) bool {
	visited := map[string]struct{}{}
	stack := []string{from}
	for len(stack) > 0 {
		cur := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if _, ok := visited[cur]; ok {
			continue
		}
		visited[cur] = struct{}{}
		ms := shipments[cur]
		if ms == nil {
			continue
		}
		for _, dep := range ms.blocksOn {
			if dep == to {
				return true
			}
			stack = append(stack, dep)
		}
	}
	return false
}

func detectForwardShipmentDependencies(
	refs map[string][]artifactRef,
	shipments map[string]*manifestShipment,
	shipmentOrder []string,
	membership map[string][]string,
) ([]DoctorFinding, error) {
	findings := make([]DoctorFinding, 0)
	reported := make(map[string]struct{})
	for _, s1 := range shipmentOrder {
		for _, item := range shipments[s1].items {
			group := refs[item]
			if len(group) == 0 {
				continue
			}
			artifact, _, err := parseFile(group[0].path)
			if err != nil {
				return nil, fmt.Errorf("parse shipment member %s: %w", item, err)
			}
			for _, dep := range artifact.Dependencies {
				if !isBlocksDependency(dep) {
					continue
				}
				for _, s2 := range membership[dep.ID] {
					if s2 == s1 || !shipmentReaches(shipments, s2, s1) {
						continue
					}
					key := s1 + "|" + s2
					if _, done := reported[key]; done {
						continue
					}
					reported[key] = struct{}{}
					findings = append(findings, newDoctorErrorFinding(FindingForwardShipmentDependency, s1,
						fmt.Sprintf("item %q in shipment %q depends on %q in shipment %q, but %q depends on %q (forward shipment dependency)",
							item, s1, dep.ID, s2, s2, s1)))
				}
			}
		}
	}
	return findings, nil
}

func detectUnsequencedQueuedShipments(
	refs map[string][]artifactRef,
	shipments map[string]*manifestShipment,
	shipmentOrder []string,
) []DoctorFinding {
	queued := make([]string, 0)
	for _, sid := range shipmentOrder {
		if shipments[sid].status == string(ShipmentQueued) {
			queued = append(queued, sid)
		}
	}
	if len(queued) < 2 {
		return nil
	}
	findings := make([]DoctorFinding, 0)
	for _, sid := range queued {
		ms := shipments[sid]
		if slices.Contains(ms.labels, "dag-root") {
			continue
		}
		sequenced := false
		for _, dep := range ms.blocksOn {
			if group := refs[dep]; len(group) > 0 && group[0].artifactType == "shipment" {
				sequenced = true
				break
			}
		}
		if sequenced {
			continue
		}
		// Advisory: no Severity, so report.HasErrors() stays false.
		findings = append(findings, DoctorFinding{
			Type:       FindingUnsequencedQueuedShipment,
			ArtifactID: sid,
			Description: fmt.Sprintf("queued shipment %q has no blocks dependency on another shipment and no dag-root label while %d shipments are queued",
				sid, len(queued)),
		})
	}
	return findings
}
