//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package core

import (
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

func verifyLocksDirHandle(dirFile *os.File, expectedDirAbs string) error {
	var st unix.Stat_t
	if err := unix.Fstat(int(dirFile.Fd()), &st); err != nil {
		return fmt.Errorf("stat pinned locks directory handle: %w", err)
	}
	if st.Mode&unix.S_IFMT != unix.S_IFDIR {
		return fmt.Errorf("pinned locks handle is not a directory: %w", blerrors.ErrValidation)
	}
	if expectedDirAbs != "" {
		lstatInfo, err := os.Lstat(expectedDirAbs)
		if err != nil {
			return fmt.Errorf("lstat expected locks directory %q: %w", expectedDirAbs, err)
		}
		if lstatInfo.Mode()&os.ModeSymlink != 0 || !lstatInfo.IsDir() {
			return fmt.Errorf("expected locks directory %q is redirected or not a directory: %w", expectedDirAbs, blerrors.ErrValidation)
		}
		handleStat, err := dirFile.Stat()
		if err != nil {
			return fmt.Errorf("stat pinned locks directory handle: %w", err)
		}
		if !os.SameFile(lstatInfo, handleStat) {
			return fmt.Errorf("pinned locks directory %q identity mismatch: %w", expectedDirAbs, blerrors.ErrValidation)
		}
	}
	return nil
}

func verifyLocksLeafHandle(f *os.File) error {
	var st unix.Stat_t
	if err := unix.Fstat(int(f.Fd()), &st); err != nil {
		return fmt.Errorf("stat locks leaf handle: %w", err)
	}
	if st.Mode&unix.S_IFMT != unix.S_IFREG {
		return fmt.Errorf("locks leaf is not a regular file: %w", blerrors.ErrValidation)
	}
	return nil
}

func tryLockAllocatorFileHandle(f *os.File) (bool, error) {
	err := unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB)
	if err == nil {
		return false, nil
	}
	if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
		return true, nil
	}
	return false, fmt.Errorf("lock id allocator sidecar: %w", err)
}

func unlockAllocatorFileHandle(f *os.File) error {
	if err := unix.Flock(int(f.Fd()), unix.LOCK_UN); err != nil {
		return fmt.Errorf("unlock id allocator sidecar: %w", err)
	}
	return nil
}
