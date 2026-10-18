package core

import (
	"context"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// ShipmentPredecessorShipped reports whether the shipment predecessor id has
// shipped. The returned string is a reason code when the predecessor is not
// ready. This declaration is a stub (187.002-T); the behavior lands in 187.004-T.
func ShipmentPredecessorShipped(_ context.Context, _ *Workspace, _ string) (bool, string, error) {
	return false, "not_implemented", blerrors.ErrNotImplemented
}
