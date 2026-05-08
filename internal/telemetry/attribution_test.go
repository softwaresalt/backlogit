package telemetry_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/softwaresalt/backlogit/internal/telemetry"
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

func TestAttributeTool_NewDefaultServers(t *testing.T) {
	tests := []struct {
		toolName string
		want     string
	}{
		{"graphtor-visualize", "graphtor"},
		{"adversarial-review-dispatch", "adversarial-review"},
	}
	for _, tt := range tests {
		t.Run(tt.toolName, func(t *testing.T) {
			got := telemetry.AttributeTool(tt.toolName)
			assert.Equal(t, tt.want, got, "tool %q should map to server %q", tt.toolName, tt.want)
		})
	}
}

func TestAttributeToolWithConfig_NilFallsToDefaults(t *testing.T) {
	assert.Equal(t, "backlogit", telemetry.AttributeToolWithConfig("backlogit_get_item", nil))
	assert.Equal(t, "engram", telemetry.AttributeToolWithConfig("engram-query_memory", nil))
}

func TestAttributeToolWithConfig_CustomPrefixAdded(t *testing.T) {
	custom := map[string]string{"myserver-": "my-mcp-server"}
	got := telemetry.AttributeToolWithConfig("myserver-do_something", custom)
	assert.Equal(t, "my-mcp-server", got)
}

func TestAttributeToolWithConfig_CustomOverridesDefault(t *testing.T) {
	// Custom prefix can remap an existing default server.
	custom := map[string]string{"backlogit_": "my-backlogit-fork"}
	got := telemetry.AttributeToolWithConfig("backlogit_create_item", custom)
	assert.Equal(t, "my-backlogit-fork", got)
}

func TestAttributeToolWithConfig_EmptyCustomFallsToDefaults(t *testing.T) {
	got := telemetry.AttributeToolWithConfig("engram-map_code", map[string]string{})
	assert.Equal(t, "engram", got)
}
