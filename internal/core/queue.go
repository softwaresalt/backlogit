package core

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
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
	Types       []string `json:"types,omitempty"`
	Statuses    []string `json:"statuses,omitempty"`
	ParentID    string   `json:"parent_id,omitempty"`
	Sprint      string   `json:"sprint,omitempty"`
	AssignedTo  string   `json:"assigned_to,omitempty"`
	Labels      []string `json:"labels,omitempty"`
	GroupBy     string   `json:"group_by,omitempty"`
	SortBy      string   `json:"sort_by,omitempty"`
	SortOrder   string   `json:"sort_order,omitempty"`
	Limit       int      `json:"limit,omitempty"`
	Offset      int      `json:"offset,omitempty"`
	OrphansOnly bool     `json:"orphans_only,omitempty"` // when true, return only orphaned items
}

const queuePositionCustomField = "queue_position"

// QueryQueue retrieves artifacts matching the filter criteria, with optional grouping.
func QueryQueue(ctx context.Context, db *sql.DB, filter *QueueFilter) (*QueueView, error) {
	if filter == nil {
		filter = &QueueFilter{}
	}
	statuses := compactStrings(filter.Statuses)
	types := compactStrings(filter.Types)

	// Build WHERE clauses
	var conditions []string
	var args []any

	if len(statuses) > 0 {
		placeholders := make([]string, len(statuses))
		for i, s := range statuses {
			placeholders[i] = "?"
			args = append(args, s)
		}
		conditions = append(conditions, "status IN ("+strings.Join(placeholders, ",")+")")
	}
	if len(types) > 0 {
		placeholders := make([]string, len(types))
		for i, t := range types {
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
	if filter.AssignedTo != "" {
		conditions = append(conditions, "assigned_to = ?")
		args = append(args, filter.AssignedTo)
	}

	query := `SELECT ` + bldb.SelectCols + ` FROM items`
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	query += buildQueueOrderClause(filter.SortBy, filter.SortOrder)
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT %d", filter.Limit)
	}
	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET %d", filter.Offset)
	}

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

	// Filter out items with unresolved dependencies (dependency-aware queue).
	if len(items) > 0 {
		items, err = filterByResolvedDependencies(ctx, db, items)
		if err != nil {
			return nil, fmt.Errorf("filter queue by resolved dependencies: %w", err)
		}
	}

	// Annotate orphan status for items whose ID implies a parent but parent_id is empty.
	for _, item := range items {
		if IsOrphan(item) {
			if item.CustomFields == nil {
				item.CustomFields = map[string]any{}
			}
			item.CustomFields["is_orphan"] = true
		}
	}

	// Filter to orphans only after annotation so callers always get the annotation.
	if filter.OrphansOnly {
		filtered := items[:0]
		for _, item := range items {
			if IsOrphan(item) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
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

func buildQueueOrderClause(sortBy, sortOrder string) string {
	direction := "ASC"
	if strings.EqualFold(strings.TrimSpace(sortOrder), "desc") {
		direction = "DESC"
	}

	queuePositionExpr := `CASE WHEN json_extract(custom_fields, '$.` + queuePositionCustomField + `') IS NULL THEN 1 ELSE 0 END ASC, ` +
		`CAST(COALESCE(json_extract(custom_fields, '$.` + queuePositionCustomField + `'), 0) AS INTEGER) ASC`

	secondary := "created_at ASC"
	switch strings.ToLower(strings.TrimSpace(sortBy)) {
	case "priority":
		secondary = `CASE LOWER(COALESCE(priority, ''))
			WHEN 'critical' THEN 0
			WHEN 'high' THEN 1
			WHEN 'medium' THEN 2
			WHEN 'low' THEN 3
			ELSE 4
		END ` + direction
	case "title":
		secondary = "title " + direction
	case "type":
		secondary = "artifact_type " + direction
	case "status":
		secondary = "status " + direction
	case "updated_at":
		secondary = "updated_at " + direction
	case "created_at":
		secondary = "created_at " + direction
	case "":
		secondary = "created_at ASC"
	}

	return " ORDER BY " + queuePositionExpr + ", " + secondary + ", id ASC"
}

func compactStrings(values []string) []string {
	if len(values) == 0 {
		return nil
	}

	compacted := make([]string, 0, len(values))
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		compacted = append(compacted, value)
	}
	if len(compacted) == 0 {
		return nil
	}
	return compacted
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

// MoveInQueue updates the durable queue order for an item within the caller's
// selected queue filter.
func MoveInQueue(ctx context.Context, ws *Workspace, itemID string, position int, filter *QueueFilter) error {
	if ws == nil {
		return fmt.Errorf("workspace is required")
	}
	if position < 1 {
		return fmt.Errorf("position must be >= 1")
	}

	view, err := QueryQueue(ctx, ws.DB, filter)
	if err != nil {
		return fmt.Errorf("load queue view: %w", err)
	}
	if len(view.Items) == 0 {
		return fmt.Errorf("queue is empty")
	}

	currentIndex := -1
	for i, item := range view.Items {
		if item.ID == itemID {
			currentIndex = i
			break
		}
	}
	if currentIndex < 0 {
		return fmt.Errorf("item %s not found in queue view", itemID)
	}

	if position > len(view.Items) {
		position = len(view.Items)
	}
	targetIndex := position - 1
	reordered := reorderQueueItems(view.Items, currentIndex, targetIndex)
	stamp := time.Now()
	originals := make(map[string]*models.Artifact, len(reordered))
	persistedIDs := make([]string, 0, len(reordered))
	for index, item := range reordered {
		desired := index + 1
		if currentQueuePosition(item) == desired {
			continue
		}
		if _, exists := originals[item.ID]; !exists {
			originals[item.ID] = cloneArtifact(item)
		}
		updated := cloneArtifact(item)
		if updated.CustomFields == nil {
			updated.CustomFields = map[string]any{}
		}
		updated.CustomFields[queuePositionCustomField] = desired
		updated.UpdatedAt = stamp
		if err := persistArtifact(ctx, ws, updated, false); err != nil {
			if rollbackErr := rollbackQueueMove(ctx, ws, originals, persistedIDs); rollbackErr != nil {
				return fmt.Errorf("persist queue position for %s: %w; rollback queue positions: %v", item.ID, err, rollbackErr)
			}
			return fmt.Errorf("persist queue position for %s: %w", item.ID, err)
		}
		persistedIDs = append(persistedIDs, item.ID)
	}

	return nil
}

func reorderQueueItems(items []*models.Artifact, fromIndex, toIndex int) []*models.Artifact {
	if fromIndex == toIndex {
		return items
	}

	reordered := append([]*models.Artifact(nil), items...)
	item := reordered[fromIndex]
	reordered = append(reordered[:fromIndex], reordered[fromIndex+1:]...)
	if toIndex >= len(reordered) {
		return append(reordered, item)
	}

	reordered = append(reordered[:toIndex], append([]*models.Artifact{item}, reordered[toIndex:]...)...)
	return reordered
}

func currentQueuePosition(item *models.Artifact) int {
	if item == nil || item.CustomFields == nil {
		return 0
	}

	raw, ok := item.CustomFields[queuePositionCustomField]
	if !ok {
		return 0
	}

	switch value := raw.(type) {
	case int:
		return value
	case int32:
		return int(value)
	case int64:
		return int(value)
	case float64:
		return int(value)
	case string:
		var parsed int
		if _, err := fmt.Sscanf(value, "%d", &parsed); err == nil {
			return parsed
		}
	}

	return 0
}

func rollbackQueueMove(ctx context.Context, ws *Workspace, originals map[string]*models.Artifact, persistedIDs []string) error {
	var rollbackErrs []error
	for i := len(persistedIDs) - 1; i >= 0; i-- {
		original := originals[persistedIDs[i]]
		if original == nil {
			continue
		}
		if err := persistArtifact(ctx, ws, cloneArtifact(original), false); err != nil {
			rollbackErrs = append(rollbackErrs, fmt.Errorf("%s: %w", persistedIDs[i], err))
		}
	}
	if len(rollbackErrs) == 0 {
		return nil
	}
	return errors.Join(rollbackErrs...)
}

// BulkUpdateResult summarises the outcome of a BulkUpdateStatus operation.
// Succeeded counts items whose Markdown file and DB index were both updated.
// Failed lists item IDs that could not be updated (e.g., missing Markdown file).
// Err carries a non-nil value only for workspace-level failures that prevent the
// batch from starting at all (nil workspace, etc.).
type BulkUpdateResult struct {
	Succeeded int      `json:"succeeded"`
	Failed    []string `json:"failed"`
	Err       error    `json:"error,omitempty"`
}

// BulkUpdateStatus changes the status of multiple items using a Markdown-first
// write path. Each item is updated independently: failures are collected in
// BulkUpdateResult.Failed rather than aborting the entire batch. The SQLite
// index is updated only after the Markdown file has been successfully written.
func BulkUpdateStatus(ctx context.Context, _ *sql.DB, ws *Workspace, itemIDs []string, newStatus string) (*BulkUpdateResult, error) {
	result := &BulkUpdateResult{}
	for _, id := range itemIDs {
		artifact, err := findArtifact(ctx, ws, id)
		if err != nil {
			slog.WarnContext(ctx, "bulk update status: artifact not found, skipping",
				"id", id, "error", err)
			result.Failed = append(result.Failed, id)
			continue
		}
		previousStatus := artifact.Status
		artifact.Status = models.ArtifactStatus(newStatus)
		artifact.UpdatedAt = time.Now()
		if err := persistArtifact(ctx, ws, artifact, shouldRelocateOnStatusChange(previousStatus, artifact.Status)); err != nil {
			slog.WarnContext(ctx, "bulk update status: persist failed, skipping",
				"id", id, "error", err)
			result.Failed = append(result.Failed, id)
			continue
		}
		result.Succeeded++
	}
	return result, nil
}

// filterByResolvedDependencies removes items from the queue whose execution-blocking
// dependencies are not yet in a terminal state (as defined by core.TerminalStatuses:
// done, accepted, archived, shipped, abandoned, rejected).
func filterByResolvedDependencies(ctx context.Context, database *sql.DB, items []*models.Artifact) ([]*models.Artifact, error) {
	terminalStatuses := make(map[string]bool, len(TerminalStatuses))
	for _, s := range TerminalStatuses {
		terminalStatuses[s] = true
	}

	// Build a set of all item IDs and their statuses from the current result set.
	statusMap := make(map[string]string)
	for _, item := range items {
		statusMap[item.ID] = string(item.Status)
	}

	result := make([]*models.Artifact, 0, len(items))
	for _, item := range items {
		deps, err := bldb.GetDependencies(ctx, database, item.ID)
		if err != nil {
			return nil, fmt.Errorf("load dependencies for %s: %w", item.ID, err)
		}
		if len(deps) == 0 {
			result = append(result, item)
			continue
		}

		blocked := false
		for _, dep := range deps {
			// Check if the dependency is in a terminal state.
			depStatus := ""
			if s, ok := statusMap[dep.DependsOn]; ok {
				depStatus = s
			} else {
				// Look up from DB if not in current result set.
				var s sql.NullString
				if scanErr := database.QueryRowContext(ctx, "SELECT status FROM items WHERE id = ?", dep.DependsOn).Scan(&s); scanErr != nil {
					if errors.Is(scanErr, sql.ErrNoRows) {
						s = sql.NullString{}
					} else {
						return nil, fmt.Errorf("look up dependency status for %s (dependency of %s): %w", dep.DependsOn, item.ID, scanErr)
					}
				}
				if s.Valid {
					depStatus = s.String
				}
			}
			if isExecutionBlockingDependency(dep.DepType) && !terminalStatuses[depStatus] {
				blocked = true
				break
			}
		}
		if !blocked {
			result = append(result, item)
		}
	}
	return result, nil
}

func isExecutionBlockingDependency(depType string) bool {
	switch strings.ToLower(strings.TrimSpace(depType)) {
	case "blocks", "parent_of", "relates_to":
		return true
	default:
		return false
	}
}
