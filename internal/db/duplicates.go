package db

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// DuplicateGroup holds a set of artifact IDs that share the same normalized title.
type DuplicateGroup struct {
	NormalizedTitle string   `json:"normalized_title"`
	IDs             []string `json:"ids"`
	Count           int      `json:"count"`
}

// FindDuplicates returns groups of artifacts that share identical normalized titles.
// Normalization lowercases the title and trims surrounding whitespace before comparison.
// Groups with only one member are excluded from the result.
func FindDuplicates(ctx context.Context, db *sql.DB) ([]DuplicateGroup, error) {
	const query = `
		SELECT lower(trim(title)) AS norm_title,
		       group_concat(id)   AS ids,
		       count(*)           AS cnt
		FROM   items
		GROUP  BY norm_title
		HAVING cnt > 1
		ORDER  BY norm_title
	`
	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("find duplicates: %w", err)
	}
	defer rows.Close()

	var groups []DuplicateGroup
	for rows.Next() {
		var normTitle, idsCsv string
		var cnt int
		if err := rows.Scan(&normTitle, &idsCsv, &cnt); err != nil {
			return nil, fmt.Errorf("find duplicates scan: %w", err)
		}
		groups = append(groups, DuplicateGroup{
			NormalizedTitle: normTitle,
			IDs:             strings.Split(idsCsv, ","),
			Count:           cnt,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("find duplicates rows: %w", err)
	}
	return groups, nil
}
