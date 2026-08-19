---
chunk_strategy: h1-h2
description: "Core seam guard for shipment shipped-transition prevention: why unconditional core-seam refusal beats gate rules for governance-critical flows."
doc_type: learning
schema_version: "1.0"
source: docs/compound/2026-08-18-shipment-shipped-prevention-envelope.md
title: "Shipment governance: unconditional core-seam refusal for shipped transitions"
---

## Insight

When a governance-critical transition must be refused regardless of the gate
enforcement state, enforce in the **core seam** unconditionally — not in a
gate rule that only fires when the gate is enabled.

## Context

Feature 144-F implemented guard 1 to prevent generic `move_item` /
`update_item` from shipping a shipment outside the `ShipShipment` envelope.
The prior implementation only refused under `formalGateEnforced()`.

With formal enforcement OFF (the default), a plain `backlogit_move_item`
could ship any shipment with no gate, no evidence, and no durable event.
This was a more complete bypass than even an operator `--force`.

## Pattern

```go
// In UpdateArtifactWithGate, BEFORE the gate-applies check:
if peek.ArtifactType == "shipment" {
    if newStatus == string(ShipmentShipped) {
        // Unconditional: ShipShipment never calls this function.
        return nil, nil, fmt.Errorf("...: %w", ErrShipmentShippedRequiresEnvelope)
    }
}
```

The governed caller (`ShipShipment`) is exempt **by construction** because it
writes via a completely separate function (`moveShipmentStatusWithHeadGuard`).
No exemption flag through `updates` or `opts` is needed — that would be a
forgeable bypass lever.

## Secondary pattern: two-seam TOCTOU close

Add both an unlocked peek (fast path, before any lock) AND a locked
check-to-write revalidation (after lock + reload) so the guarantee does not
rely on the peek alone. This closes the TOCTOU window between the peek and
the actual write.

## Archive stamping corollary

For guard 2 (`ArchiveItem` stamping `archived_status: shipped`):
the status-transition hook is not registered for archive, so the check must
be explicit. Key on `artifactType == "shipment" && oldStatus == "shipped"`
from the same `fm` map the stamp uses — no extra DB read, no extra file
load. Reuse the existing detection predicate (`shippedEventPresence`)
so prevention and detection scan the same JSONL contract.

## Applied to

`internal/core/gate_transition.go`, `internal/core/artifacts.go`,
`internal/core/shipment.go`, `internal/core/archive.go`

## See also

Design doc: `docs/design-docs/2026-08-shipment-shipped-prevention-envelope.md`
Plan: `docs/exec-plans/2026-08-18-shipment-shipped-prevention-hardening-plan.md`
