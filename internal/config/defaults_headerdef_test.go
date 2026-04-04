package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/config"
)

// TASK-002.02.02: Generate default header-def.yaml in WriteDefaults.

func TestWriteDefaults_CreatesHeaderDef(t *testing.T) {
	// Arrange
	dir := t.TempDir()

	// Act
	err := config.WriteDefaults(dir)

	// Assert
	require.NoError(t, err)
	headerDefPath := filepath.Join(dir, "header-def.yaml")
	assert.FileExists(t, headerDefPath)
}

func TestWriteDefaults_HeaderDefLoadable(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	require.NoError(t, config.WriteDefaults(dir))

	// Act — the generated file should be loadable
	cfg, err := config.LoadHeaderDef(dir)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.GreaterOrEqual(t, len(cfg.Types), 5)
}

func TestWriteDefaults_HeaderDefContainsAllTypes(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	require.NoError(t, config.WriteDefaults(dir))

	// Act
	cfg, err := config.LoadHeaderDef(dir)
	require.NoError(t, err)

	// Assert — default workspace types must be present
	expectedTypes := []string{"feature", "deliberation", "task", "review", "subtask"}
	for _, typeName := range expectedTypes {
		assert.Contains(t, cfg.Types, typeName, "missing type: %s", typeName)
	}
}

func TestWriteDefaults_HeaderDefSystemFields(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	require.NoError(t, config.WriteDefaults(dir))

	// Act
	cfg, err := config.LoadHeaderDef(dir)
	require.NoError(t, err)

	// Assert — system defaults are immutable
	assert.True(t, cfg.Defaults.ID.Immutable)
	assert.True(t, cfg.Defaults.CreatedDate.Immutable)
	assert.True(t, cfg.Defaults.UpdatedDate.Immutable)
}

func TestWriteDefaults_HeaderDefTypedPrefixes(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	require.NoError(t, config.WriteDefaults(dir))

	// Act
	cfg, err := config.LoadHeaderDef(dir)
	require.NoError(t, err)

	// Assert — default types use typed prefixes
	assert.Equal(t, "F", cfg.Types["feature"].Prefix)
	assert.Equal(t, "T", cfg.Types["task"].Prefix)
	assert.Equal(t, "R", cfg.Types["review"].Prefix)
	assert.Equal(t, "ST", cfg.Types["subtask"].Prefix)
}

func TestWriteDefaults_ReviewHeaderDefIncludesStatusAndSourceBranch(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, config.WriteDefaults(dir))

	cfg, err := config.LoadHeaderDef(dir)
	require.NoError(t, err)

	reviewType, ok := cfg.Types["review"]
	require.True(t, ok, "review type should exist in header-def")
	assert.Contains(t, reviewType.Fields, "status")
	assert.Contains(t, reviewType.Fields, "source_branch")
}

func TestWriteDefaults_DoesNotOverwriteExisting(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	headerDefPath := filepath.Join(dir, "header-def.yaml")
	customContent := []byte("custom: true\n")
	require.NoError(t, os.WriteFile(headerDefPath, customContent, 0o644))

	// Act
	err := config.WriteDefaults(dir)
	require.NoError(t, err)

	// Assert — existing file should not be overwritten
	data, err := os.ReadFile(headerDefPath)
	require.NoError(t, err)
	assert.Equal(t, customContent, data)
}
