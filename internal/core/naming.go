package core

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"

	"github.com/backlogit/backlogit/internal/config"
)

var nonAlphanumeric = regexp.MustCompile(`[^a-z0-9]+`)

// Slugify converts a title to a lowercase kebab-case slug.
func Slugify(title string, maxLen int) string {
	s := strings.ToLower(title)
	s = nonAlphanumeric.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if maxLen > 0 && len(s) > maxLen {
		s = s[:maxLen]
		s = strings.TrimRight(s, "-")
	}
	return s
}

// ResolveName generates an artifact filename from the naming template.
func ResolveName(cfg *config.ArtifactTypeConfig, title string, nextID int, maxSlugLen int) string {
	name := cfg.NameFormat
	name = strings.ReplaceAll(name, "{prefix}", cfg.Prefix)
	name = strings.ReplaceAll(name, "{NNN}", fmt.Sprintf("%03d", nextID))
	name = strings.ReplaceAll(name, "{title_slug}", Slugify(title, maxSlugLen))
	name = strings.ReplaceAll(name, "{slug}", Slugify(title, maxSlugLen))
	return name
}

// NextID queries for the next sequential artifact ID.
func NextID(ctx context.Context, db *sql.DB, artifactType string) (int, error) {
	var count int
	err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM items WHERE artifact_type = ?", artifactType).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("next id query: %w", err)
	}
	return count + 1, nil
}
