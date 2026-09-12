---
chunk_strategy: h1-h2-h3
description: "Evidence-backed compound refresh for shipment 140-S and PR #436 after merge."
doc_type: closure
docline:
  date: 2026-09-12T02:35:00Z
  status: accepted
  tags:
    - compound-refresh
    - 140-S
    - pr-436
    - static-analysis
    - test-harness
schema_version: "1.0"
source: docs/closure/2026-09-11-140-s-pr-436-compound-refresh.md
title: "Shipment 140-S / PR #436 Compound Refresh"
---

## Scope

* Scope: `recent`
* Mode: `apply`
* Context: shipment `140-S`, PR `#436`, post-merge closure
* Merged evidence: merge commit
  `c5978bc26343a1ca6e3894bcab7a0fd74f7b8215`

The refresh reviewed source-shape TDD, parity and analyzer harness design,
scanner/analyzer AST patterns, fuzz resource bounds, and test non-vacuity.
Engram indexed discovery was unavailable because its daemon IPC endpoint was
not running, so the review used merged Git history, exact source reads, the
runtime-verification record, and the operational-closure record.

## Entries Reviewed

| Entry | Classification | Evidence and action |
|---|---|---|
| `docs/compound/2026-09-08-parity-harness-design-patterns.md` | update | Section 8 correctly requires signature-aware AST checks, but it did not distinguish permanent shape contracts from transition-only scaffold assertions. Updated only that section with the lifecycle rule demonstrated by `internal/faultline/analyzer/scannerdiscipline/scaffold_shape_test.go` and the final wiring contract in `cmd/faultline-analyze/main_test.go`. |
| `docs/compound/best-practices/source-shape-harnesses-must-allow-lifecycle-successors-2026-09-11.md` | keep | The merged scaffold test accepts either the one-analyzer scaffold state with four exact reserved comments or the final five-analyzer state with no reserved comments. It does not freeze a no-op analyzer body. The declaration-shape test also limits itself to stable types, constants, fields, interfaces, and signatures. |
| `docs/compound/go-patterns/ast-types-local-dataflow-without-cfg-2026-09-11.md` | keep | `scannerdiscipline/analyzer.go` normalizes `BlockStmt`, `CaseClause`, and `CommClause` statement lists, compares `types.Var` identity, excludes nested function literals from parent maps, and separates possible aliases from definite replacement. The learning remains an accurate bounded AST and `go/types` description. |
| `docs/compound/test-failures/go-analysistest-absolute-path-and-non-vacuity-2026-09-11.md` | keep | All five analyzers invoke `analysistest.Run` directly. FL004 uses `filepath.Abs("testdata")` and asserts two package results; the other analyzers use `analysistest.TestData()`. Bad fixtures retain seeded diagnostics and each analyzer has a clean fixture, so nested subprocess success is not the contract. |
| `docs/compound/runtime-errors/bufio-scanner-incomplete-fix-missed-db-package-2026-04-25.md` | keep | This entry concerns oversized JSONL I/O and cross-package fix completeness, not the FL001 scanner-discipline analyzer. The shared word `scanner` does not create a guidance overlap. |
| `docs/compound/runtime-errors/bufio-scanner-readline-eof-pattern-2026-05-09.md` | keep | This entry concerns unbounded line reading and EOF handling. It remains distinct from both the AST analyzer guidance and the cross-package incomplete-fix lesson. |

## Classification Summary

| Classification | Count | Result |
|---|---:|---|
| keep | 5 | No edits required |
| update | 1 | Narrow lifecycle caveat and cross-link applied |
| consolidate | 0 | No entries overlapped enough to merge |
| replace | 0 | No core guidance was obsolete |
| delete | 0 | No deletion or archive action warranted |

No entry required a stale marker.

## Fuzz-Bounds Coverage Finding

No existing compound entry specifically documents the shipped compatibility
corpus fuzz-resource envelope. The merged tests establish:

* a 64 KiB maximum decoded input
* a structural and YAML depth limit of 64
* a bounded native seed-file size
* metadata checks before reading and size-consistency checks after reading
* deterministic rejection through `errFuzzInputRejected` before adapter calls
  or per-adapter copies
* exact boundary and one-over-boundary tests
* a non-empty committed native seed corpus and deterministic replay

This is current code evidence, not stale guidance. The refresh skill maintains
existing learnings rather than creating a new solved-problem entry, so no new
compound file was added. A separate `compound` capture is appropriate only if
this bounded-fuzz pattern recurs or becomes a reusable contract.

## Evidence

* PR [#436](https://github.com/softwaresalt/backlogit/pull/436)
* `docs/closure/2026-09-11-140-s-158-f-pr-436-closure.md`
* `docs/closure/2026-09-11-140-s-pr-436-runtime-verification.md`
* `internal/faultline/compatcorpus/declarations_shape_test.go`
* `internal/faultline/compatcorpus/fuzz_test.go`
* `internal/faultline/compatcorpus/fuzz_hardening_test.go`
* `internal/faultline/analyzer/scannerdiscipline/scaffold_shape_test.go`
* `internal/faultline/analyzer/scannerdiscipline/analyzer.go`
* `internal/faultline/analyzer/auditsuccess/analyzer_test.go`
* `cmd/faultline-analyze/main_test.go`

The runtime-verification record confirms direct analyzer registration,
zero-diagnostic execution on shipment-owned clean packages, representative
race-enabled compatibility-corpus tests, and successful hosted CI at the
verified PR head.

## Files Changed

* Updated
  `docs/compound/2026-09-08-parity-harness-design-patterns.md`
* Added
  `docs/closure/2026-09-11-140-s-pr-436-compound-refresh.md`

No compound entry was consolidated, replaced, archived, deleted, or marked
stale.

## Follow-Ups

* Consider a separate bounded-fuzz compound entry only after another feature
  demonstrates that the resource-envelope pattern is recurring
* Keep future source-shape tests explicit about permanent versus
  transition-only assertions
