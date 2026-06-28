package core

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/softwaresalt/backlogit/internal/models"
)

// archivedFromGolden is the byte-exact output of rewriteArchivedFromField for a
// self-referential record, captured from the PRE-refactor implementation (068.004-T).
// The U4 migration onto internal/mdfront MUST reproduce this byte-for-byte:
// sorted-key frontmatter, archived_from rewritten to the canonical queue path,
// and the CRLF body preserved verbatim.
const (
	archivedFromSelfRefIn = "---\nid: 200-T\ntitle: Self ref\nstatus: archived\nartifact_type: task\narchived_from: .backlogit/archive/200-T.md\n---\nBody line 1\r\nBody line 2\n"
	archivedFromNewValue  = ".backlogit/queue/200-T.md"
	archivedFromGolden    = "---\narchived_from: .backlogit/queue/200-T.md\nartifact_type: task\nid: 200-T\nstatus: archived\ntitle: Self ref\n---\nBody line 1\r\nBody line 2\n"
)

// TestRewriteArchivedFromField_SelfRefByteIdentical pins that the repair rewrites
// ONLY the archived_from field, preserves the body bytes verbatim (including the
// embedded CRLF), and produces output byte-identical to the captured pre-refactor
// bytes. This is the characterization gate for the mdfront swap.
func TestRewriteArchivedFromField_SelfRefByteIdentical(t *testing.T) {
	out, err := rewriteArchivedFromField([]byte(archivedFromSelfRefIn), archivedFromNewValue)
	require.NoError(t, err)
	assert.Equal(t, archivedFromGolden, string(out),
		"rewritten record must be byte-identical to the captured pre-refactor output")

	// Body bytes (after the closing fence) survive verbatim, CRLF included.
	assert.True(t, bytes.HasSuffix(out, []byte("Body line 1\r\nBody line 2\n")),
		"body bytes must be preserved verbatim")

	// Only archived_from changed: every other field keeps its original value.
	fm, _, perr := models.ParseFrontmatter(string(out))
	require.NoError(t, perr)
	assert.Equal(t, ".backlogit/queue/200-T.md", fm["archived_from"])
	assert.Equal(t, "200-T", fm["id"])
	assert.Equal(t, "task", fm["artifact_type"])
	assert.Equal(t, "archived", fm["status"])
	assert.Equal(t, "Self ref", fm["title"])
}

// TestRewriteArchivedFromField_FenceLessReturnsError pins the F1 error->skip
// contract: a record with no opening fence and a record with an opening fence but
// no closing fence must each return an ERROR (so the caller SKIPS it), NOT a
// nil-map panic and NOT a synthetic-frontmatter block wrapping the document.
func TestRewriteArchivedFromField_FenceLessReturnsError(t *testing.T) {
	cases := map[string]string{
		"no_opening_fence": "no frontmatter here\nbody line\n",
		"open_no_close":    "---\nid: x\nbody but no closing fence\n",
		"empty":            "",
		"plain_hr_in_body": "# Heading\n\n---\n\ntext\n",
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			out, err := rewriteArchivedFromField([]byte(in), archivedFromNewValue)
			require.Error(t, err, "a fence-less record must return an error (error->skip parity)")
			assert.Nil(t, out, "no bytes must be produced for a fence-less record")
			// Defense-in-depth: even if a future regression returned bytes, they
			// must never be a synthetic frontmatter block wrapping the input.
			assert.NotContains(t, string(out), archivedFromNewValue,
				"a fence-less record must never be wrapped in synthetic frontmatter")
		})
	}
}

// TestRewriteArchivedFromField_PreservesEmbeddedHorizontalRule guards the
// first-fence-wins semantics: a horizontal rule (---) in the body after a real
// frontmatter block must remain in the body, not be treated as the fence.
func TestRewriteArchivedFromField_PreservesEmbeddedHorizontalRule(t *testing.T) {
	in := "---\nid: 300-T\narchived_from: .backlogit/archive/300-T.md\n---\nintro\n\n---\na horizontal rule\n"
	out, err := rewriteArchivedFromField([]byte(in), ".backlogit/queue/300-T.md")
	require.NoError(t, err)
	assert.True(t, bytes.HasSuffix(out, []byte("intro\n\n---\na horizontal rule\n")),
		"the body horizontal rule must be preserved, not consumed as a fence")
	fm, _, perr := models.ParseFrontmatter(string(out))
	require.NoError(t, perr)
	assert.Equal(t, ".backlogit/queue/300-T.md", fm["archived_from"])
}

// TestArchiveWritePath_PreservesClampedMode pins the intended U4 write-path
// behavior change: the archive repair now writes through the hardened
// atomicfile writer, which preserves the target's existing mode CLAMPED
// (group/world write bits stripped). A 0640 source stays 0640; a 0664 source is
// clamped to 0644. POSIX modes are not represented on Windows, so this is gated.
func TestArchiveWritePath_PreservesClampedMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX permission bits are not represented on Windows filesystems")
	}
	tmp := t.TempDir()
	wsRoot := filepath.Join(tmp, ".backlogit")
	archiveDir := filepath.Join(wsRoot, "archive")
	require.NoError(t, os.MkdirAll(filepath.Join(wsRoot, "queue"), 0o755))

	selfRef := "---\nid: 400-T\ntitle: Mode\nstatus: archived\nartifact_type: task\narchived_from: .backlogit/archive/400-T.md\n---\nBody\n"
	helperWriteArtifact(t, archiveDir, "400-T.md", selfRef)
	path := filepath.Join(archiveDir, "400-T.md")
	require.NoError(t, os.Chmod(path, 0o664)) // over-permissive group write

	ws := newDoctorTestWorkspace(t, tmp, true)
	_, err := Doctor(context.Background(), ws, &DoctorOptions{CheckArchivedFrom: true, FixArchivedFrom: true})
	require.NoError(t, err)

	info, statErr := os.Stat(path)
	require.NoError(t, statErr)
	assert.Equal(t, os.FileMode(0o644), info.Mode().Perm(),
		"the archive write must clamp the over-permissive 0664 source to 0644")
}
