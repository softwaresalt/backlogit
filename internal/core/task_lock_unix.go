//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package core

import (
	"errors"
	"fmt"
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

func openStashLockHandle(lockPath string) (*stashLockFile, bool, error) {
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
	parent := os.NewFile(uintptr(dirFD), filepath.Dir(lockPath))
	fd, openErr := unix.Openat(dirFD, filepath.Base(lockPath),
		unix.O_CREAT|unix.O_RDWR|unix.O_NOFOLLOW|unix.O_NONBLOCK, 0o644)
	if openErr != nil {
		return nil, false, fmt.Errorf("open pinned stash sidecar: %w", errors.Join(openErr, parent.Close()))
	}
	file := &stashLockFile{
		File:     os.NewFile(uintptr(fd), lockPath),
		parent:   parent,
		basename: filepath.Base(lockPath),
	}
	fail := func(err error) (*stashLockFile, bool, error) {
		return nil, false, errors.Join(err, file.Close())
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

func releaseStashSidecarFile(file *stashLockFile) error {
	// Never resolve ancestors again. Cooperating contenders cannot replace a
	// flock-owned inode; after unlink, acquisition rejects an orphaned opener
	// and our deletion is already complete before a successor can be returned.
	//
	// Fstatat plus Unlinkat is NOT an atomic compare-and-delete. A
	// noncooperating writer able to mutate the pinned directory can still swap
	// the basename between these syscalls. Observed drift fails closed, but
	// hostile concurrent directory writers remain outside the advisory-lock
	// contract. The retained pin prevents ancestor redirection in either case.
	removeErr := func() error {
		if file.parent == nil {
			return fmt.Errorf("missing stash sidecar parent ownership: %w", os.ErrPermission)
		}
		var owned, current unix.Stat_t
		if err := unix.Fstat(int(file.Fd()), &owned); err != nil {
			return fmt.Errorf("stat releasing stash sidecar handle: %w", errors.Join(os.ErrPermission, err))
		}
		if owned.Mode&unix.S_IFMT != unix.S_IFREG || owned.Nlink != 1 {
			return fmt.Errorf("linked or nonregular releasing stash sidecar: %w", os.ErrPermission)
		}
		if err := unix.Fstatat(int(file.parent.Fd()), file.basename, &current, unix.AT_SYMLINK_NOFOLLOW); err != nil {
			return fmt.Errorf("stat releasing stash sidecar basename: %w", errors.Join(os.ErrPermission, err))
		}
		if current.Dev != owned.Dev || current.Ino != owned.Ino ||
			current.Mode&unix.S_IFMT != unix.S_IFREG || current.Nlink != 1 {
			return fmt.Errorf("stash sidecar identity changed before release: %w", os.ErrPermission)
		}
		if err := unix.Unlinkat(int(file.parent.Fd()), file.basename, 0); err != nil {
			return fmt.Errorf("unlink owned stash sidecar: %w", err)
		}
		return nil
	}()
	closeErr := file.Close()
	if removeErr != nil || closeErr != nil {
		return fmt.Errorf("release stash sidecar: %w", errors.Join(removeErr, closeErr))
	}
	return nil
}
