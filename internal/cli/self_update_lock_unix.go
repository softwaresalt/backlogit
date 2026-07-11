//go:build unix

package cli

import (
	"errors"
	"os"
	"syscall"
)

func openSelfUpdateLockFile(lockPath string) (*os.File, bool, error) {
	file, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, false, err
	}
	err = syscall.Flock(int(file.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		if closeErr := file.Close(); closeErr != nil {
			return nil, false, errors.Join(err, closeErr)
		}
		if errors.Is(err, syscall.EWOULDBLOCK) || errors.Is(err, syscall.EAGAIN) {
			return nil, false, nil
		}
		return nil, false, err
	}
	return file, true, nil
}
