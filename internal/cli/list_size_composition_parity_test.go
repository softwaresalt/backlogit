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

// TestListJSON_FeatureExposesSizeComposition asserts CLI `list --json` attaches
// the computed-on-read size_composition rollup to each aggregate (feature) row,
// at parity with the MCP list_items tool. Both transports route through the
// shared core list shaper so they cannot drift (117-F / 60336CC0).
func TestListJSON_FeatureExposesSizeComposition(t *testing.T) {
	root := setupCLIWorkspace(t)
	featID := setupSizedFeature(t, root)

	out, err := runRootCommand(t, "--cwd", root, "list", "--json")
	require.NoError(t, err)

	var rows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &rows), "list --json must be a valid JSON array: %s", out)

	var feature map[string]any
	tasksSeen := 0
	for _, r := range rows {
		switch r["artifact_type"] {
		case "feature":
			if r["id"] == featID {
				feature = r
			}
		case "task":
			tasksSeen++
			_, hasComp := r["size_composition"]
			assert.False(t, hasComp, "task list row must NOT carry size_composition; got: %v", r)
		}
	}

	require.NotNil(t, feature, "feature %s not present in list --json: %s", featID, out)
	comp, ok := feature["size_composition"].(map[string]any)
	require.True(t, ok, "feature list row must carry size_composition; got: %s", out)
	hist, _ := comp["histogram"].(map[string]any)
	assert.EqualValues(t, 1, hist["L"], "histogram L")
	assert.EqualValues(t, 1, hist["M"], "histogram M")
	assert.GreaterOrEqual(t, tasksSeen, 2, "both child tasks present in listing")
}

// TestListJSON_ShipmentExposesSizeComposition asserts CLI `list --json` also
// attaches the rollup to shipment rows, matching the aggregate predicate used by
// the get/queue surfaces.
func TestListJSON_ShipmentExposesSizeComposition(t *testing.T) {
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
	ship, err := core.CreateShipment(ctx, ws, "List parity shipment", []string{task.ID})
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)
	require.NoError(t, ws.Close())

	out, err := runRootCommand(t, "--cwd", root, "list", "--json")
	require.NoError(t, err)

	var rows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &rows), "list --json must be a valid JSON array: %s", out)

	found := false
	for _, r := range rows {
		if r["id"] == ship.ID {
			found = true
			comp, ok := r["size_composition"].(map[string]any)
			require.True(t, ok, "shipment list row must carry size_composition; got: %s", out)
			hist, _ := comp["histogram"].(map[string]any)
			assert.EqualValues(t, 1, hist["XL"], "histogram XL")
		}
	}
	require.True(t, found, "shipment %s not present in list --json: %s", ship.ID, out)
}

// TestListJSON_SizeComposition_MatchesCanonical locks the CLI `list --json`
// size_composition projection to the canonical core.SizeComposition rollup —
// histogram, unsized count, de-duplicated members, and (never-null) arrays — so
// the CLI surface cannot drift from the single source of truth that the MCP
// list_items surface also projects. The fixture includes an unsized child so the
// Unsized accounting is exercised on the read surface (117-F / 60336CC0;
// compound cli-mcp parity: [] never null, compare both surfaces to canonical).
func TestListJSON_SizeComposition_MatchesCanonical(t *testing.T) {
	root := setupCLIWorkspace(t)
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	feat, err := core.CreateArtifact(ctx, ws, "Canonical feature", "feature")
	require.NoError(t, err)
	tL, err := core.CreateArtifact(ctx, ws, "Task L", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	tM, err := core.CreateArtifact(ctx, ws, "Task M", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	_, err = core.CreateArtifact(ctx, ws, "Task unsized", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	_, err = core.SetArtifactSize(ctx, ws, tL.ID, "L")
	require.NoError(t, err)
	_, err = core.SetArtifactSize(ctx, ws, tM.ID, "M")
	require.NoError(t, err)
	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)

	canonical, err := core.SizeComposition(ctx, ws, feat)
	require.NoError(t, err)
	require.NoError(t, ws.Close())

	rawCanon, err := json.Marshal(canonical)
	require.NoError(t, err)
	var wantComp map[string]any
	require.NoError(t, json.Unmarshal(rawCanon, &wantComp))

	out, err := runRootCommand(t, "--cwd", root, "list", "--json")
	require.NoError(t, err)
	var rows []map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &rows), "list --json must be a valid JSON array: %s", out)

	var gotComp map[string]any
	for _, r := range rows {
		if r["id"] == feat.ID {
			gc, ok := r["size_composition"].(map[string]any)
			require.True(t, ok, "feature list row must carry size_composition; got: %s", out)
			gotComp = gc
		}
	}
	require.NotNil(t, gotComp, "feature %s not present in list --json: %s", feat.ID, out)
	assert.Equal(t, wantComp, gotComp, "list --json size_composition must equal the canonical core rollup")
	assert.EqualValues(t, 1, gotComp["unsized"], "unsized child must be reflected on the read surface")
}
