package soundness

import (
	"fmt"
)

// checkContext carries the immutable inputs shared by every rule.
type checkContext struct {
	plan Plan
	cfg  Config
	byID map[string]*Unit
}

// rule is one independently flaggable soundness check.
type rule struct {
	id    string
	check func(*checkContext) ([]Finding, error)
}

// allRules returns the registered rules in stable execution order.
func allRules() []rule {
	return []rule{
		{ruleBudget, checkBudget},
		{ruleOneResponsibility, checkOneResponsibility},
		{ruleReferences, checkReferences},
		{ruleVerificationSurface, checkVerificationSurface},
	}
}

func errFinding(rule, unit, format string, args ...any) Finding {
	return Finding{Rule: rule, UnitID: unit, Severity: SeverityError, Message: fmt.Sprintf(format, args...)}
}

// Run is the single soundness engine entrypoint. It validates cfg, runs the
// always-on structural check and every enabled rule over plan, and returns the
// findings. A rule that fails to evaluate is reported as an error finding
// rather than being skipped, so an unverifiable unit never passes. A
// non-nil error means the run itself was misconfigured.
func Run(plan Plan, cfg Config) (Report, error) {
	rules := allRules()
	known := make(map[string]bool, len(rules))
	for _, r := range rules {
		known[r.id] = true
	}
	if err := cfg.validate(known); err != nil {
		return Report{}, fmt.Errorf("run soundness engine: %w", err)
	}

	ctx := &checkContext{plan: plan, cfg: cfg, byID: map[string]*Unit{}}
	report := Report{Findings: []Finding{}, RulesRun: []string{ruleUnverifiable}}
	report.Findings = append(report.Findings, structuralFindings(ctx)...)

	for _, r := range rules {
		if !cfg.enabled(r.id) {
			report.RulesSkipped = append(report.RulesSkipped, r.id)
			continue
		}
		report.RulesRun = append(report.RulesRun, r.id)
		found, err := r.check(ctx)
		if err != nil {
			found = append(found, errFinding(r.id, "", "rule could not be evaluated: %v", err))
		}
		report.Findings = append(report.Findings, found...)
	}
	return report, nil
}

// structuralFindings flags units the engine cannot reason about: missing or
// duplicate IDs and unknown kinds. It also indexes units by ID on ctx.
func structuralFindings(ctx *checkContext) []Finding {
	var out []Finding
	for i := range ctx.plan.Units {
		u := &ctx.plan.Units[i]
		switch {
		case u.ID == "":
			out = append(out, errFinding(ruleUnverifiable, "", "unit #%d has no id", i+1))
			continue
		case ctx.byID[u.ID] != nil:
			out = append(out, errFinding(ruleUnverifiable, u.ID, "duplicate unit id %q", u.ID))
			continue
		}
		ctx.byID[u.ID] = u
		switch u.Kind {
		case KindPlan, KindFeature, KindTask, KindSubtask:
		default:
			out = append(out, errFinding(ruleUnverifiable, u.ID, "unknown unit kind %q", u.Kind))
		}
	}
	return out
}
