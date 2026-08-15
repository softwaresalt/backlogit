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
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/events"
	mcpinternal "github.com/softwaresalt/backlogit/internal/mcp"
	"github.com/softwaresalt/backlogit/internal/models"
)

// registryOperation is a single entry in the .autoharness operation map. Only the
// fields relevant to drift detection are decoded.
type registryOperation struct {
	MCPTool      string            `yaml:"mcp_tool"`
	CLICommand   string            `yaml:"cli_command"`
	MCPOnly      bool              `yaml:"mcp_only"`
	Governed     bool              `yaml:"governed"`
	GovernedName string            `yaml:"governed_name"`
	Params       map[string]string `yaml:"params"`
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

// governedOperations returns all operations with governed: true from the registry.
// The set is derived from the live registry so newly added governed operations
// enter the test automatically without a manual allowlist update.
// extractGovernedID extracts the artifact ID from CLI add command output.
func extractGovernedID(t *testing.T, output string) string {
	t.Helper()
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		parts := strings.SplitN(line, ": ", 2)
		if len(parts) == 2 {
			return strings.TrimSpace(parts[1])
		}
	}
	t.Fatalf("could not extract ID from output: %q", output)
	return ""
}

func governedOperations(t *testing.T) map[string]registryOperation {
	t.Helper()
	ops := loadRegistryOperations(t)
	governed := map[string]registryOperation{}
	for name, op := range ops {
		if op.Governed {
			governed[name] = op
		}
	}
	return governed
}

// setupGovernedWorkspace creates a workspace in a temp dir and returns the root
// path and a workspace instance. It constructs the .backlogit directory with
// default config so CLI commands resolve correctly.
func setupGovernedWorkspace(t *testing.T) (string, *core.Workspace) {
	t.Helper()
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ws.Close() })
	return root, ws
}

// runGovernedCLI executes a root command invocation against root and returns stdout.
func runGovernedCLI(t *testing.T, root string, args ...string) {
	t.Helper()
	cmd := NewRootCommand()
	out := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(errBuf)
	allArgs := append([]string{"--cwd", root}, args...)
	cmd.SetArgs(allArgs)
	require.NoError(t, cmd.Execute(), "cli %v failed: %s", args, errBuf.String())
}

// addGovernedArtifact creates an artifact via CLI and returns its ID.
func addGovernedArtifact(t *testing.T, root, artifactType, title, parent string) string {
	t.Helper()
	cmd := NewRootCommand()
	out := new(bytes.Buffer)
	cmd.SetOut(out)
	cmd.SetErr(new(bytes.Buffer))
	args := []string{"--cwd", root, "add", "--type", artifactType, "--title", title}
	if parent != "" {
		args = append(args, "--parent", parent)
	}
	cmd.SetArgs(args)
	require.NoError(t, cmd.Execute())
	return extractGovernedID(t, out.String())
}

// observeGovernedState reads all three commit-association representations for
// itemID / sha from a fresh workspace connection and the JSONL log on disk.
func observeGovernedState(t *testing.T, rootPath, itemID, sha string) (frontmatterSHA string, hasLinksRow, hasJSONLEvent bool) {
	t.Helper()
	ctx := context.Background()

	// Fresh workspace for authoritative DB state.
	freshWS, err := core.NewWorkspace(ctx, rootPath)
	require.NoError(t, err)
	defer freshWS.Close()

	// 1. Frontmatter scalar — read from markdown, not DB cache.
	artPath, err := core.FindArtifactPath(ctx, freshWS, itemID)
	require.NoError(t, err)
	data, err := os.ReadFile(artPath)
	require.NoError(t, err)
	fm, body, err := models.ParseFrontmatter(string(data))
	require.NoError(t, err)
	art, err := models.ArtifactFromFrontmatter(fm, body)
	require.NoError(t, err)
	frontmatterSHA = art.Commit

	// 2. commit_links row.
	var linkCount int
	_ = freshWS.DB.QueryRowContext(ctx,
		"SELECT COUNT(*) FROM commit_links WHERE item_id = ? AND commit_sha = ?",
		itemID, sha).Scan(&linkCount)
	hasLinksRow = linkCount > 0

	// 3. JSONL commit_tracked event.
	logsDir := core.WorkspaceLogsRoot(rootPath)
	evs, _ := events.ReadAllEvents(ctx, logsDir, itemID)
	for _, ev := range evs {
		if ev.EventType == "commit_tracked" {
			if d, ok := ev.Delta["commit_sha"].(string); ok && d == sha {
				hasJSONLEvent = true
				break
			}
		}
	}
	return frontmatterSHA, hasLinksRow, hasJSONLEvent
}

type governedCommentState struct {
	Actor        string
	Comment      string
	CommitSHA    string
	JSONLEvent   bool
	IndexedEvent bool
}

// observeGovernedCommentState reads the JSONL event and indexed projection for a
// comment so the CLI and MCP paths can be compared without relying on timestamps.
func observeGovernedCommentState(t *testing.T, rootPath, itemID, actor, comment, commitSHA string) governedCommentState {
	t.Helper()
	ctx := context.Background()
	freshWS, err := core.NewWorkspace(ctx, rootPath)
	require.NoError(t, err)
	defer freshWS.Close()

	state := governedCommentState{Actor: actor, Comment: comment, CommitSHA: commitSHA}
	for _, event := range mustReadEvents(t, rootPath, itemID) {
		if event.EventType == "comment" && event.Actor == actor && event.CommitSHA == commitSHA {
			if value, ok := event.Delta["comment"].(string); ok && value == comment {
				state.JSONLEvent = true
				break
			}
		}
	}

	expectedDelta, err := json.Marshal(map[string]string{"comment": comment})
	require.NoError(t, err)
	var count int
	err = freshWS.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM item_log_entries
		 WHERE item_id = ? AND actor = ? AND event_type = 'comment' AND delta_json = ?`,
		itemID, actor, string(expectedDelta)).Scan(&count)
	require.NoError(t, err)
	state.IndexedEvent = count > 0
	return state
}

func mustReadEvents(t *testing.T, rootPath, itemID string) []events.Event {
	t.Helper()
	evs, err := events.ReadAllEvents(context.Background(), core.WorkspaceLogsRoot(rootPath), itemID)
	require.NoError(t, err)
	return evs
}

func observeGovernedDependency(t *testing.T, rootPath, itemID, dependsOn, depType string) bool {
	t.Helper()
	ctx := context.Background()
	freshWS, err := core.NewWorkspace(ctx, rootPath)
	require.NoError(t, err)
	defer freshWS.Close()

	var count int
	err = freshWS.DB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM item_deps
		 WHERE item_id = ? AND depends_on = ? AND dep_type = ?`,
		itemID, dependsOn, depType).Scan(&count)
	require.NoError(t, err)
	return count > 0
}

// TestRegistryParity_GovernedOperationBehavioralParity is the F6/U5 behavioral
// parity assertion. For each governed: true operation in the registry, this test
// executes both surfaces against equivalent fixtures and asserts identical
// observable state. Two HARD gates:
//  1. The governed set must not be empty (no vacuous pass).
//  2. An operation with governed_name "commit_association" must exist.
func TestRegistryParity_GovernedOperationBehavioralParity(t *testing.T) {
	governed := governedOperations(t)

	// Gate 1: governed set must be non-empty.
	require.NotEmpty(t, governed,
		"governed operation set must not be empty: add governed: true to at least one operation")

	// Gate 2: every governed operation selected for this wave must be present.
	requiredNames := []string{
		"commit_association",
		"comment_append",
		"dependency_add",
		"dependency_remove",
	}
	for _, requiredName := range requiredNames {
		found := false
		for _, op := range governed {
			if op.GovernedName == requiredName {
				found = true
				break
			}
		}
		require.Truef(t, found,
			"a governed operation with governed_name: %s is required — the test cannot pass vacuously",
			requiredName)
	}

	const testSHA = "aabbccddee0001000200030004000500060007aabb"

	for opName, op := range governed {
		opName, op := opName, op
		t.Run(opName, func(t *testing.T) {
			if op.MCPTool == "" || op.CLICommand == "" {
				t.Skipf("operation %q has no CLI surface; skipping behavioral parity", opName)
			}

			switch op.MCPTool {
			case "backlogit_track_commit":
				root, ws := setupGovernedWorkspace(t)

				featID := addGovernedArtifact(t, root, "feature", "Gov parity feature", "")
				taskCLI := addGovernedArtifact(t, root, "task", "Gov parity CLI task", featID)
				taskMCP := addGovernedArtifact(t, root, "task", "Gov parity MCP task", featID)
				ctx := context.Background()

				// CLI surface: update --commit now routes through AssociateCommit.
				runGovernedCLI(t, root, "update", taskCLI, "--commit", testSHA)

				// MCP surface (track_commit): call AssociateCommit directly.
				logsDir := core.WorkspaceLogsRoot(root)
				ew := core.NewWorkspaceEventWriter(ws, logsDir)
				require.NoError(t, core.AssociateCommit(ctx, ws, ew, taskMCP, testSHA, "feat: governed parity", "test@example.com"))

				cliSHA, cliLinks, cliEvent := observeGovernedState(t, root, taskCLI, testSHA)
				mcpSHA, mcpLinks, mcpEvent := observeGovernedState(t, root, taskMCP, testSHA)

				// Structural parity: all three representations must be present on both surfaces.
				assert.True(t, cliSHA == testSHA, "op %q: CLI frontmatter scalar must equal input SHA", opName)
				assert.True(t, cliLinks, "op %q: CLI surface must write commit_links row", opName)
				assert.True(t, cliEvent, "op %q: CLI surface must write JSONL commit_tracked event", opName)
				assert.Equal(t, cliSHA, mcpSHA, "op %q: frontmatter SHA must match across surfaces", opName)
				assert.Equal(t, cliLinks, mcpLinks, "op %q: commit_links row presence must match", opName)
				assert.Equal(t, cliEvent, mcpEvent, "op %q: JSONL event presence must match", opName)

			case "backlogit_abandon_checkpoint":
				root, ws := setupGovernedWorkspace(t)
				ctx := context.Background()
				checkpointDir := filepath.Join(root, ".backlogit", "checkpoints")
				require.NoError(t, os.MkdirAll(checkpointDir, 0o755))

				validCP := func() *events.CheckpointV1 {
					now := time.Now().UTC()
					return &events.CheckpointV1{
						SchemaVersion: 1, Agent: "ship", SessionID: "gov-parity", Phase: "build",
						Status: "active", CreatedAt: now, UpdatedAt: now,
					}
				}
				cliFile, mcpFile := "checkpoint-gov-cli.json", "checkpoint-gov-mcp.json"
				for _, name := range []string{cliFile, mcpFile} {
					data, err := json.Marshal(validCP())
					require.NoError(t, err)
					require.NoError(t, os.WriteFile(filepath.Join(checkpointDir, name), data, 0o644))
				}

				const reason, operator = "governed parity", "gov-parity@example.com"
				runGovernedCLI(t, root, "checkpoint", "abandon", cliFile, "--reason", reason, "--operator", operator)

				logsDir := core.WorkspaceLogsRoot(root)
				ew := core.NewWorkspaceEventWriter(ws, logsDir)
				require.NoError(t, core.AbandonCheckpoint(ctx, ws, ew, mcpFile, reason, operator))

				cliData, err := os.ReadFile(filepath.Join(checkpointDir, cliFile))
				require.NoError(t, err)
				mcpData, err := os.ReadFile(filepath.Join(checkpointDir, mcpFile))
				require.NoError(t, err)
				cliCP, err := events.ParseCheckpoint(cliData)
				require.NoError(t, err)
				mcpCP, err := events.ParseCheckpoint(mcpData)
				require.NoError(t, err)

				assert.Equal(t, events.DispositionAbandoned, cliCP.Disposition, "op %q: CLI surface must set disposition=abandoned", opName)
				assert.Equal(t, reason, cliCP.DispositionReason, "op %q: CLI surface must record the reason", opName)
				assert.Equal(t, operator, cliCP.DispositionOperator, "op %q: CLI surface must record the operator", opName)
				assert.Equal(t, cliCP.Disposition, mcpCP.Disposition, "op %q: disposition must match across surfaces", opName)
				assert.Equal(t, cliCP.DispositionReason, mcpCP.DispositionReason, "op %q: disposition reason must match across surfaces", opName)
				assert.Equal(t, cliCP.DispositionOperator, mcpCP.DispositionOperator, "op %q: disposition operator must match across surfaces", opName)
				assert.Equal(t, cliCP.Status, mcpCP.Status, "op %q: status must match across surfaces", opName)

			case "backlogit_quarantine_checkpoint":
				root, ws := setupGovernedWorkspace(t)
				ctx := context.Background()
				checkpointDir := filepath.Join(root, ".backlogit", "checkpoints")
				require.NoError(t, os.MkdirAll(checkpointDir, 0o755))

				cliFile, mcpFile := "checkpoint-gov-cli-bad.json", "checkpoint-gov-mcp-bad.json"
				for _, name := range []string{cliFile, mcpFile} {
					require.NoError(t, os.WriteFile(filepath.Join(checkpointDir, name), []byte("not-json{"), 0o644))
				}

				const reason, operator = "governed parity malformed", "gov-parity@example.com"
				runGovernedCLI(t, root, "checkpoint", "quarantine", cliFile, "--reason", reason, "--operator", operator)

				logsDir := core.WorkspaceLogsRoot(root)
				ew := core.NewWorkspaceEventWriter(ws, logsDir)
				require.NoError(t, core.QuarantineCheckpoint(ctx, ws, ew, mcpFile, reason, operator))

				archiveDir := filepath.Join(root, ".backlogit", "archive", "checkpoints")
				cliSidecar, err := os.ReadFile(filepath.Join(archiveDir, cliFile+".disposition.json"))
				require.NoError(t, err, "op %q: CLI surface must write a disposition sidecar", opName)
				mcpSidecar, err := os.ReadFile(filepath.Join(archiveDir, mcpFile+".disposition.json"))
				require.NoError(t, err, "op %q: MCP surface must write a disposition sidecar", opName)

				var cliRec, mcpRec events.CheckpointDispositionRecord
				require.NoError(t, json.Unmarshal(cliSidecar, &cliRec))
				require.NoError(t, json.Unmarshal(mcpSidecar, &mcpRec))

				assert.FileExists(t, filepath.Join(archiveDir, cliFile), "op %q: CLI surface must quarantine the file verbatim", opName)
				assert.FileExists(t, filepath.Join(archiveDir, mcpFile), "op %q: MCP surface must quarantine the file verbatim", opName)
				assert.Equal(t, events.DispositionQuarantined, cliRec.Disposition, "op %q: CLI sidecar disposition must be quarantined", opName)
				assert.Equal(t, cliRec.Disposition, mcpRec.Disposition, "op %q: sidecar disposition must match across surfaces", opName)
				assert.Equal(t, cliRec.Reason, mcpRec.Reason, "op %q: sidecar reason must match across surfaces", opName)
				assert.Equal(t, cliRec.Operator, mcpRec.Operator, "op %q: sidecar operator must match across surfaces", opName)

			case "backlogit_append_comment":
				root, ws := setupGovernedWorkspace(t)
				ctx := context.Background()
				featID := addGovernedArtifact(t, root, "feature", "Comment parity feature", "")
				taskCLI := addGovernedArtifact(t, root, "task", "Comment parity CLI task", featID)
				taskMCP := addGovernedArtifact(t, root, "task", "Comment parity MCP task", featID)
				const actor, comment, commitSHA = "gov-parity", "comment parity", "aabbccddeeff00112233445566778899"

				runGovernedCLI(t, root, "comment", "add", taskCLI, "--actor", actor, "--comment", comment, "--commit-sha", commitSHA)
				ew := core.NewWorkspaceEventWriter(ws, core.WorkspaceLogsRoot(root))
				require.NoError(t, core.AppendComment(ctx, ws, ew, taskMCP, actor, comment, commitSHA))

				cliState := observeGovernedCommentState(t, root, taskCLI, actor, comment, commitSHA)
				mcpState := observeGovernedCommentState(t, root, taskMCP, actor, comment, commitSHA)
				assert.Equal(t, cliState, mcpState, "op %q: comment state must match across surfaces", opName)
				assert.True(t, cliState.JSONLEvent, "op %q: JSONL comment event must be present", opName)
				assert.True(t, cliState.IndexedEvent, "op %q: indexed comment event must be present", opName)

			case "backlogit_add_dependency":
				root, ws := setupGovernedWorkspace(t)
				ctx := context.Background()
				featID := addGovernedArtifact(t, root, "feature", "Dependency parity feature", "")
				taskCLI := addGovernedArtifact(t, root, "task", "Dependency parity CLI task", featID)
				taskMCP := addGovernedArtifact(t, root, "task", "Dependency parity MCP task", featID)

				runGovernedCLI(t, root, "dep", "add", taskCLI, featID, "--type", "blocks")
				require.NoError(t, core.AddDependency(ctx, ws, taskMCP, featID, "blocks"))

				cliPresent := observeGovernedDependency(t, root, taskCLI, featID, "blocks")
				mcpPresent := observeGovernedDependency(t, root, taskMCP, featID, "blocks")
				assert.True(t, cliPresent, "op %q: CLI dependency must be persisted", opName)
				assert.Equal(t, cliPresent, mcpPresent, "op %q: dependency state must match across surfaces", opName)

			case "backlogit_remove_dependency":
				root, ws := setupGovernedWorkspace(t)
				ctx := context.Background()
				featID := addGovernedArtifact(t, root, "feature", "Dependency removal parity feature", "")
				taskCLI := addGovernedArtifact(t, root, "task", "Dependency removal CLI task", featID)
				taskMCP := addGovernedArtifact(t, root, "task", "Dependency removal MCP task", featID)
				require.NoError(t, core.AddDependency(ctx, ws, taskCLI, featID, "blocks"))
				require.NoError(t, core.AddDependency(ctx, ws, taskMCP, featID, "blocks"))

				runGovernedCLI(t, root, "dep", "remove", taskCLI, featID)
				require.NoError(t, core.RemoveDependency(ctx, ws, taskMCP, featID))

				cliPresent := observeGovernedDependency(t, root, taskCLI, featID, "blocks")
				mcpPresent := observeGovernedDependency(t, root, taskMCP, featID, "blocks")
				assert.False(t, cliPresent, "op %q: CLI dependency must be removed", opName)
				assert.Equal(t, cliPresent, mcpPresent, "op %q: dependency state must match across surfaces", opName)

			default:
				t.Fatalf("governed op %q mcp_tool=%q: no behavioral fixture defined; add one to registry_parity_test.go", opName, op.MCPTool)
			}
		})
	}
}

// TestRegistryParity_ForceGatesAbsentFromMCPParams is the F6/U5 regression
// assertion: --force-gates must not appear in the MCP surface parameter map.
// The registry's update_task cli_only_flags documents this deliberate asymmetry.
func TestRegistryParity_ForceGatesAbsentFromMCPParams(t *testing.T) {
	ops := loadRegistryOperations(t)

	// Load the full raw registry to check cli_only_flags.
	regPath := findRegistryPath(t)
	data, err := os.ReadFile(regPath)
	require.NoError(t, err)
	var rawFile map[string]any
	require.NoError(t, yaml.Unmarshal(data, &rawFile))
	rawOps, _ := rawFile["operations"].(map[string]any)

	// For every MCP tool, assert force_gates is not a documented param.
	for opName, op := range ops {
		if op.MCPOnly || op.MCPTool == "" {
			continue
		}
		// Check the raw operation's params map for "force_gates" or "force-gates".
		rawOp, _ := rawOps[opName].(map[string]any)
		params, _ := rawOp["params"].(map[string]any)
		_, hasForceGates := params["force_gates"]
		_, hasForceGatesDash := params["force-gates"]
		assert.Falsef(t, hasForceGates || hasForceGatesDash,
			"operation %q must not include force_gates in its params (human-terminal-only, P-010 blast-radius rule)",
			opName)
	}

	// Assert that update_task explicitly documents force-gates as human_terminal_only.
	rawUpdateTask, _ := rawOps["update_task"].(map[string]any)
	cliOnlyFlags, _ := rawUpdateTask["cli_only_flags"].(map[string]any)
	forceGatesEntry, _ := cliOnlyFlags["force-gates"].(map[string]any)
	humanTerminalOnly, _ := forceGatesEntry["human_terminal_only"].(bool)
	assert.True(t, humanTerminalOnly,
		"registry update_task.cli_only_flags.force-gates.human_terminal_only must be true (F6/U4 deliberate asymmetry documentation)")
}
