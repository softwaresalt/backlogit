---
chunk_strategy: h1-h2-h3
description: "Execution plan for S6: Sequence 3/7 compatibility corpus, fuzzing, and targeted static analysis"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-03-s6-seq3-compat-corpus-plan.md
title: "S6 Execution Plan — Sequence 3/7 Compatibility Corpus & Static Analysis"
---

# S6 Execution Plan — Compatibility Corpus, Fuzzing & Static Analysis (Seq 3/7)

**Covering feature**: Compatibility corpus, fuzzing, and targeted static analysis
**Stash member**: B212512E (high)
**Tier**: feature (shipment sequence S6)

## Problem Frame

Parser, platform, compile, and error-path defects (malformed/truncated JSON/YAML,
duplicate/case-folded keys, CRLF/LF, oversized scanner tokens, old index versions,
Windows semantics, lock contention, context cancellation, ambiguous gate inputs)
reach review. Build a shared regression corpus with deterministic checks plus Go
analyzers for Scanner discipline, %w wrapping, fail-open branches, success-after-
audit-warning, and uncancellable-lock timeout claims.

## Constitution Check

| Principle | Compliance |
|-----------|-----------|
| I. Safety-First Go | Go 1.24; analyzers enforce discipline |
| II. Test-First (P-002) | corpus entry -> RED -> GREEN per unit |
| III. Workspace Isolation | corpus fixtures isolated |
| IV. CLI Containment | n/a |
| V. Observability | analyzer output structured |
| VI. Single Responsibility | corpus vs analyzers separated |
| VII. Destructive Approval | none |
| VIII. Safety Modes | fail-open branch analyzer enforces fail-closed |
| IX. Git-Friendly | text corpus |
| X. Context Efficiency | n/a |
| XI. Merge Commits | P-009 by Ship |

Constitution Check: pass

## Implementation Units

### U1 — Shared malformed-input compatibility corpus
* Scope: assemble the corpus (malformed/truncated JSON/YAML, duplicate/case-folded keys, CRLF/LF, oversized tokens, old db/index versions, Windows semantics) with a deterministic runner.
* Acceptance: runner executes the corpus against current parsers and reports pass/fail deterministically.

### U2 — Concurrency/cancellation fixtures
* Scope: lock-contention, context-cancellation, and ambiguous-gate-input fixtures wired to the runner.
* Acceptance: each fixture asserts the expected safe outcome. Depends on U1.

### U3 — Targeted Go static analyzers
* Scope: analyzers for Scanner.Buffer/Scanner.Err discipline, %w wrapping, fail-open error branches, success returns after audit warnings, and timeout claims reaching uncancellable locks. Each analyzer is nontrivial AST analysis, so to respect the 2-hour rule and single-responsibility this unit is harvested as ONE subtask PER analyzer (five subtasks), each with its own seeded-violation + clean-pass acceptance; they share the analyzer harness scaffolding.
* Acceptance: each of the five analyzers flags a seeded violation and passes clean code; each is wired into the check target; each lands as an independently verifiable subtask.

### U-fuzz — Bounded compatibility fuzz target
* Scope: add the Go fuzz target `FuzzCompatibilityCorpusDecode` in a single owning package (`internal/faultline/compatcorpus`) that drives the compatibility corpus decoder through malformed/truncated JSON/YAML, duplicate or case-folded keys, CRLF/LF variants, oversized-token boundaries, and old-version fixtures, reusing U1's corpus-runner semantics. Seeds use Go's package-local built-in fuzz corpus directory `internal/faultline/compatcorpus/testdata/fuzz/FuzzCompatibilityCorpusDecode/` (the single canonical seed location), augmented via `f.Add` from the deterministic corpus; only minimal non-secret fixtures.
* Execution budget: CI runs the target against its single owning package with a bounded budget: `go test -run=^$ -fuzz=^FuzzCompatibilityCorpusDecode$ -fuzztime=30s ./internal/faultline/compatcorpus` (Go's `-fuzz` requires exactly one package, so `./...` is not used), or an equivalent fixed-count local harness if CI cannot run Go fuzzing directly.
* Acceptance: the package-local seed corpus is committed; the fuzz target is crash-free over the configured budget; any discovered crashing input is minimized, committed to the corpus, and converted into a deterministic regression before the unit closes. Depends on U1 (reuses U1's corpus runner).

## Dependency Graph

U1 -> U2 (shared runner); U3 independent analyzer track; U-fuzz depends on U1's corpus runner and remains a bounded, independent fuzzing unit. Single domain per unit.

## Runtime Verification and Closure

Verification surface = corpus runner + analyzers + bounded fuzz target in CI. Closure = green corpus +
analyzers on the check target. No production behavior change.

### Plan Hardening Signals (REQUIRED)

* public API/schema/contract change: absent.
* security/auth/permission/compliance-sensitive: absent.
* migration/backfill/destructive/irreversible: absent.
* external integration/operator checkpoint/external dependency: absent.
* high runtime/rollout/rollback risk: absent.

Requires plan hardening: no

## Prior Plan Review (invalidated)

dispatch_mode: multi-agent-dispatch
decision: INVALIDATED

The prior PASS record is retained only as invalidated history. It omitted mandatory personas and is superseded by the genuine multi-agent Plan Review below.

## Plan Review

<!-- plan-review-attempt: 2 -->

dispatch_mode: multi-agent-dispatch
decision: FAIL

personas:
* Constitution Reviewer (`claude-opus-4.8`)
* Go Reviewer, anchor (`gpt-5.6-sol`, effort high)
* Scope Boundary Auditor (`gemini-3.7-flash`)
* Correctness Reviewer (`claude-sonnet-4.6`)
* Architecture Strategist (`grok-4.6`)
* Security Reviewer (`gpt-5.6-terra`) when risk-triggered for the plan
* Learnings Researcher over `docs/compound/`

Security Reviewer was not risk-triggered for this plan; all other mandatory personas ran.

Controlling P1 findings:
* The plan advertised fuzzing but had no fuzz target, seed corpus, execution budget, or implementation unit.
* The `success-after-audit-warning` and `uncancellable-lock timeout` analyzers are underspecified as bounded AST checks and likely require CFG, data-flow, or SSA scope.
* Analyzer source/sink and wrap-boundary definitions are needed to avoid noisy or unsafe fail-open checks.
* The runner output path into the S4 U4 evidence contract and S10 DAG remains unstated.

## Plan Review

<!-- plan-review-attempt: 3 -->

dispatch_mode: multi-agent-dispatch
decision: PASS

The attempt-2 FAIL is superseded. The executable harness contract that resolves
every attempt-2 P1 finding is authored in the supplement
`docs/exec-plans/2026-09-10-s6-140s-harness-contract-supplement.md`, which carries
its own genuine attempt-3 multi-agent Plan Review (decision PASS) over the
concrete task-by-task contracts for `158.001-T`..`158.008-T`. This governing plan
remains the scope authority; the supplement supplies the executable detail (API
signatures, sentinel-error corpus outcomes, analyzer source/sink boundaries with
declaration-driven allowlists, fuzz target + canonical seeds + budget, wave
ordering, and check-target ownership) without changing intended product scope.

Attempt-2 P1 dispositions (detail in the supplement's resolution tables):
* Fuzzing — RESOLVED: `U-fuzz` / `158.008-T` with exact API, canonical seed dir,
  30s single-package budget, and a valid seed-regression RED.
* success-after-audit-warning / uncancellable-lock analyzers — RESOLVED: both
  re-scoped to AST + type-info, intra-block/function, declaration-driven
  allowlists; NO CFG/SSA/data-flow.
* analyzer source/sink & wrap boundaries — RESOLVED: declared per analyzer with
  explicit false-positive exclusions.
* runner output into S4-U4 / S10 DAG — RESOLVED: stable `compatcorpus.report/v1`
  surface; DAG wiring explicitly deferred to S10 (out of 140-S scope).

Gate: **PASS** — executable and scope-bounded via the supplement. Ready for Ship.

## Amendment — Declaration/Behavior Split (attempt-4, unblocks 140-S)

**Governing finding (P-002.1/P-002.6).** Test-first RED→GREEN does not permit an
in-task two-stage harness: a single task may not bundle new package/symbol
*declarations* with the *behavior harness* that cannot compile until those
declarations exist. Two original units bundled exactly that; this Stage manifest
amendment splits them (only a Stage amendment may change the frozen work graph).
Product scope is unchanged — the same corpus, adapters, analyzers, fuzz target,
and acceptance are delivered; they are only re-partitioned across tasks with
correct RED→GREEN ordering.

**Unit → task mapping after the split:**

| Governing unit | Declaration prerequisite | Behavior task |
|----------------|--------------------------|---------------|
| U1 (corpus + runner) | `158.001-T` — compat corpus package **declarations** + source-shape go/ast harness | `158.009-T` — runner + adapters + fixtures + `report/v1` behavior (all original U1 acceptance) |
| U3a (FL001 + scaffolding) | `158.003-T` — x/tools promotion + `cmd/faultline-analyze` skeleton + `check` target + FL001 **declaration** | `158.010-T` — FL001 scanner-discipline **behavior** + `analysistest` |
| U2 (`158.002-T`), U3b–U3e (`158.004-T`..`158.007-T`), U-fuzz (`158.008-T`) | unchanged units | rewired dependencies (below) |

**Final backlog dependency edges (real `blocks` edges):**

* `158.009-T → 158.001-T`; `158.010-T → 158.003-T`
* `158.004-T → 158.003-T`; `158.005-T → 158.003-T`; `158.006-T → 158.003-T`; `158.007-T → 158.003-T`
* `158.002-T → 158.009-T`; `158.008-T → 158.009-T` (rewired off `158.001-T`; their real dependency is the implemented runner)
* `158.011-T → 158.004-T`; `158.011-T → 158.005-T`; `158.011-T → 158.006-T`; `158.011-T → 158.007-T` (serialized multichecker-wiring integration; see below)
* `158-F → 156.006-T` (unchanged)

**Serialized multichecker-wiring task (`158.011-T`).** Added during the plan
review (Go-anchor remediation) to remove parallel shared-file ownership of
`cmd/faultline-analyze/main.go`: `158.003-T` authors `main.go` with FL001 wired
live + FL002..FL005 reserved as commented slots; the analyzer tasks
`158.004-T`..`158.007-T` own only their subpackages; `158.011-T` is the single
serialized owner that fills the four FL002..FL005 slots after those analyzer vars
exist.

**Wave schedule:** Wave 1 (prereqs, parallel): `158.001-T`, `158.003-T`. Wave 2
(after prereq, parallel): `158.009-T`, `158.010-T`, `158.004-T`, `158.005-T`,
`158.006-T`, `158.007-T`. Wave 3 (after `158.009-T`): `158.002-T`, `158.008-T`.
Wave 4 (after `158.004-T`..`158.007-T`): `158.011-T`.

**Manifest `140-S`:** members are `158-F` + `158.001-T`..`158.011-T` (12 items).
The concrete per-task contracts (API signatures, RED/GREEN selectors, ownership)
live in the supplement `docs/exec-plans/2026-09-10-s6-140s-harness-contract-supplement.md`,
which carries the authoritative attempt-5 Plan Review over the split.

## Plan Review

<!-- plan-review-attempt: 4 -->

dispatch_mode: multi-agent-dispatch
decision: PASS

**Scope of this attempt:** a re-review of the P-002.1/P-002.6 declaration/behavior
split that converts `158.001-T` and `158.003-T` to declaration-only prerequisites,
adds behavior tasks `158.009-T` and `158.010-T`, promotes the analyzer harness
ordering into real backlog `blocks` edges, and rewires the two runner consumers
onto `158.009-T`. No product scope was reopened; the split only re-partitions
existing, already-reviewed scope with correct RED→GREEN ordering.

personas (genuine multi-agent dispatch over the amended governing plan + the
supplement + the mutated backlog topology):
* Constitution Reviewer (`claude-opus-4.8`) — II Test-First / P-002 ordering.
* Go Reviewer, anchor (`gpt-5.6-terra`, effort high) — declaration vs behavior
  compile ordering, x/tools promotion unchanged.
* Architecture Strategist (`grok-4.6`) — dependency DAG, wave partition.
* Scope Boundary Auditor (`gemini-3.7-flash`) — no scope creep; dark scope
  `[140-S]` only.
* Correctness Reviewer (`claude-sonnet-5`) — acyclic edges, no duplicate
  ownership.
* Template/Backlog Integrity — frontmatter valid, docs lint clean, manifest
  membership + edges consistent.

Findings and dispositions: no open P0/P1. The behavior tasks are strictly smaller
than the single units they were carved from (each previously reviewed as one ≤2h
unit); sizing/complexity semantics preserved (all members remain unsized, matching
the existing shipment composition). The dependency graph is acyclic with roots
`158.001-T`/`158.003-T`; no task owns another task's file logic (FL001 `main.go`
slot filled once by `158.003-T`).

Gate: **PASS** — this is the authoritative final governing-plan verdict; it
supersedes the attempt-3 record for the task-partition surface only. Ready for
Ship to re-claim `140-S`.
