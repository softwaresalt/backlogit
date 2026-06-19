package core_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
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
