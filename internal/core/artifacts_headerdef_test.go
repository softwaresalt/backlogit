package core_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// countArtifactFiles walks the workspace storage root and returns the number of
// on-disk artifact markdown files. It is used to assert that a fail-closed write
// persisted nothing.
func countArtifactFiles(t *testing.T, ws *core.Workspace) int {
	t.Helper()
	root := core.WorkspaceStorageRoot(ws.RootPath)
	count := 0
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if filepath.Ext(path) == ".md" {
			count++
		}
		return nil
	})
	require.NoError(t, err)
	return count
}

// findArtifactFile walks the workspace storage root and returns the on-disk path
// of the artifact file named "<id>.md", or "" if it is not present.
func findArtifactFile(t *testing.T, ws *core.Workspace, id string) string {
	t.Helper()
	root := core.WorkspaceStorageRoot(ws.RootPath)
	want := id + ".md"
	found := ""
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && d.Name() == want {
			found = path
		}
		return nil
	})
	require.NoError(t, err)
	return found
}

// TestCreateArtifact_NilHeaderDefFailsClosed proves that CreateArtifact fails
// closed (ErrConfig, not ErrValidation) when the workspace header-def schema is
// absent, persisting no artifact — while the loaded-schema path still succeeds.
// Covers AC1, AC2, AC3, AC4 for the create write path.
func TestCreateArtifact_NilHeaderDefFailsClosed(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	// Loaded-HeaderDef regression guard: the normal (schema-present) create path
	// still succeeds and persists.
	loaded, err := core.CreateArtifact(ctx, ws, "Loaded feature", "feature")
	require.NoError(t, err)
	require.NotNil(t, loaded)
	require.FileExists(t, findArtifactFile(t, ws, loaded.ID))

	// Snapshot the persisted-file count immediately before the nil-schema create.
	before := countArtifactFiles(t, ws)

	// Force the defensive branch: absent workspace schema.
	ws.HeaderDef = nil

	got, err := core.CreateArtifact(ctx, ws, "Nil feature", "feature")

	// AC1: fail closed with a non-nil error wrapping ErrConfig, not ErrValidation.
	require.Error(t, err)
	assert.Nil(t, got)
	assert.True(t, errors.Is(err, blerrors.ErrConfig),
		"nil-HeaderDef create must wrap blerrors.ErrConfig (system/config fault → MCP internal); got: %v", err)
	assert.False(t, errors.Is(err, blerrors.ErrValidation),
		"nil-HeaderDef create must NOT wrap blerrors.ErrValidation (would mis-map to validation_failed/422); got: %v", err)

	// AC2: no artifact file was persisted — the write failed closed before persist.
	after := countArtifactFiles(t, ws)
	assert.Equal(t, before, after, "nil-HeaderDef create must persist no artifact file")
}

// TestUpdateArtifact_NilHeaderDefFailsClosed proves that UpdateArtifact fails
// closed (ErrConfig, not ErrValidation) when the workspace header-def schema is
// absent, persisting no mutation — while the loaded-schema update still succeeds.
// Covers AC1, AC2, AC3, AC4 for the update write path.
func TestUpdateArtifact_NilHeaderDefFailsClosed(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	// Create the update target with the loaded HeaderDef.
	feat, err := core.CreateArtifact(ctx, ws, "Update target", "feature")
	require.NoError(t, err)
	require.NotNil(t, feat)

	// Loaded-HeaderDef regression guard: the normal update path still succeeds
	// and persists.
	_, err = core.UpdateArtifact(ctx, ws, feat.ID, map[string]any{"title": "Loaded rename"})
	require.NoError(t, err)

	path := findArtifactFile(t, ws, feat.ID)
	require.NotEmpty(t, path, "artifact file must exist after loaded update")
	baseline, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(baseline), "Loaded rename", "loaded update must persist the new title")

	// Force the defensive branch: absent workspace schema.
	ws.HeaderDef = nil

	got, err := core.UpdateArtifact(ctx, ws, feat.ID, map[string]any{"title": "Renamed"})

	// AC1: fail closed with a non-nil error wrapping ErrConfig, not ErrValidation.
	require.Error(t, err)
	assert.Nil(t, got)
	assert.True(t, errors.Is(err, blerrors.ErrConfig),
		"nil-HeaderDef update must wrap blerrors.ErrConfig (system/config fault → MCP internal); got: %v", err)
	assert.False(t, errors.Is(err, blerrors.ErrValidation),
		"nil-HeaderDef update must NOT wrap blerrors.ErrValidation (would mis-map to validation_failed/422); got: %v", err)

	// AC2: the on-disk artifact is unchanged — the mutation failed closed before persist.
	afterBytes, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(baseline), string(afterBytes),
		"nil-HeaderDef update must not persist any mutation")
	assert.NotContains(t, string(afterBytes), "Renamed",
		"nil-HeaderDef update must not persist the attempted rename")
}
