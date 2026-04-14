package db_test

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/models"
)

// TASK-002.01.02: Update DB schema and queries for new artifact fields.

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	database, err := db.Open(dbPath)
	require.NoError(t, err)
	require.NoError(t, db.EnsureSchema(database))
	t.Cleanup(func() { database.Close() })
	return database
}

func TestUpsertItem_NewFields_RoundTrip(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	artifact := &models.Artifact{
		ID:           "T001",
		Title:        "Test task",
		Status:       models.StatusQueued,
		ArtifactType: "task",
		AssignedTo:   "alice",
		Owner:        "bob",
		Labels:       []string{"backend", "urgent"},
		Dependencies: []string{"T002", "T003"},
		References:   []string{"docs/spec.md", "README.md"},
		Commit:       "abc123def",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// Act — upsert then retrieve
	err := db.UpsertItem(ctx, database, artifact)
	require.NoError(t, err)

	got, err := db.GetItem(ctx, database, "T001")
	require.NoError(t, err)

	// Assert — all new fields round-trip correctly
	assert.Equal(t, "alice", got.AssignedTo)
	assert.Equal(t, "bob", got.Owner)
	assert.Equal(t, []string{"backend", "urgent"}, got.Labels)
	assert.Equal(t, []string{"T002", "T003"}, got.Dependencies)
	assert.Equal(t, []string{"docs/spec.md", "README.md"}, got.References)
	assert.Equal(t, "abc123def", got.Commit)
}

func TestUpsertItem_EmptySliceFields(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	artifact := &models.Artifact{
		ID:           "T002",
		Title:        "Minimal task",
		Status:       models.StatusQueued,
		ArtifactType: "task",
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	// Act
	err := db.UpsertItem(ctx, database, artifact)
	require.NoError(t, err)

	got, err := db.GetItem(ctx, database, "T002")
	require.NoError(t, err)

	// Assert — nil/empty slices survive the round trip
	assert.Empty(t, got.AssignedTo)
	assert.Empty(t, got.Owner)
	assert.Empty(t, got.Labels)
	assert.Empty(t, got.Dependencies)
	assert.Empty(t, got.References)
	assert.Empty(t, got.Commit)
}

func TestQueryItems_FilterByAssignedTo(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	for _, a := range []*models.Artifact{
		{ID: "T010", Title: "Alice task", Status: models.StatusQueued, ArtifactType: "task", AssignedTo: "alice", CreatedAt: now, UpdatedAt: now},
		{ID: "T011", Title: "Bob task", Status: models.StatusQueued, ArtifactType: "task", AssignedTo: "bob", CreatedAt: now, UpdatedAt: now},
		{ID: "T012", Title: "Alice bug", Status: models.StatusActive, ArtifactType: "bug", AssignedTo: "alice", CreatedAt: now, UpdatedAt: now},
	} {
		require.NoError(t, db.UpsertItem(ctx, database, a))
	}

	// Act
	results, err := db.QueryItems(ctx, database, db.QueryFilters{AssignedTo: "alice"})
	require.NoError(t, err)

	// Assert
	assert.Len(t, results, 2)
	for _, r := range results {
		assert.Equal(t, "alice", r.AssignedTo)
	}
}

func TestQueryItems_FilterByOwner(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	for _, a := range []*models.Artifact{
		{ID: "T020", Title: "Owned by bob", Status: models.StatusQueued, ArtifactType: "task", Owner: "bob", CreatedAt: now, UpdatedAt: now},
		{ID: "T021", Title: "Owned by carol", Status: models.StatusQueued, ArtifactType: "task", Owner: "carol", CreatedAt: now, UpdatedAt: now},
	} {
		require.NoError(t, db.UpsertItem(ctx, database, a))
	}

	// Act
	results, err := db.QueryItems(ctx, database, db.QueryFilters{Owner: "bob"})
	require.NoError(t, err)

	// Assert
	assert.Len(t, results, 1)
	assert.Equal(t, "bob", results[0].Owner)
}

func TestSearchItems_MatchesLabels(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)

	artifact := &models.Artifact{
		ID:           "T030",
		Title:        "Search test",
		Status:       models.StatusQueued,
		ArtifactType: "task",
		Labels:       []string{"searchable-label"},
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	require.NoError(t, db.UpsertItem(ctx, database, artifact))

	// Act — FTS5 should index labels
	results, err := db.SearchItems(ctx, database, "searchable-label", 10)
	require.NoError(t, err)

	// Assert
	assert.Len(t, results, 1)
	assert.Equal(t, "T030", results[0].ID)
}

func TestIndexEvent_StoresItemLogRelationshipAndSearchableEntry(t *testing.T) {
	// Arrange
	database := setupTestDB(t)
	ctx := context.Background()
	now := time.Now().Truncate(time.Second)
	logsDir := filepath.Join(t.TempDir(), "logs")

	event := events.Event{
		Timestamp: now,
		Actor:     "alice",
		ItemID:    "T040",
		EventType: "comment",
		Delta: map[string]any{
			"comment": "investigated the queue migration path",
		},
	}

	// Act
	err := db.IndexEvent(ctx, database, logsDir, event)
	require.NoError(t, err)

	results, err := db.ListItemLogEntries(ctx, database, "T040", 10)
	require.NoError(t, err)

	searchResults, err := db.SearchItemLogEntries(ctx, database, "queue migration", 10)
	require.NoError(t, err)

	// Assert
	require.Len(t, results, 1)
	assert.Equal(t, "T040", results[0].ItemID)
	assert.Equal(t, "alice", results[0].Actor)
	assert.Equal(t, "comment", results[0].EventType)
	assert.Contains(t, results[0].LogPath, "logs/T040.jsonl")
	assert.Equal(t, "investigated the queue migration path", results[0].Delta["comment"])

	require.Len(t, searchResults, 1)
	assert.Equal(t, "T040", searchResults[0].ItemID)
}
