---
chunk_strategy: h1-h2-h3
description: "Design for making backlogit the authoritative pre-task-completion gate broker: blocking task done transitions until autoharness validation gates pass, returning structured stderr feedback to agents, and preserving operator-only force bypass semantics."
doc_type: design
docline:
    date: 2026-07-04T00:00:00Z
    status: proposed
    tags:
        - lifecycle-hooks
        - task-completion
        - validation-gates
        - autoharness
        - mcp
        - cli
schema_version: "1.0"
source: docs/design-docs/2026-07-04-pre-task-completion-gate-broker.md
title: "Pre-Task-Completion Gate Broker"
---

# Pre-Task-Completion Gate Broker

## Overview

This design makes backlogit the live enforcement point for deterministic
autoharness validation gates. The enforcement point is the backlog state
transition that marks a task complete:

```text
task/subtask status: active|queued|review -> done
```

Before backlogit writes that transition, it synchronously invokes the
autoharness gate engine:

```text
autoharness gate check --task <item-id> --base <base-ref> --head HEAD --workspace <workspace> --json
```

If the command exits 0, backlogit completes the transition and records gate
evidence. If it exits non-zero, backlogit refuses the transition, leaves the
artifact unchanged, and returns the structured gate failure payload to the
caller. This gives every agent that uses backlogit CLI or MCP a real
task-completion hook without depending on prompt compliance.

## Problem

autoharness already ships a deterministic gate engine:

- `.autoharness/config.yaml` can define
  `lifecycle_hooks.pre_task_completion.validation_gates`.
- `autoharness gate check` discovers changed files, matches gate globs, runs
  configured commands, captures stderr, and exits non-zero on blocking failure.
- Gate enforcement supports `absolute`, `advisory`, repeated-failure behavior,
  correction reports, and operator-only force bypass.

What is missing is the unavoidable live hook. Today an agent can still mark a
backlog task done without calling `autoharness gate check` first if the
completion path does not explicitly remember to call it.

Backlogit is the correct enforcement boundary because backlogit owns the
durable task lifecycle state. All supported agents already rely on backlogit to
transition tasks through `queued`, `active`, `done`, `blocked`, and related
states.

## Goals

1. Block task/subtask completion when configured autoharness validation gates
   fail.
2. Preserve atomic task completion: either all relevant gates pass and the task
   moves to `done`, or the task remains non-done.
3. Return gate stderr and structured correction data to CLI and MCP callers so
   agents can self-heal.
4. Make force bypass operator-only and audited.
5. Keep backlogit responsible for work-item state only; autoharness remains the
   authority for code/docs validation gate execution.
6. Preserve fail-open compatibility when no autoharness lifecycle hooks are
   configured.

## Non-goals

- Backlogit will not parse `.autoharness/config.yaml` validation gates directly.
- Backlogit will not execute arbitrary gate commands from autoharness config.
- Backlogit will not implement git-diff discovery, glob matching, or per-file
  subprocess execution. Those remain in `autoharness gate check`.
- Backlogit will not provide an agent-accessible gate bypass over MCP in the
  initial design.
- This design does not make direct file edits to `.backlogit/*.md` impossible.
  It adds supported-path enforcement and a verification path for bypass
  detection.

## Current backlogit extension points

The implementation should center on existing state mutation paths:

- `internal/core.UpdateArtifact(...)` is the common artifact update path used by
  CLI `move` / `update` and by MCP update/move tools.
- `internal/cli/move.go` calls `core.UpdateArtifact(..., {"status": status})`.
- `internal/cli/update.go` calls `core.UpdateArtifact` for frontmatter field
  updates, including `--status`.
- `internal/core/archive.go` already demonstrates pre/post hook execution with
  rollback discipline before file writes.
- `internal/hooks/` already contains built-in pre/post hook plumbing and durable
  hook event support.
- `.backlogit/hooks.yaml` already contains lifecycle configuration for built-in
  transition validation and event emission.

The gate broker should be implemented as a built-in pre-transition hook in the
core layer, not as only a CLI wrapper. Otherwise MCP and future callers could
bypass it.

## Proposed architecture

```text
CLI / MCP caller
  -> core.UpdateArtifact(status=done)
     -> detect status transition
     -> task-completion gate broker
        -> acquire task lock
        -> build gate invocation context
        -> run autoharness gate check --json
        -> pass: write status=done + gate evidence
        -> fail: return GateBlockedError; no artifact write
```

### Enforcement trigger

Run the gate broker only when all of these are true:

1. Artifact type is `task` or `subtask`.
2. The update includes a status change.
3. New status is a configured terminal completion status, default `done`.
4. Previous status is not already that terminal status.
5. Gate broker is enabled by backlogit hook configuration.

The initial default should be conservative:

```yaml
lifecycle:
  pre_task_completion_gate:
    enabled: auto
    terminal_statuses: ["done"]
```

`enabled: auto` means:

- If `.autoharness/config.yaml` is absent, or contains no lifecycle hooks,
  backlogit does not block completion.
- If `.autoharness/config.yaml` has pre-task-completion validation gates,
  backlogit invokes `autoharness gate check`.

This preserves the existing autoharness fail-open contract while enabling live
enforcement when gates are configured.

### Gate command ownership

Backlogit should invoke only the autoharness gate CLI, not arbitrary configured
gate commands. Autoharness remains responsible for parsing and executing the
actual validation gate list.

Default command:

```yaml
lifecycle:
  pre_task_completion_gate:
    command:
      argv:
        - autoharness
        - gate
        - check
        - --task
        - "{item_id}"
        - --base
        - "{base_ref}"
        - --head
        - "{head_ref}"
        - --workspace
        - "{workspace_root}"
        - --json
```

Use argv-array execution only. Do not concatenate a shell command string.

Recommended config fields:

```yaml
lifecycle:
  pre_task_completion_gate:
    enabled: auto                 # auto | true | false
    terminal_statuses: ["done"]
    autoharness_binary: autoharness
    base_ref: auto                # auto | explicit git ref
    head_ref: HEAD
    timeout_seconds: 600
    force_cli_only: true
    evidence_required: true
```

### Base-ref resolution

`autoharness gate check` requires a `--base` ref. Backlogit should resolve it in
this order:

1. Explicit `pre_task_completion_gate.base_ref` when not `auto`.
2. CLI flag for operator flows, for example `--gate-base <ref>`.
3. `origin/HEAD` symbolic default branch when available.
4. `origin/main`.
5. `main`.

If base-ref resolution fails while gates are configured, return a gate
configuration error and refuse completion. This is safer than silently using an
empty diff.

### Task lock

Use the existing task locking primitives before running the external gate:

1. Acquire task lock for `<item-id>`.
2. Re-read artifact state after lock acquisition.
3. If it no longer needs completion, no-op or return current state.
4. Run the gate.
5. Write transition only on pass.
6. Release lock.

This prevents two clients from racing one completion attempt while validation is
in progress.

## Error and response model

Introduce a typed core error, for example:

```go
type GateBlockedError struct {
    ItemID     string
    BaseRef    string
    HeadRef    string
    ExitCode   int
    ReportJSON json.RawMessage
    Stderr     string
}
```

CLI behavior:

- Human output: concise failure summary and "task remains <old status>".
- `--json` output: include the full gate report under `gate_report`.
- Exit code: non-zero distinct code, recommended `6` (`gate blocked`).

MCP behavior:

- Return `isError: true`.
- Include structured fields in the tool error content:
  - `error_type: "gate_blocked"`
  - `item_id`
  - `old_status`
  - `requested_status`
  - `base_ref`
  - `head_ref`
  - `gate_report`
  - `stderr_summary`

Agents should receive enough information to repair the failing files without
rerunning broad diagnostics.

## Gate evidence

On successful gate pass, backlogit should record evidence before or atomically
with the completion write:

```json
{
  "event_type": "pre_task_completion_gate_passed",
  "item_id": "060.004-T",
  "base_ref": "origin/main",
  "head_ref": "HEAD",
  "head_sha": "<resolved sha>",
  "autoharness_config_hash": "<sha256 or omitted when unavailable>",
  "gate_report_hash": "<sha256>",
  "timestamp": "<RFC3339>"
}
```

On blocked failure, record:

```json
{
  "event_type": "pre_task_completion_gate_blocked",
  "item_id": "060.004-T",
  "base_ref": "origin/main",
  "head_ref": "HEAD",
  "exit_code": 1,
  "stderr_summary": "...",
  "timestamp": "<RFC3339>"
}
```

Evidence should be written to existing item logs and hook event queue surfaces
so Stage/Ship and remote operators can observe the gate result.

## Force override

Force override is operator-only.

Initial design:

- CLI may expose `--force-gates --force-reason "<reason>"`.
- MCP tools must not expose a force-gates option by default.
- Force requires a non-empty reason.
- Force records a `pre_task_completion_gate_forced` event with operator-facing
  audit data.
- Force should pass `--force` to `autoharness gate check` so autoharness writes
  its own gate-force audit log too.

Agents cannot self-bypass because normal MCP completion tools have no force
field.

## Partial completion policy

No partial task completion.

If only some files pass, backlogit refuses the status transition and returns the
per-file failure report. The task remains active. If the work is genuinely
partial, the correct workflow is to split the task into smaller tasks/subtasks
or move the task to `blocked` with explicit follow-up context.

Rationale:

- Backlog tasks are atomic release-unit milestones.
- Partial completion creates ambiguous durable state.
- Autoharness gates already produce per-file feedback, so the agent can repair
  the failing subset and retry the same task completion.

## Bypass detection

Supported completion paths enforce the gate, but direct file edits can still
bypass backlogit. Add a verification check:

```text
backlogit doctor --check-gate-evidence
```

or include this in existing doctor output when enabled:

- Find task/subtask artifacts with terminal status.
- Check item logs for matching `pre_task_completion_gate_passed` or
  `pre_task_completion_gate_forced` evidence at the same or later head SHA.
- Warn when a terminal task lacks gate evidence while gates were configured.

This should be advisory at first to avoid flagging legacy artifacts. A later
release can make it strict for artifacts updated after the feature ships.

## Configuration examples

### Auto mode

```yaml
lifecycle:
  pre_task_completion_gate:
    enabled: auto
    terminal_statuses: ["done"]
    autoharness_binary: autoharness
    base_ref: auto
    timeout_seconds: 600
    force_cli_only: true
```

### Disabled

```yaml
lifecycle:
  pre_task_completion_gate:
    enabled: false
```

### Explicit base ref

```yaml
lifecycle:
  pre_task_completion_gate:
    enabled: true
    base_ref: origin/main
```

## Implementation plan

### Unit 1 - Config and command context

- Extend hooks config model and defaults.
- Add parsing/validation for `pre_task_completion_gate`.
- Add base-ref resolver.
- Add tests for `auto`, `true`, `false`, explicit base, missing binary, and
  invalid status.

### Unit 2 - Core transition broker

- Add core pre-transition hook in `UpdateArtifact`.
- Detect task/subtask transition to `done`.
- Acquire task lock and re-read state.
- Run gate through an interface (`GateRunner`) for deterministic tests.
- Verify failure leaves the artifact file and DB unchanged.

### Unit 3 - CLI/MCP response mapping

- Map `GateBlockedError` to a stable CLI exit code and JSON payload.
- Map the same error to MCP structured error content.
- Add operator-only CLI force path and audit event.
- Do not add MCP force support in this unit.

### Unit 4 - Evidence and doctor check

- Append pass/block/force evidence to item logs.
- Emit durable hook events for subscribers.
- Add doctor evidence check in advisory mode for new artifacts.

### Unit 5 - Documentation

- Update CLI docs, MCP docs, and generated command/reference docs.
- Document how this composes with autoharness `lifecycle_hooks`.
- Include operator runbooks for gate failure, missing autoharness binary, and
  force override.

## Test matrix

| Scenario | Expected result |
|---|---|
| No `.autoharness/config.yaml` | Task can move to `done`; no gate run |
| Empty/null `lifecycle_hooks` | Task can move to `done`; no gate run |
| Configured gates, all pass | Task moves to `done`; pass evidence logged |
| Configured gates, one fails | Transition refused; task remains old status; failure report returned |
| Advisory autoharness gate result | Autoharness exits 0; backlogit completes transition |
| Missing autoharness binary with gates configured | Transition refused as configuration error |
| Gate timeout | Transition refused; timeout report returned |
| Operator CLI force with reason | Task moves to `done`; forced evidence logged |
| MCP caller attempts force | No force field supported; normal gate behavior |
| Direct file edit to `done` | Doctor warns missing gate evidence |
| Concurrent completion attempts | One holds lock; the other waits/fails consistently |

## Security considerations

- Backlogit must use argv-array execution when invoking autoharness.
- Backlogit must not run arbitrary shell commands from config.
- Backlogit must not interpolate untrusted paths into shell strings.
- Force bypass must be CLI-only and audited.
- Gate failure output may include stderr from external tools; truncate human
  output while preserving full JSON report for machine callers.

## Open questions

> **Resolved 2026-07-05.** All four questions were answered in operator
> deliberation, together with three adjacent decisions and a coordination review
> of the autoharness `gate check` CLI. See
> "Deliberation outcomes and autoharness coordination (2026-07-05)" below for the
> authoritative decisions; the questions are retained here for provenance.

1. Should `pre_task_completion_gate.enabled: auto` be the default when hooks.yaml
   is absent, or should backlogit require explicit hooks.yaml opt-in?
2. Should doctor evidence checking be opt-in initially to avoid noise on legacy
   repositories?
3. Should backlogit cache the autoharness config hash in item frontmatter, item
   logs only, or both?
4. Should the MCP surface eventually support a human-approved force token, or
   should force remain permanently CLI-only?

## Recommended default decisions

- Default to `enabled: auto`.
- Keep doctor evidence checking advisory in the first release.
- Store evidence in item logs, not frontmatter.
- Keep force CLI-only until there is a robust remote-operator approval token
  model.

## Deliberation outcomes and autoharness coordination (2026-07-05)

This section records the decisions from operator deliberation on 2026-07-05 and
the coordination findings from reviewing the autoharness `gate check` CLI
(`src/autoharness/cli.py` plus `src/autoharness/gates/`, documented in the
autoharness `docs/gates-reference.md`). Where a decision here refines an earlier
section, the decision here is authoritative.

### Resolved open questions

1. Default enablement: default to `enabled: auto`. Legacy compatibility is not a
   concern (no other consumer currently uses autoharness), and `auto` stays
   fail-open when autoharness is absent (see three-valued `enabled` below).
2. Doctor evidence checking: advisory only in the first release; promote to a
   stricter check in a later release once gate evidence is widespread.
3. Evidence storage: item logs only, not frontmatter, to keep completion writes
   free of merge-conflict churn. A follow-up feature derives an indexed read-model
   column from the logs so agents can query gate evidence without scanning logs.
4. Force bypass surface: force stays CLI-only. There is no agent-self-serviceable
   MCP bypass. Revisit only with a robust remote-operator approval-token model.

### Autoharness gate CLI contract (as implemented, Phase 1)

```text
autoharness gate check --base <ref> [--task <id>] [--head HEAD] [--workspace .] [--json] [--force]
```

- Exit codes: `0` pass, no gates, no files matched, or advisory-only; `1` blocked
  (a matched file failed its gate); `2` invalid arguments or invalid gate
  configuration.
- File discovery uses `git diff --name-only <base>...<head>` (three-dot,
  merge-base). Discovery degrades to an empty list on git or ref errors and never
  crashes.
- Autoharness owns the repeated-failure counter and requeue or escalate behavior
  on the `max_gate_failures` (default third) consecutive failure
  (`.autoharness/gates/gate-state.json`); the `--force` audit log
  (`.autoharness/gates/gate-force-audit.log`); the advisory-versus-absolute
  policy; and parsing of `.autoharness/config.yaml`. Autoharness is fail-open when
  no `lifecycle_hooks` block is configured.

**Minimum autoharness version.** The broker requires autoharness `>= 1.4.7`, the
release that ships the structured `repeated_failure` object in the
`gate check --json` report and the `--no-count` advisory-run flag. The broker's
version or contract probe treats an autoharness older than 1.4.7 (missing the
`repeated_failure` field) as a configuration error under `enabled: true`.

### Base-ref resolution (refines "Base-ref resolution")

Autoharness applies the three-dot `<base>...<head>` merge-base internally, so the
broker passes `--base` as the resolved default-branch ref and lets autoharness
compute the merge-base. The broker must not pre-compute `git merge-base` and pass
the result. Resolution order for the default-branch ref: explicit config
`base_ref` when not `auto`, then CLI `--gate-base`, then `origin/HEAD`, then
`origin/main`, then `main`. If the ref cannot resolve while gates are enforced,
refuse completion with a configuration error.

### Exit-code and failure-class mapping (refines "Error and response model")

The broker distinguishes three non-success classes:

| Condition | Class | Broker behavior |
|---|---|---|
| `autoharness` binary not found (OS exec error) | setup error | Under `enabled: true`, fail-closed: refuse with an actionable "gate enforcement configured but autoharness not found" error. Under `enabled: auto`, fail-open (do not enforce). |
| Exit `2` (invalid args or invalid gate config) | configuration error | Refuse completion and surface the configuration error. |
| Exit `1` (blocked) | gate failure | Refuse; return `GateBlockedError` with the report. Backlogit CLI exit code `6`. |
| Exit `0` | pass | Complete the transition and record gate evidence. Treat exit `0` as pass even when stdout is not JSON, because the no-gates path prints a human message. |

### Three-valued `enabled` (refines "Enforcement trigger"; resolves missing-binary policy)

- `auto` (default): enforce when the `autoharness` binary is resolvable; when it
  is absent, fail-open. This keeps backlogit usable in agent environments that do
  not use autoharness. Gate content failures still fail-closed.
- `true`: strict. A missing or incompatible autoharness binary fails-closed and
  refuses completion. This is the dark-factory setting.
- `false`: disabled. The sanctioned opt-out.

The broker never parses autoharness gate config; it relies on autoharness
returning exit `0` when no gates are configured. The broker probes the autoharness
version or contract, not just binary existence, and treats an incompatible
autoharness as a configuration error under `enabled: true`. The two config
namespaces stay separate: backlogit's `lifecycle.pre_task_completion_gate` governs
broker behavior, while autoharness's `.autoharness/config.yaml`
`lifecycle_hooks.pre_task_completion.validation_gates` defines the gates.

### Lock and contention (refines "Task lock" and "Partial completion policy")

- No partial completion: any failing matched file refuses the whole transition and
  the task remains non-`done`.
- The broker holds the task lock through the entire gate execution.
- Contention behavior is bounded-wait then fail-fast. A second caller waits a short
  grace cap (a few seconds), then returns a distinct retryable error ("gate in
  progress for `<item>`, retry shortly"). Consistency is guaranteed by the lock
  plus re-read-after-acquire regardless of contention behavior; bounded-wait then
  fail-fast is chosen for dark-factory liveness so a long gate does not trip stall
  or circuit-breaker timers.

### Two-level gating: task and shipment

Gating applies at two backlog transitions.

1. Task or subtask to `done`: run the task-oriented
   `autoharness gate check --task <id> --base <default-branch> --head HEAD` and
   record gate evidence in the item log on pass.
2. Shipment to `shipped` (`ship_shipment`): both aggregate the recorded member-task
   gate evidence (every member task must carry passing gate evidence at or after
   its final head SHA) and run a shipment-level `autoharness gate check` over the
   full shipment diff (base is the shipment's merge target, head is `HEAD`). The
   shipment completes only when the aggregate evidence check and the shipment-level
   gate check both pass.

### Circuit-breaker reconciliation

Autoharness already implements a repeated-failure breaker (`max_gate_failures`,
default 3). The broker must not stack a second independent breaker on the same
signal. The broker honors autoharness's breaker outcome rather than counting gate
failures separately.

### Open coordination item — repeated-failure requeue ownership

Autoharness counts consecutive failures and, on the threshold failure, marks the
task for requeue or escalate, but backlogit owns the durable task state. This is
resolved in
`docs/decisions/2026-07-05-gate-repeated-failure-requeue-ownership-deliberation.md`
(decided 2026-07-05): backlogit is the sole executor of the transition, driven by
a structured `repeated_failure` signal that autoharness surfaces in its
`gate check --json` report. Backlogit maps threshold-reached with action `block`
to a move to `queued`, and action `escalate` to a move to `blocked`; a
below-threshold blocked result keeps the task `active`. The dependency on the
autoharness `repeated_failure` field is **satisfied in autoharness 1.4.7**
(verified: `gate check --json` emits
`repeated_failure: {count, threshold, reached, action}`, and a `--no-count`
advisory-run flag was added). The broker consumes this field directly.
