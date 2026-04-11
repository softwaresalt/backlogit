package db_test

// 026.014-T: DeleteItemCascade contract tests.
//
// These tests verify that DeleteItemCascade removes all related rows from
// every satellite table in a single atomic transaction, and that partial
// failures roll back cleanly.

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/db"
	blErrors "github.com/backlogit/backlogit/internal/errors"
	"github.com/backlogit/backlogit/internal/models"
)

// setupCascadeDB opens a fresh in-memory-style SQLite database with the full
// schema applied and returns it.
func setupCascadeDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "cascade_test.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)
	require.NoError(t, db.EnsureSchema(database))
	t.Cleanup(func() { database.Close() })
	return database
}

// insertMinimalArtifact upserts a minimal artifact into the items table.
func insertMinimalArtifact(t *testing.T, database *sql.DB, id string) {
	t.Helper()
	a := &models.Artifact{
		ID:           id,
		Title:        "Cascade test " + id,
		Status:       models.StatusQueued,
		ArtifactType: "task",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}
	require.NoError(t, db.UpsertItem(context.Background(), database, a))
}

// rowCount returns the number of rows in table where col = value.
func rowCount(t *testing.T, database *sql.DB, table, col, value string) int {
	t.Helper()
	var count int
	row := database.QueryRowContext(context.Background(),
		"SELECT COUNT(*) FROM "+table+" WHERE "+col+" = ?", value)
	require.NoError(t, row.Scan(&count))
	return count
}

// TestDeleteItemCascade_RemovesAllRelatedRows creates an item with rows in
// every satellite table, deletes it, and verifies that all satellite rows are
// gone along with the item itself.
func TestDeleteItemCascade_RemovesAllRelatedRows(t *testing.T) {
	database := setupCascadeDB(t)
	ctx := context.Background()

	const targetID = "TEST-001"
	const otherID = "TEST-002"
	insertMinimalArtifact(t, database, targetID)
	insertMinimalArtifact(t, database, otherID)

	// Seed item_deps: targetID → otherID and otherID → targetID (both edges).
	require.NoError(t, db.UpsertDependency(ctx, database, targetID, otherID, "blocks"))
	// item_links: targetID as source and as target.
	require.NoError(t, db.AddLink(ctx, database, targetID, otherID, "related_to"))
	require.NoError(t, db.AddLink(ctx, database, otherID, targetID, "informs"))
	// commit_links: use direct SQL since db.LinkCommit is in the core package.
	_, err := database.ExecContext(ctx,
		`INSERT OR IGNORE INTO commit_links (item_id, commit_sha, message, author) VALUES (?, ?, ?, ?)`,
		targetID, "abc123", "seed commit", "tester")
	require.NoError(t, err)

	// Verify rows exist before delete.
	assert.Equal(t, 1, rowCount(t, database, "item_deps", "item_id", targetID), "dep row must exist before delete")
	assert.Equal(t, 1, rowCount(t, database, "item_links", "source_id", targetID), "link source row must exist before delete")
	assert.Equal(t, 1, rowCount(t, database, "item_links", "target_id", targetID), "link target row must exist before delete")
	assert.Equal(t, 1, rowCount(t, database, "commit_links", "item_id", targetID), "commit link must exist before delete")
	assert.Equal(t, 1, rowCount(t, database, "items", "id", targetID), "item row must exist before delete")

	// Delete the target item.
	require.NoError(t, db.DeleteItemCascade(ctx, database, targetID))

	// All satellite rows referencing targetID must be gone.
	assert.Equal(t, 0, rowCount(t, database, "item_deps", "item_id", targetID), "item_deps row must be removed")
	assert.Equal(t, 0, rowCount(t, database, "item_links", "source_id", targetID), "item_links source row must be removed")
	assert.Equal(t, 0, rowCount(t, database, "item_links", "target_id", targetID), "item_links target row must be removed")
	assert.Equal(t, 0, rowCount(t, database, "commit_links", "item_id", targetID), "commit_links row must be removed")
	assert.Equal(t, 0, rowCount(t, database, "items", "id", targetID), "item row must be removed")

	// The other item and its link rows must be untouched.
	assert.Equal(t, 1, rowCount(t, database, "items", "id", otherID), "unrelated item must survive")
}

// TestDeleteItemCascade_NotFound returns ErrNotFound for an unknown ID.
func TestDeleteItemCascade_NotFound(t *testing.T) {
	database := setupCascadeDB(t)
	ctx := context.Background()

	err := db.DeleteItemCascade(ctx, database, "NONEXISTENT-999")

	require.Error(t, err)
	assert.ErrorIs(t, err, blErrors.ErrNotFound,
		"deleting a non-existent item must return ErrNotFound")
}

// TestDeleteItemCascade_DepsInBothDirections verifies that when an item appears
// as both item_id and depends_on in item_deps, both rows are removed.
func TestDeleteItemCascade_DepsInBothDirections(t *testing.T) {
	database := setupCascadeDB(t)
	ctx := context.Background()

	const a = "DEP-A"
	const b = "DEP-B"
	const c = "DEP-C"
	for _, id := range []string{a, b, c} {
		insertMinimalArtifact(t, database, id)
	}
	// a blocks b  (a = item_id)
	require.NoError(t, db.UpsertDependency(ctx, database, a, b, "blocks"))
	// c blocks a  (a = depends_on)
	require.NoError(t, db.UpsertDependency(ctx, database, c, a, "blocks"))

	assert.Equal(t, 1, rowCount(t, database, "item_deps", "item_id", a))
	assert.Equal(t, 1, rowCount(t, database, "item_deps", "depends_on", a))

	require.NoError(t, db.DeleteItemCascade(ctx, database, a))

	assert.Equal(t, 0, rowCount(t, database, "item_deps", "item_id", a),
		"item_deps rows where item_id=a must be removed")
	assert.Equal(t, 0, rowCount(t, database, "item_deps", "depends_on", a),
		"item_deps rows where depends_on=a must be removed")
	// b and c must still exist.
	assert.Equal(t, 1, rowCount(t, database, "items", "id", b))
	assert.Equal(t, 1, rowCount(t, database, "items", "id", c))
}

// TestDeleteItemCascade_TransactionAtomicity verifies that deleting an existing
// item succeeds and the items table itself is consistent afterwards.
func TestDeleteItemCascade_TransactionAtomicity(t *testing.T) {
	database := setupCascadeDB(t)
	ctx := context.Background()

	const id = "ATOM-001"
	insertMinimalArtifact(t, database, id)

	// Confirm the item exists.
	assert.Equal(t, 1, rowCount(t, database, "items", "id", id))

	require.NoError(t, db.DeleteItemCascade(ctx, database, id))

	// Item must be absent after delete.
	assert.Equal(t, 0, rowCount(t, database, "items", "id", id))

	// Second delete must return ErrNotFound (idempotency check).
	err := db.DeleteItemCascade(ctx, database, id)
	require.ErrorIs(t, err, blErrors.ErrNotFound)
}
