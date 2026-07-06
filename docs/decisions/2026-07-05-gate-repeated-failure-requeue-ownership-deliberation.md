---
chunk_strategy: h1-h2-h3
description: 'Deliberation for the pre-task-completion gate broker (feature D23DFA0B): decide which component performs the backlog requeue or escalate transition when the autoharness repeated-failure counter reaches max_gate_failures. Autoharness owns the failure counter and policy; backlogit owns durable task state. Recommends backlogit as the sole executor driven by an autoharness-surfaced signal.'
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-07-05-gate-repeated-failure-requeue-ownership-deliberation.md
title: 'Gate repeated-failure requeue ownership: who performs the backlog transition'
stash_id: D23DFA0B
decision_status: decided
tags:
  - gate-broker
  - autoharness
  - task-lifecycle
  - circuit-breaker
---

## Question

When the autoharness gate engine reaches `max_gate_failures` (default 3)
consecutive blocking failures for a task, which component performs the resulting
backlog state transition — the requeue on `on_repeated_failure: block`, or the
escalate on `on_repeated_failure: escalate`?

## Context

The coordination review of the autoharness `gate check` CLI
(`src/autoharness/cli.py`, `src/autoharness/gates/`) established these facts:

- Autoharness owns the consecutive-failure counter in
  `.autoharness/gates/gate-state.json`, resets it on a passing check, and on the
  threshold failure writes a circuit-breaker checkpoint to
  `docs/memory/{date}/circuit-break-gate-{task}.md`.
- Autoharness owns the `on_repeated_failure` policy: `block` (requeue) or
  `escalate`.
- The autoharness gate subsystem is deliberately isolated — it must not import
  the rest of the harness and is invoked as a standalone CLI. It returns exit
  codes `0`, `1`, `2` and a per-file correction report.
- Backlogit is the durable authority for task lifecycle state
  (`queued`, `active`, `done`, `blocked`). The broker is backlogit calling
  `gate check` synchronously before a `-> done` transition.

The tension: autoharness decides the repeated-failure *policy*, but only backlogit
can legitimately change *task state*. The design doc did not specify who executes
the requeue.

## Options

### Option 1 — Backlogit executes, driven by an autoharness signal (recommended)

Autoharness surfaces the repeated-failure outcome in the `gate check --json`
report (threshold reached, current count, and the recommended action derived from
`on_repeated_failure`). Backlogit reads it and performs the transition:
`block` -> move task to `queued`; `escalate` -> move task to `blocked`. Autoharness
still writes its own counter state and checkpoint.

- Pro: backlogit remains the single writer of backlog state (matches the
  constitution's task-lifecycle authority). Autoharness stays policy authority.
- Pro: no reentrancy (autoharness never calls back into backlogit).
- Pro: preserves autoharness subsystem isolation.
- Con: requires autoharness to expose the signal in structured JSON rather than
  only in the human checkpoint file — a coordination dependency.

### Option 2 — Autoharness executes by shelling out to backlogit

On threshold, `gate check` runs `backlogit move <task> --status queued|blocked`.

- Pro: the policy owner performs the action directly.
- Con: couples the isolated gate subsystem to the backlogit CLI, violating its
  no-import isolation intent.
- Con: reentrancy risk (backlogit -> gate check -> backlogit) and split state
  ownership; harder to reason about and test.

### Option 3 — Advisory only; no automatic requeue

Autoharness tracks the counter and writes the checkpoint; backlogit keeps
refusing the `-> done` transition (task stays `active`). No state move happens.

- Pro: simplest; fewest moving parts.
- Con: the documented requeue/escalate behavior becomes a no-op on backlog state;
  a stuck task silently stays `active` with no queue signal, which is poor for a
  dark-factory (no-HITL) flow where a clear `blocked`/`queued` signal is how the
  system self-routes and surfaces attention.

### Option 4 — Split by policy (a refinement of Option 1)

Same executor as Option 1 (backlogit), but explicitly map the two policies:
`block` -> `queued` (retryable by a later pass), `escalate` -> `blocked` (surfaces
for operator or a different agent). This is Option 1 with the state mapping made
explicit and is the concrete form recommended.

## Recommendation

Adopt **Option 1, in the Option 4 concrete form**: backlogit is the sole executor
of the backlog transition, driven by a structured repeated-failure signal that
autoharness surfaces in the `gate check --json` report.

Mapping:

| Autoharness outcome | Backlogit action | Evidence |
|---|---|---|
| Below threshold, blocked | Refuse `-> done`; task stays `active` | Log the gate failure |
| Threshold reached, `on_repeated_failure: block` | Move task to `queued` | Log gate-requeue event; reference the autoharness checkpoint |
| Threshold reached, `on_repeated_failure: escalate` | Move task to `blocked` | Log gate-escalation event; reference the autoharness checkpoint |
| Pass | Complete `-> done` | Record `pre_task_completion_gate_passed` |

Rationale: this keeps backlogit the single writer of durable task state (its
constitutional role), keeps autoharness the policy and counter authority, avoids
reentrancy, and preserves the gate subsystem's isolation. It also gives the
dark-factory flow the explicit `queued`/`blocked` routing signal that Option 3
would drop.

## Circuit-breaker reconciliation

The autoharness gate breaker (`max_gate_failures`, default 3) is the authoritative
breaker for *gate content* failures. Backlogit must not add a second independent
breaker on the same signal. Backlogit's own breakers (build-feature loop 5,
universal 3) govern different operations and remain separate. When backlogit moves
a task to `queued` or `blocked` on the gate breaker, that is honoring autoharness's
outcome, not counting failures itself.

## Coordination requirement for autoharness

This decision depends on one autoharness-side capability:

- `autoharness gate check --json` must expose the repeated-failure state as a
  structured field, for example
  `{"repeated_failure": {"count": 3, "threshold": 3, "action": "block"}}`, so
  backlogit acts deterministically rather than parsing the human checkpoint file.

If that field is not yet emitted, it is the single coordination ask to the
autoharness effort before the broker's requeue path can be implemented. A
secondary hardening note: the autoharness counter is keyed per task, so backlogit
should be the authoritative caller in the completion path to avoid a manual
pre-check inflating the counter; if manual pre-checks are expected, the counter
should distinguish completion-path runs from advisory runs.

## Consequences

- Backlogit gains a gate-driven requeue/escalate path in the transition broker
  (Unit 2 of the implementation plan), consuming the autoharness signal.
- Autoharness gains (or confirms) a structured `repeated_failure` field in its
  `--json` report.
- Evidence for requeue and escalate events is recorded in item logs alongside the
  pass/fail evidence (consistent with the Q3 logs-only decision).

## Status

Decided (operator-confirmed 2026-07-05). Backlogit is the sole executor of the
requeue or escalate transition, driven by autoharness's structured
`repeated_failure` signal. The autoharness coordination ask (a structured
`repeated_failure` field in the `gate check --json` report) is filed in the
autoharness workspace backlog. Fold the mapping into the gate-broker design doc's
transition-broker unit at harvest.
