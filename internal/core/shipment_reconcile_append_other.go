//go:build !windows && !(linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package core

import "errors"

func appendShipmentReconcileEventHandleRelative(string, string, []byte) (shipmentReconcileAppendResult, error) {
	return shipmentReconcileAppendResult{}, errors.New("shipment reconcile event append unsupported on this platform")
}
