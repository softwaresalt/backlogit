package core

import (
	"context"
	"fmt"
	"log/slog"
	"path/filepath"
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
	// Refuse an item ID that would resolve its log (and therefore its lock
	// sidecar) outside the logs directory. Nothing has been written when this
	// fires, so not-applied is the honest class and compensation is safe.
	//
	// Use real-path containment (EvalSymlinks) to prevent a symlinked component
	// of logsDir from being exploited to bypass the lexical check. This mirrors
	// the EvalSymlinks+pathContained pattern used by confineToStorageRoot.
	logPath := events.LogPathForItem(logsDir, itemID)
	// Use real-path containment anchored on the workspace storage root to
	// defend against a symlinked logsDir. If only the logs directory itself
	// were the anchor, a symlink from logsDir to /external would make both
	// realLogsDir and realLogPath resolve under /external and the inner check
	// would pass. Anchoring on the storage root (which uses filepath.Abs first,
	// following confineToStorageRoot) catches that case.
	absStorageRoot, absErr := filepath.Abs(WorkspaceStorageRoot(ws.RootPath))
	if absErr != nil {
		absStorageRoot = filepath.Clean(WorkspaceStorageRoot(ws.RootPath))
	}
	realStorageRoot, srErr := filepath.EvalSymlinks(absStorageRoot)
	if srErr != nil {
		realStorageRoot = absStorageRoot
	}
	realLogPath, evalErr := filepath.EvalSymlinks(logPath)
	if evalErr != nil {
		// The log file may not exist yet; resolve at least the parent directory
		// so a symlinked intermediate component is still caught.
		if realParent, perr := filepath.EvalSymlinks(filepath.Dir(logPath)); perr == nil {
			realLogPath = filepath.Join(realParent, filepath.Base(logPath))
		} else {
			realLogPath = logPath
		}
	}
	if !pathContained(realStorageRoot, realLogPath) {
		return fmt.Errorf("shipment event log for %s resolves outside the workspace storage root: %w: %w",
			itemID, blerrors.ErrWriteNotApplied, blerrors.ErrValidation)
	}
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
