package evidencedag

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

var errNotRegular = errors.New("not a regular file")

func randHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		panic("evidencedag: system entropy unavailable: " + err.Error())
	}
	return hex.EncodeToString(b)
}

// splitCanonical resolves the directory part of path (its ancestors are
// caller-configured and trusted) and returns the canonical directory and the
// leaf name. The leaf itself is never resolved.
func splitCanonical(path string) (dir, name string, err error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", "", fmt.Errorf("resolve path: %w", err)
	}
	d, name := filepath.Split(abs)
	if name == "" {
		return "", "", errors.New("path has no file name")
	}
	canon, err := filepath.EvalSymlinks(filepath.Clean(d))
	if err != nil {
		return "", "", fmt.Errorf("resolve directory: %w", err)
	}
	return canon, name, nil
}

// checkOutsideRoots requires a non-empty list of existing roots and that
// canonPath lies beneath none of them.
func checkOutsideRoots(canonPath string, roots []string) error {
	if len(roots) == 0 {
		return errors.New("at least one forbidden root is required")
	}
	for _, r := range roots {
		if r == "" {
			return errors.New("forbidden root must not be empty")
		}
		cr, err := filepath.EvalSymlinks(r)
		if err != nil {
			return fmt.Errorf("resolve forbidden root: %w", err)
		}
		if within(cr, canonPath) {
			return errors.New("path lies inside an agent-writable root")
		}
	}
	return nil
}

// openBeneath opens the leaf name directly beneath the canonical directory
// dir. It refuses a leaf that is not a regular file (symlinks, reparse points,
// directories, devices), never follows one, and proves the opened descriptor is
// the same regular file that was inspected. A missing leaf is reported with
// fs.ErrNotExist unless flag includes os.O_CREATE.
func openBeneath(dir, name string, flag int, perm os.FileMode) (*os.File, error) {
	r, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	li, lerr := r.Lstat(name)
	switch {
	case lerr == nil:
		if !li.Mode().IsRegular() {
			return nil, errNotRegular
		}
	case errors.Is(lerr, fs.ErrNotExist):
		if flag&os.O_CREATE == 0 {
			return nil, lerr
		}
	default:
		return nil, lerr
	}
	if flag&(os.O_WRONLY|os.O_RDWR) == 0 {
		flag |= readOnlyOpenFlags
	}
	f, err := r.OpenFile(name, flag, perm)
	if err != nil {
		return nil, err
	}
	fi, err := f.Stat()
	if err != nil || !fi.Mode().IsRegular() || (lerr == nil && !os.SameFile(li, fi)) {
		_ = f.Close()
		return nil, errNotRegular
	}
	return f, nil
}

// readRegularBounded reads a regular file up to max bytes. exists is false only
// for a missing leaf.
func readRegularBounded(dir, name string, max int) (data []byte, exists bool, err error) {
	f, err := openBeneath(dir, name, os.O_RDONLY, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, true, err
	}
	defer f.Close()
	raw, err := io.ReadAll(io.LimitReader(f, int64(max)+1))
	if err != nil {
		return nil, true, err
	}
	if len(raw) > max {
		return nil, true, errTooLarge
	}
	return raw, true, nil
}

var errTooLarge = errors.New("file exceeds its size bound")

// createExclusive creates name beneath dir, failing if anything (including a
// planted symlink) already exists there.
func createExclusive(dir, name string, perm os.FileMode) (*os.File, error) {
	r, err := os.OpenRoot(dir)
	if err != nil {
		return nil, err
	}
	defer r.Close()
	return r.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, perm)
}

// writeFileAtomic writes data to a randomly named exclusive temp file, syncs
// it and renames it over name, retrying the rename with backoff because
// Windows can transiently refuse it. A rename failure is returned wrapped in
// errRenameFailed; the target is then unchanged.
func writeFileAtomic(dir, name string, data []byte) error {
	tmp := name + ".tmp-" + randHex(8)
	f, err := createExclusive(dir, tmp, 0o600)
	if err != nil {
		return fmt.Errorf("stage %s: %w", name, err)
	}
	cleanup := func() { _ = os.Remove(filepath.Join(dir, tmp)) }
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		cleanup()
		return fmt.Errorf("write %s: %w", name, err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		cleanup()
		return fmt.Errorf("sync %s: %w", name, err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return fmt.Errorf("close %s: %w", name, err)
	}
	var rerr error
	delay := 10 * time.Millisecond
	for i := 0; i < 6; i++ {
		if rerr = renameFile(filepath.Join(dir, tmp), filepath.Join(dir, name)); rerr == nil {
			syncDir(dir)
			return nil
		}
		time.Sleep(delay)
		delay *= 2
	}
	cleanup()
	return fmt.Errorf("%w: %s: %v", errRenameFailed, name, rerr)
}

var errRenameFailed = errors.New("rename failed")

// renameFile is a seam so tests can inject rename failures.
var renameFile = os.Rename
