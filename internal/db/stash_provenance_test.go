package db_test

// 015.010-T: Stash JSONL rehydration provenance harness.
//
// When stash.jsonl is present (migrated workspace), all stash entries indexed
// by rehydration must report stash.jsonl as their source_path — even entries
// that originated in the legacy .stash.md file and were not yet removed from it.
//
// The production fix skips reading .stash.md when stash.jsonl exists, so that
// legacy-only entries are not indexed and no entry reports "queue/.stash.md"
// as its provenance in a migrated workspace.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/db"
	"github.com/backlogit/backlogit/internal/stash"
)

// TestRehydrate_MigratedWorkspace_SkipsLegacyStash verifies that when
// stash.jsonl is present, entries that exist only in .stash.md are NOT indexed.
// In a migrated workspace the JSONL file is the authoritative source; legacy
// content that was not migrated should not re-appear in the index.
func TestRehydrate_MigratedWorkspace_SkipsLegacyStash(t *testing.T) {
	// Arrange
	ws := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(ws, "queue"), 0o755))

	// Write a legacy .stash.md with two entries, one of which overlaps with JSONL.
	legacyContent := stash.RenderContent(nil, []stash.Entry{
		{ID: "LEGACY01", Priority: "medium", Kind: "task", Text: "legacy-only entry"},
		{ID: "OVERLAP1", Priority: "low", Kind: "bug", Text: "legacy overlap entry"},
	})
	legacyPath := filepath.Join(ws, "queue", stash.FileName)
	require.NoError(t, os.WriteFile(legacyPath, []byte(legacyContent), 0o644))

	// Write stash.jsonl with the JSONL-authoritative version of OVERLAP1 and a
	// new JSONL-only entry.
	jsonlFile, err := os.Create(filepath.Join(ws, stash.JSONLFileName))
	require.NoError(t, err)
	require.NoError(t, stash.WriteJSONL(jsonlFile, []stash.Entry{
		{ID: "OVERLAP1", Priority: "critical", Kind: "bug", Text: "jsonl override entry"},
		{ID: "JSONL001", Priority: "high", Kind: "feature", Text: "jsonl-only entry"},
	}))
	require.NoError(t, jsonlFile.Close())

	database := setupTestDB(t)
	ctx := context.Background()

	// Act
	count, err := db.Rehydrate(ctx, ws, database)
	require.NoError(t, err)

	// Assert: only JSONL entries are indexed (LEGACY01 is skipped).
	// count must be 2 (OVERLAP1 + JSONL001), NOT 3.
	assert.Equal(t, 2, count, "migrated workspace: only JSONL entries should be indexed")

	entries, err := db.ListStashEntries(ctx, database, false)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	byID := make(map[string]db.StashRecord, len(entries))
	for _, e := range entries {
		byID[e.ID] = e
	}

	// LEGACY01 must not appear.
	_, hasLegacy := byID["LEGACY01"]
	assert.False(t, hasLegacy, "LEGACY01 must not be indexed when stash.jsonl exists")

	// OVERLAP1 must use the JSONL version with JSONL source_path.
	require.Contains(t, byID, "OVERLAP1")
	assert.Equal(t, "jsonl override entry", byID["OVERLAP1"].Text)
	assert.Equal(t, "critical", byID["OVERLAP1"].Priority)
	assert.Equal(t, stash.JSONLFileName, byID["OVERLAP1"].SourcePath,
		"OVERLAP1 must report stash.jsonl as source_path in a migrated workspace")

	// JSONL001 must be indexed with correct provenance.
	require.Contains(t, byID, "JSONL001")
	assert.Equal(t, stash.JSONLFileName, byID["JSONL001"].SourcePath)
}

// TestRehydrate_LegacyOnlyWorkspace_StillIndexesStashMd verifies that a
// workspace without stash.jsonl continues to index from .stash.md correctly,
// preserving backwards compatibility.
func TestRehydrate_LegacyOnlyWorkspace_StillIndexesStashMd(t *testing.T) {
	ws := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(ws, "queue"), 0o755))

	legacyContent := stash.RenderContent(nil, []stash.Entry{
		{ID: "LEGONLY1", Priority: "medium", Kind: "task", Text: "legacy task"},
		{ID: "LEGONLY2", Priority: "high", Kind: "feature", Text: "legacy feature"},
	})
	require.NoError(t, os.WriteFile(
		filepath.Join(ws, "queue", stash.FileName), []byte(legacyContent), 0o644))

	database := setupTestDB(t)
	ctx := context.Background()

	count, err := db.Rehydrate(ctx, ws, database)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	entries, err := db.ListStashEntries(ctx, database, false)
	require.NoError(t, err)
	require.Len(t, entries, 2)

	expectedPath := filepath.ToSlash(filepath.Join("queue", stash.FileName))
	for _, e := range entries {
		assert.Equal(t, expectedPath, e.SourcePath,
			"legacy-only workspace: source_path must be queue/.stash.md")
	}
}

// TestRehydrate_JsonlOnlyWorkspace_IndexesCorrectly verifies the canonical
// post-migration path: stash.jsonl exists, .stash.md does not.
func TestRehydrate_JsonlOnlyWorkspace_IndexesCorrectly(t *testing.T) {
	ws := t.TempDir()

	jsonlFile, err := os.Create(filepath.Join(ws, stash.JSONLFileName))
	require.NoError(t, err)
	require.NoError(t, stash.WriteJSONL(jsonlFile, []stash.Entry{
		{ID: "JONLY001", Priority: "medium", Kind: "task", Text: "jsonl task"},
	}))
	require.NoError(t, jsonlFile.Close())

	database := setupTestDB(t)
	ctx := context.Background()

	count, err := db.Rehydrate(ctx, ws, database)
	require.NoError(t, err)
	assert.Equal(t, 1, count)

	entries, err := db.ListStashEntries(ctx, database, false)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, stash.JSONLFileName, entries[0].SourcePath)
}

// TestRehydrate_EmptyWorkspace_ProducesNoStashEntries verifies graceful
// handling when neither stash file exists.
func TestRehydrate_EmptyWorkspace_ProducesNoStashEntries(t *testing.T) {
	ws := t.TempDir()
	database := setupTestDB(t)
	ctx := context.Background()

	count, err := db.Rehydrate(ctx, ws, database)
	require.NoError(t, err)
	assert.Equal(t, 0, count)

	entries, err := db.ListStashEntries(ctx, database, false)
	require.NoError(t, err)
	assert.Empty(t, entries)
}

// TestRehydrate_EmptyJSONL_FallsBackToLegacyStash verifies that an empty
// stash.jsonl (as created by backlogit init) does not suppress legacy
// .stash.md entries. The JSONL file must have at least one entry before it
// is treated as authoritative.
func TestRehydrate_EmptyJSONL_FallsBackToLegacyStash(t *testing.T) {
	ws := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(ws, "queue"), 0o755))

	// Legacy entries in .stash.md.
	legacyContent := stash.RenderContent(nil, []stash.Entry{
		{ID: "LEG001", Priority: "high", Kind: "task", Text: "legacy high priority task"},
	})
	require.NoError(t, os.WriteFile(filepath.Join(ws, "queue", stash.FileName), []byte(legacyContent), 0o644))

	// Empty stash.jsonl — as created by backlogit init on a legacy workspace.
	require.NoError(t, os.WriteFile(filepath.Join(ws, stash.JSONLFileName), []byte(""), 0o644))

	database := setupTestDB(t)
	ctx := context.Background()

	count, err := db.Rehydrate(ctx, ws, database)
	require.NoError(t, err)
	assert.Equal(t, 1, count, "empty stash.jsonl must not suppress legacy stash entries")

	entries, err := db.ListStashEntries(ctx, database, false)
	require.NoError(t, err)
	require.Len(t, entries, 1)
	assert.Equal(t, "LEG001", entries[0].ID)
	assert.Equal(t, filepath.ToSlash(filepath.Join("queue", stash.FileName)), entries[0].SourcePath)
}
