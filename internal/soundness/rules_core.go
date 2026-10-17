package soundness

import (
	"errors"
	"fmt"
	"strings"

	"github.com/softwaresalt/backlogit/internal/docline"
)

// Rule IDs for the core rules.
const (
	ruleBudget              = "budget"
	ruleOneResponsibility   = "one-responsibility"
	ruleReferences          = "references"
	ruleVerificationSurface = "verification-surface"
)

// sizedKind reports whether the size budgets apply to the unit kind.
func sizedKind(kind string) bool { return kind == KindTask || kind == KindSubtask }

// checkBudget flags tasks and subtasks exceeding the configured file,
// function or scenario budgets.
func checkBudget(ctx *checkContext) ([]Finding, error) {
	var out []Finding
	b := ctx.cfg.Budgets
	for _, u := range ctx.plan.Units {
		if !sizedKind(u.Kind) {
			continue
		}
		for _, dim := range []struct {
			name      string
			have, max int
		}{
			{"files", len(u.Files), b.MaxFiles},
			{"functions", len(u.Functions), b.MaxFunctions},
			{"scenarios", len(u.Scenarios), b.MaxScenarios},
		} {
			if dim.have > dim.max {
				out = append(out, errFinding(ruleBudget, u.ID, "%s budget exceeded: %d > %d", dim.name, dim.have, dim.max))
			}
		}
	}
	return out, nil
}

// checkOneResponsibility flags tasks and subtasks that declare more
// responsibilities than the configured limit.
func checkOneResponsibility(ctx *checkContext) ([]Finding, error) {
	var out []Finding
	for _, u := range ctx.plan.Units {
		if sizedKind(u.Kind) && len(u.Responsibilities) > ctx.cfg.Budgets.MaxResponsibilities {
			out = append(out, errFinding(ruleOneResponsibility, u.ID, "unit has %d responsibilities (%s); limit is %d",
				len(u.Responsibilities), strings.Join(u.Responsibilities, ", "), ctx.cfg.Budgets.MaxResponsibilities))
		}
	}
	return out, nil
}

// checkReferences resolves every unit reference through the shared
// workspace-confined docline resolver. Without a workspace root, references
// cannot be verified and are reported as errors (fail closed).
func checkReferences(ctx *checkContext) ([]Finding, error) {
	var out []Finding
	for _, u := range ctx.plan.Units {
		for _, ref := range u.References {
			if ctx.cfg.Root == "" {
				out = append(out, errFinding(ruleReferences, u.ID, "reference %q cannot be verified: no workspace root configured", ref))
				continue
			}
			if err := resolveReference(ctx.cfg.Root, ref); err != nil {
				out = append(out, errFinding(ruleReferences, u.ID, "unresolvable reference %q: %v", ref, err))
			}
		}
	}
	return out, nil
}

// resolveReference resolves an anchor (path:line, path::Symbol) or plain path.
func resolveReference(root, ref string) error {
	refPath := ref
	if a, perr := docline.ParseAnchor(ref); perr == nil {
		refPath = a.Path
	}
	if hasRawDotDot(refPath) {
		return errors.New("path escapes the workspace root")
	}
	if _, perr := docline.ParseAnchor(ref); perr == nil {
		if _, err := docline.ResolveAnchorString(root, ref); err != nil {
			return fmt.Errorf("resolve anchor: %w", err)
		}
		return nil
	} else if !errors.Is(perr, docline.ErrAnchorSyntax) {
		return fmt.Errorf("parse anchor: %w", perr)
	}
	if _, err := docline.ResolveWorkspacePath(root, ref); err != nil {
		return fmt.Errorf("resolve path: %w", err)
	}
	return nil
}

// checkVerificationSurface requires each unit to name an explicit
// verification surface.
func checkVerificationSurface(ctx *checkContext) ([]Finding, error) {
	var out []Finding
	for _, u := range ctx.plan.Units {
		surface := 0
		for _, v := range u.Verification {
			if strings.TrimSpace(v) != "" {
				surface++
			}
		}
		if surface == 0 {
			out = append(out, errFinding(ruleVerificationSurface, u.ID, "no explicit verification surface"))
		}
	}
	return out, nil
}
