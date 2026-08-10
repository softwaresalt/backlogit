package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	mcplib "github.com/mark3labs/mcp-go/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
)

func TestHandleSaveMemory_UsesResolvedStorageRoot(t *testing.T) {
	root := t.TempDir()
	storageRoot := filepath.Join(root, ".backlog")
	require.NoError(t, os.MkdirAll(storageRoot, 0o755))
	require.NoError(t, config.WriteDefaults(storageRoot))

	ws, err := core.NewWorkspace(context.Background(), root)
	require.NoError(t, err)
	t.Cleanup(func() { _ = ws.Close() })

	s := NewServer(ws)
	req := mcplib.CallToolRequest{}
	req.Params.Arguments = map[string]any{
		"key":     "session",
		"summary": "saved through MCP",
	}

	result, err := s.handleSaveMemory(context.Background(), req)
	require.NoError(t, err)
	require.False(t, result.IsError)
	assert.FileExists(t, filepath.Join(storageRoot, "memories.json"))
}
