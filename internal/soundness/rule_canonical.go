package soundness

import "strings"

const ruleCanonicalProcedure = "canonical-procedure"

// checkCanonicalProcedure flags units naming a procedure outside the
// configured canonical set. When a unit names a procedure but no canonical
// set is configured, conformance cannot be verified and the unit is flagged.
func checkCanonicalProcedure(ctx *checkContext) ([]Finding, error) {
	var out []Finding
	for _, u := range ctx.plan.Units {
		if u.Procedure == "" {
			continue
		}
		if len(ctx.cfg.CanonicalProcedures) == 0 {
			out = append(out, errFinding(ruleCanonicalProcedure, u.ID, "procedure %q cannot be verified: no canonical procedures configured", u.Procedure))
			continue
		}
		if !contains(ctx.cfg.CanonicalProcedures, u.Procedure) {
			out = append(out, errFinding(ruleCanonicalProcedure, u.ID, "procedure %q is not canonical (expected one of: %s)",
				u.Procedure, strings.Join(ctx.cfg.CanonicalProcedures, ", ")))
		}
	}
	return out, nil
}
