package core_test

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
)

func TestAddArtifactLink_PersistsToMarkdownAndCache(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	source, err := core.CreateArtifact(ctx, ws, "Source feature", "feature")
	require.NoError(t, err)
	target, err := core.CreateArtifact(ctx, ws, "Target feature", "feature")
	require.NoError(t, err)

	require.NoError(t, core.AddArtifactLink(ctx, ws, source.ID, target.ID, "related_to"))

	filePath, err := core.FindArtifactPath(ctx, ws, source.ID)
	require.NoError(t, err)
	raw, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "links:")
	assert.Contains(t, string(raw), "target_id: "+target.ID)
	assert.Contains(t, string(raw), "link_type: related_to")

	links, err := db.GetLinks(ctx, ws.DB, source.ID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, target.ID, links[0].TargetID)
	assert.Equal(t, "related_to", links[0].LinkType)
}

// TestAddLinkDurable_InvalidLinkType_ReturnsError verifies that AddArtifactLink
// rejects unknown link types and does not mutate Markdown or SQLite.
func TestAddLinkDurable_InvalidLinkType_ReturnsError(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	source, err := core.CreateArtifact(ctx, ws, "Source feature", "feature")
	require.NoError(t, err)
	target, err := core.CreateArtifact(ctx, ws, "Target feature", "feature")
	require.NoError(t, err)

	err = core.AddArtifactLink(ctx, ws, source.ID, target.ID, "not_a_valid_type")
	require.Error(t, err)

	filePath, pathErr := core.FindArtifactPath(ctx, ws, source.ID)
	require.NoError(t, pathErr)
	raw, readErr := os.ReadFile(filePath)
	require.NoError(t, readErr)
	assert.NotContains(t, string(raw), "not_a_valid_type")

	links, dbErr := db.GetLinks(ctx, ws.DB, source.ID)
	require.NoError(t, dbErr)
	assert.Empty(t, links)
}

func TestRehydrate_RebuildsLinksFromMarkdown(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	source, err := core.CreateArtifact(ctx, ws, "Source feature", "feature")
	require.NoError(t, err)
	target, err := core.CreateArtifact(ctx, ws, "Target feature", "feature")
	require.NoError(t, err)

	require.NoError(t, core.AddArtifactLink(ctx, ws, source.ID, target.ID, "informs"))
	_, err = ws.DB.ExecContext(ctx, `DELETE FROM item_links`)
	require.NoError(t, err)

	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)

	links, err := db.GetLinks(ctx, ws.DB, source.ID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, target.ID, links[0].TargetID)
	assert.Equal(t, "informs", links[0].LinkType)
}

func TestRemoveArtifactLink_RemovesFromMarkdownAndCache(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	source, err := core.CreateArtifact(ctx, ws, "Source feature", "feature")
	require.NoError(t, err)
	target, err := core.CreateArtifact(ctx, ws, "Target feature", "feature")
	require.NoError(t, err)

	require.NoError(t, core.AddArtifactLink(ctx, ws, source.ID, target.ID, "supersedes"))
	require.NoError(t, core.RemoveArtifactLink(ctx, ws, source.ID, target.ID, "supersedes"))

	filePath, err := core.FindArtifactPath(ctx, ws, source.ID)
	require.NoError(t, err)
	raw, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.False(t, strings.Contains(string(raw), target.ID))

	links, err := db.GetLinks(ctx, ws.DB, source.ID)
	require.NoError(t, err)
	assert.Empty(t, links)
}

// TestLinkDurability_AddLink_DeleteDB_Rehydrate_LinkSurvives confirms that a
// link added via AddArtifactLink survives a full rehydration from disk.
func TestLinkDurability_AddLink_DeleteDB_Rehydrate_LinkSurvives(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	source, err := core.CreateArtifact(ctx, ws, "Durable source", "feature")
	require.NoError(t, err)
	target, err := core.CreateArtifact(ctx, ws, "Durable target", "feature")
	require.NoError(t, err)

	require.NoError(t, core.AddArtifactLink(ctx, ws, source.ID, target.ID, "spike_ref"))

	_, err = ws.DB.ExecContext(ctx, `DELETE FROM items`)
	require.NoError(t, err)
	_, err = ws.DB.ExecContext(ctx, `DELETE FROM item_links`)
	require.NoError(t, err)

	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)

	links, err := db.GetLinks(ctx, ws.DB, source.ID)
	require.NoError(t, err)
	require.Len(t, links, 1)
	assert.Equal(t, target.ID, links[0].TargetID)
	assert.Equal(t, "spike_ref", links[0].LinkType)
}

// TestLinkDurability_RemoveLink_Rehydrate_LinkGone confirms that a link
// removed via RemoveArtifactLink stays removed after rehydration.
func TestLinkDurability_RemoveLink_Rehydrate_LinkGone(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	source, err := core.CreateArtifact(ctx, ws, "Remove source", "feature")
	require.NoError(t, err)
	target, err := core.CreateArtifact(ctx, ws, "Remove target", "feature")
	require.NoError(t, err)

	require.NoError(t, core.AddArtifactLink(ctx, ws, source.ID, target.ID, "duplicate_of"))
	require.NoError(t, core.RemoveArtifactLink(ctx, ws, source.ID, target.ID, "duplicate_of"))

	_, err = db.Rehydrate(ctx, core.WorkspaceStorageRoot(ws.RootPath), ws.DB)
	require.NoError(t, err)

	links, err := db.GetLinks(ctx, ws.DB, source.ID)
	require.NoError(t, err)
	assert.Empty(t, links)
}

// TestMigrateDBOnlyLinks_WritesLinksToMarkdown verifies that links stored only
// in SQLite are written to the source artifact's Markdown frontmatter.
func TestMigrateDBOnlyLinks_WritesLinksToMarkdown(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	source, err := core.CreateArtifact(ctx, ws, "Migration source", "feature")
	require.NoError(t, err)
	target, err := core.CreateArtifact(ctx, ws, "Migration target", "feature")
	require.NoError(t, err)

	require.NoError(t, db.AddLink(ctx, ws.DB, source.ID, target.ID, "related_to"))

	filePath, err := core.FindArtifactPath(ctx, ws, source.ID)
	require.NoError(t, err)
	raw, err := os.ReadFile(filePath)
	require.NoError(t, err)
	assert.NotContains(t, string(raw), "related_to")

	result, err := core.MigrateDBOnlyLinks(ctx, ws)
	require.NoError(t, err)
	assert.Equal(t, 1, result.Written)
	assert.Zero(t, result.Skipped)

	raw, err = os.ReadFile(filePath)
	require.NoError(t, err)
	assert.Contains(t, string(raw), "target_id: "+target.ID)
	assert.Contains(t, string(raw), "link_type: related_to")
}

// TestMigrateDBOnlyLinks_Idempotent verifies that running the guard twice does
// not duplicate link entries in Markdown frontmatter.
func TestMigrateDBOnlyLinks_Idempotent(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	source, err := core.CreateArtifact(ctx, ws, "Idempotent source", "feature")
	require.NoError(t, err)
	target, err := core.CreateArtifact(ctx, ws, "Idempotent target", "feature")
	require.NoError(t, err)

	require.NoError(t, db.AddLink(ctx, ws.DB, source.ID, target.ID, "informs"))

	_, err = core.MigrateDBOnlyLinks(ctx, ws)
	require.NoError(t, err)

	result2, err := core.MigrateDBOnlyLinks(ctx, ws)
	require.NoError(t, err)
	assert.Zero(t, result2.Written)
}

// TestMigrateDBOnlyLinks_SourceNotOnDisk_Skipped confirms that a DB-only link
// whose source artifact cannot be found on disk is counted as skipped.
func TestMigrateDBOnlyLinks_SourceNotOnDisk_Skipped(t *testing.T) {
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	_, err := ws.DB.ExecContext(
		ctx,
		`INSERT OR IGNORE INTO item_links (source_id, target_id, link_type) VALUES (?, ?, ?)`,
		"GHOST-001", "GHOST-002", "related_to",
	)
	require.NoError(t, err)

	result, err := core.MigrateDBOnlyLinks(ctx, ws)
	require.NoError(t, err)
	assert.Zero(t, result.Written)
	assert.Equal(t, 1, result.Skipped)
}
