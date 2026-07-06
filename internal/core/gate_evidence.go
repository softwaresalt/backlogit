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
)

// Gate evidence event types (082-F, ST4.1). Logs-only: appended to per-item JSONL
// event logs, never to frontmatter (Q3 decision), so completion writes do not
// churn frontmatter or create merge conflicts (constitution IX).
const (
	EventGatePassed       = "pre_task_completion_gate_passed"
	EventGateBlocked      = "pre_task_completion_gate_blocked"
	EventGateForced       = "pre_task_completion_gate_forced"
	EventGateRequeued     = "pre_task_completion_gate_requeued"
	EventGateEscalated    = "pre_task_completion_gate_escalated"
	EventGateBaseOverride = "pre_task_completion_gate_base_override"
	EventGateError        = "pre_task_completion_gate_error"
)

// appendItemEventErr appends an item event and RETURNS the append error, unlike
// the best-effort appendItemEvent. It is the appender used on the gated
// completion path: under evidence_required a failed append rolls back the
// transition so a completion never persists without its audit record. Indexing
// into the read-model DB is best-effort (the durable JSONL is the source of
// truth) and only warns on failure.
func appendItemEventErr(ctx context.Context, ws *Workspace, itemID, eventType string, delta map[string]any) error {
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
		return fmt.Errorf("append gate evidence %s/%s: %w", itemID, eventType, err)
	}
	if ws.DB != nil {
		if err := bldb.IndexEvent(ctx, ws.DB, logsDir, event); err != nil {
			slog.WarnContext(ctx, "index gate evidence", "item_id", itemID, "event_type", eventType, "error", err)
		}
	}
	return nil
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
