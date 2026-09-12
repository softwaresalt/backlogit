---
chunk_strategy: h1-h2-h3
compacted_at: 2026-09-12T02:40:00Z
compacted_from:
  - docs/memory/2026-09-10-140s-declaration-behavior-split.md
  - docs/memory/2026-09-10-ship-140-s-declaration-behavior-split-blocked.md
  - docs/memory/2026-09-10-ship-140-s-dependency-pin-blocked.md
  - docs/memory/2026-09-10-ship-140-s-harness-blocked.md
  - docs/memory/2026-09-10-ship-140-s-stage-manifest-handback.md
  - docs/memory/2026-09-10-ship-140-s-wave-admission-halt.md
  - docs/memory/2026-09-10-stage-140s-harness-contract-hardening.md
doc_type: learning
schema_version: "1.0"
source: docs/memory/compacted/2026-09-11-140s-s6-compatibility-corpus-compacted.md
source_shipment: 140-S
title: "Compacted Session: 140-S S6 Compatibility Corpus"
---

## Outcome

Shipment `140-S` and feature `158-F` delivered the S6 compatibility corpus,
bounded fuzzing, cancellation and concurrency fixtures, and FL001-FL005 static
analyzers through PR #436. The implementation merged to `main` through merge
commit `c5978bc2`. The newest pre-PR checkpoint remains in place at
`docs/memory/2026-09-11-ship-140-s-pre-pr.md`.

## Final Decisions

* Declaration-only prerequisites `158.001-T` and `158.003-T` were separated
  from behavior tasks `158.009-T` and `158.010-T` to preserve executable
  RED-to-GREEN ordering
* `158.011-T` became the sole serialized owner of FL002-FL005 multichecker
  wiring, eliminating parallel ownership of `cmd/faultline-analyze/main.go`
* Analyzer implementation remained AST plus type information only; SSA, CFG,
  and cross-function data flow stayed out of scope
* `golang.org/x/tools v0.39.0` was promoted from the already selected module
  graph without a version move
* Compatibility outcomes use strict adapter validation and sentinel errors
  rather than unstable message matching
* The fuzz target remains single-package, bounded to 30 seconds, and backed by
  committed native seeds plus deterministic replay
* `compatcorpus.report/v1` is stable and sorted; S10 DAG consumption remains
  separate future scope

## Halt and Recovery History

Ship correctly halted for:

* an underspecified harness contract
* an incompatible dependency-pin mandate
* declaration and behavior bundled into non-compiling test-first units
* wave admission before the manifest and dependency graph matched the reviewed
  contract

Stage resolved each contract defect, re-reviewed the amended plan, and handed
the same bounded shipment back to Ship. No halt was bypassed.

## Verification and Closure

* The complete compatibility-corpus package suite passed
* Representative compatibility-corpus tests passed with the race detector
* FL001-FL005 were registered exactly once and produced no diagnostics on
  shipment-owned clean packages
* Six PR checks passed on head `6b7029ea`; no merge-commit CI run exists
  because CI is `pull_request`-only
* The post-merge condition was satisfied through merge parent and content
  confirmation, synced `main`, local smoke testing, and a healthy 30-minute
  observation
* No production service, public API, schema, migration, or deployment surface
  changed

## Preserved Residual IDs

Known baselines and pre-existing scope:

* `92F79833`
* `4DB1DFF1`
* `CC0EBB59`

P-021 deferred-scope captures:

* `6EB55AE6`
* `E6EE8944`
* `E2136C59`
* `E08F7890`
* `3F7782B4`
* `0197BDA3`
* `5944CE03`
* `B58C24FA`
* `2947C941`
* `EB427E20`
* `31E484D5`

These IDs remain future Stage triage scope and are not absorbed into `140-S`.

## Archived Originals

The seven verbose originals are preserved byte-for-byte under
`docs/archive/memory/`. The final actionable plan is retained in
`docs/exec-plans/2026-09-11-s6-compatibility-corpus-decided-plan.md`; both
review-heavy source plans are preserved under `docs/archive/plans/`.
