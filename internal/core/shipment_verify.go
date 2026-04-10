package core

// shipment_verify.go provides post-shipment consistency checks.
// 025.018-T (Unit 6): After ShipShipment archives items, VerifyPostShipConsistency
// confirms that all archived item IDs have been removed from the queue directory.

import (
	"context"
	"fmt"
)

// VerifyPostShipConsistency verifies that all items in archivedIDs are absent
// from the workspace queue directory. It returns an error listing any stale
// queue paths found, indicating a partial or failed archive operation.
func VerifyPostShipConsistency(_ context.Context, _ *Workspace, _ []string) error {
	return fmt.Errorf("not implemented: VerifyPostShipConsistency — 025.018-T")
}
