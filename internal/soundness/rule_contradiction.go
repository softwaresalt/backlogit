package soundness

import "fmt"

const (
	ruleContradiction = "contradiction"
	// maxSiblingContradictionFindings bounds aggregated sibling findings so a
	// plan with many opposing siblings cannot produce quadratic output.
	maxSiblingContradictionFindings = 5
	// maxDecisionContradictionFindings bounds unit-vs-decision findings the same way.
	maxDecisionContradictionFindings = 5
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
	// holders records, per stance, the first decision and the count of distinct decisions.
	type holders struct {
		first [2]string
		n     [2]int
		last  [2]string
	}
	validStance := func(s string) bool { return s == StanceRequire || s == StanceForbid }

	decided := map[string]*holders{}
	for _, d := range ctx.plan.Decisions {
		for _, dir := range d.Directives {
			if !validStance(dir.Stance) {
				return nil, fmt.Errorf("decision %q has unknown stance %q", d.ID, dir.Stance)
			}
			s := normalizeText(dir.Subject)
			h := decided[s]
			if h == nil {
				h = &holders{}
				decided[s] = h
			}
			i := stanceIndex(dir.Stance)
			if h.last[i] == d.ID && h.n[i] > 0 {
				continue
			}
			h.last[i] = d.ID
			if h.n[i] == 0 {
				h.first[i] = d.ID
			}
			h.n[i]++
		}
	}

	groups := map[string]*siblingGroup{}
	var order []*siblingGroup
	decisionEmitted, decisionOmitted := 0, false
	for _, u := range ctx.plan.Units {
		seenDirective := map[string]bool{}
		for _, dir := range u.Directives {
			if !validStance(dir.Stance) {
				out = append(out, errFinding(ruleContradiction, u.ID, "unknown directive stance %q on %q", dir.Stance, dir.Subject))
				continue
			}
			s := normalizeText(dir.Subject)
			if dk := s + "\x00" + dir.Stance; seenDirective[dk] {
				continue
			} else {
				seenDirective[dk] = true
			}
			if h := decided[s]; h != nil {
				opp := 1 - stanceIndex(dir.Stance)
				if h.n[opp] > 0 {
					switch {
					case decisionEmitted < maxDecisionContradictionFindings:
						decisionEmitted++
						msg := "%s %q contradicts decision %s (%s)"
						args := []any{dir.Stance, dir.Subject, h.first[opp], stanceName(opp)}
						if h.n[opp] > 1 {
							msg += "; %d decisions oppose it"
							args = append(args, h.n[opp])
						}
						out = append(out, errFinding(ruleContradiction, u.ID, msg, args...))
					case !decisionOmitted:
						decisionOmitted = true
						out = append(out, errFinding(ruleContradiction, "", "additional unit-vs-decision contradictions omitted after %d findings", decisionEmitted))
					}
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

func stanceIndex(stance string) int {
	if stance == StanceForbid {
		return 1
	}
	return 0
}

func stanceName(i int) string {
	if i == 1 {
		return StanceForbid
	}
	return StanceRequire
}
