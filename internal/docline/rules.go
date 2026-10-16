package docline

import (
	"fmt"
	"strings"
)

// RuleSet selects the opt-in lint rule groups that run in addition to the
// frontmatter contract validation. The zero value runs none of them, so the
// existing docs-lint gate is unchanged unless a caller enables a group.
// Findings from these groups are reported with SeverityWarn.
type RuleSet struct {
	// Structural enables the structural rules: missing frontmatter block, table
	// shape, heading rules and numeric/topology claims.
	Structural bool
}

// ParseRules builds a RuleSet from rule-group names ("structural").
func ParseRules(names []string) (RuleSet, error) {
	var rs RuleSet
	for _, n := range names {
		switch strings.ToLower(strings.TrimSpace(n)) {
		case "":
		case "structural":
			rs.Structural = true
		default:
			return RuleSet{}, fmt.Errorf("docline.ParseRules: unknown rule group %q (want structural)", n)
		}
	}
	return rs, nil
}
