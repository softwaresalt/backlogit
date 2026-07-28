package core_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/core"
)

// TestNewWorkspaceEventWriter_DurableOn asserts the helper constructs a durable
// writer when the workspace config enables durable_writes.
func TestNewWorkspaceEventWriter_DurableOn(t *testing.T) {
	t.Parallel()
	ws := &core.Workspace{Config: &config.WorkspaceConfig{DurableWrites: true}}
	w := core.NewWorkspaceEventWriter(ws, t.TempDir())
	assert.True(t, w.Durable(), "durable_writes on must yield a durable event writer")
}

// TestNewWorkspaceEventWriter_DurableOff asserts the helper leaves durability
// off when the flag is off (construction unchanged).
func TestNewWorkspaceEventWriter_DurableOff(t *testing.T) {
	t.Parallel()
	ws := &core.Workspace{Config: &config.WorkspaceConfig{DurableWrites: false}}
	w := core.NewWorkspaceEventWriter(ws, t.TempDir())
	assert.False(t, w.Durable(), "durable_writes off must yield a non-durable writer")
}

// TestNewWorkspaceEventWriter_NilSafe asserts a nil workspace or nil config
// yields a durable-off writer rather than panicking.
func TestNewWorkspaceEventWriter_NilSafe(t *testing.T) {
	t.Parallel()
	assert.False(t, core.NewWorkspaceEventWriter(nil, t.TempDir()).Durable())
	assert.False(t, core.NewWorkspaceEventWriter(&core.Workspace{}, t.TempDir()).Durable())
}

// TestWorkspaceDurableWrites_NilSafe covers the flag resolver directly.
func TestWorkspaceDurableWrites_NilSafe(t *testing.T) {
	t.Parallel()
	assert.False(t, core.WorkspaceDurableWrites(nil))
	assert.False(t, core.WorkspaceDurableWrites(&core.Workspace{}))
	assert.True(t, core.WorkspaceDurableWrites(&core.Workspace{Config: &config.WorkspaceConfig{DurableWrites: true}}))
}
