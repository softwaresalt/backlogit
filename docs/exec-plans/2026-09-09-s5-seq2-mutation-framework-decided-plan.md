---
chunk_strategy: h1-h2-h3
decided_from: docs/exec-plans/2026-09-03-s5-seq2-mutation-framework-plan.md
decided_at: 2026-09-09T05:55:00Z
doc_type: learning
schema_version: "1.0"
source: docs/exec-plans/2026-09-09-s5-seq2-mutation-framework-decided-plan.md
source_shipment: 139-S
title: "Decided Plan: S5 Mutation Postcondition Framework (139-S)"
---

## Decision Record: 139-S S5 Mutation Framework

**Plan Review**: PASS | **Shipped**: 2026-09-09 | **PR**: #434

### Scope (Executed)

New package `internal/faultline/mutation` implementing:

1. **U1** — Declarative representation-set model: `RepresentationKind` enum (Frontmatter, SQLite, EventsJSONL, ArchiveFile, ShipmentRecord, CommitLink), `RepresentationSet{Op, Representations}`, `Validate()`, thread-safe registry, `OpCreateItem`/`OpArchiveItem` declarations with `init()`.

2. **U2** — Postcondition verifier: `MutationSnapshot{Op, Before, After}`, `VerificationResult{Op, Passed, IncompleteSnapshot, DriftedReps, MissingReps}`, `VerifySuccess()`, `VerifyFailure()`.

3. **U3** — Crash-boundary fixtures: `partialWriteSnap`, `staleIndexSnap`, `oldIndexSnap`, `indeterminateAtomicSnaps` (test-only helpers).

### Authorized Constraints (Enforced)

- **No `internal/core` import** (prevents import cycle for future core→mutation direction)
- **No production behavior change** (pure test infrastructure)
- **Stdlib-only imports**: bytes, errors, fmt, sort, sync
- **No new CLI/MCP commands**

### Accepted Deferred Scope (P-021 Entries)

- E57D994A: LookupOrError convenience function
- 56286069: ResetForTesting()
- AB482FAD: Per-representation semantic comparators
- 2692E24C: Integration tests with real core ops (blocked by no-core import)
- AB31C9C5: CreateItem EventsJSONL declaration accuracy audit
- 010F437C: ArchiveItem EventsJSONL declaration accuracy audit

### Plan Review Outcome

Correctness: clean. Architecture: clean. No P0/P1/P2 findings. Requires plan hardening: no.
