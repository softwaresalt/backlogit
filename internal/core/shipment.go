package core

import (
	"context"

	"github.com/backlogit/backlogit/internal/models"
)

// ShipmentStatus represents the lifecycle state of a shipment.
type ShipmentStatus string

const (
	// ShipmentQueued indicates the shipment is created but not yet started.
	ShipmentQueued ShipmentStatus = "queued"
	// ShipmentActive indicates the shipment is in progress.
	ShipmentActive ShipmentStatus = "active"
	// ShipmentShipped indicates the shipment has been delivered.
	ShipmentShipped ShipmentStatus = "shipped"
	// ShipmentAbandoned indicates the shipment was cancelled.
	ShipmentAbandoned ShipmentStatus = "abandoned"
)

// CreateShipment creates a new shipment artifact in the workspace with the given
// title and associates the specified item IDs with it. The shipment owns the items
// list as an aggregate root.
//
// Worker: Create a new shipment Markdown artifact with YAML frontmatter containing
// the items list, set status to queued, generate ID with S prefix, write to queue
// directory, and upsert into the database index.
func CreateShipment(ctx context.Context, ws *Workspace, title string, itemIDs []string) (*models.Artifact, error) {
	panic("not implemented: Worker: Create shipment artifact with S prefix, YAML frontmatter items list, and queue file write")
}

// GetShipment retrieves a shipment artifact by ID from the workspace.
//
// Worker: Look up the shipment by ID in the database index, parse the Markdown
// source file, and return the populated Artifact with items list in CustomFields.
func GetShipment(ctx context.Context, ws *Workspace, shipmentID string) (*models.Artifact, error) {
	panic("not implemented: Worker: Retrieve shipment by ID from index and parse Markdown source")
}

// MoveShipmentStatus transitions a shipment's status. Valid transitions:
// queued->active, active->shipped, active->abandoned.
//
// Worker: Validate the transition is legal, update the Markdown frontmatter,
// rewrite the file atomically, and upsert the new status into the database.
// Emit slog.Info for status transition and events.jsonl record.
func MoveShipmentStatus(ctx context.Context, ws *Workspace, shipmentID string, newStatus ShipmentStatus) error {
	panic("not implemented: Worker: Validate status transition, update frontmatter, atomic file write, emit slog and event")
}

// AddItemToShipment associates an artifact ID with a shipment. The item must not
// already belong to another active shipment.
//
// Worker: Check the item is not already assigned to another shipment (return
// ErrItemAlreadyAssigned if so), append the item ID to the shipment's items list
// in frontmatter, rewrite the file atomically, and upsert into the database.
// Emit slog.Debug for item association and events.jsonl record.
func AddItemToShipment(ctx context.Context, ws *Workspace, shipmentID, itemID string) error {
	panic("not implemented: Worker: Validate item not in another shipment, add to items list, atomic write, emit events")
}

// ReturnBlockedItem removes an item from shipment and sets its status to blocked
// with a reason. This is an explicit operation, not a side effect.
//
// Worker: Remove the item ID from the shipment's items list, set the item's status
// to blocked, append a blocked_reason field to the item's frontmatter, rewrite
// both files atomically, and emit slog.Info + events.jsonl for the return.
// Return ErrCannotReturnItem if the item is not in this shipment.
func ReturnBlockedItem(ctx context.Context, ws *Workspace, shipmentID, itemID, reason string) error {
	panic("not implemented: Worker: Remove item from shipment items, set item status blocked with reason, atomic writes, emit events")
}
