package core

import "context"

// NewWorkspaceWithoutRecoveryForTest creates a workspace without running
// recovery so external test fixtures remain isolated from recovery state.
func NewWorkspaceWithoutRecoveryForTest(ctx context.Context, root string) (*Workspace, error) {
	return newWorkspace(ctx, root, false)
}
