package core

import (
	"context"
	"fmt"

	bldb "github.com/backlogit/backlogit/internal/db"
	"github.com/backlogit/backlogit/internal/models"
)

// ReconcileCompletionScope enforces completion consistency for a done parent:
//
//  1. Returns an error if the parent artifact is not in done status.
//  2. Recursively marks every queued or active descendant as done.
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

	children, err := bldb.QueryItems(ctx, ws.DB, bldb.QueryFilters{ParentID: parentID})
	if err != nil {
		return fmt.Errorf("reconcile completion scope %s: query children: %w", parentID, err)
	}

	for _, child := range children {
		if child.Status == models.StatusQueued || child.Status == models.StatusActive {
			if _, markErr := setArtifactStatus(ctx, ws, child.ID, models.StatusDone, "reconcile completion scope"); markErr != nil {
				return fmt.Errorf("reconcile completion scope %s: mark child %s done: %w", parentID, child.ID, markErr)
			}
		}
		if recurseErr := ReconcileCompletionScope(ctx, ws, child.ID); recurseErr != nil {
			return fmt.Errorf("reconcile completion scope %s: recurse into %s: %w", parentID, child.ID, recurseErr)
		}
	}
	return nil
}
