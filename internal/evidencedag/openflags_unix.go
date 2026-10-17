//go:build unix

package evidencedag

import (
	"os"
	"syscall"
)

// readOnlyOpenFlags makes opening a special file (such as a FIFO) non-blocking.
const readOnlyOpenFlags = syscall.O_NONBLOCK

// syncDir makes a rename durable. Errors are advisory: the data is already
// synced and the rename visible.
func syncDir(dir string) {
	if d, err := os.Open(dir); err == nil {
		_ = d.Sync()
		_ = d.Close()
	}
}
