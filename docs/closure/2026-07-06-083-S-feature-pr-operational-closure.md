---
chunk_strategy: h1-h2-h3
description: 'Pre-merge operational closure and release-readiness certification for shipment 083-S (gate-broker phase-2 hardening) feature PR #180. Certifies the F1/F4/F5/F7 + Q3 indexed gate-evidence read-model changes as release-ready: full quality gates green (go test ./..., go vet, golangci-lint, gofmt), all 4 CI checks pass, three-round Copilot review converged to zero unresolved threads, pre-push multi-model adversarial review PASS after remediating two HIGH-confidence P1 gate-blockers. Runtime verification exercised the real Q3 gate-evidence read-model: idx_gate_evidence_status present, projection populated with 9 passed rows for the gated 083.x items, indexed positive-query contract confirmed, and two consecutive backlogit sync runs produced identical doctor --check-gate-evidence output (idempotent, logs remain source of truth). Records monitoring signals, rollback plan (pure revert; projection is disposable and rebuilt on next sync), and deferred follow-ups. GATE PASS — cleared for merge-commit merge.'
doc_type: closure
docline:
    ms.date: 2026-07-06T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-06T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-06-083-S-feature-pr-operational-closure.md
title: 083-S Gate-Broker Phase-2 Hardening — Feature PR Operational Closure & Release Readiness
---

# Operational Closure — 083-S Feature PR (pre-merge)

**Date:** 2026-07-06
**Shipment:** 083-S — Gate-Broker Phase-2 Hardening
**Feature:** 083-F (+ 5 tasks, 4 subtasks — 9 executable items)
**Branch:** `feat/gate-broker-phase2-hardening`
**PR:** #180 → base `main`
**Gate:** §1.9 pre-merge release-readiness (operator-directed autonomous merge)

## 1. Scope shipped

Hardens the just-shipped Pre-Task-Completion Gate Broker (082-F) with the five
deferred phase-2 follow-ups. Strict TDD (red harness → green) per item.

| Item | Task | Change | Surface |
|---|---|---|---|
| F4 | 083.002-T | shipment member-evidence requires `(Forced) OR (Passed && ran==true)`; rejects fail-open `ran=false` passes | `internal/core` (shipment gate) + shared predicate |
| F1 | 083.001-T | advisory warning when `hooks.yaml base_ref` and `--gate-base` both set; precedence unchanged (config-first, no behavior change) | `internal/core` gate base resolution |
| F5 | 083.003-T | preserve shipment `DecisionError` exit 7/8 (config/setup vs retryable/timeout) instead of collapsing to `GateBlockedError`; wired through `shipment ship` CLI | `internal/core` + `internal/cli/shipment.go` |
| F7 | 083.004-T | `move --json` emits a structured payload for `*GateError` (parity with `*GateBlockedError`) | `internal/cli/move.go` |
| Q3 | 083.005-T (+Q3.0–Q3.3) | derived, indexed `gate_evidence` read-model populated from item logs during sync/rehydration; disposable & rebuilt on `backlogit sync`; logs remain source of truth; doctor `--check-gate-evidence` repointed to the positive index with authoritative log-scan fallback | `internal/db` (schema + rehydration), `internal/core/doctor`, new leaf `internal/gateevidence` |

## 2. Release-readiness gate (§1.9)

| Criterion | Result |
|---|---|
| `go build ./...` | ✅ 0 |
| `go test ./...` (full suite) | ✅ all packages `ok` |
| `go vet ./...` | ✅ 0 |
| `golangci-lint run` | ✅ 0 |
| `gofmt -l .` | ✅ clean |
| CI — CLI Reference Drift | ✅ pass |
| CI — Docline frontmatter gate | ✅ pass |
| CI — test (1.23) | ✅ pass |
| CI — test (1.24) | ✅ pass |
| Standard review skill (P0/P1) | ✅ none outstanding |
| Pre-push adversarial review (3 models) | ✅ PASS (2 P1 remediated pre-push) |
| Copilot review | ✅ converged — fresh review on HEAD `988900b`: "reviewed 43/43 files, generated no new comments"; 0 unresolved threads |
| Merge strategy (P-009) | ✅ repo enforces merge-commit only (`merge:true, squash:false, rebase:false`) |

**Verdict: GATE PASS — cleared for merge-commit merge.**

## 3. Runtime verification

Exercised the real Q3 gate-evidence read-model against the live workspace DB
(`.backlogit/backlogit.db`):

- `idx_gate_evidence_status` index present (Q3.1).
- `gate_evidence` projection populated: **9 `passed` rows** for the gated 083.x
  items (`083.001-T`, `083.005-T`, `083.005.001-ST`…`083.005.004-ST`, etc.) (Q3.2).
- Indexed positive-query contract (`WHERE gate_status IN ('passed','forced','forced_no_run')`)
  returns exactly the positive rows — the `LoadPassingGateEvidence` contract (Q3.3).
- **Idempotency:** two consecutive `backlogit sync` runs produced identical
  `doctor --check-gate-evidence` output (187 advisory findings, same rows) — the
  projection is a pure, deterministic function of the append-only logs and is
  fully rebuilt each sync; **logs remain the single source of truth.**
- doctor `--check-gate-evidence` exercises the positive-index fast-path with
  authoritative per-item log-scan fallback for absent/non-positive items.

F4/F5/F7 error/exit paths (fail-open rejection, exit 7/8, GateError JSON) are
covered by the green unit + integration suites and the adversarial review;
triggering their error paths at runtime requires deliberately misconfiguring the
gate on the live workspace, which was judged an unnecessary risk given full
deterministic test coverage.

## 4. Monitoring signals (post-merge)

- `backlogit doctor --check-gate-evidence` advisory findings count — a sudden
  change in `missing_gate_evidence` volume after a sync may indicate a projection
  population regression (compare against the 187 advisory baseline; exit code is
  intentionally unaffected — advisory only).
- `backlogit sync` "Indexed N artifacts" line and sync duration — the Q3
  projection now rebuilds from the same per-item events `rehydrateItemLogs`
  already parses (no second log walk), so sync log-I/O should be flat vs 082-F.
- Shipment/move exit codes: watch for exit 7/8 (config vs retryable) surfacing
  distinctly rather than collapsing to 1/6.

## 5. Rollback plan

Low risk. The change is additive + behavior-preserving on the happy path:

- **Revert:** a plain `git revert` of the merge commit fully restores 082-F
  behavior. No data migration is required.
- **`gate_evidence` table:** a disposable read-model. If a reverted binary no
  longer references it, the table is simply ignored; the next `backlogit sync`
  on any binary rebuilds/repopulates the projection from the authoritative logs.
  No item log or frontmatter is ever mutated by Q3, so there is no destructive
  state to unwind.
- **F1/F4/F5/F7:** pure code-path changes with no persisted-state footprint;
  revert is complete on binary replacement.

## 6. Deferred follow-ups (to be stashed for Stage at post-merge closure)

1. **Member-evidence empty `head_sha` staleness** — the shipment member-evidence
   staleness check skips comparison when the recorded `head_sha` is empty; an
   evidence event authored without a head SHA cannot be detected as stale. Advisory
   hardening candidate (surfaced by adversarial reviewer B; non-blocking).
2. **Unify malformed-line handling** — `parseItemLogFile` (rehydration path) errors
   on a malformed JSONL line while `events.ReadAllEvents` (doctor fallback path)
   skips it. Converging both on one policy would remove a latent divergence
   (surfaced by adversarial reviewer A P3; non-blocking).

## 7. Copilot review convergence (feature PR #180)

| Round | Commit reviewed | Outcome |
|---|---|---|
| 1 | `752f41d`/`babf9d5` | 1 thread — `rehydration.go` double log walk (explicitly optional perf tradeoff) |
| 2 | `d701734` | 2 threads — `doctor.go` dead `ws` param; `gateevidence.Latest` per-event heap copy |
| 3 | `988900b` | **0 new comments — "reviewed 43/43 files, generated no new comments"** |

All 3 threads: replied via REST `in_reply_to` referencing the fix commit, then
resolved via `gh api graphql resolveReviewThread` (`isResolved: true` confirmed).
Final state: **0 unresolved threads.**

## 8. Disposition

**GATE PASS.** Feature PR #180 is release-ready and cleared for an autonomous
merge-commit merge (admin bypass, delete branch) per standing operator
authorization. Post-merge closure (shipment ship 083-S, reconcile, compound
refresh, follow-up stash, closure PR) follows on a `post-merge/083-S` branch.
