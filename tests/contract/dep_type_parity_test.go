package contract_test

// dep_type_parity_test.go — U6 parity contract for F4.
//
// Asserts that the db-layer dep query (backing CLI dep list) and the MCP
// backlogit_get_dependencies tool return the same edge set with the same
// dep_type values, confirming one canonical dependency representation
// across both surfaces.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/softwaresalt/backlogit/internal/cli"
	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	mcpinternal "github.com/softwaresalt/backlogit/internal/mcp"
)

// TestDepTypeParity_CLIAndMCPReturnCanonicalShape verifies that the db-layer
// dep query (backing CLI dep list) and the MCP backlogit_get_dependencies tool
// return the same edge with the same dep_type for a non-default (relates_to)
// edge. This is the F4 U6 parity gate.
func TestDepTypeParity_CLIAndMCPReturnCanonicalShape(t *testing.T) {
	// Arrange: workspace with a relates_to dep between two tasks.
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { ws.Close() })

	feat, err := core.CreateArtifact(ctx, ws, "Parity feature", "feature")
	require.NoError(t, err)
	t1, err := core.CreateArtifact(ctx, ws, "Task one", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	t2, err := core.CreateArtifact(ctx, ws, "Task two", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	require.NoError(t, core.AddDependency(ctx, ws, t2.ID, t1.ID, "relates_to"))

	// CLI surface: invoke the real Cobra command behind `backlogit dep list`.
	rootCmd := cli.NewRootCommand()
	var cliOutput bytes.Buffer
	rootCmd.SetOut(&cliOutput)
	rootCmd.SetErr(&cliOutput)
	rootCmd.SetArgs([]string{"--cwd", root, "dep", "list", t2.ID})
	require.NoError(t, rootCmd.Execute())
	cliLine := fmt.Sprintf("%s → %s (%s)", t2.ID, t1.ID, "relates_to")
	assert.Equal(t, cliLine, strings.TrimSpace(cliOutput.String()),
		"CLI dep list must emit the canonical dependency shape")

	// MCP surface: backlogit_get_dependencies tool.
	s := mcpinternal.NewServer(ws)
	result, toolErr := callToolForTest(t, s, "backlogit_get_dependencies", map[string]any{
		"id":      t2.ID,
		"reverse": false,
	})
	require.NoError(t, toolErr)
	require.NotNil(t, result)
	require.False(t, result.IsError, "tool call must not error: %v", result.Content)

	require.Len(t, result.Content, 1)
	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok, "MCP result must be TextContent")
	text := tc.Text

	var mcpEdges []struct {
		ItemID    string `json:"item_id"`
		DependsOn string `json:"depends_on"`
		DepType   string `json:"dep_type"`
	}
	require.NoError(t, json.Unmarshal([]byte(text), &mcpEdges))
	require.Len(t, mcpEdges, 1, "MCP: must return exactly one dep edge")
	assert.Equal(t, t2.ID, mcpEdges[0].ItemID)
	assert.Equal(t, t1.ID, mcpEdges[0].DependsOn)
	assert.Equal(t, "relates_to", mcpEdges[0].DepType, "MCP dep_type must be relates_to")

	// Parity gate: both surfaces must return the same dep_type.
	assert.Equal(t, "relates_to", mcpEdges[0].DepType,
		"CLI and MCP must return identical dep_type for the same edge")
}
