package db

import (
	"database/sql"
	"regexp"

	"github.com/backlogit/backlogit/internal/config"
)

// columnNameRe validates column names against safe SQL identifier pattern.
// CRITICAL (P1 review finding): prevents DDL injection via crafted field names.
var columnNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

// ValidateColumnName checks that a column name is safe for use in DDL statements.
//
// Worker: Return nil if name matches ^[a-z][a-z0-9_]{0,62}$ regex, otherwise
// return a descriptive error including the invalid name.
func ValidateColumnName(name string) error {
	panic("not implemented: Worker: Validate column name against safe SQL identifier regex and return descriptive error for invalid names")
}

// MapFieldTypeToSQLite converts a YAML field type to the corresponding SQLite
// column type. Mapping: string→TEXT, int→INTEGER, enum→TEXT, list→TEXT (JSON),
// datetime→DATETIME.
//
// Worker: Implement type mapping switch. Return error for unknown types.
func MapFieldTypeToSQLite(fieldType string) (string, error) {
	panic("not implemented: Worker: Map YAML field types (string, int, enum, list, datetime) to SQLite column types")
}

// GenerateSchemaExtensions reads custom field definitions from HeaderDefConfig
// and returns ALTER TABLE statements to add columns not yet present in the items table.
//
// Worker: Enumerate all fields across all types in HeaderDefConfig. For each field,
// validate the column name, map the type, check if column already exists via
// PRAGMA table_info, and generate ALTER TABLE items ADD COLUMN if missing.
func GenerateSchemaExtensions(db *sql.DB, headerDef *config.HeaderDefConfig) ([]string, error) {
	panic("not implemented: Worker: Generate ALTER TABLE statements for custom fields, validating column names before DDL generation")
}

// ApplySchemaExtensions executes the generated ALTER TABLE statements and rebuilds
// the FTS5 index to include new searchable columns.
//
// Worker: Execute each ALTER TABLE statement. Rebuild FTS5 virtual table to
// include new columns. Use PRAGMA user_version to track schema version for
// idempotent re-runs.
func ApplySchemaExtensions(db *sql.DB, headerDef *config.HeaderDefConfig) error {
	panic("not implemented: Worker: Apply schema extensions and rebuild FTS5 index with new custom field columns")
}
