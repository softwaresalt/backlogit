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
* Scope: add the Go fuzz target function `FuzzCompatibilityCorpusDecode` that drives the compatibility corpus decoder through malformed/truncated JSON/YAML, duplicate or case-folded keys, CRLF/LF variants, oversized-token boundaries, and old-version fixtures using the same runner semantics as U1. The committed seed corpus lives under `tests/testdata/compat-corpus/fuzz/` and contains only minimal non-secret fixtures derived from the deterministic corpus.
* Execution budget: CI runs the target with a bounded budget, for example `go test ./... -run=^$ -fuzz=FuzzCompatibilityCorpusDecode -fuzztime=30s`, or an equivalent fixed-count local harness if CI cannot run Go fuzzing directly.
* Acceptance: the seed corpus is committed; the fuzz target is crash-free over the configured budget; any discovered crashing input is minimized, committed to the corpus, and converted into a deterministic regression before the unit closes. Independent fuzzing unit.

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
