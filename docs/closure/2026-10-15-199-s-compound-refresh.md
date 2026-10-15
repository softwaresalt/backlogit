---
doc_type: closure
docline:
  date: 2026-10-15T20:00:00Z
  status: final
  tags:
    - compound-refresh
    - 199-S
    - durable-writes
schema_version: "1.0"
title: "199-S compound refresh"
---

# 199-S compound refresh

Context: post-merge closure for shipment 199-S (PR #498, merge commit `1f25d10daed74ce209a1f14ec7160041bb0a4b6e`). Mode: `apply`, scope: compound entries whose anchors the 199-S change set touched.

## Entries reviewed

| Entry | Classification | Evidence | Action |
|---|---|---|---|
| `docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md` | update | Core two-class contract and commit-then-surface pattern still accurate. Anchors `internal/errors/durability_errors.go`, `adoptDurabilityErr`, and `persistArtifact` verified on 2026-10-15. 199-S extended the contract to the non-durable append path in `internal/events/stream.go` and to BlockShipment/UnblockShipment compensation accounting, which the entry did not describe. | Applied: appended a dated refresh section with citations to the closure artifact. |
| `docs/compound/2026-07-29-durable-writes-test-seam-patterns.md` | keep | `persistArtifactWriteFn` seam (`internal/core/shipment.go:1886`) and `WriteArtifactFileWithOptions(artifact, filePath, durable) error` (`internal/core/artifacts.go:925`) match the documented shape. 199-S did not change either. | None. |

## Out of scope

Other entries under `docs/compound/` do not reference the 199-S change set and were not reviewed in depth in this pass. No compound entry was deleted or consolidated.

## Known caveat recorded in the updated entry

AdoptItem re-ID of blocked or shipped shipment artifacts is a pre-existing gap (adversarial review P1, stash `34C23BE7`). The updated entry no longer presents AdoptItem as a fully correct re-ID reference.

## Follow-ups for Stage

* Stage may want to confirm that the non-durable append classification (`internal/events/stream.go`) belongs in the durable-writes learning rather than a separate entry. This pass chose an in-place update.
