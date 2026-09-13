//go:build windows

package core

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
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
	realParent, err := filepath.EvalSymlinks(parentDir)
	if err != nil {
		return nil, "", fmt.Errorf("resolve closure evidence parent %s: %w", parentDir, err)
	}
	if realParent != realRoot && !pathContained(realRoot, realParent) {
		return nil, "", fmt.Errorf("closure evidence %s resolves outside workspace root %s", closurePath, realRoot)
	}

	name, err := windows.UTF16PtrFromString(candidateAbs)
	if err != nil {
		return nil, "", fmt.Errorf("encode closure evidence path %s: %w", candidateAbs, err)
	}
	handle, err := windows.CreateFile(
		name,
		windows.GENERIC_READ,
		windows.FILE_SHARE_READ,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_ATTRIBUTE_NORMAL|windows.FILE_FLAG_OPEN_REPARSE_POINT,
		0,
	)
	if err != nil {
		return nil, "", fmt.Errorf("open closure evidence %s: %w", candidateAbs, err)
	}
	file := os.NewFile(uintptr(handle), candidateAbs)
	defer file.Close()

	var info windows.ByHandleFileInformation
	if err := windows.GetFileInformationByHandle(handle, &info); err != nil {
		return nil, "", fmt.Errorf("stat closure evidence %s: %w", candidateAbs, err)
	}
	if info.FileAttributes&windows.FILE_ATTRIBUTE_REPARSE_POINT != 0 {
		return nil, "", fmt.Errorf("closure evidence %s is a reparse point", closurePath)
	}
	finalPath, err := shipmentReconcileWindowsFinalPath(handle)
	if err != nil {
		return nil, "", fmt.Errorf("resolve final path for closure evidence %s: %w", candidateAbs, err)
	}
	if finalPath != realRoot && !pathContained(realRoot, finalPath) {
		return nil, "", fmt.Errorf("closure evidence %s resolves outside workspace root %s", finalPath, realRoot)
	}
	stat, err := file.Stat()
	if err != nil {
		return nil, "", fmt.Errorf("stat closure evidence file %s: %w", closurePath, err)
	}
	if !stat.Mode().IsRegular() {
		return nil, "", fmt.Errorf("closure evidence %s is not a regular file", closurePath)
	}
	if stat.Size() <= 0 {
		return nil, "", fmt.Errorf("closure evidence %s is empty", closurePath)
	}
	body, err := io.ReadAll(file)
	if err != nil {
		return nil, "", fmt.Errorf("read closure evidence %s: %w", closurePath, err)
	}
	if len(body) == 0 {
		return nil, "", fmt.Errorf("closure evidence %s is empty", closurePath)
	}
	return body, finalPath, nil
}

func shipmentReconcileWindowsFinalPath(handle windows.Handle) (string, error) {
	buf := make([]uint16, 260)
	for {
		n, err := windows.GetFinalPathNameByHandle(handle, &buf[0], uint32(len(buf)), 0)
		if err != nil {
			return "", err
		}
		if n < uint32(len(buf)) {
			return filepath.Clean(trimWindowsPathPrefix(windows.UTF16ToString(buf[:n]))), nil
		}
		buf = make([]uint16, n+1)
	}
}

func trimWindowsPathPrefix(path string) string {
	switch {
	case strings.HasPrefix(path, `\\?\UNC\`):
		return `\\` + strings.TrimPrefix(path, `\\?\UNC\`)
	case strings.HasPrefix(path, `\\?\`):
		return strings.TrimPrefix(path, `\\?\`)
	case strings.HasPrefix(path, `\??\`):
		return strings.TrimPrefix(path, `\??\`)
	default:
		return path
	}
}
