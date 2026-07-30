package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

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

func TestLoadHeaderDef_LegacyGeneratedDefaultAddsComplexity(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, config.WriteDefaults(dir))
	writeHeaderDefWithoutComplexity(t, dir, false)

	cfg, err := config.LoadHeaderDef(dir)
	require.NoError(t, err)

	task := cfg.Types["task"]
	require.NotNil(t, task)
	complexity, ok := task.Fields["complexity"]
	require.True(t, ok, "legacy generated header-def should load with the new complexity field")
	assert.Equal(t, "enum", complexity.Type)
	assert.Equal(t, []string{"trivial", "low", "medium", "high"}, complexity.Values)
	assert.True(t, complexity.Optional)
}

func TestLoadHeaderDef_CustomTaskSchemaWithoutComplexityPreserved(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, config.WriteDefaults(dir))
	writeHeaderDefWithoutComplexity(t, dir, true)

	cfg, err := config.LoadHeaderDef(dir)
	require.NoError(t, err)

	task := cfg.Types["task"]
	require.NotNil(t, task)
	_, ok := task.Fields["complexity"]
	assert.False(t, ok, "operator-customized task schema should not be silently widened")
}

func writeHeaderDefWithoutComplexity(t *testing.T, dir string, customize bool) {
	t.Helper()
	path := filepath.Join(dir, "header-def.yaml")
	raw, err := os.ReadFile(path)
	require.NoError(t, err)

	var cfg config.HeaderDefConfig
	require.NoError(t, yaml.Unmarshal(raw, &cfg))
	delete(cfg.Types["task"].Fields, "complexity")
	if customize {
		cfg.Types["task"].Fields["custom_operator_field"] = &config.FieldDef{Type: "string", Optional: true}
	}
	out, err := yaml.Marshal(&cfg)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(path, out, 0o644))
}
