//go:build !windows

package core

import (
	"fmt"
	"os"
	"syscall"
)

// openCustodyNoFollow opens path read-only with O_NOFOLLOW, so the kernel
// refuses a symlink at the final component. O_NONBLOCK keeps a FIFO swapped in
// after the Lstat pre-check from blocking the open.
func openCustodyNoFollow(path string) (*os.File, error) {
	fd, err := syscall.Open(path, syscall.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK|syscall.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("open %s without following links: %w", path, err)
	}
	return os.NewFile(uintptr(fd), path), nil
}
