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
		var rel *relation // built once per unit: two graph walks, not one per prior owner
		for _, d := range u.Deliverables {
			key := normalizePath(d)
			if key == "" || reported[key] {
				continue
			}
			// Mark the key processed even when no conflict is found, so a
			// unit repeating a path is neither rescanned nor re-appended.
			reported[key] = true
			for _, prior := range owners[key] {
				if prior == u {
					continue
				}
				if rel == nil {
					rel = newRelation(ctx, u)
				}
				if !rel.with(prior) {
					out = append(out, errFinding(ruleDuplicatedWork, u.ID, "deliverable %q is already delivered by unrelated unit %s", d, prior.ID))
					break
				}
			}
			owners[key] = append(owners[key], u)
		}
	}
	return out, nil
}
