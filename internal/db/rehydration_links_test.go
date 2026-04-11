package db_test

// 026.005-T: Rebuild links during rehydration from Markdown.
//
// These tests verify that:
//
//   - item_links is cleared and rebuilt from Markdown frontmatter on each rehydration
//   - Links written in Markdown frontmatter appear in SQLite after rehydration
//   - DB-only links (not in Markdown) are not reinstated after rehydration

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/backlogit/backlogit/internal/db"
)

// TestRehydrate_LinksFromFrontmatter verifies that link entries present in
// Markdown frontmatter are inserted into the item_links table after rehydration.
func TestRehydrate_LinksFromFrontmatter(t *testing.T) {
	ws := t.TempDir()
	queueDir := filepath.Join(ws, "queue")
	require.NoError(t, os.MkdirAll(queueDir, 0o755))

	// Two artifacts: source has a links block pointing to target.
	src := `---
id: SRC-001
title: Source artifact
status: queued
artifact_type: task
links:
  - target_id: TGT-001
    link_type: related_to
  - target_id: TGT-001
    link_type: informs
---

Source body`

	tgt := `---
id: TGT-001
title: Target artifact
status: queued
artifact_type: task
---

Target body`

	require.NoError(t, os.WriteFile(filepath.Join(queueDir, "SRC-001.md"), []byte(src), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(queueDir, "TGT-001.md"), []byte(tgt), 0o644))

	database := setupTestDB(t)
	ctx := context.Background()

	count, err := db.Rehydrate(ctx, ws, database)
	require.NoError(t, err)
	assert.Equal(t, 2, count)

	links, err := db.GetLinks(ctx, database, "SRC-001")
	require.NoError(t, err)
	require.Len(t, links, 2, "both links from frontmatter must appear in SQLite")

	linkTypes := make(map[string]bool, len(links))
	for _, l := range links {
		assert.Equal(t, "SRC-001", l.SourceID)
		assert.Equal(t, "TGT-001", l.TargetID)
		linkTypes[l.LinkType] = true
	}
	assert.True(t, linkTypes["related_to"], "related_to link must be indexed")
	assert.True(t, linkTypes["informs"], "informs link must be indexed")
}

// TestRehydrate_ClearsAndRebuildsLinks confirms that item_links is cleared on
// each rehydration and rebuilt exclusively from Markdown frontmatter. DB-only
// links (those injected directly into SQLite but absent from Markdown) must not
// survive the rehydration cycle.
func TestRehydrate_ClearsAndRebuildsLinks(t *testing.T) {
	ws := t.TempDir()
	queueDir := filepath.Join(ws, "queue")
	require.NoError(t, os.MkdirAll(queueDir, 0o755))

	src := `---
id: A001
title: Artifact A
status: queued
artifact_type: task
links:
  - target_id: B001
    link_type: supersedes
---

Body A`

	other := `---
id: B001
title: Artifact B
status: queued
artifact_type: task
---

Body B`

	require.NoError(t, os.WriteFile(filepath.Join(queueDir, "A001.md"), []byte(src), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(queueDir, "B001.md"), []byte(other), 0o644))

	database := setupTestDB(t)
	ctx := context.Background()

	// Initial rehydration — A001→B001 (supersedes) from Markdown.
	_, err := db.Rehydrate(ctx, ws, database)
	require.NoError(t, err)

	// Inject a DB-only link that has no corresponding Markdown entry.
	_, err = database.ExecContext(ctx,
		`INSERT OR IGNORE INTO item_links (source_id, target_id, link_type) VALUES (?, ?, ?)`,
		"A001", "B001", "duplicate_of",
	)
	require.NoError(t, err)

	// Rehydrate again — item_links must be cleared and rebuilt from Markdown only.
	_, err = db.Rehydrate(ctx, ws, database)
	require.NoError(t, err)

	links, err := db.GetLinks(ctx, database, "A001")
	require.NoError(t, err)
	require.Len(t, links, 1, "only the Markdown-sourced link must survive rehydration")
	assert.Equal(t, "supersedes", links[0].LinkType,
		"the Markdown link must be the survivor; DB-only link must be dropped")
}

// TestRehydrate_NoLinks_ItemLinksTableEmpty verifies that an artifact with no
// links in its frontmatter leaves no rows in item_links for that source ID.
func TestRehydrate_NoLinks_ItemLinksTableEmpty(t *testing.T) {
	ws := t.TempDir()
	queueDir := filepath.Join(ws, "queue")
	require.NoError(t, os.MkdirAll(queueDir, 0o755))

	md := `---
id: NOLINK-001
title: No links artifact
status: queued
artifact_type: task
---

No links here`

	require.NoError(t, os.WriteFile(filepath.Join(queueDir, "NOLINK-001.md"), []byte(md), 0o644))

	database := setupTestDB(t)
	ctx := context.Background()

	_, err := db.Rehydrate(ctx, ws, database)
	require.NoError(t, err)

	links, err := db.GetLinks(ctx, database, "NOLINK-001")
	require.NoError(t, err)
	assert.Empty(t, links, "artifact with no links frontmatter must yield no item_links rows")
}
