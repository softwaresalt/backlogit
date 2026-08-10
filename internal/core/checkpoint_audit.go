package core

import (
	"context"
	"fmt"

	blerrors "github.com/softwaresalt/backlogit/internal/errors"
	"github.com/softwaresalt/backlogit/internal/events"
)

// checkpointAuditItemID is the fixed item-log identifier under which every
// checkpoint administrative disposition audit event is appended. Using one
// fixed identifier (rather than the checkpoint's own filename) produces a
// single durable audit trail file
// (<logs>/checkpoint-disposition-audit.jsonl) that records every abandon and
// quarantine action across all checkpoints, mirroring how gate evidence
// events are appended per-item but for a cross-cutting administrative log.
const checkpointAuditItemID = "checkpoint-disposition-audit"

// EventCheckpointDisposition is the audit event type appended before any
// checkpoint disposition move or rewrite is performed.
const EventCheckpointDisposition = "checkpoint_disposition"

// appendCheckpointDispositionAudit appends an audit event recording a
// checkpoint administrative disposition action (abandon or quarantine) BEFORE
// any move or rewrite of the checkpoint file is performed. Per the protected
// invariant that a failed audit append leaves the target file untouched, the
// disposition verbs (AbandonCheckpoint, QuarantineCheckpoint) call this
// function first and refuse to proceed on any error.
//
// The write is classified per the two-class durable-write contract
// (internal/errors durability_errors.go): a plain or ErrWriteNotApplied
// failure (nothing appended) maps to ErrCheckpointAuditNotApplied, which is
// safe to retry; an ErrWriteIndeterminate failure (possibly-appended, unknown
// outcome) maps to ErrCheckpointAuditIndeterminate, which callers must never
// blindly retry or compensate.
func appendCheckpointDispositionAudit(ctx context.Context, ew *events.EventWriter, filename, verb, reason, operator string) error {
	if ew == nil {
		return fmt.Errorf("append checkpoint disposition audit: EventWriter must not be nil")
	}
	event := events.Event{
		Actor:     operator,
		ItemID:    checkpointAuditItemID,
		EventType: EventCheckpointDisposition,
		Delta: map[string]any{
			"filename": filename,
			"verb":     verb,
			"reason":   reason,
			"operator": operator,
		},
	}
	if err := ew.AppendEvent(ctx, event); err != nil {
		if blerrors.IsWriteIndeterminate(err) {
			return fmt.Errorf("%w: %v", blerrors.ErrCheckpointAuditIndeterminate, err)
		}
		return fmt.Errorf("%w: %v", blerrors.ErrCheckpointAuditNotApplied, err)
	}
	return nil
}
