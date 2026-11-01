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

func openStashLockHandle(path string) (*os.File, bool, error) {
	return openTaskLockHandle(path)
}

func releaseStashSidecarFile(file *os.File) error {
	if err := file.Close(); err != nil {
		return fmt.Errorf("close unsupported stash sidecar: %w", err)
	}
	return nil
}
