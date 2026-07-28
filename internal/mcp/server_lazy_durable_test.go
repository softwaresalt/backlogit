package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
)

// setupDurableWorkspaceRoot writes a minimal workspace whose config.yaml opts
// into (or out of) durable_writes, plus a .backlogit dir so dirExists passes.
func setupDurableWorkspaceRoot(t *testing.T, durable bool) string {
	t.Helper()
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(filepath.Join(backlogitDir, "queue"), 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))
	if durable {
		cfgPath := filepath.Join(backlogitDir, "config.yaml")
		data, err := os.ReadFile(cfgPath)
		require.NoError(t, err)
		require.NoError(t, os.WriteFile(cfgPath, append(data, []byte("\ndurable_writes: true\n")...), 0o644))
	}
	return root
}

// TestMCPServer_LazyInit_PropagatesDurableWrites is the FIX-1 regression: the
// production MCP entry constructs the server with ws=nil (NewServerForRoot), so
// the shared s.Events writer starts durable-off. When the workspace is lazily
// loaded via ensureWorkspace (funneled through requireWorkspace on the real tool
// paths), s.Events MUST be rebuilt so durable_writes reaches the append path.
func TestMCPServer_LazyInit_PropagatesDurableWrites(t *testing.T) {
	root := setupDurableWorkspaceRoot(t, true)
	s := NewServerForRoot(root)
	require.NotNil(t, s.Events)
	t.Cleanup(func() {
		if s.Workspace != nil {
			_ = s.Workspace.Close()
		}
	})

	// Pre-init: ws is nil, so the shared writer is durable-off. Documents the
	// state the lazy-init fix must correct.
	assert.False(t, s.Events.Durable(), "before lazy init the shared writer is durable-off")

	// Trigger the lazy workspace init through the production funnel.
	ws, res := s.requireWorkspace(context.Background())
	require.Nil(t, res, "workspace must initialize")
	require.NotNil(t, ws)

	assert.True(t, s.Events.Durable(),
		"after lazy init the shared writer must pick up durable_writes from config")
}

// TestMCPServer_LazyInit_DurableOffStaysOff is the symmetric off-case: a workspace
// without durable_writes leaves the shared writer durable-off after lazy init.
func TestMCPServer_LazyInit_DurableOffStaysOff(t *testing.T) {
	root := setupDurableWorkspaceRoot(t, false)
	s := NewServerForRoot(root)
	require.NotNil(t, s.Events)
	t.Cleanup(func() {
		if s.Workspace != nil {
			_ = s.Workspace.Close()
		}
	})

	ws, res := s.requireWorkspace(context.Background())
	require.Nil(t, res)
	require.NotNil(t, ws)

	assert.False(t, s.Events.Durable(),
		"a workspace without durable_writes must leave the shared writer durable-off")
}
