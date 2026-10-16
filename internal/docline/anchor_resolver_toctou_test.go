package docline

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadAnchor_SymlinkReplacedAfterResolutionDoesNotEscape(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	require.NoError(t, os.MkdirAll(root, 0o755))
	outside := filepath.Join(base, "secret.txt")
	require.NoError(t, os.WriteFile(outside, []byte("secret\n"), 0o644))
	target := filepath.Join(root, "a.txt")
	require.NoError(t, os.WriteFile(target, []byte("hello\n"), 0o644))

	swapped := false
	afterAnchorResolveHook = func(real string) {
		require.NoError(t, os.Remove(real))
		if err := os.Symlink(outside, real); err != nil {
			t.Skipf("cannot create symlink: %v", err)
		}
		swapped = true
	}
	t.Cleanup(func() { afterAnchorResolveHook = nil })

	_, content, err := loadAnchor(root, Anchor{Path: "a.txt"})
	if !swapped {
		return
	}
	require.Error(t, err)
	assert.NotContains(t, content, "secret")
}

func TestLoadAnchor_InRootSymlinkStillResolves(t *testing.T) {
	root := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(root, "real.txt"), []byte("hi\n"), 0o644))
	if err := os.Symlink(filepath.Join(root, "real.txt"), filepath.Join(root, "link.txt")); err != nil {
		t.Skipf("cannot create symlink: %v", err)
	}
	_, content, err := loadAnchor(root, Anchor{Path: "link.txt"})
	require.NoError(t, err)
	assert.Equal(t, "hi\n", content)
}
