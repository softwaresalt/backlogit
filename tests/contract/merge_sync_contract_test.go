package contract_test

// 037.005-T (contract tier): backlogit_merge_sync schema and pre-init safety
//
// These contract tests verify:
//   - backlogit_merge_sync is visible in the tool list
//   - The tool returns a workspace-not-initialized error when called before init
//   - dry_run=true returns a diff result without an error flag
//   - The response JSON shape contains expected top-level keys

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/mark3labs/mcp-go/client"
	mcplib "github.com/mark3labs/mcp-go/mcp"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
	mcpinternal "github.com/softwaresalt/backlogit/internal/mcp"
)

// TestMergeSyncContract_ToolIsListed verifies backlogit_merge_sync appears in
// the tools/list response, confirming it is unconditionally registered.
func TestMergeSyncContract_ToolIsListed(t *testing.T) {
	s := setupRealMCPServer(t)

	ctx := context.Background()
	c, err := client.NewInProcessClient(s.MCPServer())
	require.NoError(t, err)
	require.NoError(t, c.Start(ctx))
	defer c.Close() //nolint:errcheck

	initReq := mcplib.InitializeRequest{}
	initReq.Params.ClientInfo = mcplib.Implementation{Name: "test", Version: "0.0.1"}
	initReq.Params.ProtocolVersion = mcplib.LATEST_PROTOCOL_VERSION
	_, err = c.Initialize(ctx, initReq)
	require.NoError(t, err)

	toolsResult, err := c.ListTools(ctx, mcplib.ListToolsRequest{})
	require.NoError(t, err)

	found := false
	for _, tool := range toolsResult.Tools {
		if tool.Name == "backlogit_merge_sync" {
			found = true
			break
		}
	}
	assert.True(t, found, "backlogit_merge_sync must appear in tools/list")
}

// TestMergeSyncContract_PreInitReturnsError verifies that calling
// backlogit_merge_sync before the workspace exists returns an error result
// rather than a Go-level error or panic.
func TestMergeSyncContract_PreInitReturnsError(t *testing.T) {
	// Use a root without a .backlogit directory.
	root := t.TempDir()
	s := mcpinternal.NewServerForRoot(root)

	result, err := callToolForTest(t, s, "backlogit_merge_sync", map[string]any{"dry_run": false})
	require.NoError(t, err, "pre-init call must not return a Go error")
	require.NotNil(t, result)
	assert.True(t, result.IsError, "pre-init call must return an error result")
}

// TestMergeSyncContract_DryRunReturnsExpectedShape verifies that a dry_run=true
// call against an initialised workspace returns a JSON object with the expected
// top-level keys and no error flag.
func TestMergeSyncContract_DryRunReturnsExpectedShape(t *testing.T) {
	s := setupRealMCPServer(t)

	result, err := callToolForTest(t, s, "backlogit_merge_sync", map[string]any{"dry_run": true})
	require.NoError(t, err)
	require.NotNil(t, result)
	assert.False(t, result.IsError, "dry_run call must not return an error flag")
	require.NotEmpty(t, result.Content)

	tc, ok := result.Content[0].(mcplib.TextContent)
	require.True(t, ok, "response content must be TextContent")

	var data map[string]any
	require.NoError(t, json.Unmarshal([]byte(tc.Text), &data))

	// Verify the documented response keys are present.
	for _, key := range []string{"added", "changed", "deleted", "relocated", "dry_run", "fallback_used"} {
		_, exists := data[key]
		assert.True(t, exists, "expected key %q in merge_sync response", key)
	}

	dryRun, _ := data["dry_run"].(bool)
	assert.True(t, dryRun, "dry_run must be true in response when called with dry_run=true")
}

// TestMergeSyncContract_NonDryRunOnEmptyWorkspaceSuceeeds verifies that a
// live (non-dry-run) call on a freshly initialised empty workspace succeeds
// and returns zero added/changed/deleted counts.
func TestMergeSyncContract_NonDryRunOnEmptyWorkspaceSucceeds(t *testing.T) {
	s := setupRealMCPServer(t)

	data := callToolAndParseJSON(t, s, "backlogit_merge_sync", map[string]any{"dry_run": false})

	added, _ := data["added"].([]any)
	changed, _ := data["changed"].([]any)
	deleted, _ := data["deleted"].([]any)
	assert.Empty(t, added, "empty workspace must have no added entries")
	assert.Empty(t, changed, "empty workspace must have no changed entries")
	assert.Empty(t, deleted, "empty workspace must have no deleted entries")
}

// setupRealMCPServerWithRoot creates a server and returns the workspace root
// so tests can write files to trigger diffs.
func setupRealMCPServerWithRoot(t *testing.T) (s *mcpinternal.Server, root string) {
	t.Helper()
	root = t.TempDir()
	backlogitDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogitDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogitDir))

	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { ws.Close() })

	return mcpinternal.NewServer(ws), root
}

// TestMergeSyncContract_NewArtifactAppearsInAddedList verifies that a Markdown
// artifact written to the workspace after the initial sync appears in the
// "added" list on the next merge_sync call.
func TestMergeSyncContract_NewArtifactAppearsInAddedList(t *testing.T) {
	s, root := setupRealMCPServerWithRoot(t)

	// Prime the manifest with an initial dry-run so the server learns the baseline.
	_, err := callToolForTest(t, s, "backlogit_merge_sync", map[string]any{"dry_run": false})
	require.NoError(t, err)

	// Write a new artifact file.
	queueDir := filepath.Join(root, ".backlogit", "queue")
	require.NoError(t, os.MkdirAll(queueDir, 0o755))
	artifact := `---
id: 099-F
title: Contract test feature
artifact_type: feature
status: queued
---
Body.
`
	require.NoError(t, os.WriteFile(filepath.Join(queueDir, "099-F.md"), []byte(artifact), 0o644))

	// Second sync — 099-F must appear in added.
	data := callToolAndParseJSON(t, s, "backlogit_merge_sync", map[string]any{"dry_run": false})

	added, _ := data["added"].([]any)
	found := false
	for _, entry := range added {
		if m, ok := entry.(map[string]any); ok {
			if m["id"] == "099-F" {
				found = true
				break
			}
		}
	}
	assert.True(t, found, "newly written artifact 099-F must appear in the added list")
}
