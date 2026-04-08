---
title: "Data Quality & Tool Efficiency"
date: 2026-04-08
origin: ".backlogit/queue/004-DL.md"
status: reviewed
---

# Data Quality & Tool Efficiency

## Problem Frame

MCP tools return oversized payloads that consume agent context windows.
`fetch_stash` returns 34 KB, `get_queue` returns 1.2 MB, and `list_items`
returns unbounded results with full descriptions. Rehydration creates ghost
index entries from deleted markdown files, polluting queue views with 11+
phantom items. No dedicated tool identifies orphaned or duplicate artifacts.
Stash harvest lacks advisory file locking, risking race conditions during
concurrent agent sessions.

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | Add limit/offset pagination to `list_items` and `fetch_stash` | Stash 64CFF524 |
| R2 | Add compact projection mode (id, title, status, type, parent_id only) to `list_items`, `get_queue`, `fetch_stash` | Stash 64CFF524 |
| R3 | Fix rehydration to clear stale items so ghost entries don't persist | Stash B9AD4DFF |
| R4 | Add orphan detection filter to `get_queue` | Stash 40BB859A |
| R5 | Add duplicate artifact detection tool | Stash 0CBEE7D8 |
| R6 | Add advisory file locking to stash harvest | Stash 44E3C9D4 |

## Scope Boundaries

### In Scope

* Pagination parameters (limit/offset) for `list_items` and `fetch_stash`
* Compact projection mode for three high-volume tools
* Rehydration ghost entry fix (clear items table before rebuild)
* Orphan detection filter parameter on `get_queue`
* Duplicate detection MCP tool
* Advisory file locking for stash JSONL operations

### Non-Goals

* Cursor-based pagination (limit/offset is sufficient for current scale)
* Real-time duplicate prevention (detection only; prevention is a policy concern)
* Full-text search pagination (search_items already has a limit parameter)
* CLI command additions (MCP tools only for this feature set)

### Deferred to Implementation

* Whether to use OS-level file locking (`os.OpenFile` with `O_EXCL`) or sidecar
  `.lock` files for stash advisory locking
* Exact compact field set: may include `priority` and `labels` in addition to
  the core five fields if test scenarios reveal they are needed

## Implementation Units

Each unit is scoped to roughly 2 hours of human-equivalent effort. Units target
a single skill domain and produce a verifiable exit state.

### Unit 1: Pagination for list_items

**Files:** `internal/db/queries.go`, `internal/mcp/tools.go`
**Test files:** `internal/db/queries_expansion_test.go`, `tests/contract/tools_expansion_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `get_queue` already implements limit/offset via `QueueFilter` (queue.go:89-94); mirror that pattern in `QueryFilters`
**Dependencies:** none

**Approach:**

1. Add `Limit` and `Offset` int fields to `db.QueryFilters` struct.
2. In `db.QueryItems`, append `LIMIT` and `OFFSET` clauses when values are > 0,
   following the same pattern as `QueryQueue` (queue.go:89-94).
3. Register `limit` (number) and `offset` (number) parameters on the
   `backlogit_list_items` tool definition in tools.go.
4. Parse these parameters in `handleListItems` and pass to `QueryFilters`.

**Verification:**

* `go test ./internal/db/... -run TestQueryItems` passes with new limit/offset
  cases.
* MCP tool `backlogit_list_items` with `limit=5` returns at most 5 items.
* MCP tool `backlogit_list_items` with `limit=5, offset=5` returns the second
  page.

### Unit 2: Pagination for fetch_stash

**Files:** `internal/core/stash.go`, `internal/mcp/tools.go`
**Test files:** `internal/core/stash_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `FetchStashOptions` struct (stash.go:25-28); add `Limit` field
**Dependencies:** none

**Approach:**

1. Add `Limit int` field to `FetchStashOptions`.
2. In `FetchStash`, after filtering by priority and before expanding entries,
   truncate the slice to `opts.Limit` when Limit > 0.
3. Register `limit` (number) parameter on the `backlogit_fetch_stash` tool.
4. Parse the parameter in `handleFetchStash` and pass to options.

**Verification:**

* `go test ./internal/core/... -run TestFetchStash` passes with new limit case.
* MCP tool `backlogit_fetch_stash` with `limit=3` returns at most 3 entries.

### Unit 3: Compact projection mode

**Files:** `internal/mcp/tools.go`, `internal/models/artifact.go`,
`internal/core/queue.go`, `internal/core/stash.go`
**Test files:** `internal/mcp/tools_test.go`, `internal/models/artifact_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `toolResultJSON` helper in tools.go for serialization
**Dependencies:** none (parallel with Units 1-2)

**Approach:**

1. Define a `CompactArtifact` struct in `internal/models/artifact.go` with
   fields: `ID`, `Title`, `Status`, `ArtifactType`, `ParentID`, `Priority`.
   Add a `Compact()` method on `*Artifact` that returns a `CompactArtifact`.
2. Add a `compact` boolean parameter to `backlogit_list_items`,
   `backlogit_get_queue`, and `backlogit_fetch_stash` tool definitions.
3. In each handler, when `compact=true`, map the results through the `Compact()`
   method before serializing.
4. For `get_queue`, define `CompactQueueView` and `CompactQueueGroup` structs
   that mirror `QueueView`/`QueueGroup` but use `[]CompactArtifact` instead
   of `[]*models.Artifact`. The handler constructs the compact view before
   serializing. (Review finding F-1: QueueView.Items is `[]*models.Artifact`,
   so compact mode must map both top-level items AND group items.)
5. For `fetch_stash` compact mode, define a `CompactStashEntry` struct with
   fields: `ID`, `Priority`, `Kind`, `Text` (truncated to 120 chars).

**Verification:**

* `backlogit_list_items` with `compact=true` returns JSON objects with only
  the compact fields (no `description`, no `custom_fields`).
* `backlogit_get_queue` with `compact=true` returns items without full
  descriptions in both top-level items AND grouped items. Payload size is
  reduced by 80%+ compared to full mode.
* `backlogit_fetch_stash` with `compact=true` returns entries with truncated
  text.

### Unit 4: Fix rehydration ghost entries

**Files:** `internal/db/rehydration.go`
**Test files:** `internal/db/rehydration_expansion_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `DeleteAllItemLogs` already clears log tables at rehydration start (rehydration.go:26); extend this pattern
**Dependencies:** none (parallel with Units 1-3)

**Approach:**

1. At the top of `Rehydrate()`, after `DeleteAllItemLogs`, add calls to clear
   the `items`, `item_deps`, and `items_fts` tables. The `item_links` table is
   NOT cleared because it is populated by explicit tool calls and the
   `MigrateCustomFieldLinks` function, not by rehydration. Stash tables are
   cleared separately by `rehydrateStash` via `ClearStashIndex`.
2. Use `DELETE FROM items` and `DELETE FROM item_deps` in sequence. FTS content
   table cleanup is handled automatically by the delete trigger on items.
   (Review finding F-2: For large workspaces, per-row FTS trigger firing could
   be slow. If profiling reveals this, switch to explicit `DELETE FROM items_fts`
   before `DELETE FROM items` and disable triggers temporarily. For current
   scale of hundreds of items, trigger-based cleanup is acceptable.)
3. Add a test that:
   a. Creates a workspace with two markdown artifacts
   b. Rehydrates (both appear in index)
   c. Deletes one markdown file
   d. Rehydrates again
   e. Verifies only the surviving artifact is in the index

**Verification:**

* `go test ./internal/db/... -run TestRehydrate_GhostCleanup` passes.
* After deleting a markdown file and running `backlogit sync`, the deleted
  item no longer appears in `backlogit_list_items` or `backlogit_query_sql`.

### Unit 5: Orphan detection filter

**Files:** `internal/core/queue.go`, `internal/mcp/tools.go`
**Test files:** `internal/core/queue_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `IsOrphan()` at shipment_lifecycle.go:511; `QueueFilter` struct
**Dependencies:** none

**Approach:**

1. Add `OrphansOnly bool` field to `QueueFilter`.
2. In `QueryQueue`, after the dependency filtering and orphan annotation loop
   (queue.go:114-127), add a filter step: when `OrphansOnly` is true, retain
   only items where `is_orphan` custom field is set to true.
3. Register `orphans_only` boolean parameter on the `backlogit_get_queue` tool.
4. Parse in `handleGetQueue` and set on the filter.

**Verification:**

* `go test ./internal/core/... -run TestQueryQueue_OrphansOnly` passes.
* `backlogit_get_queue` with `orphans_only=true` returns only items that have
  a dot-separated ID but no parent_id set.

### Unit 6: Duplicate detection tool

**Files:** `internal/db/queries.go`, `internal/mcp/tools.go`
**Test files:** `internal/db/queries_expansion_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `SearchItems` pattern in queries.go for read-only queries
**Dependencies:** none

**Approach:**

1. Add `FindDuplicates(ctx, db) ([]DuplicateGroup, error)` function in
   `internal/db/queries.go`. Query:
   ```sql
   SELECT LOWER(title) AS norm_title, GROUP_CONCAT(id, ',') AS ids, COUNT(*) AS cnt
   FROM items
   GROUP BY LOWER(title)
   HAVING COUNT(*) > 1
   ORDER BY cnt DESC
   ```
2. Define `DuplicateGroup` struct with `Title string`, `IDs []string`,
   `Count int`.
3. Register `backlogit_find_duplicates` MCP tool (no required parameters).
4. Implement `handleFindDuplicates` handler following the five-step pattern.

**Verification:**

* `go test ./internal/db/... -run TestFindDuplicates` passes with test data
  containing duplicate-titled items.
* `backlogit_find_duplicates` returns groups of items sharing the same
  normalized title.

### Unit 7: Stash harvest advisory file locking

**Files:** `internal/core/stash.go`
**Test files:** `internal/core/stash_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** characterization-first (verify current race behavior, then add locking)
**Patterns to follow:** Atomic file write pattern already used in stash.go via `writeStashEntries`
**Dependencies:** none

**Approach:**

1. Implement a `lockStashFile(path string) (*os.File, error)` function that
   opens a sidecar `.backlogit/stash.jsonl.lock` file with `O_CREATE|O_EXCL`.
   Return the file handle for the caller to close (which releases the lock).
   If the lock file already exists AND is older than 60 seconds, remove it as
   stale and retry.
2. Implement `unlockStashFile(f *os.File)` that closes the file and removes the
   lock file.
3. Wrap `HarvestStashEntry` and `HarvestStashByPriority` with lock/unlock calls
   around the stash file read-modify-write section.
4. Also wrap `AddStashEntry` with the same locking.

**Verification:**

* `go test ./internal/core/... -run TestStashLock` passes.
* Two concurrent harvest calls for different stash IDs serialize correctly
  (second waits for first to release lock, both succeed).
* A stale lock file (>60s old) is cleaned up automatically.

## Dependency Graph

```text
Unit 1 (list_items pagination) ─┐
Unit 2 (fetch_stash pagination) ├── independent, can parallelize
Unit 3 (compact projection)   ──┤
Unit 4 (ghost entry fix)      ──┤
Unit 5 (orphan filter)        ──┤
Unit 6 (duplicate detection)  ──┤
Unit 7 (stash file locking)   ──┘
```

All units are independent. No unit depends on another. The recommended
execution order groups by risk and value:

1. Unit 4 (ghost fix) — fixes a bug visible in production
2. Unit 3 (compact mode) — highest payload reduction impact
3. Unit 1 + Unit 2 (pagination) — complement compact mode
4. Unit 5 (orphan filter) — quick win
5. Unit 6 (duplicate detection) — new capability
6. Unit 7 (stash locking) — hardening, lowest urgency

## Decisions

| # | Decision | Rationale | Alternatives Rejected |
|---|---|---|---|
| D1 | Use limit/offset pagination, not cursors | Backlog scale (hundreds, not millions) makes offset efficient. Simpler API. | Cursor-based pagination — unnecessary complexity at this scale |
| D2 | Clear items and item_deps tables at rehydration start | Rehydration rebuilds these from markdown source of truth. Prevents ghost entries. | Post-rehydration diff-and-delete — complex, error-prone, and slower |
| D3 | Do NOT clear item_links during rehydration | item_links are populated by explicit tool calls and MigrateCustomFieldLinks, not by rehydration. Clearing would lose migrated data. | Clear all tables — would require rehydration to rebuild links from custom_fields each time |
| D4 | Sidecar .lock file instead of OS file locking | Portable across Windows and Unix. Simpler to implement. Stale lock detection via file age. | flock/LockFileEx — platform-specific, harder to debug, overkill for advisory locking |
| D5 | Compact mode as a boolean parameter, not field selection | Simpler API. The compact field set is stable and well-defined. | GraphQL-style field selection — over-engineered for this use case |
| D6 | Orphan filter on get_queue, not a separate tool | Orphans are a queue concern. Adding a parameter is simpler than a new tool. | Dedicated backlogit_list_orphans tool — redundant with existing queue infrastructure |
| D7 | Duplicate detection as a new tool, not a query_sql recipe | Agents shouldn't need to know SQL GROUP BY syntax. A dedicated tool provides a clean contract. | Document a SQL recipe — poor agent ergonomics, no schema safety |

## Risks and Caveats

1. **Clearing items table during rehydration** removes all indexed items before
   rebuilding. If rehydration fails mid-walk, the index will be incomplete until
   the next successful sync. Mitigation: the index is ephemeral and gitignored;
   an incomplete index is functionally equivalent to a missing index.db, which
   triggers auto-rehydration.

2. **Sidecar lock files** can become orphaned if a process crashes between lock
   acquisition and release. Mitigation: stale lock detection (remove locks older
   than 60 seconds) prevents permanent lockout.

3. **item_links referencing ghost IDs** will persist after ghost entries are
   cleaned from the items table. This is acceptable: link queries already handle
   missing targets gracefully. A future cleanup migration can remove orphaned
   links.

4. **Compact mode** changes the JSON schema of tool responses. Agents that parse
   full artifact responses will see different fields when compact=true.
   Mitigation: compact is opt-in (default false), backward compatible.

## Learnings Applied

No directly matching learnings found in `docs/compound/`. The `go-patterns/`
and `go-implementation/` directories contain general Go patterns but nothing
specific to pagination, ghost entries, or file locking.

## Standards Check

| Principle | Status | Notes |
|---|---|---|
| I. Type-Safe Go | ✅ | New structs (CompactArtifact, DuplicateGroup) with proper tags |
| II. MCP Protocol Fidelity | ✅ | All new tools follow five-step handler pattern |
| III. Test-First Development | ✅ | Every unit specifies test files and verification |
| IV. Workspace Containment | ✅ | Lock file stays within .backlogit/ |
| V. Structured Observability | ✅ | slog logging for lock acquire/release, rehydration clearing |
| VI. Single-Binary Simplicity | ✅ | No new dependencies |
| VII. CQRS Data Architecture | ✅ | Rehydration fix reinforces CQRS: index is disposable cache |
| VIII. Git-Friendly Persistence | ✅ | Lock files are gitignored (ephemeral) |
| IX. Agent Context Efficiency | ✅ | Primary goal of this feature set |

## Review Record

**Date:** 2026-04-08
**Reviewers:** Constitution, Go Quality, Architecture, Scope Boundary (4-persona gate)
**Gate decision:** PASS with advisories

### Findings

| ID | Severity | Persona | Unit | Finding | Resolution |
|---|---|---|---|---|---|
| F-1 | P1 | architecture | Unit 3 | `QueueView.Items` is `[]*models.Artifact` and `QueueGroup.Items` likewise. Compact mode must map BOTH top-level and grouped items, requiring `CompactQueueView`/`CompactQueueGroup` parallel structs. | Amended Unit 3 approach to define compact view structs |
| F-2 | P2 | go-quality | Unit 4 | Clearing items table via `DELETE FROM items` fires per-row FTS delete triggers, which could be slow at scale. | Noted as acceptable at current scale; added profiling note |
| F-3 | P1 | scope | Unit 7 | `O_CREATE\|O_EXCL` on Windows does not prevent another process from deleting the lock file while the first process holds its handle. `os.Remove` may fail. | Noted: unlockStashFile should handle remove failure gracefully (log warning, proceed) |
| F-4 | P2 | constitution | Plan-level | Plan omits slog logging calls for new tool handlers and lock operations. Principle V requires structured observability. | Implementation must add slog.Info for tool entry/exit and lock acquire/release |
| F-5 | P2 | scope | Unit 6 | GROUP_CONCAT uses ',' separator; safe for NNN-SUFFIX IDs but fragile if ID format ever changes. | Acceptable: current ID format is well-defined; separator is implementation detail |
