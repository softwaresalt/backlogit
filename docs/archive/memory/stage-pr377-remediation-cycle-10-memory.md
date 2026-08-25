---
doc_type: memory
session: stage-d3ce9e81-2026-08-24
cycle: 10
date: 2026-08-24
author: stage-agent
---

# Stage PR #377 Remediation Cycle 10 Memory

Cycle 10 was an operator-authorized extension past the three-cycle review-fix limit.

## Findings and Corrections

Three current-head Copilot review findings were addressed:

### (a) 147.016-T / U8b — `Conforming == false` zero-value ambiguity

The `valid-but-non-conforming` row's `result.Conforming == false` assertion is a bool zero value and does not prove `ListCheckpoints` populated the projection; an unset declaration stub also yields `false`.

**Fix:** U8b's harness now asserts at least one required positive projected field alongside `Conforming == false`: `Valid == true`, `NeedsQuarantine == true`, and `RemediationCommand != ""`. Against the declaration stub's zero-value result, all three fail — proving the projection is unpopulated rather than relying on the ambiguous zero-value alone. Plan U8b description and 147.016-T acceptance criteria updated.

### (b) 147.018-T / U9b — post-quarantine restore abort cleanup lacks P-VII approval gate

Entry point (b)'s abort step can delete an invalid restored active copy, but Constitution Principle VII requires explicit approval before every destructive file action. The approval gate was missing.

**Fix:** U9b's entry point (b) now requires explicit operator approval for the potential abort cleanup before the restore begins (step 2). If approval is withheld, restoration must not start. Plan U9b description and 147.018-T acceptance criteria updated.

### (c) 147.010-T / U5b — production delta contradicts origin decision scope exclusion

U5b's cycle-8 production delta (changing the `QuarantineCheckpoint` refusal sentinel for non-active conforming targets from `ErrCheckpointUseAbandon` to `ErrCheckpointNotActive`) widens the decision's explicitly out-of-scope state-conflict class.

**Fix:** U5b is **retired**. Its state-conflict regression rows are absorbed into U5 (147.009-T) as already-green pinned guards. U5 retains its genuine red gate (row 1). 147.010-T is archived, its dependency edge removed, and its shipment membership removed. Plan U5b section replaced with a retirement notice.

## Backlog Shape Change

- Before: 27 tasks / 43 edges / 28 shipment members
- After: 26 tasks / 42 edges / 27 shipment members
- 1 task retired (147.010-T / U5b archived)
- 1 edge removed (U5 → U5b)
- 1 shipment member removed (147.010-T from 130-S)

## Files Modified

- `.backlogit/queue/147.016-T.md` — positive-projection assertions added
- `.backlogit/queue/147.018-T.md` — Principle VII approval gate added to restore abort
- `.backlogit/queue/147.009-T.md` — U5b regression rows absorbed
- `.backlogit/archive/147.010-T.md` — archived (moved from queue)
- `docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md` — cycle-10 remediation section added; U5b retirement notice; dep graph, edge table, execution order, Constitution Check II, I3 discussion, runtime verification table, stop conditions updated
- `.backlogit/checkpoints/checkpoint-20260824-191617.json` — updated through cycle-10 final state
- `.backlogit/memories.json` — entry updated through cycle-10

## Decisions

- U5b retirement: accepted per origin decision scope exclusion; state-conflict regression rows belong in U5 as already-green guards
- Positive-projection assertions in U8b: three fields (`Valid`, `NeedsQuarantine`, `RemediationCommand`) make the red gate unambiguous
- Principle VII gate: restore abort requires operator approval before the restore begins

## Next Steps

- Ship to receive handoff at PR #377 head (query live PR for CI/review state)
- If further Copilot review cycles are needed, each requires explicit operator authorization
