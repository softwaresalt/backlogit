---
chunk_strategy: h1-h2-h3
description: Evaluation of four archival strategies for completed backlogit artifacts and the recommended hybrid approach
doc_type: decision
docline:
    date: 2026-04-10T00:00:00Z
    origin: 025-F / 025.012-T
    status: accepted
ingested_at: "2026-06-26T02:33:47Z"
schema_version: "1.0"
source: docs/decisions/archival-policy.md
title: Queue Archival Policy for Completed Work Items
---

## Context

Backlogit's `.backlogit/queue/` directory accumulates completed work items over
time. Without an archival policy, the queue grows unbounded, degrading query
performance, cluttering context for agents, and obscuring active work. This
document evaluates four approaches and recommends one.

The existing `ArchiveItem` operation in `internal/core/archive.go` provides the
low-level mechanism. The question here is when archival is triggered, not how.

## Options Evaluated

### Option 1: Shipment-based archival

Items are archived automatically when their containing shipment ships. The
`ShipShipment` operation already invokes `archiveItems` for released items.

**Advantages:**

* Strongest traceability. Every archived item carries a shipment close event
  in its JSONL log, providing an auditable reason for the move.
* Already partially implemented. The mechanism exists and ships alongside the
  workspace governance work in this feature.
* No manual intervention required for the normal delivery path.

**Disadvantages:**

* Items completed outside a shipment (e.g., direct `done` status updates via
  MCP, ad hoc tasks) are never archived automatically.
* Items that remain `done` for weeks before their shipment closes accumulate
  in the queue in a terminal state with no cleanup path.

### Option 2: Time-based archival

A background sweep archives any item that has been in a done or blocked state
for longer than a configurable threshold (e.g., 30 days).

**Advantages:**

* Handles all items regardless of whether they were delivered via shipment.
* Fully automatic after initial configuration.

**Disadvantages:**

* Requires a daemon or scheduled task, adding operational complexity.
* Premature archival risk: an item done for 29 days may still be referenced by
  an open shipment or an active PR.
* No implementation exists yet. This is a net-new engineering investment.

### Option 3: Manual-only archival

The operator explicitly calls `backlogit archive <id>` or the equivalent MCP
tool for each item. No automation.

**Advantages:**

* Maximum control. The operator decides exactly when state is stable enough to
  archive.
* No risk of unexpected data movement.

**Disadvantages:**

* High operational overhead. Every completed item requires a deliberate
  follow-up action.
* Archive backlog accumulates when sessions end without cleanup. In practice,
  manual-only policies are rarely maintained.

### Option 4: Hybrid (recommended)

Shipment close is the primary archival trigger (Option 1, already implemented).
Items that reach a done state outside a shipment are flagged for review after a
configurable staleness threshold rather than archived automatically.

**Advantages:**

* Normal delivery path (shipment → archive) is fully automated and already
  works.
* Stale done detection provides visibility into items that fell outside the
  shipment flow without forcing premature archival.
* No daemon required for the primary path. Time-based detection can be
  implemented as an on-demand `doctor` check rather than a scheduled sweep.
* Avoids the premature-archival risk of Option 2 by flagging rather than
  acting.

**Disadvantages:**

* Items outside the shipment path still require manual archival after flagging.
* Implementation of stale done detection requires future engineering work.

## Recommendation

Adopt the **hybrid approach** (Option 4).

The shipment-based mechanism is the primary archival path and is already
implemented. For items completed outside a shipment, the preferred model is
detection-first rather than automatic cleanup: a future enhancement to the
`doctor` diagnostic tool will flag items in a done or blocked state beyond a
configurable age threshold (initial suggestion: 30 days), surfacing them for
manual review and archival.

This design means:

* There is no risk of silent data movement for items that are done but still
  referenced by open work.
* The archival audit trail (shipment close events in JSONL logs) is preserved
  for the common case.
* The stale detection threshold is configurable in `config.yaml` when
  implemented, defaulting to 30 days.

## Deferred Work

Time-based stale detection is explicitly deferred to a future feature. This
document describes the target policy state. Tooling to enforce the time-based
detection leg does not yet exist and is tracked separately in the stash.

## Decision Record

| Field | Value |
|---|---|
| Status | Accepted |
| Decided | 2026-04-10 |
| Origin | 025.012-T (Design queue archival policy for completed items) |
| Supersedes | None |
