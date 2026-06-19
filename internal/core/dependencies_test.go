package core_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

// edgeExists reports whether item itemID depends on dependsOn in the cache.
func edgeExists(t *testing.T, ws *core.Workspace, itemID, dependsOn string) bool {
	t.Helper()
	ctx := context.Background()
	edges, err := db.GetDependencies(ctx, ws.DB, itemID)
	require.NoError(t, err)
	for _, e := range edges {
		if e.DependsOn == dependsOn {
			return true
		}
	}
	return false
}

// TestAddDependency_PersistsToFrontmatterAndSurvivesSync guards against the
// regression where dependency edges were written only to the disposable
// SQLite cache and lost on sync_index (Rehydrate rebuilds item_deps solely
// from each artifact's Markdown frontmatter).
func TestAddDependency_PersistsToFrontmatterAndSurvivesSync(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feat, err := core.CreateArtifact(ctx, ws, "Dep feature", "feature")
	require.NoError(t, err)
	source, err := core.CreateArtifact(ctx, ws, "Source task", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	target, err := core.CreateArtifact(ctx, ws, "Target task", "task", core.WithParent(feat.ID))
	require.NoError(t, err)

	require.NoError(t, core.AddDependency(ctx, ws, source.ID, target.ID, "blocks"))

	assert.True(t, edgeExists(t, ws, source.ID, target.ID),
		"edge must be present in the cache immediately after AddDependency")

	// Simulate `backlogit sync` / backlogit_sync_index, which clears item_deps
	// and rebuilds it from Markdown frontmatter only.
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)

	assert.True(t, edgeExists(t, ws, source.ID, target.ID),
		"edge must survive sync_index because it was persisted to frontmatter")
}

// TestRemoveDependency_RemovesFromFrontmatter ensures removal also clears the
// frontmatter so the edge does not reappear on the next sync.
func TestRemoveDependency_RemovesFromFrontmatter(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feat, err := core.CreateArtifact(ctx, ws, "Dep feature", "feature")
	require.NoError(t, err)
	source, err := core.CreateArtifact(ctx, ws, "Source task", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	target, err := core.CreateArtifact(ctx, ws, "Target task", "task", core.WithParent(feat.ID))
	require.NoError(t, err)

	require.NoError(t, core.AddDependency(ctx, ws, source.ID, target.ID, "blocks"))
	require.NoError(t, core.RemoveDependency(ctx, ws, source.ID, target.ID))

	assert.False(t, edgeExists(t, ws, source.ID, target.ID),
		"edge must be gone from the cache after RemoveDependency")

	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)

	assert.False(t, edgeExists(t, ws, source.ID, target.ID),
		"edge must not reappear after sync_index once removed from frontmatter")
}

// artifactFilePath returns the on-disk Markdown path for the artifact with the
// given ID by scanning the workspace storage tree.
func artifactFilePath(t *testing.T, ws *core.Workspace, id string) string {
	t.Helper()
	root := filepath.Join(ws.RootPath, ".backlogit")
	var found string
	walkErr := filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || filepath.Ext(p) != ".md" {
			return err
		}
		data, readErr := os.ReadFile(p)
		if readErr != nil {
			return nil
		}
		fm, _, parseErr := models.ParseFrontmatter(string(data))
		if parseErr != nil {
			return nil
		}
		if idv, _ := fm["id"].(string); idv == id {
			found = p
			return filepath.SkipAll
		}
		return nil
	})
	require.NoError(t, walkErr)
	require.NotEmpty(t, found, "artifact file for %s not found", id)
	return found
}

// TestRemoveDependency_RollsBackCacheWhenFrontmatterUpdateFails guards the
// consistency invariant: if the cache edge is deleted but the frontmatter
// cannot be updated, the cache deletion must be rolled back so a later
// Rehydrate does not resurrect the edge into an inconsistent state.
func TestRemoveDependency_RollsBackCacheWhenFrontmatterUpdateFails(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	feat, err := core.CreateArtifact(ctx, ws, "Dep feature", "feature")
	require.NoError(t, err)
	source, err := core.CreateArtifact(ctx, ws, "Source task", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	target, err := core.CreateArtifact(ctx, ws, "Target task", "task", core.WithParent(feat.ID))
	require.NoError(t, err)

	require.NoError(t, core.AddDependency(ctx, ws, source.ID, target.ID, "blocks"))
	require.True(t, edgeExists(t, ws, source.ID, target.ID))

	// Remove the source artifact's Markdown file so the frontmatter update path
	// fails after the cache edge has already been deleted.
	require.NoError(t, os.Remove(artifactFilePath(t, ws, source.ID)))

	err = core.RemoveDependency(ctx, ws, source.ID, target.ID)
	require.Error(t, err, "removal must fail when the source artifact cannot be loaded")

	assert.True(t, edgeExists(t, ws, source.ID, target.ID),
		"cache edge must be rolled back when frontmatter cannot be updated")
}
