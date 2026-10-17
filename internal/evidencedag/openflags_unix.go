//go:build unix

package evidencedag

import "syscall"

// readOnlyOpenFlags makes opening a special file (such as a FIFO) non-blocking.
const readOnlyOpenFlags = syscall.O_NONBLOCK
