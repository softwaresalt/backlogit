package cli_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
)

// TestListTable_ShowsSizeAndCompositionColumns asserts the human `list` table
// surface exposes a SIZE column and a COMPOSITION rollup column at parity with
// the JSON read surfaces (114-F / D5FA1EE9).
func TestListTable_ShowsSizeAndCompositionColumns(t *testing.T) {
	root := setupCLIWorkspace(t)
	setupSizedFeature(t, root)

	out, err := runRootCommand(t, "--cwd", root, "list", "--format", "table")
	require.NoError(t, err)

	assert.Contains(t, out, "SIZE", "list table must include a SIZE column header; got: %s", out)
	assert.Contains(t, out, "COMPOSITION", "list table must include a COMPOSITION column header; got: %s", out)
	assert.Contains(t, out, "L:1", "feature row must summarize composition (L:1); got: %s", out)
	assert.Contains(t, out, "M:1", "feature row must summarize composition (M:1); got: %s", out)
}

// TestQueueViewTable_ShowsCompositionColumn asserts the human `queue view` table
// surface exposes the size rollup, at parity with `queue view --json`.
func TestQueueViewTable_ShowsCompositionColumn(t *testing.T) {
	root := setupCLIWorkspace(t)
	setupSizedFeature(t, root)

	out, err := runRootCommand(t, "--cwd", root, "queue", "view", "--format", "table")
	require.NoError(t, err)

	assert.Contains(t, out, "COMPOSITION", "queue view table must include a COMPOSITION column header; got: %s", out)
	assert.Contains(t, out, "L:1", "feature queue row must summarize composition (L:1); got: %s", out)
	assert.Contains(t, out, "M:1", "feature queue row must summarize composition (M:1); got: %s", out)
}

// TestShipmentListTable_ShowsSizeAndComposition asserts the human `shipment list`
// table surface exposes SIZE and COMPOSITION columns for shipments.
func TestShipmentListTable_ShowsSizeAndComposition(t *testing.T) {
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
	_, err = core.CreateShipment(ctx, ws, "Parity shipment", []string{task.ID})
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)
	ws.Close()

	out, err := runRootCommand(t, "--cwd", root, "shipment", "list", "--format", "table")
	require.NoError(t, err)

	assert.Contains(t, out, "SIZE", "shipment list table must include a SIZE column header; got: %s", out)
	assert.Contains(t, out, "COMPOSITION", "shipment list table must include a COMPOSITION column header; got: %s", out)
	assert.Contains(t, out, "XL:1", "shipment row must summarize composition (XL:1); got: %s", out)
}
