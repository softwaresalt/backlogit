package db

import (
	"context"
	"database/sql"
	"fmt"
)

// DependencyEdge represents a single dependency relationship between two artifacts.
type DependencyEdge struct {
	ItemID    string `json:"item_id"`
	DependsOn string `json:"depends_on"`
	DepType   string `json:"dep_type"`
}

// UpsertDependency creates or updates a dependency edge in the item_deps table.
// Both referenced items must exist in the items table.
func UpsertDependency(ctx context.Context, db *sql.DB, itemID, dependsOn, depType string) error {
	var count int
	row := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM items WHERE id IN (?, ?)`, itemID, dependsOn)
	if err := row.Scan(&count); err != nil {
		return fmt.Errorf("validate items: %w", err)
	}
	if count < 2 {
		return fmt.Errorf("one or both items not found: %s, %s", itemID, dependsOn)
	}
	_, err := db.ExecContext(ctx,
		`INSERT OR REPLACE INTO item_deps (item_id, depends_on, dep_type) VALUES (?, ?, ?)`,
		itemID, dependsOn, depType,
	)
	if err != nil {
		return fmt.Errorf("upsert dependency %s→%s: %w", itemID, dependsOn, err)
	}
	return nil
}

// DeleteDependency removes a specific dependency edge.
func DeleteDependency(ctx context.Context, db *sql.DB, itemID, dependsOn string) error {
	_, err := db.ExecContext(ctx,
		`DELETE FROM item_deps WHERE item_id = ? AND depends_on = ?`,
		itemID, dependsOn,
	)
	if err != nil {
		return fmt.Errorf("delete dependency %s→%s: %w", itemID, dependsOn, err)
	}
	return nil
}

// GetDependencies returns all items that the given item depends on (upstream edges).
func GetDependencies(ctx context.Context, db *sql.DB, itemID string) ([]DependencyEdge, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT item_id, depends_on, dep_type FROM item_deps WHERE item_id = ?`, itemID)
	if err != nil {
		return nil, fmt.Errorf("get dependencies for %s: %w", itemID, err)
	}
	defer rows.Close()
	return scanEdges(rows)
}

// GetDependents returns all items that depend on the given item (downstream/reverse edges).
func GetDependents(ctx context.Context, db *sql.DB, dependsOn string) ([]DependencyEdge, error) {
	rows, err := db.QueryContext(ctx,
		`SELECT item_id, depends_on, dep_type FROM item_deps WHERE depends_on = ?`, dependsOn)
	if err != nil {
		return nil, fmt.Errorf("get dependents of %s: %w", dependsOn, err)
	}
	defer rows.Close()
	return scanEdges(rows)
}

// DetectCycle checks whether adding a dependency from itemID to dependsOn
// would create a circular dependency in the graph. It performs a BFS from
// dependsOn following existing dependency edges. If itemID is reachable, a cycle exists.
func DetectCycle(ctx context.Context, db *sql.DB, itemID, dependsOn string) (bool, error) {
	visited := make(map[string]bool)
	queue := []string{dependsOn}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current == itemID {
			return true, nil
		}
		if visited[current] {
			continue
		}
		visited[current] = true
		deps, err := GetDependencies(ctx, db, current)
		if err != nil {
			return false, fmt.Errorf("detect cycle at %s: %w", current, err)
		}
		for _, d := range deps {
			queue = append(queue, d.DependsOn)
		}
	}
	return false, nil
}

// AddDependencyChecked atomically checks for cycles and inserts the dependency
// in a single serializable transaction, preventing TOCTOU races under concurrent access.
func AddDependencyChecked(ctx context.Context, database *sql.DB, itemID, dependsOn, depType string) error {
	tx, err := database.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return fmt.Errorf("begin add dependency transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	hasCycle, err := detectCycleTx(ctx, tx, itemID, dependsOn)
	if err != nil {
		return fmt.Errorf("detect cycle: %w", err)
	}
	if hasCycle {
		return fmt.Errorf("adding %s→%s would create a circular dependency", itemID, dependsOn)
	}
	_, err = tx.ExecContext(ctx,
		`INSERT OR REPLACE INTO item_deps (item_id, depends_on, dep_type) VALUES (?, ?, ?)`,
		itemID, dependsOn, depType,
	)
	if err != nil {
		return fmt.Errorf("upsert dependency %s→%s: %w", itemID, dependsOn, err)
	}
	return tx.Commit()
}

// detectCycleTx performs a BFS cycle check within an existing transaction.
func detectCycleTx(ctx context.Context, tx *sql.Tx, itemID, dependsOn string) (bool, error) {
	visited := make(map[string]bool)
	queue := []string{dependsOn}
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		if current == itemID {
			return true, nil
		}
		if visited[current] {
			continue
		}
		visited[current] = true
		rows, err := tx.QueryContext(ctx, `SELECT depends_on FROM item_deps WHERE item_id = ?`, current)
		if err != nil {
			return false, fmt.Errorf("detect cycle at %s: %w", current, err)
		}
		var deps []string
		for rows.Next() {
			var dep string
			if err := rows.Scan(&dep); err != nil {
				rows.Close()
				return false, fmt.Errorf("scan dep at %s: %w", current, err)
			}
			deps = append(deps, dep)
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return false, err
		}
		queue = append(queue, deps...)
	}
	return false, nil
}

func scanEdges(rows *sql.Rows) ([]DependencyEdge, error) {
	var edges []DependencyEdge
	for rows.Next() {
		var e DependencyEdge
		if err := rows.Scan(&e.ItemID, &e.DependsOn, &e.DepType); err != nil {
			return nil, fmt.Errorf("scan dependency edge: %w", err)
		}
		edges = append(edges, e)
	}
	return edges, rows.Err()
}
