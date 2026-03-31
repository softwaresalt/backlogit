package db

import (
	"context"
	"database/sql"
)

// DependencyEdge represents a single dependency relationship between two artifacts.
type DependencyEdge struct {
	ItemID    string `json:"item_id"`
	DependsOn string `json:"depends_on"`
	DepType   string `json:"dep_type"`
}

// UpsertDependency creates or updates a dependency edge in the item_deps table.
//
// Worker: INSERT OR REPLACE INTO item_deps (item_id, depends_on, dep_type)
// VALUES (?, ?, ?). Validate both IDs exist in items table first.
func UpsertDependency(ctx context.Context, db *sql.DB, itemID, dependsOn, depType string) error {
	panic("not implemented: Worker: Insert dependency edge into item_deps junction table with existence validation")
}

// DeleteDependency removes a specific dependency edge.
//
// Worker: DELETE FROM item_deps WHERE item_id = ? AND depends_on = ?.
func DeleteDependency(ctx context.Context, db *sql.DB, itemID, dependsOn string) error {
	panic("not implemented: Worker: Delete specific dependency edge from item_deps table")
}

// GetDependencies returns all items that the given item depends on (upstream edges).
//
// Worker: SELECT * FROM item_deps WHERE item_id = ? to find what this item needs.
func GetDependencies(ctx context.Context, db *sql.DB, itemID string) ([]DependencyEdge, error) {
	panic("not implemented: Worker: Query upstream dependency edges for the given item")
}

// GetDependents returns all items that depend on the given item (downstream/reverse edges).
//
// Worker: SELECT * FROM item_deps WHERE depends_on = ? to find what blocks on this item.
func GetDependents(ctx context.Context, db *sql.DB, dependsOn string) ([]DependencyEdge, error) {
	panic("not implemented: Worker: Query downstream dependency edges (who depends on this item)")
}

// DetectCycle checks whether adding a dependency from itemID to dependsOn
// would create a circular dependency in the graph.
//
// Worker: Perform DFS/BFS from dependsOn following depends_on edges. If itemID
// is reachable, a cycle exists. Return true if cycle detected.
func DetectCycle(ctx context.Context, db *sql.DB, itemID, dependsOn string) (bool, error) {
	panic("not implemented: Worker: Detect circular dependencies using graph traversal from dependsOn to itemID")
}
