package soundness

const ruleMissingArtifact = "missing-artifact"

// checkMissingArtifact flags required artifacts that neither exist in the
// workspace nor are delivered by another unit. Without a workspace root an
// artifact that no unit delivers cannot be verified and is flagged.
func checkMissingArtifact(ctx *checkContext) ([]Finding, error) {
	var out []Finding
	delivered := map[string]map[string]bool{} // artifact -> delivering unit IDs
	for _, u := range ctx.plan.Units {
		for _, d := range u.Deliverables {
			k := normalizePath(d)
			if delivered[k] == nil {
				delivered[k] = map[string]bool{}
			}
			delivered[k][u.ID] = true
		}
	}
	for _, u := range ctx.plan.Units {
		for _, req := range u.Requires {
			producers := delivered[normalizePath(req)]
			if len(producers) > 1 || (len(producers) == 1 && !producers[u.ID]) {
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
