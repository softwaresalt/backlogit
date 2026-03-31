package db_test

import (
	"testing"

	"database/sql"
	"path/filepath"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/config"
	"github.com/backlogit/backlogit/internal/db"
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
