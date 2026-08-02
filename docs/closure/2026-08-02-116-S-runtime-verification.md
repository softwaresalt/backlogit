---
title: "116-S Runtime Verification — Shipment Sequencing Primitives"
source: docs/closure/2026-08-02-116-S-runtime-verification.md
doc_type: closure
description: "Post-merge runtime verification for PR #330 / shipment 116-S: settable shipment priority and shipment-to-shipment blocking order"
chunk_strategy: h1-h2-h3
schema_version: "1.0"
docline:
    date: 2026-08-02T06:32:00Z
    severity: low
    tags:
        - shipment
        - runtime-verification
        - priority
        - blocking
        - cli
        - mcp
        - core
---

# 116-S Runtime Verification — Shipment Sequencing Primitives

**Verdict: PASS**

PR #330 merged as `f3c6f76a60e18ae1b104ceb8a7833bfc52a52f61`.
Feature 134-F | Shipment 116-S | Merge date: 2026-08-02

## Surfaces Verified

| Surface | Method | Result |
|---------|--------|--------|
| `internal/core` — priority persistence + queue sort | `go test ./internal/core/...` with targeted `-run` | PASS |
| `internal/core` — `AddShipmentBlock` validation + sync | `go test ./internal/core/...` with targeted `-run` | PASS |
| `internal/cli` — `--priority` flag + parity lock | `go test ./...` (full suite) | PASS |
| `internal/mcp` — `create_shipment` priority param | `go test ./...` (full suite) | PASS |
| `tests/integration` | `go test ./tests/integration` | PASS |
| `tests/contract` | `go test ./tests/contract` | PASS |

## Environment Prechecks

- Build artifact: merged `main` at `f3c6f76a`
- Go toolchain: 1.24.0
- `go vet ./...`: clean, exit 0
- `golangci-lint run`: clean, exit 0
- `gofmt -l .`: not run as separate gate (not in CI; project has pre-existing repository-wide formatting debt unrelated to PR #330)

## Targeted Scenarios Exercised

### Scenario 1: Shipment priority persists to frontmatter

**Test**: `TestCreateShipmentWithPriority_PersistsFrontmatter`  
**Expected**: `WithPriority("high")` option sets `priority: high` in Markdown frontmatter and SQLite index.  
**Observed**: PASS — priority field present after `sync_index`.

### Scenario 2: Queue sorts shipments by priority (critical > high > medium > low > empty)

**Test**: `TestCreateShipmentWithPriority_QueueSortOrdersByPriority`  
**Expected**: `QueryQueue` with `type=shipment` returns shipments ordered critical first, then high, medium, low. Unknown/empty sorts last with id-ascending tie-break.  
**Observed**: PASS — 5 shipments with mixed priorities returned in correct order.

### Scenario 3: Empty/unknown priority sorts last deterministically

**Test**: `TestCreateShipmentWithPriority_EmptyPriorityLastAndDeterministic`  
**Expected**: 2 shipments with no priority, one with `medium` — medium first, empties last in ID order.  
**Observed**: PASS.

### Scenario 4: AddShipmentBlock creates blocks edge between two shipments

**Test**: `TestAddShipmentBlock_CreatesBlocksEdgeBetweenShipments`  
**Expected**: `AddShipmentBlock(ctx, ws, dependentID, prerequisiteID)` persists a `blocks` dependency edge readable from `GetDependencies`.  
**Observed**: PASS.

### Scenario 5: AddShipmentBlock rejects non-shipment endpoints

**Tests**: `TestAddShipmentBlock_RejectsWhenDependentIsNotShipment`, `TestAddShipmentBlock_RejectsWhenPrerequisiteIsNotShipment`  
**Expected**: Returns typed error when either endpoint is not a shipment artifact.  
**Observed**: PASS — both endpoints validated.

### Scenario 6: Blocks edge survives sync_index rehydration

**Test**: `TestAddShipmentBlock_SurvivesSyncIndex`  
**Expected**: After `sync_index`, the `blocks` edge is rebuilt from Markdown frontmatter.  
**Observed**: PASS.

## Full Suite Results

```
ok  github.com/softwaresalt/backlogit/cmd/gen-docs        34.271s
ok  github.com/softwaresalt/backlogit/internal/cli        87.999s
ok  github.com/softwaresalt/backlogit/internal/core        6.097s
ok  github.com/softwaresalt/backlogit/internal/mcp        (cached)
ok  github.com/softwaresalt/backlogit/tests/contract      20.242s
ok  github.com/softwaresalt/backlogit/tests/integration   25.969s
[all other packages: ok/cached]
```

Exit code: 0 — all tests pass.

## Invariants Confirmed

- `CreateShipment` signature remains backward-compatible (variadic `...Option`)
- `AddDependency` generic path is byte-for-byte unchanged
- `filterByResolvedDependencies` queue suppression is read-time, non-destructive
- CLI/MCP parity for `create_shipment` locked by denylist test (`TestCreateShipmentCLIMCPParity` in `internal/cli/shipment_test.go`)

## Deferred Items (not blocking this release)

1. `MCP get_queue` priority-sort parity — `handleGetQueue` ignores `sort_by`; changing its default touches all-types queue surface; captured in deliberation 055-DL
2. `queue_position` SQL refactor (candidate (a)) — deferred
3. `ship_sequence.jsonl` audit surface (candidate (d)) — deferred
4. Pagination edge case: `filterByResolvedDependencies` applied after SQL `LIMIT/OFFSET` — Copilot flagged this as a suppressed (non-blocking) finding; tracked for future fix

## Follow-up Recommendations

- Monitor `queue view --type shipment` sort behavior in production dark-factory runs
- If pagination edge case manifests, the fix is SQL-level filtering in `QueryQueue`; no backward-incompatible API change needed
