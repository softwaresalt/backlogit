package core

import (
	"context"
	"database/sql"

	"github.com/backlogit/backlogit/internal/config"
)

// ResolveName generates an artifact filename from the naming template.
//
// Worker: Implement name format template resolution.
func ResolveName(cfg *config.ArtifactTypeConfig, title string, nextID int, maxSlugLen int) string {
	panic("not implemented: Worker: Implement name format template resolution")
}

// Slugify converts a title to a lowercase kebab-case slug.
//
// Worker: Implement title slugification with truncation.
func Slugify(title string, maxLen int) string {
	panic("not implemented: Worker: Implement title slugification")
}

// NextID queries for the next sequential artifact ID.
//
// Worker: Implement sequential ID generation from SQLite or filesystem.
func NextID(ctx context.Context, db *sql.DB, artifactType string) (int, error) {
	panic("not implemented: Worker: Implement next ID generation")
}
