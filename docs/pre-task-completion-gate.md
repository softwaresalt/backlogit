---
chunk_strategy: h1-h2-h3
description: Operator and agent reference for the backlogit pre-task-completion gate broker, its autoharness composition, config, CLI/MCP surface, and runbooks
doc_type: guide
docline:
    author: backlogit contributors
    keywords:
        - backlogit
        - autoharness
        - gate
        - pre-task-completion
        - lifecycle_hooks
        - gate_blocked
    ms.date: 2026-07-06T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/pre-task-completion-gate.md
title: Pre-Task-Completion Gate Broker
---

## Overview

The pre-task-completion gate broker makes backlogit invoke `autoharness gate check`
before a completion transition is committed. It runs at two levels:

- **Task-level gate** — before a `task`/`subtask` moves into a terminal completion
  status (default `done`), backlogit runs a task-scoped `autoharness gate check` and
  commits the transition only on a pass.
- **Shipment-level gate** — before a shipment moves to `shipped`, backlogit first
  verifies that every member task/subtask carries recorded passing (or forced) gate
  evidence, then runs a full-shipment `autoharness gate check`. Both must pass.

The broker is an inline core completion service on `UpdateArtifact` and
`ShipShipment`, not a CLI wrapper, so every caller (CLI **and** MCP) is gated and
none can bypass it. Autoharness remains the single owner of the actual validation
gate list; backlogit never parses autoharness gate config and only invokes the
`autoharness gate check` CLI.

## Three-valued `enabled` semantics

The gate is configured by `enabled`, which is three-valued:

| Value   | Behavior |
|---------|----------|
| `auto`  | **Default.** Enforce when autoharness is resolvable and a base ref can be determined; otherwise fail **open** (completion proceeds). This preserves autoharness's own fail-open contract when no gates are configured. |
| `true`  | Strict / fail **closed**. If the autoharness binary is missing or the contract probe fails, the completion is refused with a setup error rather than allowed through. |
| `false` | Kill switch. The broker is never wired; completions behave exactly as they did before the gate feature. |

Under `auto`, a missing or incompatible autoharness binary means the gate cannot run,
so the transition is allowed (fail open). Under `true`, the same condition is a hard
refusal (fail closed). Use `true` in CI and release contexts where an un-runnable gate
must never be silently skipped.

## Composition with autoharness lifecycle hooks

Backlogit and autoharness compose without either owning the other's configuration:

- Autoharness owns the gate definitions via its own `lifecycle_hooks` in
  `.autoharness/config.yaml`. Backlogit does **not** read or interpret that file.
- Backlogit owns *when* to call the gate (the completion transitions above) and
  *how* to react to the result (commit, refuse, requeue, escalate, or record
  evidence).
- The invocation is always an **argv array** — `autoharness gate check --task <id>
  --base <ref> --head <ref> --workspace <root> --json` (shipment-level runs omit
  `--task` for a full-diff check and add `--no-count`). Backlogit never builds a
  shell command string and never interpolates untrusted paths into a shell.
- The base ref is the default-branch ref, resolved in order: config `base_ref` →
  `--gate-base` → `origin/HEAD` → `origin/main` → `main`. Autoharness applies the
  three-dot merge-base itself; backlogit passes the ref through and does not
  pre-compute the merge base.
- Minimum autoharness version is **1.4.7**; the broker runs a contract probe and,
  under `enabled: true`, refuses with a setup error when the probe fails.

Because autoharness's own gate is fail-open when no gates are configured, an
`enabled: auto` backlogit workspace layered on an autoharness workspace with no
gate section completes normally — the composed behavior stays fail-open end to end.

## Configuration

The gate is configured in `.backlogit/hooks.yaml` under
`lifecycle.pre_task_completion_gate`. An absent block normalizes to the defaults
shown below, so the feature is active (in `auto`) even without an explicit block.

```yaml
lifecycle:
  pre_task_completion_gate:
    enabled: auto                 # auto | true | false
    terminal_statuses: ["done"]   # statuses whose entry triggers the gate
    autoharness_binary: autoharness  # resolved via PATH; absolute/traversal paths rejected
    base_ref: auto                # auto | explicit default-branch ref
    timeout_seconds: 600          # 1..3600; lock heartbeat refreshes during the run
    force_cli_only: true          # hard invariant: force is operator-only via the CLI
    evidence_required: true       # gate-evidence append is part of the transition contract
```

Field notes:

- `terminal_statuses` — the completion statuses that trigger the gate. Default
  `["done"]`. Entry into a non-terminal status is never gated.
- `autoharness_binary` — resolved via `PATH` by default. An absolute path or a `..`
  traversal is rejected at config validation.
- `base_ref` — `auto` discovers the default-branch ref (see composition above). An
  explicit ref pins the base.
- `timeout_seconds` — bounds the gate run in `[1, 3600]`. The completion path
  refreshes the task-lock heartbeat during the run, so a timeout above the lock
  stale-TTL cannot let a concurrent process reap the live lock mid-gate.
- `force_cli_only` — a hard v1 invariant. Force is operator-only via the CLI; an
  explicit `false` is rejected. There is **no** MCP force field.
- `evidence_required` — when `true` (default), a failed gate-evidence append makes the
  transition fail rather than silently completing without an audit trail.

## CLI surface

`backlogit move` and `backlogit update` gain gate-related flags on completion:

| Flag | Purpose |
|------|---------|
| `--gate-base <ref>` | Operator-only base-ref override for the completion gate (audited). |
| `--force-gates` | Operator-only: force completion past a gate refusal. Requires `--force-reason`. |
| `--force-reason <text>` | Justification recorded in the forced-gate audit event. |
| `--json` | Emit the machine-readable gate-outcome contract (pass envelope or blocked report). |

`backlogit doctor` gains an advisory audit:

| Flag | Purpose |
|------|---------|
| `--check-gate-evidence` | Warn when a terminal task/subtask lacks passing/forced gate evidence while gates are configured. **Advisory only** — it never changes the doctor exit code. |

### Exit codes

On a completion path the gate maps outcomes to distinct exit codes:

| Exit code | Meaning |
|-----------|---------|
| `0` | Pass — transition committed and gate evidence recorded. |
| `6` | **Gate blocked** — the gate refused the completion. The artifact is unchanged and the gate report is returned. |
| `7` | Configuration / setup error — invalid gate config (autoharness exit 2), or, under `enabled: true`, a missing/incompatible autoharness binary (fail closed). |
| `8` | Retryable — lock contention or a gate timeout. Safe to retry after a short back-off. |

The `--force-gates` path (operator-only, with `--force-reason`) passes `--force` to
autoharness (which audits it) and records a `pre_task_completion_gate_forced` event in
backlogit before completing.

## MCP surface

MCP move/update tools invoke the identical broker with no force field (force is
CLI-only). On a gate refusal the tool returns `isError: true` with a structured
error so an agent can repair the failing files without re-running broad diagnostics.

### `gate_blocked` structured error

```json
{
  "error_type": "gate_blocked",
  "item_id": "042.003-T",
  "old_status": "active",
  "requested_status": "done",
  "outcome": "blocked",
  "base_ref": "origin/main",
  "head_ref": "HEAD",
  "gate_report": { "…": "full autoharness --json report" },
  "stderr_summary": "…"
}
```

`outcome` is one of `blocked`, `requeued`, or `escalated`, mirroring the
`repeated_failure` action autoharness reports (see Requeue behavior below).

### Related error classes

Non-block failures surface with their own `error_type` so callers can branch:

| `error_type`      | Meaning | Retryable |
|-------------------|---------|-----------|
| `gate_blocked`    | Gate refused the completion (exit 6). | No — fix the files. |
| `gate_setup`      | Autoharness binary missing/incompatible under fail-closed. | No — fix the environment. |
| `gate_config`     | Invalid gate configuration (autoharness exit 2). | No — fix config. |
| `gate_timeout`    | Gate run exceeded `timeout_seconds`. | Yes. |
| `gate_in_progress`| Lock contention — another completion is mid-gate. | Yes. |

Retryable classes carry a `retry_after_ms` hint. Machine callers receive the full
JSON gate report; human CLI output truncates `stderr` for readability while the
`--json` contract preserves the full report.

## Requeue behavior

When autoharness reports a `repeated_failure {count, threshold, reached, action}`
block, backlogit executes the requeue decision (it does not stack a second breaker;
shipment-level checks use `--no-count`):

| Condition | backlogit action |
|-----------|------------------|
| `reached` and `action: block` | Move the task back to `queued`. |
| `reached` and `action: escalate` | Move the task to `blocked`. |
| Below threshold, blocked | Refuse the completion; the task retains its re-read `old_status`. |
| Pass | Complete → `done` and record evidence. |

## Gate evidence

Every gated outcome appends an evidence event to the **item log only** (never the
artifact frontmatter): `gate_passed`, `gate_blocked`, `gate_requeued`,
`gate_escalated`, `gate_forced`, `gate_base_override`, or `gate_error`. Shipment-level
passes/refusals record a `level: shipment` evidence event. The `doctor
--check-gate-evidence` audit reads these events to surface terminal items that lack a
passing/forced record — advisory only.

## Operator runbooks

### (a) A gate blocked a completion

1. Read the returned gate report (`--json` on the CLI, or `gate_report` in the MCP
   error). It lists the failing autoharness gates.
2. Fix the flagged files and re-run the completion. No backlogit state changed on a
   block — the task is still in its prior status.
3. If autoharness reports `repeated_failure` with `reached`, the task was moved to
   `queued` (block) or `blocked` (escalate). Address the root cause, then move it back
   to `active` and retry.

### (b) Missing or incompatible autoharness binary

- Under `enabled: auto` (default): the gate cannot run, so completion proceeds
  (fail open). No action required unless you intend to enforce gates.
- Under `enabled: true`: completion is **refused** with a `gate_setup` error (exit 7).
  Install or upgrade autoharness to **≥ 1.4.7** (the contract probe requires
  `gate check --json` emitting `repeated_failure` and the `--no-count` flag), or set
  `enabled: auto`/`false` if enforcement is not wanted here.

### (c) Force override (operator-only)

Use only when you have justification and accept the recorded audit trail:

```sh
backlogit move 042.003-T --status done --force-gates --force-reason "hotfix: gate infra outage, verified manually"
```

- `--force-gates` requires `--force-reason`.
- backlogit records a `pre_task_completion_gate_forced` event and passes `--force`
  to autoharness (which also audits it).
- There is **no** MCP equivalent — force is CLI-only by design (`force_cli_only`).

### (d) Kill switch (`enabled: false`)

To disable the gate entirely (e.g. during a broad autoharness outage), set:

```yaml
lifecycle:
  pre_task_completion_gate:
    enabled: false
```

The broker is never wired and completions behave exactly as before the gate feature.
Re-enable by restoring `auto` or `true`. Prefer this over uninstalling autoharness so
the change is explicit and reversible.

## Security posture

- **argv-array execution only** — the gate is never invoked through a shell string,
  and untrusted paths are never interpolated into a shell.
- **No frontmatter mutation for evidence** — gate evidence lives in the item log.
- **Truncated stderr for humans, full JSON for machines** — human CLI output
  truncates `stderr`; the `--json` contract and MCP errors preserve the full report.
- **Base override is audited** — `--gate-base` records a `gate_base_override` event.
- **Force is operator-only and audited** — no MCP force path exists.
