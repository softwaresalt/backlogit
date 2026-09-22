//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package core

import (
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

func openShipmentOpsDirectory(_ string, opsRoot string) (*os.File, error) {
	fd, err := unix.Open(opsRoot, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, fmt.Errorf("open shipment operations directory: %w", err)
	}
	return os.NewFile(uintptr(fd), opsRoot), nil
}

func readShipmentOperationJournalFile(dir *os.File, _ string, name string) ([]byte, error) {
	fd, err := unix.Openat(int(dir.Fd()), name, unix.O_RDONLY|unix.O_NOFOLLOW, 0)
	if err != nil {
		if errors.Is(err, unix.ELOOP) {
			return nil, fmt.Errorf("shipment operation journal %s is redirected: %w",
				name, blerrors.ErrValidation)
		}
		return nil, err
	}
	file := os.NewFile(uintptr(fd), name)
	defer file.Close()
	info, err := file.Stat()
	if err != nil {
		return nil, fmt.Errorf("stat shipment operation journal %s: %w", name, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("shipment operation journal %s is not regular: %w",
			name, blerrors.ErrValidation)
	}
	return io.ReadAll(file)
}

func writeShipmentOperationJournalFile(dir *os.File, _ string, name string, data []byte) error {
	var existing unix.Stat_t
	err := unix.Fstatat(int(dir.Fd()), name, &existing, unix.AT_SYMLINK_NOFOLLOW)
	if err == nil && existing.Mode&unix.S_IFMT != unix.S_IFREG {
		return fmt.Errorf("shipment operation journal %s is not regular: %w",
			name, blerrors.ErrValidation)
	}
	if err != nil && !errors.Is(err, unix.ENOENT) {
		return fmt.Errorf("inspect shipment operation journal %s: %w", name, err)
	}

	tempName := ".shipment-operation-" + name + ".tmp"
	fd, err := unix.Openat(
		int(dir.Fd()),
		tempName,
		unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW,
		0o600,
	)
	if errors.Is(err, unix.EEXIST) {
		if unlinkErr := unix.Unlinkat(int(dir.Fd()), tempName, 0); unlinkErr != nil {
			return fmt.Errorf("remove stale shipment operation temp file: %w", unlinkErr)
		}
		fd, err = unix.Openat(
			int(dir.Fd()),
			tempName,
			unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW,
			0o600,
		)
	}
	if err != nil {
		return fmt.Errorf("create shipment operation temp file: %w", err)
	}
	temp := os.NewFile(uintptr(fd), tempName)
	cleanup := true
	defer func() {
		_ = temp.Close()
		if cleanup {
			_ = unix.Unlinkat(int(dir.Fd()), tempName, 0)
		}
	}()
	if _, err := temp.Write(data); err != nil {
		return fmt.Errorf("write shipment operation temp file: %w", err)
	}
	if err := temp.Sync(); err != nil {
		return fmt.Errorf("sync shipment operation temp file: %w", err)
	}
	if err := temp.Close(); err != nil {
		return fmt.Errorf("close shipment operation temp file: %w", err)
	}
	if err := unix.Renameat(int(dir.Fd()), tempName, int(dir.Fd()), name); err != nil {
		return fmt.Errorf("replace shipment operation journal: %w", err)
	}
	cleanup = false
	if err := dir.Sync(); err != nil {
		return fmt.Errorf("sync shipment operations directory: %w", err)
	}
	return nil
}
