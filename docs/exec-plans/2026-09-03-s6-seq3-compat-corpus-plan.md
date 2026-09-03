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

## Dependency Graph

U1 -> U2 (shared runner); U3 independent analyzer track. Single domain per unit.

## Runtime Verification and Closure

Verification surface = corpus runner + analyzers in CI. Closure = green corpus +
analyzers on the check target. No production behavior change.

### Plan Hardening Signals (REQUIRED)

* public API/schema/contract change: absent.
* security/auth/permission/compliance-sensitive: absent.
* migration/backfill/destructive/irreversible: absent.
* external integration/operator checkpoint/external dependency: absent.
* high runtime/rollout/rollback risk: absent.

Requires plan hardening: no

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: PASS

Personas dispatched: Correctness Reviewer (always-on), Architecture Strategist (always-on). Plan hardening NOT required (Requires plan hardening: no).

Findings and remediation:
- Correctness P2 (U3 bundled five nontrivial AST analyzers into one unit, over the 2-hour rule): REMEDIATED — U3 is now harvested as one subtask per analyzer (five), each with its own seeded-violation + clean-pass acceptance, sharing the analyzer harness scaffolding.
- Architecture: clean — corpus vs analyzers cleanly separated.

No P0/P1. No residual blocking items.
