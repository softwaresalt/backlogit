package core

import (
	"context"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
)

// ReorderShipmentResult reports a ReorderShipment outcome or preview.
type ReorderShipmentResult struct {
	ShipmentID    string   `json:"shipment_id"`
	Mode          string   `json:"mode"`
	Status        string   `json:"status"`
	DryRun        bool     `json:"dry_run"`
	Force         bool     `json:"force,omitempty"`
	Reason        string   `json:"reason,omitempty"`
	CorrelationID string   `json:"correlation_id,omitempty"`
	ItemsBefore   []string `json:"items_before"`
	ItemsAfter    []string `json:"items_after"`
}

// ReorderShipment is a RED-phase stub.
func ReorderShipment(
	_ context.Context,
	_ *Workspace,
	_ string,
	_ ShipmentManifestOptions,
) (*ReorderShipmentResult, error) {
	return nil, blerrors.ErrNotImplemented
}
