//go:build !windows && !(linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package cli

import (
	"errors"
	"os"
)

func openSelfUpdateLockFile(lockPath string) (*os.File, bool, error) {
	return nil, false, errors.New("self-update lock unsupported on this platform")
}
