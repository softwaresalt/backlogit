package core

import (
	"context"
	"fmt"

	"github.com/backlogit/backlogit/internal/models"
)

// ReconcileCompletionScope enforces completion consistency for a done parent:
//
//  1. Returns an error if the parent artifact is not in done status.
//  2. Recursively marks every queued or active descendant as done.
//
// Worker: load the parent from the workspace, verify status is done, query
// all descendants by parent_id, mark queued/active ones done via
// setArtifactStatus, and recurse into each child that had children of its own.
func ReconcileCompletionScope(ctx context.Context, ws *Workspace, parentID string) error {
	parent, err := loadArtifact(ctx, ws, parentID)
	if err != nil {
		return fmt.Errorf("reconcile completion scope %s: load parent: %w", parentID, err)
	}
	if parent.Status != models.StatusDone {
		return fmt.Errorf(
			"reconcile completion scope %s: parent status is %q, expected %q",
			parentID, parent.Status, models.StatusDone,
		)
	}
	// Implementation: query children by parent_id, mark queued/active as done,
	// recurse into each child. Uses bldb.QueryItems and setArtifactStatus.
	panic("not implemented: ReconcileCompletionScope")
}
