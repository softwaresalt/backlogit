package core

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestReviewStashSidecarOwnership(t *testing.T) {
	for _, mode := range []string{"fresh", "abandoned", "live-aged"} {
		t.Run(mode, func(t *testing.T) {
			root := t.TempDir()
			ws := &Workspace{RootPath: root}
			path := StashFilePath(root)
			require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
			lockPath := path + ".lock"
			require.NoError(t, os.WriteFile(lockPath, nil, 0o644))
			if mode != "fresh" {
				old := time.Now().Add(-2 * staleLockTTL)
				require.NoError(t, os.Chtimes(lockPath, old, old))
			}
			if mode == "live-aged" {
				handle, busy, err := openTaskLockHandle(lockPath)
				require.NoError(t, err)
				require.False(t, busy)
				t.Cleanup(func() { require.NoError(t, handle.Close()) })
			}
			start := time.Now()
			unlock, err := tryLockActiveStash(ws)
			require.Less(t, time.Since(start), time.Second)
			if mode == "abandoned" {
				require.NoError(t, err)
				require.NoError(t, unlock())
				require.NoFileExists(t, lockPath)
			} else {
				require.ErrorIs(t, err, os.ErrExist)
				require.FileExists(t, lockPath)
			}
		})
	}
}

func TestReviewStashBlockingLockDoesNotStealLiveAgedHandle(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stash.jsonl")
	unlock, err := lockStashFile(path)
	require.NoError(t, err)
	defer func() { require.NoError(t, unlock()) }()
	// Age is deliberately not the ownership proof. A second OS handle must
	// remain excluded even if the sidecar is old (also covered above).
	handle, busy, err := openTaskLockHandle(path + ".lock")
	require.NoError(t, err)
	if handle != nil {
		require.NoError(t, handle.Close())
	}
	require.True(t, busy, "stash ownership must retain the OS-exclusive handle")
}

func TestReviewStashBlockingLockRecoversAbandonedSidecar(t *testing.T) {
	path := filepath.Join(t.TempDir(), "stash.jsonl")
	require.NoError(t, os.WriteFile(path+".lock", nil, 0o644))
	old := time.Now().Add(-2 * staleLockTTL)
	require.NoError(t, os.Chtimes(path+".lock", old, old))
	unlock, err := lockStashFile(path)
	require.NoError(t, err)
	require.NoError(t, unlock())
	// A released owner cannot remove a successor: deletion is finished before
	// this new acquisition returns.
	nextUnlock, err := lockStashFile(path)
	require.NoError(t, err)
	require.NoError(t, nextUnlock())
}

func TestReviewStashRejectsLinkedSidecars(t *testing.T) {
	for _, kind := range []string{"symlink", "hardlink", "directory-component", "nonregular"} {
		t.Run(kind, func(t *testing.T) {
			root := t.TempDir()
			outside := filepath.Join(root, "outside")
			require.NoError(t, os.Mkdir(outside, 0o755))
			target := filepath.Join(outside, "valuable.lock")
			content := []byte("operator data must not be truncated")
			require.NoError(t, os.WriteFile(target, content, 0o600))
			old := time.Now().Add(-2 * staleLockTTL)
			require.NoError(t, os.Chtimes(target, old, old))
			lockPath := filepath.Join(root, "stash.jsonl.lock")
			var err error
			switch kind {
			case "symlink":
				err = os.Symlink(target, lockPath)
			case "hardlink":
				err = os.Link(target, lockPath)
			case "directory-component":
				err = os.Symlink(outside, filepath.Join(root, "linked"))
				lockPath = filepath.Join(root, "linked", "valuable.lock")
			case "nonregular":
				err = os.Mkdir(lockPath, 0o755)
				require.NoError(t, os.Chtimes(lockPath, old, old))
			}
			if err != nil && (kind == "symlink" || kind == "directory-component") {
				t.Skipf("symlink creation unavailable: %v", err)
			}
			require.NoError(t, err)
			file, err := acquireStashSidecarFile(lockPath)
			if file != nil {
				require.NoError(t, file.Close())
			}
			require.Error(t, err, "linked/nonregular sidecars must fail before mutation")
			// Exercise the handle opener independently of the pathname
			// precheck, as for a link introduced in the check/open window.
			file, busy, err := openStashLockHandle(lockPath)
			if file != nil {
				require.NoError(t, file.Close())
			}
			require.False(t, busy)
			require.Error(t, err, "the platform opener must itself reject linked handles/components")
			after, readErr := os.ReadFile(target)
			require.NoError(t, readErr)
			require.Equal(t, content, after)
		})
	}
}
