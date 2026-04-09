package telemetry_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/backlogit/backlogit/internal/telemetry"
)

func TestAttributeTool_KnownPrefixes(t *testing.T) {
	tests := []struct {
		toolName string
		want     string
	}{
		{"backlogit_create_item", "backlogit"},
		{"backlogit_get_item", "backlogit"},
		{"backlogit_query_sql", "backlogit"},
		{"engram-query_memory", "engram"},
		{"engram-map_code", "engram"},
		{"agent-intercom-broadcast", "agent-intercom"},
		{"agent-intercom-ping", "agent-intercom"},
		{"github-create_pull_request", "github"},
		{"tavily-search", "tavily"},
		{"context7-get_library_docs", "context7"},
		{"microsoft-docs-search", "microsoft-docs"},
		{"report_intent", "copilot_builtin"},
		{"task_complete", "copilot_builtin"},
		{"view", "copilot_builtin"},
		{"edit", "copilot_builtin"},
		{"create", "copilot_builtin"},
		{"glob", "copilot_builtin"},
		{"grep", "copilot_builtin"},
		{"skill", "copilot_builtin"},
	}
	for _, tt := range tests {
		t.Run(tt.toolName, func(t *testing.T) {
			got := telemetry.AttributeTool(tt.toolName)
			assert.Equal(t, tt.want, got, "tool %q should map to server %q", tt.toolName, tt.want)
		})
	}
}

func TestAttributeTool_UnknownTool(t *testing.T) {
	assert.Equal(t, "unknown", telemetry.AttributeTool("some_random_tool_xyz"))
	assert.Equal(t, "unknown", telemetry.AttributeTool(""))
	assert.Equal(t, "unknown", telemetry.AttributeTool("zzz_not_a_server"))
}

func TestAttributeTool_LongestPrefixWins(t *testing.T) {
	// "microsoft-docs-" is longer than a hypothetical "microsoft-" prefix;
	// the longer prefix should win.
	got := telemetry.AttributeTool("microsoft-docs-search")
	assert.Equal(t, "microsoft-docs", got)
}
