---
chunk_strategy: h1-h2-h3
description: 'Pre-merge operational closure for shipment 082-S — pre-task-completion gate broker (082-F). Consolidates the delivery: 5 units (U1 config/context/base-ref/version-probe, U2 core transition broker, U3 CLI exit codes + MCP structured errors, U4 evidence seam + shipment two-level gating + advisory doctor, U5 docs) implemented TDD, all tasks+subtasks done and archived. CI 4/4 green at HEAD (test 1.23, test 1.24, CLI Reference Drift, Docline frontmatter gate). Mandatory pre-push multi-model adversarial review (Gemini 3.1 Pro / GPT-5.4 / Claude Opus) + independent Security Reviewer: no all-3-model HIGH blockers; 5 findings (1 P1 RCE-hardening, 2 P2, 2 MED) remediated pre-push in commit a2a8b7b; 4 LOW/by-design findings stashed. Copilot review: 3 valid comments (update --json section drop, gate_exit next-actions parity, dead runtime import) all fixed in 1f496c1, replied, and threads resolved; fresh Copilot review awaited on HEAD before merge-ready per §1.9. Runtime verification PASS against real autoharness 1.4.7 (fail-open compose, fail-closed exit 7, logs-only evidence, structured JSON, update+section regression). Invariants: argv-array exec only, no untrusted-path interpolation, bare-PATH binary only, MinimalEnv on all subprocesses, timeout wraps probe+base+run, logs-only evidence, force CLI-only + audited, no MCP force field, fail-open under auto / fail-closed under true. Deployment path merge-only (Go CLI/MCP + docs; no migration/service). Rollback = git revert of the additive feature commits. Readiness READY WITH CONDITIONS — operator P-014 merge approval + merge-commit strategy (P-009) + a fresh Copilot review returning zero unresolved threads on HEAD. NOT merged this run (P-014 / Principle VII).'
doc_type: closure
docline:
    ms.date: 2026-07-06T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-06T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-06-pre-task-completion-gate-broker-closure.md
title: 082-S pre-task-completion gate broker — Pre-Merge Operational Closure
---

# Operational Closure — 082-S pre-task-completion gate broker

**Mode**: `pre-merge`
**Context**: PR #178 · branch `feat/pre-task-completion-gate-broker` · shipment 082-S / feature 082-F (5 tasks + 17 subtasks).
**Verification report**: `docs/closure/2026-07-06-pre-task-completion-gate-broker-runtime-verification.md`
**Adversarial review**: `docs/closure/2026-07-06-pre-task-completion-gate-broker-adversarial-review.md`

## Summary of change

Backlogit now synchronously invokes `autoharness gate check` before a task/subtask
enters a terminal status and before a shipment is marked `shipped` (two-level gating).
The broker is an inline core completion service on `UpdateArtifact`/`ShipShipment`, so
every caller (CLI + MCP) is gated with no bypass. Autoharness remains the sole owner of
the gate list; backlogit never parses autoharness gate config. Three-valued `enabled`
(auto/true/false): fail-open under `auto`, fail-closed under `true`, kill-switch under `false`.

## CI status

4/4 green at HEAD `1f496c1`:

| Check | Result |
|---|---|
| test (1.23) | pass |
| test (1.24) | pass |
| CLI Reference Drift | pass |
| Docline frontmatter gate | pass |

## Review status

* **Standard + multi-model adversarial (pre-push, MANDATORY)** — no all-3-model HIGH-confidence blockers; locked security invariants confirmed holding. 5 findings remediated pre-push (commit `a2a8b7b`): binary-path RCE hardening (Security P1 0.98), probe/base timeout coverage (P2 0.95), version-probe MinimalEnv (P2 0.9), forced-evidence audit-integrity refusal (F2), explicit-`--gate-base` audit completeness (F3). 4 LOW/by-design deferred → stashed (see below).
* **Copilot** — 3 valid comments on the current HEAD, all fixed in `1f496c1`, replied, threads resolved: (1) `update.go` `--json` dropped `--section` writes behind an early return; (2) `gate_exit.go` offered `allowed_next_actions` for redirect outcomes, diverging from the MCP contract; (3) `runner.go` dead `runtime.GOOS`/import. A fresh Copilot review on HEAD returning zero unresolved threads is a merge condition (§1.9).

## Runtime verification

**PASS** against the installed autoharness 1.4.7 — real composed gate path exercised
(fail-open compose, fail-closed exit 7 with retained status, logs-only evidence, structured
JSON, update+section regression). See the runtime-verification artifact for full evidence.

## Invariants to preserve (must not regress)

* argv-array exec only; never a shell string; no untrusted-path interpolation.
* `autoharness_binary` must be a bare PATH-resolved name (absolute / `..` / path-qualified rejected at config validation).
* All subprocesses (gate, git base resolution, version probe) run with an allowlisted MinimalEnv and are bounded by the timeout deadline (no lock-pinning DoS).
* Evidence written to item logs only, never frontmatter.
* Fail-open under `auto`; fail-closed under `true`; kill-switch under `false`.
* Force is operator-CLI-only, audited, and refuses under `evidence_required` if the audit append fails; no MCP force field.
* No partial completion — a refusal retains the re-read prior status.

## Pre-deploy audits

* None required beyond CI. No DB migration, no schema change, no service. `hooks.yaml` gains an optional `lifecycle.pre_task_completion_gate` section that defaults to `enabled: auto` (backward-compatible: absent config = auto = fail-open when autoharness is unresolved).

## Deployment / rollout path

**Merge-only.** Pure Go CLI/MCP behavior + docs. No canary, no migration, no maintenance window. The feature is inert unless a workspace opts into `enabled: true`/`auto` with autoharness present and gates configured.

## Post-deploy checks

1. `backlogit move <task> --status done` on a real gated workspace → confirm the gate runs and evidence is recorded.
2. Confirm a workspace with no `pre_task_completion_gate` config still completes tasks (auto fail-open).
3. Confirm `enabled: true` + missing autoharness → exit 7, status retained.

## Monitoring plan (release-observability)

* **SLIs**: gate pass rate, gate block/requeue/escalate counts, gate invocation latency (probe + base + run within timeout), exit-7/8 (setup/retryable) rate.
* **Signals to watch**: item-log `pre_task_completion_gate_*` events (`_passed`/`_blocked`/`_requeued`/`_escalated`/`_forced`/`_base_override`/`_error`).
* **Dashboards/alerts**: none provisioned in-tree (single-binary local tool); observation is via item logs and CLI exit codes.

## Healthy signals

* Gated completions succeed with `outcome:passed` and a recorded `gate_report_hash`.
* Workspaces without gate config are unaffected (auto fail-open).
* No secret/env leakage into subprocesses (MinimalEnv holds).

## Failure signals (rollback triggers)

* A gated completion hangs past the timeout while holding the task lock (would indicate the probe/base/run timeout regression returned).
* `enabled: true` with autoharness present passes a task that should have been blocked (fail-open leak).
* Any gate subprocess observed running with the full ambient environment (MinimalEnv regression).
* `autoharness_binary` accepting a path-qualified value (RCE-hardening regression).

## Rollback procedure

`git revert` the additive feature commits on `main` (`d4db750`, `06ef77b`, `7b4cfd2`,
`940cb8a`, `985d994`, `a2a8b7b`, `1f496c1`) — the gate is purely additive; reverting
restores the pre-082-F ungated completion path. Alternatively set
`lifecycle.pre_task_completion_gate.enabled: false` (kill switch) to disable the broker
without reverting code.

## Validation window & owner

* **Window**: observe the next few real gated completions and the first shipment ship after merge.
* **Owner**: repository maintainer (softwaresalt) / operator on-call for the 082-S rollout.

## Follow-up (stashed for Stage)

| Stash ID | Finding | Priority |
|---|---|---|
| 162F5548 | F1 — warn when both `hooks.yaml base_ref` and `--gate-base` are set (precedence is config-first by design) | low |
| 9822F787 | F4 — shipment member-evidence accepts `ran=false` fail-open passes; consider requiring `ran=true` | low |
| 7C5EADA6 | F5 — shipment-level DecisionError collapses to GateBlockedError, losing exit 7/8 class | low |
| 83B885EE | F7 — `move --json` emits no payload for the `*GateError` class (parity with `*GateBlockedError`) | low |

Related existing stash: `7ED9CE1A` (strict index-backed gate-evidence check).

## Readiness: READY WITH CONDITIONS

Proceed to merge only when **all** conditions hold:

1. Operator **P-014** merge approval (operator is ATTENDING; approves this feature individually).
2. **P-009** merge-commit strategy (not squash/rebase).
3. A **fresh Copilot review on HEAD** returns zero unresolved threads (§1.9 gate).

**Not merged this run** (P-014 / Principle VII). PR #178 is presented merge-ready; the Ship agent HALTS for operator approval.
