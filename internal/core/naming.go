package core

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/softwaresalt/backlogit/internal/config"
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
	name = strings.ReplaceAll(name, "{suffix}", cfg.Suffix)
	name = strings.ReplaceAll(name, "{NNN}", fmt.Sprintf("%03d", nextID))
	name = strings.ReplaceAll(name, "{title_slug}", Slugify(title, maxSlugLen))
	name = strings.ReplaceAll(name, "{slug}", Slugify(title, maxSlugLen))
	return name
}

// ResolveFileName generates the on-disk filename stem for an artifact.
// When no file_name_format is configured, the stable artifact ID is used.
func ResolveFileName(cfg *config.ArtifactTypeConfig, artifactID string, title string, maxSlugLen int) string {
	if cfg == nil || cfg.FileNameFormat == "" {
		return artifactID
	}

	name := cfg.FileNameFormat
	name = strings.ReplaceAll(name, "{id}", artifactID)
	name = strings.ReplaceAll(name, "{prefix}", cfg.Prefix)
	name = strings.ReplaceAll(name, "{title_slug}", Slugify(title, maxSlugLen))
	name = strings.ReplaceAll(name, "{slug}", Slugify(title, maxSlugLen))
	return name
}

// NextID queries for the next sequential artifact ID.
func NextID(ctx context.Context, db *sql.DB, artifactType string, typeCfg *config.ArtifactTypeConfig) (int, error) {
	rows, err := db.QueryContext(ctx, "SELECT id FROM items WHERE artifact_type = ?", artifactType)
	if err != nil {
		return 0, fmt.Errorf("next id query: %w", err)
	}
	defer rows.Close()

	maxOrdinal := 0
	for rows.Next() {
		var existingID string
		if err := rows.Scan(&existingID); err != nil {
			return 0, fmt.Errorf("scan next id row: %w", err)
		}
		ordinal, ok := standaloneOrdinal(existingID, typeCfg)
		if ok && ordinal > maxOrdinal {
			maxOrdinal = ordinal
		}
	}
	if err := rows.Err(); err != nil {
		return 0, fmt.Errorf("iterate next id rows: %w", err)
	}
	return maxOrdinal + 1, nil
}

func standaloneOrdinal(artifactID string, typeCfg *config.ArtifactTypeConfig) (int, bool) {
	segment := artifactID
	if idx := strings.LastIndex(segment, "."); idx >= 0 {
		segment = segment[idx+1:]
	}
	if typeCfg != nil {
		if ordinal, ok := typedSegmentOrdinal(segment, typeCfg.Prefix, typeCfg.Suffix); ok {
			return ordinal, true
		}
	}
	numeric := leadingDigits(segment)
	if numeric == "" {
		return 0, false
	}
	ordinal, err := strconv.Atoi(numeric)
	if err != nil {
		return 0, false
	}
	return ordinal, true
}
