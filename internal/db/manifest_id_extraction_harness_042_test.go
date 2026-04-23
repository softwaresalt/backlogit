package db_test

// 042.002-T: Fix manifest ID extraction lossy 512-byte scan
//
// Correctness contract after fix:
//   - BuildManifest must extract the ItemID for any artifact whose frontmatter
//     exceeds 512 bytes, regardless of where the id: field appears.
//   - Relocation detection in ComputeDiff must work correctly for such
//     artifacts (same ItemID detected across path changes).
//   - The 512-byte truncation must not silently return an empty ItemID.
//
// Root cause: extractItemIDFromFrontmatter reads only the first 512 bytes and
// uses string-prefix matching. Artifacts with large custom_fields or many
// label entries can push the id: field beyond the 512-byte boundary.

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/db"
)

// writeLargeFrontmatterArtifact writes an artifact file where the id: field
// appears after at least minOffset bytes of frontmatter. It pads the
// frontmatter using a description field before the id field.
func writeLargeFrontmatterArtifact(t *testing.T, path, id string, padBytes int) {
	t.Helper()
	// Build a padding block large enough that id: is beyond the threshold.
	padding := strings.Repeat("x", padBytes)
	content := fmt.Sprintf("---\ntitle: Large frontmatter artifact\nstatus: queued\nartifact_type: task\ndescription: %s\nid: %s\n---\nBody\n",
		padding, id)
	require.NoError(t, os.WriteFile(path, []byte(content), 0o644))
}

// TestBuildManifest_ExtractsItemID_WhenIDFieldBeyond512Bytes verifies that an
// artifact whose id: field starts after byte 512 is correctly identified by
// BuildManifest.
func TestBuildManifest_ExtractsItemID_WhenIDFieldBeyond512Bytes(t *testing.T) {
	tmpDir := t.TempDir()
	queueDir := filepath.Join(tmpDir, "queue")
	require.NoError(t, os.MkdirAll(queueDir, 0o755))

	artifactPath := filepath.Join(queueDir, "BIG001-T.md")
	// Use 600-byte padding to push id: well past the 512-byte boundary.
	writeLargeFrontmatterArtifact(t, artifactPath, "BIG001-T", 600)

	manifest, err := db.BuildManifest(tmpDir)
	require.NoError(t, err)

	entry, ok := manifest["queue/BIG001-T.md"]
	require.True(t, ok, "artifact file must appear in manifest")
	assert.Equal(t, "BIG001-T", entry.ItemID,
		"ItemID must be extracted even when id: field is past the 512-byte boundary")
}

// TestBuildManifest_ExtractsItemID_WhenIDFieldAt512ByteBoundary verifies
// correct extraction at the exact 512-byte boundary (edge case).
func TestBuildManifest_ExtractsItemID_WhenIDFieldAt512ByteBoundary(t *testing.T) {
	tmpDir := t.TempDir()
	queueDir := filepath.Join(tmpDir, "queue")
	require.NoError(t, os.MkdirAll(queueDir, 0o755))

	// Build frontmatter where id: starts at exactly byte 512.
	// Preamble is "---\ntitle: X\nstatus: queued\nartifact_type: task\ndescription: "
	preamble := "---\ntitle: X\nstatus: queued\nartifact_type: task\ndescription: "
	padNeeded := 512 - len(preamble)
	if padNeeded < 1 {
		padNeeded = 1
	}
	padding := strings.Repeat("y", padNeeded)
	content := fmt.Sprintf("%s%s\nid: EDGE001-T\n---\nBody\n", preamble, padding)

	artifactPath := filepath.Join(queueDir, "EDGE001-T.md")
	require.NoError(t, os.WriteFile(artifactPath, []byte(content), 0o644))

	manifest, err := db.BuildManifest(tmpDir)
	require.NoError(t, err)

	entry, ok := manifest["queue/EDGE001-T.md"]
	require.True(t, ok)
	assert.Equal(t, "EDGE001-T", entry.ItemID,
		"ItemID must be extracted when id: field falls on the 512-byte boundary")
}

// TestBuildManifest_NormalArtifact_ItemIDUnchanged verifies that the fix does
// not regress on normal short-frontmatter artifacts.
func TestBuildManifest_NormalArtifact_ItemIDUnchanged(t *testing.T) {
	tmpDir := t.TempDir()
	queueDir := filepath.Join(tmpDir, "queue")
	require.NoError(t, os.MkdirAll(queueDir, 0o755))

	content := "---\nid: NORM001-T\ntitle: Normal\nstatus: queued\nartifact_type: task\n---\nBody\n"
	require.NoError(t, os.WriteFile(filepath.Join(queueDir, "NORM001-T.md"), []byte(content), 0o644))

	manifest, err := db.BuildManifest(tmpDir)
	require.NoError(t, err)

	entry, ok := manifest["queue/NORM001-T.md"]
	require.True(t, ok)
	assert.Equal(t, "NORM001-T", entry.ItemID, "normal artifact ItemID must be unaffected")
}

// TestComputeDiff_RelocationDetected_LargeFrontmatterArtifact verifies that
// relocation detection works correctly for large-frontmatter artifacts: when
// the same artifact moves from queue/ to archive/, ComputeDiff must report a
// RelocationEntry rather than a delete + add pair.
func TestComputeDiff_RelocationDetected_LargeFrontmatterArtifact(t *testing.T) {
	tmpDir := t.TempDir()
	queueDir := filepath.Join(tmpDir, "queue")
	archiveDir := filepath.Join(tmpDir, "archive")
	require.NoError(t, os.MkdirAll(queueDir, 0o755))
	require.NoError(t, os.MkdirAll(archiveDir, 0o755))

	// Build initial manifest with the artifact in queue/.
	queuePath := filepath.Join(queueDir, "REL001-T.md")
	writeLargeFrontmatterArtifact(t, queuePath, "REL001-T", 600)
	oldManifest, err := db.BuildManifest(tmpDir)
	require.NoError(t, err)

	// "Move" artifact from queue/ to archive/.
	data, readErr := os.ReadFile(queuePath)
	require.NoError(t, readErr)
	require.NoError(t, os.WriteFile(filepath.Join(archiveDir, "REL001-T.md"), data, 0o644))
	require.NoError(t, os.Remove(queuePath))

	newManifest, err := db.BuildManifest(tmpDir)
	require.NoError(t, err)

	diff := db.ComputeDiff(oldManifest, newManifest)

	require.Len(t, diff.Relocated, 1,
		"moved large-frontmatter artifact must be detected as a relocation, not delete+add")
	assert.Equal(t, "REL001-T", diff.Relocated[0].ItemID)
	assert.Equal(t, "queue/REL001-T.md", diff.Relocated[0].OldPath)
	assert.Equal(t, "archive/REL001-T.md", diff.Relocated[0].NewPath)

	// Must NOT appear in Deleted or Added.
	for _, e := range diff.Deleted {
		assert.NotEqual(t, "REL001-T", e.ItemID, "relocated artifact must not appear in Deleted")
	}
	for _, e := range diff.Added {
		assert.NotEqual(t, "REL001-T", e.ItemID, "relocated artifact must not appear in Added")
	}
}
