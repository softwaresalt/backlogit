package db_test

// 042.001-T: Validate link types during rehydration and merge_sync
//
// Validation contract after fix:
//   - Rehydration with an artifact whose frontmatter contains an invalid
//     link_type must NOT insert that link into item_links.
//   - Valid link types must still be indexed correctly.
//   - MergeSync must apply the same validation gate for changed artifacts.
//
// Root cause: internal/db/rehydration.go inserts item_links via raw SQL,
// bypassing the isValidLinkType check enforced by db.AddLink. The fix routes
// all link inserts during rehydration through the validation gate.

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/db"
)

// writeArtifactWithLinks writes a Markdown artifact file whose frontmatter includes
// a links list. Used to seed rehydration scenarios.
func writeArtifactWithLinks(t *testing.T, dir, id, title, status string, links []map[string]string) string {
	t.Helper()
	var linksYAML string
	if len(links) > 0 {
		linksYAML = "links:\n"
		for _, l := range links {
			linksYAML += "  - target_id: " + l["target_id"] + "\n    link_type: " + l["link_type"] + "\n"
		}
	}
	content := "---\nid: " + id + "\ntitle: " + title + "\nstatus: " + status + "\nartifact_type: task\n" + linksYAML + "---\nBody\n"
	path := filepath.Join(dir, id+".md")
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
	return path
}

// TestRehydrate_InvalidLinkType_NotIndexed verifies that an artifact with an
// invalid link_type in frontmatter does not cause that link to appear in the
// item_links table after rehydration.
func TestRehydrate_InvalidLinkType_NotIndexed(t *testing.T) {
	tmpDir := t.TempDir()
	queueDir := filepath.Join(tmpDir, "queue")
	require.NoError(t, os.MkdirAll(queueDir, 0o755))

	// Write artifact with an invalid link type.
	writeArtifactWithLinks(t, queueDir, "L001-T", "Source", "queued", []map[string]string{
		{"target_id": "L002-T", "link_type": "invalid_type"},
	})
	// Write target artifact (no links).
	writeArtifactWithLinks(t, queueDir, "L002-T", "Target", "queued", nil)

	database, err := db.Open(filepath.Join(tmpDir, "backlogit.db"))
	require.NoError(t, err)
	defer database.Close()
	require.NoError(t, db.EnsureSchema(database))

	_, rehydErr := db.Rehydrate(context.Background(), tmpDir, database)
	require.NoError(t, rehydErr)

	// The invalid link must NOT appear in item_links.
	links, linksErr := db.GetLinks(context.Background(), database, "L001-T")
	require.NoError(t, linksErr)
	for _, l := range links {
		assert.NotEqual(t, "invalid_type", l.LinkType,
			"invalid link_type must not be persisted during rehydration")
	}
}

// TestRehydrate_ValidLinkType_Indexed verifies that artifacts with valid link
// types are still indexed correctly after the validation gate is in place.
func TestRehydrate_ValidLinkType_Indexed(t *testing.T) {
	tmpDir := t.TempDir()
	queueDir := filepath.Join(tmpDir, "queue")
	require.NoError(t, os.MkdirAll(queueDir, 0o755))

	writeArtifactWithLinks(t, queueDir, "V001-T", "Source", "queued", []map[string]string{
		{"target_id": "V002-T", "link_type": "related_to"},
	})
	writeArtifactWithLinks(t, queueDir, "V002-T", "Target", "queued", nil)

	database, err := db.Open(filepath.Join(tmpDir, "backlogit.db"))
	require.NoError(t, err)
	defer database.Close()
	require.NoError(t, db.EnsureSchema(database))

	_, rehydErr := db.Rehydrate(context.Background(), tmpDir, database)
	require.NoError(t, rehydErr)

	links, linksErr := db.GetLinks(context.Background(), database, "V001-T")
	require.NoError(t, linksErr)

	found := false
	for _, l := range links {
		if l.TargetID == "V002-T" && l.LinkType == "related_to" {
			found = true
		}
	}
	assert.True(t, found, "valid link type must be indexed during rehydration")
}

// TestRehydrate_MixedLinkTypes_OnlyValidIndexed verifies that when an artifact
// has both valid and invalid link types in frontmatter, only the valid entries
// are persisted.
func TestRehydrate_MixedLinkTypes_OnlyValidIndexed(t *testing.T) {
	tmpDir := t.TempDir()
	queueDir := filepath.Join(tmpDir, "queue")
	require.NoError(t, os.MkdirAll(queueDir, 0o755))

	writeArtifactWithLinks(t, queueDir, "M001-T", "Mixed source", "queued", []map[string]string{
		{"target_id": "M002-T", "link_type": "informs"},
		{"target_id": "M003-T", "link_type": "bogus_link"},
		{"target_id": "M004-T", "link_type": "supersedes"},
	})
	for _, id := range []string{"M002-T", "M003-T", "M004-T"} {
		writeArtifactWithLinks(t, queueDir, id, id, "queued", nil)
	}

	database, err := db.Open(filepath.Join(tmpDir, "backlogit.db"))
	require.NoError(t, err)
	defer database.Close()
	require.NoError(t, db.EnsureSchema(database))

	_, rehydErr := db.Rehydrate(context.Background(), tmpDir, database)
	require.NoError(t, rehydErr)

	links, linksErr := db.GetLinks(context.Background(), database, "M001-T")
	require.NoError(t, linksErr)

	linkMap := make(map[string]string, len(links))
	for _, l := range links {
		linkMap[l.TargetID] = l.LinkType
	}

	assert.Equal(t, "informs", linkMap["M002-T"], "informs link must be indexed")
	assert.Equal(t, "supersedes", linkMap["M004-T"], "supersedes link must be indexed")
	assert.NotContains(t, linkMap, "M003-T", "bogus_link target must not be indexed")
}

// TestMergeSync_InvalidLinkType_NotIndexed verifies that MergeSync applies the
// same validation gate when processing changed artifacts incrementally.
func TestMergeSync_InvalidLinkType_NotIndexed(t *testing.T) {
	tmpDir := t.TempDir()
	queueDir := filepath.Join(tmpDir, "queue")
	require.NoError(t, os.MkdirAll(queueDir, 0o755))

	// Seed without links first, then add an invalid link via MergeSync.
	writeArtifactWithLinks(t, queueDir, "S001-T", "Sync source", "queued", nil)
	writeArtifactWithLinks(t, queueDir, "S002-T", "Sync target", "queued", nil)

	database, err := db.Open(filepath.Join(tmpDir, "backlogit.db"))
	require.NoError(t, err)
	defer database.Close()
	require.NoError(t, db.EnsureSchema(database))

	_, manifest, rehydErr := db.RehydrateWithManifest(context.Background(), tmpDir, database)
	require.NoError(t, rehydErr)

	// Overwrite S001-T with an invalid link type.
	writeArtifactWithLinks(t, queueDir, "S001-T", "Sync source", "queued", []map[string]string{
		{"target_id": "S002-T", "link_type": "not_a_valid_type"},
	})

	_, _, syncErr := db.MergeSync(context.Background(), tmpDir, database, manifest, false)
	require.NoError(t, syncErr)

	links, linksErr := db.GetLinks(context.Background(), database, "S001-T")
	require.NoError(t, linksErr)
	for _, l := range links {
		assert.NotEqual(t, "not_a_valid_type", l.LinkType,
			"invalid link_type must not be persisted by MergeSync")
	}
}
