package config_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/config"
)

func TestLoad_ValidConfig(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	configContent := `
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
	require.NoError(t, os.WriteFile(filepath.Join(ws, "config.yaml"), []byte(configContent), 0o644))

	// Act
	cfg, err := config.Load(context.Background(), ws)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Contains(t, cfg.ArtifactTypes, "task")
}

func TestLoad_MissingFile(t *testing.T) {
	// Arrange
	ws := t.TempDir()

	// Act
	_, err := config.Load(context.Background(), ws)

	// Assert
	assert.Error(t, err)
}

func TestDefaultConfig_ReturnsValid(t *testing.T) {
	// Act
	cfg := config.DefaultConfig()

	// Assert
	assert.NotNil(t, cfg)
	assert.NotEmpty(t, cfg.ArtifactTypes)
	assert.Contains(t, cfg.ArtifactTypes, "deliberation")
}

func TestLoad_ConfigWithSuffixAndBugLevel(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	configContent := `
artifact_types:
  feature:
    prefix: F
    suffix: "-F"
    name_format: "{NNN}{suffix}"
    allowed_children: [task, review]
fields:
  status:
    type: enum
    values: [queued, active, done]
    default: queued
max_slug_length: 60
bug_level: 3
`
	require.NoError(t, os.WriteFile(filepath.Join(ws, "config.yaml"), []byte(configContent), 0o644))

	// Act
	cfg, err := config.Load(context.Background(), ws)

	// Assert
	require.NoError(t, err)
	require.Contains(t, cfg.ArtifactTypes, "feature")
	assert.Equal(t, "-F", cfg.ArtifactTypes["feature"].Suffix)
	assert.Equal(t, 3, cfg.BugLevel)
}

func TestLoad_InvalidBugLevelRejected(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	configContent := `
artifact_types:
  feature:
    prefix: F
    suffix: "-F"
    name_format: "{NNN}{suffix}"
    allowed_children: [task, review]
fields:
  status:
    type: enum
    values: [queued, active, done]
    default: queued
max_slug_length: 60
bug_level: 4
`
	require.NoError(t, os.WriteFile(filepath.Join(ws, "config.yaml"), []byte(configContent), 0o644))

	// Act
	_, err := config.Load(context.Background(), ws)

	// Assert
	require.Error(t, err)
	assert.Contains(t, err.Error(), "bug_level")
}

func TestDefaultConfig_UsesNumericSuffixFormats(t *testing.T) {
	// Act
	cfg := config.DefaultConfig()

	// Assert
	require.Contains(t, cfg.ArtifactTypes, "feature")
	require.Contains(t, cfg.ArtifactTypes, "task")
	assert.Equal(t, "-F", cfg.ArtifactTypes["feature"].Suffix)
	assert.Equal(t, "{NNN}{suffix}", cfg.ArtifactTypes["feature"].NameFormat)
	assert.Equal(t, "-T", cfg.ArtifactTypes["task"].Suffix)
	assert.Equal(t, "{NNN}{suffix}", cfg.ArtifactTypes["task"].NameFormat)
	assert.Equal(t, 3, cfg.BugLevel)
}

func TestDefaultRegistry_ReturnsValid(t *testing.T) {
	// Act
	reg := config.DefaultRegistry()

	// Assert
	assert.NotNil(t, reg)
	assert.NotEmpty(t, reg.Directories)
}

func TestWriteDefaults_CreatesFiles(t *testing.T) {
	// Arrange
	ws := t.TempDir()

	// Act
	err := config.WriteDefaults(ws)

	// Assert
	require.NoError(t, err)
	assert.FileExists(t, filepath.Join(ws, "config.yaml"))
	assert.FileExists(t, filepath.Join(ws, "registry.yaml"))
	assert.FileExists(t, filepath.Join(ws, "queue", ".stash.md"))
	assert.FileExists(t, filepath.Join(ws, "templates", "deliberation.md"))
}
