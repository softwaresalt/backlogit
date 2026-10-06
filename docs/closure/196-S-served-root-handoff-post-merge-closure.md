---
chunk_strategy: h1-h2-h3
closure_status: READY_WITH_CONDITIONS
compaction_status: degraded
description: "196-S governed shipment closure passed at merge 651b066b. Follow-ups 227A2930 and B83081F5 are discharged; 359D8F32 stays open, and tracked residual risks are recorded."
doc_type: closure
schema_version: "1.0"
shipment_id: 196-S
feature_id: 196-F
source: docs/closure/196-S-served-root-handoff-post-merge-closure.md
title: "196-S post-merge closure"
docline:
  date: 2026-10-06T00:00:00Z
  status: reviewed
  tags:
    - operational-closure
    - post-merge
    - 196-S
    - 196-F
---

## Release record

| Field | Value |
|---|---|
| Shipment | `196-S` |
| Covering feature | `196-F` |
| Implementation PR | [#478](https://github.com/softwaresalt/backlogit/pull/478) |
| Merge commit | `651b066b749d216b88e9b5d406a586f26a69cead` (merge commit, P-009; merged 2026-10-06T22:36:45Z; no `--admin`) |
| Final reviewed HEAD | `2a123282` (local review readiness `READY_WITH_FOLLOWUPS`, P0/P1 = 0; Copilot reviewed with zero unresolved threads; `copilot-review` gate `SATISFIED`) |
| Archive commit | `4130b5e2` (`chore(backlog): archive 196-S backlog artifacts`) |
| Closure branch | `post-merge/196-f-served-root-handoff-explicit-feature-reconcile` |
| Closure PR | This branch's post-merge closure PR; its readiness block records the current HEAD |
| Closure status | `READY_WITH_CONDITIONS` |
| Context compaction | `degraded`; see `docs/closure/2026-10-06-196-s-compaction-report.md` |
| Routing | `ROUTING_DEGRADED`: the runtime could not honor `gpt-6-luna`; non-blocking |
| Mode | P-017 dark mode, scope `[196-S]`; merge pre-authorized; admin fallback not authorized and not used |

## Summary

PR #478 delivered the 195-S follow-up contract:

* The Orchestrator Served-Root Handoff Procedure, with its outcome-record
  timing (the outcome is carried in the Ship payload and persisted only on the
  shipment branch).
* The matching Ship Step 0.5 item 3a write.
* The shipment-reconcile `feature-pending-governed-completion` classification.
* Harness-manifest checksum updates.
* Integration contract tests U1–U9.

All nine tasks completed under Erratum E1. 196.001-T and 196.003-T followed the
E1.4 per-task clean-tree/fresh-baseline rule. Wave gates 2–5 and the final
unfiltered `go test -timeout=30m ./...` passed.

Copilot review ran five passes:

* Pass 1: one out-of-scope comment, declined under P-021 C1 and captured as
  `8F1CF1E1`.
* Passes 2 and 4: in-scope fixes in `e7e7da02`, `c2d309f1`, `cea1b183`, and
  `2a123282`.
* Pass 5: the final HEAD `2a123282`, reviewed with no unresolved threads.

Required checks passed: `test`, `Detect code changes`, `Docline frontmatter
gate`, and `Markdown lint (P-008)`.

## Shipment reconciliation

The exact ordered manifest is `196-F`, `196.001-T`, `196.003-T`, `196.004-T`,
`196.007-T`, `196.008-T`, `196.002-T`, `196.005-T`, `196.009-T`, and
`196.006-T`.

| Phase | Report | Result |
|---|---|---|
| Pre-close (`expected_status: done`) | `.backlogit/reconcile/196-S-pre-20261006T223826Z.md` | `PROCEED`. Nine tasks `pre-archived`; `196-F` `feature-pending-governed-completion`; shipment `record-consistent` |
| Safe-close | `.backlogit/reconcile/196-S-safe-close-20261006T225014Z.md` | `CLOSED` |
| Post-close | `.backlogit/reconcile/196-S-post-20261006T225014Z.md` | `PROCEED` |

This closure is the first live use of the `feature-pending-governed-completion`
classification that 196-S introduced. The pre-close pass accepted the `active`
explicit feature without a Stage-side status move. Compare the 195-S closure,
which first returned `RECONCILE_FAIL` and needed a manual Stage move.

The single `backlogit_ship_shipment` MCP call hit a client-side
`-32001 Request timed out` and was not retried. The server finished the
governed operation:

* status `shipped` at 22:40:47Z;
* commit traceability on every member;
* deepest-first archival;
* shipment `archived` event at 22:48:48Z;
* post-ship hook seq 3580.

The missing envelope was reconstructed from that state: `archived_ids` equal
the manifest plus `196-S`, and `returned_ids` is empty. Other checks:

* No non-member queue or archive mutation.
* Parentage preserved.
* No P-007 archive deletions.
* The 143-F halted-archival branch did not occur.
* A transient `shipped_unarchived_residue` doctor finding cleared once archival
  finished.

The recurrence is captured in
`docs/compound/2026-10-06-ship-shipment-mcp-timeout-is-client-side.md`.

## 195-S follow-up discharge

This section is the closure note the 196-S shipment description asked for. It
references `docs/closure/195-S-claim-start-proof-post-merge-closure.md`, whose
`READY_WITH_CONDITIONS` conditions it updates.

| Follow-up | Disposition |
|---|---|
| `227A2930` (Orchestrator authoritative served-root handoff) | **Discharged** by merge `651b066b749d216b88e9b5d406a586f26a69cead` (PR #478) |
| `B83081F5` (explicit feature-member reconcile contract) | **Discharged** by merge `651b066b749d216b88e9b5d406a586f26a69cead` (PR #478); exercised live by this closure's pre-close pass |
| `359D8F32` (P-020 `target: all` compaction candidate review) | **Open**; not covered by 196-S and left for a later Stage cycle |

Both discharged entries were already in `.backlogit/archive/stash.jsonl`. Stage
archived them on 2026-10-03, when it harvested them into `196-F`. Ship made no
further stash mutation.

## Residual risks

| ID / item | Note |
|---|---|
| `1EF5BDC6` | DEFERRED SCOPE EXPANSION: align the plan's U9 Change 2 body (~line 614) with the A3.2.3 Green wording and placement (`SERVED_ROOT_ATTESTATION_FAILED` halts before any claim or raw item-log read). Plan-text only; open for Stage. |
| `33C4B816` | DEFERRED SCOPE EXPANSION: update the archived 196.009-T task description to name A3.2.3 as governing and carry its Change 1 and Change 2 literals verbatim. Description-only; open for Stage. |
| E1.2 process deviation | Erratum E1.2: Ship did not record the A3.3 item 2 check immediately before the U1 (`d2248d7c`) and U3 (`e36ac003`) harness edits. Stage recomputed equivalent evidence read-only from pinned SHAs and the `origin/main` reflog (`c4c458b9` across the edit window), and E1.2 accepts it. Each of 196.001-T and 196.003-T cites E1.2 in its completion evidence. |
| E1 approval record | The operator's resume approval is recorded in the plan E1 header (commit `d9e00a6d`) and relayed by the Orchestrator; Ship did not receive it directly |
| #477 P-001/P-016 deviation | origin/main (PR #477) was merged into the active 196-S branch at `35182815` while 196-S was the single active release unit. Recorded, not re-litigated; no topology violation followed. |
| A3.6 items | `CDBCB258`, `F6F3AA0E`, `147BD825`, and `3B25D37F`, plus the R10 interim limitation, as recorded in plan A3.6 |
| `F88FE051` | CI `pipeline-topology (ambient)` fails with `PREDECESSOR_CLOSURE_INCOMPLETE` on 195-S. PyPI `autoharness==1.5.0` lacks the `dag-root` declared-root waiver that the local build honours (local: `PASS`, `predecessor_source: declared_root`). Non-required check. |
| Session deferrals | P-021 deferred entries captured this session: `180AA2C2`, `CB0F604E`, `6387A6A2`, `7C9340AC`, `0F428816`, `5B2F5EC1`, `4E0F44CE`, `5E3FBAC7`, `CC86D2E1`, `8F1CF1E1`, `F88FE051`; reused `52D18E44`; `2D682258` remains open. Only its Step 6 item a orphan-clause concern was resolved in-branch (`301a2568`). The rest is still unaddressed: the `NotContains` assertion on the removed clause, slicing `### Step 6:` before the anchor search, guard reordering and de-duplication, and the unreachable `start < 0` branch. |
| Stash usage (E1.4 rule 5) | None. No `git stash push` was needed for 196.001-T or 196.003-T. |

## Invariants to preserve

* Process only the exact ten manifest IDs and the `196-S` control record.
* Never infer membership from parentage, descendants, or source artifacts.
* An `active` explicit feature at pre-close is
  `feature-pending-governed-completion` only when every explicit task member is
  `matched` or `pre-archived`.
* A ship MCP timeout means the result is unknown: wait for the call to finish
  and verify. Never retry or restore.

## Validator evidence

| Field | Evidence |
|---|---|
| Affected runtime surfaces | None. PR #478 changed agent and skill contract text, the harness manifest, and integration contract tests. |
| Applicability | Not applicable; no application runtime or deployed-service surface changed |
| Verdict | Not applicable. No runtime validator was required or run; this is not a passing validator result. |
| Surface adapters | None required |
| Probe outcomes | None |
| Manual checkpoint evidence | None |
| Blocked prerequisites | None |
| Runtime follow-up recommendations | None from runtime verification |

## Runtime validation

Not applicable: no runtime surface changed. The automated evidence is the
contract tests, the full suite, and the required CI checks above.

## Pre-deploy audits and release path

* There is no data migration, feature flag, configuration, or rollout
  prerequisite.
* The change merged as a merge commit through PR #478.
* Post-merge backlog archival is committed on the closure branch.

## Post-deploy checks

Not applicable to a runtime. Backlog closure is verified by the pre-close,
safe-close, and post-close reports and the archived records.

## Risky action record

| ProposedAction | ActionRisk | Approval | ActionResult | Rollback |
|---|---|---|---|---|
| Governed ship of `196-S` at `651b066b`, archiving only the explicit manifest | High | P-017 dark-mode activation, scope `[196-S]` | `applied`; all reconciliation phases passed | None required; any later integrity issue follows P-007 |
| Merge PR #478 (merge commit) | High | `merge_approval_pre_authorized: true`; current-HEAD readiness, Copilot gate, and required CI all passed | `applied`; `651b066b` | `git revert -m 1 651b066b` on a branch plus a PR |
| Remove the 196-S temporary block from `.git/info/exclude` (clone-local) | Low | Operator instruction (Orchestrator) | `applied`. The decision-E "never commit" line for `checkpoint-20261005-052350.json` was kept as a standalone persistent exclusion, so removing the block cannot surface that file for commit. | Re-add the lines locally |
| Delete untracked scratch `logs\optionA\` | Low | Pre-approved in the operator instruction | `applied` | None (untracked scratch) |

## Operational signals

| Area | Evidence or action |
|---|---|
| Healthy signal | Shipment `shipped` and archived; all ten members archived with provenance and merge SHA; no non-member change |
| Failure signal | Missing archive, non-member delta, `mutation_partial`, or P-007 deletion |
| Monitoring plan | Not applicable to a runtime release |
| Rollback trigger | Contract regression observed in a later Orchestrator→Ship handoff or reconcile pass |
| Rollback procedure | Revert the merge commit on a branch, via a PR |
| Validation window | Closes when the closure PR passes local review, required CI, and the Copilot-review gate, and merges |
| Owner | Ship for closure PR readiness; Stage/Orchestrator for the named follow-ups |

## Source artifact provenance

`196-F` `custom_fields` has `harness_status` and `scheduler_baseline_claim`,
but no `source_stash_id` or `source_deliberation_id`. Both provenance fields
are `none`. No source stash or deliberation was mutated.

## Releasability evidence

**Status: `READY_WITH_CONDITIONS`.** Governed closure and every reconciliation
phase passed, and 227A2930 and B83081F5 are discharged. Two conditions remain:

1. The closure PR must pass current-HEAD local review, required CI, and the
   Copilot-review gate, and then merge (P-001 release closure).
2. The residual risks above stay tracked for Stage/Orchestrator, as does open
   follow-up `359D8F32`.

## P-020 context compaction

`compact-context` was invoked with `target: all`. The additive summary
`docs/memory/compacted/2026-10-06-196-S-compacted.md` was written. Archival
moves were deferred because checkpoints, stash entries, and the governing plan
reference the originals by path. Compaction status: `degraded`, which is
non-blocking. See `docs/closure/2026-10-06-196-s-compaction-report.md`.
Compound-refresh: `docs/closure/2026-10-06-196-s-compound-refresh.md` (six
entries kept, one new learning).

## Follow-up

* `359D8F32`: open for Stage.
* `1EF5BDC6`, `33C4B816`, A3.6 items, and the session deferrals above: open for
  Stage triage.
* `F88FE051`: raise the CI-installed `autoharness` to a build with the
  `dag-root` waiver.

No new post-merge follow-up stash entries were required beyond these existing
ones.
