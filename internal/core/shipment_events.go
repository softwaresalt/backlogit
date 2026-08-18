package core

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
)

// shipment_events.go (143.002-T / Unit 2) carries the shipment-scoped,
// error-returning event appender used by the governed ShipShipment path. It
// mirrors the internal/core/gate_evidence.go precedent rather than growing
// shipment.go, and it deliberately keeps the lock acquisition and the writer
// call in two distinguishable statements so their outcomes classify
// differently.

// wrapShipmentAppendError wraps a writer error from the shipment event appender
// and adds NO durability class of its own.
//
// Any blerrors.ErrWriteNotApplied or blerrors.ErrWriteIndeterminate the durable
// writer path already attached must survive unwrapping, and an untagged error
// from the default non-durable path must stay untagged so the governed
// classifier can treat it as unproven. Do not attempt to detect a short or
// partial write here: events.EventWriter.AppendEvent exposes only error and its
// non-durable append discards the byte count, so the distinction is not
// observable from this package.
func wrapShipmentAppendError(itemID, eventType string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("append shipment event %s/%s: %w", itemID, eventType, err)
}

// appendShipmentEventErr appends a shipment event and RETURNS the append error,
// unlike the best-effort appendItemEvent. It acquires the item-log lock itself
// so a lock failure and an append failure arise in two distinguishable
// statements: nothing has been written when the lock cannot be taken, so that
// outcome is tagged blerrors.ErrWriteNotApplied and is safe to compensate.
//
// The context returned by events.LockItemLogCrossProcess — not the caller's
// original ctx — is handed to AppendEvent and to the index call. events.LockItemLog
// is a non-reentrant, uncancellable mutex.Lock that short-circuits only on the
// ownership markers in ctx, and AppendEvent re-locks when those markers are
// absent; dropping the locked context would permanently deadlock the ship
// goroutine while it holds the membership lock and every artifact lock.
//
// Indexing into the read-model DB is best-effort (the durable JSONL is the
// source of truth) and never reclassifies the append outcome.
func appendShipmentEventErr(ctx context.Context, ws *Workspace, itemID, eventType string, delta map[string]any) error {
	logsDir := WorkspaceLogsRoot(ws.RootPath)
	lockedCtx, unlockLog, lockErr := events.LockItemLogCrossProcess(ctx, logsDir, itemID)
	if lockErr != nil {
		return fmt.Errorf("lock shipment event log %s: %w: %w", itemID, blerrors.ErrWriteNotApplied, lockErr)
	}
	defer unlockLog()

	event := events.Event{
		Timestamp: time.Now(),
		Actor:     "backlogit",
		ItemID:    itemID,
		EventType: eventType,
		Delta:     eventDeltaWithShipmentOperation(lockedCtx, delta),
	}
	writer := NewWorkspaceEventWriter(ws, logsDir)
	if err := writer.AppendEvent(lockedCtx, event); err != nil {
		return wrapShipmentAppendError(itemID, eventType, err)
	}
	if ws.DB != nil {
		if err := bldb.IndexEvent(lockedCtx, ws.DB, logsDir, event); err != nil {
			slog.WarnContext(lockedCtx, "index shipment event", "item_id", itemID, "event_type", eventType, "error", err)
		}
	}
	return nil
}

// appendShipmentEvent dispatches a shipment event append through the workspace
// seam (overridable in tests) or the real error-returning appender.
// Centralizing the dispatch lets the governed shipped-event durability tests
// inject a classified failure at exactly one point.
func (ws *Workspace) appendShipmentEvent(ctx context.Context, itemID, eventType string, delta map[string]any) error {
	if ws.shipmentEventAppend != nil {
		return ws.shipmentEventAppend(ctx, ws, itemID, eventType, delta)
	}
	return appendShipmentEventErr(ctx, ws, itemID, eventType, delta)
}
