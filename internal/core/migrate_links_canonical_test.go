package core

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/config"
	"github.com/softwaresalt/backlogit/internal/db"
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
