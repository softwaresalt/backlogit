package core

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
)

// shipmentReconcileAppendResult carries the byte-range the platform-specific
// append primitive actually wrote, so the portable orchestration can re-read
// EXACTLY that slice for the post-write durability validation, never the
// whole file (which could include unrelated prior/subsequent lines).
type shipmentReconcileAppendResult struct {
	preAppendSize int64
	bytesWritten  int
}

// appendShipmentReconcileEventImpl is the real implementation behind the
// gated appendShipmentReconcileEvent declaration (167.003-T panic body,
// 167.007-T behavior). It runs UNDER the caller's already-held item-log
// lock (167.011-T's lockShipmentReconcileItemLog, acquired by the
// transaction, 167.008-T) and MUST NOT itself acquire any item-log lock.
//
// Sequence: (1) validate eventBytes' own shape via the shared full-event
// validator BEFORE touching disk at all — a malformed input is refused with
// NO write, matching every other reconcile primitive's fail-closed/no-write
// contract; (2) open the logs directory handle no-follow/reparse-safe and
// check the existing file's trailing bytes for a partial (non-newline-
// terminated) line — a prior crash mid-append leaves exactly this signature,
// and blindly concatenating onto it would corrupt the JSONL stream, so it is
// refused as indeterminate instead; (3) append eventBytes via a
// handle-relative, ALWAYS-fsync write (decoupled from the workspace's
// durable_writes config: this append represents the governed transaction's
// own audit record and cannot be optional); (4) re-read EXACTLY the bytes
// just written and re-validate them via the SAME shared validator — proving
// the durable, on-disk bytes are what they should be, not merely what was
// requested; (5) ONLY THEN index the event (bldb.IndexEvent). An index
// failure at step 5 does NOT un-reconcile: the JSONL is already durably
// valid and remains the source of truth; the index is repaired by the next
// sync/reindex. No second append is ever attempted after a failure at any
// step — the caller (167.008-T) owns retry/resume semantics.
func appendShipmentReconcileEventImpl(ctx context.Context, ws *Workspace, itemID string, eventBytes []byte) error {
	if ws == nil {
		return fmt.Errorf("append shipment reconcile event: workspace is required: %w", blerrors.ErrValidation)
	}
	if itemID == "" {
		return fmt.Errorf("append shipment reconcile event: item id is required: %w", blerrors.ErrValidation)
	}
	if len(eventBytes) == 0 {
		return fmt.Errorf("append shipment reconcile event: event bytes are required: %w", blerrors.ErrValidation)
	}
	if err := ValidateShipmentReconciledShippedEvent(eventBytes, nil, ""); err != nil {
		return fmt.Errorf("append shipment reconcile event: refusing malformed event, no write attempted: %w", err)
	}

	logsDir := WorkspaceLogsRoot(ws.RootPath)
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return fmt.Errorf("%w: create logs directory: %w", blerrors.ErrWriteNotApplied, err)
	}
	logPath := events.LogPathForItem(logsDir, itemID)

	result, err := appendShipmentReconcileEventHandleRelative(logsDir, itemID+".jsonl", eventBytes)
	if err != nil {
		return err
	}

	f, err := os.Open(logPath)
	if err != nil {
		return fmt.Errorf("%w: re-open durably appended log for validation: %w", blerrors.ErrWriteIndeterminate, err)
	}
	defer f.Close()
	reReadBuf := make([]byte, result.bytesWritten)
	if _, readErr := f.ReadAt(reReadBuf, result.preAppendSize); readErr != nil {
		return fmt.Errorf("%w: re-read durably appended event: %w", blerrors.ErrWriteIndeterminate, readErr)
	}
	if !bytes.Equal(reReadBuf, eventBytes) {
		return fmt.Errorf("%w: durably appended bytes do not match the requested event", blerrors.ErrWriteIndeterminate)
	}
	if err := ValidateShipmentReconciledShippedEvent(reReadBuf, nil, ""); err != nil {
		return fmt.Errorf("%w: re-read validation of durably appended event failed: %w", blerrors.ErrWriteIndeterminate, err)
	}

	var event events.Event
	if err := json.Unmarshal(bytes.TrimSuffix(reReadBuf, []byte("\n")), &event); err != nil {
		// Unreachable in practice (validation above just succeeded on the
		// same bytes), but fail closed rather than index a value we cannot
		// reconstruct.
		return fmt.Errorf("%w: parse durably appended event: %w", blerrors.ErrWriteIndeterminate, err)
	}

	if idxErr := bldb.IndexEvent(ctx, ws.DB, logsDir, event); idxErr != nil {
		// VALIDATE-BEFORE-INDEX has already proven the durable bytes valid;
		// an index failure now must not un-reconcile. The JSONL remains the
		// source of truth (doctor reads it directly) and the index is
		// repaired by the next sync/reindex.
		slog.WarnContext(ctx, "append shipment reconcile event: index update failed; JSONL remains source of truth and outcome stays reconciled",
			"item_id", itemID, "error", idxErr)
	}
	return nil
}
