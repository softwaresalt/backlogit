package docline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func symlinkOrSkip(t *testing.T, target, link string) {
	t.Helper()
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
}

func TestRemediation_ResolveReturnsRealTarget(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "real/a.go", sampleGo)
	symlinkOrSkip(t, filepath.Join(root, "real", "a.go"), filepath.Join(root, "alias.go"))

	r, err := ResolveWorkspacePath(root, "alias.go")
	require.NoError(t, err)
	want, err := filepath.EvalSymlinks(filepath.Join(root, "real", "a.go"))
	require.NoError(t, err)
	assert.Equal(t, want, r.Real)
}

func TestRemediation_AnchorRequiresRegularFile(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(root, "dir"), 0o755))
	_, err := ResolveAnchorString(root, "dir:1")
	assert.ErrorIs(t, err, ErrAnchorNotFound)
}

func TestRemediation_ClaimsRejectSymlinkEscape(t *testing.T) {
	parent := t.TempDir()
	root := filepath.Join(parent, "ws")
	require.NoError(t, os.MkdirAll(root, 0o755))
	writeDoc(t, parent, "outside/secret.txt", "x")
	symlinkOrSkip(t, filepath.Join(parent, "outside"), filepath.Join(root, "link"))
	symlinkOrSkip(t, filepath.Join(parent, "outside", "secret.txt"), filepath.Join(root, "esc.txt"))

	topo := checkClaims(root, "docs/x.md", `<!-- docline:topology path="link/secret.txt" kind=file -->`+"\n")
	assert.Equal(t, []string{RuleTopologyClaim}, rulesOf(topo))
	assert.Contains(t, topo[0].Fix, "workspace-relative")

	count := checkClaims(root, "docs/x.md", `<!-- docline:count glob="esc.txt" expect=1 -->`+"\n")
	assert.Equal(t, []string{RuleNumericClaim}, rulesOf(count))

	glob := checkClaims(root, "docs/x.md", `<!-- docline:count glob="*.txt" expect=1 -->`+"\n")
	assert.Equal(t, []string{RuleNumericClaim}, rulesOf(glob), "escaping glob match must not be counted")
}

func TestRemediation_CommentDriftMissingPathIsMalformed(t *testing.T) {
	root := t.TempDir()
	got := checkCommentDrift(root, "docs/x.md", `<!-- docline:comment-ref symbol="Widget" text="x" -->`+"\n")
	require.Len(t, got, 1)
	assert.Equal(t, RuleCommentDrift, got[0].Rule)
	assert.Contains(t, got[0].Fix, "malformed")

	got = checkCommentDrift(root, "docs/x.md", `<!-- docline:comment-ref path="a.go" text="x" -->`+"\n")
	require.Len(t, got, 1)
	assert.Contains(t, got[0].Fix, "malformed")
}
