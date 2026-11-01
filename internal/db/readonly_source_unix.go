//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package db

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

func pinSnapshotDirectory(dir string) (*os.File, string, error) {
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, "", fmt.Errorf("open filesystem root: %w", err)
	}
	for _, part := range strings.Split(strings.TrimPrefix(dir, "/"), "/") {
		if part == "" {
			continue
		}
		next, openErr := unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		closeErr := unix.Close(fd)
		if openErr != nil || closeErr != nil {
			if openErr == nil {
				closeErr = errors.Join(closeErr, unix.Close(next))
			}
			return nil, "", fmt.Errorf("open snapshot ancestor: %w", errors.Join(openErr, closeErr))
		}
		fd = next
	}
	parent := os.NewFile(uintptr(fd), dir)
	opened, err := parent.Stat()
	current, statErr := os.Lstat(dir)
	if err != nil || statErr != nil || !os.SameFile(opened, current) {
		return nil, "", fmt.Errorf("snapshot directory changed: %w", errors.Join(os.ErrPermission, err, statErr, parent.Close()))
	}
	return parent, dir, nil
}

func (s *snapshotSource) open(name string) (*os.File, error) {
	fd, err := unix.Openat(int(s.parent.Fd()), name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, fmt.Errorf("open rooted snapshot file: %w", err)
	}
	return os.NewFile(uintptr(fd), filepath.Join(s.path, name)), nil
}

func (s *snapshotSource) check(file *os.File, name string) error {
	var owned, current unix.Stat_t
	if err := unix.Fstat(int(file.Fd()), &owned); err != nil {
		return fmt.Errorf("stat snapshot handle: %w", err)
	}
	if err := unix.Fstatat(int(s.parent.Fd()), name, &current, unix.AT_SYMLINK_NOFOLLOW); err != nil {
		return fmt.Errorf("stat snapshot basename: %w", err)
	}
	if owned.Mode&unix.S_IFMT != unix.S_IFREG || owned.Nlink != 1 ||
		current.Mode&unix.S_IFMT != unix.S_IFREG || current.Nlink != 1 ||
		owned.Dev != current.Dev || owned.Ino != current.Ino {
		return fmt.Errorf("linked or replaced snapshot file: %w", os.ErrPermission)
	}
	return nil
}
