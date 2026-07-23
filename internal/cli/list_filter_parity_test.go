package cli

// 122.001-T: CLI/MCP list_items filter-param parity-lock test.
//
// This test derives the CLI filter flag set from the live newListCommand cobra
// flags (NOT a hard-coded list), subtracts the documented output-only denylist,
// normalizes kebab-case → snake_case, and asserts that set equals the
// backlogit_list_items parameter names extracted from the live
// (*mcp.Server).ToolDefs() accessor. Using a DENYLIST (not an allowlist)
// ensures a future CLI-only filter flag automatically enters the derived set and
// fails this test, rather than silently re-opening the CLI/MCP asymmetry.

import (
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	mcpinternal "github.com/softwaresalt/backlogit/internal/mcp"
)

// listOutputOnlyDenylist names the flags on newListCommand that shape OUTPUT
// only and have no MCP request-param equivalent. A future output-only flag
// must be added here explicitly. A future FILTER flag must NOT be added here —
// that would silence drift; it must flow through to MCP and make this test pass.
var listOutputOnlyDenylist = map[string]bool{
	"group-by": true,
	"json":     true,
	"format":   true,
}

// TestListCLIMCPFilterParityLock asserts that the backlogit_list_items MCP
// parameter set equals the CLI `list` filter-flag set after subtracting output-
// only flags and normalizing kebab-case to snake_case.
func TestListCLIMCPFilterParityLock(t *testing.T) {
	// --- derive CLI filter set from the live cobra command ---
	cwd := ""
	listCmd := newListCommand(&cwd)

	cliFilters := make(map[string]bool)
	listCmd.Flags().VisitAll(func(f *pflag.Flag) {
		if listOutputOnlyDenylist[f.Name] {
			return
		}
		// Normalize CLI kebab-case flag names to snake_case for MCP comparison.
		snake := strings.ReplaceAll(f.Name, "-", "_")
		cliFilters[snake] = true
	})

	// --- derive MCP param set from the live ToolDefs accessor ---
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ws.Close() })

	srv := mcpinternal.NewServer(ws)
	mcpParams := make(map[string]bool)
	for _, tool := range srv.ToolDefs() {
		if tool.Name != "backlogit_list_items" {
			continue
		}
		for paramName := range tool.InputSchema.Properties {
			mcpParams[paramName] = true
		}
		break
	}

	// --- assert parity ---
	cliSorted := sortedBoolMapKeys(cliFilters)
	mcpSorted := sortedBoolMapKeys(mcpParams)

	assert.Equal(t, cliSorted, mcpSorted,
		"backlogit_list_items MCP params must equal the CLI `list` filter flags (snake_case).\n"+
			"CLI filters (derived): %v\n"+
			"MCP params (live):     %v\n"+
			"If a CLI filter flag is missing from MCP, add it. "+
			"If an MCP param has no CLI equivalent, add the CLI flag or move it to the output-only denylist.",
		cliSorted, mcpSorted,
	)
}

// sortedBoolMapKeys returns the keys of m sorted alphabetically.
func sortedBoolMapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
