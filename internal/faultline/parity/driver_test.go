package parity_test

// 156.001-T (U1): parallel-safe three-surface scenario driver harness.
//
// TestU1ScenarioDriverSeedRuns seeds one workspace, then runs a single
// read-only scenario (`list --type task`) through all three surfaces (CLI, MCP,
// internal). It asserts that every surface ran successfully on its OWN cloned
// root and that the driver mutated no process globals (os.Args). The test is
// run with -race to prove the parallel surface execution is data-race clean.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/faultline/parity"
)

// seedParityWorkspace initializes a minimal backlogit workspace at root and
// seeds one feature + one task so the read-only list scenario has content.
func seedParityWorkspace(root string) error {
	backlogitDir := filepath.Join(root, ".backlogit")
	if err := os.MkdirAll(backlogitDir, 0o755); err != nil {
		return err
	}
	if err := config.WriteDefaults(backlogitDir); err != nil {
		return err
	}

	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	if err != nil {
		return err
	}
	defer func() { _ = ws.Close() }()

	feature, err := core.CreateArtifact(ctx, ws, "Seed feature", "feature")
	if err != nil {
		return err
	}
	if _, err := core.CreateArtifact(ctx, ws, "Seed task", "task", core.WithParent(feature.ID)); err != nil {
		return err
	}
	return nil
}

func TestU1ScenarioDriverSeedRuns(t *testing.T) {
	argsBefore := append([]string(nil), os.Args...)

	d := &parity.Driver{
		CLI:      parity.CLIRunner{},
		MCP:      parity.MCPRunner{},
		Internal: parity.InternalRunner{},
	}

	sc := parity.ScenarioTable{
		Name: "list-tasks-readonly",
		Seed: seedParityWorkspace,
		Args: []string{"list", "--type", "task", "--format", "json"},
	}

	results := d.RunScenario(context.Background(), t, sc)

	surfaces := [3]string{"CLI", "MCP", "Internal"}
	for i, res := range results {
		require.NoErrorf(t, res.Err, "surface %s returned an error", surfaces[i])
		require.Equalf(t, 0, res.ExitCode, "surface %s exit code (body=%s)", surfaces[i], res.Body)
		require.NotEmptyf(t, res.Body, "surface %s produced an empty body", surfaces[i])
		require.NotEmptyf(t, res.PostStatePath, "surface %s missing post-state path", surfaces[i])
	}

	// Each surface must run against its own cloned root.
	require.NotEqual(t, results[0].PostStatePath, results[1].PostStatePath,
		"CLI and MCP surfaces must use distinct cloned roots")
	require.NotEqual(t, results[1].PostStatePath, results[2].PostStatePath,
		"MCP and Internal surfaces must use distinct cloned roots")
	require.NotEqual(t, results[0].PostStatePath, results[2].PostStatePath,
		"CLI and Internal surfaces must use distinct cloned roots")

	// The driver must not mutate any process-global state.
	require.Equal(t, argsBefore, os.Args, "os.Args must not be mutated by the driver")
}
