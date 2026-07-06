---
chunk_strategy: h1-h2-h3
description: 'One durable reliability rule graduated from 082-S — every external-process invocation on a lock-holding critical path must be wrapped in a bounded context timeout, and the FIRST such invocation is usually a version/capability probe. If the probe itself is not timeout-bounded, a hung, malicious, or stdin-reading child process blocks task completion indefinitely while the workspace lock is held, converting a health check into a denial-of-service. Bound the probe before you bound the "real" call.'
doc_type: learning
docline:
    date: 2026-07-06T00:00:00Z
    severity: high
    tags:
        - reliability
        - dos
        - timeout
        - exec
        - context
        - probe
        - locking
        - gate
        - core
schema_version: "1.0"
source: docs/compound/2026-07-06-external-process-timeout-before-probe.md
title: 'Bound the version/capability probe with a timeout, not just the main call — an unbounded probe on a lock-holding path is a DoS (082-S)'
---

# Timeout the Probe, Not Just the Main Call

One durable reliability rule graduated from shipment 082-S (feature 082-F,
"Pre-Task-Completion Gate Broker", PR #178, merge
`e47e1291c49f906a4b257c60f117a2cd05107db7`). Surfaced during the mandatory
pre-push adversarial review (probe/base timeout coverage, P2 / 0.95) and
remediated before push.

## Rule — The first external-process call (the probe) must be timeout-bounded, because it runs while the lock is held and gates everything after it

### Problem

The gate broker's flow before completing a task is:

1. Acquire the workspace lock.
2. **Probe** `autoharness version` / contract to confirm `>= 1.4.7`.
3. Run `autoharness gate check --json ...`.
4. Map the exit code, record evidence, complete the transition.
5. Release the lock.

The main `gate check` call was wrapped in `exec.CommandContext` with a
`timeout_seconds` deadline. But the **version probe** in step 2 was initially run
without its own deadline. That is the more dangerous omission:

- The probe runs **first** and **inside the held lock**, so if it hangs, nothing
  downstream ever runs and the lock is never released.
- A binary that reads stdin, deadlocks, or is deliberately slow turns a "quick
  health check" into an unbounded stall on the critical completion path.
- Because the probe gates the real call, an unbounded probe defeats the timeout
  you carefully put on the real call.

### Fix

The version/contract probe (`ExecVersionRunner`), the gate check (`ExecRunner`),
and the git base-ref resolution runner (`ExecGitRunner`) all run through
`exec.CommandContext` with a bounded, configurable deadline (`timeout_seconds`,
sensible default), so **none can hang unbounded under the lock** — that is the
load-bearing property.

The *classification* of a deadline kill differs by runner, and the docs must be
precise about it:

- **Gate check (`ExecRunner`)** maps `context.DeadlineExceeded` to
  `ErrGateTimeout` — a typed, **retryable** error (fail-fast).
- **Probe (`ExecVersionRunner`)** returns a plain run error on deadline, which
  flows through `Probe`/`failProbe`: under `enabled: true` it is classified as a
  **setup** error (`GateError{Class:"setup"}`, fail-**closed**); under
  `enabled: auto` it fails **open** (enforce=false). It is *not* surfaced as a
  retryable timeout.
- **Base-ref runner (`ExecGitRunner`)** is likewise deadline-bounded but does not
  emit `ErrGateTimeout`.

The lesson is unchanged: bound the probe so it cannot stall the lock. Just do not
overstate that every timeout is "retryable" — only the gate-check runner's is.

### Why "probe first" makes this subtle

It is natural to reason "the expensive call is the gate check, so that's what
needs the timeout." But ordering matters more than cost: **whatever runs first
under the lock is the true availability chokepoint.** A cheap-looking probe that
hangs is worse than a slow main call that is already bounded, because the probe
sits upstream of the deadline you added.

## Generalization

For any inline, lock-holding integration with an external process:

- Put a bounded `context.WithTimeout` on **every** child exec, starting with the
  **first** one (probe/handshake/capability check).
- Prefer a short probe timeout and a separate (possibly longer) work timeout.
- On timeout, kill the child and classify deliberately: a retryable error for the
  work call so transient hangs don't become permanent stalls, or a fail-closed
  setup error for a required capability probe (as the gate broker does under
  `enabled: true`). Either way the child is killed — never left hanging.
- Never hold a lock across an unbounded wait — bounded-wait with fail-fast retry
  is the pattern (here ~2–5s bounded-wait on lock contention, re-read after lock).
- Unit-test the timeout path with a runner that never returns, asserting the call
  returns within the deadline.

## Related

- `docs/design-docs/2026-07-04-pre-task-completion-gate-broker.md` — lock held
  through the entire gate; no partial completion; bounded-wait retryable on
  contention.
- `docs/compound/2026-07-06-exec-binary-config-must-be-bare-path-validated.md` —
  the RCE sibling lesson on the same exec seam.
- `docs/compound/2026-07-06-autoharness-gate-broker-integration-contract.md` — the
  full backlogit↔autoharness contract the probe defends.
