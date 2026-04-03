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
}
