//go:build !windows && !(linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package atomicfile

import (
	"errors"
	"fmt"
	"os"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

func atomicReplace(tmpName, path string, durable bool) error {
	_ = durable // Fallback rename has no durability flag.
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename temp over destination: %w",
			fmt.Errorf("%w: %w", blerrors.ErrWriteNotApplied, err))
	}
	return nil
}

func replaceTempFileInRootPlatform(
	_ *os.Root,
	_ *os.File,
	_ string,
	_, _ string,
	_, _ os.FileInfo,
	_ bool,
) error {
	return notApplied("renameat temp over destination", errors.ErrUnsupported)
}
