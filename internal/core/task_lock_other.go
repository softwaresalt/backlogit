//go:build !windows && !(linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package core

import (
	"errors"
	"os"
)

func openTaskLockHandle(string) (*os.File, bool, error) {
	return nil, false, errors.New("task lock unsupported on this platform")
}
