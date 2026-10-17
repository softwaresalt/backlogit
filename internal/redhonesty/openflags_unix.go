//go:build unix

package redhonesty

import "syscall"

// readOnlyOpenFlags makes opening a special file (such as a FIFO) non-blocking.
const readOnlyOpenFlags = syscall.O_NONBLOCK
