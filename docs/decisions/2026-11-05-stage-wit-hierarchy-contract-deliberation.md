---
chunk_strategy: h1-h2-h3
description: 'Decision to align Stage decomposition instructions with the configured backlog hierarchy.'
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-11-05-stage-wit-hierarchy-contract-deliberation.md
title: 'Stage and Backlog Hierarchy Contract Decision'
docline:
  deliberation_id: 083-DL
  decision_status: decided
  promoted_to: none
  stash_id: 4F1C9224
---

## Problem Frame

The Stage decomposition contract requires a created sub-epic beneath the
covering feature or chore and then tasks beneath that sub-epic. The live WIT
has no epic type and does not permit that parent chain. The harvest skill
describes the supported direct feature-to-task hierarchy. This conflict
blocked review of the separate plan namespace maintenance work.

## Operator Decision

The operator approved the separate correction:

> ok, I committed and pushed the changes we have so far to the remote branch.
> Yes to correcting Stage’s hierarchy contract and other tasks

The operator selected the configured hierarchy as authoritative: feature or
chore to task/review, and task to subtask. Remove the mandatory unsupported
sub-epic node and contradictory task-parent language. Preserve task sizing,
scope, readiness, role separation, and safety gates. Do not introduce a WIT
type, schema change, or epic enablement.

## Evidence and Options

* The active `.backlogit/config.yaml` declares feature and chore children as
  task/review and task children as subtask
* `docs/compound/workflow-issues/orphaned-tasks-without-parent-features-2026-04-10.md`
  records the importance of parent-first task creation
* `docs/compound/2026-09-03-stage-harvest-chore-id-collision-and-p008-gate.md`
  records a historical chore child-ID collision and the need to verify creation
* **Chosen**: align Stage's contract with supported configured parent edges and
  retain parent-first ordering and all existing gates
* **Rejected**: add or enable an epic type to preserve the unsupported chain;
  that expands schema scope without a requirement

## Provenance and Reconciliation

* Surviving deferred entry: `4F1C9224`; clean duplicate scan across active and
  indexed stash entries; no duplicate was found or archived
* Originating PR: #643; review-thread, task, feature, and shipment IDs: `N/A`
* Original source: operator-approved plan namespace rename and maintenance plan
  `docs/plans/2026-11-04-docs-plan-namespace-compatibility-plan.md`,
  review attempt 1 `FAIL`
* No late identifier found in the indexed backlog and citing closure/memory
  records. The original `N/A` values remain unchanged

## Bounded Handoff

The intended correction targets `.github/agents/_stage.agent.md`, plus directly
coupled installed or distributed harvest contract copies only if needed.
PromptBuilder owns those edits and their validation. This decision does not
itself change the harness, restart the namespace plan review, create tasks or a
shipment, or authorize implementation.
