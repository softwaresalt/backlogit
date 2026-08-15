//go:build !windows && !(linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package events

import (
	"errors"
	"os"
)

func openItemLogLockHandle(string) (*os.File, bool, error) {
	return nil, false, errors.New("item log lock unsupported on this platform")
}
