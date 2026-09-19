---
doc_type: memory
schema_version: "1.0"
title: 'Stage session — baseline-convergence restage/decomposition (PR #448)'
status: complete
---

# Stage Session: Baseline-Convergence Restage & Decomposition

**Date:** 2026-09-19
**Branch:** `stage/baseline-convergence-decomposed` (from `origin/main` @ 37a5cba)
**Commit:** `46e17ee0d08f4ed9d5f58121ad13b4830627cb31`
**Trigger:** Operator direct directive — abandon PR #448, restage, decompose 156-S.

## What was done

- Created clean staging branch from `origin/main` (P-016: no second worktree).
- Ported only Stage-owned backlog (175-F + 98 tasks, 168.001-T archive move) and
  8 planning docs. Excluded all Ship-authored implementation (`ci.yml`, verifier
  scripts, tests, runtime). No Go source touched.
- Decomposed single oversized shipment `156-S` (175-F + 98 tasks) into **13
  wave-aligned replacement shipments** `157-S`..`169-S` (RS-W00..RS-W12),
  task-IDs-only (no 175-F membership), chained by `blocks` deps.
- Non-destructive supersession of `156-S`: kept `queued`, cleared `items: []`,
  added `SUPERSEDED — DO NOT CLAIM` banner + guard edge `156-S depends_on 169-S`.
- Rewired `149-S`: removed `→156-S`, added `→169-S` (gated until full sequence ships).
- `168.001-T` remains archived/`done` (not repeated).
- One focused review cycle (Correctness Reviewer → PASS, 3 P3 advisories dispositioned).
- Both mechanical validators PASS; live-DB + file re-checks PASS.

## Wave → shipment map (task numbers of 175.NNN-T)
W0(157)=[1]; W1(158)=[2-10,40]; W2(159)=[11,13-21]; W3(160)=[22-30];
W4(161)=[31-39]; W5(162)=[41,42,45-51]; W6(163)=[43,52-59]; W7(164)=[44,60-67];
W8(165)=[68-76]; W9(166)=[77-85]; W10(167)=[86-94]; W11(168)=[95-98]; W12(169)=[12].
Counts: 1,10,10,9,9,9,9,9,9,9,9,4,1 = 98.

## Dependency edges
Chain: 158→157, 159→158, ..., 169→168. 149-S→169-S. 156-S→169-S (guard).
150-S/151-S→149-S preserved.

## Invariants (all PASS)
INV1 98 unique tasks once; INV2 no 175-F in any shipment; INV3 all queued;
INV4 acyclic ordered chain; INV5 149-S gated on 169-S; INV6 168.001-T done;
INV7 only `.backlogit/` + `docs/` changed.

## Provenance preserved
Old branch `stage/baseline-convergence-main` @ d4d394e1 untouched (read-only evidence).
PR #448 closed unmerged. `156-S` retained as historical packaging.

## Handoff
Pushed branch (no PR). Orchestrator owns the staging merge gate.
Not for Ship until merged. No BDD retrofit (deferred spec preserved).

## Out of scope (left active)
4 unrelated deferred-scope-expansion stash entries: 3F06493B, 7B71AD77,
156F65EB, FF6D467A.
