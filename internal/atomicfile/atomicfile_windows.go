//go:build windows

package atomicfile

import (
	"fmt"

	"golang.org/x/sys/windows"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// moveFileExFlags computes the MoveFileEx flag word for an atomic same-volume
// replace. It splits the two concerns the legacy remove-before-rename fallback
// conflated: MOVEFILE_REPLACE_EXISTING (atomic-replace SAFETY) is set
// UNCONDITIONALLY because removing the destination first was a real data-loss
// defect, while MOVEFILE_WRITE_THROUGH (a synchronous durability flush) is added
// ONLY when durable is on so the off-mode fast path pays no write-through
// latency. No MOVEFILE_COPY_ALLOWED: the temp file is same-directory/same-volume.
func moveFileExFlags(durable bool) uint32 {
	flags := uint32(windows.MOVEFILE_REPLACE_EXISTING)
	if durable {
		flags |= windows.MOVEFILE_WRITE_THROUGH
	}
	return flags
}

// moveFileEx is the injectable MoveFileEx syscall seam. Tests override it to
// assert the gated flag word and to simulate replace failures in-process
// (a process kill cannot return an error to the caller, so it cannot exercise
// the fail-closed classification). Production wires the real Windows syscall.
var moveFileEx = func(from, to string, flags uint32) error {
	fromPtr, err := windows.UTF16PtrFromString(from)
	if err != nil {
		return fmt.Errorf("encode source path: %w", err)
	}
	toPtr, err := windows.UTF16PtrFromString(to)
	if err != nil {
		return fmt.Errorf("encode destination path: %w", err)
	}
	return windows.MoveFileEx(fromPtr, toPtr, flags)
}

// atomicReplace renames tmpName over path using MoveFileEx so the destination
// is replaced atomically and is NEVER removed before the replacement is in
// place. ReplaceFileW is deliberately rejected (its write-through story is
// unreliable and its failure modes can leave the destination absent). On
// failure the destination is untouched, so the error is classified
// ErrWriteNotApplied.
func atomicReplace(tmpName, path string, durable bool) error {
	if err := moveFileEx(tmpName, path, moveFileExFlags(durable)); err != nil {
		return fmt.Errorf("move temp over destination: %w",
			fmt.Errorf("%w: %w", blerrors.ErrWriteNotApplied, err))
	}
	return nil
}
