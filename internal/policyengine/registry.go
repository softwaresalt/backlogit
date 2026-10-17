package policyengine

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/softwaresalt/backlogit/internal/evidencedag"
)

// maxRegistryNodes bounds the node list before any per-node work. The closed
// type set has six members, so anything near this cap is already invalid.
const maxRegistryNodes = 64

const (
	maxRegistryViolations = 24
	validationTaskID      = "000-T"
	gateEnabled           = "enabled"
	gateDisabled          = "disabled"
)

type gateDoc struct {
	ID          string `json:"id" yaml:"id"`
	Enabled     *bool  `json:"enabled" yaml:"enabled"`
	Overridable *bool  `json:"overridable" yaml:"overridable"`
}

type registryDoc struct {
	SchemaVersion int                `json:"schema_version" yaml:"schema_version"`
	Mode          string             `json:"mode" yaml:"mode"`
	Nodes         []gateDoc          `json:"nodes" yaml:"nodes"`
	Edges         []evidencedag.Edge `json:"edges" yaml:"edges"`
}

// registryError is a bounded registry defect that still unwraps to the core
// error that detected it.
type registryError struct {
	msg   string
	cause error
}

func (e *registryError) Error() string { return ErrInvalidRegistry.Error() + ": " + e.msg }

func (e *registryError) Unwrap() []error {
	if e.cause == nil {
		return []error{ErrInvalidRegistry}
	}
	return []error{ErrInvalidRegistry, e.cause}
}

func invalidRegistry(cause error, format string, args ...any) error {
	return &registryError{msg: clip(fmt.Sprintf(format, args...), maxErrorText), cause: cause}
}

// Registry is a validated, immutable workflow-policy registry.
type Registry struct {
	mode  Mode
	gates []GateDecl
	edges []evidencedag.Edge
}

// LoadRegistry decodes and validates a workflow-policy registry (JSON or
// YAML). Decoding is strict (see evidencedag.DecodeStrict). The registry must
// declare each of the six node types exactly once with explicit enabled and
// overridable flags, an explicit mode, the canonical ordering edges, and an
// enabled terminal merge-ready gate; the resulting graph is then validated by
// the shared evidencedag core (duplicate, dangling, self and cyclic edges,
// reachability from the terminal node). Any defect is an error: an ambiguous
// registry never loads.
func LoadRegistry(data []byte) (*Registry, error) {
	var doc registryDoc
	if err := evidencedag.DecodeStrict(data, &doc); err != nil {
		return nil, invalidRegistry(err, "%s", err.Error())
	}
	var vs []string
	add := func(format string, args ...any) {
		if len(vs) < maxRegistryViolations {
			vs = append(vs, clip(fmt.Sprintf(format, args...), MaxErrorBytes))
		}
	}
	if doc.SchemaVersion != SchemaVersion {
		return nil, invalidRegistry(evidencedag.ErrUnsupportedVersion, "schema_version %d is not supported (want %d)", doc.SchemaVersion, SchemaVersion)
	}
	mode := Mode(doc.Mode)
	if mode != ModeReportOnly && mode != ModeEnforce {
		add("mode must be %q or %q", ModeReportOnly, ModeEnforce)
	}
	if len(doc.Nodes) == 0 {
		add("the graph declares no nodes")
		return nil, invalidRegistry(nil, "%s", strings.Join(vs, "; "))
	}
	if len(doc.Nodes) > maxRegistryNodes || len(doc.Edges) > MaxRegistryEdges {
		return nil, invalidRegistry(nil, "graph exceeds %d nodes or %d edges", maxRegistryNodes, MaxRegistryEdges)
	}

	known := map[NodeType]struct{}{}
	for _, nt := range NodeTypes() {
		known[nt] = struct{}{}
	}
	seen := map[NodeType]GateDecl{}
	for _, n := range doc.Nodes {
		id := NodeType(n.ID)
		if _, ok := known[id]; !ok {
			add("unknown node type %q", clip(n.ID, 64))
			continue
		}
		if _, dup := seen[id]; dup {
			add("node %s is declared more than once", id)
			continue
		}
		if n.Enabled == nil || n.Overridable == nil {
			add("node %s must declare enabled and overridable explicitly", id)
			continue
		}
		if *n.Overridable && id != NodeHarnessScaffold && id != NodeRedPhaseVerify {
			add("node %s must not be overridable: only %s and %s may be (P-002.1)", id, NodeHarnessScaffold, NodeRedPhaseVerify)
		}
		seen[id] = GateDecl{ID: id, Enabled: *n.Enabled, Overridable: *n.Overridable}
	}
	gates := make([]GateDecl, 0, len(seen))
	for _, nt := range NodeTypes() {
		g, ok := seen[nt]
		if !ok {
			add("node %s is not declared", nt)
			continue
		}
		gates = append(gates, g)
	}
	if g, ok := seen[NodeMergeReady]; ok && !g.Enabled {
		add("the terminal node %s cannot be disabled", NodeMergeReady)
	}

	declared := map[evidencedag.Edge]struct{}{}
	for _, e := range doc.Edges {
		declared[e] = struct{}{}
	}
	order := NodeTypes()
	for i := 1; i < len(order); i++ {
		want := evidencedag.Edge{Node: string(order[i]), DependsOn: string(order[i-1])}
		if _, ok := declared[want]; !ok {
			add("required ordering edge %s -> %s is not declared", want.Node, want.DependsOn)
		}
	}
	if len(vs) > 0 {
		return nil, invalidRegistry(nil, "%s", strings.Join(vs, "; "))
	}

	r := &Registry{mode: mode, gates: gates, edges: append([]evidencedag.Edge(nil), doc.Edges...)}
	sort.Slice(r.edges, func(i, j int) bool {
		if r.edges[i].Node != r.edges[j].Node {
			return r.edges[i].Node < r.edges[j].Node
		}
		return r.edges[i].DependsOn < r.edges[j].DependsOn
	})
	d, err := r.Declaration(validationTaskID)
	if err != nil {
		return nil, invalidRegistry(err, "%s", err.Error())
	}
	if err := d.Validate(); err != nil {
		return nil, invalidRegistry(err, "%s", err.Error())
	}
	return r, nil
}

// Mode returns the registry's declared enforcement mode.
func (r *Registry) Mode() Mode { return r.mode }

// Gates returns a copy of the gate declarations in canonical order.
func (r *Registry) Gates() []GateDecl { return append([]GateDecl(nil), r.gates...) }

// Gate returns the declaration for id.
func (r *Registry) Gate(id NodeType) (GateDecl, bool) {
	for _, g := range r.gates {
		if g.ID == id {
			return g, true
		}
	}
	return GateDecl{}, false
}

// GateAttribute is the shipment attribute key that carries a gate's rollout
// state to the core applicability predicate.
func GateAttribute(id NodeType) string { return "gate." + string(id) }

// Declaration builds the evidencedag declaration for one task: one
// workflow-policy node per gate whose applicability is keyed off the per-gate
// rollout attribute, and the registry's ordering edges.
func (r *Registry) Declaration(taskID string) (*evidencedag.Declaration, error) {
	if !evidencedag.ValidTaskID(taskID) {
		return nil, errors.New("policyengine: task id must be a canonical backlog task ID")
	}
	d := &evidencedag.Declaration{
		SchemaVersion: evidencedag.SchemaVersion,
		Nodes:         make([]evidencedag.Node, 0, len(r.gates)),
		Edges:         append([]evidencedag.Edge{}, r.edges...),
	}
	for _, g := range r.gates {
		d.Nodes = append(d.Nodes, evidencedag.Node{
			ID:     string(g.ID),
			Family: evidencedag.FamilyWorkflowPolicy,
			Applicability: &evidencedag.Applicability{
				Requires: map[string][]string{GateAttribute(g.ID): {gateEnabled}},
			},
			Validator:       ValidatorIdentity,
			ProducingTask:   taskID,
			ProducingCommit: evidencedag.CommitHead,
			OnUnmet:         evidencedag.OnUnmetBlock,
			Terminal:        g.ID == NodeMergeReady,
		})
	}
	return d, nil
}

// predecessors returns the gates id directly depends on, in canonical order.
func (r *Registry) predecessors(id NodeType) []NodeType {
	var out []NodeType
	for _, g := range r.gates {
		for _, e := range r.edges {
			if e.Node == string(id) && e.DependsOn == string(g.ID) {
				out = append(out, g.ID)
			}
		}
	}
	return out
}

// gateAttributes returns the per-gate rollout attributes for a shipment.
func (r *Registry) gateAttributes() map[string]string {
	attrs := make(map[string]string, len(r.gates))
	for _, g := range r.gates {
		if g.Enabled {
			attrs[GateAttribute(g.ID)] = gateEnabled
		} else {
			attrs[GateAttribute(g.ID)] = gateDisabled
		}
	}
	return attrs
}
