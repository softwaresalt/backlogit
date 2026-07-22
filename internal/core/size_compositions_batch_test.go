package core_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

// initBatchWorkspace writes default config into root/.backlogit so
// core.NewWorkspace can open the workspace in these batched-rollup tests.
func initBatchWorkspace(t *testing.T, root string) {
	t.Helper()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))
}

// setupBatchComposition builds a workspace with two sized features and a shipment
// wrapping the first feature's tasks, rehydrates the index, and returns the
// workspace plus the aggregate artifacts and a lone task. Used to exercise the
// batched size-composition rollup at parity with the per-artifact path (117-F).
func setupBatchComposition(t *testing.T, root string) (f1, f2, ship, task *models.Artifact) {
	t.Helper()
	ctx := context.Background()
	initBatchWorkspace(t, root)
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	defer ws.Close()

	f1, err = core.CreateArtifact(ctx, ws, "Feature one", "feature")
	require.NoError(t, err)
	tL, err := core.CreateArtifact(ctx, ws, "Task L", "task", core.WithParent(f1.ID))
	require.NoError(t, err)
	tM, err := core.CreateArtifact(ctx, ws, "Task M", "task", core.WithParent(f1.ID))
	require.NoError(t, err)
	_, err = core.SetArtifactSize(ctx, ws, tL.ID, "L")
	require.NoError(t, err)
	_, err = core.SetArtifactSize(ctx, ws, tM.ID, "M")
	require.NoError(t, err)

	f2, err = core.CreateArtifact(ctx, ws, "Feature two", "feature")
	require.NoError(t, err)
	tS, err := core.CreateArtifact(ctx, ws, "Task S", "task", core.WithParent(f2.ID))
	require.NoError(t, err)
	_, err = core.SetArtifactSize(ctx, ws, tS.ID, "S")
	require.NoError(t, err)

	ship, err = core.CreateShipment(ctx, ws, "Batch shipment", []string{tL.ID, tM.ID})
	require.NoError(t, err)

	task = tL

	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)
	return f1, f2, ship, task
}

// TestSizeCompositions_MatchesPerArtifact asserts the batched
// core.SizeCompositions produces, for every aggregate, a rollup identical to the
// per-artifact core.SizeComposition — so the batched read-surface shaper can
// replace the per-aggregate fan-out without changing output (117-F / A6A1B47E).
func TestSizeCompositions_MatchesPerArtifact(t *testing.T) {
	root := t.TempDir()
	f1, f2, ship, task := setupBatchComposition(t, root)

	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	defer ws.Close()

	comps, err := core.SizeCompositions(ctx, ws, []*models.Artifact{f1, f2, ship, task, nil})
	require.NoError(t, err)

	for _, agg := range []*models.Artifact{f1, f2, ship} {
		single, err := core.SizeComposition(ctx, ws, agg)
		require.NoError(t, err)
		require.Contains(t, comps, agg.ID, "aggregate %s must be present in batched map", agg.ID)
		assert.Equal(t, single, comps[agg.ID], "batched rollup must equal per-artifact rollup for %s", agg.ID)
	}

	assert.NotContains(t, comps, task.ID, "a non-aggregate (task) must be absent from the batched map")
}

// TestSizeCompositions_EmptyInput asserts an empty slice yields an empty,
// non-nil map without error.
func TestSizeCompositions_EmptyInput(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	initBatchWorkspace(t, root)
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	defer ws.Close()

	comps, err := core.SizeCompositions(ctx, ws, nil)
	require.NoError(t, err)
	assert.NotNil(t, comps)
	assert.Empty(t, comps)
}
