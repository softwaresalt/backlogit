package events

import (
	"fmt"
	"os"

	"github.com/softwaresalt/backlogit/internal/atomicfile"
)

// syncAppendResult classifies how a durable append terminated so callers can map
// the outcome onto the two-class error contract: a pre-write open failure means
// nothing was appended (safe to retry), while a partial write or a post-write
// fsync/close failure is indeterminate because an append is not atomic.
type syncAppendResult struct {
	// preWrite is true when the failure occurred before any bytes could be
	// written (the file open failed). It is meaningless when err is nil.
	preWrite bool
	// err is the terminating error, or nil on success.
	err error
}

// syncAppendLineDetailed is the shared file-fsync append primitive: it opens path
// with O_CREATE|O_APPEND|O_WRONLY, writes data, fsyncs (via syncFile, defaulting
// to f.Sync()), then closes, returning a classified result. Both the hook-event
// queue path (syncAppendLine) and the item-log path (EventWriter.appendDurable)
// funnel through it so the two append implementations cannot drift.
func syncAppendLineDetailed(path string, data []byte, syncFile func(*os.File) error) syncAppendResult {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return syncAppendResult{preWrite: true, err: fmt.Errorf("open %s: %w", path, err)}
	}
	n, writeErr := f.Write(data)
	if writeErr == nil && n != len(data) {
		writeErr = fmt.Errorf("short write %s: wrote %d of %d bytes", path, n, len(data))
	}
	syncErr := writeErr
	if syncErr == nil {
		if syncFile != nil {
			syncErr = syncFile(f)
		} else {
			syncErr = f.Sync()
		}
	}
	closeErr := f.Close()
	if writeErr != nil {
		return syncAppendResult{err: fmt.Errorf("write %s: %w", path, writeErr)}
	}
	if syncErr != nil {
		return syncAppendResult{err: fmt.Errorf("sync %s: %w", path, syncErr)}
	}
	if closeErr != nil {
		return syncAppendResult{err: fmt.Errorf("close %s: %w", path, closeErr)}
	}
	return syncAppendResult{}
}

// syncAppendLine opens path with O_CREATE|O_APPEND|O_WRONLY, writes data,
// calls Sync(), then closes. Ensures write durability before returning.
// Used for append-only JSONL queue files where each line must be durable.
func syncAppendLine(path string, data []byte) error {
	if res := syncAppendLineDetailed(path, data, nil); res.err != nil {
		return fmt.Errorf("syncAppendLine %s: %w", path, res.err)
	}
	return nil
}

// syncWriteFileAtomicHook is the package-level, test-swappable seam for
// checkpoint create writes, mirroring the established checkpointAuditAppendFn
// pattern (checkpoint_audit.go). Tests that override this variable must not
// run with t.Parallel(). 148-F / U3: default converges onto
// atomicfile.WriteFileAtomicWithOptions so real write failures are classified
// as ErrWriteNotApplied or ErrWriteIndeterminate by the atomicfile layer,
// preserving errors.Is traversal through the fmt.Errorf("write checkpoint: %w")
// wrapping in CreateCheckpoint.
var syncWriteFileAtomicHook = func(path string, data []byte, _ os.FileMode) error {
	// Route through atomicfile with DurableWrites for outcome classification
	// (148-F / U3): ErrWriteNotApplied for pre-rename failures,
	// ErrWriteIndeterminate for post-rename fsync failures. DurableWrites
	// preserves the pre-existing syncWriteFileAtomic behaviour of fsyncing
	// the file before close (Copilot review: using WriteFileAtomic without
	// durability regressed checkpoint write safety).
	return atomicfile.WriteFileAtomicWithOptions(path, data, atomicfile.Options{DurableWrites: true})
}

// syncWriteFileAtomic writes data to path via a temp-file-then-rename pattern
// with an fsync before close to guarantee durability before rename.
// On POSIX, rename(2) is atomic by specification. On Windows, Go 1.24.0 uses
// MoveFileExW(MOVEFILE_REPLACE_EXISTING), which replaces the destination
// without a pre-Remove step and eliminates the two-file loss window; it does
// not provide the full crash-atomicity of POSIX rename(2). (149-F / CB71B412)
func syncWriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("syncWriteFileAtomic create tmp %s: %w", tmp, err)
	}
	n, writeErr := f.Write(data)
	if writeErr == nil && n != len(data) {
		writeErr = fmt.Errorf("syncWriteFileAtomic short write %s: wrote %d of %d bytes", tmp, n, len(data))
	}
	syncErr := f.Sync()
	closeErr := f.Close()
	if writeErr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("syncWriteFileAtomic write %s: %w", tmp, writeErr)
	}
	if syncErr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("syncWriteFileAtomic sync %s: %w", tmp, syncErr)
	}
	if closeErr != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("syncWriteFileAtomic close %s: %w", tmp, closeErr)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("syncWriteFileAtomic rename %s→%s: %w", tmp, path, err)
	}
	return nil
}
