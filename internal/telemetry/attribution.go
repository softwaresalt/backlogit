package telemetry

import "sort"

// defaultPrefixes maps known MCP server name prefixes (or exact tool names)
// to their server names. Entries without a trailing - or _ are exact matches;
// entries ending in - or _ are prefix patterns. Longest prefix wins.
//
// v1: hardcoded defaults. YAML config override is deferred (Plan Review F12).
var defaultPrefixes = map[string]string{
	"backlogit_":      "backlogit",
	"engram-":         "engram",
	"agent-intercom-": "agent-intercom",
	"github-":         "github",
	"tavily-":         "tavily",
	"context7-":       "context7",
	"microsoft-docs-": "microsoft-docs",
	"report_intent":   "copilot_builtin",
	"task_complete":   "copilot_builtin",
	"view":            "copilot_builtin",
	"edit":            "copilot_builtin",
	"create":          "copilot_builtin",
	"glob":            "copilot_builtin",
	"grep":            "copilot_builtin",
	"skill":           "copilot_builtin",
}

// prefixesByLength is the sorted-descending list of prefix keys for longest-match.
var prefixesByLength []string

func init() {
	for k := range defaultPrefixes {
		prefixesByLength = append(prefixesByLength, k)
	}
	// Sort descending by length so the first match is always the longest.
	sort.Slice(prefixesByLength, func(i, j int) bool {
		return len(prefixesByLength[i]) > len(prefixesByLength[j])
	})
}

// AttributeTool maps a tool name to its originating MCP server name using
// longest-prefix-first matching against the default prefix registry.
// Exact tool names (e.g. "view", "edit") are matched before prefix patterns.
// Unknown tool names resolve to "unknown".
func AttributeTool(toolName string) string {
	if toolName == "" {
		return "unknown"
	}
	// prefixesByLength is sorted longest-first, so the first match wins.
	for _, key := range prefixesByLength {
		if toolName == key {
			return defaultPrefixes[key]
		}
		last := key[len(key)-1]
		if (last == '-' || last == '_') && len(toolName) >= len(key) && toolName[:len(key)] == key {
			return defaultPrefixes[key]
		}
	}
	return "unknown"
}
