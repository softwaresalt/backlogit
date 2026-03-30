package db

import (
	"context"
	"database/sql"

	"github.com/backlogit/backlogit/internal/models"
)

// QueryFilters holds optional filters for item queries.
type QueryFilters struct {
	Status   string
	Type     string
	ParentID string
	Sprint   string
}

// UpsertItem inserts or replaces an artifact in the index.
//
// Worker: Implement INSERT OR REPLACE with all artifact fields.
func UpsertItem(ctx context.Context, db *sql.DB, artifact *models.Artifact) error {
	panic("not implemented: Worker: Implement item upsert")
}

// GetItem retrieves a single artifact by ID.
//
// Worker: Implement SELECT by primary key with custom_fields deserialization.
func GetItem(ctx context.Context, db *sql.DB, id string) (*models.Artifact, error) {
	panic("not implemented: Worker: Implement item retrieval by ID")
}

// DeleteItem removes an artifact from the index.
//
// Worker: Implement DELETE by primary key.
func DeleteItem(ctx context.Context, db *sql.DB, id string) error {
	panic("not implemented: Worker: Implement item deletion")
}

// QueryItems retrieves artifacts matching the provided filters.
//
// Worker: Implement parameterized SELECT with optional filters.
func QueryItems(ctx context.Context, db *sql.DB, filters QueryFilters) ([]*models.Artifact, error) {
	panic("not implemented: Worker: Implement filtered item query")
}

// SearchItems performs FTS5 full-text search across titles and descriptions.
//
// Worker: Implement FTS5 MATCH query.
func SearchItems(ctx context.Context, db *sql.DB, query string, limit int) ([]*models.Artifact, error) {
	panic("not implemented: Worker: Implement FTS5 search")
}
