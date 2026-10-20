package core

import (
	"context"
	"fmt"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/models"
)

// ShipmentDispositionOptions carries the operator-supplied inputs for a
// governed queued-shipment disposition. Every field except SupersededBy is
// required, and Confirm must be true.
type ShipmentDispositionOptions struct {
	Reason           string
	SupersededBy     string
	By               string
	AuthorizationRef string
	Confirm          bool
}

// disposeAfterGovernedWriteHook is a test seam. It is nil in production. It
// runs after the single governed write and before the journal is marked
// committed.
var disposeAfterGovernedWriteHook func() error

// DisposeQueuedShipment moves a queued shipment to abandoned under a decided
// deliberation authorization. It is a stub until the implementation lands.
func DisposeQueuedShipment(ctx context.Context, ws *Workspace, id string, opts ShipmentDispositionOptions) (*models.Artifact, error) {
	_, _, _, _ = ctx, ws, id, opts
	return nil, fmt.Errorf("dispose queued shipment: %w", blerrors.ErrNotImplemented)
}
