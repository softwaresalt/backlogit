package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
)

// TestNewServer_DurableWriterWhenFlagOn asserts the MCP server construction path
// funnels through the durable-aware helper: with durable_writes on, the server's
// shared event writer is durable.
func TestNewServer_DurableWriterWhenFlagOn(t *testing.T) {
	dir := t.TempDir()
	ws := &core.Workspace{RootPath: dir, Config: &config.WorkspaceConfig{DurableWrites: true}}
	s := NewServer(ws)
	require.NotNil(t, s.Events)
	assert.True(t, s.Events.Durable(), "MCP server must construct a durable event writer when the flag is on")
}

// TestNewServer_DurableWriterOffByDefault asserts the flag-off construction is
// unchanged (non-durable writer).
func TestNewServer_DurableWriterOffByDefault(t *testing.T) {
	dir := t.TempDir()
	ws := &core.Workspace{RootPath: dir, Config: &config.WorkspaceConfig{DurableWrites: false}}
	s := NewServer(ws)
	require.NotNil(t, s.Events)
	assert.False(t, s.Events.Durable(), "MCP server must construct a non-durable writer when the flag is off")
}
