package db

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// snapshotSource pins the caller-bound source directory. Neither the directory
// nor its leaves are resolved through symlinks to establish a new trust root.
type snapshotSource struct {
	parent *os.File
	path   string
	name   string
}

func openSnapshotSource(path string) (*snapshotSource, error) {
	path, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("resolve snapshot source: %w", err)
	}
	dir := filepath.Dir(path)
	// macOS exposes /var as a fixed system alias for /private/var. Resolve
	// only that exact alias; all remaining components still use no-follow.
	if runtime.GOOS == "darwin" && (dir == "/var" || strings.HasPrefix(dir, "/var/")) {
		info, statErr := os.Lstat("/var")
		if statErr != nil {
			return nil, fmt.Errorf("inspect system var alias: %w", statErr)
		}
		if info.Mode()&os.ModeSymlink != 0 {
			target, linkErr := os.Readlink("/var")
			if linkErr != nil {
				return nil, fmt.Errorf("read system var alias: %w", linkErr)
			}
			if target != "private/var" && target != "/private/var" {
				return nil, fmt.Errorf("redirected system var alias: %w", os.ErrPermission)
			}
			dir = "/private" + dir
		}
	}
	// Check every ancestor before the platform's capability-bound open.
	for current := dir; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil {
			return nil, fmt.Errorf("inspect snapshot directory: %w", err)
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return nil, fmt.Errorf("redirected snapshot directory: %w", os.ErrPermission)
		}
		if filepath.Dir(current) == current {
			break
		}
	}
	parent, canonical, err := pinSnapshotDirectory(dir)
	if err != nil {
		return nil, fmt.Errorf("pin snapshot directory: %w", err)
	}
	return &snapshotSource{parent: parent, path: canonical, name: filepath.Base(path)}, nil
}

func (s *snapshotSource) close() error { return s.parent.Close() }

func (s *snapshotSource) openChecked(name string) (*os.File, error) {
	if name == "" || strings.ContainsAny(name, `/\:`) || name == "." || name == ".." {
		return nil, fmt.Errorf("invalid snapshot leaf: %w", os.ErrPermission)
	}
	file, err := s.open(name)
	if err != nil {
		return nil, fmt.Errorf("open snapshot leaf %s: %w", name, err)
	}
	info, err := file.Stat()
	if err != nil {
		return nil, errors.Join(fmt.Errorf("stat snapshot leaf: %w", err), file.Close())
	}
	if !info.Mode().IsRegular() {
		return nil, errors.Join(fmt.Errorf("nonregular snapshot leaf: %w", os.ErrPermission), file.Close())
	}
	if err := s.check(file, name); err != nil {
		return nil, errors.Join(err, file.Close())
	}
	return file, nil
}

func readCheckedSnapshotFile(file *os.File) ([]byte, os.FileInfo, error) {
	if file == nil {
		return nil, nil, nil
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, nil, fmt.Errorf("seek snapshot leaf: %w", err)
	}
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, nil, fmt.Errorf("read snapshot leaf: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		return nil, nil, fmt.Errorf("stat snapshot leaf: %w", err)
	}
	return data, info, nil
}

func (s *snapshotSource) read(name string) (data []byte, info os.FileInfo, resultErr error) {
	if name == "" || strings.ContainsAny(name, `/\:`) || name == "." || name == ".." {
		return nil, nil, fmt.Errorf("invalid snapshot leaf: %w", os.ErrPermission)
	}
	file, err := s.open(name)
	if err != nil {
		return nil, nil, fmt.Errorf("open snapshot leaf %s: %w", name, err)
	}
	defer func() { resultErr = errors.Join(resultErr, file.Close()) }()
	info, err = file.Stat()
	if err != nil {
		return nil, nil, fmt.Errorf("stat snapshot leaf: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, nil, fmt.Errorf("nonregular snapshot leaf: %w", os.ErrPermission)
	}
	if err := s.check(file, name); err != nil {
		return nil, nil, err
	}
	data, err = io.ReadAll(file)
	if err != nil {
		return nil, nil, fmt.Errorf("read snapshot leaf: %w", err)
	}
	if err := s.check(file, name); err != nil {
		return nil, nil, err
	}
	return data, info, nil
}
