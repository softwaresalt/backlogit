package core

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	bldb "github.com/softwaresalt/backlogit/internal/db"
	"github.com/softwaresalt/backlogit/internal/events"
	"github.com/softwaresalt/backlogit/internal/gateevidence"
)

// Gate evidence event types (082-F, ST4.1). Logs-only: appended to per-item JSONL
// event logs, never to frontmatter (Q3 decision), so completion writes do not
// churn frontmatter or create merge conflicts (constitution IX).
//
// As of Q3.0 (083.005.001-ST) the canonical definitions live in the shared
// internal/gateevidence leaf so both core and db reference one source across the
// one-way core->db boundary; these core identifiers alias the leaf to avoid
// churning the many in-package references.
const (
	EventGatePassed       = gateevidence.EventGatePassed
	EventGateBlocked      = gateevidence.EventGateBlocked
	EventGateForced       = gateevidence.EventGateForced
	EventGateRequeued     = gateevidence.EventGateRequeued
	EventGateEscalated    = gateevidence.EventGateEscalated
	EventGateBaseOverride = gateevidence.EventGateBaseOverride
	EventGateError        = gateevidence.EventGateError
)

// appendItemEventErr appends an item event and RETURNS the append error, unlike
// the best-effort appendItemEvent. It is the appender used on the gated
// completion path: under evidence_required a failed append rolls back the
// transition so a completion never persists without its audit record. Indexing
// into the read-model DB is best-effort (the durable JSONL is the source of
// truth) and only warns on failure.
func appendItemEventErr(ctx context.Context, ws *Workspace, itemID, eventType string, delta map[string]any) error {
	return appendItemEventWithActorErr(ctx, ws, itemID, "backlogit", eventType, delta)
}

// appendItemEventWithActorErr appends an item event stamped with the supplied
// actor so actor-attributed events (e.g. estimate_history) record who authored
// the change in item_log_entries.actor rather than the default "backlogit". A
// blank actor falls back to "backlogit" so the column is never empty.
func appendItemEventWithActorErr(ctx context.Context, ws *Workspace, itemID, actor, eventType string, delta map[string]any) error {
	if actor == "" {
		actor = "backlogit"
	}
	logsDir := WorkspaceLogsRoot(ws.RootPath)
	lockedCtx, unlockLog, lockErr := events.LockItemLogCrossProcess(ctx, logsDir, itemID)
	if lockErr != nil {
		return fmt.Errorf("lock gate evidence log %s: %w", itemID, lockErr)
	}
	defer unlockLog()
	ctx = lockedCtx
	event := events.Event{
		Timestamp: time.Now(),
		Actor:     actor,
		ItemID:    itemID,
		EventType: eventType,
		Delta:     eventDeltaWithShipmentOperation(ctx, delta),
	}
	writer := NewWorkspaceEventWriter(ws, logsDir)
	if err := writer.AppendEvent(ctx, event); err != nil {
		return fmt.Errorf("append gate evidence %s/%s: %w", itemID, eventType, err)
	}
	if ws.DB != nil {
		if err := bldb.IndexEvent(ctx, ws.DB, logsDir, event); err != nil {
			slog.WarnContext(ctx, "index gate evidence", "item_id", itemID, "event_type", eventType, "error", err)
		}
	}
	return nil
}

// appendGateEvent dispatches a gate evidence append through the workspace seam
// (overridable in tests) or the real error-returning appender. Centralizing the
// dispatch lets evidence_required rollback tests inject a failure at one point.
func (ws *Workspace) appendGateEvent(ctx context.Context, itemID, eventType string, delta map[string]any) error {
	if ws.gateEvidenceAppend != nil {
		return ws.gateEvidenceAppend(ctx, ws, itemID, eventType, delta)
	}
	return appendItemEventErr(ctx, ws, itemID, eventType, delta)
}

// gateReportHash returns a stable short hash of the gate report for evidence
// cross-referencing (an empty report hashes to "").
func gateReportHash(report []byte) string {
	if len(report) == 0 {
		return ""
	}
	sum := sha256.Sum256(report)
	return hex.EncodeToString(sum[:])
}
