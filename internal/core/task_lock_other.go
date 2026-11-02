//go:build !windows && !(linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package core

import (
	"errors"
	"fmt"
	"os"
)

func openTaskLockHandle(string) (*os.File, bool, error) {
	return nil, false, errors.New("task lock unsupported on this platform")
}

func openStashLockHandle(string) (*stashLockFile, bool, error) {
	return nil, false, fmt.Errorf("stash lock unsupported on this platform: %w", errors.ErrUnsupported)
}

func releaseStashSidecarFile(file *stashLockFile) error {
	if err := file.Close(); err != nil {
		return fmt.Errorf("close unsupported stash sidecar: %w", err)
	}
	return nil
}
