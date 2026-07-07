---
chunk_strategy: h1-h2-h3
description: 'Deliberation for the pre-task-completion gate-broker phase-2 hardening bundle (covering feature over five deferred stash items 9822F787/F4, 7C5EADA6/F5, 83B885EE/F7, 162F5548/F1, 7ED9CE1A/Q3). Confirms the group is a coherent single covering feature, names it, and records the per-finding remediation direction and the two load-bearing design decisions (F4 ran-gating that preserves force semantics; F1 advisory-only dual-base warning that does not change config-first precedence). Four remediations are mutually independent single-file tasks; Q3 is a larger DB read-model task decomposed into three ordered subtasks.'
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-07-06-gate-broker-phase2-hardening-deliberation.md
title: 'Gate-broker phase-2 hardening: covering-feature scope and per-finding remediation direction'
stash_id: 9822F787,7C5EADA6,83B885EE,162F5548,7ED9CE1A
decision_status: decided
tags:
  - gate-broker
  - autoharness
  - hardening
  - shipment-gate
  - sqlite-read-model
  - cli-parity
---

## Question

The just-shipped Pre-Task-Completion Gate Broker (feature 082-F, shipment 082-S, merged to
`main`) left five follow-up items in the stash: four adversarial-review deferrals (F1, F4, F5,
F7) from `docs/closure/2026-07-06-pre-task-completion-gate-broker-adversarial-review.md` and one
Q3 design follow-up (a derived, indexed gate-evidence read-model column). Do these five form a
single coherent covering feature, and what is the right remediation direction for each?

Operator has pre-selected the grouping (all five bundled into one covering feature) and directed
planning-only execution (P-010 role isolation; Ship will execute). This deliberation validates
that the group is coherent, names the covering feature, and records the per-finding direction and
the two non-trivial design decisions so `impl-plan` and `plan-review` can proceed.

## Framing

All five items share one domain: the `internal/core/gate/*` + `internal/core/shipment_gate.go` +
`internal/cli` gate-completion seam and its `internal/db` evidence projection. Every item is a
direct descendant of 082-F. They touch the same subsystem, share the same reviewers/context, and
would naturally ship together as one coordinated hardening PR. Prior art:
`docs/compound/2026-07-06-autoharness-gate-broker-integration-contract.md` explicitly records the
F4 gap as a **known, deferred** gap ("the member scan does NOT currently inspect the evidence
`delta.ran` field ... Hardening that to reject `ran=false` is deferred, not yet in force"), and
the design doc (`docs/design-docs/2026-07-04-pre-task-completion-gate-broker.md`, §Resolved open
questions #3) names `7ED9CE1A` as the sanctioned Q3 follow-up. The grouping is therefore not
synthesized speculation — it is the exact deferred-work set the 082-F review and design already
enumerated.

**Coherence verdict: the five items are one coherent covering feature.** No item belongs in a
different group; no covering task is missing; the group's scope is bounded by "close the phase-2
hardening gaps the 082-F adversarial review deferred, plus land the Q3 evidence read-model."

## Covering feature

**Title:** `Pre-task-completion gate broker — phase-2 hardening`
**Type:** `feature` (repo has no `chore` type; this is a coordinated hardening release unit).
**Scope boundary:** the four deferred adversarial findings (F1/F4/F5/F7) plus the Q3 derived
gate-evidence read-model. Explicitly **out of scope** (deferred, do not pull in): F2/F3 (already
remediated pre-push per the closure doc), F6 (probe env/dir hardening — a separate advisory not
stashed in this set), F8/F9 (by-design/optional advisories), strict-mode gate-evidence
enforcement (a future item beyond the advisory doctor check).

## Options considered (grouping)

- **Option A — one covering feature over all five (chosen).** Matches the operator directive,
  the 082-F deferral set, and the natural PR boundary. One shipment, one covering feature, tight
  traceability back to the closure/design source docs.
- **Option B — split remediations (F1/F4/F5/F7) from the Q3 read-model into two features.**
  Rejected: the Q3 read-model's evidence predicate must stay consistent with the F4-hardened
  `latestGatePassEvidence` semantics (see Decision 2), so co-locating them in one feature keeps
  that consistency visible and reviewable in one plan. Splitting would fragment a single reviewer
  context for marginal benefit.

**Chosen direction: Option A.**

## Per-finding remediation direction (grounded in shipped code)

### F1 — `162F5548` · advisory dual-base warning (`internal/core/gate/baseref.go`)
Precedence is **config-first by locked design** (`ResolveBaseRef` checks `ConfigBaseRef` before
`GateBase`, baseref.go:59-64; confirmed by the integration-contract compound learning's base-ref
order). Operator directive: **advisory warning only, NOT a behavior change.** When a workspace
pins a non-auto `ConfigBaseRef` **and** the operator also supplies `--gate-base`, surface a
warning that the `--gate-base` override is shadowed by config. Config still wins. See Decision 1
for how the warning is surfaced.

### F4 — `9822F787` · shipment member-evidence must require `ran==true` (`internal/core/shipment_gate.go`)
`latestGatePassEvidence` (shipment_gate.go:146-155) accepts any `EventGatePassed`/`EventGateForced`
event, including a fail-open `ran=false` no-run pass, so a member that never actually ran the gate
satisfies the shipment's per-member reconciliation guarantee. Direction: require `delta["ran"] ==
true` for a real **pass** to count as gate evidence. See Decision 2 for the force-completion
nuance (this is the load-bearing design decision).

### F5 — `7C5EADA6` · shipment `DecisionError` class fidelity (`internal/core/shipment_gate.go`)
The shipment-diff branch (shipment_gate.go:63-91) collapses **every** non-proceed decision —
including `DecisionError{config}` (autoharness exit 2) and `DecisionError{timeout}` — into a
`GateBlockedError` (exit 6), losing the exit 7 (config/setup) / exit 8 (retryable/timeout)
classification that the task path preserves. Direction: before the block branch, handle
`ev.Decision.Kind == gate.DecisionError` separately and route it through the existing
`appendGateErrorEvidence` + `gateErrorFromClass(class, shipmentID, ...)` helpers — mirroring the
task-level `errorGate` (gate_transition.go:288-298) and the identical routing the file **already**
uses in its broker-`Evaluate`-error branch (shipment_gate.go:46-50). This is a low-risk
consistency fix that reuses code already present in the same file.

### F7 — `83B885EE` · `move --json` payload for the `*GateError` class (`internal/cli/move.go`, `internal/cli/gate_exit.go`)
Under `--json`, `moveGateError` (move.go:105-126) renders a body only for `*GateBlockedError`; a
`*GateError` (config/setup/timeout) returns exit 7/8 with **empty stdout**, an inconsistent
machine contract (the MCP surface does emit a structured error for these classes). The
`gateJSONPayload` struct already declares `Error`/`Retryable` fields (gate_exit.go:59-60) that are
never populated on the CLI error path. Direction: add a `renderGateErrorJSON(id, *GateError)`
branch in `gate_exit.go` (mirroring `renderGateBlockedJSON`) that marshals
`{id, outcome:"error", error, retryable}`, and call it from `moveGateError` when
`errors.As(err, &ge)` for `*GateError`. Pure additive; the human/exit-code path is unchanged.

### Q3 — `7ED9CE1A` · derived indexed gate-evidence read-model (`internal/db`, `internal/core/doctor`)
Add a **derived, disposable, indexed** read-model projection populated from the append-only item
logs during `sync_index`/rehydration, so agents can query "which done tasks have/lack gate
evidence at HEAD" without scanning logs. **Logs remain the source of truth**; the projection is
rebuilt on every `backlogit sync`. Grounded surface (from repo inspection):
- Schema DDL lives in `internal/db/schema.go` `EnsureSchema` (items table ~L270-295); pattern is
  create-if-not-exists + best-effort `ALTER TABLE ADD COLUMN` (no version table).
- Rehydration is a **clear-and-rebuild** cache (`internal/db/rehydration.go` `Rehydrate` deletes
  and repopulates `items`/`item_logs`), so a projection populated during rehydrate is inherently
  idempotent and rebuild-safe. Item-log rehydration hook: `rehydrateItemLogs` → `indexEventTx`
  (`internal/db/logs.go`).
- Gate evidence event constants live in `internal/core/gate_evidence.go`
  (`EventGatePassed`/`EventGateForced`/`EventGateError`/...); the commit SHA is `events.Event.CommitSHA`
  and the `ran` boolean + `head_sha` live in the event `Delta`.
- The advisory `doctor --check-gate-evidence` (`internal/core/doctor.go:398-420`) currently scans
  logs via `latestGatePassEvidence`; the design doc names the indexed variant as this exact
  follow-up.

Decomposition (Decision 3): schema → projection population → doctor indexed-query repoint.

## Load-bearing design decisions

### Decision 1 — how F1's advisory warning is surfaced
**Options:** (a) `slog.WarnContext` inside `ResolveBaseRef` (adds a `log/slog` import to the pure
`gate` package); (b) return a structured signal (e.g. a `ResolvedBase.OverrideShadowed bool` +
message, or a returned warnings slice) and let the core caller
(`gate_transition.go`/`buildGateBroker`) log/surface it.
**Chosen: (b) structured signal.** The `gate` package's `baseref.go` is deliberately pure and
exhaustively table-testable (it imports only stdlib + `internal/errors`); embedding `slog` couples
a pure resolver to a logging sink and makes the warning awkward to assert. A boolean/message field
on `ResolvedBase` keeps the resolver pure, is trivially table-testable (assert the field is set
iff both a non-auto `ConfigBaseRef` and a non-empty `GateBase` are supplied), and lets the core
layer own the user-facing/log surface. Config-first precedence is **unchanged** — this only adds a
signal. `plan-review` (architecture + Go personas) should confirm the field placement.

### Decision 2 — F4 and forced completions (the central nuance)
`latestGatePassEvidence` accepts both `EventGatePassed` **and** `EventGateForced`. A blanket
`ran==true` requirement would reject a legitimate operator **force** whose autoharness run did not
execute — but force is the deliberate, CLI-only, separately-audited break-glass
(`pre_task_completion_gate_forced`), and rejecting an audited force at the shipment level would
break its documented semantics.
**Chosen direction:** require `delta["ran"] == true` for an `EventGatePassed` event to count as
member gate evidence; keep `EventGateForced` acceptance **unconditional** (a forced completion is
an explicit, audited override and remains valid regardless of `ran`). This closes the fail-open
`ran=false` **pass** gap the review flagged without regressing force.
**Coupling to note for the plan:** `latestGatePassEvidence` is shared by BOTH the shipment
member-evidence scan **and** the advisory `doctor --check-gate-evidence`. Hardening it propagates
to the doctor check (desirable consistency), and the Q3 projection predicate (Decision 3) must
encode the **same** post-F4 semantics. This is a soft consistency constraint, not a hard build
dependency — the operator has confirmed the four remediations and the Q3 task are mutually
independent for scheduling. `plan-review` (security-lens + Go personas) should adjudicate whether
`EventGateForced` should also require `ran` (open question below).

### Decision 3 — Q3 subtask decomposition and ordering
The read-model exceeds the 2-hour single-task heuristics (multiple files across schema + sync +
doctor, more than four test scenarios). Decompose into three test-first subtasks:
1. **Schema** — add derived column(s)/index to `items` (or a dedicated `gate_evidence` table) in
   `EnsureSchema`; idempotent create.
2. **Projection population** — compute the projection from item logs during `Rehydrate`, using the
   post-F4 evidence predicate; assert rebuild idempotency (run twice → identical; logs
   authoritative).
3. **Doctor indexed query** — repoint the advisory `--check-gate-evidence` to the indexed column
   (with graceful behavior when the projection is absent). **Depends on subtasks 1 → 2.**
The column/projection lands **before** the doctor-query use (operator constraint). Scope guard:
subtask 3 is a thin repoint of the **existing advisory** check — it does NOT add strict-mode
enforcement (that remains a future item).

## Open questions for plan-review

1. **F4 forced-completion policy:** confirm `EventGateForced` should remain accepted regardless of
   `ran` (Decision 2), or whether an audited force with `ran=false` should also be rejected at the
   shipment level. (Security-lens + Go personas.)
2. **Q3 shape:** single derived column(s) on `items` (`last_gate_evidence_sha`, `gate_status`) vs a
   dedicated `gate_evidence` table. Column is simpler and matches the "disposable projection"
   framing; a table scales to multiple evidence rows per item. (SQLite + architecture personas.)
3. **F1 field placement:** `ResolvedBase.OverrideShadowed` vs a returned warnings slice, and where
   the core layer emits the user-facing warning. (Architecture + Go personas.)

## Decision

Proceed with **one covering feature** (`Pre-task-completion gate broker — phase-2 hardening`) over
all five items. Four mutually-independent single-file test-first remediation tasks (F1, F4, F5,
F7) plus one larger DB read-model task (Q3) decomposed into three ordered subtasks (schema →
projection → doctor query). Directions per finding as above; the two load-bearing decisions
(F4 force semantics, F1 advisory-only surface) are recorded for `plan-review` adjudication. Advance
to `impl-plan`.
