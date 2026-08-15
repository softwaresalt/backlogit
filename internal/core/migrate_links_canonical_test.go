package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

func TestMigrateDBOnlyLinksBuildsOneCanonicalIndex(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	storageRoot := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(storageRoot, 0o755))
	require.NoError(t, config.WriteDefaults(storageRoot))

	ws, err := NewWorkspace(ctx, root)
	require.NoError(t, err)
	defer ws.Close()

	feature, err := CreateArtifact(ctx, ws, "Canonical migration feature", "feature")
	require.NoError(t, err)
	target, err := CreateArtifact(ctx, ws, "Canonical migration target", "task", WithParent(feature.ID))
	require.NoError(t, err)
	for i := 0; i < 8; i++ {
		source, createErr := CreateArtifact(ctx, ws, "Canonical migration source", "task", WithParent(feature.ID))
		require.NoError(t, createErr)
		require.NoError(t, db.AddLink(ctx, ws.DB, source.ID, target.ID, "informs"))
	}

	result, err := MigrateDBOnlyLinks(ctx, ws)

	require.NoError(t, err)
	assert.Equal(t, 1, result.IndexBuilds)
	assert.Equal(t, 10, result.Indexed)
	assert.Equal(t, 8, result.Written)
	assert.GreaterOrEqual(t, result.DurationMS, int64(0))
}

func TestMigrateDBOnlyLinksRejectsDuplicateArtifactIDs(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	storageRoot := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(storageRoot, 0o755))
	require.NoError(t, config.WriteDefaults(storageRoot))

	ws, err := NewWorkspace(ctx, root)
	require.NoError(t, err)
	defer ws.Close()

	artifact, err := CreateArtifact(ctx, ws, "Duplicate migration artifact", "feature")
	require.NoError(t, err)
	target, err := CreateArtifact(ctx, ws, "Duplicate migration target", "feature")
	require.NoError(t, err)
	sourcePath, err := FindArtifactPath(ctx, ws, artifact.ID)
	require.NoError(t, err)
	archiveDir := filepath.Join(storageRoot, "archive")
	require.NoError(t, os.MkdirAll(archiveDir, 0o755))
	data, err := os.ReadFile(sourcePath)
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(archiveDir, filepath.Base(sourcePath)), data, 0o644))
	require.NoError(t, db.AddLink(ctx, ws.DB, artifact.ID, target.ID, "informs"))

	_, err = MigrateDBOnlyLinks(ctx, ws)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "duplicate artifact ID")
}

func TestMigrateDBOnlyLinksCountsUnresolvedSourceAsSkipped(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	storageRoot := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(storageRoot, 0o755))
	require.NoError(t, config.WriteDefaults(storageRoot))

	ws, err := NewWorkspace(ctx, root)
	require.NoError(t, err)
	defer ws.Close()

	require.NoError(t, db.AddLink(ctx, ws.DB, "999-T", "998-T", "informs"))
	result, err := MigrateDBOnlyLinks(ctx, ws)

	require.NoError(t, err)
	assert.Equal(t, 0, result.Written)
	assert.Equal(t, 1, result.Skipped)
}

func TestNewWorkspaceDefersDBOnlyLinkMigration(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	storageRoot := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(storageRoot, 0o755))
	require.NoError(t, config.WriteDefaults(storageRoot))

	ws, err := NewWorkspace(ctx, root)
	require.NoError(t, err)
	feature, err := CreateArtifact(ctx, ws, "Deferred migration feature", "feature")
	require.NoError(t, err)
	source, err := CreateArtifact(ctx, ws, "Deferred migration source", "task", WithParent(feature.ID))
	require.NoError(t, err)
	target, err := CreateArtifact(ctx, ws, "Deferred migration target", "task", WithParent(feature.ID))
	require.NoError(t, err)
	require.NoError(t, db.AddLink(ctx, ws.DB, source.ID, target.ID, "informs"))
	require.NoError(t, ws.Close())

	ws, err = NewWorkspace(ctx, root)
	require.NoError(t, err)
	defer ws.Close()

	path, err := FindArtifactPath(ctx, ws, source.ID)
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	frontmatter, body, err := models.ParseFrontmatter(string(data))
	require.NoError(t, err)
	artifact, err := models.ArtifactFromFrontmatter(frontmatter, body)
	require.NoError(t, err)
	assert.Empty(t, artifact.Links)

	_, err = UpdateArtifact(ctx, ws, source.ID, map[string]any{"title": "Updated source"})
	require.NoError(t, err)
	data, err = os.ReadFile(path)
	require.NoError(t, err)
	frontmatter, body, err = models.ParseFrontmatter(string(data))
	require.NoError(t, err)
	artifact, err = models.ArtifactFromFrontmatter(frontmatter, body)
	require.NoError(t, err)
	require.Len(t, artifact.Links, 1)
	assert.Equal(t, target.ID, artifact.Links[0].TargetID)
	assert.Equal(t, "informs", artifact.Links[0].LinkType)
}

func BenchmarkMigrateDBOnlyLinksCanonicalIndex(b *testing.B) {
	ctx := context.Background()
	root := b.TempDir()
	storageRoot := filepath.Join(root, ".backlogit")
	if err := os.MkdirAll(storageRoot, 0o755); err != nil {
		b.Fatal(err)
	}
	if err := config.WriteDefaults(storageRoot); err != nil {
		b.Fatal(err)
	}
	ws, err := NewWorkspace(ctx, root)
	if err != nil {
		b.Fatal(err)
	}
	defer ws.Close()

	feature, err := CreateArtifact(ctx, ws, "Benchmark feature", "feature")
	if err != nil {
		b.Fatal(err)
	}
	target, err := CreateArtifact(ctx, ws, "Benchmark target", "task", WithParent(feature.ID))
	if err != nil {
		b.Fatal(err)
	}
	sourcePaths := make([]string, 0, 32)
	sourceBytes := make([][]byte, 0, 32)
	for i := 0; i < 32; i++ {
		source, createErr := CreateArtifact(ctx, ws, "Benchmark source", "task", WithParent(feature.ID))
		if createErr != nil {
			b.Fatal(createErr)
		}
		path, pathErr := FindArtifactPath(ctx, ws, source.ID)
		if pathErr != nil {
			b.Fatal(pathErr)
		}
		data, readErr := os.ReadFile(path)
		if readErr != nil {
			b.Fatal(readErr)
		}
		sourcePaths = append(sourcePaths, path)
		sourceBytes = append(sourceBytes, data)
		if linkErr := db.AddLink(ctx, ws.DB, source.ID, target.ID, "informs"); linkErr != nil {
			b.Fatal(linkErr)
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		b.StopTimer()
		for j, path := range sourcePaths {
			if writeErr := os.WriteFile(path, sourceBytes[j], 0o644); writeErr != nil {
				b.Fatal(writeErr)
			}
		}
		b.StartTimer()
		if _, migrateErr := MigrateDBOnlyLinks(ctx, ws); migrateErr != nil {
			b.Fatal(migrateErr)
		}
	}
}

func TestRemoveArtifactLinkDeletesDBOnlyLink(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	storageRoot := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(storageRoot, 0o755))
	require.NoError(t, config.WriteDefaults(storageRoot))

	ws, err := NewWorkspace(ctx, root)
	require.NoError(t, err)
	defer ws.Close()

	source, err := CreateArtifact(ctx, ws, "DB-only removal source", "feature")
	require.NoError(t, err)
	target, err := CreateArtifact(ctx, ws, "DB-only removal target", "feature")
	require.NoError(t, err)
	require.NoError(t, db.AddLink(ctx, ws.DB, source.ID, target.ID, "informs"))

	require.NoError(t, RemoveArtifactLink(ctx, ws, source.ID, target.ID, "informs"))

	links, err := db.GetLinks(ctx, ws.DB, source.ID)
	require.NoError(t, err)
	assert.Empty(t, links)
}

func TestRemoveArtifactLinkKeepsCacheWhenMarkdownWriteIsNotApplied(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	storageRoot := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(storageRoot, 0o755))
	require.NoError(t, config.WriteDefaults(storageRoot))

	ws, err := NewWorkspace(ctx, root)
	require.NoError(t, err)
	defer ws.Close()

	source, err := CreateArtifact(ctx, ws, "Write failure source", "feature")
	require.NoError(t, err)
	target, err := CreateArtifact(ctx, ws, "Write failure target", "feature")
	require.NoError(t, err)
	require.NoError(t, AddArtifactLink(ctx, ws, source.ID, target.ID, "informs"))

	originalWriter := persistArtifactWriteFn
	persistArtifactWriteFn = func(*models.Artifact, string, bool) error {
		return fmt.Errorf("simulated markdown write failure: %w", blerrors.ErrWriteNotApplied)
	}
	t.Cleanup(func() { persistArtifactWriteFn = originalWriter })

	err = RemoveArtifactLink(ctx, ws, source.ID, target.ID, "informs")

	require.Error(t, err)
	assert.ErrorIs(t, err, blerrors.ErrWriteNotApplied)
	links, err := db.GetLinks(ctx, ws.DB, source.ID)
	require.NoError(t, err)
	assert.Len(t, links, 1)
}

func TestMigrateDBOnlyLinksFailsClosedOnParseFailure(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	storageRoot := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(filepath.Join(storageRoot, "queue"), 0o755))
	require.NoError(t, config.WriteDefaults(storageRoot))

	ws, err := NewWorkspace(ctx, root)
	require.NoError(t, err)
	defer ws.Close()

	target, err := CreateArtifact(ctx, ws, "Parse failure target", "feature")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(
		filepath.Join(storageRoot, "queue", "malformed.md"),
		[]byte("---\nid: [malformed\n---\n"),
		0o644,
	))
	require.NoError(t, db.AddLink(ctx, ws.DB, "malformed", target.ID, "informs"))

	_, err = MigrateDBOnlyLinks(ctx, ws)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse")
}

func TestMigrateDBOnlyLinksIgnoresUnlinkedDuplicateIDs(t *testing.T) {
	ctx := context.Background()
	root := t.TempDir()
	storageRoot := filepath.Join(root, ".backlogit")
	require.NoError(t, os.MkdirAll(storageRoot, 0o755))
	require.NoError(t, config.WriteDefaults(storageRoot))

	ws, err := NewWorkspace(ctx, root)
	require.NoError(t, err)
	defer ws.Close()

	artifact, err := CreateArtifact(ctx, ws, "Unlinked duplicate artifact", "feature")
	require.NoError(t, err)
	path, err := FindArtifactPath(ctx, ws, artifact.ID)
	require.NoError(t, err)
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	archiveDir := filepath.Join(storageRoot, "archive")
	require.NoError(t, os.MkdirAll(archiveDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(archiveDir, filepath.Base(path)), data, 0o644))

	_, err = MigrateDBOnlyLinks(ctx, ws)
	require.NoError(t, err)
}
