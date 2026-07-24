---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
description: "Post-merge closure for shipment 104-S — add blocked->queued and active->queued to the validated status-transition map (both definition sites) plus a load-time legacy-map upgrade for persisted hooks.yaml, resolving the doctor-doc contradiction that queued was unreachable. Feature 124-F, tasks 124.002-T/124.003-T/124.004-T, harvested from stash bug BD8DBB85. Plan staged via PR #293; code merged via PR #294 (merge commit 96664088)."
doc_type: closure
docline:
  ms.date: 2026-07-23T00:00:00Z
  ms.topic: reference
source: docs/closure/2026-07-23-104-S-return-to-queued-transition-closure.md
title: "104-S return-to-queued transition closure"
---

## Scope

Post-merge closure for shipment **104-S** (return-to-`queued` transition
alignment) — feature **124-F**, tasks **124.002-T**, **124.003-T**,
**124.004-T**, harvested from stash bug **BD8DBB85** (autoharness consumer
feedback). The implementation plan was staged via **PR #293** (merge commit
`70712c2e`); the code merged via **PR #294**, merge commit
`96664088ee817a39ae4d23a1863b22a473b03066`.

## Problem

The validated status-transition map had no path back to `queued` from any later
state: `blocked` could only resume as `active` (`blocked -> active`), and
`active` had no transition to `queued`. This contradicted two existing
behaviors:

1. The gate broker's `redirectGate` / `writeStatusDirect`
   (`internal/core/gate_transition.go`) already moves a repeatedly-failing item
   `active -> queued` by **deliberately bypassing** the transition validator.
2. The `backlogit doctor` long-help / generated
   `docs/cli-reference/backlogit_doctor.md` documented a
   `--status queued` resume that runtime enforcement rejected.

Runtime enforcement is the pre-hook `hooks.ValidateStatusTransition`
(`internal/hooks/builtin_pre.go:36`), wired in `internal/core/workspace.go:118`
and gated by config `Lifecycle.ValidateTransition`. The standalone
`core.ValidateTransition` helper (`internal/core/harness_status.go:28`) is not
the production enforcement path.

## Decision (Option A)

Add BOTH `blocked -> queued` AND `active -> queued` to the validated map
(operator-confirmed). Rejected Option B (docs-only "correct the doctor doc to
`--status active`") because it would have hidden the real asymmetry and
contradicted the gate broker's existing `active -> queued` requeue. Decision
record: `docs/decisions/2026-07-23-return-to-queued-transition-deliberation.md`.
Plan: `docs/exec-plans/2026-07-23-return-to-queued-transition-plan.md`.

## What shipped

| Deliverable | Item | Detail |
|---|---|---|
| Transition-map widening | 124.002-T | `internal/config/defaults.go` (`DefaultHooksConfig().Lifecycle.Transitions`, the production-wired map) and `internal/hooks/builtin_pre.go` (`DefaultTransitions()`, the empty-map fallback) — both gain `queued` on the `active` and `blocked` entries. Additive; no previously-valid transition removed. |
| Sync-guard test | 124.002-T | `internal/config/transitions_sync_test.go` (new) — mandatory `reflect.DeepEqual` pinning `hooks.DefaultTransitions() == config.DefaultHooksConfig().Lifecycle.Transitions`, plus positive/negative coverage feeding the production map into `ValidateStatusTransition`. |
| Persisted-config legacy upgrade | 124.004-T | `internal/config/loader.go` — `priorGeneratedDefaultTransitions` frozen constant + `upgradeLegacyTransitions` helper invoked from `LoadHooks`. Deep-equals the persisted `lifecycle.transitions` map against the known prior generated default: upgrades ONLY on an exact match (legacy generated map); any operator-customized map is left byte-for-byte untouched (never blind-inject `queued`). Mirrors the `PreTaskCompletionGate.Normalize()` precedent (082-F). |
| Loader tests | 124.004-T | `internal/config/hooks_normalize_test.go` (new) — legacy-map upgraded, operator-restricted map preserved, absent block resolves via existing fallback. |
| Comment/doc alignment | 124.003-T | `internal/core/shipment_state_integrity_test.go` stale comment updated; `internal/core/gate_transition.go` `redirectGate` bypass rationale reworded (the direct write is for gate-evidence ordering / `GateBlockedError` semantics / hook-reentry avoidance — no longer "the validator would reject it", since `active -> queued` is now validator-consistent). Doctor long-help and generated cli-reference verified accurate under Option A; no text change required. |

## The persisted-config gap (why 124.004-T exists)

`LoadHooks` (`internal/config/loader.go:120-151`) unmarshals `hooks.yaml` and
normalizes ONLY the `PreTaskCompletionGate` block — it does not merge
newly-added default transitions. `WriteDefaults` persists a full
`lifecycle.transitions:` map at init, and `internal/core/workspace.go:114-118`
wires THAT persisted map. So the `defaults.go` edit alone reaches only
newly-initialized workspaces; existing consumer workspaces (like the BD8DBB85
reporter) keep their persisted map and keep rejecting the new transitions until
it is upgraded on load. The legacy-vs-custom discrimination is the safety
constraint: a blind "always inject `queued`" would silently override an operator
who intentionally excluded it.

## Constitution deviation (documented)

Task 124.002-T touches 4 files — 2 inseparable transition-map definition sites
plus their 2 paired test files (the `reflect.DeepEqual` sync-guard needs its own
external `_test` package). Decomposition below this is impossible without
splitting a single atomic map edit. Recorded in the plan Constitution Check.

## Quality gates (HEAD be4c5267)

`go test ./...` all 27 packages green · `go vet ./...` clean ·
`golangci-lint run` zero warnings · `gofmt -l` (changed files) clean.

## CI + Copilot review (PR #294)

CI: all 4 checks green (`test` 3m11s, `Docline frontmatter gate`,
`CLI Reference Drift`, `Detect code changes`). Copilot review completed
**COMMENTED**, fresh (review `commit.oid` == HEAD `be4c5267`), reviewed **8/8
files** and **generated no comments**. §1.9 pre-merge gate: Check 1 (no pending
review request) OK, Check 2 (freshness — review covers HEAD) OK, Check 3 (zero
unresolved Copilot threads) OK.

The staging PR #293 (plan artifacts only) converged over 4 review-fix cycles
(findings 6 -> 4 -> 1 -> 1 -> 0), all resolved via GraphQL; the code PR #294
drew zero Copilot findings.

## GI/GR reconciliation (shipment-reconcile)

- **Pre-mode**: 104-S `queued` in queue; members 124-F + 124.002-T + 124.003-T +
  124.004-T `done` (Ship completed the task moves in the working tree). Claimed
  104-S `queued -> active`.
- **`ship_shipment 104-S`**: `shipment_status: shipped`; archived 124.002-T,
  124.003-T, 124.004-T, 104-S, 124-F; `returned_ids: []`.
- **Post-mode**: 104-S present in archive with `status: archived`; absent from
  queue. Index re-synced (922 artifacts).

## Regression trace

The change is purely additive to the transition map: no previously-valid
transition is removed, and the sync-guard test pins the two map copies in
lockstep so they cannot drift. The gate broker's validator-bypass write is
unchanged in behavior (it already produced `active -> queued`); only its
rationale comment was corrected. The loader upgrade is a one-way legacy-match
gate that never mutates a customized map, so no operator policy is silently
altered.

## Release-observability (operational evidence)

The shipped change modifies the status-transition validation surface and the
`hooks.yaml` load path — both runtime-affecting.

| Item | Value |
|---|---|
| Monitoring signal | **Manual** — no runtime metrics system for the local backlogit index. Health signal: `backlogit move {id} --status queued` from a `blocked` or `active` item succeeds; existing consumer workspaces honor the new transitions after the next `LoadHooks`. Automated guard: the sync-guard test + the three loader tests. |
| Owner | Ship agent (session); operator on return. |
| Observation window | Next requeue attempt (`--status queued`) in any workspace, and the first `LoadHooks` of a legacy consumer workspace. Surfaced by `go test ./internal/config/... ./internal/hooks/...` on every run. |
| Baseline | Pre-fix: `blocked`/`active` -> `queued` rejected by `ValidateStatusTransition`; doctor `--status queued` resume contradicted enforcement. Post-fix: both transitions accepted; legacy persisted maps upgraded on load; customized maps preserved. |
| Rollback trigger | A legacy consumer map NOT upgraded on load, OR an operator-customized map mutated by `LoadHooks`, OR any `transitions_sync_test.go` / `hooks_normalize_test.go` failure, OR a previously-valid transition rejected. |
| Rollback procedure | Revert merge commit `96664088` (PR #294) via `git revert -m 1 96664088`. Clean at the code level (isolated to `internal/config` + `internal/hooks` + two `internal/core` comment edits, no schema or data migration). Reverting re-exposes the original defect (queued unreachable); prefer a roll-forward fix. No persisted-workspace data is rewritten by the revert — the loader upgrade is in-memory only. |

## Residual risk and follow-ups

- **No new follow-ups filed** from this shipment — Copilot generated zero code
  findings and no P0/P1/P2 review findings were outstanding at merge.
- `016.001-R` orphan remains recorded for separate triage (unrelated to this
  shipment).
- `123-F` fsync durability remains deferred as a standalone future release unit.

## Compound-refresh

No compound learning was invalidated. The persisted-config gap
(`LoadHooks` normalizes only the gate block, not new default transitions) is a
reusable trap worth remembering when adding any new default to `hooks.yaml`:
the in-code default reaches only freshly-initialized workspaces unless a
load-time legacy-map upgrade is added. The `upgradeLegacyTransitions` /
`priorGeneratedDefaultTransitions` pair follows the `PreTaskCompletionGate.Normalize()`
precedent (082-F) and is the pattern to reuse for future default-map evolution.
