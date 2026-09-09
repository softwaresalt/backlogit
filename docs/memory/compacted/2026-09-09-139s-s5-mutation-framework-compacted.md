---
chunk_strategy: h1-h2-h3
compacted_from:
    - docs/memory/2026-09-08-ship-139s-s5-mutation-framework-wave-complete.md
compacted_at: 2026-09-09T05:55:00Z
doc_type: learning
schema_version: "1.0"
source: docs/memory/compacted/2026-09-09-139s-s5-mutation-framework-compacted.md
source_shipment: 139-S
title: "Compacted Session: 139-S S5 Mutation Framework"
---

## Shipment: 139-S — S5 Mutation Postcondition Framework

**Shipped**: 2026-09-09 | **Merge SHA**: 78279fbb | **PR**: #434

### Delivered

New package `internal/faultline/mutation` — pure test infrastructure, no runtime surfaces, no `internal/core` import (no-cycle constraint).

| Unit | Task | Key Files |
|---|---|---|
| U1 | 157.001-T | representations.go, registry.go, declarations.go |
| U2 | 157.002-T | verify.go |
| U3 | 157.003-T | crash_test.go, fixtures_test.go |

### Final Commit Range on Branch

`c7281b7f` (U1 initial) → `87f032a5` (final Copilot review fix), merged as `78279fbb`.

### Key Decisions

1. **No `internal/core` import** — prevents import cycle; declarations describe ops without referencing their implementation
2. **Ingress + egress defensive copies** — `Register` copies slice before storing; `Lookup` copies before returning (both directions)
3. **IncompleteSnapshot guard** — `VerifySuccess`/`VerifyFailure` check key *presence* (2-value map lookup) before `bytes.Equal`; nil/nil would silently pass without it
4. **testOpSeq atomic.Int64** — shared across test files via `mutation_test` package; prevents registry collision across `-count=N` runs
5. **sort.SliceStable** — deterministic output regardless of duplicate-kind scenarios
6. **ErrOpNotRegistered sentinel** — `%w`-wrapped; `errors.Is`-compatible for callers

### P-021 Deferred Findings (7 entries)

| ID | Summary |
|---|---|
| 92F79833 | Pre-existing CRLF/LF failure in `internal/faultline` (138-S) |
| E57D994A | LookupOrError convenience fn |
| 56286069 | ResetForTesting() fn |
| AB482FAD | Semantic comparators for VerifySuccess/VerifyFailure |
| 2692E24C | Integration tests with real core.CreateItem/ArchiveItem ops |
| AB31C9C5 | CreateItem EventsJSONL declaration accuracy |
| 010F437C | ArchiveItem EventsJSONL declaration accuracy |

### Pre-Existing Test Failure

`TestU4aBehaviorCanonicalByteStable` — CRLF vs LF in golden bytes (138-S, deferred 92F79833).

### Copilot Review Cycles (3 rounds, 8 threads)

- Thread 1: Register() ingress copy — **fixed**
- Thread 2: semantic comparators — deferred AB482FAD
- Threads 3+4: integration tests — deferred 2692E24C
- Thread 5: ArchiveItem EventsJSONL accuracy — deferred 010F437C
- Thread 6: VerifySuccess guard comment — **fixed**
- Thread A: CreateItem EventsJSONL accuracy — deferred AB31C9C5
- Thread B: IncompleteSnapshot guard — **fixed**
