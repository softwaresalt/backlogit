---
title: "Stage session — 140-S declaration/behavior split"
date: 2026-09-10
shipment: 140-S
feature: 158-F
branch: plan/140-s-declaration-behavior-split
---

# Stage Session Memory — 140-S Declaration/Behavior Split

## Objective
Authorized Stage manifest amendment to satisfy P-002.1/P-002.6: split in-task
two-stage harnesses (declarations bundled with behavior that cannot compile
before the declarations exist) for shipment 140-S / feature 158-F. Dark scope
strictly [140-S]. Planning/decomposition only — no code, no claim, no ship.

## Governing finding
P-002.1/P-002.6 forbids an in-task two-stage harness. Two units bundled
declarations + behavior:
- 158.001-T (compat corpus decls + runner behavior)
- 158.003-T (analyzer scaffolding + FL001 declaration + FL001 behavior)

## Actions taken
- Amended 158.001-T → declaration-only prerequisite (compat corpus package
  declarations + source-shape go/ast harness `TestCompatcorpusDeclShape`).
- Amended 158.003-T → shared analyzer scaffolding + FL001 declaration
  prerequisite (x/tools v0.39.0 direct promotion, cmd/faultline-analyze skeleton
  with FL001 live + FL002..FL005 commented reserved slots, Makefile check,
  skeleton FL001 Analyzer var, source-shape `TestScaffoldShape`).
- Created 158.009-T — compat corpus runner + adapters behavior (all original
  158.001-T behavior acceptance). Depends on 158.001-T.
- Created 158.010-T — FL001 scanner-discipline behavior + analysistest. Depends
  on 158.003-T.
- Created 158.011-T — serialized integration task, sole owner of the FL002..FL005
  multichecker wiring in cmd/faultline-analyze/main.go. Depends on
  158.004-T..158.007-T. (Added during plan review to remediate a Go-anchor P1 on
  parallel shared-file ownership.)

## Final dependency edges (acyclic; roots 158.001-T, 158.003-T)
- 158.009-T → 158.001-T
- 158.010-T → 158.003-T; 158.004-T/158.005-T/158.006-T/158.007-T → 158.003-T
- 158.002-T → 158.009-T; 158.008-T → 158.009-T (rewired off 158.001-T)
- 158.011-T → 158.004-T/158.005-T/158.006-T/158.007-T
- 158-F → 156.006-T (unchanged)

## Waves
- Wave 1: 158.001-T, 158.003-T
- Wave 2: 158.009-T, 158.010-T, 158.004-T, 158.005-T, 158.006-T, 158.007-T
- Wave 3: 158.002-T, 158.008-T
- Wave 4: 158.011-T

## Manifest 140-S (12 items)
158-F + 158.001-T..158.011-T. No new shipment; only 140-S touched.

## Review
Multi-agent plan review (multi-agent-dispatch). Cycle-0: Constitution PASS, Go
anchor FAIL (2×P1), Architecture PASS, Scope PASS, Correctness ADVISORY (1×P1),
Template Integrity PASS. Remediated all P1 (Run() ctx signature; serialized
158.011-T integration owner; stale prose in 158.002/158.008). Cycle-1 re-review:
Go anchor PASS, Correctness PASS. Final authoritative verdicts: supplement
attempt-5 PASS, governing plan attempt-4 PASS.

## Sizing
All 11 member tasks left unsized to preserve existing shipment size/complexity
semantics (whole shipment was unsized). Behavior tasks strictly smaller than the
units they were carved from.

## Validation
docs lint clean (both plan docs); backlogit doctor introduced no new orphan/
duplicate for 158-*/140-S (23 pre-existing orphans 016.001-R, 106.012..106.033-T
are out of scope). DAG acyclic. Manifest parent-first.

## Untouched (as instructed)
Deferred stash CC0EBB59 and complexity-provenance stash/design preserved.

## Handoff
Ship re-claims 140-S: `backlogit shipment claim 140-S` on the existing feature
branch `feat/140-s-s6-compatibility-corpus-fuzzing-and-static-analysis` after the
plan branch is merged/available. Start wave-1 prerequisites 158.001-T, 158.003-T.
