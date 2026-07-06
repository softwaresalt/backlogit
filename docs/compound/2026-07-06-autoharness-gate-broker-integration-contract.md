---
chunk_strategy: h1-h2-h3
description: 'The durable backlogit <-> autoharness gate-broker integration contract graduated from 082-S. Records the exact seam: min autoharness >= 1.4.7 with a contract probe; `gate check --json` emits repeated_failure {count, threshold, reached, action}; exit trichotomy (0 pass / 1 blocked / 2 config-error) plus binary-not-found and timeout; base-ref = default-branch ref with autoharness applying the three-dot merge-base (do NOT pre-compute); three-valued enabled (auto/true/false); never parse autoharness gate config; requeue/escalate driven by repeated_failure with backlogit as sole executor; injectable runner interface so unit tests mock the gate without a real binary.'
doc_type: learning
docline:
    date: 2026-07-06T00:00:00Z
    severity: high
    tags:
        - integration-contract
        - autoharness
        - gate
        - cli
        - mcp
        - core
        - exit-codes
        - dependency-injection
        - testing
schema_version: "1.0"
source: docs/compound/2026-07-06-autoharness-gate-broker-integration-contract.md
title: 'backlogit <-> autoharness pre-task-completion gate integration contract (082-S)'
---

# Autoharness Gate-Broker Integration Contract

The durable contract graduated from shipment 082-S (feature 082-F,
"Pre-Task-Completion Gate Broker", PR #178, merge
`e47e1291c49f906a4b257c60f117a2cd05107db7`). This is the reference for anyone
touching the `internal/core/gate/*` seam or bumping the autoharness dependency.

## What the gate does

backlogit synchronously invokes `autoharness gate check` **before** writing:

- a task/subtask → `done`, and
- a shipment → `shipped`.

This is a **one-way `core → gate` boundary** implemented as an inline core
completion service. `internal/core/gate/*` imports only stdlib +
`internal/errors` (no `internal/core` import → no cycle).

## Two-level gating

1. **Task-oriented gate** — a `gate check` scoped to the completing task/subtask.
2. **Shipment gate** — an **aggregate of member-task evidence** *plus* a
   full-shipment `gate check`. The aggregate scan and the fresh check are both
   required; a member that never ran the gate (`ran=false`) must not silently
   pass as fail-open at the shipment level.

## Version contract

- **Minimum autoharness: `>= 1.4.7`.** Enforced by a **contract probe** at runtime,
  not just a documented assumption.
- `1.4.7` is the first version whose `gate check --json` emits the
  `repeated_failure { count, threshold, reached, action }` object and supports the
  `--no-count` flag. Both are load-bearing for the requeue mapping below.

## Exit-code trichotomy (+ two out-of-band conditions)

| autoharness result | Meaning | backlogit behavior |
|---|---|---|
| exit `0` | pass | complete transition + record evidence |
| exit `1` | gate blocked | `GateBlockedError` → **backlogit exit code 6**; refuse the transition and return the gate report |
| exit `2` | config error | typed config error; refuse |
| binary not found | setup error | **fail-closed** when `enabled: true`; fail-open when `enabled: auto` |
| timeout | retryable | kill child, return retryable error (see the timeout-before-probe lesson) |

Map on the **task path** to backlogit exit codes 6/7/8 via the typed
`gateExitError`.

## Three-valued `enabled` (auto / true / false)

- `auto` — run the gate only when autoharness is detected; **fail-open** if absent.
- `true` — gate required; **fail-closed** (missing binary = setup error, refuse).
- `false` — gate disabled; the broker is not even wired
  (`gateCfg.Enabled != "false"` guards construction).

## Base-ref: let autoharness do the merge-base

- Base-ref resolution order: config `base_ref` → `--gate-base` flag →
  `origin/HEAD` → `origin/main` → `main`.
- backlogit passes the **default-branch ref**; **autoharness applies the
  three-dot merge-base** internally. Do **NOT** pre-compute the merge-base in
  backlogit — passing a pre-resolved SHA would double-apply the semantics.
- A base override (`--gate-base` / non-default config) is **audited**.

## Requeue / escalate mapping (backlogit is the sole executor)

Driven entirely by the `repeated_failure` object autoharness returns; backlogit
executes the state change, autoharness never mutates backlog state:

| Condition | Action |
|---|---|
| `reached: true` + action `block` | move task → `queued` |
| `reached: true` + action `escalate` | move task → `blocked` |
| below-threshold `block` | refuse; **retain** the re-read `old_status` |
| pass | move task → `done` |

Do **not** stack a second backlogit-side breaker on top of autoharness's counter —
one source of truth for the repeated-failure count.

## Config isolation

**Never parse autoharness's own gate config.** backlogit owns only its
`lifecycle.pre_task_completion_gate` block (`enabled`, `autoharness_binary`,
`base_ref`, `timeout_seconds`, `force_cli_only`, `evidence_required`). Autoharness
owns its thresholds/rules. Crossing that line couples two release cadences.

## Force (operator-only, audited)

- CLI-only: `--force-gates --force-reason "<why>"`. Passes `--force` to autoharness
  (which audits it) **and** emits a backlogit `pre_task_completion_gate_forced`
  event.
- `force_cli_only: false` is **rejected at validation** — no config-level force.
- **No MCP force field.** The MCP move/update tools call
  `UpdateArtifactWithGate(..., TransitionOptions{})` with no force/gate_base in
  their schemas. Forcing is a deliberate human-at-a-terminal action.

## Evidence

- **Item logs only** (`WorkspaceLogsRoot` JSONL), never frontmatter.
- Pass and redirect paths honor `evidence_required` and refuse on append failure.
- Doctor `--check-gate-evidence` is **advisory-only** (never blocks).

## Testability: inject the runner

The single most important design choice for maintainability: **command execution
is injected** via a runner interface/func (the same `run_fn` pattern autoharness
itself uses). Unit tests mock the gate invocation — asserting exit-code mapping,
fail-open/closed, requeue behavior, and timeout — **without a real autoharness
binary**. Integration tests may exercise the installed autoharness (verified with
1.4.7). Keep the seam injectable; do not reach for `exec` directly inside core
logic.

## Related

- `docs/design-docs/2026-07-04-pre-task-completion-gate-broker.md` — full locked
  design and deliberation outcomes.
- `docs/decisions/2026-07-05-gate-repeated-failure-requeue-ownership-deliberation.md`
  — why backlogit (not autoharness) executes the requeue.
- `docs/compound/2026-07-06-exec-binary-config-must-be-bare-path-validated.md` and
  `docs/compound/2026-07-06-external-process-timeout-before-probe.md` — the two
  security/reliability lessons on this exec seam.
