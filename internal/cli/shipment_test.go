package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	mcpinternal "github.com/softwaresalt/backlogit/internal/mcp"
)

// T003 / ST016: Verify NewShipmentCmd returns a valid command with subcommands.
func TestNewShipmentCmd_HasSubcommands(t *testing.T) {
	// Arrange & Act
	cmd := NewShipmentCmd()

	// Assert
	require.NotNil(t, cmd)
	assert.Equal(t, "shipment", cmd.Use)

	names := make([]string, 0)
	for _, sub := range cmd.Commands() {
		names = append(names, sub.Name())
	}
	assert.Contains(t, names, "create")
	assert.Contains(t, names, "get")
	assert.Contains(t, names, "list")
	assert.Contains(t, names, "claim")
	assert.Contains(t, names, "ship")
	assert.Contains(t, names, "return-blocked")
}

// T003 / ST016: Verify shipment create has required flags.
func TestShipmentCreateCmd_HasFlags(t *testing.T) {
	// Arrange & Act
	cmd := newShipmentCreateCmd()

	// Assert
	require.NotNil(t, cmd)
	assert.NotNil(t, cmd.Flags().Lookup("title"), "create must have --title flag")
}

// T003 / ST016: Verify shipment return-blocked has required flags.
func TestShipmentReturnBlockedCmd_HasFlags(t *testing.T) {
	// Arrange & Act
	cmd := newShipmentReturnBlockedCmd()

	// Assert
	require.NotNil(t, cmd)
	assert.NotNil(t, cmd.Flags().Lookup("shipment"), "return-blocked must have --shipment flag")
	assert.NotNil(t, cmd.Flags().Lookup("item"), "return-blocked must have --item flag")
	assert.NotNil(t, cmd.Flags().Lookup("reason"), "return-blocked must have --reason flag")
}

// T003 / ST016: Verify shipment ship has commit-tracking flags.
func TestShipmentShipCmd_HasFlags(t *testing.T) {
	// Arrange & Act
	cmd := newShipmentShipCmd()

	// Assert
	require.NotNil(t, cmd)
	assert.NotNil(t, cmd.Flags().Lookup("sha"), "ship must have --sha flag")
	assert.NotNil(t, cmd.Flags().Lookup("message"), "ship must have --message flag")
	assert.NotNil(t, cmd.Flags().Lookup("author"), "ship must have --author flag")
}

// T003 / ST016: Verify shipment commands have Short and Example populated.
func TestShipmentSubcommands_HaveDocumentation(t *testing.T) {
	// Arrange
	cmd := NewShipmentCmd()

	// Assert
	for _, sub := range cmd.Commands() {
		t.Run(sub.Name(), func(t *testing.T) {
			assert.NotEmpty(t, sub.Short, "%s must have .Short description", sub.Name())
			assert.NotEmpty(t, sub.Example, "%s must have .Example", sub.Name())
		})
	}
}

// T003 / ST016: Verify shipment list has --status filter flag.
func TestShipmentListCmd_HasStatusFilter(t *testing.T) {
	// Arrange & Act
	cmd := newShipmentListCmd()

	// Assert
	require.NotNil(t, cmd)
	assert.NotNil(t, cmd.Flags().Lookup("status"), "list must have --status flag")
}

// T003 / ST016: Verify help output includes shipment subcommands.
func TestShipmentCmd_HelpOutput(t *testing.T) {
	// Arrange
	cmd := NewShipmentCmd()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetArgs([]string{"--help"})

	// Act
	_ = cmd.Execute()

	// Assert
	output := buf.String()
	assert.Contains(t, output, "create")
	assert.Contains(t, output, "ship")
	assert.Contains(t, output, "return-blocked")
}

// --- 134.001-T: Failing CLI tests for shipment --priority flag (U1 red harness) ---

// TestShipmentCreateCmd_HasPriorityFlag verifies the shipment create subcommand
// exposes a --priority flag for create-time priority setting.
//
// Red harness: fails until U2 wires the --priority flag on newShipmentCreateCmd.
func TestShipmentCreateCmd_HasPriorityFlag(t *testing.T) {
	cmd := newShipmentCreateCmd()
	require.NotNil(t, cmd)
	assert.NotNil(t, cmd.Flags().Lookup("priority"),
		"shipment create must expose a --priority flag")
}

// TestShipmentCreate_PriorityFlagSetsPriority verifies that running
// `backlogit shipment create --title "..." --priority high` produces a
// shipment JSON response with priority="high".
//
// Red harness: fails until U2 wires the --priority flag and passes it to
// core.CreateShipment.
func TestShipmentCreate_PriorityFlagSetsPriority(t *testing.T) {
	root := setupShipmentCLIWorkspace(t)

	buf := new(bytes.Buffer)
	cmd := NewRootCommand()
	cmd.SetOut(buf)
	errBuf := new(bytes.Buffer)
	cmd.SetErr(errBuf)
	cmd.SetArgs([]string{"--cwd", root, "shipment", "create", "--title", "Prioritized shipment", "--priority", "high"})

	err := cmd.Execute()
	require.NoError(t, err, "shipment create with --priority must not error; stderr: %s", errBuf.String())

	// The output must be valid JSON containing priority="high".
	require.NotEmpty(t, buf.String(), "shipment create must produce output")
	var result map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &result),
		"shipment create output must be valid JSON")
	assert.Equal(t, "high", result["priority"],
		"shipment create --priority high must set priority=high in the response")
}

func TestShipmentListCmd_RejectsInvalidFormat(t *testing.T) {
	root := setupShipmentCLIWorkspace(t)

	cmd := NewRootCommand()
	buf := new(bytes.Buffer)
	cmd.SetOut(buf)
	cmd.SetErr(buf)
	cmd.SetArgs([]string{"--cwd", root, "shipment", "list", "--format", "banana"})

	err := cmd.Execute()

	require.Error(t, err)
	assert.Contains(t, err.Error(), `"banana"`)
}

func setupShipmentCLIWorkspace(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	return root
}

// --- 134.001-T: CLI/MCP create_shipment surface parity lock (U1 red harness) ---

// createShipmentOutputOnlyDenylist names the CLI shipment-create flags that shape
// OUTPUT only and have no MCP request-param equivalent.  A future output-only flag
// MUST be listed here explicitly.  A future create-time param MUST NOT be listed
// here — that would silence drift.
var createShipmentOutputOnlyDenylist = map[string]bool{
	// No output-only flags on shipment create at this time.
}

// TestCreateShipmentCLIMCPParity is the denylist parity lock: the MCP
// backlogit_create_shipment parameter set must equal the CLI `shipment create`
// flag set after subtracting output-only flags. If either surface adds a
// create-time param without the other, this test fails.
//
// Red harness: passes initially (both sides still omit "priority"), but FAILS
// if U2 adds --priority to the CLI without adding the MCP param or vice versa.
// Becomes a stable pass once U2 adds priority to both surfaces.
func TestCreateShipmentCLIMCPParity(t *testing.T) {
	// --- CLI flag set from the live newShipmentCreateCmd ---
	cliSet := make(map[string]bool)
	newShipmentCreateCmd().Flags().VisitAll(func(f *pflag.Flag) {
		if createShipmentOutputOnlyDenylist[f.Name] {
			return
		}
		cliSet[f.Name] = true
	})

	// --- MCP param set from the live ToolDefs ---
	ws := setupShipmentMCPWorkspace(t)
	srv := mcpinternal.NewServer(ws)
	mcpSet := make(map[string]bool)
	for _, def := range srv.ToolDefs() {
		if def.Name != "backlogit_create_shipment" {
			continue
		}
		for paramName := range def.InputSchema.Properties {
			mcpSet[paramName] = true
		}
		break
	}

	// --- assert parity ---
	assert.Equal(t, sortedBoolMapKeysShipment(cliSet), sortedBoolMapKeysShipment(mcpSet),
		"backlogit_create_shipment MCP params must equal the CLI `shipment create` flags "+
			"(after subtracting output-only flags).\n"+
			"CLI flags: %v\nMCP params: %v\n"+
			"Add the missing param to whichever surface is behind, or add it to "+
			"createShipmentOutputOnlyDenylist if it is output-only.",
		sortedBoolMapKeysShipment(cliSet), sortedBoolMapKeysShipment(mcpSet),
	)
}

// setupShipmentMCPWorkspace builds a minimal workspace and returns the core
// Workspace for MCP server construction in shipment parity tests.
func setupShipmentMCPWorkspace(t *testing.T) *core.Workspace {
	t.Helper()
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ws.Close() })
	return ws
}

// sortedBoolMapKeysShipment returns sorted keys of m. Named distinctly to
// avoid colliding with sortedBoolMapKeys in list_filter_parity_test.go.
func sortedBoolMapKeysShipment(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
