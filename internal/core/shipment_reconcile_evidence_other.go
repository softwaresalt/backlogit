//go:build !windows && !(linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package core

import "errors"

func readShipmentReconcileClosureEvidenceFile(string, string) ([]byte, string, error) {
	return nil, "", errors.New("shipment reconcile closure evidence read unsupported on this platform")
}
