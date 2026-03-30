package db

import (
	"context"
	"database/sql"
)

// Rehydrate walks the workspace directory tree and rebuilds the SQLite index.
//
// Worker: Implement concurrent rehydration with errgroup and TRUNCATE+INSERT.
func Rehydrate(ctx context.Context, workspacePath string, db *sql.DB) (int, error) {
	panic("not implemented: Worker: Implement concurrent rehydration engine")
}
