package cli

// 062.001-T: CLI/MCP metadata catalog parity (stash 5A41B7C3).
//
// This guards against the two metadata surfaces drifting again: the CLI command
// data exposed through the MCP metadata catalog must be identical to the CLI
// metadata path's command data. The wiring under test is the same one used by
// the live `backlogit mcp` server (wireMCPMetadataProvider).

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	mcpinternal "github.com/softwaresalt/backlogit/internal/mcp"
)

func TestMetadataCatalog_CLIAndMCPParity(t *testing.T) {
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	ctx := context.Background()

	// CLI metadata path.
	cliCatalog, err := loadMetadataCatalog(ctx, root)
	require.NoError(t, err)
	require.NotEmpty(t, cliCatalog.CLI, "CLI metadata path must expose CLI command data")

	// MCP metadata path, wired exactly as the live server wires it.
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	defer ws.Close()
	srv := mcpinternal.NewServer(ws)
	wireMCPMetadataProvider(srv, root)

	mcpCatalog, err := srv.MetadataCatalog(ctx)
	require.NoError(t, err)
	require.NotEmpty(t, mcpCatalog.CLI, "MCP metadata catalog must expose CLI command data")

	assert.Equal(t, cliCatalog.CLI, mcpCatalog.CLI,
		"CLI and MCP metadata catalogs must expose identical CLI command data")
}
