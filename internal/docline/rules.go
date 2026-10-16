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
	// References enables the reference rules: file:line and symbol anchors,
	// task-ID existence, stale terminology and implementation-comment drift.
	References bool
	// StaleTerms maps a deprecated term to its replacement for the stale
	// terminology rule. Empty means the rule reports nothing.
	StaleTerms map[string]string
	// TaskIDExists overrides task-ID lookup; nil scans the workspace backlog.
	TaskIDExists func(id string) bool
}

// ParseRules builds a RuleSet from rule-group names ("structural").
func ParseRules(names []string) (RuleSet, error) {
	var rs RuleSet
	for _, n := range names {
		switch strings.ToLower(strings.TrimSpace(n)) {
		case "":
		case "structural":
			rs.Structural = true
		case "references":
			rs.References = true
		default:
			return RuleSet{}, fmt.Errorf("docline.ParseRules: unknown rule group %q (want structural or references)", n)
		}
	}
	return rs, nil
}

// resolveTaskIDLookup returns the task-ID existence check for opts, or nil when
// no backlog directory exists.
func resolveTaskIDLookup(opts Options) (func(string) bool, error) {
	if opts.Rules.TaskIDExists != nil {
		return opts.Rules.TaskIDExists, nil
	}
	ids, present, err := workspaceTaskIDs(opts.Root)
	if err != nil {
		return nil, err
	}
	if !present {
		return nil, nil
	}
	prefixes := map[string]bool{}
	for id := range ids {
		if m := prefixIDPattern.FindStringSubmatch(id); m != nil {
			prefixes[m[1]] = true
		}
	}
	return func(id string) bool {
		// A prefix-shaped token whose letters are not a declared prefix (SHA256)
		// is not an item reference.
		if m := prefixIDPattern.FindStringSubmatch(id); m != nil && !prefixes[m[1]] {
			return true
		}
		return ids[id]
	}, nil
}
