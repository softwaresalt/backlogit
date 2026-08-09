---
chunk_strategy: h1-h2-h3
description: "Compound-refresh pass triggered by 117-S (Formal Gate F1) post-merge closure: reviewed compound entries citing files this shipment modified (shipment_gate.go, gate_transition.go, task_lock.go, gateevidence) to check for staleness. All reviewed entries remain accurate; none required updates."
doc_type: closure
docline:
    date: 2026-08-09T00:00:00Z
    tags:
        - compound-refresh
        - formal-gate
        - 117-S
schema_version: "1.0"
source: docs/closure/2026-08-09-117-S-compound-refresh.md
title: "117-S Compound Refresh"
---

# 117-S Compound Refresh

## Scope

`recent` — entries citing files modified by shipment 117-S (Formal Gate F1):
`internal/core/shipment_gate.go`, `internal/core/gate_transition.go`,
`internal/core/task_lock.go`, `internal/gateevidence/*`.

## Entries Reviewed

| Entry | Classification | Rationale |
|---|---|---|
| `2026-07-06-ancestor-aware-shipment-gate-staleness.md` | keep | Documents the ancestor-vs-equality lineage check ALGORITHM in `validateMemberGateEvidence`. 117-S changed WHICH event's `head_sha` feeds that algorithm (round 4: `res.Event` instead of the legacy `latest`), not the algorithm itself. The doc's core claim ("compare lineage, not identity") remains fully accurate. |
| `2026-07-06-autoharness-gate-broker-integration-contract.md` | keep | Describes the `GateBroker.Evaluate` contract (Force/NoCount/Request semantics). 117-S wraps additional pre/post HEAD-stability checks AROUND calls to `Evaluate` without altering the contract itself. |
| `2026-07-20-ship-gate-descoped-archived-member-exemption.md` | keep | Describes `archivedFromDescopeEligibleStatus`, an orthogonal member-exemption check untouched by 117-S's formal-admission work. |
| `2026-07-28-attach-commit-repersist-must-reload-from-markdown.md` | keep | Unrelated persistence-reload lesson; incidentally shares a file reference but no overlapping claim. |
| `best-practices/shared-parser-convergence-observable-skip-2026-07-07.md` | keep | Unrelated parser-convergence lesson; incidental file co-citation only. |
| `2026-07-13-utc-frontmatter-timestamp-normalization.md` | keep | Unrelated timestamp-formatting lesson; incidental file co-citation only. |

## New Entries Added This Cycle

* `docs/compound/security-issues/2026-08-09-authenticate-before-filter-security-check-ordering.md`
* `docs/compound/security-issues/2026-08-09-audit-all-entry-points-sharing-guarded-state-transition.md`
* `docs/compound/concurrency-issues/2026-08-09-stable-lock-keys-and-heartbeat-refresh-for-unbounded-holds.md`

These are genuinely new failure modes discovered during this shipment's
review cycle (10 Copilot PR review rounds) with no pre-existing compound
entry covering the same specific lessons — no consolidation was needed. Two
new category subdirectories (`security-issues/`, `concurrency-issues/`) were
created to match the `compound` skill's documented category taxonomy, which
previously had no populated examples in this repository.

## Outcome

No existing entries required update, consolidation, replacement, or deletion.
Three new entries added (see above). No entries marked stale.
