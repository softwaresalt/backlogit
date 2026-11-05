---
chunk_strategy: h1-h2-h3
description: 'Blocked Stage handoff for the plan namespace relocation and hierarchy contract conflict.'
doc_type: memory
schema_version: "1.0"
source: docs/memory/2026-11-05/stage-docs-plan-namespace-blocked-handoff.md
title: 'Stage Blocked Handoff for Plan Namespace Relocation'
docline:
  date: '2026-11-05T02:26:25Z'
  deferred_stash_id: 4F1C9224
  status: blocked
---

## Current State

* Branch: `stage/212-S-multi-session-mcp-runtime`; HEAD:
  `0612889b5062b593cabefa64222654a828449efe`. No new commit or push
* The approved `git mv` moved the single tracked
  `docs/exec-plans/2026-11-04-multi-session-mcp-runtime-plan.md` to the
  corresponding `docs/plans/` path. The old directory was empty at inspection
* Uncommitted Stage work includes the moved plan, 136 other plan self-source
  corrections under `docs/plans/`, the active decision and Stage memory,
  `.backlogit/queue/002-SP.md`, the `211-F` through `213-F` feature/task
  references and `212-S` through `214-S` shipment references, plus backlog
  memory, append-only hook events, stash, and same-session checkpoints
* The new maintenance decision and plan are
  `docs/decisions/2026-11-04-docs-plan-namespace-compatibility-deliberation.md`
  and `docs/plans/2026-11-04-docs-plan-namespace-compatibility-plan.md`.
  Their exact working-tree state remains uncommitted
* The pre-existing untracked note
  `docs/memory/2026-11-05/ship-212-s-pr-only-blocked.md` remains untouched

## Blocker and Deferred Capture

The docline policy currently recognizes `docs/exec-plans/` as `plan` but
classifies `docs/plans/` as `guide`. Source/config/harness implementation is
Ship-owned and remains unfinished.

The separate maintenance plan passed its initial source-entrypoint authoring
check as reported by Ship, was hardened, and received `FAIL` on review attempt
1. The installed Stage text at `.github/agents/_stage.agent.md:612-615` says:

> For each major implementation unit in the plan, create a sub-epic with:
>
> * Title matching the plan section
> * Parent reference to the top-level feature or chore via `parent_id`
> * Scope boundary description

Stage also requires tasks to reference a feature parent. The live WIT has no
epic type; feature and chore allow task/review children, and task allows
subtask children. The harvest skill specifies feature-to-task mapping.

Captured deferred scope expansion `4F1C9224` records this separate hierarchy
contract reconciliation with provisional high priority and mandatory
deliberation. It cites PR #643 and the maintenance plan, with unavailable
review-thread and work-item identifiers explicitly marked `N/A`.

## Gates and Resumption

Resolve the hierarchy contract through the deferred intake, then revise and
re-review the maintenance plan. Obtain Ship's harvest-time source-entrypoint
authoring evidence before backlog creation. If those gates pass, create the
supported hierarchy and separate queued maintenance shipment; Ship must satisfy
the applicable harness and execution gates before source implementation.
Final validation and a scoped commit/push remain outstanding.

The active same-session Stage checkpoint is
`.backlogit/checkpoints/checkpoint-20261105-022125.json`; the prior
`checkpoint-20261105-021644.json` is already resolved. The five previously
reported optional SDK-anchor warnings remain pre-existing limitations, not a
passing reference check. No Stage lint, build, test, source edit, claim, or merge
is authorized as part of this handoff.
