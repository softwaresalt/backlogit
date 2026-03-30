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
	Status     string
	Type       string
	ParentID   string
	Sprint     string
	AssignedTo string
	Owner      string
}

const selectCols = `id, title, status, artifact_type, parent_id, sprint, priority, description, custom_fields, created_at, updated_at`

// rowScanner abstracts *sql.Row and *sql.Rows for the shared scan helper.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanArtifactRow scans a single row into an Artifact.
func scanArtifactRow(row rowScanner) (*models.Artifact, error) {
	var a models.Artifact
	var status, createdAt, updatedAt string
	var parentID, sprint, priority, description, customFields sql.NullString

	if err := row.Scan(
		&a.ID, &a.Title, &status, &a.ArtifactType,
		&parentID, &sprint, &priority, &description,
		&customFields, &createdAt, &updatedAt,
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

	a.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	a.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)

	return &a, nil
}

// UpsertItem inserts or replaces an artifact in the index.
func UpsertItem(ctx context.Context, db *sql.DB, artifact *models.Artifact) error {
	cf, err := json.Marshal(artifact.CustomFields)
	if err != nil {
		return fmt.Errorf("marshal custom fields: %w", err)
	}

	_, err = db.ExecContext(ctx,
		`INSERT OR REPLACE INTO items
			(id, title, status, artifact_type, parent_id, sprint, priority, description, custom_fields, created_at, updated_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		artifact.ID,
		artifact.Title,
		string(artifact.Status),
		artifact.ArtifactType,
		nullString(artifact.ParentID),
		nullString(artifact.Sprint),
		nullString(artifact.Priority),
		nullString(artifact.Description),
		string(cf),
		artifact.CreatedAt.Format(time.RFC3339),
		artifact.UpdatedAt.Format(time.RFC3339),
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
func DeleteItem(ctx context.Context, db *sql.DB, id string) error {
	_, err := db.ExecContext(ctx, `DELETE FROM items WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete item %s: %w", id, err)
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
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("query items: %w", err)
	}
	defer rows.Close()

	return scanArtifacts(rows)
}

// SearchItems performs FTS5 full-text search across titles and descriptions.
func SearchItems(ctx context.Context, db *sql.DB, query string, limit int) ([]*models.Artifact, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT i.`+strings.ReplaceAll(selectCols, ", ", ", i.")+
			` FROM items i
			  JOIN items_fts fts ON i.rowid = fts.rowid
			 WHERE items_fts MATCH ?
			 LIMIT ?`,
		query, limit,
	)
	if err != nil {
		return nil, fmt.Errorf("search items: %w", err)
	}
	defer rows.Close()

	return scanArtifacts(rows)
}

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
