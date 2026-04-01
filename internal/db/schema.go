package db

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/backlogit/backlogit/internal/config"
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
		`CREATE TABLE IF NOT EXISTS item_deps (
			item_id    TEXT NOT NULL,
			depends_on TEXT NOT NULL,
			dep_type   TEXT NOT NULL DEFAULT 'blocks',
			PRIMARY KEY (item_id, depends_on)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_item_deps_item ON item_deps(item_id)`,
		`CREATE INDEX IF NOT EXISTS idx_item_deps_dep  ON item_deps(depends_on)`,
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
			updated_at   DATETIME NOT NULL,
			assigned_to  TEXT,
			owner        TEXT,
			labels       TEXT,
			dependencies TEXT,
			"references" TEXT,
			"commit"     TEXT,
			level          INTEGER,
			hierarchy_path TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_items_status ON items(status)`,
		`CREATE INDEX IF NOT EXISTS idx_items_type   ON items(artifact_type)`,
		`CREATE INDEX IF NOT EXISTS idx_items_parent ON items(parent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_items_sprint ON items(sprint)`,
		`CREATE INDEX IF NOT EXISTS idx_items_hierarchy ON items(hierarchy_path)`,
		`CREATE TABLE IF NOT EXISTS commit_links (
			item_id    TEXT NOT NULL,
			commit_sha TEXT NOT NULL,
			message    TEXT NOT NULL DEFAULT '',
			author     TEXT NOT NULL DEFAULT '',
			PRIMARY KEY (item_id, commit_sha)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_commit_links_item ON commit_links(item_id)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS items_fts USING fts5(
			id UNINDEXED,
			title,
			description,
			labels,
			content='items',
			content_rowid='rowid'
		)`,
		// Triggers keep the FTS content table in sync with the base items table.
		// INSERT OR REPLACE issues a DELETE then INSERT at the storage layer, so
		// the delete trigger fires before the insert trigger as expected.
		`CREATE TRIGGER IF NOT EXISTS items_ai AFTER INSERT ON items BEGIN
			INSERT INTO items_fts(rowid, id, title, description, labels)
			VALUES (new.rowid, new.id, new.title, new.description, new.labels);
		END`,
		`CREATE TRIGGER IF NOT EXISTS items_ad AFTER DELETE ON items BEGIN
			INSERT INTO items_fts(items_fts, rowid, id, title, description, labels)
			VALUES ('delete', old.rowid, old.id, old.title, old.description, old.labels);
		END`,
		`CREATE TRIGGER IF NOT EXISTS items_au AFTER UPDATE ON items BEGIN
			INSERT INTO items_fts(items_fts, rowid, id, title, description, labels)
			VALUES ('delete', old.rowid, old.id, old.title, old.description, old.labels);
			INSERT INTO items_fts(rowid, id, title, description, labels)
			VALUES (new.rowid, new.id, new.title, new.description, new.labels);
		END`,
	}

	for _, stmt := range statements {
		if _, err := tx.Exec(stmt); err != nil {
			return fmt.Errorf("execute schema statement: %w", err)
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	// Migrate existing databases: add columns introduced in the queue features release.
	// ALTER TABLE ADD COLUMN does not support IF NOT EXISTS in SQLite, so we attempt
	// each migration and ignore "duplicate column name" errors.
	migrations := []string{
		`ALTER TABLE items ADD COLUMN level INTEGER`,
		`ALTER TABLE items ADD COLUMN hierarchy_path TEXT`,
	}
	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			if !isDuplicateColumnError(err) {
				return fmt.Errorf("apply migration %q: %w", m, err)
			}
		}
	}
	return nil
}

// isDuplicateColumnError reports whether an SQLite error indicates an attempt
// to add a column that already exists.
func isDuplicateColumnError(err error) bool {
	if err == nil {
		return false
	}
	return strings.Contains(err.Error(), "duplicate column name")
}

// EnsureSchemaWithExtensions creates base schema and applies dynamic columns from header-def.
func EnsureSchemaWithExtensions(db *sql.DB, headerDef *config.HeaderDefConfig) error {
	if err := EnsureSchema(db); err != nil {
		return err
	}
	if headerDef != nil {
		return ApplySchemaExtensions(db, headerDef)
	}
	return nil
}
