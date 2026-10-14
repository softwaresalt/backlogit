---
title: "Compacted memory: 2026-09-14 resumable shipment blocked lifecycle intake (174-F / 155-S)"
date_range: "2026-09-14 to 2026-09-14"
doc_type: compacted-memory
source_count: 1
---

# Compacted memory: 2026-09-14

Compacts 1 source file (roughly 16 KB, multi-amendment Stage record). The original was moved to docs/archive/memory/ (not deleted).

## Decisions made

- Capability: a non-terminal, resumable shipment lifecycle status `blocked`. Option A was chosen in the deliberation and supersedes the parked S12 path.
- Intake: stash 808E4323 was harvested into covering feature 174-F with tasks 174.001-T through 174.021-T. The stash was consumed. Shipment 155-S was assembled and queued.
- Links: 174-F supersedes 164.001-T and is related_to 164-F.
- Stash 7AA35A39 (DEFERRED SCOPE EXPANSION, informing defect) stayed active and was not absorbed.
- Cross-repository boundary (amendment): the shared contract belongs to the upstream autoharness backlog. The backlogit implementation is a local first-mover that is expected to be superseded. No autoharness ID was invented.
- Amendment 2: a temporary generic-move bootstrap seam for 154-S was considered. Migration debt (missing blocked_reason, blocked_at, blocked_by, resume_checkpoint_ref, and the event) had to be normalized before unblock.
- Remediation after a nine-P1 BLOCKED review of fee43b0a: governed member disposition on block, partial-failure durability (U19), active-slot source of truth via an authoritative Markdown scan with CAS, a generic-move bootstrap ruled out, and unblock requires explicit confirmation.
- Remediation cycle 3: 174.028-T and 174.029-T were added. 155-S reached 30 members. A one-time operator-approved pre-governance bootstrap runbook replaced the blanket prohibition. Stage did not execute it.

## Files modified or created

- docs/product-specs/2026-09-14-resumable-shipment-blocked-lifecycle-status.md
- docs/decisions/2026-09-14-resumable-shipment-blocked-lifecycle-status-deliberation.md
- docs/exec-plans/2026-09-14-resumable-shipment-blocked-lifecycle-plan.md (units U1 to U19 and splits; several plan review records)
- Backlog: 174-F, tasks 174.001-T to 174.029-T, shipment 155-S, and 35 dependency edges. Also 174.022-T and 174.023-T from the amendments.

## Key learnings

- Stage ran in degraded mode through the backlogit CLI because MCP tools were not callable in that session.
- The generic move to blocked routes through UpdateArtifactWithGate, but shipment-specific transitions (ShipmentStatus and isValidShipmentTransition) still lacked blocked, and no governed reason or checkpoint metadata or event existed.
- The external autoharness topology gate rejects blocked (topology.py line 553). Treating blocked as non-active was an unconfirmed external assumption that had to be verified separately.
- Plan review attempt 2 was ADVISORY with zero P1 findings after P2s were remediated inline.

## Failed or rejected approaches

- Generic-move bootstrap of 154-S, rejected after review: it has no member disposition, leaves 173.006-T active, and has no valid rollback once the U2b guards land.
- Invalid rollback of `move 154-S --status active`.
- Attempt 1 of the plan review (FAIL), superseded by attempt 2.

## Outcomes and open items

- 154-S stays active for now. No code path existed yet. Ship was not invoked.
- Stash 7AA35A39 stays active.
- Open: the U18 normalizer must run before any unblock of a blocked shipment. Autoharness topology-gate treatment of blocked remains to be verified.
- Next step belongs to Ship: claim 155-S when ready and implement U1 to U16 in topological order.

## Sources

- docs/memory/2026-09-14/stage-blocked-shipment-lifecycle-harvest-memory.md
