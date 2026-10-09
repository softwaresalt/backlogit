---
title: "Deliberation: CX Ship closure protocol and gate correctness"
doc_type: "decision"
source: "docs/decisions/2026-10-08-cx-s-ship-closure-gate-correctness-deliberation.md"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
description: "Stage decision for the corrective CX release unit. It covers the Ship Step 6 closure protocol (allowlisted staging, pre/safe-close/post ordering, capability predicate), Ship gate correctness (argv-safe snapshot fallback, closure-PR gate scope), gate-aware Orchestrator eligibility and configured served roots, the harness lock-sidecar collision, ship-time validation performance, docs-migrate protection for closure gate keys, and the 140-S closure gate registration that unblocks 141-S."
topic: "Ship closure protocol, gate correctness, and the 141-S pre_claim unblock"
depth: "standard"
decision_status: "decided (operator APPROVED scope Decision 1 via the Orchestrator on 2026-10-08; work autonomously)"
promoted_to: "plan"
linked_artifacts:
  - "docs/exec-plans/2026-10-08-cx-s-ship-closure-gate-correctness-plan.md"
  - "docs/exec-plans/2026-10-03-195s-dispatch-closure-contract-plan.md"
  - "docs/closure/2026-09-11-140-s-158-f-pr-436-closure.md"
  - "docs/closure/136-S-154-F-post-merge-closure.md"
---

# Deliberation: CX Ship closure protocol and gate correctness

## Framing

The operator approved Decision 1 on 2026-10-08: stage a corrective release unit
(working name CX-S) that runs ahead of every queued shipment. The unit fixes
defects in the Ship closure path and in the gates that decide eligibility. The
queue head 152-S and the fault-line successor 141-S both block on it.

Verified state on `main` at `d2f3ff97`:

* `.github/agents/_ship.agent.md:1498` still runs `git add .backlogit/` in Step 6
  item 1.e (75E02C17 open).
* Ship Step 6 runs reconcile `mode: pre`, then calls `backlogit_ship_shipment`
  directly, then `mode: post`. The shipment-reconcile skill mandates
  pre, safe-close, post with safe-close as the only caller of the governed close
  (52D18E44 open).
* Step 6 item 1 is gated on prose ("shipments are enabled in this workspace")
  rather than the registry key `features.shipments: true` (497D20E3 open).
* `scripts/acquire_lock.ps1:391` and peers use `.<file>.lock`, which collides
  with backlogit's persistent OS-lock sidecars (67F17B6B open; 8B52A5F1 is its
  duplicate).
* 141-S fails `pipeline-topology --phase pre_claim` with
  `PREDECESSOR_CLOSURE_INCOMPLETE` because the gate looks for
  `docs/closure/140-S-*-post-merge-closure.md`. The narrative closure exists as
  `docs/closure/2026-09-11-140-s-158-f-pr-436-closure.md` with top-level
  `closure_status: READY` and `compaction_status: done`.

## Members

| Stash | Kind/priority | Shape | Route |
|---|---|---|---|
| 75E02C17 | bug/high | task, DEFERRED SCOPE EXPANSION | deliberate (forced, P-021 C6) |
| 52D18E44 | bug/high | task, DEFERRED SCOPE EXPANSION | deliberate (forced) |
| 497D20E3 | bug/high | task, DEFERRED SCOPE EXPANSION | deliberate (forced) |
| 67F17B6B | bug/medium | task | deliberate (grouped) |
| 8B52A5F1 | bug/medium | DEFERRED SCOPE EXPANSION, duplicate of 67F17B6B | archive as duplicate |
| D116AF58 | bug/medium | task (needs spike) | spike-first unit inside the plan |
| FBD6E6F8 | bug/high | task, DEFERRED SCOPE EXPANSION | deliberate (forced) |
| 8F1CF1E1 | bug/medium | task, DEFERRED SCOPE EXPANSION | deliberate (forced) |
| 6AB5E7FC | task/high | task | deliberate (grouped) |
| F05661B1 | bug/medium | task | deliberate (grouped) |
| 1293086D | bug/medium | task, DEFERRED SCOPE EXPANSION | deliberate (forced) |
| (new) | task | 140-S closure gate registration | plan unit |

## Grouping

All members are task-shaped and touch one coherent surface: the Ship closure
and eligibility gate path that every shipment passes through. They form one
covering feature. The grouping keeps installed agent text, lock scripts, and
the two Go fixes (D116AF58, 1293086D) together because each one is a gate or
closure correctness defect that can halt or corrupt the next closure. The Go
fixes are small and touch separate packages (`internal/core` ship-time
validation, `internal/docline` normalization), so they do not widen the blast
radius of the agent-text edits.

Covering feature title: **Ship closure protocol and gate correctness (CX)**.

## Options and Decisions

### D1: Closure staging (75E02C17)

* Option A: stage `.backlogit/queue/` and `.backlogit/archive/` as directories.
  This still absorbs unrelated pre-existing changes, for example a dirty
  `.backlogit/stash.jsonl`.
* Option B (chosen): stage an explicit path allowlist derived from the
  safe-close report. The allowlist is the queue and archive path of every ID in
  `M` plus `shipment_id`, the shipment's own reconcile reports, and the closed
  set of transaction side-effect files that ShipShipment is verified to write.
  After staging, `git diff --cached --name-only` must equal the allowlist that
  actually changed. Any other staged path halts closure.

The implementer must characterize ShipShipment's tracked write set by code
inspection (archive event append, linked-stash archival, hook queue) and list
it literally in the Ship text. Stage does not guess it.

### D2: Closure protocol ordering (52D18E44)

* Option A: keep the direct `backlogit_ship_shipment` call and document it as
  equivalent. Rejected: safe-close's baseline capture, non-member check, and
  parentage check are then skipped.
* Option B (chosen): Ship Step 6 item 1 runs `mode: pre`, then
  `mode: safe-close` (the only caller of the governed close), then
  `mode: post`. The third-branch (halted archival) and compensation guidance
  stays, but it now refers to the safe-close result envelope. The owner
  harness `TestUSR3_ShipmentReconcileExplicitFeatureMemberContract` subtest
  `ProceedAndShipStep6` currently anchors on
  "b. Call `backlogit_ship_shipment` with the merge commit SHA". The plan
  re-anchors it on the safe-close step in the same release (an intended
  contract change, not a regression).

Related stash entry 6387A6A2 (the `RECONCILE_FAIL` token mismatch and other
196-S wording clarifications) touches the same block. It is not part of the
approved scope and stays in the stash. The new Step 6 text must not introduce
any new `RECONCILE_FAIL` reference.

### D3: Capability predicate (497D20E3)

Chosen: gate Step 6 item 1 on the registry key
`features.shipments: true` read from `.autoharness/backlog-registry.yaml`,
matching the predicate already used at the top of the Ship agent. When the key
is absent or false, skip shipment closure and record why.

### D4: Snapshot fallback argument safety (FBD6E6F8)

Chosen: before any CLI fallback in Ship Step 4.0 item 1, validate each frozen
ID against the canonical task pattern `^[0-9]+\.[0-9]+-T$` and pass it as a
discrete argv element (never string-interpolated into a shell line). A
non-matching ID fails closed with `WAVE_SNAPSHOT_UNRELIABLE`. The ambiguous
candidates BFACAE09, B3701713, and 4A990AF9 are separate expansions (history
tool, CI path-filter coverage, lint debt), so this is not a duplicate.

### D5: Closure-PR gate scope (F05661B1)

* Backlogit-side (chosen, in scope): state in Ship Step 5 item 5a and Step 6.0
  item 4 that the 5a lifecycle gate applies only to the feature PR. Step 6.0
  closure PRs run P-014 and P-018 only. The Step 6.1 a0 lifecycle gate keeps
  running before safe-close while the shipment is still active.
* Upstream (out of scope, recorded ask): autoharness should add a closure or
  `post_ship` phase, or accept a just-shipped shipment with a post-merge
  branch, so agent mode has a phase that fits post-merge closure. 183-F
  explicitly excludes F05661B1, so this ask is recorded here only.

### D6: Gate-aware eligibility (6AB5E7FC)

Chosen: Orchestrator Step 0 and every "eligible shipments" report run
`autoharness gate pipeline-topology --mode agent --shipment {id} --phase pre_claim --json`
for each DAG-ready candidate and show the verdict token next to DAG readiness.
A candidate is reported eligible only when both agree. If the gate is not
installed, the report says so and does not fabricate a verdict. This is
reporting only; it never claims, and the gate stays the authority at claim.

### D7: Configured served roots (8F1CF1E1)

Chosen: the Served-Root Handoff Procedure's manifest-location step resolves
the queue and archive directories from the attested catalog
(`workspace.queue_path` and `workspace.archive_path` from
`backlogit_get_metadata_catalog`), each required to be contained in the
served storage root. The manifest must exist in exactly one of those two
directories. This supersedes requirement R4(c) of
`docs/exec-plans/2026-10-03-195s-dispatch-closure-contract-plan.md`, which
bound the manifest to the default `<storage>/queue` or `<storage>/archive`.
The U1 harness `TestUSR1_OrchestratorServedRootHandoffContract` pins the
literal "exactly one of `queue` or `archive`" and is updated in the same
release to the new literals plus a nested-root assertion.

### D8: Lock sidecar collision (67F17B6B, supersedes 8B52A5F1)

* Option A: skip `.backlogit/` targets in the harness scripts. Rejected: the
  harness may still need to lock backlogit artifacts during concurrent work.
* Option B (chosen): rename the harness lock file to `.<file>.agent-lock` in
  all four scripts and in `concurrency.instructions.md` and the `file-lock`
  skill. Stale `.<file>.lock` files created by older scripts are never deleted
  by the new scripts, because those names belong to backlogit.

The scripts and docs are installed autoharness artifacts. Each edit needs a
harness-manifest drift record, and the upstream template should receive the
same rename (recorded as an upstream ask in the closure).

### D9: Ship-time validation performance (D116AF58)

Chosen: spike first. A benchmark task profiles `validateMemberGateEvidence`
(`internal/core/shipment_gate.go`) on a 45-member fixture and confirms or
refutes the hypothesis that per-member `findArtifact` WalkDir plus a full
event-log read dominates. The fix task is gated on the spike verdict:
resolve member paths once per gate pass and read the event log once, and add
progress output on the CLI surface. If the spike refutes the hypothesis, the
fix task is re-planned before it is claimed.

### D10: Closure gate keys under docs migrate (1293086D)

* Option A: exclude `docs/closure/` from `docs migrate`. Rejected: closure
  docs still benefit from migration of other keys.
* Option B (chosen): for `doc_type: closure`, keep `closure_status`,
  `compaction_status`, and `conditions` at the top level during
  normalization (allowlist in `internal/docline/policy.go`, consumed by
  `normalize.go`). Test-first.

### D11: 140-S closure gate registration (new unit)

Chosen: create `docs/closure/140-S-158-F-post-merge-closure.md` as a
gate-registration record following the 136-S precedent: top-level
`closure_status: READY`, `compaction_status: done`, `doc_type: closure`, and
the "do not docs migrate" comment. It points at the existing narrative
closure. Renaming the narrative file was rejected because three documents
reference its current path. Ship verifies the result with
`autoharness gate pipeline-topology --mode agent --shipment 141-S --phase pre_claim --json`.
Stage does not create the file.

## Scope Boundaries

In scope: the members above, the owner-harness re-anchors that the D2 and D7
contract changes require, and harness-manifest drift records.

Out of scope: the plugin Ship agent (`plugin/agents/ship.agent.md`), the
autoharness upstream templates, 6387A6A2 wording clarifications, EB95F5F7
(session-start condition surfacing), AC530DE2 (dark-mode activation
preflight), the CI autoharness pin (F88FE051), and CT-S.

## P-021 C5/C6 Triage Records

### Duplicate detection (unconditional)

| Entry | Scan result |
|---|---|
| 75E02C17 | clean scan, no duplicate |
| 52D18E44 | clean scan; 732FA61E (148-S used the CLI cascade without the skill) and 6387A6A2 (196-S wording) overlap by keyword but are different expansions |
| 497D20E3 | clean scan, no duplicate |
| 8B52A5F1 | DUPLICATE of 67F17B6B. Surviving entry: 67F17B6B (earliest captured, broader statement). Archived duplicate: 8B52A5F1 via `stash archive`, linked `duplicate_of` 67F17B6B where the tool supports stash links. Its source refs (173-F, 154-S, PR #466) are carried here. |
| FBD6E6F8 | clean scan; DISCOVERY-STATUS AMBIGUOUS candidates BFACAE09, B3701713, 4A990AF9 checked and are different expansions |
| 8F1CF1E1 | clean scan; CDBCB258 (direct-Ship served-root intake) is a different expansion |
| 1293086D | clean scan; EB95F5F7 (session-start surfacing) is a different expansion |

The non-marker members (67F17B6B, D116AF58, 6AB5E7FC, F05661B1) were also
scanned: 6AB5E7FC overlaps 7B71AD77, A441A74C, and AC530DE2 by keyword only
(different expansions). Clean scan for the rest.

### Late-identifier reconciliation (triggered by N/A source refs)

| Entry | N/A field | Outcome |
|---|---|---|
| 75E02C17 | pr, review_thread | no late identifier found; N/A stands |
| 52D18E44 | pr, review_thread | no late identifier for the 155-S capture; N/A stands. Related evidence: the defect is cited again in the 196-S closure (PR #478) |
| 497D20E3 | pr, review_thread | no late identifier found; N/A stands |
| 8B52A5F1 | review_thread | no late identifier found (threadless post-merge finding); archived as duplicate |
| FBD6E6F8 | PR, review_thread | no late identifier found (pre-PR finding); N/A stands |
| 8F1CF1E1 | none | not triggered (PR #478, thread PRRT_kwDORzozKM6pqgRI recorded) |
| 1293086D | none recorded as N/A | not triggered |

## Decision

Proceed to planning with the D1-D11 choices. The plan must keep every unit
within the 2-hour rule and a single domain, sequence the RED contract tests
ahead of the agent-text edits, and run the harness-manifest drift records last.
