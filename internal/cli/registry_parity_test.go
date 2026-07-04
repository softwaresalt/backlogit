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

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

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

// findCommandByPath resolves a space-delimited cobra command path (e.g.
// "backlogit link add") to the concrete *cobra.Command, or nil if any segment
// is missing. The leading token is the root command name and is skipped.
func findCommandByPath(root *cobra.Command, path string) *cobra.Command {
	fields := strings.Fields(path)
	if len(fields) == 0 {
		return nil
	}
	cur := root
	for _, name := range fields[1:] {
		var next *cobra.Command
		for _, sub := range cur.Commands() {
			if sub.Name() == name {
				next = sub
				break
			}
		}
		if next == nil {
			return nil
		}
		cur = next
	}
	return cur
}

// lookupFlag returns the --name flag from the command's local, persistent, or
// inherited flag sets, or nil if the command exposes no such flag.
func lookupFlag(cmd *cobra.Command, name string) *pflag.Flag {
	for _, fs := range []*pflag.FlagSet{cmd.Flags(), cmd.PersistentFlags(), cmd.InheritedFlags()} {
		if f := fs.Lookup(name); f != nil {
			return f
		}
	}
	return nil
}

// TestRegistryParity_FlagAndPositionalParity is the U6 (079.006-T) load-bearing
// assertion (v): for every registry cli_command, each literal --flag must
// resolve to a real flag on the target cobra command, and the number of
// positional {{...}} placeholders must satisfy the command's declared Args
// validator. This closes the gap left by resolveCLIPath, which validated only
// the command PATH — a typo'd flag name or a wrong positional count in a
// fallback row previously passed drift detection yet broke the fallback at
// runtime. Optional flags may be omitted from the template (documented in
// params), mirroring the existing archive_item/commit_sha convention.
func TestRegistryParity_FlagAndPositionalParity(t *testing.T) {
	ops := loadRegistryOperations(t)
	root := NewRootCommand()

	for name, op := range ops {
		if op.CLICommand == "" {
			continue
		}
		path := resolveCLIPath(op.CLICommand)
		cmd := findCommandByPath(root, path)
		require.NotNilf(t, cmd, "operation %q cli_command %q resolves to unknown command path %q",
			name, op.CLICommand, path)

		tokens := strings.Fields(op.CLICommand)
		pathLen := len(strings.Fields(path))
		positionals := 0
		templateFlags := map[string]bool{}
		for i := pathLen; i < len(tokens); i++ {
			tok := tokens[i]
			switch {
			case strings.HasPrefix(tok, "--"):
				flagName := strings.TrimPrefix(tok, "--")
				templateFlags[flagName] = true
				f := lookupFlag(cmd, flagName)
				assert.NotNilf(t, f,
					"operation %q cli_command references --%s but command %q exposes no such flag (flag-parity drift)",
					name, flagName, path)
				// A non-boolean flag consumes the following token as its value —
				// either a {{placeholder}} or a literal (e.g. `--status done`).
				// Boolean flags take no value.
				if f != nil && f.Value.Type() != "bool" &&
					i+1 < len(tokens) && !strings.HasPrefix(tokens[i+1], "--") {
					i++
				}
			case strings.HasPrefix(tok, "{{"):
				positionals++
			default:
				// A literal token passed positionally (an enum value supplied
				// without a flag). Counts toward the positional arity.
				positionals++
			}
		}

		// Every flag the target command marks required must appear in the
		// template — otherwise the fallback invocation fails at runtime with
		// "required flag(s) not set" even though the command path resolves. This
		// closes the required-flag false-negative in the flag-parity gate.
		cmd.Flags().VisitAll(func(f *pflag.Flag) {
			req := f.Annotations[cobra.BashCompOneRequiredFlag]
			if len(req) == 1 && req[0] == "true" {
				assert.Truef(t, templateFlags[f.Name],
					"operation %q cli_command omits required flag --%s of command %q (fallback would fail at runtime)",
					name, f.Name, path)
			}
		})

		// The positional placeholder count must satisfy the command's Args
		// validator (e.g. cobra.ExactArgs(N)); a nil Args means arbitrary args.
		if cmd.Args != nil {
			assert.NoErrorf(t, cmd.Args(cmd, make([]string, positionals)),
				"operation %q supplies %d positional placeholder(s), violating command %q Args validator (positional-parity drift)",
				name, positionals, path)
		}
	}
}
