---
chunk_strategy: h1-h2-h3
description: 'Post-merge operational closure for shipment 082-S — pre-task-completion gate broker (082-F). PR #178 merged into main via a true 2-parent merge commit e47e1291c49f906a4b257c60f117a2cd05107db7 (merge strategy preserved per P-009; admin-bypassed the human-approval ruleset under explicit operator P-014 approval). Shipment 082-S closed via backlogit shipment ship: shipment_status=shipped, 24 archived_ids (feature 082-F + 5 tasks + 17 subtasks + shipment), merge SHA recorded, 0 returned/blocked. shipment-reconcile pre (all 23 items pre-archived, PROCEED) and post (23/23 in archive, 0 deletions, PROCEED) both passed; P-007 archive integrity clean. Knowledge graduation: design doc already resident in docs/design-docs/; 3 durable compound learnings added (bare-PATH exec-binary validation / RCE, timeout-before-probe DoS, autoharness gate-broker integration contract), all docline-clean; compound-refresh classified all 9 existing entries KEEP (no supersession). Deployment path merge-only (Go CLI/MCP + docs; gate is opt-in via lifecycle.pre_task_completion_gate.enabled auto/true/false; no migration/service/rollout). Rollback = git revert of the merge commit. Readiness READY (merged). Closure work isolated on post-merge/082-S branch; closure PR awaits its own operator P-014 approval per §1.10 — NOT merged this run.'
doc_type: closure
docline:
    ms.date: 2026-07-06T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-06T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-06-pre-task-completion-gate-broker-post-merge-closure.md
title: 082-S pre-task-completion gate broker — Post-Merge Operational Closure
---

# Post-Merge Operational Closure — 082-S pre-task-completion gate broker

**Mode**: `post-merge`
**Context**: PR #178 **MERGED** · shipment 082-S / feature 082-F (5 tasks + 17 subtasks) · closure work on branch `post-merge/082-S`.
**Feature merge commit**: `e47e1291c49f906a4b257c60f117a2cd05107db7` (2-parent merge commit — parents `59cdc6d` prior-main + `a24751b` feature HEAD; merge strategy preserved per P-009).
**Merge authorization**: explicit operator **P-014** approval + authorized admin bypass of the `PR-Review` human-approval ruleset (`--admin`). `--delete-branch` removed the feature branch.
**Verification report**: `docs/closure/2026-07-06-pre-task-completion-gate-broker-runtime-verification.md` (PASS, real autoharness 1.4.7).
**Adversarial review**: `docs/closure/2026-07-06-pre-task-completion-gate-broker-adversarial-review.md`.
**Pre-merge closure**: `docs/closure/2026-07-06-pre-task-completion-gate-broker-closure.md`.

## Summary of change

backlogit now synchronously invokes `autoharness gate check` before a task/subtask
enters a terminal status and before a shipment is marked `shipped` (two-level
gating), as an inline core completion service on `UpdateArtifact`/`ShipShipment`.
Every caller (CLI + MCP) is gated with no bypass. Three-valued `enabled`
(auto/true/false): fail-open under `auto`, fail-closed under `true`, kill-switch
under `false`. Merged and live on `main`.

## Merge confirmation

- `gh pr view 178 --json state,mergedAt,mergeCommit` → `state: MERGED`,
  `mergedAt: 2026-07-06T22:35:11Z`, `mergeCommit.oid: e47e1291…`.
- `git show --no-patch` confirms **2 parents** (`59cdc6d7…` + `a24751b0…`) →
  true merge commit, **not** squash/rebase (P-009 preserved).
- `git merge-base --is-ancestor e47e1291… origin/main` → exit 0: merge is in
  `origin/main` history. **MERGE_CONFIRMED.**

## Shipment closure

- `backlogit shipment ship 082-S --sha e47e1291…` → `shipment_status: shipped`,
  **24 archived_ids** (082-F + 5 tasks + 17 subtasks + 082-S), `returned_ids: []`,
  `commit_sha` recorded.
- Covering feature 082-F moved `done` → archive at closure (all child tasks were
  already done+archived during build).
- **shipment-reconcile pre** (`expected_status: done`): all 23 manifest items
  `pre-archived`, 0 orphans/missing/mismatch → **PROCEED**
  (`.backlogit/reconcile/082-S-pre-2026-07-06-153830.md`).
- **shipment-reconcile post**: 23/23 manifest items present in archive + shipment
  file archived, 0 git deletions → **PROCEED**
  (`.backlogit/reconcile/082-S-post-2026-07-06-153935.md`).
- **P-007 archive integrity**: `git status -- .backlogit/archive/` reported **0
  deletions** — no `git restore` required.

## Invariants to preserve (must not regress)

1. **argv-array exec only** — never a shell string; base-ref and paths passed as
   discrete argv elements.
2. **Bare-PATH binary only** — `validateGateBinary` rejects path-qualified
   `autoharness_binary`; resolution via PATH (RCE containment).
3. **MinimalEnv allowlist** on every subprocess (gate check, version probe, git
   base-ref runner).
4. **Timeout wraps every child exec**, including the version probe that runs first
   under the lock.
5. **Logs-only evidence** — item JSONL only, never frontmatter; doctor
   `--check-gate-evidence` advisory-only.
6. **Force is CLI-only + audited** (`--force-gates --force-reason`), no MCP force
   field, `force_cli_only:false` rejected at validation.
7. **fail-open under `auto` / fail-closed under `true`**; broker unwired under
   `false`.
8. **One-way `core → gate` boundary** — `internal/core/gate/*` never imports
   `internal/core`.
9. **backlogit never parses autoharness gate config**; min autoharness `>= 1.4.7`
   enforced by contract probe.

## Pre-deploy audits

None required — additive Go CLI/MCP + docs change. No schema migration, no service
config, no access changes. The gate is **opt-in**: absent a
`lifecycle.pre_task_completion_gate` config with `enabled: true`/`auto` + a present
autoharness, behavior is unchanged.

## Deployment / rollout path

**Merge-only.** Ships as part of the backlogit binary; consumers pick it up on
their next build/pull of `main`. No canary, phased rollout, or maintenance window.

## Post-deploy checks

1. `backlogit --version` builds clean from `main` at `e47e1291…`.
2. In a workspace **without** the gate config: task/shipment completion behaves
   exactly as before (no autoharness invocation).
3. In a workspace **with** `enabled: auto` + autoharness 1.4.7 present: a passing
   gate lets `move --status done` complete and records logs-only evidence; a
   blocking gate refuses with exit 6 and returns the report.
4. With `enabled: true` and autoharness **absent**: completion fails closed
   (setup error) — confirmed in runtime verification (exit 7, task retained).

## Healthy signals

- Completion latency unchanged when gate disabled/`auto` + no autoharness.
- Gate pass path records exactly one logs-only evidence event per completion.
- CI 4/4 green on `main` (test 1.23, test 1.24, CLI Reference Drift, Docline gate).

## Failure signals (investigate / consider rollback)

- Task completions hanging (would indicate a timeout-bounding regression on an
  exec seam).
- `move --status done` refused in a workspace that never opted into the gate
  (would indicate the `enabled != "false"` wiring guard regressed).
- Evidence written to frontmatter instead of logs (invariant #5 regression).
- Any `autoharness_binary` path-qualified value being accepted (invariant #2 / RCE
  regression).

## Monitoring plan

- Watch item JSONL logs for `pre_task_completion_gate_*` events and
  `pre_task_completion_gate_forced` audit events.
- Watch CI on `main` for the four required checks staying green.
- No dashboards/alerts — CLI tool, observation is log- and CI-based.

## Rollback

- **Trigger**: a confirmed completion-path hang, a fail-closed regression blocking
  legitimate completions, or any accepted path-qualified binary (RCE).
- **Procedure**: `git revert -m 1 e47e1291c49f906a4b257c60f117a2cd05107db7` (revert
  the merge commit, first-parent mainline) on a branch, PR, and merge. The feature
  is purely additive, so revert is clean; no data migration to unwind. As an
  interim mitigation, operators can set
  `lifecycle.pre_task_completion_gate.enabled: false` to disable the broker without
  reverting code.

## Validation window & owner

- **Window**: 7 days of normal backlogit usage on `main`.
- **Owner**: Derek Williams (softwaresalt) / backlogit maintainers.

## Risky action record

| Action | Risk | Approval | Result |
|---|---|---|---|
| Admin-bypass merge of PR #178 over the `PR-Review` ruleset | MEDIUM (bypasses required-review gate) | Explicit operator P-014 approval | Merged as 2-parent merge commit `e47e1291…`; P-009 strategy preserved |
| `backlogit shipment ship 082-S` (dev/archival mutation on `.backlogit`) | LOW (backlog state only, reversible via git) | Ship Step 6 protocol | 24 items archived; reconcile pre+post PROCEED; P-007 clean |

## Knowledge graduation

- **Design doc**: `docs/design-docs/2026-07-04-pre-task-completion-gate-broker.md`
  already resident in `docs/design-docs/` — no graduation move needed.
- **Compound learnings added** (all docline-clean, 0 violations):
  - `docs/compound/2026-07-06-exec-binary-config-must-be-bare-path-validated.md`
    — the adversarial-review **P1 binary-path RCE** pattern (bare-PATH-only
    validation for exec'd binaries).
  - `docs/compound/2026-07-06-external-process-timeout-before-probe.md`
    — the **timeout-before-probe DoS** lesson.
  - `docs/compound/2026-07-06-autoharness-gate-broker-integration-contract.md`
    — the **backlogit ↔ autoharness gate-broker integration contract**.
- **compound-refresh**: reviewed all 9 existing top-level compound entries; none
  are superseded, duplicated, or invalidated by this shipment (net-new feature).
  Classification: **KEEP all 9**; **ADD 3**. No consolidate/replace/delete.
- **Decisions record**:
  `docs/decisions/2026-07-05-gate-repeated-failure-requeue-ownership-deliberation.md`
  retained as a permanent decision record (not a disposable source artifact).

## Source artifact cleanup

Feature 082-F carries no `custom_fields.source_stash_id` and no
`custom_fields.source_deliberation_id` (only `harness_status`). There is therefore
**no source stash entry or source deliberation to retire** for this shipment. The
referenced deliberation/decision doc is a permanent record and is retained.
- Stash removed: none.
- Deliberations archived: none.
- Skipped/not-applicable: source_stash_id (absent), source_deliberation_id (absent).

## Follow-ups

The four LOW/by-design adversarial findings were already stashed pre-merge (low
priority): `162F5548` (F1 base-ref precedence UX), `9822F787` (F4 member-evidence
`ran=false` fail-open), `7C5EADA6` (F5 shipment DecisionError class collapse),
`83B885EE` (F7 `move --json` GateError payload). No **new** post-merge follow-ups
were identified. Nothing further to stash this step.

## Readiness

**READY (merged).** The feature is merged, verified, reconciled, and archived. This
post-merge closure and the accompanying knowledge graduation are isolated on
`post-merge/082-S`; the **closure PR awaits its own operator P-014 approval**
per §1.10 and is **NOT merged** this run.
