package db

import "database/sql"

// EnsureSchema creates the items table and FTS5 index idempotently.
//
// Worker: Implement CREATE TABLE IF NOT EXISTS with FTS5 and indexes.
func EnsureSchema(db *sql.DB) error {
	panic("not implemented: Worker: Implement schema bootstrapping with FTS5")
}
