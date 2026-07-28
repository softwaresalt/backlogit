package atomicfile

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// Options controls optional durability behavior of an atomic write.
type Options struct {
	// DurableWrites opts into the durable_writes fsync protocol (123-F): the
	// temp file is fsynced before close and, on POSIX, the parent directory is
	// fsynced after a successful rename so a crash/power-loss cannot lose the
	// just-written file or its new dirent. Default false is the fast path that
	// adds no fsync cost.
	DurableWrites bool
}

// durableSeams carries the injectable fsync seams and the platform gate for the
// parent-directory fsync. Production wires os fsyncs; tests override them to
// assert ordering and to simulate post-rename failures in-process (a process
// kill cannot return an error to the caller nor simulate power loss).
type durableSeams struct {
	// dirSyncEnabled gates the parent-directory fsync. It is (runtime.GOOS !=
	// "windows") in production because Windows has no directory-handle flush.
	dirSyncEnabled bool
	// syncFile fsyncs the temp file before close.
	syncFile func(*os.File) error
	// syncDir fsyncs the parent-directory handle after a successful rename.
	syncDir func(*os.File) error
}

// defaultDurableSeams returns the production fsync seams.
func defaultDurableSeams() durableSeams {
	return durableSeams{
		dirSyncEnabled: runtime.GOOS != "windows",
		syncFile:       func(f *os.File) error { return f.Sync() },
		syncDir:        func(d *os.File) error { return d.Sync() },
	}
}

// WriteFileAtomic writes data to path via a same-directory temp file and an
// atomic rename, so a partial or interrupted write can never leave a corrupt or
// truncated file in place. It is the durable-off wrapper (no added fsync); see
// WriteFileAtomicWithOptions to opt into the durable_writes fsync protocol. See
// the package doc for the path-agnostic contract, the clamped mode policy, and
// the durability semantics.
func WriteFileAtomic(path string, data []byte) error {
	return WriteFileAtomicWithOptions(path, data, Options{})
}

// WriteFileAtomicWithOptions is WriteFileAtomic with opt-in durability. When
// opts.DurableWrites is set it fsyncs the temp file before close, opens the
// parent-directory handle before the rename, and (on POSIX) fsyncs that handle
// after a successful rename. Any failure before the rename commits is classified
// ErrWriteNotApplied (the destination is untouched, safe to retry the atomic
// write); a parent-dir fsync failure after the rename is ErrWriteIndeterminate
// (the file is already replaced, so the outcome is uncertain — do not blindly
// retry). With durability off it is the sync-free fast path.
func WriteFileAtomicWithOptions(path string, data []byte, opts Options) error {
	return writeFileAtomic(path, data, opts.DurableWrites, defaultDurableSeams())
}

// writeFileAtomic is the seam-injectable implementation shared by the public
// entrypoints and the durability tests.
func writeFileAtomic(path string, data []byte, durable bool, seams durableSeams) error {
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
		return notApplied("create temp", err)
	}
	tmpName := tmp.Name()
	// Remove the temp file on any failure path; on success the rename has moved
	// it away and this Remove is a harmless no-op (its error is ignored).
	defer func() { _ = os.Remove(tmpName) }()

	if err := writeAll(tmp, data); err != nil {
		_ = tmp.Close()
		return notApplied("write temp", err)
	}
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return notApplied("chmod temp", err)
	}
	if durable {
		if err := seams.syncFile(tmp); err != nil {
			_ = tmp.Close()
			return notApplied("fsync temp", err)
		}
	}
	if err := tmp.Close(); err != nil {
		return notApplied("close temp", err)
	}

	// Durable POSIX: open the parent-directory handle BEFORE the rename so the
	// post-rename fsync uses a handle acquired ahead of the mutation.
	var dirHandle *os.File
	if durable && seams.dirSyncEnabled {
		dh, openErr := os.Open(dir)
		if openErr != nil {
			return notApplied("open parent dir", openErr)
		}
		dirHandle = dh
		defer func() { _ = dirHandle.Close() }()
	}

	if err := atomicReplace(tmpName, path, durable); err != nil {
		// atomicReplace already classifies its failure as ErrWriteNotApplied.
		return err
	}

	// The rename has committed and the new content is visible. A parent-dir
	// fsync failure now cannot un-apply the write, so it is indeterminate.
	if durable && seams.dirSyncEnabled {
		if err := seams.syncDir(dirHandle); err != nil {
			return fmt.Errorf("fsync parent dir after rename: %w",
				fmt.Errorf("%w: %w", blerrors.ErrWriteIndeterminate, err))
		}
	}
	return nil
}

// notApplied wraps a pre-rename failure with the ErrWriteNotApplied class: the
// destination is untouched, so the failed atomic write is safe to retry.
func notApplied(context string, err error) error {
	return fmt.Errorf("%s: %w", context,
		fmt.Errorf("%w: %w", blerrors.ErrWriteNotApplied, err))
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
