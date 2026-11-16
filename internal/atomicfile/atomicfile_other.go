//go:build !windows

package atomicfile

import (
	"fmt"
	"os"

	"golang.org/x/sys/unix"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// atomicReplace renames tmpName over path with a plain POSIX os.Rename, which
// replaces the destination atomically without ever removing it first. The
// durable argument is unused here: on POSIX the rename's own durability is
// provided by the parent-directory fsync in WriteFileAtomicWithOptions (durable
// mode), not by a per-rename flag. On failure the destination is untouched, so
// the error is classified ErrWriteNotApplied.
func atomicReplace(tmpName, path string, durable bool) error {
	_ = durable // POSIX rename needs no durability flag; see doc above.
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp over destination: %w",
			fmt.Errorf("%w: %w", blerrors.ErrWriteNotApplied, err))
	}
	return nil
}

func replaceTempFileInRootPlatform(
	_ *os.Root,
	dirFile *os.File,
	_ string,
	tmpRel, dstRel string,
	_, _ os.FileInfo,
	durable bool,
) error {
	dirFD := int(dirFile.Fd())
	if err := unix.Renameat(dirFD, tmpRel, dirFD, dstRel); err != nil {
		return notApplied("renameat temp over destination", err)
	}
	if durable {
		if err := dirFile.Sync(); err != nil {
			return fmt.Errorf("fsync pinned parent dir after renameat: %w",
				fmt.Errorf("%w: %w", blerrors.ErrWriteIndeterminate, err))
		}
	}
	return nil
}
