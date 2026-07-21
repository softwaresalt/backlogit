package cli_test

import (
	"context"
	"regexp"
	"strings"
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

	// Cover the SIZE projection independently of COMPOSITION: a seeded task row
	// must carry its standalone stored size in the SIZE column. Without this an
	// always-empty `size` cell would still pass the header/composition asserts
	// above (the "L"/"M" tokens there come from the feature's composition rollup).
	assertTaskSizeCell(t, out, "Sized task L", "L")
	assertTaskSizeCell(t, out, "Sized task M", "M")
}

// assertTaskSizeCell asserts the table row whose TITLE matches taskTitle carries
// wantSize as a standalone SIZE column cell — an exact field, not a substring of
// the title or of the aggregate composition summary. The table renderer
// separates columns with runs of two or more spaces, so splitting a data row on
// that boundary isolates each cell; the multi-word title stays a single field
// and can never equal a one-letter size, so an exact field match uniquely
// identifies the SIZE cell.
func assertTaskSizeCell(t *testing.T, out, taskTitle, wantSize string) {
	t.Helper()
	sep := regexp.MustCompile(`\s{2,}`)
	for _, line := range strings.Split(out, "\n") {
		if !strings.Contains(line, taskTitle) {
			continue
		}
		for _, f := range sep.Split(strings.TrimRight(line, " \t"), -1) {
			if f == wantSize {
				return
			}
		}
		t.Fatalf("task row %q missing standalone SIZE cell %q; line=%q", taskTitle, wantSize, line)
	}
	t.Fatalf("no table row found for task title %q in output:\n%s", taskTitle, out)
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

// TestListGroupedView_ShowsComposition asserts the human `list --group-by`
// surface also carries the computed-on-read composition summary for aggregate
// rows, at parity with the ungrouped table surface (114-F). The grouped renderer
// previously returned early through FormatGroupedView, whose row shape omitted
// both the size and composition projections.
func TestListGroupedView_ShowsComposition(t *testing.T) {
	root := setupCLIWorkspace(t)
	setupSizedFeature(t, root)

	out, err := runRootCommand(t, "--cwd", root, "list", "--group-by", "type")
	require.NoError(t, err)

	assert.Contains(t, out, "L:1", "grouped feature row must summarize composition (L:1); got: %s", out)
	assert.Contains(t, out, "M:1", "grouped feature row must summarize composition (M:1); got: %s", out)
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
