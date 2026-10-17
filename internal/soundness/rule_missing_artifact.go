package soundness

const ruleMissingArtifact = "missing-artifact"

// checkMissingArtifact flags required artifacts that neither exist in the
// workspace nor are delivered by another unit. Every required and delivered
// path is confined to the workspace before producers are matched, so an
// out-of-root path can never be satisfied by a producer. A requirement met
// only by producers outside the consumer's transitive dependencies yields a
// warning. Without a workspace root an artifact that no unit delivers cannot
// be verified and is flagged.
func checkMissingArtifact(ctx *checkContext) ([]Finding, error) {
	var out []Finding
	delivered := map[string]map[string]bool{} // artifact -> delivering unit IDs
	for _, u := range ctx.plan.Units {
		seenDeliverable := map[string]bool{}
		for _, d := range u.Deliverables {
			if seenDeliverable[d] {
				continue
			}
			seenDeliverable[d] = true
			k, err := confinedArtifactIn(ctx.cfg.Root, d)
			if err != nil {
				out = append(out, errFinding(ruleMissingArtifact, u.ID, "delivered artifact %q is invalid: %v", d, err))
				continue
			}
			if delivered[k] == nil {
				delivered[k] = map[string]bool{}
			}
			delivered[k][u.ID] = true
		}
	}
	for _, u := range ctx.plan.Units {
		var deps map[string]bool // computed once per unit, only when needed
		seenRequire := map[string]bool{}
		for _, req := range u.Requires {
			if seenRequire[req] {
				continue
			}
			seenRequire[req] = true
			k, err := confinedArtifactIn(ctx.cfg.Root, req)
			if err != nil {
				out = append(out, errFinding(ruleMissingArtifact, u.ID, "required artifact %q is invalid: %v", req, err))
				continue
			}
			var producers []string
			for id := range delivered[k] {
				if id != u.ID {
					producers = append(producers, id)
				}
			}
			if len(producers) > 0 {
				if deps == nil {
					deps = transitiveDependencies(ctx, u.ID)
				}
				ordered := false
				for _, id := range producers {
					ordered = ordered || deps[id]
				}
				if !ordered {
					out = append(out, warnFinding(ruleMissingArtifact, u.ID, "required artifact %q is delivered only by units that are not among its dependencies", req))
				}
				continue
			}
			if ctx.cfg.Root == "" {
				out = append(out, errFinding(ruleMissingArtifact, u.ID, "required artifact %q is not delivered by any unit and cannot be verified: no workspace root configured", req))
				continue
			}
			if err := resolveReference(ctx.cfg.Root, req); err != nil {
				out = append(out, errFinding(ruleMissingArtifact, u.ID, "required artifact %q is missing: %v", req, err))
			}
		}
	}
	return out, nil
}
