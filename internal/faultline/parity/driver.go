// Package parity provides a parallel-safe, three-surface scenario driver for the
// cross-surface golden parity harness (156-F / U1).
//
// The driver runs one governed scenario through THREE surfaces — the CLI, the
// MCP tool layer, and the internal core API — and returns each surface's result
// so a comparator can assert cross-surface parity. Every MUTATING scenario gives
// each surface its OWN root cloned from a single seed fixture so that sequential
// mutation and concurrent Markdown/SQLite/event contention cannot masquerade as
// a parity divergence.
//
// The driver never mutates process-global state: it uses a fresh cobra command
// with SetArgs for the CLI surface (never os.Args), and it never touches
// os.Stdout, os.Chdir, os.Setenv, or slog.SetDefault.
package parity

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sync"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/softwaresalt/backlogit/internal/cli"
	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/mcp"
)

// SurfaceResult holds one surface's response to a scenario run.
type SurfaceResult struct {
	// ExitCode is the surface's process-equivalent exit code (0 == success).
	ExitCode int
	// Body is the raw JSON response body captured from the surface.
	Body []byte
	// Err is a non-nil error when the surface failed to run the scenario.
	Err error
	// PostStatePath is the surface's cloned root, used for post-state comparison.
	PostStatePath string
}

// ScenarioRunner runs a scenario against one surface, using its own cloned root.
type ScenarioRunner interface {
	// Run executes args against the surface rooted at root and returns the
	// surface result. A non-nil error mirrors SurfaceResult.Err for callers that
	// prefer error-first handling.
	Run(ctx context.Context, root string, args []string) (SurfaceResult, error)
}

// ScenarioTable holds one named scenario for table-driven parity testing.
type ScenarioTable struct {
	// Name identifies the scenario in test output.
	Name string
	// Seed sets up the seed workspace state at root before it is cloned per
	// surface. Seed may be nil for scenarios that need only the default
	// workspace.
	Seed func(root string) error
	// Args are the CLI-style command arguments for the scenario.
	Args []string
}

// Driver runs a scenario against all three surfaces, cloning the seed fixture
// for each surface so mutations on one surface never interfere with another.
type Driver struct {
	// CLI runs the scenario through the cobra CLI surface.
	CLI ScenarioRunner
	// MCP runs the scenario through the MCP tool surface.
	MCP ScenarioRunner
	// Internal runs the scenario through the internal core API surface.
	Internal ScenarioRunner
}

// RunScenario runs sc against all three surfaces, each with its own cloned root
// derived from the seed fixture, and returns the results indexed as
// {CLI, MCP, Internal}.
//
// The seed fixture is created once and cloned per surface so a mutating surface
// never observes another surface's writes. The three surfaces run concurrently;
// the driver launches exactly one goroutine per surface and joins them via a
// WaitGroup before returning. Each surface runner closes its own workspace
// before returning, so all SQLite/event state is drained before the cloned
// roots are removed by the test's temp-dir cleanup.
func (d *Driver) RunScenario(ctx context.Context, t testing.TB, sc ScenarioTable) [3]SurfaceResult {
	t.Helper()
	if ctx == nil {
		ctx = context.Background()
	}

	seedRoot := t.TempDir()
	if sc.Seed != nil {
		if err := sc.Seed(seedRoot); err != nil {
			t.Fatalf("parity: seed scenario %q: %v", sc.Name, err)
		}
	}

	runners := [3]ScenarioRunner{d.CLI, d.MCP, d.Internal}
	var results [3]SurfaceResult
	var wg sync.WaitGroup

	for i := range runners {
		surfaceRoot := t.TempDir()
		if err := copyTree(seedRoot, surfaceRoot); err != nil {
			t.Fatalf("parity: clone seed for surface %d: %v", i, err)
		}

		wg.Add(1)
		// Each goroutine writes only results[idx] (a distinct array element) and
		// never touches shared mutable state, so the fan-out is data-race clean.
		go func(idx int, runner ScenarioRunner, root string) {
			defer wg.Done()
			res, err := runner.Run(ctx, root, sc.Args)
			if err != nil && res.Err == nil {
				res.Err = err
			}
			if res.PostStatePath == "" {
				res.PostStatePath = root
			}
			results[idx] = res
		}(i, runners[i], surfaceRoot)
	}

	wg.Wait()
	return results
}

// CLIRunner runs a scenario through the cobra CLI surface using a fresh root
// command and SetArgs. It never mutates os.Args, os.Stdout, or any other
// process global.
type CLIRunner struct{}

// Run executes args through a fresh cobra command rooted at root.
func (CLIRunner) Run(_ context.Context, root string, args []string) (SurfaceResult, error) {
	fullArgs := make([]string, 0, len(args)+3)
	fullArgs = append(fullArgs, "--cwd", root, "--no-update-check")
	fullArgs = append(fullArgs, args...)

	cmd := cli.NewRootCommand()
	var out, errOut bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&errOut)
	cmd.SetArgs(fullArgs)

	runErr := cmd.Execute()
	body := out.Bytes()
	if len(body) == 0 {
		body = errOut.Bytes()
	}
	return SurfaceResult{
		ExitCode:      cli.ExitCodeFor(runErr),
		Body:          body,
		Err:           runErr,
		PostStatePath: root,
	}, runErr
}

// MCPRunner runs a scenario through the MCP tool surface via an in-process
// server bound to the surface root.
type MCPRunner struct{}

// Run translates args into an MCP tool invocation, executes it, and closes the
// per-surface workspace before returning.
func (MCPRunner) Run(ctx context.Context, root string, args []string) (SurfaceResult, error) {
	toolName, toolArgs, err := translateToMCP(args)
	if err != nil {
		return SurfaceResult{PostStatePath: root, Err: err}, err
	}

	s := mcp.NewServerForRoot(root)
	req := mcplib.CallToolRequest{}
	req.Params.Name = toolName
	req.Params.Arguments = toolArgs

	result, invokeErr := s.InvokeTool(ctx, toolName, req)
	// The Event/Telemetry/Hook writers are synchronous, so closing the workspace
	// here drains SQLite before the cloned root is removed.
	if s.Workspace != nil {
		_ = s.Workspace.Close()
	}
	if invokeErr != nil {
		return SurfaceResult{PostStatePath: root, Err: invokeErr}, invokeErr
	}

	exit := 0
	if result != nil && result.IsError {
		exit = 1
	}
	return SurfaceResult{
		ExitCode:      exit,
		Body:          mcpResultBody(result),
		PostStatePath: root,
	}, nil
}

// InternalRunner runs a scenario directly against the internal core API.
type InternalRunner struct{}

// Run executes args against a per-surface core.Workspace and closes it before
// returning.
func (InternalRunner) Run(ctx context.Context, root string, args []string) (SurfaceResult, error) {
	if len(args) == 0 || args[0] != "list" {
		err := fmt.Errorf("parity internal surface: unsupported scenario command %q", firstArg(args))
		return SurfaceResult{PostStatePath: root, Err: err}, err
	}

	ws, err := core.NewWorkspace(ctx, root)
	if err != nil {
		return SurfaceResult{PostStatePath: root, Err: err}, err
	}
	defer func() { _ = ws.Close() }()

	filters := listFiltersToQuery(parseListFilters(args))
	artifacts, err := db.QueryItems(ctx, ws.DB, filters)
	if err != nil {
		return SurfaceResult{PostStatePath: root, Err: err}, err
	}

	body, err := json.MarshalIndent(core.ListWithSizeComposition(ctx, ws, artifacts), "", "  ")
	if err != nil {
		return SurfaceResult{PostStatePath: root, Err: err}, err
	}
	return SurfaceResult{ExitCode: 0, Body: body, PostStatePath: root}, nil
}

// translateToMCP maps CLI-style scenario args to an MCP tool name and arguments.
func translateToMCP(args []string) (string, map[string]any, error) {
	if len(args) == 0 {
		return "", nil, fmt.Errorf("parity MCP surface: empty scenario args")
	}
	switch args[0] {
	case "list":
		filters := parseListFilters(args)
		toolArgs := make(map[string]any, len(filters))
		for k, v := range filters {
			toolArgs[k] = v
		}
		return "backlogit_list_items", toolArgs, nil
	default:
		return "", nil, fmt.Errorf("parity MCP surface: unsupported scenario command %q", args[0])
	}
}

// listFilterKeys maps CLI list filter flag names (kebab-case) to their
// snake_case parameter names shared by the MCP and internal surfaces.
var listFilterKeys = map[string]string{
	"type":        "type",
	"status":      "status",
	"priority":    "priority",
	"complexity":  "complexity",
	"assigned-to": "assigned_to",
	"owner":       "owner",
	"sprint":      "sprint",
}

// parseListFilters extracts the known list filter flags from CLI-style args,
// returning a map keyed by snake_case parameter name. Unknown flags (e.g.
// --format, --json) are skipped so CLI-only output flags do not leak into the
// MCP or internal surfaces.
func parseListFilters(args []string) map[string]string {
	out := make(map[string]string)
	// Skip args[0] (the command verb) when present.
	i := 1
	for i < len(args) {
		tok := args[i]
		if len(tok) < 2 || tok[:2] != "--" {
			i++
			continue
		}
		name := tok[2:]
		var value string
		hasInline := false
		if eq := indexByte(name, '='); eq >= 0 {
			value = name[eq+1:]
			name = name[:eq]
			hasInline = true
		}
		snake, known := listFilterKeys[name]
		if known {
			if !hasInline {
				if i+1 < len(args) && !isFlag(args[i+1]) {
					value = args[i+1]
					i++
				}
			}
			out[snake] = value
			i++
			continue
		}
		// Unknown flag: consume a trailing non-flag value (e.g. --format json)
		// so it is not misread as a positional command.
		if !hasInline && i+1 < len(args) && !isFlag(args[i+1]) {
			i++
		}
		i++
	}
	return out
}

// listFiltersToQuery maps snake_case filter params to a db.QueryFilters value.
func listFiltersToQuery(filters map[string]string) db.QueryFilters {
	return db.QueryFilters{
		Type:       filters["type"],
		Status:     filters["status"],
		Priority:   filters["priority"],
		Complexity: filters["complexity"],
		AssignedTo: filters["assigned_to"],
		Owner:      filters["owner"],
		Sprint:     filters["sprint"],
	}
}

// mcpResultBody extracts the text body from an MCP tool result.
func mcpResultBody(result *mcplib.CallToolResult) []byte {
	if result == nil || len(result.Content) == 0 {
		return nil
	}
	if tc, ok := result.Content[0].(mcplib.TextContent); ok {
		return []byte(tc.Text)
	}
	return nil
}

// isFlag reports whether tok looks like a flag token rather than a value.
func isFlag(tok string) bool {
	return len(tok) > 0 && tok[0] == '-'
}

// firstArg returns args[0] or "" for an empty slice, for diagnostics.
func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}

// indexByte returns the index of the first occurrence of b in s, or -1.
func indexByte(s string, b byte) int {
	for i := 0; i < len(s); i++ {
		if s[i] == b {
			return i
		}
	}
	return -1
}

// copyTree recursively copies the directory tree at src into dst.
func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		return copyFile(path, target)
	})
}

// copyFile copies a single file from src to dst, creating parent dirs as needed.
func copyFile(src, dst string) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	data, err := os.ReadFile(src) //nolint:gosec // src is a test-owned seed fixture path.
	if err != nil {
		return err
	}
	if err := os.WriteFile(dst, data, 0o644); err != nil { //nolint:gosec // fixture clone, not sensitive.
		return err
	}
	return nil
}
