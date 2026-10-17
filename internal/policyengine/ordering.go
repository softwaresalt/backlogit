package policyengine

import (
	"context"
	"time"

	"github.com/softwaresalt/backlogit/internal/evidencedag"
)

// checkOrdering enforces the declared prerequisite ordering between the
// well-formed evidence of adjacent gates: the successor's commit must descend
// from its predecessor's and it must not be recorded earlier. It reads direct
// edges only; transitive propagation of failures is the core's.
func (e *Engine) checkOrdering(ctx context.Context, facts map[NodeType]fact) {
	base := make(map[NodeType]fact, len(facts))
	for id, f := range facts {
		base[id] = f
	}
	// Untimed derived facts inherit the latest timestamp of their timestamped
	// ancestors so ordering is still compared across them.
	visiting := map[NodeType]bool{}
	var effective func(id NodeType) (time.Time, bool)
	effective = func(id NodeType) (time.Time, bool) {
		f, ok := base[id]
		if !ok || !f.ok {
			return time.Time{}, false
		}
		if f.hasAt {
			return f.at, true
		}
		if visiting[id] {
			return time.Time{}, false
		}
		visiting[id] = true
		defer delete(visiting, id)
		var latest time.Time
		found := false
		for _, p := range e.reg.predecessors(id) {
			if at, ok := effective(p); ok && (!found || at.After(latest)) {
				latest, found = at, true
			}
		}
		return latest, found
	}
	for id, f := range base {
		if at, ok := effective(id); ok && !f.hasAt {
			f.at, f.hasAt = at, true
			base[id] = f
		}
	}
	for _, g := range e.reg.Gates() {
		n, ok := base[g.ID]
		if !ok || !n.ok {
			continue
		}
		for _, p := range e.reg.predecessors(g.ID) {
			pf, ok := base[p]
			if !ok || !pf.ok {
				continue
			}
			if bad, violated := e.orderViolation(ctx, p, pf, n); violated {
				facts[g.ID] = bad
				break
			}
		}
	}
}

// orderViolation compares one gate's evidence with its predecessor's.
func (e *Engine) orderViolation(ctx context.Context, pred NodeType, pf, n fact) (fact, bool) {
	if pf.commit != n.commit {
		anc, err := e.opts.Git.IsAncestor(ctx, pf.commit, n.commit)
		if err != nil {
			return failed(evidencedag.StatusError, CodeSHAUnverifiable, "the ordering against %s cannot be verified against git history", pred), true
		}
		if !anc {
			return failed(evidencedag.StatusError, CodeOutOfOrder, "the evidence commit does not descend from the %s evidence commit", pred), true
		}
	}
	if n.hasAt && pf.hasAt && n.at.Before(pf.at) {
		return failed(evidencedag.StatusError, CodeOutOfOrder, "the evidence was recorded before the %s evidence", pred), true
	}
	return fact{}, false
}
