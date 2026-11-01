//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package core

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

func openTaskLockHandle(lockPath string) (*os.File, bool, error) {
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, false, fmt.Errorf("open task lock sidecar %s: %w", lockPath, err)
	}

	if err := unix.Flock(int(file.Fd()), unix.LOCK_EX|unix.LOCK_NB); err != nil {
		closeErr := file.Close()
		if closeErr != nil {
			return nil, false, fmt.Errorf("close busy task sidecar: %w", errors.Join(err, closeErr))
		}
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return nil, true, nil
		}
		return nil, false, fmt.Errorf("lock task sidecar %s: %w", lockPath, err)
	}
	return file, false, nil
}

func openStashLockHandle(lockPath string) (*os.File, bool, error) {
	// Walk from the filesystem root using no-follow directory-relative opens.
	// Replacing a component cannot redirect the next open outside the pinned
	// ancestry, including between the common Lstat checks and this walk.
	dirFD, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, false, fmt.Errorf("open stash filesystem root: %w", err)
	}
	for _, component := range strings.Split(strings.TrimPrefix(filepath.Dir(lockPath), "/"), "/") {
		if component == "" {
			continue
		}
		next, openErr := unix.Openat(dirFD, component, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
		closeErr := unix.Close(dirFD)
		if openErr != nil || closeErr != nil {
			if openErr == nil {
				closeErr = errors.Join(closeErr, unix.Close(next))
			}
			return nil, false, fmt.Errorf("pin stash directory component: %w", errors.Join(openErr, closeErr))
		}
		dirFD = next
	}
	fd, openErr := unix.Openat(dirFD, filepath.Base(lockPath),
		unix.O_CREAT|unix.O_RDWR|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0o644)
	closeErr := unix.Close(dirFD)
	if openErr != nil {
		return nil, false, fmt.Errorf("open pinned stash sidecar: %w", errors.Join(openErr, closeErr))
	}
	file := os.NewFile(uintptr(fd), lockPath)
	fail := func(err error) (*os.File, bool, error) {
		return nil, false, errors.Join(err, file.Close())
	}
	if closeErr != nil {
		return fail(fmt.Errorf("close stash directory pin: %w", closeErr))
	}
	var info unix.Stat_t
	if err := unix.Fstat(fd, &info); err != nil {
		return fail(fmt.Errorf("stat stash sidecar handle: %w", err))
	}
	if info.Mode&unix.S_IFMT != unix.S_IFREG || info.Nlink != 1 {
		return fail(fmt.Errorf("linked or nonregular stash sidecar handle: %w", os.ErrPermission))
	}
	if err := unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB); err != nil {
		closeErr := file.Close()
		if closeErr == nil && (errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN)) {
			return nil, true, nil
		}
		return nil, false, fmt.Errorf("lock stash sidecar: %w", errors.Join(err, closeErr))
	}
	return file, false, nil
}

func releaseStashSidecarFile(file *os.File) error {
	// Unlink while locked. Contenders opening the old inode fail SameFile
	// validation; contenders opening the new path cannot be removed by us.
	removeErr := unlockStashFile(strings.TrimSuffix(file.Name(), ".lock"))
	closeErr := file.Close()
	if removeErr != nil || closeErr != nil {
		return fmt.Errorf("release stash sidecar: %w", errors.Join(removeErr, closeErr))
	}
	return nil
}

// unlockStashFile removes the advisory lock sidecar created by lockStashFile.
// Callers must retain ownership while removing the sidecar.
func unlockStashFile(path string) error {
	lockPath := path + ".lock"
	if err := os.Remove(lockPath); err != nil && !os.IsNotExist(err) {
		slog.Warn("failed to remove stash lock file", "path", lockPath, "error", err)
		return fmt.Errorf("remove stash sidecar: %w", err)
	}
	return nil
}
