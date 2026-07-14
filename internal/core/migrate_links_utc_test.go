package core_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
)

// TestMigrateDBOnlyLinks_EmitsUTCUpdatedAt proves that migrating a DB-only link
// into a source artifact's Markdown frontmatter restamps updated_at in canonical
// UTC even under a non-UTC local zone (site: migrate_links.go MigrateDBOnlyLinks).
func TestMigrateDBOnlyLinks_EmitsUTCUpdatedAt(t *testing.T) {
	withNonUTCLocal(t)
	ws := setupTestWorkspace(t)
	ctx := context.Background()

	source, err := core.CreateArtifact(ctx, ws, "Migrate source feature", "feature")
	require.NoError(t, err)
	target, err := core.CreateArtifact(ctx, ws, "Migrate target feature", "feature")
	require.NoError(t, err)

	// Seed a DB-only link (present in item_links, absent from Markdown frontmatter).
	require.NoError(t, db.AddLink(ctx, ws.DB, source.ID, target.ID, "related_to"))

	result, err := core.MigrateDBOnlyLinks(ctx, ws)
	require.NoError(t, err)
	require.GreaterOrEqual(t, result.Written, 1)

	assertFrontmatterUTC(t, readArtifactContent(t, ctx, ws, source.ID), "updated_at")
}
