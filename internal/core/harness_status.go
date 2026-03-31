package core

import (
	"context"
	"database/sql"

	"github.com/backlogit/backlogit/internal/models"
)

// StatusTransitionRule defines a valid state transition for an artifact type.
type StatusTransitionRule struct {
	From string `json:"from" yaml:"from"`
	To   string `json:"to"   yaml:"to"`
}

// StatusConfig holds the complete status configuration for the workspace.
type StatusConfig struct {
	ValidStatuses []string               `json:"valid_statuses" yaml:"valid_statuses"`
	Transitions   []StatusTransitionRule  `json:"transitions"    yaml:"transitions"`
	TypeOverrides map[string][]StatusTransitionRule `json:"type_overrides,omitempty" yaml:"type_overrides,omitempty"`
}

// ValidateTransition checks whether moving from currentStatus to newStatus is
// allowed for the given artifact type.
//
// Worker: Look up the type in TypeOverrides first, fall back to global Transitions.
// Return nil if the transition is valid, or an error describing the invalid transition.
func ValidateTransition(cfg *StatusConfig, artifactType, currentStatus, newStatus string) error {
	panic("not implemented: Worker: Validate status transition against config rules with type-specific overrides")
}

// ComputeParentStatus determines the rollup status for a parent artifact based
// on the statuses of its children.
//
// Worker: Query all children of parentID. Apply rollup rules:
// - If all children are "done" → parent is "done"
// - If any child is "blocked" → parent is "blocked"
// - If any child is "active" → parent is "active"
// - Otherwise → parent stays "queued"
// Return the computed status (do not apply it).
func ComputeParentStatus(ctx context.Context, db *sql.DB, parentID string) (models.ArtifactStatus, error) {
	panic("not implemented: Worker: Compute rollup status from children statuses using priority rules")
}

// CascadeStatusUpdate propagates a status change to parent artifacts up the
// hierarchy, recalculating rollup status at each level.
//
// Worker: After a child's status changes, look up its parent. Call
// ComputeParentStatus. If the parent's status should change, update it and
// recurse to the grandparent. Stop when status is unchanged or root is reached.
func CascadeStatusUpdate(ctx context.Context, db *sql.DB, ws *Workspace, itemID string) error {
	panic("not implemented: Worker: Propagate status changes up the artifact hierarchy via rollup computation")
}
