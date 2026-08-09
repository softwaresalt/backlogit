---
chunk_strategy: h1-h2-h3
description: "Operational closure for 117-S (Formal Gate F1 - evidence authenticity and manifest binding): invariants, monitoring plan, rollback triggers, and readiness verdict."
doc_type: closure
docline:
    date: 2026-08-09T00:00:00Z
    status: accepted
    tags:
        - operational-closure
        - formal-gate
        - 117-S
        - security
schema_version: "1.0"
source: docs/closure/2026-08-09-117-S-formal-gate-f1-closure.md
title: "117-S Formal Gate F1 — Operational Closure"
---

# 117-S Formal Gate F1 — Operational Closure

## Readiness Status: READY

## Summary

Shipment 117-S (Formal Gate F1 — evidence authenticity and manifest binding,
tasks 106.003-T through 106.011-T) implements an optional, backward-compatible
HMAC-authenticated proof bound into pre-task-completion gate evidence, plus a
manifest-binding digest for shipment-level evidence, closing the "anyone with
write access to `.backlogit/logs/` can hand-edit a passing gate evidence event
into existence" gap. Implementation PR #333 merged to `main` via merge commit
`23d88904faf917a4f4003042f185de9b4e568530`. This closure covers post-merge
shipment archival, runtime verification, and monitoring handoff.

## Review History (context for monitoring emphasis)

This was reviewed with unusual depth given its security-sensitive nature: an
initial 8-persona standard/security/adversarial multi-model review round,
followed by **10 additional Copilot PR review rounds**, together surfacing and
fixing ~30 distinct findings — including three genuinely critical
admission-bypass defects (an unauthenticated member-evidence check with zero
production call sites; an operator `--force` completion that produced a fully
formally-admissible signed proof regardless of the underlying check's real
outcome; and a complete bypass of `ShipShipment`'s entire verification chain
via the general-purpose `move_item`/`update_item` tools). Two lower-severity
findings were explicitly deferred with tracked backlog follow-ups
(`106.032-T`, `106.033-T`) rather than fixed inline, given diminishing
severity and increasing design scope relative to the fixes already shipped.

## Invariants to Preserve

* **Backward compatibility**: a workspace that never sets
  `BACKLOGIT_GATE_EVIDENCE_KEY` and never enables `formal_gate` in config sees
  byte-identical evidence to pre-117-S behavior.
* **No unauthenticated fallback under enforcement**: once
  `BACKLOGIT_FORMAL_GATE_REQUIRED` is truthy (or workspace config enables it),
  every refusal path fails closed — no code path silently completes a gated
  transition without a genuinely verified signed proof.
* **Environment anchor authority**: workspace config may only RAISE
  enforcement strictness, never lower what the environment anchor requires.
* **Ship-only shipment completion under enforcement**: a shipment may reach
  `shipped` status ONLY via `ShipShipment` while formal gate evidence is
  enforced — never via a direct `move`/`update` status write.
* **Counter-uniqueness / anti-replay**: the per-item formal-gate counter is
  monotonic and cannot be duplicated or replayed, even under process crash or
  concurrent completion attempts.

## Pre-Deploy Audits

* [x] No migration required — this is purely additive; existing JSONL
  evidence files are read unchanged when formal admission is not enforced.
* [x] `BACKLOGIT_GATE_EVIDENCE_KEY` and `BACKLOGIT_FORMAL_GATE_REQUIRED` are
  environment-anchored, not workspace config — no config schema migration.
* [x] Full test suite (27 Go packages) green, including `-race` runs on every
  concurrency-sensitive change (counter lock, shipment membership lock, gate
  evaluation head-drift bracket).
* [x] `golangci-lint run` and `go vet ./...` clean throughout.
* [x] Runtime verification PASS (see
  `docs/closure/2026-08-09-117-S-formal-gate-f1-runtime-verification.md`):
  fresh binary built from the merged tip; CLI-level confirmation of the
  round-8 shipment-bypass fix and the legitimate ship path, both via the real
  binary, not only unit tests.

## Deployment / Rollout Path

Merge-only. `backlogit` is distributed as a CLI/MCP-server binary; there is no
separate deploy/release step tracked by this closure beyond the merge to
`main` and this post-merge shipment archival. Formal-gate enforcement itself
is OPT-IN (environment anchor or workspace config) — merging this code does
NOT itself enable enforcement for any existing workspace.

## Post-Deploy Checks

* Confirmed shipment 117-S and all 9 member tasks archived with
  `archived_status: shipped` / `status: done` (shipment-reconcile post-mode:
  PROCEED, see `.backlogit/reconcile/117-S-post-*.md`).
* Confirmed `origin/main` reflects the merge commit and the shipment/task
  archival changes (post-closure-PR merge, tracked below).

## Risky Action Record

| Action | Risk | Result |
|---|---|---|
| Merge PR #333 to `main` (merge commit) | moderate (security-sensitive feature) | applied — commit `23d88904` |
| Ship shipment 117-S (archive 9 tasks + shipment) | low (additive, reconciled pre/post) | applied — reconcile PROCEED both phases |
| Runtime verification of the round-8 shipment-bypass fix | low (disposable scratch workspace only) | applied — PASS, scratch workspace fully removed |

No destructive actions were taken against the primary (dirty) worktree or any
shared/production data at any point in this cycle.

## Healthy Signals

* `backlogit_move_item` / `backlogit_update_item` / CLI `move`/`update`
  commands continue to work normally for all non-shipment-terminal
  transitions, with byte-identical evidence when formal admission is not
  enforced.
* `ShipShipment` / CLI `shipment ship` continues to succeed for legitimately
  gated shipments.
* When formal enforcement IS opted into, `FormalAdmit`-eligible completions
  produce a `proof`/`key_id`/`proof_schema`/`counter`/`timestamp_utc` bearing
  delta, verifiable via `gateproof.Verify`.

## Failure Signals

* Any `ErrFormalGateRequired` / `formal_gate_proof_invalid` /
  `formal_gate_proof_unverifiable` refusal rate that is unexpectedly HIGH
  after an operator enables enforcement — may indicate a key-rotation
  mismatch, a misconfigured `key_id`, or a genuine attempted tamper.
* Any report of a task or shipment completing successfully WITHOUT the
  expected signed evidence when `BACKLOGIT_FORMAL_GATE_REQUIRED` is known to
  be set — would indicate a regression in the fail-closed guarantees this
  shipment establishes.
* Any report of a shipment reaching `shipped` status through a path other
  than `ShipShipment` while formal enforcement is active.

## Monitoring Plan

* This is a CLI tool without a centralized telemetry backend for individual
  installs; monitoring is advisory-level for operators who enable formal
  enforcement:
  * Watch `.backlogit/logs/*.jsonl` for `pre_task_completion_gate_error`
    events classified under the `formal_gate_*` MCP error codes
    (`internal/mcp/formal_gate_errors.go`) after enabling enforcement.
  * `backlogit_telemetry_harvest` / `internal/telemetry` tool-usage tracking
    (when enabled) can surface unusual refusal-rate spikes across sessions.
* No dashboards are provisioned as part of this shipment; this is a
  documented, manual observation practice for operators adopting formal
  enforcement, recorded here so it is not lost.

## Rollback Trigger

* A confirmed regression allowing an unauthenticated or bypassed completion
  under enforced formal-gate evidence (i.e., a fail-open where fail-closed
  was guaranteed) is an IMMEDIATE rollback trigger.
* A confirmed false-positive refusal rate that blocks legitimate operator
  workflows (e.g., the typo-tolerance fix from round 8 behaving unexpectedly
  in production) is a rollback-consideration trigger, though lower urgency
  than a fail-open regression.

## Rollback Procedure

* Revert the merge commit (`23d88904`) via a standard `git revert -m 1` merge
  commit revert, preserving history per Constitution Principle XI (merge
  commits only, never history rewrite).
* Formal-gate enforcement is opt-in via environment anchor; an operator who
  hits an issue can immediately mitigate by unsetting
  `BACKLOGIT_FORMAL_GATE_REQUIRED` and disabling `formal_gate.enabled` in
  workspace config without needing a code rollback, while a permanent fix is
  prepared.

## Validation Window

30 days from merge (2026-08-09 through 2026-09-08) — this is a new,
security-sensitive, opt-in capability; the window allows time for any
early-adopting workspace to surface configuration or key-rotation friction
before this is considered fully stable.

## Owner

Repository maintainer (softwaresalt) — no dedicated on-call rotation exists
for this CLI tool; the owner is the same operator who would enable formal
enforcement in their own workspace(s).

## Source Artifact Cleanup

No `custom_fields.source_stash_id` or `custom_fields.source_deliberation_id`
found on shipment 117-S or parent feature 106-F — this feature's planning
artifacts (`docs/decisions/2026-07-14-formal-gate-architecture-spike.md`,
`docs/decisions/2026-08-07-f1-evidence-authenticity-mechanism-deliberation.md`,
`docs/exec-plans/2026-08-07-f1-evidence-authenticity-manifest-binding-plan.md`)
are referenced via the standard `references` frontmatter field, not the
stash/deliberation linkage fields this step targets. Nothing to retire.

## Follow-Up Backlog Items

* `106.032-T` — Bind resolved `base_ref` into the formal gate proof envelope
  (schema v2 candidate). Priority: low.
* `106.033-T` — Repository-ref CAS/guard for the narrow post-manifest-signing
  HEAD-drift window in `ShipShipment`. Priority: medium.

Both are queued under parent feature `106-F` for future prioritization; 118-S
is explicitly NOT started as part of this cycle.
