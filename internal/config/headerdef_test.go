package config_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/config"
)

// TASK-002.02.01: Implement header-def.yaml schema and loader.

func TestLoadHeaderDef_ValidFile(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	content := `defaults:
  id:
    type: string
    immutable: true
  created_date:
    type: datetime
    immutable: true
  updated_date:
    type: datetime
    immutable: true
types:
  task:
    prefix: OP
    id_format: "{prefix}{NNN}"
    fields:
      status:
        type: enum
        values: [queued, active, blocked, review, done]
        default: queued
      priority:
        type: enum
        values: [low, medium, high]
        default: medium
  bug:
    prefix: OP
    id_format: "{prefix}{NNN}"
    fields:
      status:
        type: enum
        values: [queued, active, blocked, review, done]
        default: queued
      severity:
        type: enum
        values: [low, medium, high, critical]
        default: medium
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "header-def.yaml"), []byte(content), 0o644))

	// Act
	cfg, err := config.LoadHeaderDef(dir)

	// Assert
	require.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.True(t, cfg.Defaults.ID.Immutable)
	assert.Len(t, cfg.Types, 2)
	assert.Equal(t, "OP", cfg.Types["task"].Prefix)
}

func TestLoadHeaderDef_MissingFile(t *testing.T) {
	// Arrange
	dir := t.TempDir()

	// Act
	_, err := config.LoadHeaderDef(dir)

	// Assert
	require.Error(t, err)
}

func TestLoadHeaderDef_InvalidYAML(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "header-def.yaml"), []byte(":::invalid"), 0o644))

	// Act
	_, err := config.LoadHeaderDef(dir)

	// Assert
	require.Error(t, err)
}

func TestHeaderDefConfig_ResolveFieldSchema(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	content := `defaults:
  id:
    type: string
    immutable: true
  created_date:
    type: datetime
    immutable: true
  updated_date:
    type: datetime
    immutable: true
types:
  task:
    prefix: OP
    id_format: "{prefix}{NNN}"
    fields:
      status:
        type: enum
        values: [queued, active, blocked, done]
        default: queued
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "header-def.yaml"), []byte(content), 0o644))
	cfg, err := config.LoadHeaderDef(dir)
	require.NoError(t, err)

	// Act
	schema, err := cfg.ResolveFieldSchema("task")

	// Assert
	require.NoError(t, err)
	assert.Contains(t, schema, "status")
	assert.Contains(t, schema, "id")
	assert.Contains(t, schema, "created_date")
}

func TestHeaderDefConfig_ResolveFieldSchema_UnknownType(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	content := `defaults:
  id:
    type: string
    immutable: true
  created_date:
    type: datetime
    immutable: true
  updated_date:
    type: datetime
    immutable: true
types:
  task:
    prefix: OP
    id_format: "{prefix}{NNN}"
    fields:
      status:
        type: enum
        values: [queued]
        default: queued
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "header-def.yaml"), []byte(content), 0o644))
	cfg, err := config.LoadHeaderDef(dir)
	require.NoError(t, err)

	// Act
	_, err = cfg.ResolveFieldSchema("nonexistent")

	// Assert
	require.Error(t, err)
}

func TestHeaderDefConfig_IsImmutable(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	content := `defaults:
  id:
    type: string
    immutable: true
  created_date:
    type: datetime
    immutable: true
  updated_date:
    type: datetime
    immutable: true
types:
  task:
    prefix: OP
    id_format: "{prefix}{NNN}"
    fields:
      status:
        type: enum
        values: [queued]
        default: queued
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "header-def.yaml"), []byte(content), 0o644))
	cfg, err := config.LoadHeaderDef(dir)
	require.NoError(t, err)

	// Act & Assert
	assert.True(t, cfg.IsImmutable("id"))
	assert.True(t, cfg.IsImmutable("created_date"))
	assert.True(t, cfg.IsImmutable("updated_date"))
	assert.False(t, cfg.IsImmutable("status"))
	assert.False(t, cfg.IsImmutable("title"))
}

func TestLoadHeaderDef_AllEightTypes(t *testing.T) {
	// Arrange
	dir := t.TempDir()
	content := `defaults:
  id:
    type: string
    immutable: true
  created_date:
    type: datetime
    immutable: true
  updated_date:
    type: datetime
    immutable: true
types:
  epic:
    prefix: OP
    id_format: "{prefix}{NNN}"
    fields:
      status:
        type: enum
        values: [queued, active, done]
        default: queued
  feature:
    prefix: OP
    id_format: "{prefix}{NNN}"
    fields:
      status:
        type: enum
        values: [queued, active, done]
        default: queued
  sub-epic:
    prefix: OP
    id_format: "{prefix}{NNN}"
    fields:
      status:
        type: enum
        values: [queued, active, done]
        default: queued
  user-story:
    prefix: OP
    id_format: "{prefix}{NNN}"
    fields:
      status:
        type: enum
        values: [queued, active, done]
        default: queued
  task:
    prefix: OP
    id_format: "{prefix}{NNN}"
    fields:
      status:
        type: enum
        values: [queued, active, done]
        default: queued
  sub-task:
    prefix: OP
    id_format: "{prefix}{NNN}"
    fields:
      status:
        type: enum
        values: [queued, active, done]
        default: queued
  bug:
    prefix: OP
    id_format: "{prefix}{NNN}"
    fields:
      status:
        type: enum
        values: [queued, active, done]
        default: queued
  decision:
    prefix: OP
    id_format: "{prefix}{NNN}"
    fields:
      status:
        type: enum
        values: [queued, active, done]
        default: queued
`
	require.NoError(t, os.WriteFile(filepath.Join(dir, "header-def.yaml"), []byte(content), 0o644))

	// Act
	cfg, err := config.LoadHeaderDef(dir)

	// Assert
	require.NoError(t, err)
	assert.Len(t, cfg.Types, 8)
	for _, typeName := range []string{"epic", "feature", "sub-epic", "user-story", "task", "sub-task", "bug", "decision"} {
		assert.Contains(t, cfg.Types, typeName, "missing type: %s", typeName)
	}
}
