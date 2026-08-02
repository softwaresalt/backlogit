package core_test

import (
	"context"
	"fmt"
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
			return fmt.Errorf("read %s: %w", p, readErr)
		}
		fm, _, parseErr := models.ParseFrontmatter(string(data))
		if parseErr != nil {
			return fmt.Errorf("parse frontmatter %s: %w", p, parseErr)
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

// --- 134.003-T: Failing tests for AddShipmentBlock affordance (U3 red harness) ---

// TestAddShipmentBlock_CreatesBlocksEdgeBetweenShipments verifies that
// AddShipmentBlock creates a "blocks" edge between two shipments.
//
// Red harness: fails until U4 implements AddShipmentBlock (current stub returns an error).
func TestAddShipmentBlock_CreatesBlocksEdgeBetweenShipments(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	prereq, err := core.CreateShipment(ctx, ws, "Prerequisite shipment", nil)
	require.NoError(t, err)
	dependent, err := core.CreateShipment(ctx, ws, "Dependent shipment", nil)
	require.NoError(t, err)

	// "prereq must ship before dependent" = dependent depends_on prereq.
	err = core.AddShipmentBlock(ctx, ws, dependent.ID, prereq.ID)
	require.NoError(t, err, "AddShipmentBlock must succeed for two shipment endpoints")

	assert.True(t, edgeExists(t, ws, dependent.ID, prereq.ID),
		"blocks edge from dependent to prereq must be present in the cache after AddShipmentBlock")
}

// TestAddShipmentBlock_RejectsWhenDependentIsNotShipment verifies that
// AddShipmentBlock returns an error when the dependent endpoint is not a shipment.
//
// Red harness: fails until U4 implements AddShipmentBlock validation (stub returns
// a generic "not yet implemented" error, not the specific endpoint-type error).
func TestAddShipmentBlock_RejectsWhenDependentIsNotShipment(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	prereq, err := core.CreateShipment(ctx, ws, "Prerequisite shipment", nil)
	require.NoError(t, err)
	feat, err := core.CreateArtifact(ctx, ws, "Non-shipment feature", "feature")
	require.NoError(t, err)

	// dependent is a feature (not a shipment).
	err = core.AddShipmentBlock(ctx, ws, feat.ID, prereq.ID)
	require.Error(t, err,
		"AddShipmentBlock must return an error when dependent is not a shipment")
	// The error must not have created a stray edge.
	assert.False(t, edgeExists(t, ws, feat.ID, prereq.ID),
		"no edge must be created when AddShipmentBlock rejects non-shipment dependent")
}

// TestAddShipmentBlock_RejectsWhenPrerequisiteIsNotShipment verifies that
// AddShipmentBlock returns an error when the prerequisite endpoint is not a shipment.
//
// Red harness: fails until U4 implements AddShipmentBlock validation.
func TestAddShipmentBlock_RejectsWhenPrerequisiteIsNotShipment(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	dependent, err := core.CreateShipment(ctx, ws, "Dependent shipment", nil)
	require.NoError(t, err)
	feat, err := core.CreateArtifact(ctx, ws, "Non-shipment feature", "feature")
	require.NoError(t, err)

	// prerequisite is a feature (not a shipment).
	err = core.AddShipmentBlock(ctx, ws, dependent.ID, feat.ID)
	require.Error(t, err,
		"AddShipmentBlock must return an error when prerequisite is not a shipment")
	assert.False(t, edgeExists(t, ws, dependent.ID, feat.ID),
		"no edge must be created when AddShipmentBlock rejects non-shipment prerequisite")
}

// TestAddDependency_GenericPathAcceptsShipmentToNonShipmentEdge verifies that
// the generic AddDependency path is unchanged by the shipment-block affordance:
// a shipment→non-shipment edge is still valid via AddDependency.
//
// This is the additive-guard regression scenario: the guard lives only on
// AddShipmentBlock, not on AddDependency, so previously-valid edges must
// still succeed.
func TestAddDependency_GenericPathAcceptsShipmentToNonShipmentEdge(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	shipment, err := core.CreateShipment(ctx, ws, "Shipment with non-ship dep", nil)
	require.NoError(t, err)
	feat, err := core.CreateArtifact(ctx, ws, "Feature task", "feature")
	require.NoError(t, err)

	// A shipment→feature blocks edge via generic AddDependency must succeed.
	err = core.AddDependency(ctx, ws, shipment.ID, feat.ID, "blocks")
	require.NoError(t, err,
		"generic AddDependency must still accept a shipment→non-shipment edge (additive guard)")
	assert.True(t, edgeExists(t, ws, shipment.ID, feat.ID),
		"edge must exist after generic AddDependency for shipment→non-shipment")
}

// TestAddShipmentBlock_SurvivesSyncIndex verifies that a blocking edge created by
// AddShipmentBlock persists through frontmatter and survives a full index rebuild.
//
// Red harness: fails until U4 implements AddShipmentBlock (stub errors out, so
// no edge is created to survive).
func TestAddShipmentBlock_SurvivesSyncIndex(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	prereq, err := core.CreateShipment(ctx, ws, "Prereq shipment for sync", nil)
	require.NoError(t, err)
	dependent, err := core.CreateShipment(ctx, ws, "Dependent shipment for sync", nil)
	require.NoError(t, err)

	require.NoError(t, core.AddShipmentBlock(ctx, ws, dependent.ID, prereq.ID),
		"AddShipmentBlock must succeed for two shipment endpoints")

	// Simulate backlogit sync_index: clear item_deps and rebuild from frontmatter.
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)

	assert.True(t, edgeExists(t, ws, dependent.ID, prereq.ID),
		"blocks edge must survive sync_index (Rehydrate rebuilds item_deps from frontmatter)")
}
