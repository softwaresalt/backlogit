package db

import (
	"context"
	"database/sql"

	"github.com/backlogit/backlogit/internal/models"
)

// UpsertItemsTx inserts or replaces one or more artifacts within an existing
// SQL transaction. All writes share the same transaction so a rollback reverts
// every artifact atomically.
//
// Worker: iterate artifacts, marshal fields using the same logic as UpsertItem,
// and call tx.ExecContext for each artifact. Return the first error encountered.
func UpsertItemsTx(ctx context.Context, tx *sql.Tx, artifacts ...*models.Artifact) error {
	panic("not implemented: UpsertItemsTx")
}
