package core

import (
	"context"
	"database/sql"
	"fmt"

	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// TerminalStatuses lists the artifact statuses that satisfy the downward
// blocking cascade check. A parent may only move to a terminal status when
// all of its children are in one of these states.
var TerminalStatuses = []string{"done", "accepted", "archived", "shipped", "abandoned", "rejected"}

// StatusOption configures optional behavior for status-change operations.
type StatusOption func(*statusCheckOptions)

type statusCheckOptions struct {
	skipChildCheck bool
}

// SkipChildCheck returns a StatusOption that bypasses the child-terminal check
// during a status transition. Shipment automation uses this when releasing
// items in batch so it is not blocked by its own cascade logic.
func SkipChildCheck() StatusOption {
	return func(o *statusCheckOptions) {
		o.skipChildCheck = true
	}
}

// ChildStatus captures the ID and current status of a non-terminal child.
type ChildStatus struct {
	ID     string `json:"id"`
	Status string `json:"status"`
}

// ChildBlockingError is returned when a parent artifact cannot move to a
// terminal status because one or more of its children are still in progress.
type ChildBlockingError struct {
	ParentID string
	Children []ChildStatus
}

// Error implements the error interface.
func (e *ChildBlockingError) Error() string {
	return fmt.Sprintf("parent %s has %d non-terminal child(ren) blocking completion", e.ParentID, len(e.Children))
}

// Is enables errors.Is matching against ErrChildrenNotTerminal.
func (e *ChildBlockingError) Is(target error) bool {
	return target == corerrors.ErrChildrenNotTerminal
}

// CheckChildrenTerminal verifies that every child of parentID has a terminal
// status. It returns a *ChildBlockingError when any non-terminal children are
// found, and nil when all children are terminal (or the parent has no children).
// Pass SkipChildCheck() to bypass this check entirely.
func CheckChildrenTerminal(ctx context.Context, database *sql.DB, parentID string, opts ...StatusOption) error {
	o := &statusCheckOptions{}
	for _, opt := range opts {
		opt(o)
	}
	if o.skipChildCheck {
		return nil
	}

	terminalSet := make(map[string]bool, len(TerminalStatuses))
	for _, s := range TerminalStatuses {
		terminalSet[s] = true
	}

	rows, err := database.QueryContext(ctx,
		`SELECT id, status FROM items WHERE parent_id = ?`, parentID,
	)
	if err != nil {
		return fmt.Errorf("query children of %s: %w", parentID, err)
	}
	defer rows.Close()

	var blocking []ChildStatus
	for rows.Next() {
		var id, status string
		if err := rows.Scan(&id, &status); err != nil {
			return fmt.Errorf("scan child row: %w", err)
		}
		if !terminalSet[status] {
			blocking = append(blocking, ChildStatus{ID: id, Status: status})
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate children: %w", err)
	}

	if len(blocking) > 0 {
		return &ChildBlockingError{ParentID: parentID, Children: blocking}
	}
	return nil
}
