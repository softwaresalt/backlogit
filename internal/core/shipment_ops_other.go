//go:build !windows && !(linux || darwin || freebsd || netbsd || openbsd || dragonfly)

package core

import (
	"fmt"
	"os"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

func openShipmentOpsDirectory(string, string) (*os.File, error) {
	return nil, fmt.Errorf("shipment operation journals are unsupported on this platform: %w",
		blerrors.ErrValidation)
}

func readShipmentOperationJournalFile(*os.File, string, string) ([]byte, error) {
	return nil, fmt.Errorf("shipment operation journal reads are unsupported on this platform: %w",
		blerrors.ErrValidation)
}

func writeShipmentOperationJournalFile(*os.File, string, string, []byte) error {
	return fmt.Errorf("shipment operation journal writes are unsupported on this platform: %w",
		blerrors.ErrValidation)
}

func removeShipmentOperationJournalTempFile(*os.File, string, string) error {
	return fmt.Errorf("shipment operation journal temp cleanup is unsupported on this platform: %w",
		blerrors.ErrValidation)
}
