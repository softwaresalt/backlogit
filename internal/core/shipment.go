package core

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	bldb "github.com/backlogit/backlogit/internal/db"
	blerrors "github.com/backlogit/backlogit/internal/errors"
	"github.com/backlogit/backlogit/internal/events"
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
	items := append([]string(nil), itemIDs...)
	if items == nil {
		items = []string{}
	}

	shipment, err := CreateArtifact(ctx, ws, title, "shipment", WithFields(map[string]any{"items": items}))
	if err != nil {
		return nil, fmt.Errorf("create shipment %q: %w", title, err)
	}
	normalizeShipmentArtifact(shipment)
	if err := bldb.UpsertItem(ctx, ws.DB, shipment); err != nil {
		return nil, fmt.Errorf("create shipment %s: %w", shipment.ID, err)
	}

	appendItemEvent(ctx, ws, shipment.ID, "shipment_created", map[string]any{
		"title": title,
		"items": items,
	})

	return shipment, nil
}

// GetShipment retrieves a shipment artifact by ID from the workspace.
//
// Worker: Look up the shipment by ID in the database index, parse the Markdown
// source file, and return the populated Artifact with items list in CustomFields.
func GetShipment(ctx context.Context, ws *Workspace, shipmentID string) (*models.Artifact, error) {
	artifact, err := loadArtifact(ctx, ws, shipmentID)
	if err != nil {
		if errors.Is(err, blerrors.ErrNotFound) {
			return nil, fmt.Errorf("get shipment %s: %w", shipmentID, blerrors.ErrShipmentNotFound)
		}
		return nil, fmt.Errorf("get shipment %s: %w", shipmentID, err)
	}
	if artifact.ArtifactType != "shipment" {
		return nil, fmt.Errorf("get shipment %s: %w", shipmentID, blerrors.ErrShipmentNotFound)
	}

	normalizeShipmentArtifact(artifact)
	return artifact, nil
}

// MoveShipmentStatus transitions a shipment's status. Valid transitions:
// queued->active, active->shipped, active->abandoned.
//
// Worker: Validate the transition is legal, update the Markdown frontmatter,
// rewrite the file atomically, and upsert the new status into the database.
// Emit slog.Info for status transition and events.jsonl record.
func MoveShipmentStatus(ctx context.Context, ws *Workspace, shipmentID string, newStatus ShipmentStatus) error {
	shipment, err := GetShipment(ctx, ws, shipmentID)
	if err != nil {
		return err
	}

	if !isValidShipmentTransition(shipment.Status, newStatus) {
		return fmt.Errorf(
			"move shipment %s from %s to %s: %w",
			shipmentID,
			shipment.Status,
			newStatus,
			blerrors.ErrShipmentConflict,
		)
	}

	shipment.Status = models.ArtifactStatus(newStatus)
	shipment.UpdatedAt = time.Now()
	if err := persistArtifact(ctx, ws, shipment, true); err != nil {
		return fmt.Errorf("move shipment %s: %w", shipmentID, err)
	}

	slog.InfoContext(ctx, "shipment status changed", "shipment_id", shipmentID, "new_status", newStatus)
	appendItemEvent(ctx, ws, shipmentID, "shipment_status_changed", map[string]any{
		"status": string(newStatus),
	})

	return nil
}

// AddItemToShipment associates an artifact ID with a shipment. The item must not
// already belong to another active shipment.
//
// Worker: Check the item is not already assigned to another shipment (return
// ErrItemAlreadyAssigned if so), append the item ID to the shipment's items list
// in frontmatter, rewrite the file atomically, and upsert into the database.
// Emit slog.Debug for item association and events.jsonl record.
func AddItemToShipment(ctx context.Context, ws *Workspace, shipmentID, itemID string) error {
	shipment, err := GetShipment(ctx, ws, shipmentID)
	if err != nil {
		return err
	}

	shipments, err := bldb.QueryItems(ctx, ws.DB, bldb.QueryFilters{Type: "shipment", IncludeArchived: true})
	if err != nil {
		return fmt.Errorf("list shipments: %w", err)
	}
	for _, existing := range shipments {
		if existing.ID == shipmentID || existing.Status == models.StatusAbandoned {
			continue
		}
		if containsString(shipmentItems(existing), itemID) {
			return fmt.Errorf("add item %s to shipment %s: %w", itemID, shipmentID, blerrors.ErrItemAlreadyAssigned)
		}
	}

	items := shipmentItems(shipment)
	if containsString(items, itemID) {
		return nil
	}

	items = append(items, itemID)
	if shipment.CustomFields == nil {
		shipment.CustomFields = map[string]any{}
	}
	shipment.CustomFields["items"] = items
	shipment.UpdatedAt = time.Now()
	if err := persistArtifact(ctx, ws, shipment, false); err != nil {
		return fmt.Errorf("add item %s to shipment %s: %w", itemID, shipmentID, err)
	}

	slog.DebugContext(ctx, "shipment item added", "shipment_id", shipmentID, "item_id", itemID)
	appendItemEvent(ctx, ws, shipmentID, "shipment_item_added", map[string]any{
		"item_id": itemID,
	})

	return nil
}

// ReturnBlockedItem removes an item from shipment and sets its status to blocked
// with a reason. This is an explicit operation, not a side effect.
//
// Worker: Remove the item ID from the shipment's items list, set the item's status
// to blocked, append a blocked_reason field to the item's frontmatter, rewrite
// both files atomically, and emit slog.Info + events.jsonl for the return.
// Return ErrCannotReturnItem if the item is not in this shipment.
func ReturnBlockedItem(ctx context.Context, ws *Workspace, shipmentID, itemID, reason string) error {
	shipment, err := GetShipment(ctx, ws, shipmentID)
	if err != nil {
		return err
	}

	items := shipmentItems(shipment)
	if !containsString(items, itemID) {
		return fmt.Errorf("return item %s from shipment %s: %w", itemID, shipmentID, blerrors.ErrCannotReturnItem)
	}

	item, err := loadArtifact(ctx, ws, itemID)
	if err != nil {
		return fmt.Errorf("load item %s: %w", itemID, err)
	}

	if shipment.CustomFields == nil {
		shipment.CustomFields = map[string]any{}
	}
	shipment.CustomFields["items"] = removeString(items, itemID)
	shipment.UpdatedAt = time.Now()

	if item.CustomFields == nil {
		item.CustomFields = map[string]any{}
	}
	item.Status = models.StatusBlocked
	item.CustomFields["blocked_reason"] = reason
	item.UpdatedAt = time.Now()

	if err := persistArtifact(ctx, ws, shipment, false); err != nil {
		return fmt.Errorf("update shipment %s: %w", shipmentID, err)
	}
	if err := persistArtifact(ctx, ws, item, true); err != nil {
		return fmt.Errorf("update item %s: %w", itemID, err)
	}

	slog.InfoContext(ctx, "shipment item returned blocked", "shipment_id", shipmentID, "item_id", itemID)
	appendItemEvent(ctx, ws, shipmentID, "shipment_item_returned_blocked", map[string]any{
		"item_id": itemID,
		"reason":  reason,
	})
	appendItemEvent(ctx, ws, itemID, "item_blocked", map[string]any{
		"shipment_id":    shipmentID,
		"blocked_reason": reason,
	})

	return nil
}

func appendItemEvent(ctx context.Context, ws *Workspace, itemID, eventType string, delta map[string]any) {
	logsDir := WorkspaceLogsRoot(ws.RootPath)
	event := events.Event{
		Timestamp: time.Now(),
		Actor:     "backlogit",
		ItemID:    itemID,
		EventType: eventType,
		Delta:     delta,
	}

	writer := events.NewEventWriter(logsDir)
	if err := writer.AppendEvent(ctx, event); err != nil {
		slog.WarnContext(ctx, "append shipment event", "item_id", itemID, "event_type", eventType, "error", err)
		return
	}
	if err := bldb.IndexEvent(ctx, ws.DB, logsDir, event); err != nil {
		slog.WarnContext(ctx, "index shipment event", "item_id", itemID, "event_type", eventType, "error", err)
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func isValidShipmentTransition(current models.ArtifactStatus, next ShipmentStatus) bool {
	switch current {
	case models.StatusQueued:
		return next == ShipmentActive
	case models.StatusActive:
		return next == ShipmentShipped || next == ShipmentAbandoned
	default:
		return false
	}
}

func loadArtifact(ctx context.Context, ws *Workspace, id string) (*models.Artifact, error) {
	artifact, err := bldb.GetItem(ctx, ws.DB, id)
	if err == nil {
		return artifact, nil
	}
	if !errors.Is(err, blerrors.ErrNotFound) {
		return nil, err
	}

	artifact, findErr := findArtifact(ctx, ws, id)
	if findErr != nil {
		return nil, fmt.Errorf("load artifact %s: %w", id, blerrors.ErrNotFound)
	}
	if upsertErr := bldb.UpsertItem(ctx, ws.DB, artifact); upsertErr != nil {
		return nil, fmt.Errorf("upsert artifact %s: %w", id, upsertErr)
	}
	return artifact, nil
}

func normalizeShipmentArtifact(artifact *models.Artifact) {
	if artifact.CustomFields == nil {
		artifact.CustomFields = map[string]any{}
	}
	artifact.CustomFields["items"] = shipmentItems(artifact)
}

func persistArtifact(ctx context.Context, ws *Workspace, artifact *models.Artifact, relocate bool) error {
	if err := artifact.Validate(); err != nil {
		return fmt.Errorf("validate artifact: %w", err)
	}

	filePath, err := FindArtifactPath(ctx, ws, artifact.ID)
	if err != nil {
		return fmt.Errorf("find artifact path: %w", err)
	}
	if relocate {
		filePath, err = RelocateArtifactFile(ctx, ws, artifact.ArtifactType, artifact.ID, string(artifact.Status))
		if err != nil {
			return fmt.Errorf("relocate artifact: %w", err)
		}
	}
	if err := WriteArtifactFile(artifact, filePath); err != nil {
		return fmt.Errorf("write artifact file: %w", err)
	}
	if err := bldb.UpsertItem(ctx, ws.DB, artifact); err != nil {
		return fmt.Errorf("upsert item: %w", err)
	}
	return nil
}

func removeString(values []string, target string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if value != target {
			result = append(result, value)
		}
	}
	return result
}

func shipmentItems(artifact *models.Artifact) []string {
	if artifact == nil || artifact.CustomFields == nil {
		return []string{}
	}

	raw, ok := artifact.CustomFields["items"]
	if !ok || raw == nil {
		return []string{}
	}

	switch items := raw.(type) {
	case []string:
		return append([]string(nil), items...)
	case []any:
		normalized := make([]string, 0, len(items))
		for _, item := range items {
			if value, ok := item.(string); ok {
				normalized = append(normalized, value)
			}
		}
		return normalized
	default:
		return []string{}
	}
}
