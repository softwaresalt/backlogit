package config_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/softwaresalt/backlogit/internal/config"
)

const durableBaseConfig = `
artifact_types:
  task:
    prefix: T
    name_format: "{prefix}{NNN}-{title_slug}"
    allowed_children: []
fields:
  status:
    type: enum
    values: [todo, in_progress, done]
    default: todo
max_slug_length: 60
`

// TestDurableWrites_DefaultsToFalse verifies that a config without an explicit
// durable_writes key loads with the flag off (the opt-in default).
func TestDurableWrites_DefaultsToFalse(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(ws, "config.yaml"), []byte(durableBaseConfig), 0o644))

	cfg, err := config.Load(context.Background(), ws)
	require.NoError(t, err)
	assert.False(t, cfg.DurableWrites, "durable_writes must default to false when absent")
}

// TestDurableWrites_ExplicitTrueRoundTrips verifies that durable_writes: true is
// parsed into the config struct.
func TestDurableWrites_ExplicitTrueRoundTrips(t *testing.T) {
	t.Parallel()
	ws := t.TempDir()
	content := durableBaseConfig + "durable_writes: true\n"
	require.NoError(t, os.WriteFile(filepath.Join(ws, "config.yaml"), []byte(content), 0o644))

	cfg, err := config.Load(context.Background(), ws)
	require.NoError(t, err)
	assert.True(t, cfg.DurableWrites, "durable_writes: true must round-trip into the config")
}

// TestDurableWrites_OmitemptyKeepsOutputClean verifies that a false flag is
// elided from serialized YAML so existing configs stay byte-unchanged.
func TestDurableWrites_OmitemptyKeepsOutputClean(t *testing.T) {
	t.Parallel()
	cfg := &config.WorkspaceConfig{DurableWrites: false}

	out, err := yaml.Marshal(cfg)
	require.NoError(t, err)
	assert.NotContains(t, string(out), "durable_writes",
		"a false durable_writes must be omitted from serialized YAML (omitempty)")
}
