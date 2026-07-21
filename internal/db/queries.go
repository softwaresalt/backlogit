package db

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	blErrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
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

	return RetryWrite(ctx, func() error {
		// Use a distinct variable (execErr) rather than reassigning the outer err
		// so the closure does not shadow the earlier marshalling errors.
		_, execErr := db.ExecContext(ctx,
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
		if execErr != nil {
			return fmt.Errorf("upsert item %s: %w", artifact.ID, execErr)
		}
		return nil
	})
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

// GetItemsByIDs resolves multiple artifacts from the index in a single indexed
// query per chunk (chunked to respect SQLite's bound-parameter limit), returning
// a map keyed by artifact ID. Input IDs are de-duplicated and empties are
// ignored; an ID with no indexed row is simply absent from the result map (a
// miss is not an error). It is the batch resolver behind the size-composition
// rollup, replacing per-member filesystem lookups (114-F / 47ED88ED).
func GetItemsByIDs(ctx context.Context, db *sql.DB, ids []string) (map[string]*models.Artifact, error) {
	result := make(map[string]*models.Artifact, len(ids))
	if db == nil || len(ids) == 0 {
		return result, nil
	}

	seen := make(map[string]struct{}, len(ids))
	unique := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		unique = append(unique, id)
	}

	const chunkSize = 900 // stay below SQLite's default 999 bound-parameter limit
	// Run each chunk as an implicit (deferred) read against the pooled handle
	// rather than wrapping the batch in an explicit BeginTx. db.Open configures
	// _txlock=immediate (connection.go), so an explicit ReadOnly transaction would
	// still acquire the write lock up front and serialize every composition read
	// behind writers (up to the busy timeout), defeating WAL reader/writer
	// concurrency. Implicit single-statement SELECTs use SQLite's deferred read
	// locking and keep that concurrency. size_composition is a best-effort,
	// computed-on-read rollup that already tolerates a staleness window, so
	// cross-chunk snapshot atomicity is not required here.
	for start := 0; start < len(unique); start += chunkSize {
		end := start + chunkSize
		if end > len(unique) {
			end = len(unique)
		}
		chunk := unique[start:end]
		placeholders := make([]string, len(chunk))
		args := make([]any, len(chunk))
		for i, id := range chunk {
			placeholders[i] = "?"
			args[i] = id
		}
		query := `SELECT ` + selectCols + ` FROM items WHERE id IN (` + strings.Join(placeholders, ",") + `)`
		if err := queryItemsInto(ctx, db, query, args, result); err != nil {
			return nil, fmt.Errorf("batch item lookup chunk %d:%d: %w", start, end, err)
		}
	}
	return result, nil
}

// rowQuerier is the read subset of *sql.DB / *sql.Tx used by queryItemsInto so it
// can run either directly on the pooled handle or inside a read transaction.
type rowQuerier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

// queryItemsInto runs a SELECT that returns artifact rows and scans each into
// dst keyed by ID. It isolates rows.Close handling so GetItemsByIDs stays flat.
func queryItemsInto(ctx context.Context, q rowQuerier, query string, args []any, dst map[string]*models.Artifact) error {
	rows, err := q.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("query items by ids: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		a, scanErr := scanArtifactRow(rows)
		if scanErr != nil {
			return fmt.Errorf("scan item row: %w", scanErr)
		}
		dst[a.ID] = a
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate items by ids: %w", err)
	}
	return nil
}

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
	return RetryWrite(ctx, func() error {
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
	})
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

// RewriteDependencyEdges updates all item_deps rows that reference oldID,
// changing them to reference newID. Both item_id and depends_on columns are
// rewritten. This function operates within an existing transaction.
func RewriteDependencyEdges(ctx context.Context, tx *sql.Tx, oldID, newID string) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE item_deps SET item_id = ? WHERE item_id = ?`, newID, oldID)
	if err != nil {
		return fmt.Errorf("rewrite dep edges item_id %s→%s: %w", oldID, newID, err)
	}
	_, err = tx.ExecContext(ctx,
		`UPDATE item_deps SET depends_on = ? WHERE depends_on = ?`, newID, oldID)
	if err != nil {
		return fmt.Errorf("rewrite dep edges depends_on %s→%s: %w", oldID, newID, err)
	}
	return nil
}

// RewriteLinkEdges updates all item_links rows that reference oldID,
// changing them to reference newID. Both source_id and target_id columns are
// rewritten. This function operates within an existing transaction.
func RewriteLinkEdges(ctx context.Context, tx *sql.Tx, oldID, newID string) error {
	_, err := tx.ExecContext(ctx,
		`UPDATE item_links SET source_id = ? WHERE source_id = ?`, newID, oldID)
	if err != nil {
		return fmt.Errorf("rewrite link edges source_id %s→%s: %w", oldID, newID, err)
	}
	_, err = tx.ExecContext(ctx,
		`UPDATE item_links SET target_id = ? WHERE target_id = ?`, newID, oldID)
	if err != nil {
		return fmt.Errorf("rewrite link edges target_id %s→%s: %w", oldID, newID, err)
	}
	return nil
}

// RewriteAncillaryReferences updates item_id references in commit_links,
// stash_links, item_logs, and item_log_entries. For item_logs and
// RewriteAncillaryReferences updates item_id columns in commit_links,
// stash_links, item_logs, and item_log_entries from oldID to newID. For
// item_logs and item_log_entries the log_path column is also set to
// newLogPath (a .backlogit/-relative path like "logs/<id>.jsonl").
// This function operates within an existing transaction.
func RewriteAncillaryReferences(ctx context.Context, tx *sql.Tx, oldID, newID, newLogPath string) error {
	// commit_links
	if _, err := tx.ExecContext(ctx,
		`UPDATE commit_links SET item_id = ? WHERE item_id = ?`, newID, oldID); err != nil {
		return fmt.Errorf("rewrite commit_links %s→%s: %w", oldID, newID, err)
	}
	// stash_links
	if _, err := tx.ExecContext(ctx,
		`UPDATE stash_links SET item_id = ? WHERE item_id = ?`, newID, oldID); err != nil {
		return fmt.Errorf("rewrite stash_links %s→%s: %w", oldID, newID, err)
	}
	// item_logs — UPDATE rewrites item_id (PK) and log_path in one statement.
	// SQLite allows PK updates; there is no conflict because the old row is
	// the only one being modified.
	if _, err := tx.ExecContext(ctx,
		`UPDATE item_logs SET item_id = ?, log_path = ? WHERE item_id = ?`, newID, newLogPath, oldID); err != nil {
		return fmt.Errorf("rewrite item_logs %s→%s: %w", oldID, newID, err)
	}
	// item_log_entries
	if _, err := tx.ExecContext(ctx,
		`UPDATE item_log_entries SET item_id = ?, log_path = ? WHERE item_id = ?`, newID, newLogPath, oldID); err != nil {
		return fmt.Errorf("rewrite item_log_entries %s→%s: %w", oldID, newID, err)
	}
	return nil
}

// DeleteItemTx removes an item from the items table within an existing
// transaction. Returns ErrNotFound when no row matched the ID.
func DeleteItemTx(ctx context.Context, tx *sql.Tx, id string) error {
	result, err := tx.ExecContext(ctx, `DELETE FROM items WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete item %s: %w", id, err)
	}
	rowsAffected, _ := result.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("delete item %s: %w", id, blErrors.ErrNotFound)
	}
	return nil
}

// UpsertItemTx inserts or replaces an artifact in the items table within an
// existing transaction. Mirrors UpsertItem's column set and value formatting
// (including hierarchy_path and RFC3339Nano timestamps) to ensure scan
// compatibility via scanArtifactRow.
func UpsertItemTx(ctx context.Context, tx *sql.Tx, artifact *models.Artifact) error {
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

	_, err = tx.ExecContext(ctx,
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
