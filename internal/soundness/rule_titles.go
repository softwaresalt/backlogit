package soundness

const ruleUniqueTitles = "unique-titles"

// checkUniqueTitles flags units sharing a normalized title unless they are
// intentionally related (dependency, parent/child or related edge).
func checkUniqueTitles(ctx *checkContext) ([]Finding, error) {
	var out []Finding
	firstByTitle := map[string][]*Unit{}
	for i := range ctx.plan.Units {
		u := &ctx.plan.Units[i]
		if ctx.byID[u.ID] != u {
			continue
		}
		key := normalizeText(u.Title)
		if key == "" {
			continue
		}
		var rel *relation
		for _, prior := range firstByTitle[key] {
			if rel == nil {
				rel = newRelation(ctx, u)
			}
			if !rel.with(prior) {
				out = append(out, errFinding(ruleUniqueTitles, u.ID, "title duplicates unrelated unit %s: %q", prior.ID, u.Title))
				break
			}
		}
		firstByTitle[key] = append(firstByTitle[key], u)
	}
	return out, nil
}
