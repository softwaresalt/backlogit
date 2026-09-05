package contract_test

// governed_create_checkpoint_test.go — U4 governed-parity fixture for
// create_checkpoint (155.004-T).
//
// Follows the compound learning:
// docs/compound/2026-08-15-governed-parity-fixtures-must-dispatch-authoritative-registry.md
//
// The fixture:
//  1. Loads the AUTHORITATIVE .autoharness/backlog-registry.yaml
//  2. Selects the create_checkpoint operation
//  3. Asserts governed:true and governed_name:checkpoint_create
//  4. Invokes the REGISTERED MCP tool (backlogit_create_checkpoint) — not
//     core.CreateCheckpoint directly — so the registry mapping is exercised
//  5. Invokes the REGISTERED CLI command (backlogit checkpoint create) — same
//     shared pipeline, confirming CLI/MCP surface parity

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/softwaresalt/backlogit/internal/cli"
	"github.com/softwaresalt/backlogit/internal/config"
)

// TestU4_CreateCheckpointGoverned asserts that the authoritative
// backlog-registry.yaml declares create_checkpoint with governed:true and
// governed_name:checkpoint_create, and that invoking the registered handlers on
// both surfaces succeeds against a valid state dump (155.004-T / U4).
func TestU4_CreateCheckpointGoverned(t *testing.T) {
	// -------------------------------------------------------------------------
	// Step 1: Read the authoritative registry and assert the governed markers.
	// This assertion is RED before the registry YAML is updated and GREEN after.
	// -------------------------------------------------------------------------
	registryPath := findRegistry(t)
	raw, err := os.ReadFile(registryPath)
	require.NoError(t, err)

	var reg map[string]any
	require.NoError(t, yaml.Unmarshal(raw, &reg))

	ops, ok := reg["operations"].(map[string]any)
	require.True(t, ok, "registry must have an operations section")

	createCheckpoint, ok := ops["create_checkpoint"].(map[string]any)
	require.True(t, ok, "operations.create_checkpoint must exist in the registry")

	governed, _ := createCheckpoint["governed"].(bool)
	assert.True(t, governed, "create_checkpoint must have governed: true")

	governedName, _ := createCheckpoint["governed_name"].(string)
	assert.Equal(t, "checkpoint_create", governedName,
		"create_checkpoint must have governed_name: checkpoint_create")

	mcpTool, _ := createCheckpoint["mcp_tool"].(string)
	assert.NotEmpty(t, mcpTool, "create_checkpoint must have mcp_tool")

	cliCommand, _ := createCheckpoint["cli_command"].(string)
	assert.NotEmpty(t, cliCommand, "create_checkpoint must have cli_command")

	// -------------------------------------------------------------------------
	// Step 2: Invoke the REGISTERED MCP handler via the authoritative tool name.
	// Using setupRealMCPServerWithRoot returns an explicit workspace root so we
	// can assert that the MCP surface independently persisted a checkpoint there,
	// and distinguish it from the CLI workspace below (Copilot review thread 1:
	// separate workspaces prevent second-precision filename collisions).
	// -------------------------------------------------------------------------
	s, mcpRoot := setupRealMCPServerWithRoot(t)

	const validDump = `{"schema_version":1,"agent":"ship","session_id":"u4-governed","phase":"build"}`

	// Success path: valid state_dump.
	mcpData := callToolAndParseJSON(t, s, mcpTool, map[string]any{
		"state_dump": validDump,
	})
	mcpPath, _ := mcpData["path"].(string)
	require.NotEmpty(t, mcpPath, "MCP %s must return a non-empty checkpoint path for a valid dump", mcpTool)
	// The returned path must be inside the MCP workspace, not the CLI workspace.
	assert.Contains(t, filepath.ToSlash(mcpPath), filepath.ToSlash(mcpRoot),
		"MCP checkpoint path must be within the MCP workspace root")

	// Error path: invalid agent must be rejected.
	const badDump = `{"schema_version":1,"agent":"invalid-agent","session_id":"u4","phase":"build"}`
	badResult, err := callToolForTest(t, s, mcpTool, map[string]any{
		"state_dump": badDump,
	})
	require.NoError(t, err)
	require.True(t, badResult.IsError,
		"MCP %s must reject a state_dump with an invalid agent", mcpTool)

	// -------------------------------------------------------------------------
	// Step 3: Invoke the REGISTERED CLI handler in a SEPARATE workspace.
	// Using a distinct cliRoot from mcpRoot guarantees the CLI surface
	// independently persists a checkpoint without any risk of path collision with
	// the MCP workspace above (Copilot review thread 1).
	// -------------------------------------------------------------------------
	cliRoot := t.TempDir()
	backlogitDir := filepath.Join(cliRoot, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	rootCmd := cli.NewRootCommand()
	var cliOut bytes.Buffer
	rootCmd.SetOut(&cliOut)
	rootCmd.SetErr(new(bytes.Buffer))
	rootCmd.SetArgs([]string{"--cwd", cliRoot, "checkpoint", "create", "--state-dump", validDump})
	require.NoError(t, rootCmd.Execute(),
		"CLI checkpoint create must succeed for a valid state dump")

	var cliData map[string]any
	require.NoError(t, json.Unmarshal(cliOut.Bytes(), &cliData))
	cliPath, _ := cliData["path"].(string)
	require.NotEmpty(t, cliPath, "CLI checkpoint create must return a non-empty checkpoint path")
	// The CLI path must be inside the CLI workspace, not the MCP workspace.
	assert.Contains(t, filepath.ToSlash(cliPath), filepath.ToSlash(cliRoot),
		"CLI checkpoint path must be within the CLI workspace root, not the MCP workspace")
	assert.NotContains(t, filepath.ToSlash(cliPath), filepath.ToSlash(mcpRoot),
		"CLI checkpoint path must be in a distinct workspace from the MCP checkpoint")
}

// findRegistry walks up from the test's working directory looking for
// .autoharness/backlog-registry.yaml, returning its path.
// Fails the test if the file cannot be located.
func findRegistry(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		candidate := filepath.Join(dir, ".autoharness", "backlog-registry.yaml")
		if _, statErr := os.Stat(candidate); statErr == nil {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatal("could not find .autoharness/backlog-registry.yaml — walked to filesystem root")
		}
		dir = parent
	}
}
