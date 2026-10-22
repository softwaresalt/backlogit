---
title: "Ship Session 160-S Intake Resumed"
date: 2026-10-22
---

Shipment: `160-S`
Scope: this invocation only; dark-factory scope does not include `161-S` or later.

## Intake History and Dispositions

Two prior intake attempts halted: the first on a covering-feature membership
interpretation, and the second on `WORKTREE_DIRTY`. The prior memory note was
moved out of the worktree by the Orchestrator. The remaining active checkpoint
was `checkpoint-20261022-193907.json`; its old resume hint requested a Stage
manifest amendment. The operator has since explicitly directed Ship to proceed
with the unchanged task-only manifest. That disposition supersedes the old
resume hint; Ship does not add or remove shipment members.

The approved membership design is task-only for multi-wave covering feature
`175-F`, consistent with decision
`docs/decisions/2026-09-19-baseline-convergence-decomposition.md` H6 and
shipments `157-S`, `158-S`, `159-S`, and `176-S`. Adding `175-F` would archive
the feature while its remaining queued tasks were outside this shipment. The
shipment manifest was inspected only for intake validation and remains
unchanged. Exact task lookups confirmed the explicit members have
`parent_id: 175-F`, satisfying the covering-parent check without expanding
shipment membership. This intake observation is not the frozen P-002.6 task
set `M`; Ship freezes `M` only at Step 3 after resolving every manifest
member's artifact type and the status model.

The shipment response carried no hold/refusal labels. The transitive blocks chain is
`160-S -> 159-S -> 158-S -> 157-S -> 176-S -> 154-S`. Condition (b) passed
using the authoritative archived Markdown and append-only item log after
served-root attestation: `154-S` is `archived` with `archived_status: shipped`;
its log contains the shipped-status event at
`2026-09-30T08:44:25.1831744-07:00`; the newest qualifying candidate is the
operator's `CONDITION_B_ATTESTED` comment at
`2026-10-06T23:31:36.0953115-07:00`. All relevant timestamps parsed as
RFC3339. Reads used no-follow opens, canonical-handle verification, and
reparse-point/containment checks. The operator's earlier report that
condition (b) had passed was independently reconfirmed.

## Served-Root and Branch Evidence

The metadata catalog's served workspace and storage roots matched the
Orchestrator handoff. The read-only `pragma_database_list` check returned one
`main` database at `.backlogit/backlogit.db`. Workspace-relative roots:

* workspace: `.`
* storage: `.backlogit`
* database: `.backlogit/backlogit.db`

`pipeline-topology` `pre_claim` passed with
`BRANCH_CREATE_ELIGIBLE` and `CARRY_FORWARD_ELIGIBLE`; the only carry-forward
was the active checkpoint. Main was up to date, and Ship created
`feat/160-s-baseline-convergence-rs-w03-remediation-wave-3`. Per the gate's
isolated-carry-forward instruction, the checkpoint was committed first:
`e1a200b7` (`chore(ship): carry forward 160-S intake checkpoint`), with the
required Copilot co-author trailer.

## Dark-Mode Authority and Execution State

The operator's dark-mode activation covers the ordered sequence beginning
`160-S`, but this invocation is limited to `160-S`. Merge is pre-authorized
only for merge-commit strategy after all mandatory gates pass. Admin fallback
is not authorized and cannot bypass CI, current-HEAD local review, P-018
Copilot review, P-009, or P-016.

The initial `backlogit_claim_shipment` RPC timed out. The first post-claim
topology check returned `CLAIM_NOT_OBSERVED` (exit 3). The registry-declared
CLI status fallback then failed because a shipment-lifecycle gate was in
progress. An exact MCP item read observed `160-S` as `active`; the bounded
post-claim re-verification passed with `160-S` as the sole active shipment.
No claim retry was issued. The selected stale checkpoint was resolved after
the branch and memory note were established.

The compile-only preflight `go test -run=^$ -count=1 ./...` passed. A preliminary
P-001 CLI query used an unsupported flag and is not evidence; the proper
read-only MCP count query returned `active_release_units: 0`, so P-001 passed.

Next: run intake reconciliation for `expected_status: active`. Freeze `M` only
at Step 3 after the required manifest, status, dependency, harness,
red-deliverable, regression-command, and scheduler-simulation checks.
