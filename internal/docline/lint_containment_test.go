package docline

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDecodeDoc_ContainmentGuard is scenario 1 of 146.017-T (U7b): a direct
// sentinel guard on decodeDoc, not a LintTree propagation assertion.
// decodeDoc(root, rel) is called directly with a rel that makes its own
// core.SafeResolve fail, and the resulting error must satisfy
// errors.Is(err, ErrPathEscapesWorkspace) while errors.Is(err,
// ErrFrontmatterDecode) is false — a containment failure can never be
// classified as decodable data.
func TestDecodeDoc_ContainmentGuard(t *testing.T) {
	root := t.TempDir()

	_, err := decodeDoc(root, "../escape.md")
	require.Error(t, err)
	assert.True(t, errors.Is(err, ErrPathEscapesWorkspace))
	assert.False(t, errors.Is(err, ErrFrontmatterDecode))
}

// TestLintTree_ContainmentGuard_EscapingPath is scenario 1b of 146.017-T
// (U7b): LintTree with Options{Path: "../escape"} returns a nil findings
// slice and an error satisfying errors.Is(err, ErrPathEscapesWorkspace),
// with errors.Is(err, ErrFrontmatterDecode) false, guarding the
// collectInScopeDocs containment edge end-to-end.
func TestLintTree_ContainmentGuard_EscapingPath(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "docs/decisions/x.md", "---\ntitle: X\nsource: y\ndoc_type: decision\n---\nBody.\n")

	findings, err := LintTree(Options{Root: root, Path: "../escape", Profile: ProfileAuthoring})
	require.Error(t, err)
	assert.Nil(t, findings)
	assert.True(t, errors.Is(err, ErrPathEscapesWorkspace))
	assert.False(t, errors.Is(err, ErrFrontmatterDecode))
}

// TestDecodeDoc_UnreadableFile is scenario 2 of 146.017-T (U7b): an
// unreadable file — decodeDoc called directly with a rel resolving to a
// DIRECTORY, so os.ReadFile fails on every platform (a chmod 0o000
// construction is rejected as a no-op on Windows) — returns an error for
// which errors.Is(err, ErrFrontmatterDecode) is false. Over an ordinary
// corpus, no finding's File or Fix string contains the workspace absolute
// path prefix; every one is repo-relative POSIX.
func TestDecodeDoc_UnreadableFile(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "docs", "decisions", "a-directory.md"), 0o755))

	_, err := decodeDoc(root, "docs/decisions/a-directory.md")
	require.Error(t, err)
	assert.False(t, errors.Is(err, ErrFrontmatterDecode))
}

// TestLintTree_FindingsNeverLeakAbsolutePath is the corpus half of scenario 2
// of 146.017-T (U7b): over an ordinary corpus, no finding's File or Fix
// string contains the workspace absolute path prefix; every one is
// repo-relative POSIX, matching the rest of the package.
func TestLintTree_FindingsNeverLeakAbsolutePath(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "docs/decisions/missing-source.md", "---\ntitle: Missing Source\n---\nBody.\n")

	findings, err := LintTree(Options{Root: root, Profile: ProfileAuthoring})
	require.NoError(t, err)
	require.NotEmpty(t, findings)

	absRoot, err := filepath.Abs(root)
	require.NoError(t, err)
	for _, f := range findings {
		assert.False(t, strings.Contains(f.File, absRoot), "finding File must be repo-relative POSIX, not absolute")
		assert.False(t, strings.Contains(f.Fix, absRoot), "finding Fix must never leak the workspace absolute path")
		assert.False(t, strings.HasPrefix(f.File, "/"), "finding File must be repo-relative, not rooted")
		assert.False(t, strings.Contains(f.File, "\\"), "finding File must be POSIX-separated")
	}
}
