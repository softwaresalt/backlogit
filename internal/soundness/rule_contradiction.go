package soundness

import "fmt"

const (
	ruleContradiction = "contradiction"
	// maxSiblingContradictionFindings bounds aggregated sibling findings so a
	// plan with many opposing siblings cannot produce quadratic output.
	maxSiblingContradictionFindings = 5
)

// siblingGroup accumulates the stances declared on one subject by the children
// of one parent, so conflicts are counted in O(n) rather than enumerated.
type siblingGroup struct {
	parent, subject string
	units           map[string]int // unit ID -> stance bitmask (1 require, 2 forbid)
	req, forbid     []string       // first two units holding each stance
	nReq, nForbid   int
	nBoth           int
}

func (g *siblingGroup) add(id, stance string) {
	bit := 1
	if stance == StanceForbid {
		bit = 2
	}
	prev := g.units[id]
	if prev&bit != 0 {
		return
	}
	g.units[id] = prev | bit
	if prev != 0 {
		g.nBoth++
	}
	if bit == 1 {
		g.nReq++
		if len(g.req) < 2 {
			g.req = append(g.req, id)
		}
		return
	}
	g.nForbid++
	if len(g.forbid) < 2 {
		g.forbid = append(g.forbid, id)
	}
}

// pairs returns the number of conflicting sibling pairs and a representative one.
func (g *siblingGroup) pairs() (n int, a, b string) {
	n = g.nReq*g.nForbid - g.nBoth
	for _, r := range g.req {
		for _, f := range g.forbid {
			if r != f {
				return n, r, f
			}
		}
	}
	return n, "", ""
}

// checkContradiction flags units whose directives oppose an originating
// decision, or oppose a sibling unit's directive on the same subject. An
// unknown stance cannot be evaluated and is flagged.
func checkContradiction(ctx *checkContext) ([]Finding, error) {
	var out []Finding
	type holder struct{ id, stance string }
	validStance := func(s string) bool { return s == StanceRequire || s == StanceForbid }

	decided := map[string][]holder{}
	for _, d := range ctx.plan.Decisions {
		for _, dir := range d.Directives {
			if !validStance(dir.Stance) {
				return nil, fmt.Errorf("decision %q has unknown stance %q", d.ID, dir.Stance)
			}
			s := normalizeText(dir.Subject)
			decided[s] = append(decided[s], holder{d.ID, dir.Stance})
		}
	}

	groups := map[string]*siblingGroup{}
	var order []*siblingGroup
	for _, u := range ctx.plan.Units {
		for _, dir := range u.Directives {
			if !validStance(dir.Stance) {
				out = append(out, errFinding(ruleContradiction, u.ID, "unknown directive stance %q on %q", dir.Stance, dir.Subject))
				continue
			}
			s := normalizeText(dir.Subject)
			for _, h := range decided[s] {
				if h.stance != dir.Stance {
					out = append(out, errFinding(ruleContradiction, u.ID, "%s %q contradicts decision %s (%s)", dir.Stance, dir.Subject, h.id, h.stance))
				}
			}
			if u.ParentID == "" {
				continue
			}
			key := u.ParentID + "\x00" + s
			g := groups[key]
			if g == nil {
				g = &siblingGroup{parent: u.ParentID, subject: dir.Subject, units: map[string]int{}}
				groups[key] = g
				order = append(order, g)
			}
			g.add(u.ID, dir.Stance)
		}
	}
	emitted := 0
	for _, g := range order {
		n, a, b := g.pairs()
		if n <= 0 {
			continue
		}
		if emitted == maxSiblingContradictionFindings {
			out = append(out, errFinding(ruleContradiction, "", "additional sibling contradictions omitted after %d groups", emitted))
			break
		}
		emitted++
		out = append(out, errFinding(ruleContradiction, b, "%q is required by sibling %s but forbidden by sibling %s under %s (%d conflicting sibling pairs)", g.subject, a, b, g.parent, n))
	}
	return out, nil
}
