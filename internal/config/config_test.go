package config_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/config"
	"github.com/backlogit/backlogit/internal/core"
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
  task:
    prefix: T
    suffix: "-T"
    name_format: "{NNN}{suffix}"
    allowed_children: [subtask]
  bug:
    prefix: B
    suffix: "-B"
    name_format: "{NNN}{suffix}"
fields:
  status:
    type: enum
    values: [queued, active, done]
    default: queued
max_slug_length: 60
bug_level: 3
queue_layout:
  root_dir: queue
  levels:
    - level: 1
      types: [feature]
    - level: 2
      types: [task]
    - level: 3
      types: [subtask]
`
	require.NoError(t, os.WriteFile(filepath.Join(ws, "config.yaml"), []byte(configContent), 0o644))

	// Act
	cfg, err := config.Load(context.Background(), ws)

	// Assert
	require.NoError(t, err)
	require.Contains(t, cfg.ArtifactTypes, "feature")
	assert.Equal(t, "-F", cfg.ArtifactTypes["feature"].Suffix)
	assert.Equal(t, 3, cfg.BugLevel)
	assert.Contains(t, cfg.ArtifactTypes["task"].AllowedChildren, "bug")
	assert.NotContains(t, cfg.ArtifactTypes["feature"].AllowedChildren, "bug")
	level, levelErr := core.LevelForType(cfg.QueueLayout, "bug")
	require.NoError(t, levelErr)
	assert.Equal(t, 3, level)
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
	assert.Contains(t, cfg.ArtifactTypes["task"].AllowedChildren, "bug")
	assert.NotContains(t, cfg.ArtifactTypes["feature"].AllowedChildren, "bug")
	level, err := core.LevelForType(cfg.QueueLayout, "bug")
	require.NoError(t, err)
	assert.Equal(t, 3, level)
}

func TestLoad_BugLevelTwoMovesBugParentingAndHierarchy(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	configContent := `
artifact_types:
  feature:
    prefix: F
    suffix: "-F"
    name_format: "{NNN}{suffix}"
    allowed_children: [task, review]
  task:
    prefix: T
    suffix: "-T"
    name_format: "{NNN}{suffix}"
    allowed_children: [subtask]
  subtask:
    prefix: ST
    suffix: "-ST"
    name_format: "{NNN}{suffix}"
  bug:
    prefix: B
    suffix: "-B"
    name_format: "{NNN}{suffix}"
fields:
  status:
    type: enum
    values: [queued, active, done]
    default: queued
max_slug_length: 60
bug_level: 2
queue_layout:
  root_dir: queue
  levels:
    - level: 1
      types: [feature]
    - level: 2
      types: [task]
    - level: 3
      types: [subtask]
`
	require.NoError(t, os.WriteFile(filepath.Join(ws, "config.yaml"), []byte(configContent), 0o644))

	// Act
	cfg, err := config.Load(context.Background(), ws)

	// Assert
	require.NoError(t, err)
	assert.Contains(t, cfg.ArtifactTypes["feature"].AllowedChildren, "bug")
	assert.NotContains(t, cfg.ArtifactTypes["task"].AllowedChildren, "bug")
	level, levelErr := core.LevelForType(cfg.QueueLayout, "bug")
	require.NoError(t, levelErr)
	assert.Equal(t, 2, level)
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
