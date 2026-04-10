package core_test

// 018.004-T: blocking downward cascade harness.
//
// Implementation complete. These tests validate the blocking cascade contract
// for CheckChildrenTerminal:
//
//   - parent cannot move to "done" while non-terminal children exist
//   - *ChildBlockingError contains the blocking child IDs and statuses
//   - SkipChildCheck() option bypasses the check entirely
//   - a parent whose children are all terminal transitions without error

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/config"
	"github.com/backlogit/backlogit/internal/core"
	corerrors "github.com/backlogit/backlogit/internal/errors"
)

func setupCascadeWorkspace(t *testing.T) *core.Workspace {
	t.Helper()
	root := t.TempDir()
	backlogDir := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(backlogDir, 0o755))
	require.NoError(t, config.WriteDefaults(backlogDir))
	ctx := context.Background()
	ws, err := core.NewWorkspace(ctx, root)
	require.NoError(t, err)
	t.Cleanup(func() { ws.Close() })
	return ws
}

func TestCheckChildrenTerminal_AllDone_NoError(t *testing.T) {
	ws := setupCascadeWorkspace(t)
	ctx := context.Background()

	parent, err := core.CreateArtifact(ctx, ws, "Parent feature", "feature")
	require.NoError(t, err)

	child, err := core.CreateArtifact(ctx, ws, "Done child", "task", core.WithParent(parent.ID))
	require.NoError(t, err)
	_, err = core.UpdateArtifact(ctx, ws, child.ID, map[string]any{"status": "done"})
	require.NoError(t, err)

	err = core.CheckChildrenTerminal(ctx, ws.DB, parent.ID)

	require.NoError(t, err)
}

func TestCheckChildrenTerminal_NonTerminalChild_ReturnsBlockingError(t *testing.T) {
	ws := setupCascadeWorkspace(t)
	ctx := context.Background()

	parent, err := core.CreateArtifact(ctx, ws, "Parent feature", "feature")
	require.NoError(t, err)

	active, err := core.CreateArtifact(ctx, ws, "Active child", "task", core.WithParent(parent.ID))
	require.NoError(t, err)
	_, err = core.UpdateArtifact(ctx, ws, active.ID, map[string]any{"status": "active"})
	require.NoError(t, err)

	err = core.CheckChildrenTerminal(ctx, ws.DB, parent.ID)

	require.Error(t, err)
	assert.True(t, errors.Is(err, corerrors.ErrChildrenNotTerminal),
		"error must satisfy errors.Is(err, ErrChildrenNotTerminal)")

	var blockErr *core.ChildBlockingError
	require.True(t, errors.As(err, &blockErr), "error must be *ChildBlockingError")
	assert.Equal(t, parent.ID, blockErr.ParentID)
	require.Len(t, blockErr.Children, 1)
	assert.Equal(t, active.ID, blockErr.Children[0].ID)
	assert.Equal(t, "active", blockErr.Children[0].Status)
}

func TestCheckChildrenTerminal_MultipleNonTerminal_AllReported(t *testing.T) {
	ws := setupCascadeWorkspace(t)
	ctx := context.Background()

	parent, err := core.CreateArtifact(ctx, ws, "Parent", "feature")
	require.NoError(t, err)

	child1, err := core.CreateArtifact(ctx, ws, "Active child 1", "task", core.WithParent(parent.ID))
	require.NoError(t, err)
	_, err = core.UpdateArtifact(ctx, ws, child1.ID, map[string]any{"status": "active"})
	require.NoError(t, err)

	child2, err := core.CreateArtifact(ctx, ws, "Blocked child 2", "task", core.WithParent(parent.ID))
	require.NoError(t, err)
	_, err = core.UpdateArtifact(ctx, ws, child2.ID, map[string]any{"status": "blocked"})
	require.NoError(t, err)

	err = core.CheckChildrenTerminal(ctx, ws.DB, parent.ID)

	var blockErr *core.ChildBlockingError
	require.True(t, errors.As(err, &blockErr))
	assert.Len(t, blockErr.Children, 2)
}

func TestCheckChildrenTerminal_NoChildren_NoError(t *testing.T) {
	ws := setupCascadeWorkspace(t)
	ctx := context.Background()

	parent, err := core.CreateArtifact(ctx, ws, "Childless feature", "feature")
	require.NoError(t, err)

	err = core.CheckChildrenTerminal(ctx, ws.DB, parent.ID)

	require.NoError(t, err)
}

func TestCheckChildrenTerminal_SkipChildCheck_BypassesCheck(t *testing.T) {
	ws := setupCascadeWorkspace(t)
	ctx := context.Background()

	parent, err := core.CreateArtifact(ctx, ws, "Parent", "feature")
	require.NoError(t, err)

	active, err := core.CreateArtifact(ctx, ws, "Active child", "task", core.WithParent(parent.ID))
	require.NoError(t, err)
	_, err = core.UpdateArtifact(ctx, ws, active.ID, map[string]any{"status": "active"})
	require.NoError(t, err)

	err = core.CheckChildrenTerminal(ctx, ws.DB, parent.ID, core.SkipChildCheck())

	require.NoError(t, err, "SkipChildCheck must bypass the cascade check entirely")
}

func TestCheckChildrenTerminal_AcceptedStatus_IsTerminal(t *testing.T) {
	ws := setupCascadeWorkspace(t)
	ctx := context.Background()

	parent, err := core.CreateArtifact(ctx, ws, "Parent", "feature")
	require.NoError(t, err)

	child, err := core.CreateArtifact(ctx, ws, "Accepted child", "task", core.WithParent(parent.ID))
	require.NoError(t, err)
	_, err = core.UpdateArtifact(ctx, ws, child.ID, map[string]any{"status": "accepted"})
	require.NoError(t, err)

	err = core.CheckChildrenTerminal(ctx, ws.DB, parent.ID)

	require.NoError(t, err, "accepted status must be considered terminal")
}
