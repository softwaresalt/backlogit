//go:build !windows && !(linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package core

import (
	"errors"
	"os"
)

func openShipmentReconcileLockHandleRelative(string, string) (*os.File, bool, error) {
	return nil, false, errors.New("shipment reconcile item log lock unsupported on this platform")
}
