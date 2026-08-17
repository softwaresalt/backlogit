---
description: "Traceability record for shipment 127-S and the shipped-event durability reconciliation work"
doc_type: reference
ms.date: 2026-08-17
ms.topic: reference
schema_version: "1.0"
source: docs/traces/127-s-trace.md
title: Shipment 127-S trace
---

## Scope

Shipment 127-S covers the shipped-event durability and doctor reconciliation
work for feature 143-F.

## Trace

* Stash: `0115F71F`
* Deliberation: `059-DL`
* Plan: `docs/exec-plans/2026-08-16-shipment-shipped-event-audit-log-plan.md`
* Decision: `docs/decisions/2026-08-16-shipment-shipped-event-audit-log-deliberation.md`
* Core surfaces: `internal/core/shipment.go`, `internal/core/shipment_lifecycle.go`,
  `internal/core/doctor.go`, `internal/events/audit.go`

## Evidence

Distinguish what already exists in current source from what this work plans and
what is deferred:

* Existing governed ShipShipment behavior (characterized baseline): current source
  routes the active-to-shipped `shipment_status_changed` emission through an
  error-returning per-ws append seam (`ws.shipmentEventAppend`), propagating
  `shipmentEventAppendError` from `moveShipmentStatusWithHeadGuard`.
* Planned work (143-F; tasks 143.001-T through 143.007-T): the failure taxonomy and
  rollback around that seam (class-aware NotApplied vs Indeterminate handling,
  synchronous under-lock covering-feature rollup restoration, a `MutationPartialError`
  return, event-ordering/compensation coverage) delivered test-first (143.001-T RED
  harness, then 143.002-T implementation, then 143.003-T integration/regression),
  plus the report-only doctor audit and its CLI and MCP surfaces with parity.
* Deferred: generic non-ShipShipment transition/archive prevention (closing bypass
  paths such as `UpdateArtifactWithGate` / `ArchiveItem`) is out of scope and tracked
  as stash `47B48DB0`. The report-only doctor audit is the standing detection net for
  historical or bypass-produced residue and never rewrites historical JSONL.
