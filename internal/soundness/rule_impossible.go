package soundness

import "fmt"

const ruleImpossibleTest = "impossible-test"

// checkImpossibleTest flags planned tests whose assertions cannot all hold:
// one subject equal to two different values, or equal and not equal to the
// same value. An unknown operator cannot be evaluated and is flagged.
func checkImpossibleTest(ctx *checkContext) ([]Finding, error) {
	var out []Finding
	for _, u := range ctx.plan.Units {
		for _, t := range u.Tests {
			if msg := impossibleReason(t); msg != "" {
				out = append(out, errFinding(ruleImpossibleTest, u.ID, "test %q is unsatisfiable: %s", t.Name, msg))
			}
		}
	}
	return out, nil
}

func impossibleReason(t TestSpec) string {
	eq := map[string]string{}
	ne := map[string]map[string]bool{}
	for _, a := range t.Assertions {
		if a.Op != OpEqual && a.Op != OpNotEqual {
			return fmt.Sprintf("unknown operator %q on %q", a.Op, a.Subject)
		}
		s, v := normalizeText(a.Subject), normalizeText(a.Value)
		if a.Op == OpEqual {
			if prev, ok := eq[s]; ok && prev != v {
				return fmt.Sprintf("%q must equal both %q and %q", a.Subject, prev, a.Value)
			}
			eq[s] = v
		} else {
			if ne[s] == nil {
				ne[s] = map[string]bool{}
			}
			ne[s][v] = true
		}
	}
	for s, v := range eq {
		if ne[s][v] {
			return fmt.Sprintf("%q must both equal and not equal %q", s, v)
		}
	}
	return ""
}
