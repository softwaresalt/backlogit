//go:build linux || darwin || freebsd || netbsd || openbsd || dragonfly

package core

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"golang.org/x/sys/unix"
)

func readShipmentReconcileClosureEvidenceFile(rootPath, closurePath string) ([]byte, string, error) {
	rootAbs, err := filepath.Abs(filepath.Clean(rootPath))
	if err != nil {
		return nil, "", fmt.Errorf("resolve workspace root %s: %w", rootPath, err)
	}
	realRoot, err := filepath.EvalSymlinks(rootAbs)
	if err != nil {
		return nil, "", fmt.Errorf("resolve workspace root real path: %w", err)
	}
	candidateAbs := closurePath
	if !filepath.IsAbs(candidateAbs) {
		candidateAbs = filepath.Join(rootAbs, closurePath)
	}
	candidateAbs = filepath.Clean(candidateAbs)

	parentDir := filepath.Dir(candidateAbs)
	fileName := filepath.Base(candidateAbs)
	realParent, err := filepath.EvalSymlinks(parentDir)
	if err != nil {
		return nil, "", fmt.Errorf("resolve closure evidence parent %s: %w", parentDir, err)
	}
	if realParent != realRoot && !pathContained(realRoot, realParent) {
		return nil, "", fmt.Errorf("closure evidence %s resolves outside workspace root %s", closurePath, realRoot)
	}

	dirFD, err := unix.Open(realParent, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, "", fmt.Errorf("open closure evidence directory %s: %w", realParent, err)
	}
	dirFile := os.NewFile(uintptr(dirFD), realParent)
	defer dirFile.Close()

	fd, err := unix.Openat(int(dirFile.Fd()), fileName, unix.O_RDONLY|unix.O_NOFOLLOW, 0)
	if err != nil {
		return nil, "", fmt.Errorf("open closure evidence %s relative to %s: %w", fileName, realParent, err)
	}
	file := os.NewFile(uintptr(fd), filepath.Join(realParent, fileName))
	defer file.Close()

	var stat unix.Stat_t
	if err := unix.Fstat(int(file.Fd()), &stat); err != nil {
		return nil, "", fmt.Errorf("stat closure evidence %s: %w", fileName, err)
	}
	if stat.Mode&unix.S_IFMT != unix.S_IFREG {
		return nil, "", fmt.Errorf("closure evidence %s is not a regular file", closurePath)
	}
	if stat.Size <= 0 {
		return nil, "", fmt.Errorf("closure evidence %s is empty", closurePath)
	}

	body, err := io.ReadAll(file)
	if err != nil {
		return nil, "", fmt.Errorf("read closure evidence %s: %w", closurePath, err)
	}
	if len(body) == 0 {
		return nil, "", fmt.Errorf("closure evidence %s is empty", closurePath)
	}
	openedPath := filepath.Join(realParent, fileName)
	if openedPath != realRoot && !pathContained(realRoot, openedPath) {
		return nil, "", fmt.Errorf("opened closure evidence %s resolves outside workspace root %s", openedPath, realRoot)
	}
	return body, openedPath, nil
}
