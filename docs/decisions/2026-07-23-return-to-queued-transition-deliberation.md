---
chunk_strategy: h1-h2-h3
description: 'Deliberation for stash BD8DBB85 (bug, high): the validated status-transition map has no path back to queued from any later state, so a blocked item can only resume as active, yet the backlogit doctor long-help tells operators to resume with --status queued. Recommends Option A — add blocked->queued (and active->queued) to the validated map so the user-facing path matches the gate broker requeue and the documented resume.'
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-07-23-return-to-queued-transition-deliberation.md
title: 'Return-to-queued transition: align the validated status map with the documented and gate-driven requeue'
topic: "Status-transition map has no path back to queued from blocked/active"
depth: lightweight
stash_id: BD8DBB85
decision_status: decided
promoted_to: plan
linked_artifacts:
  - "docs/exec-plans/2026-07-23-return-to-queued-transition-plan.md"
  - "docs/decisions/2026-07-05-gate-repeated-failure-requeue-ownership-deliberation.md"
tags:
  - task-lifecycle
  - state-machine
  - transition-validation
  - gate-broker
  - cli-doc-parity
---

## Question

The validated status-transition map allows no path back to `queued` from any
later state: a `blocked` item can only resume as `active` (`blocked -> active`),
and `active` has no transition to `queued` either. The `backlogit doctor`
long-help text tells operators to resume a task with `--status queued`, which
the user-facing validator rejects. Should backlogit (Option A) add a
return-to-ready transition (`blocked -> queued`, and/or `active -> queued`) to
the validated map, or (Option B) correct the documentation to say
`--status active` and leave the map restrictive?

## Context

Verified against source on 2026-07-23:

- **The validated map has no return-to-`queued` path.** The default
  `Transitions` map is defined identically in two places:
  `internal/config/defaults.go:508-514` (`DefaultHooksConfig().Lifecycle.Transitions`,
  with `ValidateTransition: true` at `:506`) and
  `internal/hooks/builtin_pre.go:16-24` (`DefaultTransitions()`):
  - `queued: {active, blocked}`
  - `active: {done, blocked, review, shipped, abandoned}`  — no `queued`
  - `blocked: {active}`  — no `queued`
  - `review: {done, accepted, rejected}`
  - `done: {archived}`

- **Enforcement path.** User-facing moves (e.g. `backlogit move <id> --status
  queued`) run through the pre-hook `hooks.ValidateStatusTransition`
  (`internal/hooks/builtin_pre.go:36`), wired in
  `internal/core/workspace.go:118` and gated by config
  `Lifecycle.ValidateTransition` (yaml `validate_transition`,
  `internal/config/schema.go:129`). This path **rejects** `blocked -> queued`
  and `active -> queued`. The standalone helper `core.ValidateTransition`
  (`internal/core/harness_status.go:28`) is NOT the production enforcement path.

- **The system already lands items in `queued` from a later state internally.**
  The gate broker `Workspace.redirectGate` -> `writeStatusDirect`
  (`internal/core/gate_transition.go:264-283`) moves items `active -> queued`
  on repeated gate failure (requeue). It does so by **deliberately bypassing**
  the user-facing transition validator, with an explicit comment
  (`gate_transition.go:266-267`): "The redirect write bypasses the user-facing
  transition validator because the gate is the completion authority deciding
  this backward move." So `queued` reached from `active` is already a blessed,
  intended state — just not reachable through the validated user path.

- **Prior decision D23DFA0B is directly on point.**
  `docs/decisions/2026-07-05-gate-repeated-failure-requeue-ownership-deliberation.md`
  (`decision_status: decided`) established that on `on_repeated_failure: block`
  backlogit **requeues the task to `queued`**, and on `escalate` moves it to
  `blocked`. It reasons that a clear `queued`/`blocked` routing signal is how a
  dark-factory (no-HITL) flow self-routes and surfaces attention. That decision
  makes `active -> queued` an intended, operator-blessed lifecycle transition in
  at least the gate path.

- **The doc contradiction.** The `backlogit doctor` long-help
  (`internal/cli/doctor.go:64-69`, source of the generated
  `docs/cli-reference/backlogit_doctor.md`) reads: "On repeated gate failure,
  transition the task with the existing `backlogit move {id} --status blocked`
  (and `--status queued` to resume)". The generated `.md` matches this Go source.
  It promises a `blocked -> queued` resume that the validator currently rejects.

- **Test surface (no invariant depends on `queued` being unreachable).** No test
  asserts `blocked -> queued` or `active -> queued` is forbidden.
  `internal/hooks/builtin_pre_test.go:100` (`TestValidateStatusTransition_AllDefaultTransitions`)
  enumerates the currently-valid transitions and would gain the new entries.
  `internal/core/shipment_state_integrity_test.go:139` has a comment
  ("`blocked -> active` is the only hook-allowed exit") that becomes stale but
  its assertion (a `blocked -> active` move) stays valid.
  `internal/config/hooks_config_test.go` covers `DefaultHooksConfig` but does not
  pin the exact transition set.

## The tension

Autoharness (a downstream consumer) reports the asymmetry as a bug: the gate can
requeue an item to `queued`, and the docs tell operators to resume with
`--status queued`, but the validated user path forbids it. Either the code is
wrong (too restrictive) or the docs are wrong (over-promising). Both are
internally-consistent resolutions; the question is which better serves the
manual-recovery workflow and the established lifecycle semantics.

## Options

### Option A — Add a return-to-ready transition to the validated map (recommended)

Add `blocked -> queued` and `active -> queued` to the `Transitions` map in both
`internal/config/defaults.go` and `internal/hooks/builtin_pre.go`, so an operator
can manually resume/requeue a task into the ready pool.

- **Pro:** matches what the gate broker already does internally (`active ->
  queued`, D23DFA0B) and what the doctor long-help already tells users
  (`blocked -> queued`). It removes a real semantic asymmetry rather than papering
  over it.
- **Pro:** the doctor doc/help becomes **accurate as written** — no doc edit
  required; the code is brought into line with its own documentation and its own
  internal behavior.
- **Pro:** semantically correct recovery. `queued` means "ready / unassigned in
  the queue"; `active` means "being worked". Resuming a `blocked` item into
  `queued` returns it to the ready pool for re-selection, which is more faithful
  than forcing it straight to `active` (which implies someone is actively on it).
- **Pro:** gives dark-factory / multi-agent flows a manual requeue that mirrors
  the automated gate requeue, so operators and agents share one mental model.
- **Con:** widens the allowed manual transition set. Mitigated: no test or
  runtime invariant depends on `queued` being unreachable post-activation; the
  change is additive and backward-compatible (nothing that was valid becomes
  invalid).
- **Con:** two map copies must be kept in sync (see the memory note); the plan
  updates both and adds a test that pins the pair.

### Option B — Correct the docs only

Change the `backlogit doctor` long-help (`internal/cli/doctor.go`) and regenerate
`docs/cli-reference/backlogit_doctor.md` to say `--status active` (resume as
active); leave the validated map restrictive.

- **Pro:** minimal, lowest immediate risk; touches only doc/help text.
- **Con:** leaves the semantic asymmetry intact — the gate can requeue to
  `queued`, but operators cannot. The manual-recovery workflow stays
  second-class: an operator recovering a stuck/blocked item is forced to
  `active` even when returning it to the ready pool is the correct intent.
- **Con:** contradicts the direction of D23DFA0B, which treats `queued` as the
  legitimate "retryable / ready to be re-picked" landing state for backward
  moves. Option B entrenches a validator that disagrees with the blessed gate
  behavior.
- **Con:** does not satisfy the downstream consumer's actual need (a
  return-to-ready path); it only removes the documentation that advertised it.

## Recommendation

**Adopt Option A.** Add `blocked -> queued` and `active -> queued` to the
validated transition map in both definition sites.

Rationale (one paragraph): The bug is not that the documentation over-promises —
it is that the validated user-facing map is out of step with backlogit's own
established semantics. The gate broker already moves `active -> queued` on
repeated failure, and prior decision D23DFA0B explicitly blessed `queued` as the
"requeue / retryable / ready" landing state for backward lifecycle moves,
precisely so dark-factory flows can self-route on a clear queue signal. The
doctor long-help already documents a `--status queued` resume. Option A resolves
the contradiction by making the code true to both its documentation and its
internal behavior, and it upgrades manual recovery from a forced `-> active` to a
correct return-to-ready `-> queued`, mirroring the automated path operators
already rely on. The change is small (two additive map entries in two files) and
additive (no previously-valid transition becomes invalid, no invariant depends on
`queued` being unreachable), plus a loader-side normalization so existing
persisted workspaces are reached, so the risk is low and the fix targets the real
asymmetry rather than hiding it. `blocked -> queued` is the transition that
directly discharges the documented contradiction; `active -> queued` is included
for parity with the gate broker's common `active` completion path, so the manual
and automated requeue paths are symmetric for the states operators most often
recover from. This is deliberately narrower than full broker parity: the gate
broker redirects any non-terminal task/subtask that fails to enter a configured
terminal status (`gate_transition.go:99-117`), so it can also produce
`review -> queued`. Adding `review -> queued` to the validated manual map is
intentionally left out of scope here and can be deliberated separately if a
manual `review` recovery need emerges.

## Consequences

- The validated map gains `blocked -> queued` and `active -> queued`; operators
  and agents can manually requeue blocked or active items into the ready pool.
- The `backlogit doctor` long-help / generated `docs/cli-reference/backlogit_doctor.md`
  become accurate without edits (the code now honors what they describe).
- Existing workspaces that persist an explicit `lifecycle.transitions:` block in
  `hooks.yaml` are reached by normalizing the loaded map in `LoadHooks`
  (`internal/config/loader.go`), not by the in-code default edit alone; the plan
  carries this as a dedicated implementation unit (task `124.004-T`).
- Tests: `TestValidateStatusTransition_AllDefaultTransitions` gains two cases; a
  new assertion pins the new transitions in both map copies; the stale comment in
  `internal/core/shipment_state_integrity_test.go:139` is corrected.
- The gate broker's validator bypass (`gate_transition.go`) remains correct and
  harmless — it stays as an explicit "completion authority" write and is now
  simply consistent with, rather than in tension with, the validated map.

## Status

Decided — **Option A, adding BOTH `blocked -> queued` AND `active -> queued`**,
operator-confirmed at the staging PR. This is no longer pending: Ship implements
Option A as specified. Promoted to plan:
`docs/exec-plans/2026-07-23-return-to-queued-transition-plan.md`. The harvested
shipment (`104-S`) stays `queued` only as its normal pre-execution state in the
work queue — not because the decision is open. Option B (docs-only) was considered
and rejected; it is retained in this document solely as the record of the
alternative, not as an outstanding choice.
