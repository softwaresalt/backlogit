---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for the Pre-Task-Completion Gate Broker: backlogit synchronously invokes autoharness gate check before task/subtask -> done and shipment -> shipped transitions, with argv-array exec, exit-code trichotomy, three-valued enablement, autoharness-driven requeue/escalate, operator-only force, and logs-only gate evidence.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-07-06-pre-task-completion-gate-broker-plan.md
title: 'Pre-Task-Completion Gate Broker implementation plan'
---

# Pre-Task-Completion Gate Broker implementation plan

Source documents (authoritative):

- `docs/design-docs/2026-07-04-pre-task-completion-gate-broker.md` — design and the
  authoritative "Deliberation outcomes and autoharness coordination (2026-07-05)"
  section.
- `docs/decisions/2026-07-05-gate-repeated-failure-requeue-ownership-deliberation.md` —
  decided requeue/escalate ownership.
- Stash intake: `D23DFA0B` (feature, medium).

All decisions in those documents are final. This plan translates the design's
five implementation units into repo-grounded, 2-hour-sized work with explicit
file targets, test surfaces, dependency edges, and hardening signals.

## Problem Frame

Backlogit owns durable task-lifecycle state. Today an agent can move a
`task`/`subtask` to `done` (or a shipment to `shipped`) without any deterministic
validation, because the completion path in `internal/core.UpdateArtifact`
(`internal/core/artifacts.go:463`) and the shipment path in
`internal/core.ShipShipment` (`internal/core/shipment_lifecycle.go:136`) have no
gate. Autoharness already ships a deterministic gate engine (`autoharness gate
check`), but nothing forces it to run at the completion boundary.

This work inserts a **built-in pre-transition gate broker** at the core mutation
layer (not only in CLI) so both CLI and MCP callers are gated identically. The
broker shells out to `autoharness gate check --json` via an **injectable command
runner** (there is no existing command-runner seam in `internal/`; `exec.Command`
is used directly, e.g. `internal/core/commits.go:132`), maps the autoharness exit
trichotomy to backlogit outcomes, records logs-only evidence, and consumes the
autoharness `repeated_failure` signal to drive requeue/escalate.

Enforcement boundary and insertion points already exist:

- Pre-hook fire site: `ws.HookRunner.FirePre(..., hooks.HookUpdateArtifact, ...)`
  at `internal/core/artifacts.go:476-490` — clean pre-transition insertion point
  that both CLI (`internal/cli/move.go`, `internal/cli/update.go`) and MCP
  (`internal/mcp/tools.go` `handleMoveItem` :511, `handleUpdateItem` :715) flow
  through.
- Hook types: `internal/hooks/hooks.go` (`HookPoint`, `HookContext` with
  `OldValues`/`NewValues map[string]any`, `HookFunc`, `HookRunner.FirePre`
  returning the first error). Built-in pre-hook precedent:
  `internal/hooks/builtin_pre.go` `ValidateStatusTransition`.
- Shipment ship path: `internal/core/shipment_lifecycle.go` `ShipShipment` :136,
  member enumeration via `NormalizeShipmentItems`.

## Requirements Trace

| # | Requirement (source) | Implementation action | Unit |
|---|---|---|---|
| R1 | Block task/subtask completion when configured gates fail; atomic (all-pass or non-done) | Pre-transition broker in `UpdateArtifact`; refuse write on non-zero, leave file+DB unchanged | U2 |
| R2 | Invoke only `autoharness gate check --json` via argv-array (no shell) | `GateRunner` interface + default `os/exec` argv-array runner | U2 |
| R3 | Base-ref = default-branch ref; autoharness computes 3-dot merge-base | Base-ref resolver (config `base_ref` -> `--gate-base` -> `origin/HEAD` -> `origin/main` -> `main`); never pre-compute `git merge-base` | U1 |
| R4 | Exit trichotomy: not-found=setup, exit2=config, exit1=blocked(6), exit0=pass | Exit-class mapper + `GateBlockedError`; treat exit 0 as pass even if stdout not JSON | U2, U3 |
| R5 | Three-valued `enabled`: auto/true/false | Config field + enforcement decision (auto fail-open when unresolvable; true fail-closed; false off) | U1, U2 |
| R6 | Min autoharness `>= 1.4.7` (`repeated_failure` present; `--no-count` for advisory) | Version/contract probe; treat missing `repeated_failure` as config error under `enabled: true` | U1 |
| R7 | Backlogit is sole executor of requeue/escalate driven by `repeated_failure` | Map reached+block -> `queued`; reached+escalate -> `blocked`; below-threshold blocked -> refuse (retain prior non-terminal status); no second breaker | U2 |
| R8 | Task lock held through gate; bounded-wait then fail-fast; re-read after acquire; no partial completion | Add bounded-wait wrapper over `lockTaskFile`; re-read state after lock; single atomic write on pass | U2 |
| R9 | Structured failure payload to CLI (`--json` `gate_report`, exit 6) and MCP (structured error content) | CLI exit-code + JSON mapping; MCP `gate_blocked` structured error | U3 |
| R10 | Operator-only, audited force; pass `--force` to autoharness; backlogit force event; no MCP force | `--force-gates`/`--force-reason` CLI flags; `pre_task_completion_gate_forced` event | U3, U4 |
| R11 | Logs-only gate evidence (pass/block/force/requeue/escalate) + durable hook events | Evidence appender mirroring `appendItemEvent` (`internal/core/shipment.go:279-315`) | U4 |
| R12 | Two-level gating: shipment aggregates member evidence AND runs shipment-diff gate | Shipment-level gate in `ShipShipment` reusing `GateRunner` + member-evidence aggregation | U4 |
| R13 | Advisory `doctor --check-gate-evidence` for new artifacts | Doctor advisory check | U4 |
| R14 | Docs: CLI/MCP/reference, autoharness composition, operator runbooks | Documentation unit | U5 |
| R15 | Fail-open when no autoharness lifecycle hooks configured | Rely on autoharness exit 0 for no-gates; never parse `.autoharness/config.yaml` | U1, U2 |

## Implementation Units

Global posture: **test-first** (constitution II, NON-NEGOTIABLE). Each subtask
writes a compiling-but-failing test harness first (red), then implementation
(green). Unit tests use the injectable `GateRunner` fake so no real autoharness
binary is needed; integration tests may use the installed autoharness `1.4.7`.
### Enforcement mechanism (revised — review-driven, replaces "pre-hook host")

The gate is enforced as a **first-class inline core completion service**, NOT hosted
inside the generic `HookUpdateArtifact` pre-hook. The generic pre-hook fires at
`internal/core/artifacts.go:487` but the write (`persistArtifact`) happens at `:557`
after the hook returns, `HookFunc` returns only `error`, and it cannot hold a lock
across the write or redirect the transition to `queued`/`blocked`. Relying on it would
create a TOCTOU window, force a self-deadlocking nested locked write (`lockTaskFile`
is a non-reentrant `TryLock` returning `ErrTaskBusy`), or depend on fragile
`updates`-map aliasing.

Instead, `core.UpdateArtifact` (task/subtask `-> done`) and `core.ShipShipment`
(shipment `-> shipped`) call a broker service **inline**, owning the full sequence:
`acquire bounded-wait task lock -> re-read state -> run gate -> receive an explicit
GateDecision -> apply the single locked write -> append evidence under the same lock
-> release`. The broker service returns a typed `GateDecision`
(`proceed | redirect_queued | redirect_blocked | block | error{class}`); the core
caller applies the decision to exactly one write. The generic `HookUpdateArtifact`
pre-hook remains a trigger/observation point only. This makes "no signature change to
`UpdateArtifact`" **false** — the entry points gain a request-scoped options
parameter (below), and lock ownership moves to wrap `FirePre`+`persist`. That rework
is budgeted into Unit 2.

**Sole durable-mutation owner (review-driven, attempt-2 Arch P1).** `core.UpdateArtifact`
and `core.ShipShipment` are the ONLY owners of the durable completion transition. The CLI
adapters (`internal/cli/move.go`, `update.go`) and the MCP adapters
(`internal/mcp/tools.go` `handleMoveItem`/`handleUpdateItem`) MUST NOT perform any
post-core relocate/write/upsert/status mutation of their own after calling the core API:
they receive a single core result contract carrying the final `status`, `outcome`, and
gate payload, and their only remaining job is to render an exit code (CLI) or an MCP
result/error body. This keeps the gated path single-authoritative — an adapter cannot
complete or redirect an item outside the gate. ST3.2/ST3.3 assert the adapters contain no
independent completion mutation.

#### Package boundary (revised — review-driven, one-way `core -> gate`)

`internal/core/gate/` is narrowed to the **autoharness integration boundary only** and
depends on NO `core` internals (prevents the `core -> gate -> core` import cycle). It
owns: the `GateRunner` exec seam, base-ref resolver, version/contract probe, `--json`
report parsing, and pure value types (`GateCheckRequest`, `GateResult`, `GateDecision`,
`RepeatedFailure{count,threshold,reached,action}`). It declares the small collaborator
interfaces it consumes (e.g. `RefResolver`) rather than importing `core`. Lock
ownership, re-read, durable state writes (including requeue/escalate), evidence
persistence, shipment aggregation, and doctor all live in **package `core`**, which
constructs adapters over its own unexported helpers (`lockTaskFile`, `appendItemEvent`,
`findArtifact`, `persistArtifact` — none exported merely to satisfy layout) and drives
the gate boundary. Typed errors/sentinels (`GateBlockedError`, and the config / setup /
timeout / retryable classes) live in the existing `internal/errors` leaf so
`internal/cli` and `internal/mcp` couple only to that leaf and route via `errors.As`
(matching `mcp/errors.go` `domainError`).

#### Request-scoped options (revised — review-driven)

Add a zero-value-safe `TransitionOptions`/`GateOptions` value threaded through the core
completion entry points, carrying: `GateBase` (from `--gate-base`), `Force bool`,
`ForceReason string`, and a `ForceSource` marker (CLI-only). CLI and MCP both flow
through the same typed core API; MCP passes the zero value (no force, no base override),
so force intent cannot leak via context values or mutable workspace state, and the
broker refuses force from any non-CLI `ForceSource`.

### Unit 1 — Config and command context (Task T1)

Domain: config + resolver code. Depends on: none.

- **ST1.1 — Config schema + defaults (config domain).**
  - Files: `internal/config/schema.go` (add `PreTaskCompletionGateConfig` and a
    `PreTaskCompletionGate` field on `LifecycleHooksConfig` at :126),
    `internal/config/defaults.go` (:473-509 `DefaultHooksConfig`/`defaultHooksYAML`).
  - Fields: `enabled` (string enum `auto|true|false`, default `auto`),
    `terminal_statuses` (`["done"]`), `autoharness_binary` (`autoharness`),
    `base_ref` (`auto`), `timeout_seconds` (`600`),
    `force_cli_only` (`true`), `evidence_required` (`true`). The head ref is NOT an
    operator-tunable field: it is pinned to the fixed constant `HEAD` (review-driven,
    attempt-2 Security P1 — a configurable head ref could select an empty-diff ref that
    silently passes the gate). The `1.4.7` version floor is a **code constant** in the
    probe (ST1.3), NOT a config field (dropping the YAGNI operator-tunable
    `min_autoharness_version`).
  - Validation (review-driven): reject `enabled` outside `auto|true|false`; reject
    empty/unknown `terminal_statuses`; reject non-positive or out-of-range
    `timeout_seconds` — enforce a sane min AND a max STRICTLY LESS THAN the task-lock
    stale TTL (`taskStaleLockTTL`, 60s in `internal/core/task_lock.go`) unless the gate
    path refreshes the lock sidecar (ST2.3), so a long gate cannot both hang the completion
    path under lock (DoS) and let a concurrent process reap the "stale" live lock; reject
    `force_cli_only: false` for v1 (CLI-only force is a hard invariant); constrain
    `autoharness_binary` resolution (reject `..` traversal and absolute paths outside
    PATH/workspace; prefer PATH lookup) since it is a config-controlled executable the
    broker auto-invokes.
  - Tests: YAML round-trip; default injection; `enabled` validation; empty
    `terminal_statuses` rejected; `timeout_seconds` bounds (0/negative/huge rejected;
    value `>= taskStaleLockTTL` rejected unless heartbeat mode);
    `force_cli_only: false` rejected; `autoharness_binary` traversal rejected.
  - Posture: test-first. Test matrix rows: auto / true / false / invalid status.
  - Milestone: config loads and validates with new block; `go test ./internal/config/...` green.

- **ST1.2 — Base-ref resolver + ref verification (code domain).**
  - Files: `internal/core/gate/baseref.go` (+ `_test.go`).
  - Behavior: resolution order — explicit config `base_ref` when not `auto` ->
    caller-supplied `--gate-base` (from `TransitionOptions`) -> `origin/HEAD` (symbolic
    default) -> `origin/main` -> `main`. Return the resolved ref string only; DO NOT run
    `git merge-base` (autoharness computes the 3-dot merge-base). The head ref is always
    the fixed constant `HEAD` (not resolver-controlled).
  - Ref verification (review-driven — closes the fail-open suppression gap): before
    invocation, verify the resolved base ref AND `HEAD` actually resolve (a `git rev-parse
    --verify`-equivalent through the injected resolver), and reject an invalid explicit
    `base_ref`/`--gate-base`. Because `autoharness gate check` degrades git/ref errors to
    an empty diff and exit `0`, an unverified bad ref would silently suppress gating; so a
    ref that cannot be verified while gates are enforced (`true`, or `auto` with the gate
    engaged) MUST refuse completion with a configuration error rather than pass.
  - Base-override break-glass (review-driven, attempt-2 Security P1): a non-default base
    (explicit `base_ref != auto` or a `--gate-base` that does not resolve to the
    default-branch ref) is a privileged override, because narrowing/altering the base can
    collapse the diff and weaken the gate without the audited force path. Treat it with the
    SAME discipline as force: `--gate-base` is an operator-only CLI flag (never on MCP),
    and any non-default base while enforcing records a durable
    `pre_task_completion_gate_base_override` evidence event (U4). It does NOT bypass the
    gate — it still runs against the chosen base — but it is auditable and non-agent.
  - Tests: each precedence rung; `origin/HEAD` resolution via injected git runner;
    unresolved -> config error; invalid explicit `--gate-base` -> config error; a
    non-default base while enforcing emits the override audit event; head is always `HEAD`;
    verified ref -> proceeds. Use an injected ref-resolver func (mirror the `confineFn`
    test-seam at `internal/core/doctor_target.go:121-129`).
  - Posture: test-first. Milestone: resolver + verifier unit tests green with no real git dependency.

- **ST1.3 — autoharness version/contract probe (code domain).**
  - Files: `internal/core/gate/probe.go` (+ `_test.go`).
  - Behavior: resolve binary (`autoharness_binary`), probe version/contract for
    `>= 1.4.7` (a code constant, `minAutoharnessVersion = "1.4.7"`) and presence of the
    `repeated_failure` contract. Under `enabled: true` an unresolvable/incompatible binary
    is a **configuration error** (fail-closed); under `enabled: auto` an unresolvable
    binary yields "not enforceable" (fail-open). Pass a minimal/allowlisted environment to
    the probe/runner rather than inheriting the full ambient environment (trust-boundary
    hardening); `enabled: true` documents that it trusts an operator-managed autoharness
    binary and config.
  - Tests: resolvable+compatible -> enforce; missing binary under `auto` -> fail-open;
    missing binary under `true` -> config error; version `< 1.4.7` under `true` ->
    config error. Runner injected; no real binary in unit tests.
  - Posture: test-first. Milestone: probe returns the correct enforcement decision
    per (enabled, binary-state) matrix; tests green.

### Unit 2 — Core transition broker (Task T2)

Domain: core code. Depends on: **U1** (config, resolver, probe). Enforced as an
inline core completion service (see "Enforcement mechanism"), not a generic pre-hook.

- **ST2.1 — GateRunner exec seam + default argv-array runner (code domain).**
  - Files: `internal/core/gate/runner.go` (+ `_test.go`).
  - Define `type GateRunner interface { Run(ctx context.Context, args []string, workspace string, env []string) (GateResult, error) }`.
    `GateResult` carries ONLY process-outcome fields: `ExitCode int`, `Stdout []byte`,
    `Stderr []byte`. Failure-to-run is a **returned error**, not a struct field:
    binary-not-found returns a wrapped sentinel `errors.ErrGateBinaryNotFound`
    (`errors.Is`); a process that ran and exited N returns `GateResult{ExitCode:N}` with a
    nil error (extract via `var ee *exec.ExitError; errors.As(err, &ee)`).
  - Timeout (review-driven): honor `timeout_seconds` via `exec.CommandContext`; detect a
    timeout via `errors.Is(ctx.Err(), context.DeadlineExceeded)` and return a distinct
    timeout class — NEVER map the killed-process exit code (`-1`, platform-dependent on
    Windows where this repo runs) to exit `1`/blocked.
  - Security invariants: argv-array exec ONLY (never a shell string); fixed argv template
    with positional substitution (`{item_id}`, `{base_ref}`, `{head_ref}`,
    `{workspace_root}`); no value concatenated into a shell string; no untrusted-path
    interpolation; a minimal/allowlisted `env` passed in (not the full ambient env).
  - Tests: exact argv assembled (golden argv, table asserts NO shell-metacharacter path);
    binary-not-found -> `ErrGateBinaryNotFound`; ran-and-exited-N -> `GateResult{ExitCode:N}`,
    nil error; deadline -> timeout class (not exit 1). Milestone: runner unit tests green.

- **ST2.2 — Exit-class mapping, GateDecision, and typed errors (code domain).**
  - Files: `internal/core/gate/decision.go` (+ `_test.go`) for the pure mapper; typed
    errors/sentinels in the `internal/errors` leaf (`GateBlockedError` struct +
    `ErrGateBinaryNotFound`, `ErrGateConfig`, `ErrGateTimeout`, `ErrGateInProgress`).
  - Behavior: a pure function maps `(enabled, GateResult|error, RepeatedFailure)` to a
    typed `GateDecision`:
    - binary-not-found -> under `auto` `proceed` (fail-open); under `true` `error{setup}`;
    - exit `2` -> `error{config}` (refuse);
    - timeout -> `error{timeout}` (refuse, retryable-ish; distinct from blocked);
    - exit `1` + `repeated_failure.reached=true, action=block` -> `redirect_queued`;
    - exit `1` + `reached=true, action=escalate` -> `redirect_blocked`;
    - exit `1` + below threshold -> `block` (carry `GateBlockedError{ItemID, BaseRef, HeadRef, ExitCode, ReportJSON, Stderr}`; refuse, no write — the retained status is NOT decided here: the pure mapper has no view of the artifact's current status, so retained/`new_status` is resolved by the ST2.3 reread `old_status` and the ST2.5 result contract, whatever the prior non-terminal status was);
    - exit `0` -> `proceed` (treat exit 0 as pass even if stdout is not JSON);
    - malformed/missing `repeated_failure` under `true` -> `error{config}` (contract violation).
  - This is a pure mapper (no I/O), so it is exhaustively table-testable without a
    workspace. Splitting the mapper out of the transition wiring (ST2.3) keeps each
    subtask under the 3-file / 4-scenario envelope.
  - Tests: full decision table (table-driven, one scenario family) incl. every exit class,
    both requeue/escalate rows, below-threshold block, malformed contract.
  - Milestone: decision mapper covers every class deterministically.

- **ST2.3 — Bounded-wait task-lock lifecycle + sidecar heartbeat + reread (lock domain).**
  - Files: `internal/core/task_lock.go` (add a bounded-wait acquire and a lock-sidecar
    ModTime refresh; today `lockTaskFile` is a non-blocking `TryLock` with a 60s
    `taskStaleLockTTL` and no heartbeat), `internal/core/gate_transition.go` (new lock
    wrapper used by the inline service).
  - Behavior: acquire the task lock with a bounded wait — retry on
    `errors.Is(err, ErrTaskBusy)` up to a ~2-5s cap with backoff, selecting on
    `ctx.Done()`; fail-fast with `ErrGateInProgress` ("gate in progress for `<item>`, retry
    shortly") on cap. **Heartbeat (review-driven, attempt-2 Go P1):** because a gate run may
    exceed `taskStaleLockTTL` (default `timeout_seconds` 600s >> 60s TTL) and backlogit runs
    as per-invocation subprocesses, the wrapper MUST refresh the lock sidecar's ModTime on a
    heartbeat interval strictly less than `taskStaleLockTTL` for the whole hold, so a
    concurrent CROSS-PROCESS caller never treats the live lock as crash residue and reaps it
    mid-gate (which would let two processes both write `done`, defeating single-completion).
    Alternative satisfied by config: if `timeout_seconds < taskStaleLockTTL` the heartbeat is
    a no-op, but the heartbeat is the general guard. After acquire, **re-read** artifact
    state; if it no longer needs completion, no-op/return current state. Release on all paths
    (defer).
  - Tests: contended lock -> second caller gets `ErrGateInProgress` after the bounded wait;
    a simulated gate run longer than `taskStaleLockTTL` -> the sidecar stays fresh and a
    concurrent acquire still sees the lock held (no stale reap); re-read shows
    already-completed -> no-op; lock always released.
  - Milestone: lock is held safely across a >TTL gate with no cross-process stale reap;
    contention is deterministic.

- **ST2.4 — Inline completion service: gate call + single locked write + evidence hook (code domain).**
  - Files: `internal/core/gate/broker.go` (orchestration over injected collaborators),
    `internal/core/gate_transition.go` (apply the decision), `internal/core/artifacts.go`
    (`UpdateArtifact` gains the `TransitionOptions` param and the inline gate call around
    `FirePre`+`persistArtifact`).
  - Behavior: trigger only when artifact type is `task|subtask`, the update includes a
    status change, the new status is a `terminal_statuses` value, the previous status is
    not already terminal, and the broker is enabled. Inside the ST2.3 lock/reread wrapper:
    run the gate, receive the single `GateDecision`, and apply it to exactly one locked
    write (`proceed` -> `done`; `redirect_queued` -> `queued`; `redirect_blocked` ->
    `blocked`; `block`/`error` -> no write). Evidence is appended under the SAME lock BEFORE
    the status write is finalized (review-driven ordering, attempt-2 Go advisory): under
    `evidence_required` a failed evidence append rolls back / prevents the status write so a
    completion never persists without its audit record (ST4.1 supplies the error-returning
    appender; the current `appendItemEvent` is best-effort/void). `UpdateArtifact`/
    `ShipShipment` are the sole durable-mutation owners (see "Sole durable-mutation owner");
    no nested locked write, no partial completion, no `updates`-map aliasing.
  - Tests (matrix, table-driven where possible): no config/empty hooks -> allowed, no gate
    run; all pass -> `done`; one fail -> refused, file+DB unchanged (assert both, mirroring
    `archive_test.go` rollback assertions); advisory exit 0 -> allowed; missing binary +
    `true` -> config error; timeout -> refused (timeout class); injected evidence-append
    failure under `evidence_required` -> transition rolled back, state unchanged.
  - Milestone: the inline service enforces atomically; refusal leaves file+DB unchanged;
    evidence is transactional with the write.

- **ST2.5 — repeated_failure requeue/escalate mapping + synchronous outcome contract (code domain).**
  - Files: `internal/core/gate_transition.go` (surface the outcome), `internal/core/gate/decision.go`
    (already emits the decision, ST2.2).
  - Behavior (backlogit is the sole executor): the redirect writes are performed by the
    ST2.4 single locked write — `redirect_queued` -> `queued`, `redirect_blocked` ->
    `blocked`, below-threshold `block` -> refuse and RETAIN the artifact's actual prior
    non-terminal status, `proceed` -> `done`. The prior status is NOT assumed to be
    `active`: a `-> done` attempt can originate from any non-terminal status including
    `review` (the model defines `queued|active|blocked|review|done`), so the reread
    `old_status` (captured in ST2.3) is the authoritative retained status on a block
    (review-driven, attempt-3 Architecture P1). Reference the autoharness checkpoint in the
    evidence. Do NOT add a second circuit breaker (honor autoharness's). The completion-path
    run MUST NOT double-count the autoharness counter (completion path is the authoritative
    counting caller; advisory pre-checks use `--no-count`). Return a core result contract to
    callers carrying `old_status`, `outcome` (`passed|blocked|requeued|escalated`),
    `state_changed bool`, the resulting `new_status` (equal to `old_status` on a block),
    and the `repeated_failure {count,threshold,reached,action}` object so U3 can build the
    synchronous CLI/MCP response (agents must know synchronously whether the item moved and
    whether to retry or stop).
  - Tests: each decision-doc mapping row produces the correct durable transition + outcome;
    malformed `repeated_failure` under `true` -> config error; below-threshold block from
    `active` retains `active` AND below-threshold block from `review` retains `review`
    (both with `state_changed:false` and `new_status == old_status`); requeue/escalate
    report `state_changed:true` and the new status; no double-count on the counting caller.
  - Posture: test-first. Milestone: all four decision-table rows produce the correct durable
    transition and a machine-actionable outcome contract.

### Unit 3 — CLI/MCP response mapping (Task T3)

Domain: CLI/MCP adapter code. Depends on: **U2**. Both surfaces route the typed
errors/decisions from the `internal/errors` leaf via `errors.As`.

- **ST3.1 — Versioned exit-code table + ExitError mapping (code domain).**
  - Files: `internal/cli/exit_error.go` (reuse `ExitError`/`ExitCodeFor`), `cmd/backlogit`
    wiring where `ExitCodeFor` is honored.
  - Define the FULL versioned gate exit-code table (review-driven — not just blocked=6),
    verified to NOT collide with doctor's existing 0-4 contract: `6` = gate blocked,
    `7` = gate configuration error (missing/incompatible binary under `true`, exit 2,
    malformed contract), `8` = gate in progress / retryable (lock contention, timeout).
    Each is carried by a distinct typed error via `*cli.ExitError` so `ExitCodeFor`
    resolves it deterministically instead of collapsing to generic `1`.
  - Tests: `ExitCodeFor` returns 6/7/8 for the respective wrapped typed errors; no
    collision with doctor codes. Milestone: exit-code table green.

- **ST3.2 — CLI move/update gate mapping + outcome payload (code domain).**
  - Files: `internal/cli/move.go`, `internal/cli/update.go` (add `--gate-base`,
    `--force-gates`, `--force-reason`, threaded via `TransitionOptions`; map decisions to
    `ExitError`). These adapters render exit codes ONLY — they perform no independent
    completion/status mutation (the core API is the sole mutation owner; assert this).
    `--gate-base` is operator-only break-glass; a non-default base emits the
    `pre_task_completion_gate_base_override` audit event (ST1.2/U4).
  - Human output (review-driven wording fix): report the ACTUAL retained/post-transition
    status from the core result's `old_status`/`new_status`, NOT a hard-coded literal —
    because both requeue/escalate change the status AND a block can occur from `active` or
    `review`. Blocked -> "task remains `<old_status>` (e.g. active/review); gate blocked";
    requeued -> "task moved to `queued` (repeated gate failure)"; escalated -> "task moved
    to `blocked` (escalated)". Truncate stderr in human output. `--json` output carries the
    full contract: `old_status`, `outcome (passed|blocked|requeued|escalated)`,
    `state_changed` (true on requeue/escalate/pass, false on below-threshold block),
    `new_status` (equal to `old_status` on a block), `gate_report` (full), and
    `repeated_failure {count,threshold,reached,action}`. On the PASS path `--json` also
    carries `outcome: passed` + a gate-evidence reference (`gate_report_hash`, `head_sha`)
    so a completed gate is machine-visible.
  - Tests: block from `active` -> exit 6 + `gate_report` + `state_changed:false` +
    `new_status: active`; block from `review` -> retains `review` in wording and
    `new_status`; requeued -> exit 6-family + correct wording + `new_status: queued` +
    `state_changed:true`; escalated -> `new_status: blocked`; retryable -> exit 8; pass ->
    `outcome: passed` + evidence ref; `--json` retains full report; human truncates stderr;
    adapter performs no mutation of its own.
  - Milestone: CLI conveys retry-vs-stop synchronously; pass path unaffected.

- **ST3.3 — MCP structured errors: full outcome-class contract + parity (code domain).**
  - Files: `internal/mcp/tools.go` (`handleMoveItem` :511, `handleUpdateItem` :715),
    `internal/mcp/errors.go` (add structured bodies mirroring `blockingChildrenResult`
    :100-126; route the gate typed errors in `domainError` so NONE collapse to the generic
    `internal` branch :95, and ensure the retryable case is NOT collapsed into the generic
    `conflict`/`ErrTaskBusy` mapping at `errors.go:86`). These handlers render MCP
    result/error bodies ONLY — no independent completion mutation (core is the sole owner).
  - Behavior: every non-pass outcome class gets a distinct, machine-actionable
    `error_type` (review-driven, attempt-2 Parity P1 — the CLI has a full 6/7/8 trichotomy
    so MCP must too):
    - `gate_blocked` (non-retryable, self-heal): `item_id`, `old_status`,
      `requested_status`, `base_ref`, `head_ref`, `gate_report`, `stderr_summary`, and an
      `allowed_next_actions: ["repair_and_retry","move_to_non_terminal"]` hint (the agent
      escape hatch is discoverable in-band, not only in U5 docs).
    - `gate_requeued` / `gate_escalated` (state DID change): distinct types carrying
      `state_changed: true`, `outcome`, `new_status`, `repeated_failure` so an agent
      branching on the primary `error_type` cannot mistake a completed redirect for a
      no-op block and drive a retry/escalation loop.
    - `gate_config` / `gate_setup` (non-retryable, ESCALATE to operator — maps CLI exit 7):
      missing/incompatible binary under `true`, autoharness exit 2, malformed
      `repeated_failure` contract; carry an operator-remediation hint. This is the outcome
      class that must NOT be self-healed by editing code or blindly retried.
    - `gate_timeout` (retryable, maps CLI exit 8): `retryable: true`, `retry_after_ms`
      distinct from lock contention.
    - `gate_in_progress` (retryable, lock contention, exit 8): `retryable: true`,
      `retry_after_ms`, `item_id`.
  - Pass parity: on the MCP success path, include `outcome: passed` + a gate-evidence
    reference (`gate_report_hash`, `head_sha`) in the result so a passed gate is
    machine-visible symmetrically with CLI `--json` (not merely inferable from
    `status=done`).
  - Machine-context parity (review-driven): the MCP machine path carries equivalent repair
    context to CLI `--json` — assert all actionable detail lives in `gate_report` so a
    stderr summary is lossless. MCP exposes NO force field.
  - Tests: blocked -> `gate_blocked` with every field + `allowed_next_actions`; contention
    -> `gate_in_progress` retryable; timeout -> `gate_timeout` retryable; config/setup ->
    `gate_config`/`gate_setup` non-retryable with remediation hint (NOT `internal`);
    requeue/escalate -> `gate_requeued`/`gate_escalated` + `state_changed:true` +
    `new_status`; pass -> `outcome: passed` + evidence ref; MCP has NO force field.
  - Milestone: every gate outcome class has a distinct agent-actionable MCP contract; none
    fall through to `internal`.

- **ST3.4 — Operator-only CLI force path with source marker (code domain).**
  - Files: `internal/cli/move.go`, `internal/cli/update.go` (force flags set
    `TransitionOptions.ForceSource = CLI`); broker consumes and enforces the CLI-only rule.
  - Behavior: `--force-gates` requires a non-empty `--force-reason`; the broker refuses
    force from any non-CLI `ForceSource` (hard invariant, complementing `force_cli_only`
    schema validation and the absent MCP force field). On valid force it passes `--force`
    to `autoharness gate check` (autoharness writes its own force audit) and records a
    backlogit `pre_task_completion_gate_forced` event (evidence in U4). Milestone: forced
    CLI completion moves to `done` and emits the forced event; MCP has no force surface;
    non-CLI force intent is refused.
  - Tests: force without reason -> usage error; force with reason (CLI source) -> argv
    includes `--force`, completes; force with non-CLI source -> refused; MCP path has no
    force flag.

### Unit 4 — Evidence, shipment gating, and doctor check (Task T4)

Domain: core code + doctor. Depends on: **U2** (uses the decision service + `GateRunner`).

- **ST4.1 — Transactional gate evidence (logs-only) + error events (code domain).**
  - Files: `internal/core/gate_evidence.go` (in package `core`, mirroring
    `appendItemEvent`/`appendItemEventWithCommit` at `internal/core/shipment.go:279-315`,
    `events.NewEventWriter`). Evidence lives in `core`, not the `gate` boundary package.
  - Events: `pre_task_completion_gate_passed`, `_blocked`, `_forced`, `_requeued`,
    `_escalated`, `_base_override` (non-default base while enforcing, per ST1.2), and
    (review-driven) `_error` for setup/config/timeout refusals so those refusals are
    traceable and appear in the monitoring signals. Item logs only (NOT frontmatter, per
    Q3). Include `base_ref`, `head_ref`, `head_sha`, `gate_report_hash`, timestamp; forced
    events include operator reason; requeue/escalate reference the autoharness checkpoint.
  - Transactionality (review-driven): under `evidence_required: true`, appending the
    backlogit `pre_task_completion_gate_*` evidence is PART of the transition contract —
    the append is atomic, occurs under the held task lock, and happens BEFORE the status
    write is finalized (attempt-2 ordering fix) so a passing/forced/redirect completion
    cannot persist without its backlogit audit record; a failed append rolls back / prevents
    the transition. This requires a NEW error-returning appender (the existing
    `appendItemEvent` at `shipment.go:279-315` is best-effort/void) — add
    `appendItemEventErr` (or equivalent) returning an error that the ST2.4 write path
    honors. Durable hook-event fan-out to subscribers remains best-effort and separate from
    the mandatory item-log evidence.
  - Tests: each event type appended with expected fields; `_error` emitted on
    setup/config/timeout; no frontmatter mutation; under `evidence_required`, an injected
    append failure rolls back the transition (state unchanged); append happens under lock.
  - Milestone: evidence JSONL present for every outcome incl. errors; transactional under
    `evidence_required`. Note: the DERIVED indexed read-model column is OUT OF SCOPE
    (follow-up stash `7ED9CE1A`).

- **ST4.2 — Shipment-level two-level gating (code domain). Depends on: U2 + ST4.1.**
  - Files: `internal/core/shipment_lifecycle.go` (`ShipShipment` :136); two explicit
    `core` collaborators — a member-evidence validator and a shipment-diff gate check
    (reusing the `GateRunner`). Split the two concerns rather than blending read-model
    validation with new state mutation.
  - Behavior: when moving a shipment to `shipped`, BOTH (a) validate recorded member-task
    gate evidence — every member task (via `NormalizeShipmentItems`) MUST carry passing
    gate evidence at or after its final head SHA — AND (b) run a shipment-level
    `autoharness gate check` over the full shipment diff (base = shipment merge target,
    head = `HEAD`). Complete only when both pass; otherwise refuse and return the report,
    leaving shipment state unchanged.
  - Reconciliation (review-driven): `ShipShipment` today completes release-scope items via
    `completeReleaseScope`. Decision to document and honor: shipment gating MUST NOT
    auto-complete an ungated member task — a member lacking valid gate evidence causes the
    ship transition to REFUSE (not silently complete it through an ungated path). Release
    finalization does not become a gate-bypass.
  - Tests: all members have valid evidence + shipment gate passes -> shipped; a member
    lacks evidence -> refused (not auto-completed); member evidence stale (before final
    head SHA) -> refused; shipment-level gate fails -> refused. Milestone: ship path
    enforces both checks; refusal leaves shipment state unchanged.

- **ST4.3 — Doctor advisory gate-evidence check (code domain).**
  - Files: `internal/core/doctor.go` / `internal/core/doctor_target.go`, CLI wiring
    `internal/cli/doctor.go`.
  - Behavior: `backlogit doctor --check-gate-evidence` (advisory only in first release):
    find terminal `task`/`subtask` artifacts, scan item logs for matching
    `pre_task_completion_gate_passed`/`_forced` evidence at or after the same head SHA, and
    WARN (do not fail) when a terminal task lacks evidence while gates were configured.
    First release performs a log scan (the indexed read-model query column is the separate
    follow-up `7ED9CE1A`); a strict, query-backed check lands later. Milestone: doctor
    emits advisory warnings only; exit code unaffected in advisory mode.
  - Tests: terminal task with evidence -> no warning; without evidence (gates configured)
    -> advisory warning; advisory mode never changes exit code.

### Unit 5 — Documentation (Task T5)

Domain: docs. Depends on: **U1-U4** complete (documents the shipped surface). Last.

- **ST5.1 — CLI/MCP/reference docs + command map (docs domain).**
  - Update CLI docs for `--force-gates`/`--force-reason`/`--gate-base` and exit code
    `6`; MCP docs for the `gate_blocked` structured error; regenerate the generated
    command/reference docs and command map. Milestone: `make docs-lint` clean;
    reference regenerated.

- **ST5.2 — Composition + operator runbooks (docs domain).**
  - Document how the broker composes with autoharness `lifecycle_hooks`; the
    three-valued `enabled` semantics; the two-level gating model; and operator runbooks
    for (a) gate failure, (b) missing/incompatible autoharness binary, (c) force
    override, (d) kill-switch `enabled: false`. Milestone: runbooks present and lint-clean.

## Dependency Graph

```text
U1 (config/resolver/probe)
  -> U2 (core transition broker)          [U2 depends on U1]
       -> U3 (CLI/MCP response mapping)    [U3 depends on U2]
       -> U4 (evidence/shipment/doctor)    [U4 depends on U2]
U5 (documentation)                          [U5 last; depends on U1-U4]
```

Intra-unit subtask order: ST1.1 -> ST1.2 -> ST1.3; ST2.1 -> ST2.2 -> ST2.3 -> ST2.4 -> ST2.5;
ST3.1 -> ST3.2 -> ST3.3 -> ST3.4; ST4.1 -> ST4.2 -> ST4.3; ST5.1 -> ST5.2. No cycles.
Note ST4.2 (shipment gating) additionally depends on ST4.1 (evidence) because it
validates recorded member-task evidence.

## Decisions and Rationale

- **Enforcement is an inline core completion service, NOT the generic pre-hook**
  (review-driven). The `HookUpdateArtifact` pre-hook fires before `persistArtifact`
  (`artifacts.go:487` vs `:557`), returns only `error`, and cannot hold a lock across the
  write nor redirect the transition. `UpdateArtifact`/`ShipShipment` therefore call the
  broker inline (lock -> reread -> gate -> decide -> single write -> evidence -> release)
  and gain a `TransitionOptions` parameter. The generic pre-hook stays a trigger point
  only. Rejected alternative: hosting the broker in the pre-hook — it produces a TOCTOU
  window, a self-deadlocking nested locked write, or fragile `updates`-map aliasing.
- **One-way `core -> gate` package boundary** (review-driven). `internal/core/gate/` is the
  autoharness integration boundary only (runner, resolver, probe, report parse, pure value
  types incl. `GateDecision`); it imports NO `core` internals, avoiding the `core -> gate ->
  core` import cycle. Lock ownership, durable writes, evidence, shipment aggregation, and
  doctor live in package `core`, which injects adapters over its own unexported helpers.
  Typed errors/sentinels live in the `internal/errors` leaf so `cli`/`mcp` couple only to
  the leaf and route via `errors.As`.
- **Request-scoped `TransitionOptions`** (review-driven). `--gate-base`/force intent flow
  through a typed, zero-value-safe options object on the core entry points, not via context
  values or mutable workspace state. MCP passes the zero value; CLI sets `ForceSource=CLI`.
- **Explicit `GateResult`/error contract** (review-driven). `GateResult` holds only
  process-outcome; failure-to-run is a wrapped sentinel (`errors.Is`); timeout is detected
  via `ctx.Err()` (never the platform-dependent `-1` exit code) as a distinct class.
- **Full versioned exit-code + response contract** (review-driven). Distinct codes for
  blocked (6), config (7), and retryable/in-progress (8), verified against the doctor 0-4
  table; requeue/escalate outcomes and `repeated_failure` are carried synchronously to CLI
  `--json` and MCP so agents know whether to retry or stop.
- **New `GateRunner` seam.** No command-runner seam exists in `internal/`; adding one is
  required so unit tests never touch a real autoharness binary (design's run_fn-style
  injection). Argv-array-only exec is both a security invariant and a testability seam.
- **Base-ref passed as the default-branch ref; autoharness computes the 3-dot merge-base.**
  Matches the decided contract; avoids a subtle double-merge-base bug from pre-computing.
  Refs are verified before invocation so a bad ref cannot silently fail-open to exit 0.
- **Backlogit is the sole executor of requeue/escalate.** Preserves backlogit as the single
  writer of durable task state and autoharness as policy/counter authority; no reentrancy;
  no second circuit breaker (honor autoharness's).
- **Evidence in item logs only, and transactional under `evidence_required`.** Avoids
  completion-write frontmatter churn/merge conflicts (constitution IX); the indexed
  read-model column is a separate follow-up (`7ED9CE1A`). A passing/forced completion cannot
  succeed without its backlogit audit record.
- **Force is CLI-only, source-marked, and audited.** Agents cannot self-bypass: MCP exposes
  no force field, `force_cli_only: false` is rejected at schema validation, and the broker
  refuses force from a non-CLI `ForceSource`. Both autoharness and backlogit write audit records.
- **Bounded-wait then fail-fast on lock contention.** `lockTaskFile` is non-blocking today;
  a bounded-wait wrapper (selecting on `ctx.Done()`) gives dark-factory liveness without
  letting a long gate trip stall or circuit-breaker timers, while re-read-after-acquire
  preserves consistency. Contention surfaces as a distinct retryable error/exit code.

## Risks and Caveats

- **Blast radius: core completion path for every agent.** Mitigation: the `enabled: false`
  kill-switch, `auto` default fail-open when autoharness is unresolvable, and comprehensive
  matrix tests including "no config -> transition allowed".
- **External process execution (security-sensitive).** Mitigation: argv-array-only exec, no
  shell interpolation, no untrusted-path concatenation, fixed argv template with positional
  substitution, timeout bound with validated min/max, minimal/allowlisted environment.
- **Ref-error fail-open suppression.** Autoharness degrades git/ref errors to an empty diff +
  exit 0. Mitigation: verify base + `HEAD` refs before invocation; a ref that cannot be
  verified while gates are enforced refuses completion (config error) rather than silently
  passing.
- **External-binary trust boundary.** `autoharness_binary` is config-controlled and runs with
  backlogit's ambient credentials, and autoharness executes repo-defined gates. Mitigation:
  constrain binary resolution (reject `..`/traversal, prefer PATH), minimal env, and document
  that `.backlogit/hooks.yaml` is a trusted, code-reviewed file and `enabled: true` trusts an
  operator-managed autoharness install/config.
- **Force path is a privileged bypass.** Mitigation: operator-only CLI, source-marked
  (`ForceSource=CLI`), mandatory reason, `force_cli_only: false` rejected at schema, dual
  audit (autoharness `--force` audit log + backlogit forced event), no MCP surface.
- **Contract coupling to autoharness `1.4.7`.** Mitigation: explicit version/contract probe
  (1.4.7 code constant); missing `repeated_failure` treated as a config error under
  `enabled: true`, fail-open under `auto`.
- **Shipment-level gating touches release finalization.** Mitigation: refusal leaves shipment
  state unchanged; both member-evidence validation and shipment-diff gate must pass; a member
  lacking evidence refuses the ship rather than being auto-completed; covered by shipment tests.
- **Stderr may contain external-tool output.** Mitigation: truncate for human output; preserve
  the full report for machine callers with parity between CLI `--json` and MCP.
- **Evidence-write failure.** Under `evidence_required: true`, a failed backlogit evidence
  append rolls back the transition so a completion never lands without its audit record.

## Plan Hardening Signals (REQUIRED)

| Signal | Present? | Justification |
|---|---|---|
| public API, schema, or contract change | **Yes** | New config schema (`pre_task_completion_gate`), new CLI flags + exit code `6`, new MCP `gate_blocked` structured error, new item-log evidence event contract, and a cross-tool contract dependency on autoharness `>= 1.4.7`. |
| security/auth/permission/compliance-sensitive behavior | **Yes** | External process execution (command injection surface) and an operator-only force bypass of a validation gate. |
| migration/backfill/destructive/irreversible step | **No** | No data migration or destructive action; evidence is additive (append-only logs). Doctor is advisory. |
| external integration / operator checkpoint / external dependency | **Yes** | Hard dependency on the external `autoharness` binary and its `gate check --json` contract; operator force checkpoint. |
| high runtime/rollout/rollback risk | **Yes** | Changes the core task/shipment completion path for all agents; needs a clean kill-switch and staged enablement. |

Requires plan hardening: yes

## Runtime Verification and Closure

Changed runtime surfaces: CLI (`move`/`update`/`doctor`, new flags, exit codes `6`/`7`/`8`),
MCP (`backlogit_move_item`/`backlogit_update_item` structured error classes), the task-lock
heartbeat, and the shipment ship path.

- **U1**: config surface — verify a workspace with the new block loads; verify
  default injection on a fresh workspace.
- **U2**: core transition — verify against the installed autoharness `1.4.7`
  integration path that a real failing gate refuses `-> done` and a passing gate
  completes with evidence; verify `auto` fail-open when the binary is absent.
- **U3**: CLI/MCP — verify exit codes `6` (blocked) / `7` (config/setup) / `8` (retryable,
  timeout + contention) and `gate_report` in `--json`; verify each MCP outcome class has a
  distinct structured `error_type` — `gate_blocked` (with `allowed_next_actions`),
  `gate_requeued`/`gate_escalated` (`state_changed:true`+`new_status`), `gate_config`/
  `gate_setup` (operator-remediation, non-retryable, never `internal`), `gate_timeout` and
  `gate_in_progress` (retryable) — plus the requeue/escalate `outcome`+`repeated_failure`
  fields and the pass-path `outcome: passed`+evidence ref on BOTH surfaces; verify CLI/MCP
  adapters perform no completion mutation of their own; verify force requires a reason and
  is CLI-source-only.
- **U4**: shipment ship refuses when a member lacks evidence or the shipment-diff
  gate fails; doctor emits advisory-only warnings.
- **U5**: `make docs-lint` clean; reference regenerated.

Operational closure: kill-switch is `enabled: false`; rollback trigger is any
unexpected completion refusal in a repo without configured gates (should be
impossible given `auto` fail-open — treat as a rollback signal). Owner: gate-broker
feature owner. Validation window: one full Stage->Ship cycle after enablement.
Monitoring signals: `pre_task_completion_gate_blocked` / `_forced` event volume and
retryable "gate in progress" errors.

## Constitution Check

| Principle | Compliance |
|---|---|
| I. Safety-First Go | All new code in Go; errors wrapped with `%w`; sentinel `GateBlockedError` + not-found/config/setup error classes; `go vet`/`golangci-lint`/`gofmt` gates apply. |
| II. Test-First (NON-NEGOTIABLE) | Every subtask is test-first with a failing harness first; `GateRunner` fake enables red-before-green without a real binary. |
| III/IV. Workspace Isolation & CLI Containment | Broker only reads/writes within the workspace; `--workspace <root>` passed to autoharness; no writes outside cwd; argv-array exec avoids shell traversal. |
| V. Structured Observability | Logs-only evidence (incl. `_error` for setup/config/timeout) + best-effort durable hook events for every outcome; structured CLI/MCP errors carrying outcome/new_status/repeated_failure. |
| VI. Single Responsibility | Reuses existing hook, lock, event, and config machinery; only new dependency is the already-installed external `autoharness` binary (no new Go modules). |
| VII/VIII. Destructive Approval & Safety Modes | Force is operator-only, source-marked, reason-required, dual-audited; no destructive terminal command; strict-safety risky-action classification applied in hardening. |
| IX. Git-Friendly Persistence | Evidence is append-only JSONL, appended atomically under the task lock; no frontmatter churn on completion. |
| X. Context Efficiency | Structured `gate_report` + outcome fields returned to callers rather than raw logs. Note: `doctor --check-gate-evidence` performs a log scan in the first release (the indexed read-model query column is the separate follow-up `7ED9CE1A`); a query-backed check lands later, so this is aspirational for doctor in v1. |
| XI. Merge Commit Preservation | Not affected (no merge-strategy change). |

No justified violations. Force bypass is a sanctioned, audited operator capability,
not a constitutional violation.

## Security Considerations (design-sourced + review-driven, carried into subtasks)

- Argv-array execution only; never concatenate a shell command string (ST2.1).
- No arbitrary shell commands from config; backlogit invokes only the autoharness gate
  CLI, never gate commands from `.autoharness/config.yaml` (ST2.1/ST2.2).
- No untrusted-path interpolation into shell strings; fixed argv template with positional
  substitution (ST2.1).
- External-binary trust boundary: constrain `autoharness_binary` resolution (reject `..`
  traversal / absolute paths outside PATH/workspace), pass a minimal/allowlisted
  environment, and document the `.backlogit/hooks.yaml` trust boundary; `enabled: true`
  trusts an operator-managed autoharness install/config (ST1.1/ST1.3/ST2.1).
- Ref verification before invocation so a bad/attacker-influenced ref cannot silently
  fail-open to autoharness's empty-diff exit 0 (ST1.2).
- Ref-override is privileged: the head ref is pinned to the constant `HEAD` (not
  configurable), and a non-default base (`base_ref != auto` or a `--gate-base` that does not
  resolve to the default branch) is operator-only break-glass, audited via a
  `pre_task_completion_gate_base_override` event, never available on MCP — so the gate scope
  cannot be quietly narrowed to an empty diff outside the audited path (ST1.2/ST3.2/U4).
- Force bypass CLI-only, source-marked, reason-required, and audited; no MCP force surface;
  `force_cli_only: false` rejected at schema validation (ST1.1/ST3.4).
- Timeout bounds validated (min/max, non-positive rejected) so a bad config cannot hang the
  locked completion path as a DoS (ST1.1/ST2.1).
- Evidence is part of the transition contract under `evidence_required`: a failed backlogit
  audit append rolls back the transition (ST4.1).
- Truncate stderr for human output while preserving the full report for machine callers,
  with parity between CLI `--json` and MCP (ST3.2/ST3.3).

## Plan Hardening

**Hardening required: yes.** Triggers: public API/schema/contract change; security-
sensitive external process execution and an operator force bypass; hard external
dependency on autoharness `>= 1.4.7`; high rollout/rollback risk because the change
touches the core task/shipment completion path for every agent. This section deepens
verification, rollback, and guardrails and classifies the risky actions per the
`strict-safety` action contract. All actions are `planned` here — Ship executes; this
Stage artifact only records intent, risk, approval path, and expected closure.

### Learnings and instructions consulted

- `docs/compound/` searched for gate/exec/hook/force/lock/transition prior art — no
  directly relevant prior solution exists (confidence: low), so no prior resolution is
  contradicted or reused. The docline-frontmatter contract compound note governs the
  plan artifact itself (self-lint passed, 0 violations).
- `.github/instructions/constitution.instructions.md` (I, II, III/IV, VII/VIII, IX),
  `.github/instructions/strict-safety.instructions.md`, and
  `.github/instructions/backlogit.instructions.md`.

### Protected invariants (must not regress)

1. A repo with no configured autoharness lifecycle gates MUST still allow `-> done`
   (fail-open). Regression here breaks every agent's completion path.
2. On any gate refusal, the artifact file AND the DB row remain byte-for-byte unchanged
   (no partial completion). Mirror the rollback assertions in `archive_test.go`.
3. Agents MUST NOT be able to bypass gates: MCP move/update expose no force field, and
   the gate is an inline core completion service on the shared mutation path (not a
   CLI-only wrapper), so both CLI and MCP are gated identically.
4. The autoharness repeated-failure counter MUST NOT be double-counted by the completion
   path (advisory pre-checks use `--no-count`; the completion path is the authoritative
   counting caller).
5. External invocation is argv-array only; no shell string is ever constructed.

### Risky actions (ProposedAction / ActionRisk / ActionResult)

- **PA-1 — Insert a blocking inline gate service into `core.UpdateArtifact`/`ShipShipment`.**
  - targets: `internal/core/artifacts.go` (`UpdateArtifact` gains `TransitionOptions` + inline gate call around `FirePre`+`persistArtifact`), `internal/core/gate_transition.go` (lock+reread+decide+write), `internal/core/gate/broker.go` (orchestration over injected collaborators).
  - change_kind: shared-code + contract change (alters completion semantics for CLI and MCP; adds an entry-point parameter).
  - rollback: `enabled: false` kill-switch disables the broker with no code revert; code-level revert removes the inline call.
  - approval_required: yes (operator-attended staging; changes a core shared path).
  - ActionRisk: **high**. ActionResult: **planned**.
  - Note (review-driven): enforcement is an inline core service, NOT the generic pre-hook (which cannot hold the lock across the write nor redirect the transition).

- **PA-2 — Execute the external `autoharness` binary from the completion path.**
  - targets: `autoharness gate check --json` via new `GateRunner` (`internal/core/gate/runner.go`).
  - change_kind: external call / new process execution surface.
  - rollback: `enabled: false`; `auto` mode fail-open when the binary is unresolvable.
  - approval_required: yes (security-sensitive external exec).
  - ActionRisk: **high**. ActionResult: **planned**.
  - Guardrails: argv-array only; fixed argv template with positional substitution; timeout bound; no shell; no config-sourced command execution.

- **PA-3 — Operator force bypass of a validation gate.**
  - targets: `internal/cli/move.go`, `internal/cli/update.go` (`--force-gates`/`--force-reason`); autoharness `--force`; backlogit `pre_task_completion_gate_forced` event.
  - change_kind: privileged bypass / rollout control.
  - rollback: force is per-invocation and audited; no persistent state beyond the audit trail.
  - approval_required: yes — force is operator-only, requires a non-empty reason, and is dual-audited. No MCP surface.
  - ActionRisk: **high** (a deliberate gate bypass; not `destructive` because it neither deletes nor irreversibly mutates data, but it MUST remain operator-only and audited).
  - ActionResult: **planned**.

- **PA-4 — Backlogit-executed requeue/escalate transition driven by `repeated_failure`.**
  - targets: `internal/core/gate_transition.go` applying the decision's redirect (task -> `queued`/`blocked`) through the single locked write.
  - change_kind: durable state transition.
  - rollback: transitions are ordinary backlog moves, reversible via `backlogit move`; no second breaker is added.
  - approval_required: no (deterministic mapping of the autoharness signal; backlogit is the decided sole executor).
  - ActionRisk: **moderate**. ActionResult: **planned**.

- **PA-5 — Shipment-level two-level gating in `ShipShipment`.**
  - targets: `internal/core/shipment_lifecycle.go` (:136), `core` member-evidence validator + shipment-diff gate collaborators.
  - change_kind: shared-code + contract change on the release-finalization path.
  - rollback: refusal leaves shipment state unchanged; `enabled: false` disables enforcement.
  - approval_required: yes (touches release finalization for all shipments).
  - ActionRisk: **high**. ActionResult: **planned**.

No `ActionRisk: destructive` actions are planned. Evidence is append-only; doctor is
advisory. Because no destructive action exists, no destructive-approval halt is
triggered; the `high` actions above are gated by operator-attended staging plus the
`enabled: false` kill-switch and `auto` fail-open default.

### Deepened verification (beyond Runtime Verification section)

- Environment prechecks: assert `autoharness --version >= 1.4.7` and that `gate check
  --json` emits `repeated_failure` before enabling `true` mode in any integration test.
- Target scenarios (must be proven before absorption): (a) no-config fail-open allows
  `-> done`; (b) real failing gate refuses with unchanged file+DB; (c) `auto` fail-open
  on absent binary; (d) `true` fail-closed on absent/incompatible binary; (e) forced CLI
  completion emits both audit records; (f) shipment refused when a member lacks evidence;
  (g) concurrent completion — one holds the lock, the other returns the retryable error.
- Blocked-path handling: a retryable "gate in progress" error must be distinguishable
  from a `GateBlockedError` (different exit/error class) so callers retry vs. self-heal
  correctly.

### Operational closure (deepened)

- Monitoring signals: volume of `pre_task_completion_gate_blocked`, `_forced`,
  `_requeued`, `_escalated` events; rate of retryable "gate in progress" errors.
- Rollback trigger: any completion refusal in a repo WITHOUT configured gates (should be
  impossible under `auto` fail-open) -> immediately set `enabled: false` and investigate.
- Rollback procedure: set `lifecycle.pre_task_completion_gate.enabled: false` in
  `.backlogit/hooks.yaml` (no redeploy needed); code revert removes the hook registration
  in `internal/core/workspace.go`.
- Owner: gate-broker feature owner (assigned at Ship claim). Validation window: one full
  Stage -> Ship cycle post-enablement, watching the monitoring signals above.

### Unresolved operator decisions blocking safe execution

None. All design decisions are resolved (Q1-Q4 + adjacent + shipment-gate + requeue
ownership) and the autoharness `>= 1.4.7` dependency is satisfied and verified. Staged
enablement (ship with default `auto`, flip to `true` only after the validation window)
is the recommended rollout and is captured in U5 runbooks.

<!-- plan-review-attempt: 1 -->

## Plan Review

### Attempt 1 — Gate: FAIL (2026-07-06)

Multi-persona review (Constitution, Scope Boundary, Go, Architecture Strategist,
Security Lens, Agent-Native Parity). Hardening required: yes — a `## Plan Hardening`
section is present with `ProposedAction`/`ActionRisk` classification (strict-safety),
so the hardening-missing FAIL condition did not apply. The gate FAILED on P0/P1
findings, all resolved in the revised plan body above (this is why the plan sections
carry "review-driven" annotations).

**P0 (blocking) — resolved:**
- *Import cycle* `core -> gate -> core` and *subpackage access to `core` unexported
  helpers* (`lockTaskFile`, `appendItemEvent`, `findArtifact`, `persistArtifact`).
  Resolution: one-way `core -> gate` boundary — `internal/core/gate` owns only the
  autoharness integration (runner/resolver/probe/parse/pure types), depends on no
  `core` internals; lock/write/evidence/shipment/doctor live in `core`, which injects
  adapters. Typed errors in the `internal/errors` leaf. (See "Package boundary".)

**P1 (blocking) — resolved:**
- *Generic pre-hook cannot hold the lock across the write nor redirect the transition;
  "no signature change" is false* (Go, Architecture, Constitution). Resolution: inline
  core completion service (lock -> reread -> gate -> decide -> single locked write ->
  evidence -> release) invoked by `UpdateArtifact`/`ShipShipment`, returning an explicit
  `GateDecision`; entry points gain `TransitionOptions`. (See "Enforcement mechanism",
  ST2.3-ST2.5.)
- *Requeue/escalate redirect + reentrancy self-deadlock* (Go). Resolution: broker returns
  a decision the caller applies to one write; no nested locked write; no map aliasing.
- *`gate` package too wide (god-object) + CLI options leaking via context/mutable state*
  (Architecture). Resolution: narrowed `gate` package + request-scoped `TransitionOptions`.
- *Ref-error fail-open suppression* (Security). Resolution: verify base/HEAD refs before
  invocation; unverifiable ref while enforced -> config error (ST1.2).
- *External-binary/env trust boundary* (Security). Resolution: constrain `autoharness_binary`
  resolution, minimal env, documented hooks.yaml trust boundary (ST1.1/ST1.3/ST2.1).
- *`evidence_required` not transactional* (Security). Resolution: evidence append is part
  of the transition contract; failure rolls back the transition (ST4.1).
- *Retryable "gate in progress" indistinguishable from a block* (Parity, Go, Constitution).
  Resolution: distinct `ErrGateInProgress`, exit code `8`, and MCP `gate_in_progress`
  retryable error (ST3.1/ST3.3).
- *No synchronous response contract for requeue/escalate; CLI "task remains" wording wrong*
  (Parity). Resolution: CLI `--json` + MCP carry `outcome`/`new_status`/`repeated_failure`;
  human wording reports the actual post-transition status (ST3.2/ST3.3).

**P2 (addressed):** `GateResult` no longer embeds a not-found sentinel (uses `errors.Is`/
`exec.ExitError`); timeout detected via `ctx.Err()`, not exit `-1`; full versioned
exit-code table (6/7/8) verified against doctor's 0-4; typed errors placed in
`internal/errors`; `_error` evidence event for setup/config/timeout refusals; `timeout_seconds`
bounds validated; `force_cli_only: false` rejected + CLI `ForceSource` marker; MCP/CLI
machine-context parity for the report; shipment gating reconciled with `ShipShipment`
(no ungated auto-completion); YAGNI `min_autoharness_version` config field dropped (code
constant); subtask sizing re-split (ST2.2 mapper vs ST2.3 wiring; ST3.1 exit-table vs ST3.2
mapping) to hold the 2-hour envelope.

**P3 (addressed/acknowledged):** Constitution X doctor claim tempered (log scan in v1 pending
the `7ED9CE1A` read-model); atomic evidence append under lock stated; `autoharness_binary`
traversal rejection; agent non-force escape hatch (repair+retry or move to non-terminal
status) to be documented in U5 MCP docs.

Scope Boundary and Constitution personas returned no P0/P1 (only P2/P3 sizing/observability
items, addressed above). Zero scope creep; all five design units and all requirements
remain design-grounded.

<!-- plan-review-attempt: 2 -->

### Attempt 2 — Gate: FAIL (2026-07-06)

Re-review by the five personas that owned the attempt-1 P0/P1 (Go, Architecture, Security
Lens, Agent-Native Parity, Constitution). **Constitution -> PASS.** All attempt-1 P0/P1
findings were verified RESOLVED (import cycle, inline-service redesign, gate-package narrowing,
options object, ref verification, trust boundary, transactional evidence, gate-in-progress
class, response contract). The gate FAILED on FOUR NEW P1 findings surfaced by the deeper
redesign — convergent, not recurring — all resolved in the revised plan body above:

**New P1 (blocking) — resolved:**
- *Lock-lifetime vs stale-TTL collision* (Go). A gate may run up to `timeout_seconds` (600s)
  while `taskStaleLockTTL` is 60s and backlogit runs as per-invocation subprocesses, so a
  concurrent cross-process caller could treat the live lock as crash residue, reap it
  mid-gate, and let two processes both write `done`. Resolution: new ST2.3 adds a lock-sidecar
  ModTime heartbeat (interval `< taskStaleLockTTL`) held for the whole gate, plus config
  validation that `timeout_seconds` stays under the TTL unless heartbeating; concurrency test
  proves no stale reap across a `>TTL` gate.
- *Single authoritative mutation path under-specified at the CLI/MCP boundary* (Architecture).
  Resolution: "Sole durable-mutation owner" preamble + ST3.2/ST3.3 assert the CLI and MCP
  adapters perform NO post-core completion/status mutation and only render exit codes / MCP
  bodies from the single core result contract.
- *Unaudited ref-override bypass* (Security). A configurable `head_ref` and an unaudited
  `--gate-base` could select refs collapsing the diff to empty and pass exit 0 outside the
  audited force path. Resolution: head ref pinned to the constant `HEAD` (dropped as config);
  non-default base is operator-only CLI break-glass, audited via
  `pre_task_completion_gate_base_override`, never on MCP (ST1.1/ST1.2/ST3.2/U4).
- *MCP outcome-class contract incomplete* (Parity). CLI had a full 6/7/8 trichotomy but MCP
  only defined `gate_blocked`+`gate_in_progress`, so config/setup and timeout collapsed to
  `internal` — the very class an agent must escalate rather than self-heal. Resolution: ST3.3
  now defines distinct `gate_config`/`gate_setup` (non-retryable, operator-remediation) and
  `gate_timeout` (retryable) error types, routed in `domainError` so none fall through to
  `internal`.

**P2/P3 (addressed):** redirect outcomes given distinct `gate_requeued`/`gate_escalated`
types + `state_changed:true` so an agent cannot mistake a completed redirect for a no-op
block; pass path surfaces `outcome: passed` + evidence ref on BOTH CLI `--json` and MCP;
evidence append re-ordered BEFORE the status write with a new error-returning appender
(the current `appendItemEvent` is void); `allowed_next_actions` hint added to the
`gate_blocked` MCP payload so the escape hatch is discoverable in-band; ST2.3 split out of
the inline-write subtask to hold the 2-hour envelope (U2 is now ST2.1-ST2.5).

<!-- plan-review-attempt: 3 -->

### Attempt 3 — Gate: FAIL -> resolved in-plan (2026-07-06)

Re-review by the four personas that owned the attempt-2 P1s. **Go -> PASS, Security -> PASS,
Parity -> PASS** (all four attempt-2 P1s verified resolved; no new P0/P1 from Go, Security,
or Parity — only P2/P3 implementation-hardening advisories carried into Unit 2). The gate
FAILED on ONE narrow new P1 from Architecture, since resolved in the plan body:

**New P1 (blocking) — resolved:**
- *Below-threshold block hard-coded the retained status to `active`* (Architecture). The
  model defines non-terminal statuses `queued|active|blocked|review` (verified in
  `internal/models/artifact.go:15-19`), so a `-> done` attempt can originate from `review`;
  reporting "task remains `active`" (ST3.2) or "stay `active`" (ST2.5) would misreport the
  retained status and set a wrong `new_status`. Resolution: the block outcome now RETAINS the
  reread `old_status` (captured under lock in ST2.3) — the core result contract carries
  `old_status` and sets `new_status == old_status` on a block; CLI/MCP report the actual
  retained status (`active` OR `review`), with tests covering a block from each
  (ST2.5/ST3.2). No status is assumed.

**Advisories (non-blocking; recorded for Ship/Unit 2 execution, not gate-blocking):**
- Go P2: bound the heartbeat goroutine lifecycle to the deferred lock release (stop
  channel / WaitGroup) so the ticker cannot outlive the hold or race `os.Remove` at release.
- Go P2: make `taskStaleLockTTL`/heartbeat interval injectable so the `>TTL` concurrency
  test is deterministic and fast (avoid a 60s wall-clock test).
- Go P2: an append-only JSONL cannot literally "roll back"; use write-ahead-intent/commit
  pairing or a compensating `_error` event if `persistArtifact` fails after the evidence
  append. Keep the evidence-before-write ordering but make the failure semantics explicit.
- Go P2: adding a required `TransitionOptions` param to `UpdateArtifact`/`ShipShipment` forces
  all call sites (CLI + MCP) to change in the same build; use a variadic/functional-option
  shim OR schedule the call-site edits within the same buildable increment as ST2.4.
- Go/Spec P3: reconcile ST1.1 ("unless heartbeat mode") vs ST2.3 ("heartbeat is the
  always-on guard") — state whether the heartbeat is unconditional (timeout<TTL cap becomes
  belt-and-suspenders) or a toggle.
- Parity P3: extend the CLI `--json` `outcome`/`error_class` enum to include
  config/setup/timeout/in_progress (MCP is now richer than CLI for these classes), and make
  explicit that CLI agents MUST read `--json state_changed`/`new_status` to distinguish a
  redirect from a block (CLI exit 6 is shared across block/requeue/escalate).

### Gate cycle status (P-005 boundary)

Attempt 3 reached the `<!-- plan-review-attempt: 3 -->` counter, which is the maximum of two
re-entry cycles (attempts 2 and 3) after the initial attempt. Per the plan-review cycle rule,
Stage HALTS for operator intervention rather than auto-running a fourth review. The sole
attempt-3 P1 has been resolved in-plan with a deterministic, verified fix (grounded in the
confirmed status model); Go/Security/Parity are already PASS. The operator decision required
before harvest is recorded in the session narrative: accept this revised plan as the gate
outcome, or authorize one confirmation re-review of the retained-status fix.

<!-- plan-review-attempt: 4 -->

### Attempt 4 — Operator-authorized confirmation pass (2026-07-06)

Operator selected Option B (one confirmation re-review) rather than accept a fixed-but-
unreviewed plan on this security-sensitive core-lifecycle feature. Scope: confirm the
attempt-3 retained-status fix and re-confirm Go/Security/Parity remain PASS.

- **Go -> PASS.** The block reports the ST2.3 post-lock reread `old_status` (authoritative,
  no new TOCTOU); block performs no durable write so "retain" = "do not mutate"; no new
  import cycle / reentrancy / lock issue.
- **Security -> PASS.** No control regressed — head still pinned to `HEAD`, base-override
  still audited break-glass, force still CLI-only; `new_status == old_status` only on true
  blocks, so a state change cannot be masked as a no-op (redirects still carry distinct
  outcome/type + changed `new_status`).
- **Parity -> PASS.** `old_status`/`new_status`/`state_changed` are symmetric across CLI
  `--json` and MCP `gate_blocked`; all outcome classes remain distinct; no new asymmetry.
- **Architecture -> FAIL, but PARTIAL on the SAME retained-status finding (no new P0/P1).**
  ST2.5/ST3.2/ST3.3/R7 were all fixed correctly, but ST2.2's pure decision-mapper
  description still hard-coded "stays `active`" on below-threshold block, contradicting the
  corrected contract. The reviewer explicitly stated "New P0/P1 introduced by the fix:
  none — the only blocker is the stale ST2.2 contradiction," and prescribed the exact fix:
  make ST2.2 status-agnostic on block and defer retained/`new_status` to the ST2.3 reread +
  ST2.5 contract.

**Resolution applied (completion of the already-approved fix, NOT a new decision):** ST2.2's
below-threshold `block` row is now status-agnostic — it emits `block` (refuse, no write) and
explicitly states the retained status is resolved by the ST2.3 reread `old_status` and the
ST2.5 result contract, because the pure mapper has no view of the artifact's current status.
A full-file sweep confirms no other hard-coded `active` retained-status remains. Re-lint: 0
violations. Go/Security/Parity are PASS; the Architecture FAIL was solely the stale mapper
line, now consistent with the corrected contract the reviewer described as correct.

**Gate status:** This attempt-4 finding is the SAME retained-status finding incompletely
propagated (a one-line stale contradiction), not a new substantive P0/P1. Per the operator's
attempt-4 mandate — harvest only on a clean PASS; on any non-PASS report to the operator and
do NOT run a 5th attempt — Stage does NOT self-harvest here. The completion fix is applied and
recorded; the operator decision (accept the completed fix and proceed to harvest, or take
another action) is pending. No 5th review attempt will be auto-run.
