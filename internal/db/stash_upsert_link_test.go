package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/db"
)

func TestUpsertAndLinkStashEntry_Success(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()

	require.NoError(t, db.UpsertAndLinkStashEntry(ctx, database, "s1", "001-T", "high", "idea", "text", "", "harvested", "stash.jsonl", time.Now()))

	records, err := db.ListStashEntries(ctx, database, true)
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, "harvested", records[0].State)
	assert.Equal(t, "001-T", records[0].ItemID)
}

func TestUpsertAndLinkStashEntry_LinkFailureRollsBackEntry(t *testing.T) {
	database := setupTestDB(t)
	ctx := context.Background()
	_, err := database.ExecContext(ctx, `DROP TABLE stash_links`)
	require.NoError(t, err)

	err = db.UpsertAndLinkStashEntry(ctx, database, "s1", "001-T", "high", "idea", "text", "", "harvested", "stash.jsonl", time.Now())
	require.Error(t, err)

	var n int
	require.NoError(t, database.QueryRowContext(ctx, `SELECT COUNT(*) FROM stash_entries`).Scan(&n))
	assert.Equal(t, 0, n, "stash entry must not persist when the link write fails")
}
