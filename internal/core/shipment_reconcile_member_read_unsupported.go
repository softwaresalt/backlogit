//go:build !windows && !linux && !darwin && !freebsd && !netbsd && !openbsd && !dragonfly

package core

import (
	"fmt"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// readShipmentReconcileMemberFile fails closed on platforms without
// descriptor-relative no-follow opens.
func readShipmentReconcileMemberFile(_, path string) ([]byte, error) {
	return nil, fmt.Errorf("%w: contained member read is unsupported on this platform (%s)", blerrors.ErrValidation, path)
}
