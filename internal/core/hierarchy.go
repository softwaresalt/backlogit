package core

import (
	"database/sql"
	"fmt"
	"path/filepath"
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
	if parentID == "" {
		return layout.RootDir, nil
	}
	return filepath.Join(layout.RootDir, parentID), nil
}

// NextHierarchicalID computes the next available ID at a given hierarchy level
// by querying the SQLite index for the maximum existing sibling ordinal.
func NextHierarchicalID(db *sql.DB, parentID string, layout *config.QueueLayoutConfig) (string, error) {
	var maxOrdinal sql.NullInt64
	var err error
	if parentID == "" {
		err = db.QueryRow(`SELECT MAX(CAST(id AS INTEGER)) FROM items WHERE parent_id IS NULL`).Scan(&maxOrdinal)
	} else {
		err = db.QueryRow(
			`SELECT MAX(CAST(SUBSTR(id, LENGTH(?)+2) AS INTEGER)) FROM items WHERE parent_id = ?`,
			parentID, parentID,
		).Scan(&maxOrdinal)
	}
	if err != nil && err != sql.ErrNoRows {
		return "", fmt.Errorf("next hierarchical id: %w", err)
	}

	next := 1
	if maxOrdinal.Valid {
		next = int(maxOrdinal.Int64) + 1
	}
	segment := FormatHierarchicalID(next, layout)
	if parentID == "" {
		return segment, nil
	}
	return parentID + "." + segment, nil
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
		n, err := strconv.Atoi(p)
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
