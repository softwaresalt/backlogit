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
	// tU is left unsized on purpose: an existing-but-unsized member must
	// increment Unsized (never a histogram bucket) identically on the batched and
	// per-artifact paths (117-F / A6A1B47E).
	_, err = core.CreateArtifact(ctx, ws, "Task unsized", "task", core.WithParent(f1.ID))
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

	// The shipment references two direct tasks plus a FEATURE (f2), which must be
	// expanded into its child tasks and never counted itself. This exercises the
	// shipment feature-expansion path on both rollup paths.
	ship, err = core.CreateShipment(ctx, ws, "Batch shipment", []string{tL.ID, tM.ID, f2.ID})
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

// TestSizeCompositions_ShipmentDanglingAndFeatureMember asserts the batched
// rollup matches the per-artifact rollup for a shipment whose manifest mixes a
// directly-listed task, a feature (expanded into its child tasks and never
// counted itself), and a dangling id (warn-skipped: counted in neither the
// histogram nor the unsized total, but surfaced under Skipped). The shipment is
// upserted directly to bypass CreateShipment manifest validation so the
// dangling-member semantics can be exercised on both paths (117-F / A6A1B47E;
// ratified composition semantics).
func TestSizeCompositions_ShipmentDanglingAndFeatureMember(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	initBatchWorkspace(t, root)
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	defer ws.Close()

	featA, err := core.CreateArtifact(ctx, ws, "Expanded feature", "feature")
	require.NoError(t, err)
	childA, err := core.CreateArtifact(ctx, ws, "Child M", "task", core.WithParent(featA.ID))
	require.NoError(t, err)
	_, err = core.SetArtifactSize(ctx, ws, childA.ID, "M")
	require.NoError(t, err)

	featB, err := core.CreateArtifact(ctx, ws, "Direct owner", "feature")
	require.NoError(t, err)
	directTask, err := core.CreateArtifact(ctx, ws, "Direct S", "task", core.WithParent(featB.ID))
	require.NoError(t, err)
	_, err = core.SetArtifactSize(ctx, ws, directTask.ID, "S")
	require.NoError(t, err)

	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)

	ship := &models.Artifact{
		ID:           "998-S",
		Title:        "Dangling manifest shipment",
		Status:       models.StatusActive,
		ArtifactType: "shipment",
		CustomFields: map[string]any{"items": []string{directTask.ID, featA.ID, "999.999-T"}},
	}
	require.NoError(t, db.UpsertItem(ctx, ws.DB, ship))

	single, err := core.SizeComposition(ctx, ws, ship)
	require.NoError(t, err)
	comps, err := core.SizeCompositions(ctx, ws, []*models.Artifact{ship})
	require.NoError(t, err)
	require.Contains(t, comps, ship.ID)
	assert.Equal(t, single, comps[ship.ID], "batched rollup must equal per-artifact for a dangling+feature manifest")

	assert.EqualValues(t, 1, single.Histogram["S"], "directly-listed task S must be counted")
	assert.EqualValues(t, 1, single.Histogram["M"], "expanded feature child M must be counted")
	assert.Equal(t, 0, single.Unsized, "a dangling id must not increment unsized")
	assert.Contains(t, single.Skipped, "999.999-T", "a dangling manifest id must be surfaced as skipped")
}

// TestQueueViewWithSizeComposition_DegradesOnRollupError asserts the shared queue
// shaper returns the unprojected payload (rather than an error) when the rollup
// batch fails, so the CLI `queue view --json` and MCP get_queue surfaces degrade
// identically instead of aborting. This locks the consolidated degradation
// contract that replaced the CLI abort / MCP degrade asymmetry (117-F review:
// Arch-P1 + Go/Parity-P2).
func TestQueueViewWithSizeComposition_DegradesOnRollupError(t *testing.T) {
	root := t.TempDir()
	ctx := context.Background()
	initBatchWorkspace(t, root)
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)

	feat, err := core.CreateArtifact(ctx, ws, "Queue feature", "feature")
	require.NoError(t, err)
	view := &core.QueueView{Items: []*models.Artifact{feat}}

	// Force a rollup failure by closing the DB the batched resolver queries; the
	// in-memory view still marshals, so the shaper must degrade, not error.
	require.NoError(t, ws.Close())

	payload, err := core.QueueViewWithSizeComposition(ctx, ws, view)
	require.NoError(t, err, "queue shaper must degrade gracefully, not error, on rollup failure")
	require.NotNil(t, payload)
	items, ok := payload["items"].([]any)
	require.True(t, ok, "degraded payload must still carry the queue items")
	require.Len(t, items, 1)
	im, ok := items[0].(map[string]any)
	require.True(t, ok)
	_, hasComp := im[core.SizeCompositionKey]
	assert.False(t, hasComp, "on rollup failure the aggregate must be left unprojected")
}
