package soundness

const ruleDuplicatedWork = "duplicated-work"

// checkDuplicatedWork flags units that deliver the same artifact as an
// unrelated unit. A unit extending its dependency or parent is intentional.
func checkDuplicatedWork(ctx *checkContext) ([]Finding, error) {
	var out []Finding
	owners := map[string][]*Unit{}
	for i := range ctx.plan.Units {
		u := &ctx.plan.Units[i]
		if ctx.byID[u.ID] != u {
			continue
		}
		reported := map[string]bool{}
		for _, d := range u.Deliverables {
			key := normalizePath(d)
			if key == "" || reported[key] {
				continue
			}
			for _, prior := range owners[key] {
				if prior != u && !related(prior, u) {
					out = append(out, errFinding(ruleDuplicatedWork, u.ID, "deliverable %q is already delivered by unrelated unit %s", d, prior.ID))
					reported[key] = true
					break
				}
			}
			owners[key] = append(owners[key], u)
		}
	}
	return out, nil
}
