package core

import (
	"context"
	"fmt"
	"log/slog"

	bldb "github.com/backlogit/backlogit/internal/db"
	"github.com/backlogit/backlogit/internal/models"
)

// maxReconcileDepth guards against cycles or unexpectedly deep hierarchies.
// The known max hierarchy depth is 3 (feature → task → subtask).
const maxReconcileDepth = 10

// ReconcileCompletionScope enforces completion consistency for a done parent:
//
//  1. Returns an error if the parent artifact is not in done status.
//  2. Recursively marks every queued or active descendant as done.
func ReconcileCompletionScope(ctx context.Context, ws *Workspace, parentID string) error {
	return reconcileCompletionScopeDepth(ctx, ws, parentID, 0)
}

func reconcileCompletionScopeDepth(ctx context.Context, ws *Workspace, parentID string, depth int) error {
	if depth > maxReconcileDepth {
		return fmt.Errorf("reconcile completion scope: max depth %d exceeded at %s — possible cycle or unexpectedly deep hierarchy",
			maxReconcileDepth, parentID)
	}

	slog.Debug("reconciling completion scope", "parent_id", parentID, "depth", depth)

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
			slog.Debug("marking descendant done", "child_id", child.ID, "parent_id", parentID)
			if _, markErr := setArtifactStatus(ctx, ws, child.ID, models.StatusDone, "reconcile completion scope"); markErr != nil {
				return fmt.Errorf("reconcile completion scope %s: mark child %s done: %w", parentID, child.ID, markErr)
			}
		}
		if recurseErr := reconcileCompletionScopeDepth(ctx, ws, child.ID, depth+1); recurseErr != nil {
			return fmt.Errorf("reconcile completion scope %s: recurse into %s: %w", parentID, child.ID, recurseErr)
		}
	}
	return nil
}
