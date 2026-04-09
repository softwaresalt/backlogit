package db_test

// 018.001-T: item_links core function harness.
//
// Implementation complete. These tests validate the contract for AddLink,
// RemoveLink, GetLinks, and GetLinksByType.

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/db"
	"github.com/backlogit/backlogit/internal/models"
)

func setupLinksTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "links_test.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)
	require.NoError(t, db.EnsureSchema(database))
	t.Cleanup(func() { database.Close() })

	ctx := context.Background()
	for _, a := range []*models.Artifact{
		{ID: "A001", Title: "Artifact 1", Status: models.StatusQueued, ArtifactType: "task"},
		{ID: "A002", Title: "Artifact 2", Status: models.StatusQueued, ArtifactType: "task"},
		{ID: "A003", Title: "Artifact 3", Status: models.StatusQueued, ArtifactType: "task"},
	} {
		require.NoError(t, db.UpsertItem(ctx, database, a))
	}
	return database
}

func TestAddLink_ValidType(t *testing.T) {
	database := setupLinksTestDB(t)
	ctx := context.Background()

	err := db.AddLink(ctx, database, "A001", "A002", "related_to")

	require.NoError(t, err)
}

func TestAddLink_InvalidType_Rejected(t *testing.T) {
	database := setupLinksTestDB(t)
	ctx := context.Background()

	err := db.AddLink(ctx, database, "A001", "A002", "invented_type")

	require.Error(t, err, "invalid link_type must be rejected")
}

func TestAddLink_AllValidTypes_Accepted(t *testing.T) {
	database := setupLinksTestDB(t)
	ctx := context.Background()

	for _, lt := range db.ValidLinkTypes {
		t.Run(lt, func(t *testing.T) {
			err := db.AddLink(ctx, database, "A001", "A002", lt)
			require.NoError(t, err, "valid link_type %q must be accepted", lt)
			// Clean up for next sub-test.
			require.NoError(t, db.RemoveLink(ctx, database, "A001", "A002", lt))
		})
	}
}

func TestAddLink_Idempotent(t *testing.T) {
	database := setupLinksTestDB(t)
	ctx := context.Background()

	require.NoError(t, db.AddLink(ctx, database, "A001", "A002", "informs"))
	require.NoError(t, db.AddLink(ctx, database, "A001", "A002", "informs"), "duplicate add must not error")

	links, err := db.GetLinks(ctx, database, "A001")
	require.NoError(t, err)
	assert.Len(t, links, 1, "upsert must not create duplicate rows")
}

func TestGetLinks_ReturnsAllOutgoing(t *testing.T) {
	database := setupLinksTestDB(t)
	ctx := context.Background()
	require.NoError(t, db.AddLink(ctx, database, "A001", "A002", "related_to"))
	require.NoError(t, db.AddLink(ctx, database, "A001", "A003", "informs"))

	links, err := db.GetLinks(ctx, database, "A001")

	require.NoError(t, err)
	require.Len(t, links, 2)
	targets := make(map[string]bool)
	for _, l := range links {
		targets[l.TargetID] = true
		assert.Equal(t, "A001", l.SourceID)
	}
	assert.True(t, targets["A002"])
	assert.True(t, targets["A003"])
}

func TestGetLinks_EmptyWhenNone(t *testing.T) {
	database := setupLinksTestDB(t)
	ctx := context.Background()

	links, err := db.GetLinks(ctx, database, "A001")

	require.NoError(t, err)
	assert.Empty(t, links)
}

func TestGetLinksByType_FiltersCorrectly(t *testing.T) {
	database := setupLinksTestDB(t)
	ctx := context.Background()
	require.NoError(t, db.AddLink(ctx, database, "A001", "A002", "related_to"))
	require.NoError(t, db.AddLink(ctx, database, "A001", "A003", "duplicate_of"))

	links, err := db.GetLinksByType(ctx, database, "A001", "related_to")

	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, "A002", links[0].TargetID)
	assert.Equal(t, "related_to", links[0].LinkType)
}

func TestGetLinksByType_EmptyForMissingType(t *testing.T) {
	database := setupLinksTestDB(t)
	ctx := context.Background()
	require.NoError(t, db.AddLink(ctx, database, "A001", "A002", "informs"))

	links, err := db.GetLinksByType(ctx, database, "A001", "supersedes")

	require.NoError(t, err)
	assert.Empty(t, links)
}

func TestRemoveLink_DeletesEdge(t *testing.T) {
	database := setupLinksTestDB(t)
	ctx := context.Background()
	require.NoError(t, db.AddLink(ctx, database, "A001", "A002", "related_to"))

	err := db.RemoveLink(ctx, database, "A001", "A002", "related_to")

	require.NoError(t, err)
	links, err := db.GetLinks(ctx, database, "A001")
	require.NoError(t, err)
	assert.Empty(t, links)
}

func TestRemoveLink_NoopWhenMissing(t *testing.T) {
	database := setupLinksTestDB(t)
	ctx := context.Background()

	err := db.RemoveLink(ctx, database, "A001", "A002", "related_to")

	require.NoError(t, err, "removing a non-existent link must not error")
}
