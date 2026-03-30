package db

import (
	"database/sql"
	"fmt"
)

// EnsureSchema creates the items table, indexes, FTS5 virtual table, and
// maintenance triggers idempotently, wrapped in a single transaction.
func EnsureSchema(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin schema transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	statements := []string{
		`CREATE TABLE IF NOT EXISTS items (
			id           TEXT PRIMARY KEY,
			title        TEXT NOT NULL,
			status       TEXT NOT NULL,
			artifact_type TEXT NOT NULL,
			parent_id    TEXT,
			sprint       TEXT,
			priority     TEXT,
			description  TEXT,
			custom_fields TEXT,
			created_at   DATETIME NOT NULL,
			updated_at   DATETIME NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_items_status ON items(status)`,
		`CREATE INDEX IF NOT EXISTS idx_items_type   ON items(artifact_type)`,
		`CREATE INDEX IF NOT EXISTS idx_items_parent ON items(parent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_items_sprint ON items(sprint)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS items_fts USING fts5(
			id UNINDEXED,
			title,
			description,
			content='items',
			content_rowid='rowid'
		)`,
		// Triggers keep the FTS content table in sync with the base items table.
		// INSERT OR REPLACE issues a DELETE then INSERT at the storage layer, so
		// the delete trigger fires before the insert trigger as expected.
		`CREATE TRIGGER IF NOT EXISTS items_ai AFTER INSERT ON items BEGIN
			INSERT INTO items_fts(rowid, id, title, description)
			VALUES (new.rowid, new.id, new.title, new.description);
		END`,
		`CREATE TRIGGER IF NOT EXISTS items_ad AFTER DELETE ON items BEGIN
			INSERT INTO items_fts(items_fts, rowid, id, title, description)
			VALUES ('delete', old.rowid, old.id, old.title, old.description);
		END`,
		`CREATE TRIGGER IF NOT EXISTS items_au AFTER UPDATE ON items BEGIN
			INSERT INTO items_fts(items_fts, rowid, id, title, description)
			VALUES ('delete', old.rowid, old.id, old.title, old.description);
			INSERT INTO items_fts(rowid, id, title, description)
			VALUES (new.rowid, new.id, new.title, new.description);
		END`,
	}

	for _, stmt := range statements {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("execute schema statement: %w", err)
		}
	}

	return tx.Commit()
}
