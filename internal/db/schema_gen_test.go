package db_test

import (
	"context"
	"testing"

	"database/sql"
	"path/filepath"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/db"
)

func TestValidateColumnName(t *testing.T) {
	tests := []struct {
		name    string
		col     string
		wantErr bool
	}{
		{name: "valid lowercase", col: "status", wantErr: false},
		{name: "valid with underscore", col: "assigned_to", wantErr: false},
		{name: "valid with numbers", col: "field1", wantErr: false},
		{name: "starts with number", col: "1field", wantErr: true},
		{name: "contains uppercase", col: "Status", wantErr: true},
		{name: "contains hyphen", col: "my-field", wantErr: true},
		{name: "empty string", col: "", wantErr: true},
		{name: "SQL injection attempt", col: "x; DROP TABLE items--", wantErr: true},
		{name: "too long (64 chars)", col: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			err := db.ValidateColumnName(tt.col)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestMapFieldTypeToSQLite(t *testing.T) {
	tests := []struct {
		name      string
		fieldType string
		want      string
		wantErr   bool
	}{
		{name: "string maps to TEXT", fieldType: "string", want: "TEXT"},
		{name: "int maps to INTEGER", fieldType: "int", want: "INTEGER"},
		{name: "enum maps to TEXT", fieldType: "enum", want: "TEXT"},
		{name: "list maps to TEXT", fieldType: "list", want: "TEXT"},
		{name: "datetime maps to DATETIME", fieldType: "datetime", want: "DATETIME"},
		{name: "unknown type errors", fieldType: "blob", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got, err := db.MapFieldTypeToSQLite(tt.fieldType)

			// Assert
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGenerateSchemaExtensions(t *testing.T) {
	// Arrange
	database := setupSchemaGenTestDB(t)
	headerDef := &config.HeaderDefConfig{
		Defaults: config.SystemDefaults{
			ID:          config.FieldDef{Type: "string", Immutable: true},
			CreatedDate: config.FieldDef{Type: "datetime", Immutable: true},
			UpdatedDate: config.FieldDef{Type: "datetime", Immutable: false},
		},
		Types: map[string]*config.TypeDefConfig{
			"task": {
				Prefix:   "T",
				IDFormat: "{prefix}{NNN}",
				Fields: map[string]*config.FieldDef{
					"custom_priority": {Type: "enum", Values: []string{"low", "medium", "high"}},
				},
			},
		},
	}

	// Act
	stmts, err := db.GenerateSchemaExtensions(database, headerDef)

	// Assert
	require.NoError(t, err)
	assert.NotEmpty(t, stmts, "should generate ALTER TABLE for custom_priority")
}

func TestApplySchemaExtensions_Idempotent(t *testing.T) {
	// Arrange
	database := setupSchemaGenTestDB(t)
	headerDef := &config.HeaderDefConfig{
		Defaults: config.SystemDefaults{
			ID:          config.FieldDef{Type: "string", Immutable: true},
			CreatedDate: config.FieldDef{Type: "datetime", Immutable: true},
			UpdatedDate: config.FieldDef{Type: "datetime", Immutable: false},
		},
		Types: map[string]*config.TypeDefConfig{
			"task": {
				Prefix:   "T",
				IDFormat: "{prefix}{NNN}",
				Fields: map[string]*config.FieldDef{
					"severity": {Type: "string"},
				},
			},
		},
	}

	// Act — run twice
	err1 := db.ApplySchemaExtensions(database, headerDef)
	err2 := db.ApplySchemaExtensions(database, headerDef)

	// Assert — both succeed (idempotent)
	require.NoError(t, err1)
	require.NoError(t, err2)
}

func setupSchemaGenTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)
	require.NoError(t, db.EnsureSchema(database))
	t.Cleanup(func() { database.Close() })
	return database
}

func TestIntrospectSchema_ReturnsAllCoreTables(t *testing.T) {
	// Arrange
	database := setupSchemaGenTestDB(t)
	ctx := context.Background()

	// Act
	tables, err := db.IntrospectSchema(ctx, database)

	// Assert
	require.NoError(t, err)

	// Collect table names.
	names := make(map[string]bool, len(tables))
	for _, ts := range tables {
		names[ts.Name] = true
	}

	// Core tables that EnsureSchema creates.
	expectedTables := []string{
		"items", "item_deps", "commit_links", "stash_entries",
		"item_links", "stash_links", "item_logs", "item_log_entries",
	}
	for _, name := range expectedTables {
		assert.True(t, names[name], "expected table %q in schema", name)
	}

	// FTS virtual tables should be detected as virtual.
	expectedVirtual := []string{"items_fts", "item_log_entries_fts"}
	for _, name := range expectedVirtual {
		assert.True(t, names[name], "expected virtual table %q in schema", name)
	}

	// Verify FTS tables are marked as virtual.
	for _, ts := range tables {
		if ts.Name == "items_fts" || ts.Name == "item_log_entries_fts" {
			assert.True(t, ts.IsVirtual, "table %q should be virtual", ts.Name)
		}
	}

	// FTS shadow tables should NOT appear.
	for name := range names {
		assert.False(t,
			contains(name, "_fts_content") || contains(name, "_fts_data") ||
				contains(name, "_fts_config") || contains(name, "_fts_docsize") ||
				contains(name, "_fts_idx"),
			"FTS shadow table %q should be excluded", name)
	}
}

func TestIntrospectSchema_ColumnsAndIndexes(t *testing.T) {
	// Arrange
	database := setupSchemaGenTestDB(t)
	ctx := context.Background()

	// Act
	tables, err := db.IntrospectSchema(ctx, database)
	require.NoError(t, err)

	// Find the items table.
	var itemsTable *db.TableSchema
	for i := range tables {
		if tables[i].Name == "items" {
			itemsTable = &tables[i]
			break
		}
	}
	require.NotNil(t, itemsTable, "items table should exist")

	// Columns should include core fields.
	colNames := make(map[string]bool, len(itemsTable.Columns))
	for _, c := range itemsTable.Columns {
		colNames[c.Name] = true
	}
	assert.True(t, colNames["id"], "items should have 'id' column")
	assert.True(t, colNames["title"], "items should have 'title' column")
	assert.True(t, colNames["status"], "items should have 'status' column")
	assert.True(t, colNames["artifact_type"], "items should have 'artifact_type' column")

	// Primary key detection.
	for _, c := range itemsTable.Columns {
		if c.Name == "id" {
			assert.True(t, c.PrimaryKey, "'id' should be primary key")
		}
	}

	// Indexes should include named indexes (not autoindexes).
	idxNames := make(map[string]bool, len(itemsTable.Indexes))
	for _, idx := range itemsTable.Indexes {
		idxNames[idx.Name] = true
	}
	assert.True(t, idxNames["idx_items_status"], "items should have idx_items_status index")
	assert.True(t, idxNames["idx_items_type"], "items should have idx_items_type index")

	// Each index should have at least one column.
	for _, idx := range itemsTable.Indexes {
		assert.NotEmpty(t, idx.Columns, "index %q should have columns", idx.Name)
	}
}

func TestIntrospectSchema_SortedByTableName(t *testing.T) {
	// Arrange
	database := setupSchemaGenTestDB(t)
	ctx := context.Background()

	// Act
	tables, err := db.IntrospectSchema(ctx, database)
	require.NoError(t, err)

	// Assert — tables are sorted alphabetically.
	for i := 1; i < len(tables); i++ {
		assert.True(t, tables[i-1].Name <= tables[i].Name,
			"tables should be sorted: %q should come before %q", tables[i-1].Name, tables[i].Name)
	}
}

func TestIntrospectSchema_VirtualTablesHaveNoIndexes(t *testing.T) {
	// Arrange
	database := setupSchemaGenTestDB(t)
	ctx := context.Background()

	// Act
	tables, err := db.IntrospectSchema(ctx, database)
	require.NoError(t, err)

	// Assert — virtual tables should have empty Indexes.
	for _, ts := range tables {
		if ts.IsVirtual {
			assert.Empty(t, ts.Indexes, "virtual table %q should have no indexes", ts.Name)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchSubstring(s, substr)
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
