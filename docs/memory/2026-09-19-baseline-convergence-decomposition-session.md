---
doc_type: memory
schema_version: "1.0"
title: 'Stage session — baseline-convergence restage/decomposition (PR #449)'
status: complete
---

# Stage Session: Baseline-Convergence Restage & Decomposition

**Date:** 2026-09-19 (updated PR #449 review cycle 3)
**Branch:** `stage/baseline-convergence-decomposed` (from `origin/main` @ 37a5cba)
**Reviewed head:** `e0f77878f1bdec473bfbf6623f77036f0bb90a49` (PR #449 cycle-3 review census head;
the cycle-3 review-fix commit succeeds it)
**Trigger:** Operator direct directive — abandon PR #448, restage, decompose 156-S. Subsequently
hardened under PR #449 review (cycles 1–3).

## What was done

- Created clean staging branch from `origin/main` (P-016: no second worktree).
- Ported only Stage-owned backlog (175-F + 98 tasks, 168.001-T archive move) and
  8 planning docs. Excluded all Ship-authored implementation (`ci.yml`, verifier
  scripts, tests, runtime). No Go source touched.
- Decomposed single oversized shipment `156-S` (175-F + 98 tasks) into **13
  wave-aligned replacement shipments** `157-S`..`169-S` (RS-W00..RS-W12),
  task-IDs-only (no 175-F membership), chained by `blocks` deps (later joined by the
  RS-W(-1) prerequisite shipment `176-S`; see cycle 2 below → **14 shipments** total).
- Non-destructive supersession of `156-S`: kept `queued`, cleared `items: []`,
  added `SUPERSEDED — DO NOT CLAIM` banner + guard edge `156-S depends_on 169-S`.
- Rewired `149-S`: removed `→156-S`, added `→169-S` (advisory ordering; governed by
  claim-routing policy, see stash `6434A4D7`).
- `168.001-T` remains archived/`done` (not repeated).
- **PR #449 review cycle 2 — runner-bootstrap prerequisite added.** The three shared
  lint runners (`scripts/verify-task-lint.ps1`, `scripts/verify-baseline-lint.ps1`,
  `scripts/verify-terminal-lint.ps1`) had no owning task. Added one runner-bootstrap
  prerequisite task `175.099-T` (99th executable member) in a new prerequisite shipment
  `176-S` (RS-W(-1)), gating the replacement sequence via `157-S depends_on 176-S` and
  reinforced by `175.001-T depends_on 175.099-T`. Feature now **99 tasks / 14 shipments**.
- **PR #449 review cycle 3 — bootstrap harness made behavioral.** The `175.099-T`
  bootstrap harness was strengthened from existence/AST/param-only (vacuous, so a no-op
  script could pass) to a behavioral, fixture-driven contract that exercises each runner's
  success and failure/fail-closed scenarios with divergent exit codes, while preserving
  the gate invariant (the owned Go harness — never the created runner — is the gate).
  This durable session handoff was updated to the 99-task/14-shipment graph.
- Review cycles: Correctness Reviewer PASS (initial, 3 P3 advisories dispositioned) plus
  Copilot PR #449 cycles 1–3 (dependency-guard honesty, runner-bootstrap prerequisite,
  behavioral bootstrap harness).
- Both mechanical validators PASS; live-DB + file re-checks PASS.

## Wave → shipment map (task numbers of 175.NNN-T)
RS-W(-1)(176)=[99] runner-bootstrap prerequisite (`175.099-T`); gates the sequence via `157 depends_on 176`.
W0(157)=[1]; W1(158)=[2-10,40]; W2(159)=[11,13-21]; W3(160)=[22-30];
W4(161)=[31-39]; W5(162)=[41,42,45-51]; W6(163)=[43,52-59]; W7(164)=[44,60-67];
W8(165)=[68-76]; W9(166)=[77-85]; W10(167)=[86-94]; W11(168)=[95-98]; W12(169)=[12].
Remediation counts: 1,10,10,9,9,9,9,9,9,9,9,4,1 = 98 across 13 shipments (157-S..169-S).
Total: 98 remediation + 1 bootstrap (175.099-T) = **99 tasks**; 13 replacement + 1 prerequisite
(176-S) = **14 shipments**.

## Dependency edges
Prerequisite gate: `157-S depends_on 176-S`; `175.001-T depends_on 175.099-T` (task-level
reinforcement — U1 waits for the runner bootstrap). `176-S`/`175.099-T` is the in-degree-zero
source of the full feature graph.
Chain: 158→157, 159→158, ..., 169→168. 149-S→169-S (advisory). 156-S→169-S (advisory guard).
150-S/151-S→149-S preserved.

## Invariants (all PASS)
INV1 99 unique executable tasks once (98 remediation + `175.099-T` bootstrap); INV2 no 175-F in
any shipment; INV3 all queued (157-S..169-S + 176-S); INV4 acyclic ordered chain incl. the
`176→157` / `175.099→175.001` prerequisite gate; INV5 149-S ordered behind 169-S (advisory);
INV6 168.001-T done; INV7 only `.backlogit/` + `docs/` changed; INV8 176-S is the single
prerequisite shipment, sole owner of `175.099-T`, present in no other shipment; INV9 the
`175.099-T` bootstrap gate is buildable before its runners exist (RED before / GREEN after).

## Provenance preserved
Old branch `stage/baseline-convergence-main` @ d4d394e1 untouched (read-only evidence).
PR #448 closed unmerged. `156-S` retained as historical packaging.

## Handoff
PR #449 open on branch `stage/baseline-convergence-decomposed`; hardened through review cycle 3
(cycle-3 census head `e0f77878f1bdec473bfbf6623f77036f0bb90a49`, two unresolved Copilot threads
addressed by the succeeding cycle-3 review-fix commit: behavioral bootstrap harness for
`175.099-T` and this durable-handoff update). Current graph: **99 tasks / 14 shipments**. CI
green; no active shipment. Orchestrator owns the staging merge gate. Not for Ship until merged.
No BDD retrofit (deferred spec preserved). Governance follow-ups remain in stash: `6434A4D7`
(tool-level prerequisite/claim enforcement) and `B633E9B9` (machine-authenticated
reauthorization).

## Out of scope (left active)
4 unrelated deferred-scope-expansion stash entries: 3F06493B, 7B71AD77,
156F65EB, FF6D467A.
