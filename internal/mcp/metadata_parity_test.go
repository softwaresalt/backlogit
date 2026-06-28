package mcp

// 062.001-T: Restore CLI metadata parity in MCP (stash 5A41B7C3).
//
// The MCP metadata catalog must carry the same CLI command information that the
// CLI metadata path produces. Before the fix, loadMetadataCatalog passed a nil
// cliRoot to core.BuildMetadataCatalog, so the catalog returned over MCP omitted
// the CLI command data entirely. These tests lock in parity so the two surfaces
// cannot drift again.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
)

func setupCatalogServer(t *testing.T) *Server {
	t.Helper()
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { ws.Close() })

	return NewServer(ws)
}

// When a CLICommandProvider is wired, the MCP metadata catalog must include the
// CLI command data it supplies.
func TestMetadataCatalog_IncludesCLICommandsFromProvider(t *testing.T) {
	s := setupCatalogServer(t)
	ctx := context.Background()

	want := []core.CommandInfo{
		{Command: "backlogit add", Use: "add", Short: "Add an artifact"},
		{Command: "backlogit list", Use: "list", Short: "List artifacts"},
	}
	s.CLICommandProvider = func() []core.CommandInfo { return want }

	catalog, err := s.MetadataCatalog(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, catalog.CLI,
		"MCP metadata catalog must include CLI command data when a provider is wired")
	assert.Equal(t, want, catalog.CLI,
		"MCP metadata catalog CLI data must match the provider output")
}

// handleGetMetadataCatalog must surface the CLI command data in its JSON output.
func TestHandleGetMetadataCatalog_EmitsCLICommands(t *testing.T) {
	s := setupCatalogServer(t)
	ctx := context.Background()

	s.CLICommandProvider = func() []core.CommandInfo {
		return []core.CommandInfo{{Command: "backlogit shipment", Use: "shipment"}}
	}

	req := mcplib.CallToolRequest{}
	req.Params.Name = "backlogit_get_metadata_catalog"
	req.Params.Arguments = map[string]any{}
	result, err := s.handleGetMetadataCatalog(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.IsError, "catalog request should succeed")

	require.NotEmpty(t, result.Content)
	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok)
	assert.Contains(t, tc.Text, "backlogit shipment",
		"serialized catalog must contain CLI command data")
}
