---
chunk_strategy: h1-h2-h3
decided_at: 2026-09-12T02:40:00Z
decided_from:
  - docs/archive/plans/2026-09-03-s6-seq3-compat-corpus-plan.md
  - docs/archive/plans/2026-09-10-s6-140s-harness-contract-supplement.md
doc_type: learning
schema_version: "1.0"
source: docs/exec-plans/2026-09-11-s6-compatibility-corpus-decided-plan.md
source_shipment: 140-S
title: "Decided Plan: S6 Compatibility Corpus, Fuzzing, and Static Analysis"
---

## Decision Record

Shipment `140-S`, feature `158-F`, and tasks `158.001-T` through `158.011-T`
implemented a deterministic malformed-input compatibility corpus, concurrency
and cancellation fixtures, bounded Go fuzzing, and five targeted static
analyzers. Final plan review outcome: **PASS**.

## Executed Scope

| Unit | Task ownership | Final contract |
|---|---|---|
| Corpus declarations | `158.001-T` | Compilable package declarations plus source-shape harness |
| Corpus behavior | `158.009-T` | Runner, adapters, fixtures, sentinels, and `compatcorpus.report/v1` |
| Concurrency and cancellation | `158.002-T` | Lock, cancellation, ambiguity, and panic-containment fixtures |
| Analyzer scaffold | `158.003-T` | `x/tools` promotion, FL001 declaration, multichecker skeleton, and check target |
| FL001 behavior | `158.010-T` | Scanner discipline analyzer and `analysistest` fixtures |
| FL002-FL005 | `158.004-T` through `158.007-T` | Isolated analyzer packages with seeded and clean fixtures |
| Bounded fuzzing | `158.008-T` | Single-package fuzz target, committed seeds, and deterministic replay |
| Analyzer wiring | `158.011-T` | Sole serialized owner of FL002-FL005 multichecker registration |

## Final Dependency and Wave Decisions

* Wave 1: `158.001-T`, `158.003-T`
* Wave 2: `158.009-T`, `158.010-T`, `158.004-T` through `158.007-T`
* Wave 3: `158.002-T`, `158.008-T`
* Wave 4: `158.011-T`

Behavior depends on declarations. Runner consumers depend on `158.009-T`.
Analyzer behavior depends on `158.003-T`. Final multichecker wiring depends on
all four FL002-FL005 analyzer packages.

## Contract Decisions

* Analyzer framework: `golang.org/x/tools` `go/analysis`, `analysistest`, and
  `multichecker`
* Dependency version: promote already selected `v0.39.0`; no upgrade,
  downgrade, or unrelated module-chain move
* Analysis depth: AST plus type information only
* Diagnostic IDs: FL001 Scanner discipline, FL002 `%w` wrapping, FL003
  fail-open branches, FL004 success after audit warning, FL005 timeout claims
  reaching uncancellable locks
* Confidence posture: high-confidence under-approximation with explicit
  exclusions and suppression comments
* Corpus outcomes: strict adapter validation with `errors.Is`-compatible
  sentinels
* Report: stable sorted `compatcorpus.report/v1`
* Fuzzing: `FuzzCompatibilityCorpusDecode`, one package, 30-second budget,
  committed native seeds, deterministic seed replay

## Preserved Boundaries

* No production runtime behavior
* No public API or persisted schema change
* No SSA, CFG, or cross-function data-flow expansion
* No repository-wide analyzer remediation
* No S10 evidence-DAG node integration
* No duplicate shared-file ownership
* No dependency version movement beyond direct promotion of the selected
  `x/tools` version

## Rejected Alternatives

* Bundling new declarations with behavior tests that cannot compile in RED
* Forcing `x/tools v0.28.0` and downgrading the selected module graph
* Parsing library error strings instead of matching sentinels
* Running Go fuzzing across `./...`
* Allowing each analyzer task to edit the shared multichecker entry point
* Expanding the analyzers into SSA, CFG, or general data-flow analysis

## Source Traceability

The review-heavy source plans remain available at:

* `docs/archive/plans/2026-09-03-s6-seq3-compat-corpus-plan.md`
* `docs/archive/plans/2026-09-10-s6-140s-harness-contract-supplement.md`
