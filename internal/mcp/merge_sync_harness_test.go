package mcp

// 037.004-T: Server Manifest Integration
//
// These harnesses verify:
//   - backlogit_merge_sync is registered in the tool list
//   - The handler JSON schema exposes the dry_run boolean parameter
//   - Concurrent calls to handleMergeSync do not race on the manifest field
//   - handleMergeSync returns a workspace-not-initialized error when called
//     before the workspace exists

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

// TestMergeSync_ToolIsRegistered verifies that backlogit_merge_sync is present
// in the tool name list exposed by the Server.
func TestMergeSync_ToolIsRegistered(t *testing.T) {
	root := t.TempDir()
	s := NewServerForRoot(root)

	found := false
	for _, name := range s.toolNames {
		if name == "backlogit_merge_sync" {
			found = true
			break
		}
	}
	assert.True(t, found, "backlogit_merge_sync must be registered in Server.toolNames")
}

// TestMergeSync_ToolSchemaHasDryRunParam verifies the tool definition exposes
// a dry_run boolean parameter so callers can inspect schema without invoking.
func TestMergeSync_ToolSchemaHasDryRunParam(t *testing.T) {
	root := t.TempDir()
	s := NewServerForRoot(root)

	var tool *mcplib.Tool
	for i := range s.toolDefs {
		if s.toolDefs[i].Name == "backlogit_merge_sync" {
			tool = &s.toolDefs[i]
			break
		}
	}
	require.NotNil(t, tool, "backlogit_merge_sync must be in toolDefs")

	// The tool schema is exposed as InputSchema; confirm dry_run is present.
	props, ok := tool.InputSchema.Properties["dry_run"]
	require.True(t, ok, "dry_run parameter must be present in InputSchema.Properties")
	schemaType, _ := props.(map[string]any)["type"].(string)
	assert.Equal(t, "boolean", schemaType, "dry_run must be declared as a boolean")
}

// TestMergeSync_PreInitReturnsWorkspaceNotInitialized verifies that calling
// handleMergeSync before the workspace exists returns a workspace-not-initialized
// error result rather than panicking.
func TestMergeSync_PreInitReturnsWorkspaceNotInitialized(t *testing.T) {
	root := t.TempDir()
	// Intentionally do NOT create .backlogit/ so the workspace is uninitialized.
	s := NewServerForRoot(root)

	ctx := context.Background()
	req := mcplib.CallToolRequest{}
	req.Params.Name = "backlogit_merge_sync"
	req.Params.Arguments = map[string]any{"dry_run": false}

	result, err := s.handleMergeSync(ctx, req)
	require.NoError(t, err, "handler must not return a Go error for missing workspace")
	require.NotNil(t, result)
	assert.True(t, result.IsError, "result must indicate an error for uninitialized workspace")
}

// TestMergeSync_ConcurrentCallsDoNotRace verifies that concurrent invocations
// of handleMergeSync do not data-race on the manifest field. The race detector
// will fail this test if the locking is absent.
// Run with: go test -race ./internal/mcp/
func TestMergeSync_ConcurrentCallsDoNotRace(t *testing.T) {
	root := t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { ws.Close() })

	s := NewServer(ws)

	const goroutines = 8
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for range goroutines {
		go func() {
			defer wg.Done()
			req := mcplib.CallToolRequest{}
			req.Params.Name = "backlogit_merge_sync"
			req.Params.Arguments = map[string]any{"dry_run": true}
			_, _ = s.handleMergeSync(ctx, req)
		}()
	}
	wg.Wait()
}
