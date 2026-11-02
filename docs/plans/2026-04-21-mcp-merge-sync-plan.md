---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for the backlogit_merge_sync MCP tool: incremental SQLite cache refresh from .backlogit/ file diffs'
doc_type: plan
docline:
    date: 2026-04-21T00:00:00Z
    origin: .backlogit/queue/037-DL.md
    status: draft
ingested_at: "2026-06-26T02:33:21Z"
schema_version: "1.0"
source: docs/exec-plans/2026-04-21-mcp-merge-sync-plan.md
title: 'MCP Merge Sync: Incremental Cache Refresh for .backlogit Drift'
---

## Problem Frame

During long-running agent sessions, external changes to `.backlogit/` files (git merge, manual edits, concurrent MCP processes) cause the SQLite cache to drift from the Markdown source of truth. The only current recovery is `backlogit_sync_index`, which triggers a full `Rehydrate` — a destructive DELETE-all + WalkDir + batch-insert cycle that briefly empties the index and rebuilds stash and log tables unnecessarily when only a few artifact files changed.

We need a lightweight `backlogit_merge_sync` MCP tool that detects which files changed since the last sync and performs targeted cache updates. Full rehydrate remains the safety fallback for ambiguous or large deltas.

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | In-memory file manifest populated at startup, updated on each merge_sync call | Deliberation 037-DL, Chosen Direction Q1 |
| R2 | Manifest keyed by relative path, storing mtime, size, fileKind, and item_id | Deliberation 037-DL, Chosen Direction |
| R3 | Explicit-only trigger via `backlogit_merge_sync` MCP tool call | Deliberation 037-DL, Chosen Direction Q2 |
| R4 | Fallback to full rehydrate when delta exceeds 50% of manifest entries OR 50 files | Deliberation 037-DL, Chosen Direction Q3 |
| R5 | Dry-run mode returning drift without applying changes | Deliberation 037-DL, Chosen Direction Q4 |
| R6 | Structured JSON response with touched IDs, fallback status, and stash/log refresh flags | Deliberation 037-DL, Chosen Direction Q5 |
| R7 | Relocation detection: same item_id at a different path treated as update, not delete+add | Deliberation 037-DL, Open Questions Q2 |
| R8 | Use existing full-rebuild functions for stash and log refresh in v1 | Deliberation 037-DL, Open Questions Q1 |
| R9 | All DB writes through `RetryWrite` wrapper; all errors propagated, never swallowed | Compound learnings: atomic-rehydration, sqlite-locked-retry, batch-failure-anti-pattern |
| R10 | Separate `manifestMu` mutex for manifest access; `workspaceMu` held only during apply phase | Deliberation 037-DL, Open Questions Q3 |

## Scope Boundaries

### In Scope

* New `internal/db/manifest.go` package with `FileManifest` type, diff computation, and manifest-building from WalkDir
* New `backlogit_merge_sync` MCP tool handler in `internal/mcp/`
* Manifest population hook during `Rehydrate` (modify existing function to return manifest data)
* Manifest field on `mcp.Server` struct with dedicated mutex
* Targeted upsert/delete of individual artifacts using existing `UpsertItemTx` and `DeleteItemTx`
* Full stash/log rebuild triggered when `stash.jsonl` or `logs/*.jsonl` appear in the drift set
* Dry-run mode parameter
* Unit tests for manifest diff, merge sync logic, and the tool handler
* Contract tests for the new MCP tool

### Non-Goals

* Incremental stash or log updates (v1 uses existing full-rebuild functions)
* SQLite-persisted manifest (memory-only in v1)
* Content-hash-based change detection
* Automatic/lazy triggering of merge sync
* CLI `backlogit merge-sync` command (MCP-only in v1)

### Deferred to Implementation

* Exact threshold tuning (50% / 50-file limits are starting points; adjust based on test results)
* Whether `parseMarkdownArtifact` needs performance optimization for single-file re-parse (likely fine as-is)

## Implementation Units

Each unit is scoped to roughly 2 hours of human-equivalent effort. Units target a single skill domain and specify a verifiable exit state.

### Unit 1: File Manifest Data Types and Diff Logic

**Files:** `internal/db/manifest.go`
**Test files:** `internal/db/manifest_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `internal/db/retry.go` (small, focused package file with clear type definitions)
**Dependencies:** none

**Approach:**

Define the core manifest types and pure diff function:

```go
// FileEntry represents a single tracked file in the manifest.
type FileEntry struct {
    RelPath  string    // relative to .backlogit/
    Kind     FileKind  // artifact, stash, log, config, other
    Size     int64
    ModTime  time.Time
    ItemID   string    // extracted artifact ID (empty for non-artifact files)
}

// FileKind classifies workspace files for selective rebuild.
type FileKind string

const (
    FileKindArtifact FileKind = "artifact"
    FileKindStash    FileKind = "stash"
    FileKindLog      FileKind = "log"
    FileKindConfig   FileKind = "config"
    FileKindOther    FileKind = "other"
)

// DiffResult holds the sets of added, changed, deleted, and relocated files.
type DiffResult struct {
    Added     []FileEntry
    Changed   []FileEntry
    Deleted   []FileEntry
    Relocated []RelocationEntry // same ItemID, different path
}

// RelocationEntry tracks a file that moved (e.g., queue/ to done/).
type RelocationEntry struct {
    ItemID  string
    OldPath string
    NewPath string
    Entry   FileEntry
}
```

`ComputeDiff(old map[string]FileEntry, current map[string]FileEntry) DiffResult` — pure function comparing two manifest snapshots. Detects relocations by matching ItemID across the delete+add sets.

`ClassifyFile(relPath string) FileKind` — categorizes a relative path by directory prefix (`queue/`, `done/`, `active/`, `archive/` → artifact; `stash.jsonl` → stash; `logs/*.jsonl` → log; `config.yaml`, `registry.yaml`, `hooks.yaml`, `header-def.yaml` → config).

`ShouldFallback(diff DiffResult, manifestSize int, maxChangedFiles int) (bool, string)` — returns true + reason when the delta is too large for incremental sync.

**Verification:**
* All tests pass: diff computation with adds, changes, deletes, relocations
* Fallback threshold logic tested at boundary conditions (49%, 50%, 51%; 49, 50, 51 files)
* Relocation detection matches by ItemID across delete+add sets
* `go test -race ./internal/db/...` passes

### Unit 2: Manifest Walk and Population

**Files:** `internal/db/manifest.go` (extend), `internal/db/rehydration.go` (modify)
**Test files:** `internal/db/manifest_test.go` (extend)
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `Rehydrate` Phase 1 collect walk in `rehydration.go:53-79`
**Dependencies:** Unit 1

**Approach:**

`BuildManifest(workspacePath string) (map[string]FileEntry, error)` — walks the `.backlogit/` directory tree, stats each file, classifies it by kind, and extracts the artifact ID from markdown files by reading only the frontmatter `id:` field (not a full parse). Returns a map keyed by relative path.

For artifact ID extraction without full parse, read the first ~500 bytes and extract the `id:` field from YAML frontmatter. This avoids parsing the entire file body just to populate the manifest.

Modify `Rehydrate` to optionally return the manifest built during Phase 1. Add a new exported function:

`RehydrateWithManifest(ctx context.Context, workspacePath string, db *sql.DB) (int, map[string]FileEntry, error)` — calls existing `Rehydrate` logic but also builds and returns the manifest from the Phase 1 walk. The existing `Rehydrate` function delegates to `RehydrateWithManifest` and discards the manifest for backward compatibility.

**Verification:**
* `BuildManifest` correctly walks a temp workspace with artifacts, stash, logs, and config files
* Artifact IDs extracted correctly from frontmatter without full body parse
* `RehydrateWithManifest` returns the same count as `Rehydrate` plus a populated manifest
* Existing `Rehydrate` tests continue to pass (backward compatibility)
* `go test -race ./internal/db/...` passes

### Unit 3: Incremental Sync Engine

**Files:** `internal/db/merge_sync.go`
**Test files:** `internal/db/merge_sync_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `Rehydrate` Phase 3 batch pattern (rehydration.go:108-175), `RetryWrite` usage pattern
**Dependencies:** Unit 1, Unit 2

**Approach:**

`MergeSyncResult` struct:

```go
type MergeSyncResult struct {
    Added          []SyncEntry `json:"added"`
    Changed        []SyncEntry `json:"changed"`
    Deleted        []SyncEntry `json:"deleted"`
    Relocated      []SyncEntry `json:"relocated"`
    StashRefreshed bool        `json:"stash_refreshed"`
    LogsRefreshed  bool        `json:"logs_refreshed"`
    FallbackUsed   bool        `json:"fallback_used"`
    FallbackReason string      `json:"fallback_reason,omitempty"`
    DryRun         bool        `json:"dry_run"`
}

type SyncEntry struct {
    ID   string `json:"id"`
    Path string `json:"path"`
}
```

`MergeSync(ctx context.Context, workspacePath string, database *sql.DB, manifest map[string]FileEntry, dryRun bool) (MergeSyncResult, map[string]FileEntry, error)`:

1. Call `BuildManifest` to get current state.
2. Call `ComputeDiff` against the provided manifest.
3. Check `ShouldFallback`. If fallback needed, call `Rehydrate` + `BuildManifest` and return result with `FallbackUsed: true`.
4. If dry-run, return diff result without applying changes.
5. Apply incremental changes within a `RetryWrite`-wrapped transaction:
   - For each added/changed artifact: `parseMarkdownArtifact` → `UpsertItemTx` (with deps and links, following the Rehydrate Phase 3 pattern)
   - For each deleted artifact: `DeleteItemTx`
   - For each relocated artifact: `UpsertItemTx` with the new path's parsed content (effectively an update)
6. If stash.jsonl appears in the diff, call `rehydrateStash` (existing full-rebuild, R8).
7. If any `logs/*.jsonl` file appears in the diff, call `rehydrateItemLogs` (existing full-rebuild, R8).
8. Return the new manifest and result.

All DB writes use `RetryWrite` (R9). Errors propagate; no log-and-swallow (R9).

**Verification:**
* Add a new artifact file to a temp workspace → sync detects and indexes it
* Modify an existing artifact → sync updates the index
* Delete an artifact file → sync removes it from the index
* Relocate a file (same ID, different directory) → sync treats as update
* Exceed threshold → sync falls back to full rehydrate
* Dry-run returns diff without database changes
* Stash JSONL change triggers stash rebuild
* Log JSONL change triggers log rebuild
* `go test -race ./internal/db/...` passes

### Unit 4: Server Manifest Integration

**Files:** `internal/mcp/server.go` (modify), `internal/mcp/tools.go` (modify)
**Test files:** `internal/mcp/merge_sync_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `ensureWorkspace` mutex pattern (server.go:106-128), `handleSyncIndex` handler (tools.go:720-729)
**Dependencies:** Unit 2, Unit 3

**Approach:**

Add manifest state to `Server`:

```go
type Server struct {
    // ... existing fields ...
    manifest   map[string]db.FileEntry
    manifestMu sync.RWMutex  // protects manifest reads/writes
}
```

Modify `ensureWorkspace` to populate the manifest on first workspace init. After `core.NewWorkspace` succeeds, call `db.RehydrateWithManifest` and store the returned manifest. This is the one-time baseline population (R1).

Register the `backlogit_merge_sync` tool in `RegisterTools`:

```go
s.addTool(
    mcplib.NewTool("backlogit_merge_sync",
        mcplib.WithDescription("Detect .backlogit file drift and apply targeted cache updates"),
        mcplib.WithBoolean("dry_run",
            mcplib.Description("Return drift report without applying changes"),
        ),
    ),
    s.handleMergeSync,
)
```

`handleMergeSync` handler:

1. `requireWorkspace` (existing pattern)
2. `s.manifestMu.RLock()` to snapshot the current manifest, then `RUnlock()`
3. Call `db.MergeSync(ctx, storagePath, ws.DB, manifestSnapshot, dryRun)`
4. If not dry-run: `s.manifestMu.Lock()`, update `s.manifest` with the returned new manifest, `Unlock()`
5. Return the `MergeSyncResult` as JSON

The `workspaceMu` is NOT held during the walk phase — only `manifestMu` coordinates manifest access (R10). The `requireWorkspace` call at the top ensures the workspace exists but releases `workspaceMu` before the sync work begins.

**Verification:**
* Tool is registered and discoverable via the MCP tool list
* Handler returns structured JSON matching `MergeSyncResult` schema
* Manifest populated after first workspace init
* Concurrent `handleMergeSync` calls serialize correctly via `manifestMu`
* Dry-run flag is respected (returns drift, no DB writes)
* `go test -race ./internal/mcp/...` passes

### Unit 5: Contract and Integration Tests

**Files:** `tests/contract/merge_sync_contract_test.go`, `tests/integration/merge_sync_integration_test.go`
**Test files:** (these ARE the test files)
**Effort size:** medium
**Skill domain:** tests
**Execution note:** test-first (write contract tests against expected schema, then verify passing)
**Patterns to follow:** `tests/contract/` existing tests for other MCP tools, `internal/db/rehydration_expansion_test.go` pattern
**Dependencies:** Unit 4

**Approach:**

**Contract tests** (`tests/contract/merge_sync_contract_test.go`):
* Validate `backlogit_merge_sync` tool parameter schema (dry_run is optional boolean)
* Validate response JSON structure matches `MergeSyncResult` fields
* Validate error response when workspace not initialized
* Validate dry_run returns results without modifying the database

**Integration tests** (`tests/integration/merge_sync_integration_test.go`):
* End-to-end: init workspace → rehydrate → add file externally → merge_sync → verify item indexed
* End-to-end: init workspace → rehydrate → delete file externally → merge_sync → verify item removed
* End-to-end: init workspace → rehydrate → modify stash.jsonl → merge_sync → verify stash refreshed
* Fallback: init workspace with 100+ artifacts → modify 60% → merge_sync → verify fallback triggered
* Relocation: move file from queue/ to done/ → merge_sync → verify single update (not delete+add)

**Verification:**
* All contract tests pass
* All integration tests pass
* `go test -race ./tests/...` passes
* Full gate: `go test -race ./...` passes

## Dependency Graph

```text
Unit 1: Manifest Types & Diff
    │
    ├──→ Unit 2: Manifest Walk & Population
    │        │
    │        ├──→ Unit 3: Incremental Sync Engine
    │        │        │
    │        │        └──→ Unit 4: Server Integration
    │        │                 │
    │        │                 └──→ Unit 5: Contract & Integration Tests
    │        │
    │        └──→ Unit 4 (also depends on Unit 2)
    │
    └──→ Unit 3 (also depends on Unit 1)
```

Sequential execution order: 1 → 2 → 3 → 4 → 5

Units 1 and 2 build the foundation. Unit 3 implements the core sync logic. Unit 4 wires it into the MCP server. Unit 5 validates the full stack.

## Decisions

| # | Decision | Rationale | Alternatives Rejected |
|---|---|---|---|
| D1 | In-memory manifest on Server struct | Server persists for stdio session; SQLite is ephemeral cache — storing manifest there contradicts CQRS | SQLite `file_manifest` table (schema complexity, marginal benefit) |
| D2 | mtime-based change detection | Reliable with atomic writes; avoids reading file contents on every sync | Content hashing (defeats lightweight detection purpose) |
| D3 | Explicit-only trigger | Avoids complexity of stale-detection timing; agents already call sync explicitly | Lazy/automatic triggering (interference risk with in-flight writes) |
| D4 | Reuse existing full-rebuild for stash/logs | Fast enough for v1 workspaces; avoids complexity of incremental JSONL parsing | Incremental JSONL delta parsing (premature optimization) |
| D5 | Separate `manifestMu` RWMutex | Allows concurrent reads during walk; `workspaceMu` not held during expensive walk phase | Single `workspaceMu` for everything (blocks reads during walk) |
| D6 | `RehydrateWithManifest` wrapper | Captures manifest during existing Phase 1 walk at zero extra cost; backward compatible | Separate `BuildManifest` call after Rehydrate (double walk) |
| D7 | Frontmatter-only ID extraction for manifest | Reading ~500 bytes per file is sufficient for manifest; avoids full Markdown parse cost | Full `parseMarkdownArtifact` per file (unnecessary overhead for manifest) |
| D8 | Relocation detection by ItemID | Prevents unnecessary delete+add churn in the index; preserves event continuity | Treat relocations as separate delete+add operations (wasteful) |

## Risks and Caveats

1. **mtime resolution on some filesystems** — FAT32 has 2-second mtime resolution. If backlogit runs on FAT32 and two changes happen within 2 seconds, the second change may be missed. Mitigation: backlogit targets development environments where NTFS/ext4/APFS are standard. Document the FAT32 limitation.

2. **Race between external write and manifest snapshot** — If a file is written between the manifest snapshot read and the walk, the diff may miss it. Mitigation: acceptable for v1 — the next `merge_sync` call will catch it. The sync is designed to be called repeatedly.

3. **Batch failure propagation** — Per compound learning `batch-failure-silent-nil-return-anti-pattern`, individual artifact upsert failures during incremental sync must propagate. Unlike full rehydrate (which can skip and rebuild next time), incremental sync that silently drops a file leaves the index permanently stale. Mitigation: fail the entire sync transaction on any upsert error.

4. **SQLITE_LOCKED in retry predicate** — Per compound learning `sqlite-locked-missing-from-retry-predicate`, the existing `RetryWrite` wrapper already handles SQLITE_LOCKED (verified in `retry.go:62`). No additional work needed, but implementation must use `RetryWrite` consistently.

5. **Manifest memory usage** — Each `FileEntry` is roughly 200 bytes. A workspace with 1000 files uses ~200KB. Negligible for the MCP server process.

## Plan Hardening Signals (REQUIRED)

* **Public API, schema, or contract change**: YES — new `backlogit_merge_sync` MCP tool is a public API addition. Agents will depend on its response schema. However, this is an additive change (no existing tools modified).
* **Security, auth, permission, or compliance-sensitive behavior**: NO — the tool reads/writes only within the existing `.backlogit/` containment boundary using existing `SafeResolve` patterns.
* **Migration, backfill, destructive data/config action, or irreversible step**: NO — the sync writes to the ephemeral SQLite cache only. Markdown source of truth is read-only during sync.
* **External integration, operator checkpoint, or external dependency**: NO — pure local operation.
* **High runtime, rollout, or rollback risk**: LOW — the fallback to full rehydrate provides a safety net. If merge_sync produces incorrect results, `backlogit_sync_index` (full rehydrate) remains available as the recovery path.

Requires plan hardening: no

The new MCP tool is additive, writes only to the ephemeral cache, and has a full-rehydrate safety fallback. The response schema is new (no backward compatibility concern). Standard review is sufficient.

## Runtime Verification and Closure

### Runtime Surfaces Changed

* **MCP tool surface**: New `backlogit_merge_sync` tool added to the MCP server tool registry
* **Server initialization**: `ensureWorkspace` modified to populate manifest during first workspace init
* **Rehydration engine**: `Rehydrate` refactored to `RehydrateWithManifest` (backward-compatible wrapper preserved)

### Runtime Verification

1. Start the MCP server (`backlogit mcp`), initialize a workspace, and verify the manifest is populated by calling `backlogit_merge_sync` with `dry_run: true` — should return an empty diff (no changes since startup).
2. Externally modify a `.backlogit/` artifact file, then call `backlogit_merge_sync` — should detect the changed file and return it in the `changed` array.
3. Add a new artifact file externally, call `merge_sync` — should appear in `added` array and be queryable via `backlogit_query_sql`.
4. Delete an artifact file externally, call `merge_sync` — should appear in `deleted` array and be removed from `backlogit_query_sql` results.
5. Verify `backlogit_sync_index` (existing full rehydrate) still works correctly after the `RehydrateWithManifest` refactor.

### Operational Closure

* **Rollback trigger**: If `merge_sync` consistently returns incorrect diffs or corrupts the index, agents should fall back to `backlogit_sync_index`.
* **Rollback procedure**: Revert the code change. The `backlogit_sync_index` tool remains unchanged and provides full recovery.
* **Monitoring**: The `MergeSyncResult` JSON response includes `fallback_used` and `fallback_reason` — agents can monitor these fields to detect when incremental sync is not effective.
* **Validation window**: 1 week of agent usage across dev sessions. Monitor for drift complaints or index staleness reports.
* **Owner**: Ship agent for implementation; operator for production validation.

## Learnings Applied

| Learning | File Path | How Applied |
|---|---|---|
| Rehydration must be transactional | `docs/compound/database-issues/atomic-rehydration-sqlite-transaction-2026-04-08.md` | Incremental sync uses `RetryWrite`-wrapped transactions for all DB writes (Unit 3) |
| SQLITE_LOCKED must be in retry predicate | `docs/compound/db-reliability/sqlite-locked-missing-from-retry-predicate-2026-04-13.md` | Verified existing `RetryWrite` already handles SQLITE_LOCKED. All new writes routed through it (Unit 3) |
| Batch failures must propagate errors | `docs/compound/db-reliability/batch-failure-silent-nil-return-anti-pattern-2026-04-13.md` | Incremental sync fails the entire transaction on any upsert error (Risk #3, Unit 3) |

## Standards Check

| Standard | Compliance | Notes |
|---|---|---|
| Go 1.22+ | Yes | Uses standard library `filepath.WalkDir`, `time.Time`, `sync.RWMutex` |
| GoDoc on all exports | Required | All new exported types, functions, and constants must have GoDoc comments |
| `golangci-lint` zero warnings | Required | New code must pass lint |
| Error handling via sentinel/wrapping | Required | New errors wrap with `fmt.Errorf("context: %w", err)` |
| `log/slog` structured logging | Required | All diagnostic output via `slog.With("package", "db")` |
| TDD (test-first) | Required | Every unit specifies test-first execution |
| `RetryWrite` for DB writes | Required | Per compound learning and constitution |
| `path/filepath` for all paths | Required | No raw string concatenation |
| No `panic()` in library code | Required | — |
| Parameterized SQL queries | Required | Via existing `UpsertItemTx`/`DeleteItemTx` patterns |

## Plan Review

**Gate Decision: ADVISORY**

**Date**: 2026-04-21
**Reviewers**: Constitution Reviewer (Haiku), Go Quality Reviewer (Sonnet), Scope Boundary Auditor (Haiku), Learnings Researcher (Sonnet), Architecture Strategist (GPT-5.4), Agent-Native Parity Reviewer (GPT-5.4-mini)
**Plan hardening**: Not required (additive MCP tool, ephemeral cache writes, safety fallback exists)

### Summary

6 reviewer personas analyzed the plan across constitutional compliance, Go quality, scope boundaries, institutional learnings, architecture strategy, and agent-native parity. After deduplication and severity calibration: **0 P0, 0 P1, 12 P2, 5 P3**. No blocking issues found. Advisory findings presented for user discretion before harvest.

### P0 — Critical (must fix before proceeding)

None.

### P1 — High (should fix before proceeding)

None.

### P2 — Moderate (user discretion)

**P2-01: Per-unit verification checklists should mention slog and GoDoc explicitly** (CONST-001 + CONST-005)
Standards Check covers these at the plan level, but individual unit verification checklists do not reference specific slog logging points or GoDoc requirements. Risk: implementers may skip observability without per-unit reminders.
*Recommendation*: Add slog expectations to Units 2–4 verification (manifest build completion, diff results, fallback triggers, tool entry/exit).

**P2-02: Pre-init tool behavior needs explicit contract test** (CONST-003 + CONST-002)
The plan relies on the existing `requireWorkspace` pattern but Unit 5 contract tests do not explicitly verify that calling `backlogit_merge_sync` before workspace init returns a descriptive error.
*Recommendation*: Add contract test case: call merge_sync before init → expect descriptive JSON error.

**P2-03: JSON response schema needs contract test** (CONST-009)
Unit 5 should validate that MergeSyncResult JSON includes all expected fields (added, changed, deleted, relocated, stash_refreshed, logs_refreshed, fallback_used, dry_run) and is not raw file content.
*Recommendation*: Add schema validation contract test.

**P2-04: Non-goal boundary needs clarification** (SB-8)
Plan marks "incremental stash/log updates" as non-goal but triggers selective table rebuilds based on file changes. The distinction between "no line-level JSONL diffing" and "selective table rebuild triggering" is ambiguous.
*Recommendation*: Clarify non-goal to: "v1 rebuilds the entire stash/log table when any associated file changes; no line-level JSONL delta parsing."

**P2-05: Relocation detection should be marked deferrable** (SB-1 + SB-4 + SB-10)
Deliberation recommended relocation detection, so it is not scope creep. However, the plan should explicitly mark R7 as deferrable if implementation complexity exceeds estimate. Add ItemID uniqueness invariant test to Unit 5.
*Recommendation*: Mark R7 as "required but deferrable to v1.1 if complexity exceeds 2 hours."

**P2-06: Dry-run results are point-in-time snapshots** (SB-2 + SB-7)
Low implementation cost justifies inclusion, but agents may assume dry-run predicts the next real sync result. No documentation warns about this.
*Recommendation*: Add to tool description: "Dry-run results are point-in-time snapshots. Subsequent file changes may alter actual sync results."

**P2-07: Fallback threshold lacks empirical basis** (SB-3)
The 50%/50-file threshold is marked "deferred to implementation" for tuning. Consider starting higher (75%/100 files) to keep incremental sync as the dominant path while gathering real data.
*Recommendation*: Note the threshold is a tuning parameter; start conservatively and adjust based on telemetry.

**P2-08: Unit 5 test scope may be undersized** (SB-5)
Unit 5 covers contract validation, integration end-to-end, fallback triggering, and relocation detection in a "medium" estimate. Relocation collision, fallback boundary, dry-run isolation, and concurrent access scenarios need adequate coverage.
*Recommendation*: Consider expanding Unit 5 scope or splitting into Unit 5a (contract) and Unit 5b (integration).

**P2-09: Frontmatter-only parsing needs error case specification** (SB-6)
Unit 2 specifies reading first ~500 bytes for ID extraction but does not specify behavior for malformed frontmatter, missing id field, or id appearing after the frontmatter boundary.
*Recommendation*: Specify: malformed frontmatter → skip file with slog.Warn, empty ItemID in manifest entry. Add test cases.

**P2-10: Lock ordering convention needed** (GQ-1 + AS-1)
Two mutexes (workspaceMu and manifestMu) require a documented ordering convention to prevent potential deadlocks.
*Recommendation*: Document: never hold both simultaneously. ensureWorkspace releases workspaceMu before handleMergeSync acquires manifestMu.

**P2-11: Tool should appear in metadata catalog** (AP-1)
`backlogit_merge_sync` should be documented in `backlogit_get_metadata_catalog` output and the exported command map for agent discoverability.
*Recommendation*: Add tool to metadata catalog during Unit 4 implementation.

**P2-12: Response could include changed item IDs** (AP-2)
The response lists file paths in added/changed/deleted arrays but agents may want item IDs for selective knowledge refresh. Currently SyncEntry includes both id and path.
*Recommendation*: Confirm SyncEntry.ID is populated in all response arrays (including deleted items where the file is gone). Already designed correctly; verify in implementation.

### P3 — Low (advisory)

**P3-01**: Explicitly state "no new external dependencies" in Standards Check (CONST-006).
**P3-02**: Add negative test confirming no `file_manifest` table in SQLite (CONST-007).
**P3-03**: Note merge sync writes to SQLite only, not .md files — atomic file write concern (CONST-008) does not apply.
**P3-04**: FileKind classification is hardcoded; document that new `.backlogit/` file types require code changes to ClassifyFile (SB-9).
**P3-05**: Units 1 and 2 could potentially be parallelized since Unit 1 is pure types/logic (AS-2).

### Reviewer Attribution

| Finding | Reviewer | Model |
|---|---|---|
| P2-01 | Constitution Reviewer | Haiku |
| P2-02 | Constitution Reviewer | Haiku |
| P2-03 | Constitution Reviewer | Haiku |
| P2-04 | Scope Boundary Auditor | Haiku |
| P2-05 | Scope Boundary Auditor + Architecture Strategist | Haiku + GPT-5.4 |
| P2-06 | Scope Boundary Auditor | Haiku |
| P2-07 | Scope Boundary Auditor | Haiku |
| P2-08 | Scope Boundary Auditor | Haiku |
| P2-09 | Scope Boundary Auditor | Haiku |
| P2-10 | Go Quality Reviewer + Architecture Strategist | Sonnet + GPT-5.4 |
| P2-11 | Agent-Native Parity Reviewer | GPT-5.4-mini |
| P2-12 | Agent-Native Parity Reviewer | GPT-5.4-mini |
| P3-01–05 | Multiple | Mixed |

### Next Steps

Gate decision is **ADVISORY**. User decides: revise the plan to address P2 findings, or proceed to `harvest` with these findings acknowledged as implementation guidance.
