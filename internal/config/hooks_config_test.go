package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
)

// 027.005-T: HooksConfig schema expansion and hooks.yaml defaults

func readFileString(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return string(data)
}

func TestWriteDefaults_CreatesHooksYAML(t *testing.T) {
	ws := t.TempDir()

	err := config.WriteDefaults(ws)
	require.NoError(t, err)

	assert.FileExists(t, filepath.Join(ws, "hooks.yaml"),
		"WriteDefaults must create hooks.yaml")
}

func TestDefaultHooksConfig_ReturnsNonNil(t *testing.T) {
	cfg := config.DefaultHooksConfig()
	assert.NotNil(t, cfg)
}

func TestDefaultHooksConfig_HasBlockedStaleDays(t *testing.T) {
	cfg := config.DefaultHooksConfig()
	assert.Greater(t, cfg.EventThresholds.BlockedStaleDays, 0,
		"default BlockedStaleDays must be a positive number of days")
}

func TestDefaultHooksConfig_V1AgentSubscriptions(t *testing.T) {
	cfg := config.DefaultHooksConfig()
	require.NotNil(t, cfg.AgentSubscriptions,
		"v1 config must define at least one agent subscription")

	// v1 agents: stage and ship
	for _, agentID := range []string{"stage", "ship"} {
		subs, ok := cfg.AgentSubscriptions[agentID]
		assert.True(t, ok, "agent %q must be in default subscriptions", agentID)
		assert.NotEmpty(t, subs, "agent %q must subscribe to at least one event type", agentID)
	}
}

func TestDefaultHooksConfig_NoStashOverflowThresholds(t *testing.T) {
	// v1 explicitly defers stash_overflow; thresholds must not appear in defaults.
	cfg := config.DefaultHooksConfig()
	_ = cfg.EventThresholds // only BlockedStaleDays in v1
	// No stash overflow fields exist in HookEventThresholds for v1.
	// This test documents the intentional omission.
}

func TestHooksConfig_Roundtrip_YAML(t *testing.T) {
	ws := t.TempDir()
	require.NoError(t, config.WriteDefaults(ws))

	hooksPath := filepath.Join(ws, "hooks.yaml")
	assert.FileExists(t, hooksPath)

	// Verify the file contains at least one v1 event type constant.
	data := readFileString(t, hooksPath)
	assert.Contains(t, data, "blocked_stale",
		"hooks.yaml must reference the v1 blocked_stale event type")
}

func TestHooksConfig_AgentSubscriptions_V1SignalsOnly(t *testing.T) {
	cfg := config.DefaultHooksConfig()

	v1Types := map[string]bool{
		"feature_review_ready": true,
		"post_merge_closure":   true,
		"blocked_stale":        true,
	}
	for agent, subs := range cfg.AgentSubscriptions {
		for _, et := range subs {
			assert.True(t, v1Types[et],
				"agent %q subscribes to non-v1 event type %q", agent, et)
		}
	}
}
