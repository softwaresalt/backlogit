---
task_id: "T008"
date: 2026-04-05 00:00
type: checkpoint
feature: F015
---

# Checkpoint: T008

## Files Modified

- **`.backlogit/config.yaml`** — Added shipment artifact type (prefix S, queue level 1), extended status enum to include shipped and abandoned states
- **`.backlogit/header-def.yaml`** — Added shipment type definition with status, branch, and items fields
- **`.backlogit/templates/shipment.md`** — New shipment template with Description, Items, and Blocked Returns sections
- **`.backlogit/stash.jsonl`** — New file: migrated 12 stash entries from legacy .stash.md using JSONL format (one JSON object per line)
- **`internal/stash/stash.go`** — Added JSONLFileName constant to eliminate hardcoded filename in rehydration logic
- **`internal/db/rehydration.go`** — Updated rehydrateStash to read both .stash.md (legacy) and stash.jsonl (new), deduplicating by ID; db.Rehydrate now returns combined count of stash entries + markdown artifacts
- **`internal/cli/migrate_import_test.go`** — Fixed hardcoded template count assertion: 6→7 (T001 added shipment template, test was unpatched)

## Decisions

- **db.Rehydrate return signature** — Now returns stash entry count + markdown artifact count so downstream tests and callers can assert `count > 0` after stash.jsonl creation. Enables verification that stash migration succeeded.

- **Stash rehydration merge strategy** — rehydrateStash merges .stash.md (legacy) and stash.jsonl (new) by deduplicating on ID, allowing gradual migration path without data loss. Both formats coexist during transition period.

- **JSONLFileName constant** — Extracted hardcoded filename to stash package constant so rehydration logic doesn't depend on magic strings. Reduces maintenance burden if filename changes.

- **JSONL format adoption** — .backlogit/stash.jsonl uses newline-delimited JSON (one complete JSON object per line) matching industry standard for append-only logs. Maintains order and enables streaming reads.

## Errors Resolved

- **TestShipmentWorkflow_StashJSONLRehydration failing with count=0** — Root cause: db.Rehydrate only counted markdown artifacts, ignoring stash entries. Fixed by updating rehydrateStash to return entry count and adding it to the total count returned by db.Rehydrate. Test now correctly verifies stash migration.

- **TestMigrateCommand_ImportStructuredBacklogWorkspace_IsIdempotent expects 6 items, got 7** — Root cause: T001 added shipment template but this test had hardcoded count. Fixed by updating assertion from 6→7 to match the new template inventory.

## Review Findings

**P0 (Blockers):** None

**P1 (Must-fix before ship):**
- ReturnBlockedItem dual-write not atomic — Task F015.T009 created to implement transactional updates for blocked return state changes

**P2 (Should-fix soon):**
- Stash rehydration source_path provenance issue — Task F015.T010 created to track which format (.md vs .jsonl) each stash entry came from
- MCP shipment error classification — Task F015.T011 created to align error responses with Foundry agent expectations

**P3 (Nice-to-have):**
- Shipment CLI commands use context.Background() instead of cmd.Context() — Low priority refactoring for consistency

## Next Task Context

- **F015 completion** — All 8 tasks in feature F015 are now done. Follow-up work distributed to T009, T010, T011 for atomicity, provenance, and error classification improvements.

- **Legacy stash file** — The `.backlogit/queue/.stash.md` file persists as legacy format. Plan: after stash.jsonl proves stable in production, deprecate and remove .stash.md to simplify backlogit file structure.

- **All gates passed** — gofmt, go vet, golangci-lint, and go test ./... all passing. Feature is ready for merge.

- **Migration strategy** — Current dual-read approach (both .stash.md and stash.jsonl) ensures no data loss. Monitor adoption of new format; when confidence is high, can drop legacy reader.
