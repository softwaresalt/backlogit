package core_test

// F3 (106.002-T) characterization: pin the 6-status downward blocking cascade set
// {done, accepted, archived, shipped, abandoned, rejected} through its real
// consumer, CheckChildrenTerminal. A parent may reach a terminal status only when
// EVERY child is terminal. This references NO taxonomy symbol (it forces the child
// status directly in the DB and reads it back through CheckChildrenTerminal) so it
// survives the refactor unchanged and remains a golden truth-table regression guard.

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
)

// forceChildStatus creates a child task under parentID and forces its status
// directly in the DB, bypassing transition validation so the full status universe
// (including terminal statuses with no direct transition path) can be exercised.
func forceChildStatus(t *testing.T, ctx context.Context, ws *core.Workspace, parentID, title, status string) string {
	t.Helper()
	child, err := core.CreateArtifact(ctx, ws, title, "task", core.WithParent(parentID))
	require.NoError(t, err)
	_, err = ws.DB.ExecContext(ctx, `UPDATE items SET status = ? WHERE id = ?`, status, child.ID)
	require.NoError(t, err)
	return child.ID
}

// TestCharacterization_CheckChildrenTerminal_SixStatusSet pins which single-child
// statuses let a parent complete (terminal → no block) versus which block it
// (non-terminal → *ChildBlockingError). The six terminal statuses and four
// non-terminal statuses partition the lifecycle enum exactly as they do today.
func TestCharacterization_CheckChildrenTerminal_SixStatusSet(t *testing.T) {
	cases := []struct {
		status    string
		wantBlock bool
	}{
		// 6-status cascade terminal set (parent may complete).
		{"done", false},
		{"accepted", false},
		{"archived", false},
		{"shipped", false},
		{"abandoned", false},
		{"rejected", false},
		// Non-terminal (parent blocked).
		{"queued", true},
		{"active", true},
		{"blocked", true},
		{"review", true},
	}
	for _, tc := range cases {
		t.Run(tc.status, func(t *testing.T) {
			ws := setupCascadeWorkspace(t)
			ctx := context.Background()

			parent, err := core.CreateArtifact(ctx, ws, "Parent", "feature")
			require.NoError(t, err)
			forceChildStatus(t, ctx, ws, parent.ID, "Child", tc.status)

			err = core.CheckChildrenTerminal(ctx, ws.DB, parent.ID)
			if tc.wantBlock {
				var blockErr *core.ChildBlockingError
				require.True(t, errors.As(err, &blockErr),
					"status %q must block parent completion", tc.status)
				require.Len(t, blockErr.Children, 1)
				assert.Equal(t, tc.status, blockErr.Children[0].Status)
			} else {
				require.NoError(t, err,
					"status %q must be treated as cascade-terminal", tc.status)
			}
		})
	}
}

// TestCharacterization_CheckChildrenTerminal_ParentTerminalOnlyWhenEveryChildTerminal
// pins the AND semantics: a parent with one terminal (done) child and one
// non-terminal (active) child is blocked, and only the non-terminal child is
// reported.
func TestCharacterization_CheckChildrenTerminal_ParentTerminalOnlyWhenEveryChildTerminal(t *testing.T) {
	ws := setupCascadeWorkspace(t)
	ctx := context.Background()

	parent, err := core.CreateArtifact(ctx, ws, "Parent", "feature")
	require.NoError(t, err)
	forceChildStatus(t, ctx, ws, parent.ID, "Terminal child", "done")
	activeID := forceChildStatus(t, ctx, ws, parent.ID, "In-flight child", "active")

	err = core.CheckChildrenTerminal(ctx, ws.DB, parent.ID)

	var blockErr *core.ChildBlockingError
	require.True(t, errors.As(err, &blockErr))
	require.Len(t, blockErr.Children, 1, "only the non-terminal child blocks")
	assert.Equal(t, activeID, blockErr.Children[0].ID)
	assert.Equal(t, "active", blockErr.Children[0].Status)
}
