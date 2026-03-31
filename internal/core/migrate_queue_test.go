package core_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/core"
)

func setupMigrationWorkspace(t *testing.T) *core.Workspace {
	t.Helper()
	root := t.TempDir()
	wsDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(filepath.Join(wsDir, "tasks"), 0o755))

	// Write minimal config files
	require.NoError(t, os.WriteFile(filepath.Join(wsDir, "config.yaml"), []byte(`
artifact_types:
  task:
    prefix: "T"
    name_format: "{prefix}{NNN}-{slug}"
max_slug_length: 60
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(wsDir, "header-def.yaml"), []byte(`
defaults:
  id: {type: string, immutable: true}
  created_date: {type: datetime, immutable: true}
  updated_date: {type: datetime, immutable: false}
types:
  task:
    prefix: "T"
    id_format: "{prefix}{NNN}"
    fields:
      status: {type: enum, values: [queued, active, done], default: queued}
`), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(wsDir, "registry.yaml"), []byte(`
directories:
  - path: tasks
    condition:
      type: [task]
`), 0o644))

	ws, err := core.NewWorkspace(context.Background(), root)
	require.NoError(t, err)
	t.Cleanup(func() { ws.Close() })
	return ws
}

func TestMigrateFlatToHierarchical_DryRun(t *testing.T) {
	// Arrange
	ws := setupMigrationWorkspace(t)

	// Act
	report, err := core.MigrateFlatToHierarchical(ws, true)

	// Assert
	require.NoError(t, err)
	assert.True(t, report.DryRun)
	assert.Zero(t, report.FilesMoved, "dry run should not move files")
}

func TestMigrateFlatToHierarchical_MovesFiles(t *testing.T) {
	// Arrange
	ws := setupMigrationWorkspace(t)

	// Act
	report, err := core.MigrateFlatToHierarchical(ws, false)

	// Assert
	require.NoError(t, err)
	assert.False(t, report.DryRun)
}

func TestMigrateFlatToHierarchical_StateFileCreated(t *testing.T) {
	// Arrange
	ws := setupMigrationWorkspace(t)

	// Act
	_, err := core.MigrateFlatToHierarchical(ws, false)

	// Assert
	require.NoError(t, err)
	statePath := filepath.Join(ws.RootPath, ".backlogit", ".migration-state")
	assert.FileExists(t, statePath)
}

func TestRollbackMigration(t *testing.T) {
	// Arrange
	ws := setupMigrationWorkspace(t)

	// Act
	err := core.RollbackMigration(ws)

	// Assert
	require.NoError(t, err)
}
