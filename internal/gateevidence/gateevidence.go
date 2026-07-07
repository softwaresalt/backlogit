// Package gateevidence is a leaf package (importing only internal/events) that
// owns the canonical gate-evidence event-type constants and the single shared
// predicate used to derive an item's gate-evidence status from its event stream.
//
// It exists so both internal/core (the completion/shipment gate) and internal/db
// (the derived gate_evidence projection) can compute gate evidence from the same
// finalized F4 composed predicate across the one-way core->db import boundary,
// with no import cycle (core->gateevidence->events, db->gateevidence->events).
package gateevidence

import "github.com/softwaresalt/backlogit/internal/events"

// Gate evidence event types (082-F, ST4.1). Logs-only: appended to per-item JSONL
// event logs, never to frontmatter (Q3 decision). This leaf package is the
// canonical home; internal/core aliases these to avoid a duplicate source.
const (
	EventGatePassed       = "pre_task_completion_gate_passed"
	EventGateBlocked      = "pre_task_completion_gate_blocked"
	EventGateForced       = "pre_task_completion_gate_forced"
	EventGateRequeued     = "pre_task_completion_gate_requeued"
	EventGateEscalated    = "pre_task_completion_gate_escalated"
	EventGateBaseOverride = "pre_task_completion_gate_base_override"
	EventGateError        = "pre_task_completion_gate_error"
)

// Derived gate-evidence status tokens for the read-model projection (Q3.2).
const (
	// StatusMissing means the item carries no valid passing/forced gate evidence.
	StatusMissing = "missing"
	// StatusPassed means the latest valid evidence is a real gate pass (ran==true).
	StatusPassed = "passed"
	// StatusForced means the latest valid evidence is an audited break-glass force
	// whose delta records ran==true.
	StatusForced = "forced"
	// StatusForcedNoRun means the latest valid evidence is an audited break-glass
	// force whose delta records ran==false — the member shipped on a force rather
	// than a real gate run. Surfaced distinctly for audit visibility (SecLens P2).
	StatusForcedNoRun = "forced_no_run"
)

// Evidence is the derived gate-evidence read-model for a single item, computed
// from its event stream via the finalized F4 composed predicate.
type Evidence struct {
	// Status is one of StatusMissing/StatusPassed/StatusForced/StatusForcedNoRun.
	Status string
	// EvidenceSHA is the gate_report_hash recorded on the selected event (may be empty).
	EvidenceSHA string
	// HeadSHA is the head_sha recorded on the selected event (may be empty).
	HeadSHA string
	// Event points at the selected evidence event, or nil when Status==StatusMissing.
	Event *events.Event
}

// Latest returns the derived Evidence for an item's event stream, encoding the
// finalized F4 composed member-evidence predicate (082-F F4 hardening, 083.002-T):
// the most recent event that is EventGateForced (unconditional audited
// break-glass, any ran) OR EventGatePassed whose delta records ran==true is the
// valid evidence. A fail-open EventGatePassed no-run (ran missing/false) is NOT
// valid and is skipped, so an earlier qualifying event is promoted.
//
// The returned Event is exactly the event core's member-evidence scan selects,
// so callers that need the raw event (e.g. the head_sha staleness check) get
// identical behavior. Status additionally distinguishes a forced event with
// ran==false (StatusForcedNoRun) from a forced real run (StatusForced) for the
// audit-visible projection; this does not change which event is selected.
func Latest(evs []events.Event) Evidence {
	var selected *events.Event
	forcedNoRun := false
	for i := range evs {
		switch evs[i].EventType {
		case EventGateForced:
			// Unconditional break-glass: valid regardless of ran. Record whether it
			// was a no-run force for the audit-visible status token.
			e := evs[i]
			selected = &e
			ran, _ := evs[i].Delta["ran"].(bool)
			forcedNoRun = !ran
		case EventGatePassed:
			// Comma-ok read: a missing or non-bool "ran" yields false, correctly
			// treated as not-ran and skipped (fail-open rejection).
			if ran, _ := evs[i].Delta["ran"].(bool); ran {
				e := evs[i]
				selected = &e
				forcedNoRun = false
			}
		}
	}
	if selected == nil {
		return Evidence{Status: StatusMissing}
	}
	status := StatusPassed
	if selected.EventType == EventGateForced {
		if forcedNoRun {
			status = StatusForcedNoRun
		} else {
			status = StatusForced
		}
	}
	sha, _ := selected.Delta["gate_report_hash"].(string)
	head, _ := selected.Delta["head_sha"].(string)
	return Evidence{Status: status, EvidenceSHA: sha, HeadSHA: head, Event: selected}
}
