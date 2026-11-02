//go:build windows

package core

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/windows"
)

func TestReviewStashWindowsShortPath(t *testing.T) {
	root := t.TempDir()
	longDir := filepath.Join(root, "stash-shortpath-long-parent-suffix")
	require.NoError(t, os.Mkdir(longDir, 0o755))
	name, err := windows.UTF16PtrFromString(longDir)
	require.NoError(t, err)
	// TEMP may already contain an 8.3 component, even when the volume does not
	// create new aliases. Compare against the full long spelling of the directory.
	size, err := windows.GetLongPathName(name, nil, 0)
	require.NoError(t, err)
	require.Positive(t, size)
	buffer := make([]uint16, size)
	length, err := windows.GetLongPathName(name, &buffer[0], size)
	require.NoError(t, err)
	require.Positive(t, length)
	require.Less(t, length, size)
	longDir = windows.UTF16ToString(buffer)
	name, err = windows.UTF16PtrFromString(longDir)
	require.NoError(t, err)
	size, err = windows.GetShortPathName(name, nil, 0)
	require.NoError(t, err)
	require.Positive(t, size)
	buffer = make([]uint16, size)
	length, err = windows.GetShortPathName(name, &buffer[0], size)
	require.NoError(t, err)
	require.Positive(t, length)
	require.Less(t, length, size)
	shortDir := windows.UTF16ToString(buffer)
	require.False(t, strings.EqualFold(longDir, shortDir),
		"blocked prerequisite: volume must support real Windows 8.3 aliases")
	require.Contains(t, shortDir, "~",
		"blocked prerequisite: directory or ancestor must have an actual 8.3 alias")
	t.Logf("actual 8.3 alias verified; created directory has alias: %t",
		strings.Contains(filepath.Base(shortDir), "~"))

	for _, mode := range []string{"new", "abandoned", "fresh", "live-aged"} {
		t.Run(mode, func(t *testing.T) {
			lockPath := filepath.Join(longDir, mode+".lock")
			aliasPath := filepath.Join(shortDir, mode+".lock")
			before := []byte("existing sidecar must not be stolen")
			if mode != "new" {
				require.NoError(t, os.WriteFile(lockPath, before, 0o600))
				if mode != "fresh" {
					old := time.Now().Add(-2 * staleLockTTL)
					require.NoError(t, os.Chtimes(lockPath, old, old))
				}
			}
			var live *os.File
			if mode == "live-aged" {
				var busy bool
				live, busy, err = openTaskLockHandle(lockPath)
				require.NoError(t, err)
				require.False(t, busy)
				t.Cleanup(func() {
					if live != nil {
						require.NoError(t, live.Close())
					}
				})
			}
			file, acquireErr := acquireStashSidecarFile(aliasPath)
			if mode == "fresh" || mode == "live-aged" {
				if file != nil {
					require.NoError(t, releaseStashSidecarFile(file))
				}
				require.ErrorIs(t, acquireErr, os.ErrExist)
				if live != nil {
					require.NoError(t, live.Close())
					live = nil
				}
				after, readErr := os.ReadFile(lockPath)
				require.NoError(t, readErr)
				require.Equal(t, before, after)
				old := time.Now().Add(-2 * staleLockTTL)
				require.NoError(t, os.Chtimes(lockPath, old, old))
				file, acquireErr = acquireStashSidecarFile(aliasPath)
			}
			if acquireErr != nil && file != nil {
				require.NoError(t, releaseStashSidecarFile(file))
			}
			require.NoError(t, acquireErr)
			t.Cleanup(func() {
				if file != nil {
					require.NoError(t, releaseStashSidecarFile(file))
				}
			})
			competitor, busy, openErr := openTaskLockHandle(lockPath)
			if competitor != nil {
				require.NoError(t, competitor.Close())
			}
			require.NoError(t, openErr)
			require.True(t, busy, "long-name contender cannot steal alias ownership")
			require.NoError(t, releaseStashSidecarFile(file))
			file = nil
			require.NoFileExists(t, lockPath)
			require.NoFileExists(t, aliasPath)
		})
	}
	entries, err := os.ReadDir(longDir)
	require.NoError(t, err)
	require.Empty(t, entries, "no leaked sidecar under either spelling")
}

func TestReviewStashRejectsJunctionComponent(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "outside")
	require.NoError(t, os.Mkdir(outside, 0o755))
	target := filepath.Join(outside, "stash.jsonl.lock")
	before := []byte("junction target must remain unchanged")
	require.NoError(t, os.WriteFile(target, before, 0o600))
	old := time.Now().Add(-2 * staleLockTTL)
	require.NoError(t, os.Chtimes(target, old, old))
	link := filepath.Join(root, "junction")
	require.NoError(t, createReconcileTestJunction(t, link, outside))
	lockPath := filepath.Join(link, "stash.jsonl.lock")
	file, err := acquireStashSidecarFile(lockPath)
	if file != nil {
		require.NoError(t, file.Close())
	}
	require.Error(t, err)
	file, busy, err := openStashLockHandle(lockPath)
	if file != nil {
		require.NoError(t, file.Close())
	}
	require.False(t, busy)
	require.Error(t, err)
	after, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, before, after)
}

func TestReviewStashRejectsJunctionAncestorAfterPrecheck(t *testing.T) {
	root := t.TempDir()
	outside := filepath.Join(root, "outside")
	nested := filepath.Join(outside, "long-nonreparse-child-directory")
	require.NoError(t, os.MkdirAll(nested, 0o755))
	target := filepath.Join(nested, "stash.jsonl.lock")
	before := []byte("ancestor junction target must remain unchanged")
	require.NoError(t, os.WriteFile(target, before, 0o600))
	old := time.Now().Add(-2 * staleLockTTL)
	require.NoError(t, os.Chtimes(target, old, old))
	beforeInfo, err := os.Stat(target)
	require.NoError(t, err)
	link := filepath.Join(root, "junction")
	require.NoError(t, createReconcileTestJunction(t, link, outside))
	lockPath := filepath.Join(link, filepath.Base(nested), filepath.Base(target))
	// Bypass the pathname precheck as if the ancestor became a junction after
	// validation. Long-name expansion must not turn its target into the
	// authorized destination; the leaf directory itself is not a reparse point.
	file, busy, err := openStashLockHandle(lockPath)
	if file != nil {
		require.NoError(t, file.Close())
	}
	require.False(t, busy)
	require.Error(t, err)
	after, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, before, after)
	afterInfo, err := os.Stat(target)
	require.NoError(t, err)
	require.Equal(t, beforeInfo.ModTime(), afterInfo.ModTime())
	entries, err := os.ReadDir(nested)
	require.NoError(t, err)
	require.Len(t, entries, 1, "no extra sidecar may be created in redirected ancestry")
}
