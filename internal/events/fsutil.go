package events

import (
	"fmt"
	"os"
	"runtime"
)

// syncAppendLine opens path with O_CREATE|O_APPEND|O_WRONLY, writes data,
// calls Sync(), then closes. Ensures write durability before returning.
// Used for append-only JSONL queue files where each line must be durable.
func syncAppendLine(path string, data []byte) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("syncAppendLine open %s: %w", path, err)
	}
	_, writeErr := f.Write(data)
	syncErr := f.Sync()
	closeErr := f.Close()
	if writeErr != nil {
		return fmt.Errorf("syncAppendLine write %s: %w", path, writeErr)
	}
	if syncErr != nil {
		return fmt.Errorf("syncAppendLine sync %s: %w", path, syncErr)
	}
	if closeErr != nil {
		return fmt.Errorf("syncAppendLine close %s: %w", path, closeErr)
	}
	return nil
}

// syncWriteFileAtomic writes data to path via a temp-file-then-rename pattern
// with an fsync before close to guarantee durability before rename.
// On Windows, removes the destination file before renaming because os.Rename
// does not atomically overwrite an existing destination on Windows.
func syncWriteFileAtomic(path string, data []byte, perm os.FileMode) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, perm)
	if err != nil {
		return fmt.Errorf("syncWriteFileAtomic create tmp %s: %w", tmp, err)
	}
	_, writeErr := f.Write(data)
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
	// On POSIX, os.Rename atomically replaces the destination (no pre-remove needed).
	// On Windows, os.Rename fails when the destination already exists; remove first.
	// The removal window is narrow and acceptable for regenerable files.
	if runtime.GOOS == "windows" {
		_ = os.Remove(path)
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("syncWriteFileAtomic rename %s→%s: %w", tmp, path, err)
	}
	return nil
}
