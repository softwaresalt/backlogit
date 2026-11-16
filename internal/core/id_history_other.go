//go:build !windows && !(linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package core

import (
	"fmt"
	"os"
)

func verifyLocksDirHandle(dirFile *os.File, _ string) error {
	info, err := dirFile.Stat()
	if err != nil {
		return fmt.Errorf("stat pinned locks directory handle: %w", err)
	}
	if !info.IsDir() {
		return fmt.Errorf("pinned locks handle is not a directory")
	}
	return nil
}

func verifyLocksLeafHandle(f *os.File) error {
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat locks leaf handle: %w", err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("locks leaf is not a regular file")
	}
	return nil
}

func tryLockAllocatorFileHandle(*os.File) (bool, error) {
	return false, fmt.Errorf("allocator file lock unsupported on this platform")
}

func unlockAllocatorFileHandle(*os.File) error {
	return nil
}
