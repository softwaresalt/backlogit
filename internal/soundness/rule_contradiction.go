package soundness

import "fmt"

const ruleContradiction = "contradiction"

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

	siblings := map[string]map[string][]holder{} // parent -> subject -> holders
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
			if siblings[u.ParentID] == nil {
				siblings[u.ParentID] = map[string][]holder{}
			}
			for _, h := range siblings[u.ParentID][s] {
				if h.stance != dir.Stance && h.id != u.ID {
					out = append(out, errFinding(ruleContradiction, u.ID, "%s %q contradicts sibling %s (%s)", dir.Stance, dir.Subject, h.id, h.stance))
				}
			}
			siblings[u.ParentID][s] = append(siblings[u.ParentID][s], holder{u.ID, dir.Stance})
		}
	}
	return out, nil
}
