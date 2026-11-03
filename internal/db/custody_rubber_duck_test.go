package db_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/db"
)

func TestRubberDuck_F7_FailedItemRowLeavesNoCustodyRows(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()
	ws := t.TempDir()
	writeCustodyShipmentMarkdown(t, ws, custodyEnvelope(1, "offered", "rcpt-duck-f7"))
	_, err := database.ExecContext(ctx, `CREATE TRIGGER duck_fail_item_insert BEFORE INSERT ON items BEGIN SELECT RAISE(ABORT, 'forced item insert failure'); END`)
	require.NoError(t, err)

	_, err = db.Rehydrate(ctx, ws, database)
	require.NoError(t, err)
	assert.Empty(t, queryCustodyRows(t, database), "a shipment whose item row failed must not keep custody_receipts rows")
}
