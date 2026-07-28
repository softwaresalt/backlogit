//go:build !windows

package atomicfile

import (
	"fmt"
	"os"

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
