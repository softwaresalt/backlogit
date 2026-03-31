package core

import (
	"context"
	"database/sql"

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

// QueryQueue retrieves artifacts matching the filter criteria, with optional
// grouping and sorting.
//
// Worker: Build a parameterized SQL query from the filter fields. Execute via
// db.QueryItems or a custom query builder. Apply grouping in Go if GroupBy is set.
// Return QueueView with items, groups, and total count.
func QueryQueue(ctx context.Context, db *sql.DB, filter *QueueFilter) (*QueueView, error) {
	panic("not implemented: Worker: Build and execute filtered queue query with grouping and pagination support")
}

// MoveInQueue changes an artifact's position within its parent's children,
// updating the sort order for sibling items.
//
// Worker: Load siblings under the same parent. Remove the item from its current
// position. Insert at the new position. Update sort_order for all affected siblings.
func MoveInQueue(ctx context.Context, db *sql.DB, itemID string, newPosition int) error {
	panic("not implemented: Worker: Reorder artifact within parent's children by updating sort_order")
}

// BulkUpdateStatus changes the status of multiple items in a single transaction.
//
// Worker: Begin transaction. For each item ID, validate status transition is valid,
// update the Markdown frontmatter, update the DB record, and emit a status_changed event.
// Commit on success, rollback on any failure.
func BulkUpdateStatus(ctx context.Context, db *sql.DB, ws *Workspace, itemIDs []string, newStatus string) (int, error) {
	panic("not implemented: Worker: Bulk status update with transaction safety and event emission")
}
