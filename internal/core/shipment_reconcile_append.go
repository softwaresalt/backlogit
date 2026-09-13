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
// append primitive actually wrote, plus the durable re-read of exactly that
// range performed from the SAME still-open file handle used for the append
// itself (readBack). The portable orchestration validates readBack directly
// rather than re-opening logPath BY PATHNAME a second time: a fresh pathname
// open is a TOCTOU window in which the path could be swapped between the
// handle-relative append and that re-open, letting validation read a
// different (possibly outside-workspace) file while reporting the append as
// durably verified (167.007-T hardening).
type shipmentReconcileAppendResult struct {
	preAppendSize int64
	bytesWritten  int
	readBack      []byte
}

// shipmentReconcileAppendSeams carries the injectable handle-relative append
// primitive so tests can prove the portable orchestration below validates
// the SAME re-read the primitive itself performed (result.readBack) rather
// than re-deriving bytes via a fresh pathname open of logPath — the exact
// TOCTOU distinction shipmentReconcileAppendResult documents. Production
// always wires the real platform-specific
// appendShipmentReconcileEventHandleRelative implementation.
type shipmentReconcileAppendSeams struct {
	appendHandleRelative func(logsDir, fileName string, eventBytes []byte) (shipmentReconcileAppendResult, error)
}

// defaultShipmentReconcileAppendSeams returns the production seam wiring.
func defaultShipmentReconcileAppendSeams() shipmentReconcileAppendSeams {
	return shipmentReconcileAppendSeams{appendHandleRelative: appendShipmentReconcileEventHandleRelative}
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
// own audit record and cannot be optional); (4) re-validate EXACTLY the
// bytes the platform-specific primitive itself durably re-read from the SAME
// handle it wrote through (never a fresh pathname re-open, which would be a
// TOCTOU window) via the SAME shared validator — proving the durable,
// on-disk bytes are what they should be, not merely what was requested; (5)
// ONLY THEN index the event (bldb.IndexEvent). An index failure at step 5
// does NOT un-reconcile: the JSONL is already durably valid and remains the
// source of truth; the index is repaired by the next sync/reindex. No second
// append is ever attempted after a failure at any step — the caller
// (167.008-T) owns retry/resume semantics.
func appendShipmentReconcileEventImpl(ctx context.Context, ws *Workspace, itemID string, eventBytes []byte) error {
	return appendShipmentReconcileEventImplWithSeams(ctx, ws, itemID, eventBytes, defaultShipmentReconcileAppendSeams())
}

// appendShipmentReconcileEventImplWithSeams is the seam-injectable
// implementation shared by appendShipmentReconcileEventImpl and the
// portable-orchestration tests (shipment_reconcile_append_test.go).
func appendShipmentReconcileEventImplWithSeams(ctx context.Context, ws *Workspace, itemID string, eventBytes []byte, seams shipmentReconcileAppendSeams) error {
	if ws == nil {
		return fmt.Errorf("append shipment reconcile event: workspace is required: %w", blerrors.ErrValidation)
	}
	if itemID == "" {
		return fmt.Errorf("append shipment reconcile event: item id is required: %w", blerrors.ErrValidation)
	}
	if len(eventBytes) == 0 {
		return fmt.Errorf("append shipment reconcile event: event bytes are required: %w", blerrors.ErrValidation)
	}
	if err := ValidateShipmentReconciledShippedEvent(eventBytes, nil, "", itemID); err != nil {
		return fmt.Errorf("append shipment reconcile event: refusing malformed event, no write attempted: %w", err)
	}

	logsDir := WorkspaceLogsRoot(ws.RootPath)
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return fmt.Errorf("%w: create logs directory: %w", blerrors.ErrWriteNotApplied, err)
	}
	logPath := events.LogPathForItem(logsDir, itemID)

	result, err := seams.appendHandleRelative(logsDir, itemID+".jsonl", eventBytes)
	if err != nil {
		return err
	}

	// Validate the re-read the platform-specific append primitive already
	// performed from the SAME handle it wrote through — never a fresh
	// pathname open of logPath (see shipmentReconcileAppendResult).
	reReadBuf := result.readBack
	if len(reReadBuf) != result.bytesWritten {
		return fmt.Errorf("%w: durably appended re-read length mismatch for %s: got %d want %d bytes", blerrors.ErrWriteIndeterminate, logPath, len(reReadBuf), result.bytesWritten)
	}
	if !bytes.Equal(reReadBuf, eventBytes) {
		return fmt.Errorf("%w: durably appended bytes do not match the requested event", blerrors.ErrWriteIndeterminate)
	}
	if err := ValidateShipmentReconciledShippedEvent(reReadBuf, nil, "", itemID); err != nil {
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
