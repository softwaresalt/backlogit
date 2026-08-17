---
description: "Traceability record for feature 143-F and its shared CLI/MCP durability surfaces"
doc_type: reference
ms.date: 2026-08-17
ms.topic: reference
schema_version: "1.0"
source: docs/traces/143-s-trace.md
title: Feature 143-F trace
---

## Scope

Feature 143-F builds on the existing error-returning shipped-event append seam to
deliver the shipped-event failure taxonomy and class-aware rollback, plus the
shipped-event doctor reconciliation surfaces across the shared CLI and MCP core.

## Trace

* Task set: `143.001-T` through `143.007-T`
* Shared core: `internal/core/shipment.go`, `internal/core/shipment_lifecycle.go`
* Audit surface: `internal/core/doctor.go`, `internal/cli/doctor.go`,
  `internal/mcp/tools.go`, `internal/events/audit.go`

## Evidence

* Characterized baseline (already in current source): the active-to-shipped
  `shipment_status_changed` emission routes through the error-returning per-ws append
  seam (`ws.shipmentEventAppend`), propagating `shipmentEventAppendError`.
* Planned taxonomy/rollback (143.001-T RED harness, 143.002-T implementation,
  143.003-T integration/regression): a captured shipped-append `ErrWriteNotApplied`
  (and a clean pre-append lock-acquisition tagged NotApplied) compensates the
  active-to-shipped transition; a post-append `ErrWriteIndeterminate` (and untagged,
  by safe default) never rolls back and returns a structured `MutationPartialError`,
  with synchronous under-lock covering-feature rollup restoration.
* Planned doctor surfaces (143.004-T through 143.006-T): doctor reports both missing
  shipped events and shipped-but-unarchived residue without rewriting historical
  JSONL, via a shared shipped-event helper, exposed on the CLI and MCP with parity.
* Planned MCP recovery guidance (143.007-T): the indeterminate ship error surfaces
  structurally in MCP and directs the caller to
  `doctor --check-shipped-event-completeness`.
* Deferred: generic non-ShipShipment transition/archive prevention is out of scope
  and tracked as stash `47B48DB0`.
