package mcp_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	mcpinternal "github.com/softwaresalt/backlogit/internal/mcp"
)

// TestMCPReconcileArchivedLifecycle_ToolRegistered verifies that the
// backlogit_reconcile_archived_lifecycle tool is registered on the MCP server.
//
// This test passes once the stub in tools_reconcile.go registers the tool via
// registerReconcileTools() and RegisterTools() calls it.
func TestMCPReconcileArchivedLifecycle_ToolRegistered(t *testing.T) {
	tmpDir := t.TempDir()
	s := mcpinternal.NewServerForRoot(tmpDir)
	assert.Contains(t, s.ListTools(), "backlogit_reconcile_archived_lifecycle",
		"backlogit_reconcile_archived_lifecycle must be registered on the MCP server")
}

// TestMCPCorrectStashProvenance_ToolRegistered verifies that the
// backlogit_correct_stash_provenance tool is registered on the MCP server.
//
// This test passes once the stub in tools_reconcile.go registers the tool via
// registerReconcileTools() and RegisterTools() calls it.
func TestMCPCorrectStashProvenance_ToolRegistered(t *testing.T) {
	tmpDir := t.TempDir()
	s := mcpinternal.NewServerForRoot(tmpDir)
	assert.Contains(t, s.ListTools(), "backlogit_correct_stash_provenance",
		"backlogit_correct_stash_provenance must be registered on the MCP server")
}
