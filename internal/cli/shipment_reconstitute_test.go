package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	bldb "github.com/softwaresalt/backlogit/internal/db"
	mcpinternal "github.com/softwaresalt/backlogit/internal/mcp"
)

// reconstituteSplitScenario is the 201-S shape: 190-S <- 201-S <- 198-S, where
// 201-S holds 11 tasks that must split into six chained shipments.
type reconstituteSplitScenario struct {
	root        string
	predecessor string
	source      string
	downstream  string
	feature     string
	tasks       []string
}

func buildReconstituteSplitScenario(t *testing.T) *reconstituteSplitScenario {
	t.Helper()
	ctx := context.Background()
	root, ws := setupGovernedWorkspace(t)
	sc := &reconstituteSplitScenario{root: root}

	feature, err := core.CreateArtifact(ctx, ws, "201-S covering feature", "feature")
	require.NoError(t, err)
	require.NoError(t, bldb.UpsertItem(ctx, ws.DB, feature))
	sc.feature = feature.ID

	ids := make([]string, 0, 11)
	for i := 1; i <= 11; i++ {
		task, taskErr := core.CreateArtifact(ctx, ws, fmt.Sprintf("201-S task %d", i), "task", core.WithParent(feature.ID))
		require.NoError(t, taskErr)
		require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))
		sc.tasks = append(sc.tasks, task.ID)
		ids = append(ids, task.ID)
	}

	neighbor := func(title string) string {
		neighborFeature, featureErr := core.CreateArtifact(ctx, ws, title+" feature", "feature")
		require.NoError(t, featureErr)
		require.NoError(t, bldb.UpsertItem(ctx, ws.DB, neighborFeature))
		task, taskErr := core.CreateArtifact(ctx, ws, title, "task", core.WithParent(neighborFeature.ID))
		require.NoError(t, taskErr)
		require.NoError(t, bldb.UpsertItem(ctx, ws.DB, task))
		return task.ID
	}
	predecessor, err := core.CreateShipment(ctx, ws, "190-S predecessor", []string{neighbor("190-S task")})
	require.NoError(t, err)
	_, err = core.UpdateArtifact(ctx, ws, predecessor.ID, map[string]any{"labels": []string{"dag-root"}})
	require.NoError(t, err)
	source, err := core.CreateShipment(ctx, ws, "201-S source", ids)
	require.NoError(t, err)
	downstream, err := core.CreateShipment(ctx, ws, "198-S downstream", []string{neighbor("198-S task")})
	require.NoError(t, err)
	require.NoError(t, core.AddDependency(ctx, ws, source.ID, predecessor.ID, "blocks"))
	require.NoError(t, core.AddDependency(ctx, ws, downstream.ID, source.ID, "blocks"))

	sc.predecessor, sc.source, sc.downstream = predecessor.ID, source.ID, downstream.ID
	require.NoError(t, ws.Close())
	return sc
}

// planYAML splits the source into five new shipments (S2..S6, 9 re-parented
// tasks under 5 new features) and keeps the source ID for the final two tasks.
func (sc *reconstituteSplitScenario) planYAML() string {
	t := sc.tasks
	var b strings.Builder
	fmt.Fprintf(&b, "source_shipments: [%s]\nshipments:\n", sc.source)
	groups := [][]string{{t[0], t[1]}, {t[2], t[3]}, {t[4], t[5]}, {t[6], t[7]}, {t[8]}}
	for i, group := range groups {
		blocker := sc.predecessor
		if i > 0 {
			blocker = fmt.Sprintf("s%d", i+1)
		}
		fmt.Fprintf(&b, "  - label: s%d\n    title: 201-S split %d\n    feature:\n      create:\n        title: Split feature %d\n        related_to: %s\n    items: [%s]\n    blocks_on: [%s]\n",
			i+2, i+2, i+2, sc.feature, strings.Join(group, ", "), blocker)
	}
	fmt.Fprintf(&b, "  - id: %s\n    keep_id: true\n    feature: {id: %s}\n    items: [%s, %s]\n    blocks_on: [s6]\n",
		sc.source, sc.feature, t[9], t[10])
	return b.String()
}

func runReconstituteCLI(t *testing.T, root string, args ...string) (string, error) {
	t.Helper()
	cmd := NewRootCommand()
	out := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(errBuf)
	cmd.SetArgs(append([]string{"--cwd", root}, args...))
	err := cmd.Execute()
	if err != nil {
		return out.String() + errBuf.String(), err
	}
	return out.String(), nil
}

func reconstituteStorageSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()
	base := filepath.Join(root, ".backlogit")
	snapshot := map[string]string{}
	require.NoError(t, filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if ext := filepath.Ext(path); ext != ".md" && ext != ".jsonl" {
			return nil
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		rel, relErr := filepath.Rel(base, path)
		if relErr != nil {
			return relErr
		}
		snapshot[filepath.ToSlash(rel)] = string(data)
		return nil
	}))
	return snapshot
}

func invokeReconstituteMCP(t *testing.T, root string, arguments map[string]any) core.ReconstituteShipmentResult {
	t.Helper()
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ws.Close() })

	const tool = "backlogit_reconstitute_shipment"
	request := mcplib.CallToolRequest{}
	request.Params.Name = tool
	request.Params.Arguments = arguments
	result, err := mcpinternal.NewServer(ws).InvokeTool(ctx, tool, request)
	require.NoError(t, err, "MCP tool %s must be registered and succeed", tool)
	require.NotNil(t, result)
	require.False(t, result.IsError, "MCP tool %s returned an error result", tool)
	require.NotEmpty(t, result.Content)
	text, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok, "MCP result must be text content")

	var out core.ReconstituteShipmentResult
	require.NoError(t, json.Unmarshal([]byte(text.Text), &out), "MCP payload: %s", text.Text)
	return out
}

// TestU3_ShipmentReconstituteCLIMCPParityAnd201SScenario pins the CLI and MCP
// reconstitute surfaces, the registry mapping, and the 201-S one-to-six split.
func TestU3_ShipmentReconstituteCLIMCPParityAnd201SScenario(t *testing.T) {
	ctx := context.Background()
	sc := buildReconstituteSplitScenario(t)
	plan := sc.planYAML()
	planPath := filepath.Join(sc.root, "plans", "201-s-split.yaml")
	require.NoError(t, os.MkdirAll(filepath.Dir(planPath), 0o755))
	require.NoError(t, os.WriteFile(planPath, []byte(plan), 0o644))

	// The plan must be valid against the implemented core engine.
	coreWS, err := core.NewWorkspace(ctx, sc.root)
	require.NoError(t, err)
	expected, err := core.ReconstituteShipments(ctx, coreWS, core.ReconstituteOptions{PlanYAML: plan, DryRun: true})
	require.NoError(t, err, "scenario plan must validate in core")
	require.NoError(t, coreWS.Close())
	require.Len(t, expected.CreatedFeatures, 5)
	require.Len(t, expected.CreatedShipments, 5)

	t.Run("registry_and_command_wiring", func(t *testing.T) {
		op, found := loadRegistryOperations(t)["reconstitute_shipment"]
		require.True(t, found, "registry must declare reconstitute_shipment")
		assert.Equal(t, "backlogit_reconstitute_shipment", op.MCPTool)
		assert.Equal(t, "backlogit shipment reconstitute --plan {{plan_path}}", op.CLICommand)
		assert.Equal(t, map[string]string{
			"plan_path": "plan_path",
			"plan_yaml": "plan_yaml",
			"dry_run":   "dry_run",
			"force":     "force",
			"reason":    "reason",
		}, op.Params)

		liveTools, _, cliPaths := liveSurfaces(t)
		assert.True(t, liveTools["backlogit_reconstitute_shipment"], "MCP tool must be registered")
		assert.True(t, cliPaths["backlogit shipment reconstitute"], "CLI command must be registered")

		cmd, _, findErr := NewRootCommand().Find([]string{"shipment", "reconstitute"})
		require.NoError(t, findErr)
		require.Equal(t, "reconstitute", cmd.Name())
		for _, flag := range []string{"plan", "dry-run", "force", "reason", "json"} {
			assert.NotNil(t, cmd.Flags().Lookup(flag), "reconstitute must expose --%s", flag)
		}
	})

	t.Run("dry_run_is_write_free_and_matches_mcp", func(t *testing.T) {
		before := reconstituteStorageSnapshot(t, sc.root)

		out, runErr := runReconstituteCLI(t, sc.root, "shipment", "reconstitute", "--plan", planPath, "--dry-run", "--json")
		require.NoError(t, runErr, "CLI output: %s", out)
		var cliPreview core.ReconstituteShipmentResult
		require.NoError(t, json.Unmarshal([]byte(out), &cliPreview), "CLI output: %s", out)
		assert.Equal(t, "dry_run", cliPreview.Status)
		assert.True(t, cliPreview.DryRun)
		assert.Equal(t, expected.PlanDigest, cliPreview.PlanDigest)

		byPath := invokeReconstituteMCP(t, sc.root, map[string]any{"plan_path": planPath, "dry_run": true})
		byYAML := invokeReconstituteMCP(t, sc.root, map[string]any{"plan_yaml": plan, "dry_run": true})
		assert.Equal(t, cliPreview.PlanDigest, byPath.PlanDigest, "plan_path must match the CLI preview")
		assert.Equal(t, cliPreview.PlanDigest, byYAML.PlanDigest, "plan_yaml must match the CLI preview")
		assert.Equal(t, cliPreview.AfterManifests, byPath.AfterManifests)
		assert.Equal(t, cliPreview.AfterManifests, byYAML.AfterManifests)

		assert.Equal(t, before, reconstituteStorageSnapshot(t, sc.root), "dry runs must perform zero writes")
	})

	t.Run("live_apply_splits_201S_and_doctor_is_clean", func(t *testing.T) {
		out, runErr := runReconstituteCLI(t, sc.root, "shipment", "reconstitute", "--plan", planPath, "--json")
		require.NoError(t, runErr, "CLI output: %s", out)
		var applied core.ReconstituteShipmentResult
		require.NoError(t, json.Unmarshal([]byte(out), &applied), "CLI output: %s", out)
		assert.False(t, applied.DryRun)
		assert.NotEqual(t, "dry_run", applied.Status)
		assert.Len(t, applied.CreatedFeatures, 5)
		assert.Len(t, applied.CreatedShipments, 5)
		assert.Equal(t, []string{sc.feature, sc.tasks[9], sc.tasks[10]}, applied.AfterManifests[sc.source],
			"201-S keeps its identity with its covering feature and the two unmoved tasks")

		ws, wsErr := core.NewWorkspace(ctx, sc.root)
		require.NoError(t, wsErr)
		defer func() { _ = ws.Close() }()
		reparented := 0
		for _, taskID := range sc.tasks[:9] {
			newID, adopted := applied.AdoptedIDMap[taskID]
			require.True(t, adopted, "task %s must be adopted under a new feature", taskID)
			task, getErr := bldb.GetItem(ctx, ws.DB, newID)
			require.NoError(t, getErr)
			if task.ParentID != sc.feature {
				reparented++
			}
		}
		assert.Equal(t, 9, reparented, "nine tasks must be re-parented to the new features")

		chain := append(append([]string{}, applied.TargetShipments...), sc.downstream)
		assert.Contains(t, chain, sc.source)

		doctorOut, doctorErr := runReconstituteCLI(t, sc.root, "doctor", "--format", "json")
		require.NoError(t, doctorErr, "doctor output: %s", doctorOut)
		var report struct {
			Findings []json.RawMessage `json:"findings"`
		}
		require.NoError(t, json.Unmarshal([]byte(doctorOut), &report), "doctor output: %s", doctorOut)
		assert.Empty(t, report.Findings, "doctor must report 0 findings after reconstitution")
	})
}
