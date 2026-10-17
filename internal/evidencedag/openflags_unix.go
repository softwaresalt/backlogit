//go:build unix

package evidencedag

import (
	"errors"
	"os"
	"syscall"
)

// readOnlyOpenFlags makes opening a special file (such as a FIFO) non-blocking.
const readOnlyOpenFlags = syscall.O_NONBLOCK

// syncDir makes a rename or file creation durable by fsyncing dir. Filesystems
// that do not support fsync on a directory (EINVAL/ENOTSUP) are treated as
// unsupported and not an error; every other failure is returned.
func syncDir(dir string) error {
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	serr := d.Sync()
	if errors.Is(serr, syscall.EINVAL) || errors.Is(serr, syscall.ENOTSUP) {
		serr = nil
	}
	return errors.Join(serr, d.Close())
}
