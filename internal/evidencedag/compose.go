package evidencedag

import (
	"errors"
	"fmt"
)

// Compose merges policy-node contributions (for example S11 policy nodes) into
// base and returns a new, validated declaration; base is never mutated. This
// is the documented extension point: contributed terminal nodes join the one
// authoritative terminal set that Gate evaluates, so no contributor can gate
// review-ready independently. A contribution may add nodes and edges whose
// source node it contributes; it cannot add, remove or alter edges of nodes it
// does not own, so it cannot rewire the base DAG. Contributed nodes record their Source, a source may be supplied only once, and a contributed terminal cannot weaken a base node: blocking semantics propagate downstream from unmet upstream nodes.
func Compose(base *Declaration, contribs ...Contribution) (*Declaration, error) {
	if base == nil {
		return nil, fmt.Errorf("%w: nil base declaration", ErrInvalidDeclaration)
	}
	merged := cloneDeclaration(base)
	sources := map[string]struct{}{}
	for _, c := range contribs {
		if !validID(c.Source) {
			return nil, fmt.Errorf("%w: contribution source %q is invalid", ErrInvalidDeclaration, truncate(c.Source))
		}
		if _, dup := sources[c.Source]; dup {
			return nil, fmt.Errorf("%w: contribution source %q is supplied more than once", ErrInvalidDeclaration, c.Source)
		}
		sources[c.Source] = struct{}{}
		if len(c.Nodes) == 0 {
			return nil, fmt.Errorf("%w: contribution %s declares no nodes", ErrInvalidDeclaration, c.Source)
		}
		own := make(map[string]struct{}, len(c.Nodes))
		for _, n := range c.Nodes {
			if n.Source != "" && n.Source != c.Source {
				return nil, fmt.Errorf("%w: contribution %s declares node %q with a different source", ErrInvalidDeclaration, c.Source, truncate(n.ID))
			}
			own[n.ID] = struct{}{}
			cn := cloneNode(n)
			cn.Source = c.Source
			merged.Nodes = append(merged.Nodes, cn)
		}
		for _, e := range c.Edges {
			if _, ok := own[e.Node]; !ok {
				return nil, fmt.Errorf("%w: contribution %s adds an edge from node %q that it does not own", ErrInvalidDeclaration, c.Source, truncate(e.Node))
			}
			merged.Edges = append(merged.Edges, e)
		}
		if len(merged.Nodes) > MaxNodes || len(merged.Edges) > MaxEdges {
			return nil, errors.New("evidencedag: composed declaration exceeds node or edge bounds")
		}
	}
	if err := merged.Validate(); err != nil {
		return nil, fmt.Errorf("evidencedag: composed declaration: %w", err)
	}
	return merged, nil
}
