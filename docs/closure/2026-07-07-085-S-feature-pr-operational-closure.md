---
chunk_strategy: h1-h2-h3
description: 'Pre-merge operational closure and release-readiness certification for shipment 085-S (shipment-gate empty-head fail-closed hardening) feature PR. Certifies the empty-shipment-head and empty-member-head fail-closed change in internal/core/shipment_gate.go as release-ready: full quality gates green (go test ./..., go vet, golangci-lint, gofmt), three-model pre-push adversarial review that first BLOCKED on finding F1 (a present-but-broken .git pointer misclassified as no-repo, re-opening the fail-open) and, after the fix, RE-REVIEW PASS with F1 closed and SEC-1/SEC-2 confirmed, plus two non-blocking residuals (N1 empty-.git-dir, N2 unanchored match) remediated with a message-independent os.Stat guard. Runtime verification exercises the real ShipShipment -> gateShipmentCompletion -> validateMemberGateEvidence path with real git subprocesses across the full behavioral matrix. Records monitoring signals (EventGateBlocked reason=empty-shipment-head / empty-member-head), rollback plan (pure revert; no schema/data migration), and follow-ups. Merged AUTONOMOUSLY under standing AFK operator authorization (merge-commit, P-009).'
doc_type: closure
docline:
    ms.date: 2026-07-07T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-07T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-07-085-S-feature-pr-operational-closure.md
title: 085-S Shipment-Gate Empty-Head Fail-Closed Hardening — Feature PR Operational Closure & Release Readiness
---

# Operational Closure — 085-S Feature PR (pre-merge)

**Date:** 2026-07-07
**Shipment:** 085-S — Shipment-gate empty-head fail-closed hardening
**Feature:** 085-F (+ task 085.001-T + 3 subtasks — 5 items)
**Branch:** `feat/085-shipment-gate-empty-head-fail-closed`
**PR:** #185 → base `main` — **MERGED** 2026-07-07T15:35:07Z, merge commit `7c129b0407db9beb943bc737df4bc3b287286b77` (2-parent: `ae00054` + `1e01843`)
**Gate:** §1.9 pre-merge release-readiness — **AUTONOMOUS merge under standing AFK operator authorization** (merge-commit, P-009)

## 1. Scope shipped

Closes two empty-head **fail-open holes** in the ENFORCED shipment-completion gate:
an empty **shipment** head and an empty **member** head_sha were each silently
skipped under enforcement, allowing a shipment with unprovable lineage to ship.
`ev.Enforced` does not track work-tree presence (the test broker fakes the git
probe), so a bounded, fail-closed repo-presence probe is the discriminator between
"real work tree → fail closed" and "genuine no-repo → preserve the legacy skip".
Strict TDD (red → green) per unit.

| Unit | Subtask | Change | Surface | Commit |
|---|---|---|---|---|
| 1 | 085.001.001-ST | bounded fail-closed repo-presence probe `inGitWorktreeBounded` + `initGitRepoNoCommits` fixture + `TestInGitWorktreeBounded` | `internal/core/shipment_gate.go` (+ tests) | 4844e45 |
| 2 | 085.001.002-ST | empty-shipment-head fail-closed in `gateShipmentCompletion` + `shipmentHeadUnresolvedInRepoError` (1AEA2B0E) | `internal/core/shipment_gate.go` (+ tests) | 9eb5a2f |
| 3 | 085.001.003-ST | empty-member-head fail-closed in `validateMemberGateEvidence` + flip R7 (B85DAEE8) | `internal/core/shipment_gate.go` (+ tests) | bf80557 |
| — | adversarial F1 | fail closed on present-but-broken `.git` pointer (tighten no-repo marker to `(or any of the parent directories)` + `LC_ALL=C`) | `internal/core/shipment_gate.go` (+ tests) | 586993f |
| — | adversarial N1/N2 | message-independent broken-repo guard (`os.Stat(RootPath/.git)` → fail closed) + empty-`.git`-dir regression | `internal/core/shipment_gate.go` (+ tests) | 203a4b1 |

**Scope discipline:** empty-head fail-closed only. Untouched: malformed-JSONL
(`F3844849`, next phase) and all other stash items. The 084 ancestor-aware path
(equality fast-path, `isAncestor`, malformed-guard) is preserved unchanged.

## 2. Release-readiness gate (§1.9)

| Criterion | Result |
|---|---|
| `go test ./...` | ✅ PASS (all packages) |
| `go vet ./internal/core/...` | ✅ PASS |
| `golangci-lint run ./internal/core/...` | ✅ PASS (exit 0) |
| `gofmt` (structural, LF-normalized) | ✅ clean |
| Pre-push adversarial review (3-model) | ✅ PASS on RE-REVIEW — first pass BLOCKED on F1, fixed; 0 gate-blocking (HIGH P0/P1); SEC-1 + SEC-2 CONFIRMED |
| CI checks | ✅ PASS — all 4: test (1.23), test (1.24), Docline frontmatter gate, CLI Reference Drift |
| Copilot review | ✅ 0 unresolved — 2 fix rounds (os.Stat indeterminate-error fail-closed 1e3b31d; no-repo test git-guard 1e01843), each replied + resolved; fresh review on 1e01843 generated no new comments |
| Runtime verification | ✅ PASS — real `ShipShipment`/gate + real git; full behavioral matrix |

Artifacts: `docs/closure/2026-07-07-085-S-adversarial-review.md` (incl. F1 + N1/N2
remediation + re-review PASS), `docs/closure/2026-07-07-085-S-feature-pr-runtime-verification.md`.

## 3. Security posture

- **Fail-open holes closed (SEC-1 CONFIRMED by 3-model re-review):** an empty
  shipment head (1AEA2B0E) and an empty member head_sha (B85DAEE8) inside a real
  work tree under enforcement now **refuse** — lineage cannot be proven, so the
  ship is blocked rather than silently passed. A present-but-broken repo (broken
  `.git` pointer F1, empty/corrupt `.git` dir N1) also fails closed.
- **Legitimate-empty preserved (SEC-2 CONFIRMED, no completion breakage):** a
  genuine no-repo empty shipment head still ships, a no-repo empty member head
  stays skipped, and non-enforcement / bare-repo / inside-`.git` paths are
  unchanged. A forced/break-glass member that still records a real head is honored
  via the ancestor-aware path; the empty-head refusal is **uniform** — a real work
  tree under enforcement with no recorded head fails closed regardless of a forced
  flag (it cannot prove lineage; F5 intended-by-design). Discriminator is
  `inGitWorktreeBounded`, not `ev.Enforced`.
- **Fail-closed discipline:** probe timeout, cancellation, exec failure, corrupt
  repo, or missing git → non-nil error → refuse. `runCtx.Err()` is checked FIRST
  (a context-killed git reporting a platform exit code is never misread). The
  primary broken-repo discriminator is a **message-independent** `os.Stat` of the
  `.git` entry (defends against git message/locale/version drift); `LC_ALL=C`
  additionally pins the English no-repo marker.
- **Injection-closed / DoS-bounded:** argv-array exec (no shell) + `gate.MinimalEnv()`
  (allowlisted env) preserved; the probe self-derives a bounded deadline
  (`boundedHelperTimeout` ≤ `ancestryCheckTimeout` = 5s) so it cannot pin the
  workspace lock.
- **Observability (Constitution Principle V):** both fail-closed branches emit an
  `EventGateBlocked` evidence event (`reason: empty-shipment-head` /
  `empty-member-head`) AND a `slog.WarnContext`, so an over-refusal is a real
  monitoring signal, not a silent refusal.

## 4. Monitoring signals (post-merge)

- A `shipment ship` refusal citing "cannot resolve shipment head in repository" or
  "gate evidence has no recorded head_sha (cannot verify lineage under enforcement)"
  is the new guard working as intended — investigate the underlying git/evidence
  state, not the gate.
- `EventGateBlocked` events with `reason=empty-shipment-head` / `empty-member-head`
  in the gate evidence log are the observable signal for over-refusal monitoring.
- No new metrics/telemetry surface; behavior is observable via the existing gate
  evidence event log and `shipment ship` exit codes.

## 5. Rollback plan

- **Pure revert.** The change is confined to `internal/core/shipment_gate.go`
  (+ tests). No schema change, no data migration, no persisted-format change.
  Reverting the feature/fix commits restores the prior (fail-open) skip behavior
  with zero cleanup. Backlog archival (done-state) is independent and need not be
  reverted.
- Blast radius: the shipment completion gate only. Task-level gating, harvest, and
  all non-ship paths are untouched.

## 6. Follow-ups (stashed for Stage)

- **F3844849 (next phase):** malformed-JSONL member-evidence hardening — explicitly
  out of scope for 085-S; remains stashed for a future shipment.
- No new deferred follow-ups from the adversarial review: F1 fixed (586993f),
  N1 + N2 fixed (203a4b1), F2/F3/F4 addressed or unreachable, F5 intended-by-design.

## 7. Merge gate

**AUTONOMOUS merge under standing AFK operator authorization.** The operator is AFK
with full standing authority to open and merge this security-hardening PR. Merge
executed via admin bypass (`gh pr merge 185 --merge --admin --delete-branch`) with
**merge-commit** strategy (P-009). **Merge confirmed:** PR #185 state MERGED, merge
commit `7c129b0407db9beb943bc737df4bc3b287286b77` is a two-parent merge
(`ae00054` main + `1e01843` branch head) and is an ancestor of `origin/main`. CI
green (4/4) and Copilot 0-unresolved were confirmed before merge. Feature branch
deleted on merge.
