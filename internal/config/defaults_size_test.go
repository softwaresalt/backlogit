package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
)

// U6 (071.003-T): the task schema exposes an optional T-shirt `size` enum with
// no default, so existing tasks without `size` stay valid.

func TestWriteDefaults_TaskHasSizeEnum(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, config.WriteDefaults(dir))

	cfg, err := config.LoadHeaderDef(dir)
	require.NoError(t, err)

	schema, err := cfg.ResolveFieldSchema("task")
	require.NoError(t, err)

	sizeDef, ok := schema["size"]
	require.True(t, ok, "task schema must define a size field")
	assert.Equal(t, "enum", sizeDef.Type)
	assert.Equal(t, []string{"XS", "S", "M", "L", "XL"}, sizeDef.Values)
	assert.True(t, sizeDef.Optional, "size must be optional")
	assert.Empty(t, sizeDef.Default, "size must have no default")
}
