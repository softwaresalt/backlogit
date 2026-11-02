//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package core

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"golang.org/x/sys/unix"
)

func TestReviewStashReleaseAncestorRedirect(t *testing.T) {
	root := t.TempDir()
	parent := filepath.Join(root, "parent")
	external := filepath.Join(root, "external")
	moved := filepath.Join(root, "moved")
	require.NoError(t, os.Mkdir(parent, 0o755))
	require.NoError(t, os.Mkdir(external, 0o755))
	lockPath := filepath.Join(parent, "stash.lock")
	protected := filepath.Join(external, "stash.lock")
	content := []byte("external sidecar must survive")
	require.NoError(t, os.WriteFile(protected, content, 0o600))
	victim, err := os.Lstat(protected)
	require.NoError(t, err)
	file, err := acquireStashSidecarFile(lockPath)
	require.NoError(t, err)
	t.Cleanup(func() { _ = file.Close() }) // Release also closes on failure.
	leafFD := int(file.Fd())
	pins := stashParentPins(t, parent, leafFD)
	require.Len(t, pins, 1, "ownership must retain exactly one parent descriptor")
	if err := os.Rename(parent, moved); err != nil {
		// DrvFS may deny renaming a directory with a retained pin. That
		// prevents the redirect itself, but still requires complete release.
		require.True(t, errors.Is(err, unix.EACCES) || errors.Is(err, unix.EPERM) || errors.Is(err, unix.EBUSY),
			"unexpected ancestor rename failure: %v", err)
		t.Logf("ancestor redirect prevented by retained-pin rename denial: %v", err)
		require.NoError(t, releaseStashSidecarFile(file))
		var info unix.Stat_t
		require.ErrorIs(t, unix.Fstat(leafFD, &info), unix.EBADF)
		require.ErrorIs(t, unix.Fstat(pins[0], &info), unix.EBADF)
		require.NoFileExists(t, lockPath)
		_, err := os.Lstat(moved)
		require.ErrorIs(t, err, os.ErrNotExist, "denied rename must not create a redirect location")
		original, err := os.Lstat(parent)
		require.NoError(t, err)
		require.True(t, original.IsDir())
		require.Zero(t, original.Mode()&os.ModeSymlink, "no redirect may be installed")
		after, err := os.ReadFile(protected)
		require.NoError(t, err)
		require.Equal(t, content, after)
		current, err := os.Lstat(protected)
		require.NoError(t, err)
		require.True(t, os.SameFile(victim, current), "external victim name must still identify the original file")
		entries, err := os.ReadDir(external)
		require.NoError(t, err)
		require.Len(t, entries, 1)
		require.Equal(t, "stash.lock", entries[0].Name())
		reusedParent, err := os.Open(parent)
		require.NoError(t, err)
		defer func() { require.NoError(t, reusedParent.Close()) }()
		reusedLeaf, err := os.Open(protected)
		require.NoError(t, err)
		defer func() { require.NoError(t, reusedLeaf.Close()) }()
		require.NoError(t, file.Close(), "release must close each resource exactly once")
		_, err = reusedParent.Stat()
		require.NoError(t, err, "second Close must not close a reused parent descriptor")
		_, err = reusedLeaf.Stat()
		require.NoError(t, err, "second Close must not close a reused leaf descriptor")
		return
	}
	t.Log("ancestor rename succeeded; executing symlink-redirection regression")
	require.NoError(t, os.Symlink(external, parent))
	require.NoError(t, releaseStashSidecarFile(file))
	require.NoFileExists(t, filepath.Join(moved, "stash.lock"))
	after, err := os.ReadFile(protected)
	require.NoError(t, err)
	require.Equal(t, content, after)
}

func TestReviewStashReleaseReplacement(t *testing.T) {
	for _, kind := range []string{"successor", "symlink", "hardlink", "directory", "missing"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			path := filepath.Join(root, "stash.lock")
			file, err := acquireStashSidecarFile(path)
			require.NoError(t, err)
			t.Cleanup(func() { _ = file.Close() }) // Idempotent resource cleanup.
			require.NoError(t, os.Rename(path, filepath.Join(root, "old.lock")))
			target := filepath.Join(root, "valuable")
			content := []byte("replacement must survive")
			require.NoError(t, os.WriteFile(target, content, 0o600))
			switch kind {
			case "successor":
				require.NoError(t, os.WriteFile(path, content, 0o600))
				old := time.Now().Add(-2 * staleLockTTL)
				require.NoError(t, os.Chtimes(path, old, old))
				next, err := acquireStashSidecarFile(path)
				require.NoError(t, err)
				t.Cleanup(func() { _ = next.Close() }) // Release closes on failure.
				require.ErrorIs(t, releaseStashSidecarFile(file), os.ErrPermission)
				require.FileExists(t, path, "even a cooperating successor must remain owned")
				contender, busy, err := openStashLockHandle(path)
				require.NoError(t, err)
				require.True(t, busy)
				require.Nil(t, contender)
				require.NoError(t, releaseStashSidecarFile(next))
				require.NoFileExists(t, path)
				return
			case "symlink":
				require.NoError(t, os.Symlink(target, path))
			case "hardlink":
				require.NoError(t, os.Link(target, path))
			case "directory":
				require.NoError(t, os.Mkdir(path, 0o755))
			}
			require.ErrorIs(t, releaseStashSidecarFile(file), os.ErrPermission)
			if kind != "missing" {
				_, err := os.Lstat(path)
				require.NoError(t, err, "replacement basename must remain untouched")
			}
			after, err := os.ReadFile(target)
			require.NoError(t, err)
			require.Equal(t, content, after)
		})
	}
}

func TestReviewStashReleasedOwnerCannotDeleteNormalSuccessor(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stash.lock")
	first, err := acquireStashSidecarFile(path)
	require.NoError(t, err)
	require.NoError(t, releaseStashSidecarFile(first))
	next, err := acquireStashSidecarFile(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = next.Close() }) // Release closes on failure.
	require.Error(t, releaseStashSidecarFile(first))
	require.FileExists(t, path)
	require.NoError(t, releaseStashSidecarFile(next))
	require.NoFileExists(t, path)
}

// stashParentPins finds descriptors for this unique test directory, without
// depending on an implementation type that does not exist during RED.
func stashParentPins(t *testing.T, parent string, leafFD int) []int {
	t.Helper()
	var directory unix.Stat_t
	require.NoError(t, unix.Stat(parent, &directory))
	pins := make([]int, 0, 1)
	for fd := 0; fd <= leafFD+32; fd++ {
		var info unix.Stat_t
		if err := unix.Fstat(fd, &info); err != nil {
			require.ErrorIs(t, err, unix.EBADF)
			continue
		}
		if info.Dev == directory.Dev && info.Ino == directory.Ino {
			pins = append(pins, fd)
		}
	}
	return pins
}

func TestReviewStashDirectCloseReleasesParent(t *testing.T) {
	for _, brokenLeaf := range []bool{false, true} {
		t.Run(map[bool]string{false: "normal", true: "leaf-close-error"}[brokenLeaf], func(t *testing.T) {
			parent := t.TempDir()
			file, err := acquireStashSidecarFile(filepath.Join(parent, "stash.lock"))
			require.NoError(t, err)
			t.Cleanup(func() { _ = file.Close() }) // Cached close errors are asserted below.
			pins := stashParentPins(t, parent, int(file.Fd()))
			require.Len(t, pins, 1, "ownership must retain exactly one parent descriptor")
			if brokenLeaf {
				// No descriptor allocation occurs before Close; this forces a
				// real leaf-close error without risking reuse of its descriptor.
				require.NoError(t, unix.Close(int(file.Fd())))
			}
			closeErr := file.Close()
			if brokenLeaf {
				require.ErrorIs(t, closeErr, unix.EBADF)
			} else {
				require.NoError(t, closeErr)
			}
			var info unix.Stat_t
			require.ErrorIs(t, unix.Fstat(pins[0], &info), unix.EBADF)
			reused, err := os.Open(parent)
			require.NoError(t, err)
			defer func() { require.NoError(t, reused.Close()) }()
			again := file.Close()
			if brokenLeaf {
				require.ErrorIs(t, again, unix.EBADF)
			} else {
				require.NoError(t, again)
			}
			_, err = reused.Stat()
			require.NoError(t, err, "second Close must not close a reused parent descriptor")
		})
	}
}
