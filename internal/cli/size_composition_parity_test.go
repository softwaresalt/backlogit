package cli_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
)

// setupSizedFeature creates a feature with two sized child tasks (L and M),
// rehydrates the index, and returns the feature ID. Used to exercise the
// computed-on-read size_composition rollup on CLI read surfaces (114-F).
func setupSizedFeature(t *testing.T, root string) string {
	t.Helper()
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	defer ws.Close()

	feat, err := core.CreateArtifact(ctx, ws, "Composition feature", "feature")
	require.NoError(t, err)
	taskL, err := core.CreateArtifact(ctx, ws, "Sized task L", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	taskM, err := core.CreateArtifact(ctx, ws, "Sized task M", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	_, err = core.SetArtifactSize(ctx, ws, taskL.ID, "L")
	require.NoError(t, err)
	_, err = core.SetArtifactSize(ctx, ws, taskM.ID, "M")
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)
	return feat.ID
}

// TestGetJSON_FeatureExposesSizeComposition asserts CLI `get <feature> --json`
// carries the derived size_composition rollup, at parity with MCP get_item.
func TestGetJSON_FeatureExposesSizeComposition(t *testing.T) {
	root := setupCLIWorkspace(t)
	featID := setupSizedFeature(t, root)

	out, err := runRootCommand(t, "--cwd", root, "get", featID, "--json")
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &m), "get --json must be valid JSON: %s", out)
	comp, ok := m["size_composition"].(map[string]any)
	require.True(t, ok, "feature get --json must expose size_composition; got: %s", out)

	hist, _ := comp["histogram"].(map[string]any)
	assert.EqualValues(t, 1, hist["L"], "histogram L")
	assert.EqualValues(t, 1, hist["M"], "histogram M")
	assert.EqualValues(t, 0, comp["unsized"], "unsized count")
	members, _ := comp["members"].([]any)
	assert.Len(t, members, 2, "two task members")
}

// TestGetJSON_TaskOmitsSizeComposition asserts a non-aggregate artifact (task)
// does NOT carry size_composition, matching MCP get_item which only projects
// feature/shipment.
func TestGetJSON_TaskOmitsSizeComposition(t *testing.T) {
	root := setupCLIWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	feat, err := core.CreateArtifact(ctx, ws, "F", "feature")
	require.NoError(t, err)
	task, err := core.CreateArtifact(ctx, ws, "T", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	_, err = core.SetArtifactSize(ctx, ws, task.ID, "S")
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)
	ws.Close()

	out, err := runRootCommand(t, "--cwd", root, "get", task.ID, "--json")
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &m))
	_, ok := m["size_composition"]
	assert.False(t, ok, "task get --json must NOT expose size_composition; got: %s", out)
}

// TestGetJSON_ShipmentExposesSizeComposition asserts CLI `get <shipment> --json`
// carries the derived rollup, at parity with MCP get_shipment/get_item.
func TestGetJSON_ShipmentExposesSizeComposition(t *testing.T) {
	root := setupCLIWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	feat, err := core.CreateArtifact(ctx, ws, "Ship feature", "feature")
	require.NoError(t, err)
	task, err := core.CreateArtifact(ctx, ws, "Ship task", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	_, err = core.SetArtifactSize(ctx, ws, task.ID, "XL")
	require.NoError(t, err)
	ship, err := core.CreateShipment(ctx, ws, "Parity shipment", []string{task.ID})
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)
	ws.Close()

	out, err := runRootCommand(t, "--cwd", root, "get", ship.ID, "--json")
	require.NoError(t, err)

	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &m), "get --json must be valid JSON: %s", out)
	comp, ok := m["size_composition"].(map[string]any)
	require.True(t, ok, "shipment get --json must expose size_composition; got: %s", out)
	hist, _ := comp["histogram"].(map[string]any)
	assert.EqualValues(t, 1, hist["XL"], "histogram XL")
}

// TestQueueViewJSON_FeatureExposesSizeComposition asserts CLI `queue view --json`
// injects size_composition into each aggregate (feature/shipment) queue item, at
// parity with MCP get_queue.
func TestQueueViewJSON_FeatureExposesSizeComposition(t *testing.T) {
	root := setupCLIWorkspace(t)
	featID := setupSizedFeature(t, root)

	out, err := runRootCommand(t, "--cwd", root, "queue", "view", "--format", "json")
	require.NoError(t, err)

	var view map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &view), "queue view --json must be valid JSON: %s", out)
	items, ok := view["items"].([]any)
	require.True(t, ok, "queue view must have items array: %s", out)

	found := false
	for _, it := range items {
		im, ok := it.(map[string]any)
		if !ok {
			continue
		}
		if im["id"] == featID {
			found = true
			_, hasComp := im["size_composition"]
			assert.True(t, hasComp, "feature queue item must carry size_composition; got: %s", out)
		}
	}
	require.True(t, found, "feature %s not present in queue view: %s", featID, out)
}
