---
title: "Compacted memory: 2026-09-11 shipment 140-S pre-PR and FL005 remediation"
date_range: "2026-09-11 to 2026-09-11"
doc_type: compacted-memory
source_count: 2
---

# Compacted memory: 2026-09-11

Compacts 2 source files. The originals were moved to docs/archive/memory/ (not deleted).

## Decisions made

- Shipment 140-S (covering feature 158-F): all eleven frozen task members were done. The Stage amendment commit 37706eb4 was integrated with its identity preserved through merge commit 6448e407.
- FL005 analyzer: every real selector method named Lock or Acquire is a sink when its explicit parameters do not accept context.Context. Plain functions, function fields, and context-taking methods stay excluded.
- FL005 analysis stays syntactic and intra-function (AST ancestry and types.Scope). No CFG or SSA was introduced.
- Lock and Acquire are receiver-agnostic, but the selector must resolve to a method with a receiver. RLock stays restricted to sync.RWMutex.
- Suppressions are bound to the direct block statement and the nearest call, with exact preceding-line support.
- Task 158.007-T stays active for the owning review workflow. The local FL005 commit must not be pushed.

## Files modified or created

- internal/faultline/analyzer/locktimeout/analyzer.go
- internal/faultline/analyzer/locktimeout/locks.go
- internal/faultline/analyzer/locktimeout/testdata/src/fl005bad/bad.go
- internal/faultline/analyzer/locktimeout/testdata/src/fl005good/good.go
- The 140-S branch as a whole (compatibility corpus runner, strict adapters, deterministic concurrency fixtures, bounded fuzzing, FL001 to FL005 analyzers, five-analyzer multichecker wiring). No file list was recorded.

## Key learnings

- The final standard review of 140-S returned READY_WITH_FOLLOWUPS with P0=0, P1=0, P2=0, P3=0. A three-reviewer adversarial review returned READY.
- Windows CRLF checkout bytes make the parity_v1 golden canonical-byte test fail. This is a pre-existing repository condition (92F79833).
- There are 56 pre-existing repository-wide lint findings and line-ending-driven gofmt output (4DB1DFF1). No changed shipment file contributes to either.
- The FL005 red run exposed two gaps: a custom Lock() method was missed, and a function field named Acquire was diagnosed as a method.
- Deferred scope expansion captures recorded on 140-S: 6EB55AE6, E6EE8944, E2136C59, E08F7890, 3F7782B4, 0197BDA3, 5944CE03, B58C24FA, 2947C941, EB427E20. Stash CC0EBB59 stayed outside shipment scope and unmodified.

## Failed or rejected approaches

- The initial FL005 version missed custom Lock() methods and misdiagnosed a function field named Acquire as a method. The final red run caught both.

## Outcomes and open items

- 140-S pre-PR checkpoint status: pre-pr-ready. Next steps: push the branch, run a current-HEAD report-only review, prepare the PR with local review and build evidence, confirm required CI and Copilot review state, then merge with a merge commit under the recorded dark-mode pre-authorization.
- Passed for FL005: focused FL005 tests, scoped vet and lint, changed-file format check, and the faultline-analyze build.
- Open: 158.007-T remains active pending the owning review workflow.

## Sources

- docs/memory/2026-09-11-ship-140-s-pre-pr.md
- docs/memory/2026-09-11/fl005-review-remediation-memory.md
