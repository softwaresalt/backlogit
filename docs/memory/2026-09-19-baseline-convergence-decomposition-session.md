---
doc_type: memory
schema_version: "1.0"
title: 'Stage session — baseline-convergence restage/decomposition (PR #449)'
status: complete
---

# Stage Session: Baseline-Convergence Restage & Decomposition

**Date:** 2026-09-19
**Branch:** `stage/baseline-convergence-decomposed` (from `origin/main` @ 37a5cba)
**Trigger:** Operator direct directive — abandon PR #448, restage, decompose 156-S.

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
- **Runner-bootstrap prerequisite.** The three shared lint runners
  (`scripts/verify-task-lint.ps1`, `scripts/verify-baseline-lint.ps1`,
  `scripts/verify-terminal-lint.ps1`) had no owning task. Added three dependency-ordered
  runner-bootstrap prerequisite tasks `175.099-T` (task-lint), `175.100-T` (baseline-lint),
  `175.101-T` (terminal-lint) — the 99th/100th/101st executable members, one runner each
  (split from a prior oversized single bootstrap task per PR #449 cycle 9) — in a new
  prerequisite shipment
  `176-S` (RS-W(-1)), gating the replacement sequence via `157-S depends_on 176-S` and
  reinforced by `175.001-T depends_on 175.101-T` (the bootstrap sink). Feature is
  **101 tasks / 14 shipments**.
- **Behavioral bootstrap harness.** Each bootstrap harness is a behavioral,
  fixture-driven contract that exercises its runner's success and failure/fail-closed
  scenarios with divergent exit codes (existence/AST/param-only checks alone would be
  vacuous), while preserving the gate invariant (the owned Go harness — never the created
  runner — is the gate). The terminal runner's coverage independently proves both
  supported GOOS surfaces (`windows`, `linux`), fails closed on a residual warning in
  either surface, and asserts its `TERMINAL-LINT-OK:175-F` success marker.
- **Terminal gate through the canonical runner.** U12 (`175.012-T`) delegates its
  mandatory zero-warning terminal lint gate to `scripts/verify-terminal-lint.ps1
  -FeatureId 175-F`, which screens both supported GOOS surfaces (`windows`, `linux`) via
  an in-process `GOOS` override on a single host and emits `TERMINAL-LINT-OK:175-F` only
  after both surfaces are clean — replacing a Windows-native-only direct `golangci-lint
  run`.

## Wave → shipment map (task numbers of 175.NNN-T)
RS-W(-1)(176)=[99,100,101] runner-bootstrap prerequisites (`175.099-T` -> `175.100-T` -> `175.101-T`); gates the sequence via `157 depends_on 176`.
W0(157)=[1]; W1(158)=[2-10,40]; W2(159)=[11,13-21]; W3(160)=[22-30];
W4(161)=[31-39]; W5(162)=[41,42,45-51]; W6(163)=[43,52-59]; W7(164)=[44,60-67];
W8(165)=[68-76]; W9(166)=[77-85]; W10(167)=[86-94]; W11(168)=[95-98]; W12(169)=[12].
Remediation counts: 1,10,10,9,9,9,9,9,9,9,9,4,1 = 98 across 13 shipments (157-S..169-S).
Total: 98 remediation + 3 bootstrap (175.099-T, 175.100-T, 175.101-T) = **101 tasks**; 13 replacement + 1 prerequisite
(176-S) = **14 shipments**.

## Dependency edges
Prerequisite gate: `157-S depends_on 176-S`; bootstrap chain `175.099-T -> 175.100-T -> 175.101-T`;
`175.001-T depends_on 175.101-T` (task-level
reinforcement — U1 waits for the bootstrap sink / terminal-lint runner). `176-S`/`175.099-T` is the in-degree-zero
source of the full feature graph; `175.101-T` is its sink.
Chain: 158→157, 159→158, ..., 169→168. 149-S→169-S (advisory). 156-S→169-S (advisory guard).
150-S/151-S→149-S preserved.

## Invariants (all PASS)
INV1 101 unique executable tasks once (98 remediation + `175.099-T`/`175.100-T`/`175.101-T` bootstrap); INV2 no 175-F in
any shipment; INV3 all queued (157-S..169-S + 176-S); INV4 acyclic ordered chain incl. the
`176→157` / `175.099→175.100→175.101→175.001` prerequisite gate; INV5 149-S ordered behind 169-S (advisory);
INV6 168.001-T done; INV7 only `.backlogit/` + `docs/` changed; INV8 176-S is the single
prerequisite shipment, sole owner of the three bootstrap tasks, each present in no other shipment; INV9 each
bootstrap gate is buildable before its runner exists (RED before / GREEN after).

## Provenance preserved
Old branch `stage/baseline-convergence-main` @ d4d394e1 untouched (read-only evidence).
PR #448 closed unmerged. `156-S` retained as historical packaging.

## Handoff
PR #449 tracks this staging work on branch `stage/baseline-convergence-decomposed`.
Volatile review chronology, current-head evidence, and CI/readiness state are owned by
the PR and Git history, not this durable handoff. Current graph: **101 tasks / 14
shipments**. No active shipment; the Orchestrator owns the staging merge gate, and the
work is not for Ship until merged. No BDD retrofit (deferred spec preserved). Governance
follow-ups remain in stash: `6434A4D7` (tool-level prerequisite/claim enforcement) and
`B633E9B9` (machine-authenticated reauthorization).

## Out of scope (left active)
4 unrelated deferred-scope-expansion stash entries: 3F06493B, 7B71AD77,
156F65EB, FF6D467A.
