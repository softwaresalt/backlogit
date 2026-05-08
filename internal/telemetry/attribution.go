package telemetry

import "sort"

// defaultPrefixes maps known MCP server name prefixes (or exact tool names)
// to their server names. Entries without a trailing - or _ are exact matches;
// entries ending in - or _ are prefix patterns. Longest prefix wins.
var defaultPrefixes = map[string]string{
	"backlogit_":          "backlogit",
	"engram-":             "engram",
	"agent-intercom-":     "agent-intercom",
	"github-":             "github",
	"tavily-":             "tavily",
	"context7-":           "context7",
	"microsoft-docs-":     "microsoft-docs",
	"graphtor-":           "graphtor",
	"adversarial-review-": "adversarial-review",
	"report_intent":       "copilot_builtin",
	"task_complete":       "copilot_builtin",
	"view":                "copilot_builtin",
	"edit":                "copilot_builtin",
	"create":              "copilot_builtin",
	"glob":                "copilot_builtin",
	"grep":                "copilot_builtin",
	"skill":               "copilot_builtin",
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
	return AttributeToolWithConfig(toolName, nil)
}

// BuildAttributor compiles a merged attribution registry once and returns a
// reusable attribution function. Use this in harvest and correlate loops where
// the same customPrefixes apply to many tool names, to avoid per-call sort and
// map allocation overhead.
func BuildAttributor(customPrefixes map[string]string) func(string) string {
	if len(customPrefixes) == 0 {
		return func(toolName string) string {
			return AttributeTool(toolName)
		}
	}

	merged := make(map[string]string, len(defaultPrefixes)+len(customPrefixes))
	for k, v := range defaultPrefixes {
		merged[k] = v
	}
	for k, v := range customPrefixes {
		if k != "" && v != "" {
			merged[k] = v
		}
	}

	keys := make([]string, 0, len(merged))
	for k := range merged {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) > len(keys[j])
		}
		return keys[i] < keys[j]
	})

	return func(toolName string) string {
		if toolName == "" {
			return "unknown"
		}
		for _, key := range keys {
			if toolName == key {
				return merged[key]
			}
			last := key[len(key)-1]
			if (last == '-' || last == '_') && len(toolName) >= len(key) && toolName[:len(key)] == key {
				return merged[key]
			}
		}
		return "unknown"
	}
}

// AttributeToolWithConfig maps a tool name to its originating MCP server name.
// Custom prefixes from workspace config are merged with the built-in defaults;
// custom entries take priority when the same prefix key exists in both.
// Nil or empty customPrefixes falls back to built-in defaults only.
// Unknown tool names resolve to "unknown".
func AttributeToolWithConfig(toolName string, customPrefixes map[string]string) string {
	if toolName == "" {
		return "unknown"
	}
	if len(customPrefixes) == 0 {
		// Fast path: use the pre-sorted built-in slice.
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

	// Merge: built-in defaults first, then custom overrides.
	merged := make(map[string]string, len(defaultPrefixes)+len(customPrefixes))
	for k, v := range defaultPrefixes {
		merged[k] = v
	}
	for k, v := range customPrefixes {
		if k != "" && v != "" {
			merged[k] = v
		}
	}

	// Sort merged keys descending by length so the first match is always
	// the longest. Ties broken alphabetically for stable output.
	keys := make([]string, 0, len(merged))
	for k := range merged {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if len(keys[i]) != len(keys[j]) {
			return len(keys[i]) > len(keys[j])
		}
		return keys[i] < keys[j]
	})

	for _, key := range keys {
		if toolName == key {
			return merged[key]
		}
		last := key[len(key)-1]
		if (last == '-' || last == '_') && len(toolName) >= len(key) && toolName[:len(key)] == key {
			return merged[key]
		}
	}
	return "unknown"
}
