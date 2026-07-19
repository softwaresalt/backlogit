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

func TestSE1SizeSchemaFeatureShipmentHarness(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, config.WriteDefaults(dir))

	cfg, err := config.LoadHeaderDef(dir)
	require.NoError(t, err)

	tests := []struct {
		artifactType string
		wantSize     bool
	}{
		{artifactType: "task", wantSize: true},
		{artifactType: "feature", wantSize: true},
		{artifactType: "shipment", wantSize: true},
	}
	for _, tt := range tests {
		t.Run(tt.artifactType, func(t *testing.T) {
			schema, err := cfg.ResolveFieldSchema(tt.artifactType)
			require.NoError(t, err)

			sizeDef, ok := schema["size"]
			if tt.wantSize && !ok {
				t.Fatalf("TODO: implement SE-1 size schema for %s", tt.artifactType)
			}
			require.True(t, ok, "%s schema must define size", tt.artifactType)
			assert.Equal(t, "enum", sizeDef.Type)
			assert.Equal(t, []string{"XS", "S", "M", "L", "XL"}, sizeDef.Values)
			assert.True(t, sizeDef.Optional)

			sourceDef, ok := schema["size_source"]
			if !ok {
				t.Fatalf("TODO: implement SE-1 size_source schema for %s", tt.artifactType)
			}
			assert.Equal(t, "enum", sourceDef.Type)
			assert.Equal(t, []string{"human", "agent", "derived"}, sourceDef.Values)
			assert.True(t, sourceDef.Optional)

			rulesetDef, ok := schema["size_ruleset_version"]
			if !ok {
				t.Fatalf("TODO: implement SE-1 bounded size_ruleset_version schema for %s", tt.artifactType)
			}
			assert.True(t, rulesetDef.Optional)
			assert.NotEqual(t, "string", rulesetDef.Type, "ruleset version must not be unbounded free text")
		})
	}
}
