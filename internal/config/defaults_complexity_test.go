package config_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
)

func TestWriteDefaults_TaskHasComplexityEnum(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, config.WriteDefaults(dir))

	cfg, err := config.LoadHeaderDef(dir)
	require.NoError(t, err)

	schema, err := cfg.ResolveFieldSchema("task")
	require.NoError(t, err)

	complexityDef, ok := schema["complexity"]
	require.True(t, ok, "task schema must define a complexity field")
	assert.Equal(t, "enum", complexityDef.Type)
	assert.Equal(t, []string{"trivial", "low", "medium", "high"}, complexityDef.Values)
	assert.True(t, complexityDef.Optional, "complexity must be optional")
	assert.Empty(t, complexityDef.Default, "complexity must have no default")
}

func TestWriteDefaults_ComplexityIsTaskOnly(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, config.WriteDefaults(dir))

	cfg, err := config.LoadHeaderDef(dir)
	require.NoError(t, err)

	for _, parentType := range []string{"feature", "shipment"} {
		t.Run(parentType, func(t *testing.T) {
			schema, err := cfg.ResolveFieldSchema(parentType)
			require.NoError(t, err)

			_, ok := schema["complexity"]
			assert.False(t, ok, "%s must not define a stored complexity field", parentType)
		})
	}
}

func TestWriteDefaults_TaskWithoutComplexityRemainsValid(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, config.WriteDefaults(dir))

	cfg, err := config.LoadHeaderDef(dir)
	require.NoError(t, err)

	schema, err := cfg.ResolveFieldSchema("task")
	require.NoError(t, err)

	complexityDef, ok := schema["complexity"]
	require.True(t, ok, "task schema must define complexity")
	require.True(t, complexityDef.Optional, "tasks without complexity remain valid")
}
