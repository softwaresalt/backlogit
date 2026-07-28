package atomicfile

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// WriteFileAtomic writes data to path via a same-directory temp file and an
// atomic rename, so a partial or interrupted write can never leave a corrupt or
// truncated file in place. See the package doc for the path-agnostic contract,
// the clamped mode policy, and the deliberate sync-free design.
func WriteFileAtomic(path string, data []byte) error {
	// Mode policy: preserve the destination's existing mode but clamp the
	// group/world write bits (perm &^ 0o022); default a new file to 0644.
	mode := os.FileMode(0o644)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm() &^ 0o022
	}

	dir := filepath.Dir(path)
	// A non-".md" prefix keeps markdown scanners from picking up the temp file.
	tmp, err := os.CreateTemp(dir, ".atomicfile-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpName := tmp.Name()
	// Remove the temp file on any failure path; on success the rename has moved
	// it away and this Remove is a harmless no-op (its error is ignored).
	defer func() { _ = os.Remove(tmpName) }()

	if err := writeAll(tmp, data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temp: %w", err)
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod temp: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp: %w", err)
	}

	if err := atomicReplace(tmpName, path, false); err != nil {
		return err
	}
	return nil
}

// writeAll writes data to w through the io.Writer seam and treats a short write
// (n != len(data)) as a hard error. A real *os.File.Write never short-writes
// without also returning an error, but routing the write through an io.Writer
// makes the guard falsifiably testable with a short-writing fake and defends
// against any future writer that does.
func writeAll(w io.Writer, data []byte) error {
	n, err := w.Write(data)
	if err != nil {
		return fmt.Errorf("write: %w", err)
	}
	if n != len(data) {
		return fmt.Errorf("short write: wrote %d of %d bytes", n, len(data))
	}
	return nil
}
