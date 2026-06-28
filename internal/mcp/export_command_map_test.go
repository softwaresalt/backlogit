package mcp

// 062.002-T: Align export-command-map workspace root (stash 6DD3062F).
//
// The MCP export_command_map handler must resolve the target path against the
// workspace root, exactly as the CLI does, instead of against the .backlogit
// storage directory. Before the fix it called core.WriteCommandMap with
// s.backlogitDir(), so the same workspace-relative target landed under a
// different root than the CLI and valid workspace-relative paths could fail the
// containment check.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
)

func setupExportServer(t *testing.T) *Server {
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

// export_command_map must write the target under the workspace root, matching
// the CLI behavior (core.WriteCommandMap(workspaceRoot, ...)).
func TestExportCommandMap_WritesUnderWorkspaceRoot(t *testing.T) {
	s := setupExportServer(t)
	ctx := context.Background()

	target := filepath.Join(".github", "instructions", "backlogit-command-map.md")
	req := mcplib.CallToolRequest{}
	req.Params.Name = "backlogit_export_command_map"
	req.Params.Arguments = map[string]any{"path": target}

	result, err := s.handleExportCommandMap(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.False(t, result.IsError, "valid workspace-relative target must not fail containment")

	// The file must exist under the workspace root, NOT under .backlogit.
	wantPath := filepath.Join(s.RootPath, target)
	_, statErr := os.Stat(wantPath)
	require.NoError(t, statErr, "command map must be written under the workspace root at %s", wantPath)

	insideBacklogit := filepath.Join(s.backlogitDir(), target)
	_, backErr := os.Stat(insideBacklogit)
	assert.True(t, os.IsNotExist(backErr),
		"command map must NOT be written under .backlogit at %s", insideBacklogit)
}

// A target escaping the workspace root must still be rejected by containment.
func TestExportCommandMap_RejectsEscapeOutsideWorkspace(t *testing.T) {
	s := setupExportServer(t)
	ctx := context.Background()

	escape := strings.Join([]string{"..", "..", "outside.md"}, string(filepath.Separator))
	req := mcplib.CallToolRequest{}
	req.Params.Name = "backlogit_export_command_map"
	req.Params.Arguments = map[string]any{"path": escape}

	result, err := s.handleExportCommandMap(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.True(t, result.IsError, "a path escaping the workspace root must be rejected")
}
