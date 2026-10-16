package soundness

import (
	"regexp"
	"strings"
)

const ruleIndependentVerifiability = "independent-verifiability"

var vagueCriterion = regexp.MustCompile(`(?i)\b(works?|working|properly|correctly|as expected|appropriate(ly)?|etc|good|nice|reasonable|as needed|should work)\b`)

// checkIndependentVerifiability flags units whose acceptance cannot be
// verified on its own: no acceptance criteria, vague criteria, or
// verification that relies on a unit not declared as a dependency.
func checkIndependentVerifiability(ctx *checkContext) ([]Finding, error) {
	var out []Finding
	for _, u := range ctx.plan.Units {
		if len(u.Acceptance) == 0 {
			out = append(out, errFinding(ruleIndependentVerifiability, u.ID, "no acceptance criteria"))
		}
		for _, c := range u.Acceptance {
			if strings.TrimSpace(c) == "" {
				out = append(out, errFinding(ruleIndependentVerifiability, u.ID, "empty acceptance criterion"))
			} else if m := vagueCriterion.FindString(c); m != "" {
				out = append(out, errFinding(ruleIndependentVerifiability, u.ID, "acceptance criterion %q is not objectively verifiable (vague term %q)", c, m))
			}
		}
		for _, other := range u.VerifiedWith {
			if other != u.ID && !contains(u.Dependencies, other) {
				out = append(out, errFinding(ruleIndependentVerifiability, u.ID, "verification relies on %s, which is not a declared dependency", other))
			}
		}
	}
	return out, nil
}
