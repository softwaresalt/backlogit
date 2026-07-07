---
chunk_strategy: h1-h2-h3
description: 'Pre-merge operational closure and release-readiness certification for shipment 084-S (ancestor-aware shipment-gate member-evidence staleness) feature PR #182. Certifies the strict-equality to ancestor-aware staleness change in internal/core/shipment_gate.go as release-ready: full quality gates green (go test ./..., go vet, golangci-lint, gofmt), all four CI checks pass, pre-push three-model adversarial review PASS with zero gate-blocking P0/P1 findings (non-weakening and fail-closed confirmed), one Copilot review round with three findings (two timeout-cap DoS hardening + one duplicate-message cleanup) all fixed/replied/graphql-resolved, and runtime verification exercising the real gateShipmentCompletion + real git merge-base --is-ancestor subprocess demonstrating the post-merge ancestor pass, divergent refusal, equality pass, and fail-closed paths. Records monitoring signals, rollback plan (pure revert; no schema or data migration), and three deferred follow-ups (ABA head-drift binding, ambient-HEAD-vs-target-SHA anchor, joint scope re-evaluation with B85DAEE8/F3844849). HALTED at the P-014 merge gate for explicit operator approval — NOT merged.'
doc_type: closure
docline:
    ms.date: 2026-07-06T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-06T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-06-084-S-feature-pr-operational-closure.md
title: 084-S Ancestor-Aware Shipment-Gate Staleness — Feature PR Operational Closure & Release Readiness
---

# Operational Closure — 084-S Feature PR (pre-merge)

**Date:** 2026-07-06
**Shipment:** 084-S — Ancestor-aware shipment-gate member-evidence staleness
**Feature:** 084-F (+ task 084.001-T + 3 subtasks — 5 items)
**Branch:** `feat/084-ancestor-aware-staleness`
**PR:** #182 → base `main`
**Gate:** §1.9 pre-merge release-readiness — **HALT for operator P-014 merge approval**

## 1. Scope shipped

Makes the shipment completion gate's member-evidence **staleness** check ancestor-aware,
unblocking post-merge multi-commit shipment closure (which blocked 083-S). Strict TDD
(red harness → green) per unit.

| Unit | Subtask | Change | Surface |
|---|---|---|---|
| 1 | 084.001.001-ST | bounded git ancestor-lineage helper `isAncestor` + SHA-shape guard `isGitObjectName` | `internal/core/shipment_gate.go` |
| 2 | 084.001.002-ST | rewire `validateMemberGateEvidence` staleness branch to accept ancestor-or-equal heads, reject divergent, fail closed | `internal/core/shipment_gate.go` + tests |
| 3 | 084.001.003-ST | bracket `gateShipmentCompletion` with a single bounded head pre-resolve + fail-closed head-resolve error + last-read head-drift guard | `internal/core/shipment_gate.go` + tests |
| — | (Copilot remediation) | factor `boundedHelperTimeout` — hard-cap the fast git-metadata helpers at `ancestryCheckTimeout` (5s) so a 600s configured gate-command timeout cannot hold the workspace lock; dedup context-abort message | `internal/core/shipment_gate.go` |

**Scope discipline:** NON-empty member head only. Untouched: empty-member-head bypass
(`B85DAEE8`), empty-shipment-head fail-open (`1AEA2B0E`), malformed-JSONL (`F3844849`).

## 2. Release-readiness gate (§1.9)

| Criterion | Result |
|---|---|
| `go test ./...` | ✅ PASS (all packages) |
| `go vet ./internal/core/` | ✅ PASS |
| `golangci-lint run ./internal/core/...` | ✅ PASS (exit 0) |
| `gofmt -l internal/core/shipment_gate.go` | ✅ clean (LF-normalized) |
| CI — test (1.23) | ✅ pass |
| CI — test (1.24) | ✅ pass |
| CI — Docline frontmatter gate | ✅ pass |
| CI — CLI Reference Drift | ✅ pass |
| Pre-push adversarial review (3-model) | ✅ PASS — 0 gate-blocking (P0/P1); non-weakening + fail-closed confirmed |
| Copilot review | ✅ 3 findings, all fixed + replied + GraphQL-resolved; 0 unresolved threads |
| Runtime verification | ✅ PASS — real `gateShipmentCompletion` + real git; ancestor pass, divergent refuse, equality pass, fail-closed |

Artifacts: `docs/closure/2026-07-06-084-S-feature-pr-adversarial-review.md`,
`docs/closure/2026-07-06-084-S-feature-pr-runtime-verification.md`.

## 3. Security posture

- **Non-weakening (confirmed by all three reviewers):** `git merge-base --is-ancestor`
  exit 0 proves the member's gated commit is **reachable in shipment lineage**; final-tree
  content integrity is delegated to the **unchanged** shipment-level aggregate full-diff
  check #2. Divergent heads are still rejected.
- **Fail-closed (confirmed):** git exec error, exit 128, any non-{0,1} code, context
  deadline, parent cancel, missing binary, malformed/absent object → refuse. The
  `runCtx.Err()` check precedes the ExitError trichotomy so a context-killed process
  reporting exit 1 (Windows) is never misread as "not-ancestor".
- **Injection-closed:** untrusted on-disk `head_sha` is SHA-shape-validated before any git
  exec; argv-array (no shell) + `gate.MinimalEnv()` preserved.
- **DoS-bounded (Copilot):** helper subprocesses hard-capped at 5s regardless of the
  gate-command timeout, so a hung git metadata read cannot pin the workspace lock.

## 4. Monitoring signals (post-merge)

- Post-merge shipment closures for multi-commit shipments should now succeed where they
  previously failed with "member … gate evidence is stale". Watch for the first real
  post-merge `shipment ship` (e.g. re-attempt of the 083-S closure that this unblocks).
- Any `shipment ship` refusal citing "recorded at a divergent head", "cannot verify gate
  evidence lineage", or "head advanced during evaluation" is the guard working as intended —
  investigate the underlying git state, not the gate.
- No new metrics/telemetry surface added; behavior is observable via the existing gate
  evidence event log and `shipment ship` exit codes.

## 5. Rollback plan

- **Pure revert.** The change is confined to `internal/core/shipment_gate.go` (+ tests).
  No schema change, no data migration, no persisted-format change (member evidence
  `head_sha` recording is unchanged). Reverting the feature commits restores strict-equality
  behavior with zero cleanup. Backlog archival (task done-state) is independent and need not
  be reverted.
- Blast radius: the shipment completion gate only. Task-level gating, harvest, and all
  non-ship paths are untouched.

## 6. Deferred follow-ups (stashed for Stage)

From the adversarial review (all advisory, none a regression from the strict-equality
baseline):

- **ADV-2 (P2):** ABA / there-and-back HEAD race — the drift bracket detects net HEAD drift
  between two `rev-parse` samples but not a transient advance-and-reset, and does not bind
  the broker's internal HEAD resolution to backlogit's samples. Requires local repo write
  access in a seconds-wide window.
- **ADV-3 (P2):** the gate anchors to ambient `git rev-parse HEAD` rather than the explicit
  `--sha` target commit passed to `ShipShipment`. Pre-existing design property; recommend
  threading the target SHA in a future hardening.
- **ADV-5 (P3):** re-evaluate the interaction of ancestor-awareness with the excluded
  empty-member-head bypass (`B85DAEE8`) and malformed-JSONL (`F3844849`) when those stash
  items are addressed (older-ancestor-passing-event selection path).

## 7. Merge gate

**HALTED at §1.9 / P-014.** The feature PR #182 is merge-ready. Per operator direction,
this security-critical change awaits **explicit operator merge approval** and was **NOT
merged**. Merge-commit strategy (P-009) to be verified at the merge gate.
