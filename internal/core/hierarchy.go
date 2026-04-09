package core

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"

	"github.com/backlogit/backlogit/internal/config"
)

// QueueLayoutConfig is an alias for config.QueueLayoutConfig, kept for backward compatibility.
type QueueLayoutConfig = config.QueueLayoutConfig

// HierarchyLevel is an alias for config.HierarchyLevel, kept for backward compatibility.
type HierarchyLevel = config.HierarchyLevel

// ResolveHierarchicalPath determines the target file path for a new artifact
// within the hierarchical queue layout based on its parent ID and artifact type.
func ResolveHierarchicalPath(layout *config.QueueLayoutConfig, parentID string, artifactType string) (string, error) {
	if _, err := LevelForType(layout, artifactType); err != nil {
		return "", err
	}
	return layout.RootDir, nil
}

// NextHierarchicalID computes the next available ID at a given hierarchy level
// by querying the SQLite index for the maximum existing sibling ordinal.
// Root-level IDs must be purely numeric segments (e.g., "001", "002"); items
// with non-numeric IDs such as legacy prefix IDs ("T001", "BUG-3") are excluded
// from the ordinal computation.
func NextHierarchicalID(db *sql.DB, parentID string, layout *config.QueueLayoutConfig) (string, error) {
	var (
		rows *sql.Rows
		err  error
	)
	if parentID == "" {
		rows, err = db.Query(`SELECT id FROM items WHERE parent_id IS NULL`)
	} else {
		rows, err = db.Query(`SELECT id FROM items WHERE parent_id = ?`, parentID)
	}
	if err != nil {
		return "", fmt.Errorf("next hierarchical id: %w", err)
	}
	defer rows.Close()

	maxOrdinal := 0
	for rows.Next() {
		var existingID string
		if err := rows.Scan(&existingID); err != nil {
			return "", fmt.Errorf("scan hierarchical id: %w", err)
		}
		segment := lastIDSegment(existingID)
		numeric := leadingDigits(segment)
		if numeric == "" {
			continue
		}
		remainder := strings.TrimPrefix(segment, numeric)
		if remainder != "" && !strings.HasPrefix(remainder, "-") {
			continue
		}
		ordinal, err := strconv.Atoi(numeric)
		if err != nil {
			continue
		}
		if ordinal > maxOrdinal {
			maxOrdinal = ordinal
		}
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterate hierarchical ids: %w", err)
	}

	next := 1
	if maxOrdinal > 0 {
		next = maxOrdinal + 1
	}
	segment := FormatHierarchicalID(next, layout)
	if parentID == "" {
		return segment, nil
	}
	numParent, err := numericParentPath(parentID, layout)
	if err != nil {
		return "", fmt.Errorf("normalize parent id: %w", err)
	}
	return numParent + "." + segment, nil
}

// NextTypedHierarchicalID computes the next available typed hierarchical ID for
// the given artifact type and parent.
func NextTypedHierarchicalID(
	ctx context.Context,
	db *sql.DB,
	parentID string,
	artifactType string,
	typeCfg *config.ArtifactTypeConfig,
	layout *config.QueueLayoutConfig,
) (string, error) {
	if typeCfg == nil {
		return "", fmt.Errorf("artifact type config is required")
	}
	if _, err := LevelForType(layout, artifactType); err != nil {
		return "", err
	}

	var (
		rows *sql.Rows
		err  error
	)
	if parentID == "" {
		// Restrict to root-level IDs (no dot separator) so that child IDs like
		// "019.001-T" are not counted when computing the next root ordinal.
		// IDs are immutable after creation, so filtering by ID structure is
		// reliable even when parent_id changes via UpdateArtifact.
		rows, err = db.QueryContext(ctx,
			`SELECT id FROM items WHERE artifact_type = ? AND id NOT LIKE '%.%'`,
			artifactType,
		)
	} else {
		rows, err = db.QueryContext(ctx,
			`SELECT id FROM items WHERE parent_id = ? AND artifact_type = ?`,
			parentID, artifactType,
		)
	}
	if err != nil {
		return "", fmt.Errorf("query hierarchical ids: %w", err)
	}
	defer rows.Close()

	maxOrdinal := 0
	for rows.Next() {
		var existingID string
		if err := rows.Scan(&existingID); err != nil {
			return "", fmt.Errorf("scan hierarchical id: %w", err)
		}
		ordinal, ok := typedSegmentOrdinal(lastIDSegment(existingID), typeCfg.Prefix, typeCfg.Suffix)
		if ok && ordinal > maxOrdinal {
			maxOrdinal = ordinal
		}
	}
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterate hierarchical ids: %w", err)
	}

	segment := formatTypedSegment(typeCfg.Prefix, typeCfg.Suffix, maxOrdinal+1)
	if parentID == "" {
		return segment, nil
	}
	parentPath, err := numericParentPath(parentID, layout)
	if err != nil {
		return "", fmt.Errorf("parent numeric path: %w", err)
	}
	return parentPath + "." + segment, nil
}

// ParseHierarchicalID splits a hierarchical ID (e.g., "001.002.003") into its
// component level segments as integers.
func ParseHierarchicalID(id string) ([]int, error) {
	if id == "" {
		return nil, fmt.Errorf("hierarchical ID must not be empty")
	}
	parts := strings.Split(id, ".")
	result := make([]int, 0, len(parts))
	for _, p := range parts {
		if p == "" {
			return nil, fmt.Errorf("hierarchical ID has empty segment: %q", id)
		}
		numeric := leadingDigits(p)
		if numeric == "" {
			return nil, fmt.Errorf("hierarchical ID segment %q is not numeric in %q", p, id)
		}
		remainder := strings.TrimPrefix(p, numeric)
		if remainder != "" && !strings.HasPrefix(remainder, "-") {
			return nil, fmt.Errorf("hierarchical ID segment %q is not numeric in %q", p, id)
		}
		n, err := strconv.Atoi(numeric)
		if err != nil {
			return nil, fmt.Errorf("hierarchical ID segment %q is not numeric in %q", p, id)
		}
		result = append(result, n)
	}
	return result, nil
}

// LevelForType returns the hierarchy level number for the given artifact type
// based on the QueueLayoutConfig mapping.
func LevelForType(layout *config.QueueLayoutConfig, artifactType string) (int, error) {
	for _, lvl := range layout.Levels {
		for _, t := range lvl.Types {
			if t == artifactType {
				return lvl.Level, nil
			}
		}
	}
	return 0, fmt.Errorf("artifact type %q is not mapped to any hierarchy level", artifactType)
}

// FormatHierarchicalID formats a numeric segment with zero-padding to match
// the queue layout naming convention (e.g., 1 → "001").
func FormatHierarchicalID(segment int, layout *config.QueueLayoutConfig) string {
	return fmt.Sprintf("%03d", segment)
}

func formatTypedSegment(prefix string, suffix string, ordinal int) string {
	if suffix != "" {
		return fmt.Sprintf("%03d%s", ordinal, suffix)
	}
	return fmt.Sprintf("%s%03d", prefix, ordinal)
}

func lastIDSegment(id string) string {
	if idx := strings.LastIndex(id, "."); idx != -1 {
		return id[idx+1:]
	}
	return id
}

func leadingDigits(value string) string {
	var digits strings.Builder
	for _, r := range value {
		if r < '0' || r > '9' {
			break
		}
		digits.WriteRune(r)
	}
	return digits.String()
}

func typedSegmentOrdinal(segment, prefix string, suffix string) (int, bool) {
	numeric := ""
	switch {
	case suffix != "":
		if !strings.HasSuffix(segment, suffix) {
			return 0, false
		}
		numeric = strings.TrimSuffix(segment, suffix)
	case strings.HasPrefix(segment, prefix):
		numeric = strings.TrimPrefix(segment, prefix)
	default:
		return 0, false
	}
	if numeric == "" {
		return 0, false
	}
	ordinal, err := strconv.Atoi(numeric)
	if err != nil {
		return 0, false
	}
	return ordinal, true
}

func numericParentPath(parentID string, layout *config.QueueLayoutConfig) (string, error) {
	segments, err := ParseHierarchicalID(parentID)
	if err != nil {
		return "", fmt.Errorf("parse hierarchical parent %q: %w", parentID, err)
	}
	formatted := make([]string, 0, len(segments))
	for _, segment := range segments {
		formatted = append(formatted, FormatHierarchicalID(segment, layout))
	}
	return strings.Join(formatted, "."), nil
}
