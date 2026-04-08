package db

import (
	"context"
	"database/sql"
	"fmt"
)

// DuplicateGroup holds a set of artifact IDs that share the same normalized title.
type DuplicateGroup struct {
	NormalizedTitle string   `json:"normalized_title"`
	IDs             []string `json:"ids"`
	Count           int      `json:"count"`
}

// FindDuplicates returns groups of artifacts that share identical normalized titles.
// Normalization lowercases the title and collapses whitespace before comparison.
// Groups with only one member are excluded from the result.
func FindDuplicates(ctx context.Context, db *sql.DB) ([]DuplicateGroup, error) {
	return nil, fmt.Errorf("not implemented: FindDuplicates")
}
