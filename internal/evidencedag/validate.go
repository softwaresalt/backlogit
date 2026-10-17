package evidencedag

import (
	"fmt"
	"sort"
	"strings"

	"github.com/softwaresalt/backlogit/internal/faultline"
)

const maxViolations = 50

// Violation is one deterministic declaration defect.
type Violation struct {
	Code    string
	Subject string
	Detail  string
}

// ValidationError aggregates declaration violations in deterministic order.
type ValidationError struct {
	Violations []Violation
}

// Error implements error.
func (e *ValidationError) Error() string {
	parts := make([]string, 0, len(e.Violations))
	for _, v := range e.Violations {
		parts = append(parts, fmt.Sprintf("[%s] %s: %s", v.Code, v.Subject, v.Detail))
	}
	return ErrInvalidDeclaration.Error() + ": " + joinSorted(parts)
}

// Unwrap lets errors.Is classify the error as ErrInvalidDeclaration.
func (e *ValidationError) Unwrap() error { return ErrInvalidDeclaration }

// evidenceFamilyFor maps a DAG family to the S4 U4 evidence node_family that
// must back its terminal verification. Families without a registered S4
// payload cannot be verified and evaluate as unverifiable.
func evidenceFamilyFor(f Family) (string, bool) {
	switch f {
	case FamilyParity:
		return faultline.NodeFamilyParity, true
	case FamilyRedTestHonesty:
		return faultline.NodeFamilyRedGreen, true
	default:
		return "", false
	}
}

// Validate checks the declaration deterministically and fails closed. It
// reports duplicate IDs, dangling and self edges, cycles (with the cycle
// path), unknown families, missing terminal nodes and nodes unreachable from
// any terminal node.
func (d *Declaration) Validate() error {
	if d == nil {
		return &ValidationError{Violations: []Violation{{"nil", "declaration", "declaration is nil"}}}
	}
	if d.SchemaVersion != SchemaVersion {
		return fmt.Errorf("%w: got %d, want %d", ErrUnsupportedVersion, d.SchemaVersion, SchemaVersion)
	}
	var vs []Violation
	add := func(code, subject, detail string) { vs = append(vs, Violation{code, subject, detail}) }

	if len(d.Nodes) > MaxNodes {
		add("limits", "nodes", fmt.Sprintf("%d nodes exceeds %d", len(d.Nodes), MaxNodes))
	}
	if len(d.Edges) > MaxEdges {
		add("limits", "edges", fmt.Sprintf("%d edges exceeds %d", len(d.Edges), MaxEdges))
	}
	if len(vs) > 0 {
		return finish(vs)
	}

	ids := make(map[string]Node, len(d.Nodes))
	terminals := 0
	for _, n := range d.Nodes {
		if !validID(n.ID) {
			add("bad_id", truncate(n.ID), "id must match [a-z0-9][a-z0-9._-]* and be at most 96 bytes")
			continue
		}
		if _, dup := ids[n.ID]; dup {
			add("duplicate_id", n.ID, "node id declared more than once")
			continue
		}
		ids[n.ID] = n
		if n.Terminal {
			terminals++
		}
		validateNode(n, add)
	}

	type pair struct{ a, b string }
	seenEdge := map[pair]struct{}{}
	deps := map[string][]string{}
	for _, e := range d.Edges {
		_, okN := ids[e.Node]
		_, okD := ids[e.DependsOn]
		subj := truncate(e.Node + " -> " + e.DependsOn)
		if !okN || !okD {
			add("dangling_edge", subj, "edge references an undeclared node")
			continue
		}
		if e.Node == e.DependsOn {
			add("self_edge", subj, "a node cannot depend on itself")
			continue
		}
		p := pair{e.Node, e.DependsOn}
		if _, dup := seenEdge[p]; dup {
			add("duplicate_edge", subj, "edge declared more than once")
			continue
		}
		seenEdge[p] = struct{}{}
		deps[e.Node] = append(deps[e.Node], e.DependsOn)
	}
	for k := range deps {
		sort.Strings(deps[k])
	}

	for _, c := range findCycles(ids, deps) {
		add("cycle", c[0], strings.Join(c, " -> "))
	}
	if terminals == 0 {
		add("no_terminal", "declaration", "at least one terminal node is required")
	} else {
		reach := map[string]bool{}
		var queue []string
		for id, n := range ids {
			if n.Terminal {
				reach[id] = true
				queue = append(queue, id)
			}
		}
		for len(queue) > 0 {
			cur := queue[0]
			queue = queue[1:]
			for _, dep := range deps[cur] {
				if !reach[dep] {
					reach[dep] = true
					queue = append(queue, dep)
				}
			}
		}
		for id := range ids {
			if !reach[id] {
				add("unreachable_node", id, "node is not reachable from any terminal node")
			}
		}
	}
	return finish(vs)
}

func finish(vs []Violation) error {
	if len(vs) == 0 {
		return nil
	}
	sort.Slice(vs, func(i, j int) bool {
		a, b := vs[i], vs[j]
		if a.Code != b.Code {
			return a.Code < b.Code
		}
		if a.Subject != b.Subject {
			return a.Subject < b.Subject
		}
		return a.Detail < b.Detail
	})
	if len(vs) > maxViolations {
		vs = append(vs[:maxViolations], Violation{"truncated", "declaration", "further violations omitted"})
	}
	return &ValidationError{Violations: vs}
}

func validateNode(n Node, add func(code, subject, detail string)) {
	if !knownFamily(n.Family) {
		add("unknown_family", n.ID, fmt.Sprintf("family %q is not a known family", truncate(string(n.Family))))
	}
	if n.WaiverFamily != "" && !knownFamily(n.WaiverFamily) {
		add("unknown_family", n.ID, fmt.Sprintf("waiver_family %q is not a known family", truncate(string(n.WaiverFamily))))
	}
	validateApplicability(n, add)
	if n.Validator == "" || len(n.Validator) > MaxIDLength {
		add("validator", n.ID, "validator reference is required and at most 96 bytes")
	}
	if !taskPattern.MatchString(n.ProducingTask) {
		add("bad_task", n.ID, "producing_task must be a canonical backlog task ID")
	}
	if n.ProducingCommit != CommitHead && !faultline.ValidCommitSHA(n.ProducingCommit) {
		add("bad_commit", n.ID, `producing_commit must be "head" or a full lowercase commit SHA`)
	}
	if n.OnUnmet != OnUnmetBlock && n.OnUnmet != OnUnmetReport {
		add("on_unmet", n.ID, `on_unmet must be "block" or "report"`)
	}
	if n.RequiredEvidence != "" {
		want, ok := evidenceFamilyFor(n.Family)
		if !ok || want != n.RequiredEvidence {
			add("evidence_family", n.ID, "required_evidence does not match the S4 evidence family bound to this node family")
		}
	}
}

func validateApplicability(n Node, add func(code, subject, detail string)) {
	a := n.Applicability
	switch {
	case a == nil:
		add("applicability", n.ID, "applicability must be declared explicitly")
	case a.Always && len(a.Requires) > 0:
		add("applicability", n.ID, "applicability must set either always or requires, not both")
	case !a.Always && len(a.Requires) == 0:
		add("applicability", n.ID, "applicability must set always or a non-empty requires")
	case len(a.Requires) > MaxRequiresKeys:
		add("applicability", n.ID, "requires has too many keys")
	default:
		for k, vals := range a.Requires {
			if k == "" || len(k) > MaxIDLength || len(vals) == 0 || len(vals) > MaxRequiresValues {
				add("applicability", n.ID, "requires keys and value sets must be non-empty and bounded")
				return
			}
			for _, v := range vals {
				if len(v) > MaxIDLength {
					add("applicability", n.ID, "requires value too long")
					return
				}
			}
		}
	}
}

// findCycles returns each distinct cycle once, rotated to start at its
// smallest node ID, using an iterative DFS in sorted order (no recursion).
func findCycles(nodes map[string]Node, deps map[string][]string) [][]string {
	ids := make([]string, 0, len(nodes))
	for id := range nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	const (
		white = iota
		gray
		black
	)
	color := map[string]int{}
	seen := map[string]struct{}{}
	var out [][]string
	type frame struct {
		id  string
		idx int
	}
	for _, root := range ids {
		if color[root] != white {
			continue
		}
		stack := []frame{{root, 0}}
		color[root] = gray
		for len(stack) > 0 {
			top := &stack[len(stack)-1]
			if top.idx >= len(deps[top.id]) {
				color[top.id] = black
				stack = stack[:len(stack)-1]
				continue
			}
			next := deps[top.id][top.idx]
			top.idx++
			switch color[next] {
			case white:
				color[next] = gray
				stack = append(stack, frame{next, 0})
			case gray:
				start := 0
				for i, f := range stack {
					if f.id == next {
						start = i
						break
					}
				}
				cyc := make([]string, 0, len(stack)-start)
				for _, f := range stack[start:] {
					cyc = append(cyc, f.id)
				}
				cyc = rotateToMin(cyc)
				key := strings.Join(cyc, ">")
				if _, dup := seen[key]; !dup && len(out) < 10 {
					seen[key] = struct{}{}
					out = append(out, append(cyc, cyc[0]))
				}
			}
		}
	}
	return out
}

func rotateToMin(c []string) []string {
	m := 0
	for i := range c {
		if c[i] < c[m] {
			m = i
		}
	}
	return append(append([]string{}, c[m:]...), c[:m]...)
}
