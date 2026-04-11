package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	blErrors "github.com/backlogit/backlogit/internal/errors"
	"github.com/backlogit/backlogit/internal/models"
)

// QueryFilters holds optional filters for item queries.
type QueryFilters struct {
	Status          string
	Type            string
	ParentID        string
	Sprint          string
	AssignedTo      string
	Owner           string
	Priority        string
	IncludeArchived bool // when false (default), archived items are excluded from results
	Limit           int  // max results to return (0 = no limit)
	Offset          int  // number of results to skip for pagination
}

const selectCols = `id, title, status, artifact_type, parent_id, sprint, priority, description, custom_fields, created_at, updated_at, assigned_to, owner, labels, dependencies, "references", "commit", level, hierarchy_path`

// rowScanner abstracts *sql.Row and *sql.Rows for the shared scan helper.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanArtifactRow scans a single row into an Artifact.
func scanArtifactRow(row rowScanner) (*models.Artifact, error) {
	var a models.Artifact
	var status, createdAt, updatedAt string
	var parentID, sprint, priority, description, customFields sql.NullString
	var assignedTo, owner, labels, dependencies, references, commit sql.NullString
	var level sql.NullInt64
	var hierarchyPath sql.NullString

	if err := row.Scan(
		&a.ID, &a.Title, &status, &a.ArtifactType,
		&parentID, &sprint, &priority, &description,
		&customFields, &createdAt, &updatedAt,
		&assignedTo, &owner, &labels, &dependencies, &references, &commit,
		&level, &hierarchyPath,
	); err != nil {
		return nil, err
	}

	a.Status = models.ArtifactStatus(status)
	if parentID.Valid {
		a.ParentID = parentID.String
	}
	if sprint.Valid {
		a.Sprint = sprint.String
	}
	if priority.Valid {
		a.Priority = priority.String
	}
	if description.Valid {
		a.Description = description.String
	}
	if customFields.Valid && customFields.String != "" && customFields.String != "null" {
		if err := json.Unmarshal([]byte(customFields.String), &a.CustomFields); err != nil {
			return nil, fmt.Errorf("unmarshal custom fields: %w", err)
		}
	}

	var createdAtParseErr, updatedAtParseErr error
	a.CreatedAt, createdAtParseErr = time.Parse(time.RFC3339Nano, createdAt)
	if createdAtParseErr != nil {
		// Fall back to second-precision RFC3339 for legacy records.
		a.CreatedAt, createdAtParseErr = time.Parse(time.RFC3339, createdAt)
	}
	if createdAtParseErr != nil {
		return nil, fmt.Errorf("parse created_at %q: %w", createdAt, createdAtParseErr)
	}
	a.UpdatedAt, updatedAtParseErr = time.Parse(time.RFC3339Nano, updatedAt)
	if updatedAtParseErr != nil {
		// Fall back to second-precision RFC3339 for legacy records.
		a.UpdatedAt, updatedAtParseErr = time.Parse(time.RFC3339, updatedAt)
	}
	if updatedAtParseErr != nil {
		return nil, fmt.Errorf("parse updated_at %q: %w", updatedAt, updatedAtParseErr)
	}

	if assignedTo.Valid {
		a.AssignedTo = assignedTo.String
	}
	if owner.Valid {
		a.Owner = owner.String
	}
	if commit.Valid {
		a.Commit = commit.String
	}
	if labels.Valid && labels.String != "" && labels.String != "null" {
		if err := json.Unmarshal([]byte(labels.String), &a.Labels); err != nil {
			return nil, fmt.Errorf("unmarshal labels: %w", err)
		}
	}
	if dependencies.Valid && dependencies.String != "" && dependencies.String != "null" {
		if err := json.Unmarshal([]byte(dependencies.String), &a.Dependencies); err != nil {
			return nil, fmt.Errorf("unmarshal dependencies: %w", err)
		}
	}
	if references.Valid && references.String != "" && references.String != "null" {
		if err := json.Unmarshal([]byte(references.String), &a.References); err != nil {
			return nil, fmt.Errorf("unmarshal references: %w", err)
		}
	}

	if level.Valid {
		a.Level = int(level.Int64)
	}
	if hierarchyPath.Valid {
		a.HierarchyPath = hierarchyPath.String
	}

	return &a, nil
}

// UpsertItem inserts or replaces an artifact in the index.
func UpsertItem(ctx context.Context, db *sql.DB, artifact *models.Artifact) error {
	cf, err := json.Marshal(artifact.CustomFields)
	if err != nil {
		return fmt.Errorf("marshal custom fields: %w", err)
	}

	labelsJSON, err := json.Marshal(artifact.Labels)
	if err != nil {
		return fmt.Errorf("marshal labels: %w", err)
	}
	depsJSON, err := json.Marshal(artifact.Dependencies)
	if err != nil {
		return fmt.Errorf("marshal dependencies: %w", err)
	}
	refsJSON, err := json.Marshal(artifact.References)
	if err != nil {
		return fmt.Errorf("marshal references: %w", err)
	}

	// Store JSON slice fields as NULL when empty to keep rows tidy.
	labelsVal := nullString(string(labelsJSON))
	if string(labelsJSON) == "null" {
		labelsVal = sql.NullString{}
	}
	depsVal := nullString(string(depsJSON))
	if string(depsJSON) == "null" {
		depsVal = sql.NullString{}
	}
	refsVal := nullString(string(refsJSON))
	if string(refsJSON) == "null" {
		refsVal = sql.NullString{}
	}

	_, err = db.ExecContext(ctx,
		`INSERT OR REPLACE INTO items
			(id, title, status, artifact_type, parent_id, sprint, priority, description,
			 custom_fields, created_at, updated_at,
			 assigned_to, owner, labels, dependencies, "references", "commit",
			 level, hierarchy_path)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		artifact.ID,
		artifact.Title,
		string(artifact.Status),
		artifact.ArtifactType,
		nullString(artifact.ParentID),
		nullString(artifact.Sprint),
		nullString(artifact.Priority),
		nullString(artifact.Description),
		string(cf),
		artifact.CreatedAt.Format(time.RFC3339Nano),
		artifact.UpdatedAt.Format(time.RFC3339Nano),
		nullString(artifact.AssignedTo),
		nullString(artifact.Owner),
		labelsVal,
		depsVal,
		refsVal,
		nullString(artifact.Commit),
		nullInt64(artifact.Level),
		nullString(artifact.HierarchyPath),
	)
	if err != nil {
		return fmt.Errorf("upsert item %s: %w", artifact.ID, err)
	}
	return nil
}

// GetItem retrieves a single artifact by ID.
func GetItem(ctx context.Context, db *sql.DB, id string) (*models.Artifact, error) {
	row := db.QueryRowContext(ctx,
		`SELECT `+selectCols+` FROM items WHERE id = ?`, id)
	a, err := scanArtifactRow(row)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("get item %s: %w", id, blErrors.ErrNotFound)
	}
	if err != nil {
		return nil, fmt.Errorf("get item %s: %w", id, err)
	}
	return a, nil
}

// DeleteItem removes an artifact from the index.
// DeleteItem removes an artifact from the index together with all related rows
// in a single atomic transaction. It delegates to DeleteItemCascade.
//
// Deprecated: call DeleteItemCascade directly; this shim exists for callsite
// compatibility only.
func DeleteItem(ctx context.Context, db *sql.DB, id string) error {
	return DeleteItemCascade(ctx, db, id)
}

// deleteStep describes one table/column pair to delete during a cascade.
type deleteStep struct {
	table string
	col   string
}

// cascadeSteps is the ordered sequence of DELETE operations performed when an
// item is removed. Dependent rows are removed before the item row itself to
// respect FK semantics in databases that enforce them.
var cascadeSteps = []deleteStep{
	{table: "item_log_entries", col: "item_id"},
	{table: "item_logs", col: "item_id"},
	{table: "item_links", col: "source_id"},
	{table: "item_links", col: "target_id"},
	{table: "item_deps", col: "item_id"},
	{table: "item_deps", col: "depends_on"},
	{table: "stash_links", col: "item_id"},
	{table: "commit_links", col: "item_id"},
}

// DeleteItemCascade removes an artifact and all its related rows (deps, links,
// logs, events, stash links, commit links) from the index in a single atomic
// transaction. It returns ErrNotFound when no items row matched id.
func DeleteItemCascade(ctx context.Context, database *sql.DB, id string) error {
	tx, err := database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("delete item %s: begin cascade transaction: %w", id, err)
	}
	defer tx.Rollback() //nolint:errcheck

	for _, step := range cascadeSteps {
		stmt := fmt.Sprintf("DELETE FROM %s WHERE %s = ?", step.table, step.col)
		if _, err := tx.ExecContext(ctx, stmt, id); err != nil {
			return fmt.Errorf("delete item %s: cascade %s.%s: %w", id, step.table, step.col, err)
		}
	}

	result, err := tx.ExecContext(ctx, `DELETE FROM items WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete item %s: %w", id, err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete item %s: rows affected: %w", id, err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("delete item %s: %w", id, blErrors.ErrNotFound)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("delete item %s: commit cascade transaction: %w", id, err)
	}
	return nil
}

// QueryItems retrieves artifacts matching the provided filters.
func QueryItems(ctx context.Context, db *sql.DB, filters QueryFilters) ([]*models.Artifact, error) {
	query := `SELECT ` + selectCols + ` FROM items`
	var conditions []string
	var args []any

	if filters.Status != "" {
		conditions = append(conditions, "status = ?")
		args = append(args, filters.Status)
	} else if !filters.IncludeArchived {
		// Exclude archived items from all default queries unless explicitly requested.
		conditions = append(conditions, "status != ?")
		args = append(args, string(models.StatusArchived))
	}
	if filters.Type != "" {
		conditions = append(conditions, "artifact_type = ?")
		args = append(args, filters.Type)
	}
	if filters.ParentID != "" {
		conditions = append(conditions, "parent_id = ?")
		args = append(args, filters.ParentID)
	}
	if filters.Sprint != "" {
		conditions = append(conditions, "sprint = ?")
		args = append(args, filters.Sprint)
	}
	if filters.AssignedTo != "" {
		conditions = append(conditions, "assigned_to = ?")
		args = append(args, filters.AssignedTo)
	}
	if filters.Owner != "" {
		conditions = append(conditions, "owner = ?")
		args = append(args, filters.Owner)
	}
	if filters.Priority != "" {
		conditions = append(conditions, "priority = ?")
		args = append(args, filters.Priority)
	}
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	if filters.Limit > 0 {
		// ORDER BY is required for stable, non-overlapping pages across calls.
		query += " ORDER BY id ASC LIMIT ?"
		args = append(args, filters.Limit)
		if filters.Offset > 0 {
			query += " OFFSET ?"
			args = append(args, filters.Offset)
		}
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query items: %w", err)
	}
	defer rows.Close()

	return scanArtifacts(rows)
}

// SearchItems performs FTS5 full-text search across titles, descriptions, and labels.
// The query is wrapped in FTS5 phrase-quote delimiters so that hyphens and
// other FTS5 operator characters in the input are treated as literal phrase
// content rather than query operators.
func SearchItems(ctx context.Context, db *sql.DB, query string, limit int) ([]*models.Artifact, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT i.`+strings.ReplaceAll(selectCols, ", ", ", i.")+
			` FROM items i
			  JOIN items_fts fts ON i.rowid = fts.rowid
			 WHERE items_fts MATCH ?
			 LIMIT ?`,
		escapeFTS5Query(query), limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search items: %w", err)
	}
	defer rows.Close()

	return scanArtifacts(rows)
}

// escapeFTS5Query wraps q in FTS5 double-quote phrase delimiters and escapes
// any embedded double-quotes so that operators like hyphens, OR, and AND are
// treated as literal characters rather than FTS5 query syntax.
func escapeFTS5Query(q string) string {
	return `"` + strings.ReplaceAll(q, `"`, `""`) + `"`
}

// ScanArtifactRow is the exported wrapper for scanning a single row into an Artifact.
// It is used by packages that build custom queries against the items table.
func ScanArtifactRow(row rowScanner) (*models.Artifact, error) {
	return scanArtifactRow(row)
}

// SelectCols is the canonical column list for SELECT queries against the items table.
const SelectCols = selectCols

// scanArtifacts collects all rows into a slice of Artifacts.
func scanArtifacts(rows *sql.Rows) ([]*models.Artifact, error) {
	var artifacts []*models.Artifact
	for rows.Next() {
		a, err := scanArtifactRow(rows)
		if err != nil {
			return nil, fmt.Errorf("scan artifact row: %w", err)
		}
		artifacts = append(artifacts, a)
	}
	return artifacts, rows.Err()
}

// nullString converts an empty string to a NULL-valued sql.NullString.
func nullString(s string) sql.NullString {
	return sql.NullString{String: s, Valid: s != ""}
}

// nullInt64 converts a zero int to a NULL-valued sql.NullInt64.
func nullInt64(n int) sql.NullInt64 {
	return sql.NullInt64{Int64: int64(n), Valid: n != 0}
}
