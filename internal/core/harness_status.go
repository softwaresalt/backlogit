package core

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/backlogit/backlogit/internal/models"
)

// StatusTransitionRule defines a valid state transition for an artifact type.
type StatusTransitionRule struct {
	From string `json:"from" yaml:"from"`
	To   string `json:"to"   yaml:"to"`
}

// StatusConfig holds the complete status configuration for the workspace.
type StatusConfig struct {
	ValidStatuses []string                         `json:"valid_statuses" yaml:"valid_statuses"`
	Transitions   []StatusTransitionRule           `json:"transitions"    yaml:"transitions"`
	TypeOverrides map[string][]StatusTransitionRule `json:"type_overrides,omitempty" yaml:"type_overrides,omitempty"`
}

// ValidateTransition checks whether moving from currentStatus to newStatus is
// allowed for the given artifact type.
func ValidateTransition(cfg *StatusConfig, artifactType, currentStatus, newStatus string) error {
	if currentStatus == newStatus {
		return nil
	}

	rules := cfg.Transitions
	if overrides, ok := cfg.TypeOverrides[artifactType]; ok {
		rules = overrides
	}

	for _, r := range rules {
		if r.From == currentStatus && r.To == newStatus {
			return nil
		}
	}
	return fmt.Errorf("invalid transition %q → %q for type %q", currentStatus, newStatus, artifactType)
}

// ComputeParentStatus determines the rollup status for a parent artifact based
// on the statuses of its children.
func ComputeParentStatus(ctx context.Context, db *sql.DB, parentID string) (models.ArtifactStatus, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT status FROM items WHERE parent_id = ?`, parentID)
	if err != nil {
		return "", fmt.Errorf("query children: %w", err)
	}
	defer rows.Close()

	var statuses []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return "", fmt.Errorf("scan child status: %w", err)
		}
		statuses = append(statuses, s)
	}
	if err := rows.Err(); err != nil {
		return "", err
	}

	if len(statuses) == 0 {
		return models.StatusQueued, nil
	}

	allDone := true
	for _, s := range statuses {
		if s == string(models.StatusBlocked) {
			return models.StatusBlocked, nil
		}
		if s != string(models.StatusDone) {
			allDone = false
		}
	}
	if allDone {
		return models.StatusDone, nil
	}
	for _, s := range statuses {
		if s == string(models.StatusActive) {
			return models.StatusActive, nil
		}
	}
	return models.StatusQueued, nil
}

// CascadeStatusUpdate propagates a status change to parent artifacts up the hierarchy.
func CascadeStatusUpdate(ctx context.Context, db *sql.DB, ws *Workspace, itemID string) error {
	var parentID sql.NullString
	err := db.QueryRowContext(ctx, `SELECT parent_id FROM items WHERE id = ?`, itemID).Scan(&parentID)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return fmt.Errorf("query parent: %w", err)
	}
	if !parentID.Valid || parentID.String == "" {
		return nil
	}

	newStatus, err := ComputeParentStatus(ctx, db, parentID.String)
	if err != nil {
		return err
	}

	var current string
	err = db.QueryRowContext(ctx, `SELECT status FROM items WHERE id = ?`, parentID.String).Scan(&current)
	if err != nil {
		return fmt.Errorf("query parent status: %w", err)
	}

	if current == string(newStatus) {
		return nil
	}

	if _, err := db.ExecContext(ctx, `UPDATE items SET status = ? WHERE id = ?`, newStatus, parentID.String); err != nil {
		return fmt.Errorf("update parent status: %w", err)
	}

	return CascadeStatusUpdate(ctx, db, ws, parentID.String)
}
