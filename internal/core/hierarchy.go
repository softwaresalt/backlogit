package core

import (
	"database/sql"
)

// QueueLayoutConfig defines the hierarchical file organization for .backlogit/queue/.
type QueueLayoutConfig struct {
	RootDir    string           `yaml:"root_dir" validate:"required"`
	Levels     []HierarchyLevel `yaml:"levels" validate:"required,min=1,dive"`
	NameFormat string           `yaml:"name_format"`
}

// HierarchyLevel maps a hierarchy depth to one or more artifact types.
type HierarchyLevel struct {
	Level int      `yaml:"level" validate:"required,gte=1,lte=5"`
	Types []string `yaml:"types" validate:"required,min=1"`
}

// ResolveHierarchicalPath determines the target file path for a new artifact
// within the hierarchical queue layout based on its parent ID and artifact type.
//
// Worker: Look up the artifact type in QueueLayoutConfig levels to determine
// depth. Combine parent path prefix with the next sibling ordinal. Return the
// full relative path under .backlogit/queue/.
func ResolveHierarchicalPath(layout *QueueLayoutConfig, parentID string, artifactType string) (string, error) {
	panic("not implemented: Worker: Implement hierarchical path resolution using QueueLayoutConfig levels and parent ID prefix")
}

// NextHierarchicalID computes the next available ID at a given hierarchy level
// by querying the SQLite index for the maximum existing sibling ordinal.
//
// Worker: Execute SELECT MAX(CAST(suffix AS INTEGER)) FROM items WHERE parent_id = ?
// to find the next sibling ordinal. Return parentID + "." + zero-padded next ordinal.
func NextHierarchicalID(db *sql.DB, parentID string, layout *QueueLayoutConfig) (string, error) {
	panic("not implemented: Worker: Implement scoped counter query to find next sibling ordinal within parent scope")
}

// ParseHierarchicalID splits a hierarchical ID (e.g., "001.002.003") into its
// component level segments as integers.
//
// Worker: Split on "." delimiter, parse each segment as int, return []int.
// Return error for malformed IDs (empty segments, non-numeric values).
func ParseHierarchicalID(id string) ([]int, error) {
	panic("not implemented: Worker: Implement hierarchical ID parsing with dot-separated numeric segments")
}

// LevelForType returns the hierarchy level number for the given artifact type
// based on the QueueLayoutConfig mapping.
//
// Worker: Iterate QueueLayoutConfig.Levels to find which level contains the
// given artifact type. Return error if type is not mapped to any level.
func LevelForType(layout *QueueLayoutConfig, artifactType string) (int, error) {
	panic("not implemented: Worker: Look up artifact type in QueueLayoutConfig levels and return the matching level number")
}

// FormatHierarchicalID formats a numeric segment with zero-padding to match
// the queue layout naming convention (e.g., 1 → "001").
//
// Worker: Apply the layout's NameFormat (default "{NNN}" = 3-digit zero-pad).
func FormatHierarchicalID(segment int, layout *QueueLayoutConfig) string {
	panic("not implemented: Worker: Format numeric segment with zero-padding per NameFormat")
}
