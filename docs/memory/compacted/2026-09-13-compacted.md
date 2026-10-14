---
title: "Compacted memory: 2026-09-13 reliability and PR 425 security hardening (Stage, dark factory)"
date_range: "2026-09-13 to 2026-09-13"
doc_type: compacted-memory
source_count: 1
---

# Compacted memory: 2026-09-13

Compacts 1 source file (roughly 18 KB, multi-cycle Stage record). The original was moved to docs/archive/memory/ (not deleted).

## Decisions made

- Mode: P-017 dark factory (DARK_MODE_ACTIVE). Stage scope was planning artifacts only: no source, build, test execution, push, PR, or merge. merge_preapproval true; admin_fallback false; preclaim_force false.
- Stash triage of eight scoped IDs (6D53A33F, CC0EBB59, 1E0C2251, FE440C62, 3B661FCE, BFF76433, BF18DA1D, 4E210DB4). All were archived as state removed.
- Three groups: A (concurrency, FE440C62 and 1E0C2251, into 172-F / 153-S); B (shipment lifecycle, CC0EBB59, into 173-F / 154-S); C (PR 425 workspace-writer threat model, into hardening tasks under 168, 169, 170 and shipments 149-S, 150-S, 151-S).
- Reliability decisions: the universal claim-activation marker is the final claim contract, and record-only claim mode was dropped. Revocation must be authoritative and external, not an advisory status flip. Required-auth enablement is externally authoritative (170.009-T). Attestation uses durable signed material (169.007-T), superseding presence-based acceptance. Concurrency Unit 2 (archived_status CAS) is independent of Unit 1 (lock order).
- Final cycle 4 (operator-authorized one-time extension): 151-S may be claimed to build the control, but the hard gate is pre-completion, pre-shipping, and merge readiness. Documentation-only closure of 170.008-T is forbidden. The gate must rest on externally protected atomic compare-and-consume state.
- Queue positions were spaced by 100 (154-S=100, 153-S=200, 141-S=300, 152-S=400, and so on). No dependency edges changed.

## Files modified or created

- docs/decisions/ (dated 2026-09-13): six-slug decision set including 6d53a33f-disposition, concurrency-writer-hardening-deliberation, shipment-claim-scheduler-reconciliation-deliberation, pr425-workspace-writer-hardening-deliberation.
- docs/exec-plans/ (dated 2026-09-13): concurrency-writer-hardening-plan, shipment-claim-scheduler-reconciliation-plan, pr425-workspace-writer-hardening-plan.
- .backlogit/queue/170.008-T.md, 172.014-T.md, 172.015-T.md, plus new tasks 172.011-T, 172.012-T, 172.013-T, 168.009-T, 168.010-T, 169.008-T, 169.009-T, 170.009-T, 170.010-T, 173.006-T, 173.007-T, 172.008-T, 172.009-T, 172.010-T.
- Commits recorded on branch chore/stage-154-s-reliability (not pushed): harvest 88c54c7a; review-fix cycle 1; cycle 2 1d1eab39; remediation 9c06c159.

## Key learnings

- Plan review attempt 1 failed with a P1 in each plan; after remediation, attempt 2 passed in all three.
- Test-first ordering was restored by adding RED predecessor tasks for the RED harnesses, and by splitting GREEN work out.
- Source-shape harnesses must read production source as data using go/parser and go/ast, and must not reference symbols that do not exist yet. Their RED state must be an AST assertion failure, never a compile failure.
- Cycle 2 verification: three plan reviews passed, the dependency DAG was acyclic (500 edges, 436 nodes), docs lint had 0 violations, and sync indexed 1470 artifacts.
- Rollback guidance was rewritten so that rollback never restores forgeable or replayable behavior.
- Provenance limitation: stash correct was unsupported for seven generically archived entries, because no harvested_artifact_id existed. Hand-editing archive/stash.jsonl is prohibited. Compensating provenance was kept through [STASH-ID] tags.

## Failed or rejected approaches

- Record-only claim mode (dropped).
- Advisory-only revoke status flip (rejected in favor of external revoke or tombstone).
- Documentation-only closure for 170.008-T (forbidden).
- Using go test -race as proof of no lock-order inversion (replaced with a deterministic barrier).
- A pre-claim gate for 151-S that was circular (removed in cycle 4).
- Hand-editing the stash archive to fix provenance.

## Outcomes and open items

- Final state at cycle 4: five PR 442 findings were remediated in planning, backlog, and doc artifacts. The source does not record a post-cycle review result. No push, PR creation, or merge was performed.
- Open P2 advisory: the immutable attempt-2 review text in the PR 425 plan still says bounded tracked residual. It was left intact as an audit record.
- Open residual: the provenance limitation above.
- Shipments queued for Ship: 149-S, 150-S, 151-S, 153-S, 154-S. Ship should claim in reliability-weighted order. The 152-S claimability caveat (autoharness implicit numeric predecessor) remains a Ship-time decision.

## Sources

- docs/memory/2026-09-13-stage-reliability-security-hardening.md
