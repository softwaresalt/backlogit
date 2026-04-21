package db_test

// 037.001-T: File Manifest Data Types and Diff Logic
// 037.002-T: Manifest Walk and Population
//
// These harnesses verify:
//   - ClassifyFile correctly categorises workspace-relative paths
//   - ComputeDiff detects adds, changes, deletes, and relocations
//   - ShouldFallback respects the 50 % and absolute-count thresholds
//   - BuildManifest walks a temp workspace and populates entries
//   - RehydrateWithManifest returns the same count as Rehydrate plus a manifest

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/db"
)

// ── ClassifyFile ──────────────────────────────────────────────────────────────

func TestClassifyFile_ArtifactPaths(t *testing.T) {
	cases := []struct {
		relPath string
	}{
		{"queue/037-F.md"},
		{"done/001-T.md"},
		{"active/002-T.md"},
		{"archive/003-F.md"},
	}
	for _, tc := range cases {
		t.Run(tc.relPath, func(t *testing.T) {
			got := db.ClassifyFile(tc.relPath)
			assert.Equal(t, db.FileKindArtifact, got, "expected FileKindArtifact for %s", tc.relPath)
		})
	}
}

func TestClassifyFile_StashPath(t *testing.T) {
	got := db.ClassifyFile("stash.jsonl")
	assert.Equal(t, db.FileKindStash, got)
}

func TestClassifyFile_LogPaths(t *testing.T) {
	cases := []string{
		"logs/037-F.jsonl",
		"logs/037.001-T.jsonl",
	}
	for _, relPath := range cases {
		t.Run(relPath, func(t *testing.T) {
			got := db.ClassifyFile(relPath)
			assert.Equal(t, db.FileKindLog, got, "expected FileKindLog for %s", relPath)
		})
	}
}

func TestClassifyFile_ConfigPaths(t *testing.T) {
	cases := []string{
		"config.yaml",
		"registry.yaml",
		"hooks.yaml",
		"header-def.yaml",
	}
	for _, relPath := range cases {
		t.Run(relPath, func(t *testing.T) {
			got := db.ClassifyFile(relPath)
			assert.Equal(t, db.FileKindConfig, got, "expected FileKindConfig for %s", relPath)
		})
	}
}

func TestClassifyFile_OtherPaths(t *testing.T) {
	got := db.ClassifyFile("some/random/file.txt")
	assert.Equal(t, db.FileKindOther, got)
}

// ── ComputeDiff ───────────────────────────────────────────────────────────────

func makeEntry(relPath, itemID string, mod time.Time, size int64) db.FileEntry {
	return db.FileEntry{
		RelPath: relPath,
		Kind:    db.ClassifyFile(relPath),
		Size:    size,
		ModTime: mod,
		ItemID:  itemID,
	}
}

func TestComputeDiff_DetectsAdded(t *testing.T) {
	now := time.Now()
	old := map[string]db.FileEntry{}
	current := map[string]db.FileEntry{
		"queue/037-F.md": makeEntry("queue/037-F.md", "037-F", now, 100),
	}

	diff := db.ComputeDiff(old, current)

	require.Len(t, diff.Added, 1)
	assert.Equal(t, "queue/037-F.md", diff.Added[0].RelPath)
	assert.Empty(t, diff.Changed)
	assert.Empty(t, diff.Deleted)
	assert.Empty(t, diff.Relocated)
}

func TestComputeDiff_DetectsChanged(t *testing.T) {
	base := time.Now()
	path := "queue/037-F.md"
	old := map[string]db.FileEntry{
		path: makeEntry(path, "037-F", base, 100),
	}
	current := map[string]db.FileEntry{
		path: makeEntry(path, "037-F", base.Add(time.Second), 200),
	}

	diff := db.ComputeDiff(old, current)

	assert.Empty(t, diff.Added)
	require.Len(t, diff.Changed, 1)
	assert.Equal(t, path, diff.Changed[0].RelPath)
	assert.Empty(t, diff.Deleted)
	assert.Empty(t, diff.Relocated)
}

func TestComputeDiff_DetectsDeleted(t *testing.T) {
	now := time.Now()
	path := "queue/037-F.md"
	old := map[string]db.FileEntry{
		path: makeEntry(path, "037-F", now, 100),
	}
	current := map[string]db.FileEntry{}

	diff := db.ComputeDiff(old, current)

	assert.Empty(t, diff.Added)
	assert.Empty(t, diff.Changed)
	require.Len(t, diff.Deleted, 1)
	assert.Equal(t, path, diff.Deleted[0].RelPath)
	assert.Empty(t, diff.Relocated)
}

func TestComputeDiff_DetectsRelocation(t *testing.T) {
	// Same ItemID in old (queue/) and current (done/) → relocation, not delete+add.
	now := time.Now()
	oldPath := "queue/037-F.md"
	newPath := "done/037-F.md"
	old := map[string]db.FileEntry{
		oldPath: makeEntry(oldPath, "037-F", now, 100),
	}
	current := map[string]db.FileEntry{
		newPath: makeEntry(newPath, "037-F", now.Add(time.Second), 100),
	}

	diff := db.ComputeDiff(old, current)

	assert.Empty(t, diff.Added, "relocation should not appear in Added")
	assert.Empty(t, diff.Deleted, "relocation should not appear in Deleted")
	require.Len(t, diff.Relocated, 1)
	assert.Equal(t, "037-F", diff.Relocated[0].ItemID)
	assert.Equal(t, oldPath, diff.Relocated[0].OldPath)
	assert.Equal(t, newPath, diff.Relocated[0].NewPath)
}

func TestComputeDiff_UnchangedFileNotReported(t *testing.T) {
	now := time.Now()
	path := "queue/037-F.md"
	entry := makeEntry(path, "037-F", now, 100)
	old := map[string]db.FileEntry{path: entry}
	current := map[string]db.FileEntry{path: entry} // identical

	diff := db.ComputeDiff(old, current)

	assert.Empty(t, diff.Added)
	assert.Empty(t, diff.Changed)
	assert.Empty(t, diff.Deleted)
	assert.Empty(t, diff.Relocated)
}

// ── ShouldFallback ────────────────────────────────────────────────────────────

func makeDiffWithChanges(n int) db.DiffResult {
	diff := db.DiffResult{}
	for i := 0; i < n; i++ {
		diff.Added = append(diff.Added, db.FileEntry{RelPath: "queue/X.md"})
	}
	return diff
}

func TestShouldFallback_BelowAbsoluteThreshold(t *testing.T) {
	diff := makeDiffWithChanges(49)
	ok, reason := db.ShouldFallback(diff, 200, 50)
	assert.False(t, ok, "49 changes < 50 threshold should not fall back")
	assert.Empty(t, reason)
}

func TestShouldFallback_AtAbsoluteThreshold(t *testing.T) {
	diff := makeDiffWithChanges(50)
	ok, reason := db.ShouldFallback(diff, 200, 50)
	assert.True(t, ok, "50 changes == 50 threshold should fall back")
	assert.NotEmpty(t, reason)
}

func TestShouldFallback_AboveAbsoluteThreshold(t *testing.T) {
	diff := makeDiffWithChanges(51)
	ok, reason := db.ShouldFallback(diff, 200, 50)
	assert.True(t, ok)
	assert.NotEmpty(t, reason)
}

func TestShouldFallback_BelowPercentThreshold(t *testing.T) {
	// 49 changes out of 100 manifest entries = 49% — below 50% threshold.
	diff := makeDiffWithChanges(49)
	ok, reason := db.ShouldFallback(diff, 100, 200)
	assert.False(t, ok)
	assert.Empty(t, reason)
}

func TestShouldFallback_AtPercentThreshold(t *testing.T) {
	// 50 changes out of 100 manifest entries = 50% — at threshold.
	diff := makeDiffWithChanges(50)
	ok, reason := db.ShouldFallback(diff, 100, 200)
	assert.True(t, ok)
	assert.NotEmpty(t, reason)
}

func TestShouldFallback_ZeroManifestSize_UsesAbsoluteOnly(t *testing.T) {
	// manifestSize=0 means the percent check cannot fire; only absolute applies.
	diff := makeDiffWithChanges(10)
	ok, _ := db.ShouldFallback(diff, 0, 50)
	assert.False(t, ok)
}

// ── BuildManifest ─────────────────────────────────────────────────────────────

func writeArtifactFile(t *testing.T, dir, relPath, content string) {
	t.Helper()
	full := filepath.Join(dir, relPath)
	require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
	require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
}

const minimalArtifact = `---
id: 037-F
title: Test feature
artifact_type: feature
status: queued
---
Body content.
`

func TestBuildManifest_PopulatesArtifactEntry(t *testing.T) {
	ws := t.TempDir()
	writeArtifactFile(t, ws, "queue/037-F.md", minimalArtifact)

	manifest, err := db.BuildManifest(ws)

	require.NoError(t, err)
	entry, ok := manifest["queue/037-F.md"]
	require.True(t, ok, "expected manifest entry for queue/037-F.md")
	assert.Equal(t, db.FileKindArtifact, entry.Kind)
	assert.Equal(t, "037-F", entry.ItemID, "ItemID must be extracted from frontmatter")
	assert.Positive(t, entry.Size)
	assert.False(t, entry.ModTime.IsZero())
}

func TestBuildManifest_PopulatesStashEntry(t *testing.T) {
	ws := t.TempDir()
	writeArtifactFile(t, ws, "stash.jsonl", `{"id":"AABBCCDD","text":"test","priority":"medium"}`)

	manifest, err := db.BuildManifest(ws)

	require.NoError(t, err)
	entry, ok := manifest["stash.jsonl"]
	require.True(t, ok)
	assert.Equal(t, db.FileKindStash, entry.Kind)
	assert.Empty(t, entry.ItemID, "stash files have no ItemID")
}

func TestBuildManifest_SkipsMalformedFrontmatter(t *testing.T) {
	ws := t.TempDir()
	writeArtifactFile(t, ws, "queue/bad.md", "no frontmatter here at all")

	manifest, err := db.BuildManifest(ws)

	require.NoError(t, err)
	entry, ok := manifest["queue/bad.md"]
	if ok {
		// If the file is included, ItemID must be empty (not a panic or error).
		assert.Empty(t, entry.ItemID, "malformed frontmatter must yield empty ItemID, not an error")
	}
}

func TestBuildManifest_EmptyWorkspace(t *testing.T) {
	ws := t.TempDir()

	manifest, err := db.BuildManifest(ws)

	require.NoError(t, err)
	assert.NotNil(t, manifest)
	assert.Empty(t, manifest)
}

// ── RehydrateWithManifest ─────────────────────────────────────────────────────

func TestRehydrateWithManifest_ReturnsSameCountAsRehydrate(t *testing.T) {
	ws := t.TempDir()
	writeArtifactFile(t, ws, "queue/037-F.md", minimalArtifact)

	database := setupTestDB(t)
	ctx := context.Background()

	// Baseline from Rehydrate.
	countA, err := db.Rehydrate(ctx, ws, database)
	require.NoError(t, err)

	// Reset and run RehydrateWithManifest.
	database2 := setupTestDB(t)
	countB, manifest, err := db.RehydrateWithManifest(ctx, ws, database2)
	require.NoError(t, err)

	assert.Equal(t, countA, countB, "RehydrateWithManifest must index the same count as Rehydrate")
	require.NotNil(t, manifest)
	assert.NotEmpty(t, manifest, "manifest must be populated after rehydration")
}

func TestRehydrateWithManifest_ManifestContainsArtifactEntry(t *testing.T) {
	ws := t.TempDir()
	writeArtifactFile(t, ws, "queue/037-F.md", minimalArtifact)

	database := setupTestDB(t)
	ctx := context.Background()

	_, manifest, err := db.RehydrateWithManifest(ctx, ws, database)
	require.NoError(t, err)

	entry, ok := manifest["queue/037-F.md"]
	require.True(t, ok, "manifest must contain an entry for the artifact file")
	assert.Equal(t, "037-F", entry.ItemID)
}
