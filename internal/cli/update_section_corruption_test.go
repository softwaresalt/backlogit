package cli

// 062.004-T: Stop CLI section corruption fallback (stash 51D7384A).
//
// The CLI `update --section` path must surface an error when section markers are
// malformed (a BEGIN tag with no matching END tag) instead of blindly appending
// a fresh section block. Before the fix, any error from parser.WriteSections —
// including a structural/malformed-marker error — was swallowed and ALL requested
// sections were appended, which both hid the corruption and duplicated existing
// section blocks.

import (
	"context"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/core"
	"github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/models"
)

// writeRawArtifact replaces the on-disk body of an artifact with the given body
// while preserving its frontmatter.
func writeRawArtifact(t *testing.T, ws *core.Workspace, id, body string) string {
	t.Helper()
	ctx := context.Background()
	filePath, err := core.FindArtifactPath(ctx, ws, id)
	require.NoError(t, err)
	raw, err := os.ReadFile(filePath)
	require.NoError(t, err)
	fm, _, err := models.ParseFrontmatter(string(raw))
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filePath, []byte(models.SerializeFrontmatter(fm, body)), 0o644))
	return filePath
}

// A malformed section marker (BEGIN with no END) must produce an error, and the
// CLI must NOT append a duplicate block that masks the corruption.
func TestUpdateCommand_MalformedSectionMarker_ReturnsError(t *testing.T) {
	root, ws := setupUpdateTestWorkspace(t)
	ctx := context.Background()

	feat, err := core.CreateArtifact(ctx, ws, "Corruption feature", "feature")
	require.NoError(t, err)
	artifact, err := core.CreateArtifact(ctx, ws, "Corruption task", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	require.NoError(t, db.UpsertItem(ctx, ws.DB, artifact))

	// Malformed: a BEGIN:description tag with no matching END tag.
	malformed := "Intro paragraph.\n\n<!-- BEGIN:description -->\nOriginal description content\n"
	filePath := writeRawArtifact(t, ws, artifact.ID, malformed)

	cwd := root
	cmd := newUpdateCommand(&cwd)
	cmd.SetArgs([]string{artifact.ID, "--section", "description=New description"})
	err = cmd.Execute()
	require.Error(t, err, "malformed section markers must return an error, not silently append")

	raw, readErr := os.ReadFile(filePath)
	require.NoError(t, readErr)
	content := string(raw)
	beginCount := strings.Count(content, "<!-- BEGIN:description -->")
	assert.Equal(t, 1, beginCount,
		"a malformed section must not be duplicated by an append fallback")
}

// When one requested section exists and another does not, only the missing one
// is appended; the existing section must not be duplicated.
func TestUpdateCommand_MixedExistingAndMissing_NoDuplicate(t *testing.T) {
	root, ws := setupUpdateTestWorkspace(t)
	ctx := context.Background()

	feat, err := core.CreateArtifact(ctx, ws, "Mixed feature", "feature")
	require.NoError(t, err)
	artifact, err := core.CreateArtifact(ctx, ws, "Mixed task", "task", core.WithParent(feat.ID))
	require.NoError(t, err)
	require.NoError(t, db.UpsertItem(ctx, ws.DB, artifact))

	// Well-formed existing "description" section; "notes" is absent.
	body := "Intro.\n\n<!-- BEGIN:description -->\nExisting description\n<!-- END:description -->\n"
	filePath := writeRawArtifact(t, ws, artifact.ID, body)

	cwd := root
	cmd := newUpdateCommand(&cwd)
	cmd.SetArgs([]string{
		artifact.ID,
		"--section", "description=Updated description",
		"--section", "notes=Fresh notes",
	})
	err = cmd.Execute()
	require.NoError(t, err, "updating an existing section and adding a new one must succeed")

	raw, readErr := os.ReadFile(filePath)
	require.NoError(t, readErr)
	content := string(raw)

	assert.Equal(t, 1, strings.Count(content, "<!-- BEGIN:description -->"),
		"existing section must be updated in place, not duplicated")
	assert.Contains(t, content, "Updated description", "existing section should be updated")
	assert.NotContains(t, content, "Existing description", "old content should be replaced")
	assert.Contains(t, content, "<!-- BEGIN:notes -->", "missing section should be appended")
	assert.Contains(t, content, "Fresh notes", "appended section content should be present")
}
