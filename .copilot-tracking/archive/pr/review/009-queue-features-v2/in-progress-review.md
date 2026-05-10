<!-- markdownlint-disable-file -->
# PR Review Status: 009-queue-features-v2

## Review Status

* Phase: 3 — Delegated Review (in progress)
* Last Updated: 2026-04-01
* Summary: Feature 009 Queue Features V2 — hierarchical file org, WIT type system, dependency graph, archive lifecycle, work queue, workflow policy

## Branch and Metadata

* Normalized Branch: `009-queue-features-v2`
* Source Branch: `009-queue-features-v2`
* Base Branch: `main`
* Commits: 4 (8531123, 6f5c97d, 94fdee8, 73fd360)
* Files Changed: 56 (+941 / -102)
* Linked Work Items: TASK-009 (root epic), TASK-009.01 through TASK-009.06

## Diff Mapping

| File | Type | New Lines | Notes |
|------|------|-----------|-------|
| `internal/mcp/tools.go` | Modified | +271 | 8 new MCP tool handlers wired |
| `internal/core/queue.go` | Added | +63 | QueueView, QueryQueue, BulkUpdateStatus, filterByResolvedDeps |
| `internal/cli/archive.go` | Added | +53 | `backlogit archive` CLI command |
| `internal/cli/migrate.go` | Added | +63 | `backlogit migrate` CLI command |
| `internal/cli/get.go` | Modified | +94 | --deps, --history flags |
| `internal/cli/list.go` | Modified | +21 | --queue, --group-by flags |
| `internal/cli/update.go` | Modified | +15 | --harness-status flag |
| `internal/cli/root.go` | Modified | +4 | Wire archive, migrate commands |
| `internal/core/archive.go` | Modified | +25 | StatusArchived, direct SQL update, events logging |
| `internal/core/archive_test.go` | Modified | +39 | 2 new archive exclusion tests |
| `internal/core/artifacts.go` | Modified | +45 | CreateArtifact, functional options |
| `internal/core/hierarchy.go` | Modified | +25 | QueueLayoutConfig alias, NextHierarchicalID, ParseHierarchicalID |
| `internal/core/wit_metadata.go` | Modified | +4 | DescribeType, ListTypes |
| `internal/core/workspace.go` | Modified | +44 | Workspace struct, NewWorkspace, SafeResolve |
| `internal/db/queries.go` | Modified | +46 | IncludeArchived filter, SelectCols export |
| `internal/db/rehydration.go` | Modified | +19 | level, hierarchy_path columns |
| `internal/db/schema.go` | Modified | +18 | item_deps, commit_links, level/hierarchy_path cols |
| `internal/models/artifact.go` | Modified | +5 | StatusArchived, updated validate oneof |
| `internal/config/defaults.go` | Modified | +42 | QueueLayoutConfig defaults |
| `internal/config/schema.go` | Modified | +14 | QueueLayoutConfig, HierarchyLevel structs |
| `internal/config/templates.go` | Modified | +6 | minor cleanup |
| `backlogit.exe` | Added | Binary | 🚨 17MB Windows binary committed |
| `.backlog/tasks/task-009*.md` | Modified | Metadata | Status fields updated to Done |
| `.github/agents/*.agent.md` | Modified | Minor | Whitespace / metadata updates |
| `.github/instructions/constitution.instructions.md` | Modified | Minor | Whitespace update |
| `.github/skills/impl-plan/SKILL.md` | Modified | Minor | Whitespace update |
| `.github/skills/plan-review/SKILL.md` | Modified | Minor | Whitespace update |

## Instruction Files Reviewed

* `.github/instructions/go.instructions.md`: Applies — all Go source changes
* `.github/instructions/constitution.instructions.md`: Applies — architectural patterns

## Review Items — Merged (5 Personas + Manual Analysis)

### 🔍 In Review

---

#### ⛔ P0 — BLOCKING

**[P0-01] `backlogit.exe` binary (17MB) committed** | `backlogit.exe`
Sources: Manual, CON-001
`.gitignore` covers `bin/` but not root `*.exe`. Must be removed before merge. Add `*.exe` and `backlogit.db` to `.gitignore`.

---

#### 🔴 P1 — High Priority

**[P1-01] `harness-status` CLI flag accepted but not persisted** | `internal/cli/update.go:54`, `internal/core/artifacts.go`
Sources: GQ-011
`--harness-status` populates `updates["harness_status"]` but `core.UpdateArtifact` has no handler for that key. Callers see "Updated ✓" with no actual change made.

**[P1-02] `MoveInQueue` is a silent no-op stub** | `internal/core/queue.go:153`
Sources: Manual
Bumps `updated_at` only. Explicitly comments "not yet implemented." Should return `ErrNotImplemented`; currently any `backlogit_move_item` call succeeds silently.

**[P1-03] Hardcoded `QueueLayoutConfig` in two MCP handlers** | `internal/mcp/tools.go` (~line 640–690)
Sources: Manual, MCP-001, MCP-002
`handleGetWITMetadata` and `handleListTypes` both construct an inline `QueueLayoutConfig` with hardcoded types. The `Workspace` struct already carries `Config` and `HeaderDef`; this bypasses them.

**[P1-04] `BulkUpdateStatus` makes SQLite authoritative over Markdown** | `internal/core/queue.go:167`
Sources: GQ-009, CON-002
Transaction commits DB first; Markdown sync is best-effort with errors silently dropped. Constitution § VII requires Markdown to be the source of truth.

**[P1-05] `filterByResolvedDependencies` treats DB errors as "no dependencies"** | `internal/core/queue.go:221`
Sources: GQ-010
When `GetDependencies` fails, the item is appended to the unblocked queue. Blocked items can surface when the dependency table is unreachable.

**[P1-06] `commit_links` table is SQLite-only — not in Markdown/JSONL** | `internal/db/schema.go:54`
Sources: CON-005
Commit linkages are stored only in SQLite. After `backlogit sync`, all `commit_links` are lost. Constitution § VII requires SQLite to be a rebuildable cache.

**[P1-07] `UnarchiveItem` restore path anchors to workspace root, not `.backlogit/`** | `internal/core/archive.go:127`
Sources: CON-003, CON-004
Path traversal validation uses `ws.RootPath` boundary; a crafted `archived_from` could restore files into any subdirectory of the project. Should anchor to `ws.RootPath/.backlogit/`.

**[P1-08] New MCP tools have no contract tests** | `internal/mcp/tools.go`
Sources: CON-007
8 new handlers (`add_dependency`, `remove_dependency`, `get_dependencies`, `archive_item`, `get_queue`, `track_commit`, `get_wit_metadata`, `list_types`) have zero test coverage in `tests/contract/`. Constitution § III requires contract tests for every MCP tool.

**[P1-09] `CAST(id AS INTEGER)` returns 0 for prefix-based IDs** | `internal/core/hierarchy.go:37`
Sources: Manual, SQL-003
`NextHierarchicalID` uses `MAX(CAST(id AS INTEGER))`. For IDs like `T001`, `EPIC-009`, this always returns 0, so `next = 1` every call — generating duplicate IDs.

**[P1-10] Schema migration gap: new columns absent on existing databases** | `internal/db/schema.go`
Sources: LR-005, SQL-001
`CREATE TABLE IF NOT EXISTS` is a no-op for existing databases. Workspaces upgrading from before this PR will have no `level`, `hierarchy_path`, `item_deps`, or `commit_links` columns, causing query failures.

**[P1-11] Rehydration does not populate `level` / `hierarchy_path`** | `internal/db/rehydration.go`
Sources: SQL-007
After `backlogit sync`, all items will have `NULL` level and hierarchy_path. These columns are never derived from artifact ID/parentID during rehydration.

---

#### 🟡 P2 — Should Address

**[P2-01] N+1 query in `filterByResolvedDependencies`** | `internal/core/queue.go:207`
Sources: Manual, SQL-004
One `GetDependencies` call per queue item = O(N×M) DB round-trips. Should batch-load all edges in a single query.

**[P2-02] Unbounded `QueryQueue` without default row cap** | `internal/core/queue.go:84`
Sources: Manual, SQL-009
`filter.Limit == 0` returns the entire `items` table, then P2-01 fires N times. Default cap of 500–1000 recommended.

**[P2-03] `UnarchiveItem` silently ignores `ArtifactFromFrontmatter` failures** | `internal/core/archive.go:154`
Sources: Manual, GQ-005
Same `type` vs `artifact_type` YAML mismatch that was fixed in `ArchiveItem` — `UnarchiveItem` still uses the broken pattern; DB index won't be updated on restore.

**[P2-04] `dependencies TEXT` column and `item_deps` table can diverge** | `internal/db/`
Sources: SQL-008, LR-007
Two representations for the same data with no sync guarantee between them. Source of truth is undefined.

**[P2-05] `QueueFilter` missing `IncludeArchived`** | `internal/core/queue.go:30`
Sources: SQL-011
`QueryQueue` doesn't exclude archived items (no `IncludeArchived` logic) while `QueryItems` does. Inconsistent behavior across the two query paths.

**[P2-06] MCP tool execution has no `slog` instrumentation** | `internal/mcp/tools.go`
Sources: CON-008
No structured log entries for tool invocations, failures, or timings. Constitution § V explicitly requires this.

**[P2-07] Workspace init suppresses malformed config errors** | `internal/core/workspace.go:39`
Sources: GQ-007, CON-009
`LoadHeaderDef` and `LoadTemplates` errors are fully swallowed regardless of whether they're "not found" vs "parse failure". Malformed config silently disables features.

**[P2-08] Rehydration discards dependency upsert errors** | `internal/db/rehydration.go:51`
Sources: GQ-014
`upsertDependencyBestEffort` errors are ignored silently; dependency graph can be incomplete with no caller signal.

**[P2-09] Missing index on `level` column** | `internal/db/schema.go`
Sources: SQL-002
`hierarchy_path` has `idx_items_hierarchy`; `level` does not. Queries filtering by level will scan.

---

#### 🔵 P3 — Advisory / Low Priority

- **[P3-01]** `FormatHierarchicalID` ignores `layout` parameter — always `%03d` (Manual)
- **[P3-02]** `.gitignore` missing `*.exe` and `backlogit.db` — preventive hygiene (Manual)
- **[P3-03]** `backlogit_unarchive_item` MCP tool missing — asymmetric archive/restore API (MCP-011)
- **[P3-04]** Multiple `//nolint:errcheck` directives without justification comments (GQ-003,004,008,012,013,015,017)
- **[P3-05]** GoDoc gaps: status constants block, `FindArtifactPath` comment (GQ-001,002)
- **[P3-06]** `NextHierarchicalID` doesn't accept `context.Context` (GQ-006)
- **[P3-07]** MCP handler response for `get_dependencies` with `reverse=true` lacks direction label (MCP-006)
- **[P3-08]** No FK constraints on `parent_id` / `item_deps` references (SQL-015)
- **[P3-09]** `DetectCycle` is public but not concurrency-safe outside a transaction (SQL-014)
- **[P3-10]** `backlogit_add_dependency` description should clarify cycle detection error behavior (MCP-014)



### ✅ Approved for PR Comment

* (populated after Phase 3 user decisions)

### ❌ Rejected / No Action

* (populated after Phase 3 user decisions)

## Next Steps

* [x] Phase 1: Initialize Review
* [x] Phase 2: Analyze Changes
* [ ] Phase 3: User review of findings (starting now)
* [ ] Phase 4: Finalize Handoff / Create PR
