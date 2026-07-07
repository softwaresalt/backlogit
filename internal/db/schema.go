package db

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"sort"
	"strings"

	"github.com/softwaresalt/backlogit/internal/config"
)

// ColumnSchema describes a single column in a SQLite table.
type ColumnSchema struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	NotNull    bool   `json:"not_null"`
	PrimaryKey bool   `json:"primary_key"`
}

// IndexSchema describes a single index on a SQLite table.
type IndexSchema struct {
	Name    string   `json:"name"`
	Unique  bool     `json:"unique"`
	Columns []string `json:"columns"`
}

// TableSchema describes the schema of a single SQLite table, including
// its columns, indexes, and whether it is an FTS5 virtual table.
type TableSchema struct {
	Name      string         `json:"name"`
	IsVirtual bool           `json:"is_virtual"`
	Columns   []ColumnSchema `json:"columns"`
	Indexes   []IndexSchema  `json:"indexes,omitempty"`
}

// systemTables lists SQLite internal tables that should be excluded from
// schema introspection results.
var systemTables = map[string]bool{
	"sqlite_sequence": true,
}

// IntrospectSchema returns the schema of all user-defined tables in the
// database, including columns, indexes, and FTS5 virtual table detection.
// Results are sorted by table name for deterministic output.
func IntrospectSchema(ctx context.Context, db *sql.DB) ([]TableSchema, error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, fmt.Errorf("begin schema introspection transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	tables, err := listTables(ctx, tx)
	if err != nil {
		return nil, err
	}

	slog.Debug("introspect schema: found tables", "count", len(tables))

	var result []TableSchema
	for _, t := range tables {
		ts, err := introspectTable(ctx, tx, t.name, t.isVirtual)
		if err != nil {
			return nil, fmt.Errorf("introspect table %q: %w", t.name, err)
		}
		result = append(result, ts)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})
	return result, nil
}

type tableEntry struct {
	name      string
	isVirtual bool
}

// listTables enumerates user-defined tables from sqlite_master, identifying
// FTS5 virtual tables by their CREATE VIRTUAL TABLE statement.
func listTables(ctx context.Context, tx *sql.Tx) ([]tableEntry, error) {
	rows, err := tx.QueryContext(ctx,
		`SELECT name, type, sql FROM sqlite_master WHERE type IN ('table') ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("query sqlite_master: %w", err)
	}
	defer rows.Close()

	var entries []tableEntry
	for rows.Next() {
		var name, ttype string
		var sqlText sql.NullString
		if err := rows.Scan(&name, &ttype, &sqlText); err != nil {
			return nil, fmt.Errorf("scan sqlite_master: %w", err)
		}
		if systemTables[name] {
			continue
		}
		// FTS shadow tables (e.g. items_fts_content, items_fts_data) should be excluded.
		// They are implementation details of FTS5 virtual tables.
		if isFTSShadowTable(name) {
			continue
		}
		isVirtual := sqlText.Valid && strings.Contains(strings.ToUpper(sqlText.String), "VIRTUAL TABLE")
		entries = append(entries, tableEntry{name: name, isVirtual: isVirtual})
	}
	return entries, rows.Err()
}

// isFTSShadowTable returns true for FTS5 internal shadow tables.
// FTS5 creates shadow tables with suffixes: _content, _data, _config,
// _docsize, _idx.
func isFTSShadowTable(name string) bool {
	suffixes := []string{"_content", "_data", "_config", "_docsize", "_idx"}
	for _, suffix := range suffixes {
		if strings.HasSuffix(name, suffix) {
			// Confirm the prefix matches a known FTS table naming pattern.
			prefix := strings.TrimSuffix(name, suffix)
			if strings.HasSuffix(prefix, "_fts") {
				return true
			}
		}
	}
	return false
}

// introspectTable gathers column and index metadata for a single table.
func introspectTable(ctx context.Context, tx *sql.Tx, name string, isVirtual bool) (TableSchema, error) {
	if !columnNameRe.MatchString(name) {
		return TableSchema{}, fmt.Errorf("invalid table name %q", name)
	}

	ts := TableSchema{
		Name:      name,
		IsVirtual: isVirtual,
	}

	// Columns via PRAGMA table_info.
	cols, err := introspectColumns(ctx, tx, name)
	if err != nil {
		return ts, err
	}
	ts.Columns = cols

	// Indexes via PRAGMA index_list + index_info (skip for virtual tables).
	if !isVirtual {
		idxs, err := introspectIndexes(ctx, tx, name)
		if err != nil {
			return ts, err
		}
		ts.Indexes = idxs
	}

	return ts, nil
}

// introspectColumns queries PRAGMA table_info for column metadata.
func introspectColumns(ctx context.Context, tx *sql.Tx, tableName string) ([]ColumnSchema, error) {
	//nolint:gosec // tableName validated by columnNameRe above.
	rows, err := tx.QueryContext(ctx, fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return nil, fmt.Errorf("pragma table_info(%s): %w", tableName, err)
	}
	defer rows.Close()

	var cols []ColumnSchema
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return nil, fmt.Errorf("scan table_info: %w", err)
		}
		cols = append(cols, ColumnSchema{
			Name:       name,
			Type:       colType,
			NotNull:    notNull == 1,
			PrimaryKey: pk > 0,
		})
	}
	return cols, rows.Err()
}

// introspectIndexes queries PRAGMA index_list and index_info for index metadata.
func introspectIndexes(ctx context.Context, tx *sql.Tx, tableName string) ([]IndexSchema, error) {
	//nolint:gosec // tableName validated by columnNameRe above.
	rows, err := tx.QueryContext(ctx, fmt.Sprintf("PRAGMA index_list(%s)", tableName))
	if err != nil {
		return nil, fmt.Errorf("pragma index_list(%s): %w", tableName, err)
	}
	defer rows.Close()

	var indexes []IndexSchema
	for rows.Next() {
		var seq int
		var name, origin string
		var unique, partial int
		if err := rows.Scan(&seq, &name, &unique, &origin, &partial); err != nil {
			return nil, fmt.Errorf("scan index_list: %w", err)
		}
		// Skip auto-generated indexes for PRIMARY KEY constraints.
		if strings.HasPrefix(name, "sqlite_autoindex_") {
			continue
		}
		cols, err := introspectIndexColumns(ctx, tx, name)
		if err != nil {
			return nil, err
		}
		indexes = append(indexes, IndexSchema{
			Name:    name,
			Unique:  unique == 1,
			Columns: cols,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	sort.Slice(indexes, func(i, j int) bool {
		return indexes[i].Name < indexes[j].Name
	})
	return indexes, nil
}

// introspectIndexColumns queries PRAGMA index_info for the columns in an index.
func introspectIndexColumns(ctx context.Context, tx *sql.Tx, indexName string) ([]string, error) {
	if !columnNameRe.MatchString(indexName) {
		return nil, fmt.Errorf("invalid index name %q", indexName)
	}
	//nolint:gosec // indexName validated above.
	rows, err := tx.QueryContext(ctx, fmt.Sprintf("PRAGMA index_info(%s)", indexName))
	if err != nil {
		return nil, fmt.Errorf("pragma index_info(%s): %w", indexName, err)
	}
	defer rows.Close()

	var cols []string
	for rows.Next() {
		var seqno, cid int
		var name string
		if err := rows.Scan(&seqno, &cid, &name); err != nil {
			return nil, fmt.Errorf("scan index_info: %w", err)
		}
		cols = append(cols, name)
	}
	return cols, rows.Err()
}

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
		`CREATE TABLE IF NOT EXISTS stash_entries (
			stash_id    TEXT PRIMARY KEY,
			priority    TEXT NOT NULL,
			kind        TEXT NOT NULL,
			text        TEXT NOT NULL,
			deliberation_id TEXT,
			state       TEXT NOT NULL,
			source_path TEXT NOT NULL,
			updated_at  DATETIME NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_stash_entries_state ON stash_entries(state)`,
		`CREATE TABLE IF NOT EXISTS item_links (
			source_id  TEXT NOT NULL,
			target_id  TEXT NOT NULL,
			link_type  TEXT NOT NULL,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY (source_id, target_id, link_type)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_item_links_source ON item_links(source_id)`,
		`CREATE INDEX IF NOT EXISTS idx_item_links_target ON item_links(target_id)`,
		`CREATE INDEX IF NOT EXISTS idx_item_links_type ON item_links(link_type)`,
		`CREATE TABLE IF NOT EXISTS stash_links (
			stash_id    TEXT PRIMARY KEY,
			item_id     TEXT NOT NULL,
			linked_at   DATETIME NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_stash_links_item ON stash_links(item_id)`,
		`CREATE TABLE IF NOT EXISTS item_logs (
			item_id    TEXT PRIMARY KEY,
			log_path   TEXT NOT NULL,
			updated_at DATETIME NOT NULL
		)`,
		`CREATE UNIQUE INDEX IF NOT EXISTS idx_item_logs_path ON item_logs(log_path)`,
		`CREATE TABLE IF NOT EXISTS item_log_entries (
			id         INTEGER PRIMARY KEY AUTOINCREMENT,
			item_id    TEXT NOT NULL,
			log_path   TEXT NOT NULL,
			timestamp  DATETIME NOT NULL,
			actor      TEXT NOT NULL,
			event_type TEXT NOT NULL,
			content    TEXT NOT NULL DEFAULT '',
			delta_json TEXT NOT NULL DEFAULT '',
			UNIQUE (item_id, timestamp, actor, event_type, delta_json)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_item_log_entries_item ON item_log_entries(item_id)`,
		`CREATE INDEX IF NOT EXISTS idx_item_log_entries_path ON item_log_entries(log_path)`,
		`CREATE INDEX IF NOT EXISTS idx_item_log_entries_timestamp ON item_log_entries(timestamp)`,
		// gate_evidence is a derived, advisory-only projection (Q3.1 / 083.005.002-ST):
		// disposable read-model rebuilt from item_log_entries on every `backlogit sync`.
		// The per-item JSONL event logs remain the source of truth; this table stores
		// ONLY the derived status token + SHAs (never report JSON / stderr / force_reason).
		// The gate_status discriminator index backs the advisory doctor query (Q3.3).
		`CREATE TABLE IF NOT EXISTS gate_evidence (
			item_id      TEXT PRIMARY KEY,
			gate_status  TEXT,
			evidence_sha TEXT,
			head_sha     TEXT
		)`,
		`CREATE INDEX IF NOT EXISTS idx_gate_evidence_status ON gate_evidence(gate_status)`,
		`CREATE VIRTUAL TABLE IF NOT EXISTS item_log_entries_fts USING fts5(
			item_id UNINDEXED,
			actor,
			event_type,
			content,
			content='item_log_entries',
			content_rowid='id'
		)`,
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
		`CREATE TRIGGER IF NOT EXISTS item_log_entries_ai AFTER INSERT ON item_log_entries BEGIN
			INSERT INTO item_log_entries_fts(rowid, item_id, actor, event_type, content)
			VALUES (new.id, new.item_id, new.actor, new.event_type, new.content);
		END`,
		`CREATE TRIGGER IF NOT EXISTS item_log_entries_ad AFTER DELETE ON item_log_entries BEGIN
			INSERT INTO item_log_entries_fts(item_log_entries_fts, rowid, item_id, actor, event_type, content)
			VALUES ('delete', old.id, old.item_id, old.actor, old.event_type, old.content);
		END`,
		`CREATE TRIGGER IF NOT EXISTS item_log_entries_au AFTER UPDATE ON item_log_entries BEGIN
			INSERT INTO item_log_entries_fts(item_log_entries_fts, rowid, item_id, actor, event_type, content)
			VALUES ('delete', old.id, old.item_id, old.actor, old.event_type, old.content);
			INSERT INTO item_log_entries_fts(rowid, item_id, actor, event_type, content)
			VALUES (new.id, new.item_id, new.actor, new.event_type, new.content);
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
		`ALTER TABLE stash_entries ADD COLUMN priority TEXT NOT NULL DEFAULT 'medium'`,
		`ALTER TABLE stash_entries ADD COLUMN deliberation_id TEXT`,
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
