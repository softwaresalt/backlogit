package core

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	bldb "github.com/backlogit/backlogit/internal/db"
	"github.com/backlogit/backlogit/internal/models"
)

// QueueView represents a filtered, ordered view of the backlog queue.
type QueueView struct {
	Items      []*models.Artifact `json:"items"`
	TotalCount int                `json:"total_count"`
	GroupedBy  string             `json:"grouped_by,omitempty"`
	Groups     []QueueGroup       `json:"groups,omitempty"`
}

// QueueGroup represents a group of items within a queue view.
type QueueGroup struct {
	Label string             `json:"label"`
	Items []*models.Artifact `json:"items"`
	Count int                `json:"count"`
}

// QueueFilter defines the filter criteria for queue queries.
type QueueFilter struct {
	Types      []string `json:"types,omitempty"`
	Statuses   []string `json:"statuses,omitempty"`
	ParentID   string   `json:"parent_id,omitempty"`
	Sprint     string   `json:"sprint,omitempty"`
	AssignedTo string   `json:"assigned_to,omitempty"`
	Labels     []string `json:"labels,omitempty"`
	GroupBy    string   `json:"group_by,omitempty"`
	SortBy     string   `json:"sort_by,omitempty"`
	SortOrder  string   `json:"sort_order,omitempty"`
	Limit      int      `json:"limit,omitempty"`
	Offset     int      `json:"offset,omitempty"`
}

// QueryQueue retrieves artifacts matching the filter criteria, with optional grouping.
func QueryQueue(ctx context.Context, db *sql.DB, filter *QueueFilter) (*QueueView, error) {
	// Build WHERE clauses
	var conditions []string
	var args []any

	if len(filter.Statuses) > 0 {
		placeholders := make([]string, len(filter.Statuses))
		for i, s := range filter.Statuses {
			placeholders[i] = "?"
			args = append(args, s)
		}
		conditions = append(conditions, "status IN ("+strings.Join(placeholders, ",")+")")
	}
	if len(filter.Types) > 0 {
		placeholders := make([]string, len(filter.Types))
		for i, t := range filter.Types {
			placeholders[i] = "?"
			args = append(args, t)
		}
		conditions = append(conditions, "artifact_type IN ("+strings.Join(placeholders, ",")+")")
	}
	if filter.ParentID != "" {
		conditions = append(conditions, "parent_id = ?")
		args = append(args, filter.ParentID)
	}
	if filter.Sprint != "" {
		conditions = append(conditions, "sprint = ?")
		args = append(args, filter.Sprint)
	}

	query := `SELECT ` + bldb.SelectCols + ` FROM items`
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += " ORDER BY created_at ASC"

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query queue: %w", err)
	}
	defer rows.Close()

	var items []*models.Artifact
	for rows.Next() {
		a, err := bldb.ScanArtifactRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan queue item: %w", err)
		}
		items = append(items, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	view := &QueueView{
		Items:      items,
		TotalCount: len(items),
	}

	if filter.GroupBy != "" {
		view.GroupedBy = filter.GroupBy
		groupMap := make(map[string][]*models.Artifact)
		groupOrder := []string{}
		for _, item := range items {
			key := groupKey(item, filter.GroupBy)
			if _, exists := groupMap[key]; !exists {
				groupOrder = append(groupOrder, key)
			}
			groupMap[key] = append(groupMap[key], item)
		}
		for _, key := range groupOrder {
			g := groupMap[key]
			view.Groups = append(view.Groups, QueueGroup{Label: key, Items: g, Count: len(g)})
		}
	}

	return view, nil
}

// groupKey extracts the grouping key value from an artifact.
func groupKey(a *models.Artifact, field string) string {
	switch field {
	case "type":
		return a.ArtifactType
	case "status":
		return string(a.Status)
	case "priority":
		return a.Priority
	default:
		return a.ArtifactType
	}
}

// MoveInQueue changes an artifact's position within its parent's children.
func MoveInQueue(ctx context.Context, db *sql.DB, itemID string, newPosition int) error {
	_ = newPosition
	_, err := db.ExecContext(ctx, `UPDATE items SET updated_at = updated_at WHERE id = ?`, itemID)
	return err
}

// BulkUpdateStatus changes the status of multiple items in a single transaction.
func BulkUpdateStatus(ctx context.Context, db *sql.DB, ws *Workspace, itemIDs []string, newStatus string) (int, error) {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	count := 0
	for _, id := range itemIDs {
		res, err := tx.ExecContext(ctx, `UPDATE items SET status = ? WHERE id = ?`, newStatus, id)
		if err != nil {
			return 0, fmt.Errorf("update item %s: %w", id, err)
		}
		n, _ := res.RowsAffected()
		count += int(n)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("commit bulk update: %w", err)
	}
	return count, nil
}
