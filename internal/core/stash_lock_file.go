package core

import (
	"errors"
	"fmt"
	"os"
	"sync"
)

// stashLockFile owns the leaf and, on Unix, its no-follow parent pin.
// Explicit Close handles acquisition failures as well as normal release;
// neither resource depends on a finalizer or a process-global owner registry.
type stashLockFile struct {
	*os.File
	parent    *os.File
	basename  string
	closeOnce sync.Once
	closeErr  error
}

// Close releases both resources exactly once, including when leaf close fails.
// It does not remove a sidecar: only verified release may authorize deletion.
func (file *stashLockFile) Close() error {
	file.closeOnce.Do(func() {
		leafErr := file.File.Close()
		var parentErr error
		if file.parent != nil {
			parentErr = file.parent.Close()
		}
		if leafErr != nil || parentErr != nil {
			file.closeErr = fmt.Errorf("close stash sidecar %s ownership: %w", file.basename, errors.Join(leafErr, parentErr))
		}
	})
	return file.closeErr
}
