package db

import (
	"database/sql"
	"fmt"
	"regexp"

	"github.com/backlogit/backlogit/internal/config"
)

// columnNameRe validates column names against safe SQL identifier pattern.
// CRITICAL (P1 review finding): prevents DDL injection via crafted field names.
var columnNameRe = regexp.MustCompile(`^[a-z][a-z0-9_]{0,62}$`)

// ValidateColumnName checks that a column name is safe for use in DDL statements.
func ValidateColumnName(name string) error {
	if !columnNameRe.MatchString(name) {
		return fmt.Errorf("invalid column name %q: must match ^[a-z][a-z0-9_]{0,62}$", name)
	}
	return nil
}

// MapFieldTypeToSQLite converts a YAML field type to the corresponding SQLite column type.
func MapFieldTypeToSQLite(fieldType string) (string, error) {
	switch fieldType {
	case "string", "enum", "list":
		return "TEXT", nil
	case "int":
		return "INTEGER", nil
	case "datetime":
		return "DATETIME", nil
	default:
		return "", fmt.Errorf("unknown field type %q: cannot map to SQLite type", fieldType)
	}
}

// GenerateSchemaExtensions reads custom field definitions from HeaderDefConfig and returns
// ALTER TABLE statements to add columns not yet present in the items table.
func GenerateSchemaExtensions(db *sql.DB, headerDef *config.HeaderDefConfig) ([]string, error) {
	existing, err := existingColumns(db)
	if err != nil {
		return nil, err
	}

	seen := make(map[string]bool)
	var stmts []string
	for _, typeCfg := range headerDef.Types {
		for fieldName, def := range typeCfg.Fields {
			if seen[fieldName] || existing[fieldName] {
				continue
			}
			seen[fieldName] = true
			if err := ValidateColumnName(fieldName); err != nil {
				return nil, err
			}
			sqlType, err := MapFieldTypeToSQLite(def.Type)
			if err != nil {
				return nil, fmt.Errorf("field %q: %w", fieldName, err)
			}
			stmts = append(stmts, fmt.Sprintf(`ALTER TABLE items ADD COLUMN "%s" %s`, fieldName, sqlType))
		}
	}
	return stmts, nil
}

// ApplySchemaExtensions executes the generated ALTER TABLE statements idempotently
// within an explicit transaction to prevent partial schema migrations on failure.
func ApplySchemaExtensions(db *sql.DB, headerDef *config.HeaderDefConfig) error {
	stmts, err := GenerateSchemaExtensions(db, headerDef)
	if err != nil {
		return err
	}
	if len(stmts) == 0 {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin schema extension transaction: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	for _, stmt := range stmts {
		if _, err := tx.Exec(stmt); err != nil {
			// SQLite returns error if column already exists; treat as idempotent.
			if !isColumnExistsError(err) {
				return fmt.Errorf("apply schema extension: %w", err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit schema extensions: %w", err)
	}
	return nil
}

// existingColumns returns a set of column names already present in the items table.
func existingColumns(db *sql.DB) (map[string]bool, error) {
	rows, err := db.Query("PRAGMA table_info(items)")
	if err != nil {
		return nil, fmt.Errorf("pragma table_info: %w", err)
	}
	defer rows.Close()

	cols := make(map[string]bool)
	for rows.Next() {
		var cid int
		var name, colType string
		var notNull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return nil, fmt.Errorf("scan table_info: %w", err)
		}
		cols[name] = true
	}
	return cols, rows.Err()
}

// isColumnExistsError returns true for the SQLite "duplicate column name" error.
func isColumnExistsError(err error) bool {
	return err != nil && (contains(err.Error(), "duplicate column name") ||
		contains(err.Error(), "already exists"))
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub ||
		len(s) > 0 && len(sub) > 0 && indexStr(s, sub) >= 0)
}

func indexStr(s, sub string) int {
	if len(sub) == 0 {
		return 0
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
