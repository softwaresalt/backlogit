# Stage Session Memory — Telemetry Quality Harvest

**Date**: 2026-04-25  
**Session**: Stage telemetry quality shipment  
**Phase completed**: harvest-complete

---

## Stash Entries Processed

| Stash ID | Priority | Kind | Summary | Disposition |
|---|---|---|---|---|
| `144CA2BB` | high | bug | bufio.Scanner token-too-long crashes telemetry parser on large log files | Removed — harvested into 047.001-T |
| `736ABA8A` | high | task | Fix `backlogit telemetry top` to rank by token usage, not call count | Removed — harvested into 047.002-T |
| `1FB3E504` | medium | task | Document undocumented side effect: harvest rehydrates SQLite | Removed — harvested into 047.003-T |
| `6DE63CCD` | medium | task | Write CLI reference doc for telemetry fields and metrics | Removed — harvested into 047.004-T |
| `D001CBE0` | high | feature | Umbrella stash grouping all 4 entries above | Harvested (produced 048-F; 048-F deleted as duplicate) |
| `21E17BFC` | low | feature | Singleton MCP server with multiplexed transport (contingency) | **Deferred** — contingency trigger condition not met |

---

## Artifacts Created

### Deliberation
- **`041-DL`**: Telemetry Quality — Parser Fix & Documentation  
  - Linked to umbrella stash D001CBE0  
  - Chosen direction: Option A (single shipment, address all 4 together)

### Implementation Plan
- **`docs/exec-plans/2026-04-25-telemetry-quality-plan.md`**  
  - Status: `reviewed`, gate: `PASS`  
  - 4 implementation units
  - Revised after plan review to address 3 P1 findings:
    1. Committed to `bufio.NewReader + ReadString('\n')` (not Scanner buffer increase)
    2. Added zero-token session validation (`TotalTokens==0 && ToolCalls>0` → reject)
    3. Cited 4 compound learnings explicitly

### Backlog Hierarchy
- **`047-F`**: Telemetry Quality — Parser Fix & Documentation (queued, shipment-ready)
  - **`047.001-T`**: Fix bufio.Scanner token-too-long handling (high, code, unit-1)
  - **`047.002-T`**: Fix telemetry top command to rank by token usage (high, code, unit-2)
  - **`047.003-T`**: Document telemetry harvest side effects (medium, docs, unit-3) — blocked by 047.001-T
  - **`047.004-T`**: Document harvested telemetry fields and metrics (medium, docs, unit-4) — blocked by 047.001-T

### Semantic Links
- `041-DL` informs `047-F`

### Dependencies
- `047.003-T` blocks on `047.001-T`
- `047.004-T` blocks on `047.001-T`

---

## Key Technical Decisions

1. **bufio.NewReader over Scanner**: bufio.Scanner has a hard maximum token size regardless of buffer configuration. Must replace with `bufio.NewReader` + `ReadString('\n')` for unbounded line reading.
2. **Proportional token attribution**: `ToolUsageRecord` has no per-tool token field. `TokensByServer` in `SessionSummary` is `map[string]string` (attribution set, not counts). Viable approach: `tokens_per_server[s] = session.TotalTokens × (ToolCallsByServer[s] / total_tool_calls)`.
3. **Zero-token session validation**: Must reject sessions where `TotalTokens==0 && ToolCalls>0` — these are partial sessions from dropped oversized log entries.

---

## Key File Locations

| Purpose | Path |
|---|---|
| Telemetry parser (Unit 1 target) | `internal/telemetry/parser.go` (Scanner at lines 62-136, 1MB buffer at line 63-64) |
| Harvest pipeline (secondary scanner) | `internal/telemetry/harvest.go` (Scanner at 231-265, SQLite rehydration at 131-137) |
| Reporter (Unit 2 target) | `internal/telemetry/reporter.go` (formatServerTable at 178-202, sorts by call count) |
| CLI top command | `internal/cli/telemetry.go` (top command at 90-110, uses GroupBy: "server") |
| Types | `internal/telemetry/types.go` (SessionSummary at 81-102) |
| Records | `internal/telemetry/records.go` (SessionSummaryRecord at 11-33, ToolUsageRecord at 38-46) |
| Schema | `internal/db/telemetry_schema.go` (tables at 54-81, rehydration at 114-164) |

---

## Compound Learnings Cited in Plan

- `docs/compound/database-issues/atomic-rehydration-sqlite-transaction-2026-04-08.md`
- `docs/compound/db-reliability/batch-failure-silent-nil-return-anti-pattern-2026-04-13.md`
- `docs/compound/db-reliability/sqlite-locked-missing-from-retry-predicate-2026-04-13.md`
- `docs/compound/best-practices/windows-safe-atomic-rename-goos-gate-2026-04-23.md`

---

## Deferred Stash Entry

**`21E17BFC`** (low/feature): Singleton MCP server with multiplexed HTTP/SSE transport.  
- Contingency: only pursue if concurrent SQLite contention recurs under real workloads.  
- All four original fixes shipped in 031-F (032-S). Trigger condition not met.  
- Leave in stash, revisit only if multi-process contention recurs.

---

## Cleanup Notes

- `048-F` was created as a side effect of `backlogit_harvest_stash` (duplicate of 047-F) — deleted.
- All 4 individual source stash entries removed after harvest confirmed.
- Hook event seq 241 (ship_shipment for 043-S) acknowledged.

---

## Shipment

**`045-S`**: Telemetry Quality — Parser Fix & Documentation (queued)  
Items: `047-F`, `047.001-T`, `047.002-T`, `047.003-T`, `047.004-T`

## Next Steps for Ship Agent

1. Claim shipment `045-S`
2. Scaffold test harness for `047.001-T` (parser fix) and `047.002-T` (reporter fix)
4. Run build-feature loop: fix bufio.Scanner in `internal/telemetry/parser.go`
5. Run build-feature loop: fix formatServerTable in `internal/telemetry/reporter.go`
6. Write docs for units 3 & 4 (unblocked after 047.001-T is done)
7. Review → CI → PR lifecycle
