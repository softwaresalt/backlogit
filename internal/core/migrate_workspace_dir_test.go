package core

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
)

func TestMigrateWorkspaceDir_DryRunReportsMovePlan(t *testing.T) {
	root := t.TempDir()
	source := seedLegacyWorkspaceDir(t, root)

	result, err := MigrateWorkspaceDir(root, MigrateWorkspaceDirOptions{DryRun: true})
	require.NoError(t, err)

	assert.Equal(t, source, result.Source)
	assert.Equal(t, filepath.Join(root, ".backlog"), result.Destination)
	assert.True(t, result.DryRun)
	assert.False(t, result.AlreadyDone)
	assert.True(t, slices.Contains(result.Files, "config.yaml"))
	assert.DirExists(t, source)
	assert.NoDirExists(t, result.Destination)
}

func TestMigrateWorkspaceDir_MovesLegacyWorkspaceDir(t *testing.T) {
	root := t.TempDir()
	source := seedLegacyWorkspaceDir(t, root)

	result, err := MigrateWorkspaceDir(root, MigrateWorkspaceDirOptions{})
	require.NoError(t, err)

	assert.Equal(t, source, result.Source)
	assert.Equal(t, filepath.Join(root, ".backlog"), result.Destination)
	assert.DirExists(t, result.Destination)
	assert.NoDirExists(t, source)
	assert.FileExists(t, filepath.Join(result.Destination, "config.yaml"))
	assert.FileExists(t, filepath.Join(result.Destination, "queue", "001-F.md"))
}

func TestMigrateWorkspaceDir_AlreadyDoneWhenBacklogAlreadyExists(t *testing.T) {
	root := t.TempDir()
	destination := filepath.Join(root, ".backlog")
	require.NoError(t, os.MkdirAll(destination, 0o755))
	require.NoError(t, config.WriteDefaults(destination))

	result, err := MigrateWorkspaceDir(root, MigrateWorkspaceDirOptions{})
	require.NoError(t, err)

	assert.True(t, result.AlreadyDone)
	assert.Equal(t, destination, result.Destination)
}

func TestMigrateWorkspaceDir_RefusesWhenBothRootsExist(t *testing.T) {
	root := t.TempDir()
	seedLegacyWorkspaceDir(t, root)
	destination := filepath.Join(root, ".backlog")
	require.NoError(t, os.MkdirAll(destination, 0o755))
	require.NoError(t, config.WriteDefaults(destination))

	_, err := MigrateWorkspaceDir(root, MigrateWorkspaceDirOptions{})
	require.Error(t, err)
}

func TestMigrateWorkspaceDir_NoSourceIsNoOp(t *testing.T) {
	root := t.TempDir()

	result, err := MigrateWorkspaceDir(root, MigrateWorkspaceDirOptions{})
	require.NoError(t, err)

	assert.False(t, result.AlreadyDone)
	assert.Equal(t, filepath.Join(root, ".backlogit"), result.Source)
	assert.Equal(t, filepath.Join(root, ".backlog"), result.Destination)
	assert.Empty(t, result.Files)
}

func seedLegacyWorkspaceDir(t *testing.T, root string) string {
	t.Helper()

	source := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(source, 0o755))
	require.NoError(t, config.WriteDefaults(source))
	require.NoError(t, os.WriteFile(filepath.Join(source, "queue", "001-F.md"), []byte("fixture"), 0o644))

	return source
}
