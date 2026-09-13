//go:build !windows && !(linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package core

import "errors"

func writeShipmentReconcileArchiveFileHandleRelative(string, string, []byte, shipmentReconcileFSSeams) error {
	return errors.New("shipment reconcile archive file writer unsupported on this platform")
}
