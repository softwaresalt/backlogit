//go:build !windows && !linux

package cli

import (
	"fmt"

	corerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// terminateVerifiedProcess fails closed: this platform has no identity-bound
// process handle, so a PID lookup followed by a kill could hit a recycled PID.
func terminateVerifiedProcess(pid int, _ int64, _ string, _ func() error) error {
	return fmt.Errorf("%w: verified termination of pid %d is unsupported on this platform", corerrors.ErrValidation, pid)
}
