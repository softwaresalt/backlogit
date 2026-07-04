package cli

// U2 (078.002-T): CLI/MCP registry drift-detection test.
//
// This guards `.autoharness/backlog-registry.yaml` (the operation map that is the
// single source of truth for the MCP->CLI fallback pairing) against drifting from
// the live surfaces. Enumeration is driven from the authoritative MCP tool set
// (mcp.Server.ListTools) and the real CLI command paths (core.DescribeCLICommands),
// NOT a hand-count, so a future unmapped tool or over-claimed command fails here.
//
// Drift classes caught (see docs/reviews/2026-07-03-cli-mcp-parity-matrix.md):
//   - Class-B (missing row):   a registered MCP tool with no registry row, or a
//                              row lacking both cli_command and mcp_only.
//   - Class-C (over-claim):    a registry cli_command that resolves to a
//                              non-existent cobra command.
//   - Orphan mcp_tool:         a registry mcp_tool absent from ListTools().
//   - Discovery drift:         the export-command-map name lists (DescribeTools +
//                              DescribeCLICommands) diverging from the live
//                              surfaces the registry pairs against.

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	mcpinternal "github.com/softwaresalt/backlogit/internal/mcp"
)

// registryOperation is a single entry in the .autoharness operation map. Only the
// fields relevant to drift detection are decoded.
type registryOperation struct {
	MCPTool    string            `yaml:"mcp_tool"`
	CLICommand string            `yaml:"cli_command"`
	MCPOnly    bool              `yaml:"mcp_only"`
	Params     map[string]string `yaml:"params"`
}

// registryFile is the top-level shape of .autoharness/backlog-registry.yaml.
type registryFile struct {
	Operations map[string]registryOperation `yaml:"operations"`
}

// cliOnlyIntentional is the Class-E allow-list: CLI commands that intentionally
// have no MCP tool counterpart. Source of truth for this classification is the
// U1 parity matrix (docs/reviews/2026-07-03-cli-mcp-parity-matrix.md). The test
// asserts each entry genuinely exists as a CLI command and is not paired to an
// MCP tool in the registry, so a future accidental MCP pairing here is caught.
var cliOnlyIntentional = []string{
	"backlogit init",
	"backlogit mcp",
	"backlogit manifest",
	"backlogit migrate",
	"backlogit status",
	"backlogit docs classify",
	"backlogit queue bulk-status",
	"backlogit queue move",
	"backlogit telemetry branch",
	"backlogit telemetry list",
	"backlogit telemetry report",
	"backlogit telemetry schema",
	"backlogit telemetry top",
	"backlogit telemetry trend",
}

// resolveCLIPath extracts the cobra command path from a registry cli_command
// template by taking the leading tokens up to the first flag (--x) or templated
// parameter ({{x}}). e.g. "backlogit shipment add {{shipment_id}} {{item_id}}"
// -> "backlogit shipment add"; "backlogit move {{id}} --status {{status}}"
// -> "backlogit move".
func resolveCLIPath(cliCommand string) string {
	var tokens []string
	for _, f := range strings.Fields(cliCommand) {
		if strings.HasPrefix(f, "--") || strings.HasPrefix(f, "{{") {
			break
		}
		tokens = append(tokens, f)
	}
	return strings.Join(tokens, " ")
}

// findRegistryPath walks up from the test's working directory to the repo root
// (identified by a go.mod marker) and returns the .autoharness registry path.
// Fails loudly if the root marker or the registry file cannot be found.
func findRegistryPath(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	require.NoError(t, err)
	for {
		if _, statErr := os.Stat(filepath.Join(dir, "go.mod")); statErr == nil {
			regPath := filepath.Join(dir, ".autoharness", "backlog-registry.yaml")
			_, regErr := os.Stat(regPath)
			require.NoErrorf(t, regErr, "registry not found at repo root %s", dir)
			return regPath
		}
		parent := filepath.Dir(dir)
		require.NotEqualf(t, parent, dir, "walked to filesystem root without finding go.mod")
		dir = parent
	}
}

func loadRegistryOperations(t *testing.T) map[string]registryOperation {
	t.Helper()
	data, err := os.ReadFile(findRegistryPath(t))
	require.NoError(t, err, "read backlog-registry.yaml")
	var rf registryFile
	require.NoError(t, yaml.Unmarshal(data, &rf), "unmarshal backlog-registry.yaml")
	require.NotEmpty(t, rf.Operations, "registry operations must not be empty")
	return rf.Operations
}

func liveSurfaces(t *testing.T) (liveTools map[string]bool, describeToolNames map[string]bool, cliPaths map[string]bool) {
	t.Helper()
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ws.Close() })

	srv := mcpinternal.NewServer(ws)

	liveTools = map[string]bool{}
	for _, name := range srv.ListTools() {
		liveTools[name] = true
	}
	describeToolNames = map[string]bool{}
	for _, ti := range srv.DescribeTools() {
		describeToolNames[ti.Name] = true
	}
	cliPaths = map[string]bool{}
	for _, ci := range core.DescribeCLICommands(NewRootCommand()) {
		cliPaths[ci.Command] = true
	}
	return liveTools, describeToolNames, cliPaths
}

// TestRegistryParity_EveryMCPToolMappedOrDeferred is assertion (i): every
// registered MCP tool has either a resolvable cli_command OR an explicit
// mcp_only: true marker. Catches Class-B "missing row" recurrence.
func TestRegistryParity_EveryMCPToolMappedOrDeferred(t *testing.T) {
	ops := loadRegistryOperations(t)
	liveTools, _, cliPaths := liveSurfaces(t)

	byMCPTool := map[string][]registryOperation{}
	for _, op := range ops {
		if op.MCPTool != "" {
			byMCPTool[op.MCPTool] = append(byMCPTool[op.MCPTool], op)
		}
	}

	for tool := range liveTools {
		matched := byMCPTool[tool]
		require.NotEmptyf(t, matched, "registered MCP tool %q has no registry operation row (Class-B missing row)", tool)

		satisfied := false
		for _, op := range matched {
			if op.MCPOnly {
				satisfied = true
				break
			}
			if op.CLICommand != "" && cliPaths[resolveCLIPath(op.CLICommand)] {
				satisfied = true
				break
			}
		}
		assert.Truef(t, satisfied,
			"registered MCP tool %q must have a resolvable cli_command or mcp_only: true", tool)
	}
}

// TestRegistryParity_EveryCLICommandResolves is assertion (ii): every registry
// cli_command resolves to an existing cobra command. Catches Class-C over-claim.
func TestRegistryParity_EveryCLICommandResolves(t *testing.T) {
	ops := loadRegistryOperations(t)
	_, _, cliPaths := liveSurfaces(t)

	for name, op := range ops {
		if op.CLICommand == "" {
			continue
		}
		assert.Falsef(t, op.MCPOnly,
			"operation %q has both cli_command and mcp_only: true (mutually exclusive)", name)
		path := resolveCLIPath(op.CLICommand)
		assert.Truef(t, cliPaths[path],
			"operation %q cli_command %q resolves to non-existent command path %q (Class-C over-claim)",
			name, op.CLICommand, path)
	}
}

// TestRegistryParity_NoOrphanMCPTool is assertion (iii): every registry mcp_tool
// references a name present in the live MCP tool set.
func TestRegistryParity_NoOrphanMCPTool(t *testing.T) {
	ops := loadRegistryOperations(t)
	liveTools, _, _ := liveSurfaces(t)

	for name, op := range ops {
		if op.MCPTool == "" {
			continue
		}
		assert.Truef(t, liveTools[op.MCPTool],
			"operation %q references orphan mcp_tool %q absent from ListTools()", name, op.MCPTool)
	}
}

// TestRegistryParity_DiscoverabilityConsistency is assertion (iv): the
// export-command-map discovery surfaces (DescribeTools + DescribeCLICommands, the
// exact sources metadata export-command-map renders from) stay 1:1 with the live
// tool registrations and a superset of the registry's resolvable leaves, so the
// exported discovery artifact cannot silently drift from the live surfaces.
func TestRegistryParity_DiscoverabilityConsistency(t *testing.T) {
	ops := loadRegistryOperations(t)
	liveTools, describeToolNames, cliPaths := liveSurfaces(t)

	// (iv-a) The MCP discovery list (DescribeTools) rendered into export-command-map
	// is 1:1 with the authoritative ListTools registration set.
	assert.Equal(t, sortedKeys(liveTools), sortedKeys(describeToolNames),
		"DescribeTools (export MCP list) must be 1:1 with ListTools (live registrations)")

	// (iv-b) Every registry mcp_tool is discoverable in the exported MCP list.
	for name, op := range ops {
		if op.MCPTool == "" {
			continue
		}
		assert.Truef(t, describeToolNames[op.MCPTool],
			"operation %q mcp_tool %q missing from export-command-map MCP list", name, op.MCPTool)
	}

	// (iv-c) Every registry resolvable cli_command leaf is discoverable in the
	// exported CLI list.
	for name, op := range ops {
		if op.CLICommand == "" {
			continue
		}
		path := resolveCLIPath(op.CLICommand)
		assert.Truef(t, cliPaths[path],
			"operation %q cli_command leaf %q missing from export-command-map CLI list", name, path)
	}

	// Class-E allow-list has teeth: each declared CLI-only command must genuinely
	// exist and must NOT be paired to an MCP tool in the registry.
	pairedCLIPaths := map[string]bool{}
	for _, op := range ops {
		if op.CLICommand != "" {
			pairedCLIPaths[resolveCLIPath(op.CLICommand)] = true
		}
	}
	for _, cmd := range cliOnlyIntentional {
		assert.Truef(t, cliPaths[cmd], "declared CLI-only command %q does not exist", cmd)
		assert.Falsef(t, pairedCLIPaths[cmd],
			"declared CLI-only command %q is unexpectedly paired to an MCP tool in the registry", cmd)
	}
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
