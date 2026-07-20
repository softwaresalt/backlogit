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

func TestSE1SizeSchemaTaskOnlyHarness(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, config.WriteDefaults(dir))

	cfg, err := config.LoadHeaderDef(dir)
	require.NoError(t, err)

	// Task is the only sizable unit: it carries the full size + provenance schema.
	t.Run("task", func(t *testing.T) {
		schema, err := cfg.ResolveFieldSchema("task")
		require.NoError(t, err)

		sizeDef, ok := schema["size"]
		require.True(t, ok, "task schema must define size")
		assert.Equal(t, "enum", sizeDef.Type)
		assert.Equal(t, []string{"XS", "S", "M", "L", "XL"}, sizeDef.Values)
		assert.True(t, sizeDef.Optional)

		sourceDef, ok := schema["size_source"]
		require.True(t, ok, "task schema must define size_source")
		assert.Equal(t, "enum", sourceDef.Type)
		assert.Equal(t, []string{"human", "agent", "derived"}, sourceDef.Values)
		assert.True(t, sourceDef.Optional)

		rulesetDef, ok := schema["size_ruleset_version"]
		require.True(t, ok, "task schema must define size_ruleset_version")
		assert.True(t, rulesetDef.Optional)
		assert.Equal(t, "string", rulesetDef.Type, "ruleset version is an opaque stored version string")
	})

	// Features and shipments are rollup parents, not sizable units: they must NOT
	// define a stored size field (their size is a computed-on-read composition).
	for _, parentType := range []string{"feature", "shipment"} {
		t.Run(parentType, func(t *testing.T) {
			schema, err := cfg.ResolveFieldSchema(parentType)
			require.NoError(t, err)
			_, ok := schema["size"]
			assert.False(t, ok, "%s must not define a stored size field (task-only sizing)", parentType)
		})
	}
}
