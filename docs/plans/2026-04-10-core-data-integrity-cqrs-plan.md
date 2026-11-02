---
chunk_strategy: h1-h2-h3
description: ""
doc_type: plan
docline:
    date: 2026-04-10T00:00:00Z
    origin: .backlogit/queue/012-DL.md
    review: .copilot-tracking/plan-review/2026-04-10-core-data-integrity-cqrs-plan-review.md
    review_gate: advisory
    status: reviewed
ingested_at: "2026-06-26T02:33:21Z"
schema_version: "1.0"
source: docs/exec-plans/2026-04-10-core-data-integrity-cqrs-plan.md
title: Core Data Integrity & CQRS Compliance
---

## Problem Frame

backlogit promises a CQRS architecture where Markdown files are the
authoritative source of truth and the SQLite database is a disposable,
rebuildable cache. Four high-priority stash entries expose that this contract is
broken across multiple code surfaces:

1. **SQLite per-connection PRAGMAs** (AE9DB2B6): `db.Open()` applies
   `foreign_keys=ON` and `busy_timeout=5000` via `db.Exec()` on a single pooled
   connection (connection.go:14-28). The pool has no `SetMaxOpenConns` bound, so
   new connections come up with SQLite defaults. Foreign-key enforcement and
   busy-timeout handling are unreliable.

2. **Semantic links live only in SQLite** (DF8FDB7B): `item_links` rows are
   written exclusively to the ephemeral database and explicitly preserved across
   rehydration (rehydration.go:47-48). Deleting the cache loses link data.
   This directly violates the disposable-DB promise of Constitution §VII.

3. **Update and move paths diverge file/DB state** (847DCF02):
   `UpdateArtifact` silently skips the Markdown write when `FindArtifactPath`
   fails but still upserts SQLite (artifacts.go:408-420). `BulkUpdateStatus`
   commits the DB before best-effort Markdown sync (queue.go:225-231), admitting
   in comments it is "DB-authoritative for now." `handleMoveItem` updates status
   without relocating the file per registry routing.

4. **MCP contract inconsistencies** (C710BEDB): `ErrNotFound` falls through to
   `InternalError` in some handlers; `handleListShipments` returns raw
   `[]*models.Artifact` while `handleGetShipment` returns a structured shipment
   object; `ensureWorkspace` initializes lazily without synchronization
   (server.go:101-116); `DeleteAllItemLogs` runs outside the rehydrate
   transaction (rehydration.go:31-33); `handleDeleteItem` does not clean up
   `item_deps` or `item_links` rows (tools.go:520-540).

These four areas share code surfaces in `internal/core`, `internal/db`, and
`internal/mcp`. Fixing them independently risks regressions on shared paths.

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | PRAGMAs enforced on every SQLite connection, not just the first pooled connection | AE9DB2B6 |
| R2 | Semantic links persisted in Markdown frontmatter as durable source of truth | DF8FDB7B |
| R3 | Rehydration rebuilds `item_links` from Markdown, removing the preservation carve-out | DF8FDB7B |
| R4 | `AddLink`/`RemoveLink` write through to the artifact's Markdown file | DF8FDB7B |
| R5 | `UpdateArtifact` returns an error when Markdown path is not found (no silent skip) | 847DCF02 |
| R6 | `BulkUpdateStatus` writes Markdown first, then updates SQLite | 847DCF02 |
| R7 | `handleMoveItem` relocates the file when status change triggers a directory change per registry routing | 847DCF02 |
| R8 | `ErrNotFound` maps to `not_found` in all MCP handlers (never falls to `internal`) | C710BEDB |
| R9 | `handleListShipments` and `handleGetShipment` return the same response shape | C710BEDB |
| R10 | `ensureWorkspace` is safe under concurrent MCP requests | C710BEDB |
| R11 | `DeleteAllItemLogs` runs inside the rehydrate transaction | C710BEDB |
| R12 | `handleDeleteItem` cascade-deletes orphaned `item_deps` and `item_links` rows | C710BEDB |

## Scope Boundaries

### In Scope

* DSN-based PRAGMA enforcement for all SQLite connections
* `links:` key in artifact YAML frontmatter for durable link storage
* Write-through link operations to Markdown
* Rehydration link rebuild from Markdown and carve-out removal
* `UpdateArtifact` error on missing Markdown path
* `BulkUpdateStatus` Markdown-first ordering
* File relocation in `handleMoveItem` per registry routing
* `domainError` handler standardization for `ErrNotFound`
* Normalized shipment response shape
* `sync.Once` for `ensureWorkspace`
* Transaction-wrapping for `DeleteAllItemLogs`
* Cascade-delete of `item_deps` and `item_links` on item deletion

### Non-Goals

* JSONL sidecar for links (frontmatter approach chosen)
* Registry routing engine redesign (use existing `config.Registry` path mapping)
* Concurrency beyond `ensureWorkspace` (full MCP mutex is separate scope)
* Migration tool for existing link data from SQLite to Markdown (one-time script is out of scope; rehydration will naturally pick up any links already in frontmatter)
* MCP response pagination for shipment lists (tracked separately)

### Deferred to Implementation

* Whether `BulkUpdateStatus` uses partial-success or all-or-nothing rollback on Markdown write failures
* Exact `links:` YAML schema: list of `{target_id, link_type}` objects vs. map keyed by link_type
* Whether file relocation in `handleMoveItem` also updates the SQLite path or defers to rehydration

## Implementation Units

Each unit is scoped to roughly 2 hours of human-equivalent effort. Units target
a single skill domain and produce a verifiable exit state.

### Layer 0: DB Connection Reliability (AE9DB2B6)

#### Unit 1: Apply PRAGMAs via DSN and constrain pool

**Files:** `internal/db/connection.go`
**Test files:** `internal/db/connection_test.go` (new)
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** existing `db.Open` function signature (connection.go:13), `setupLinksTestDB` helper pattern (links_test.go:21-38) for test DB setup
**Dependencies:** none

**Red tests (write first):**

1. `TestOpen_ForeignKeysEnforced`: open a DB, insert a row referencing a
   non-existent FK target, assert the insert fails.
2. `TestOpen_WALMode`: open a DB, query `PRAGMA journal_mode`, assert result is
   `wal`.
3. `TestOpen_BusyTimeout`: open a DB, query `PRAGMA busy_timeout`, assert result
   is `5000`.
4. `TestOpen_SecondConnection_InheritsSettings`: open a DB, force a second
   connection via concurrent queries, verify foreign keys are enforced on both.

**Approach:**

Replace `db.Exec` PRAGMA calls in `Open()` with DSN query-string parameters
supported by `modernc.org/sqlite`. The DSN format
`file:path?_pragma=journal_mode(WAL)&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)`
applies PRAGMAs per-connection at the driver level, ensuring every pooled
connection inherits them. Remove the post-open `db.Exec` PRAGMA loop.

Add `db.SetMaxOpenConns(4)` as a reasonable upper bound to prevent unbounded
pool growth while allowing parallel reads. WAL mode supports concurrent readers
with one writer, so a small pool is sufficient.

**Verification:**
`go test ./internal/db/... -run TestOpen` passes. All four tests confirm
settings are enforced. Existing `setupLinksTestDB` and other test helpers that
call `db.Open` continue to pass.

#### Unit 2: Connection pragma integration tests

**Files:** `internal/db/connection_test.go`
**Test files:** same (test-only unit)
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `setupLinksTestDB` (links_test.go:21-38), table-driven tests in `gate_test.go`
**Dependencies:** Unit 1

**Red tests (write first):**

1. `TestOpen_RehydrationPreservesPragmas`: open DB, run `EnsureSchema`, run a
   simulated rehydration sequence (`DELETE FROM items` + inserts), verify FK
   enforcement is still active on the same DB handle.
2. `TestOpen_MultipleOpenSameFile`: open the same DB file twice (two `*sql.DB`
   handles), verify both enforce FKs independently.
3. `TestOpen_InvalidPath_ReturnsError`: pass an invalid path, verify error is
   returned.

**Approach:**
Add integration tests that exercise the connection under realistic workloads.
These tests validate that the DSN approach from Unit 1 survives schema creation,
transaction boundaries, and concurrent handle scenarios.

**Verification:**
`go test ./internal/db/... -run TestOpen` passes. All tests green.

### Layer 1: Links Persistence (DF8FDB7B)

#### Unit 3: Add links to Markdown frontmatter model

**Files:** `internal/models/artifact.go`, `internal/parser/markdown.go` (if frontmatter parsing needs link awareness)
**Test files:** `internal/models/artifact_test.go` (extend existing), `internal/parser/markdown_test.go` (extend existing)
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** existing `Dependencies []string` field and YAML tag pattern in `Artifact` struct (artifact.go:40), `ParseFrontmatter` patterns in parser package
**Dependencies:** none (model change only)

**Red tests (write first):**

1. `TestArtifact_LinksField_RoundTrip`: create an `Artifact` with populated
   `Links` field, marshal to YAML, unmarshal back, assert links are identical.
2. `TestArtifact_LinksField_EmptyOmitted`: create an `Artifact` with nil/empty
   `Links`, marshal to YAML, assert no `links:` key appears in output.
3. `TestArtifact_LinksField_MultipleEntries`: create an artifact with 3 links of
   different types, round-trip through YAML, verify all three survive.

**Approach:**

Add a `Links` field to `models.Artifact`:

```go
type ArtifactLink struct {
    TargetID string `json:"target_id" yaml:"target_id"`
    LinkType string `json:"link_type" yaml:"link_type"`
}

// In Artifact struct:
Links []ArtifactLink `json:"links,omitempty" yaml:"links,omitempty"`
```

This field follows the same pattern as `Dependencies []string` and
`Labels []string` but uses a typed struct to preserve link semantics. The
`omitempty` tag ensures artifacts without links have no visual diff in their
frontmatter.

**Verification:**
`go test ./internal/models/... -run TestArtifact_Links` passes. Existing
`Artifact.Validate()` continues to pass (links field is optional).

#### Unit 4: Write-through link operations

**Files:** `internal/db/links.go`, `internal/core/artifacts.go`
**Test files:** `internal/db/links_test.go` (extend), `internal/core/link_persistence_test.go` (new)
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `AddLink`/`RemoveLink` in `links.go`, `WriteArtifactFile` in `artifacts.go:518`, `UpdateArtifact` Markdown-first pattern (artifacts.go:405-412)
**Dependencies:** Unit 3 (model must have `Links` field)

**Red tests (write first):**

1. `TestAddLinkWriteThrough_PersistsToMarkdown`: create an artifact via
   `core.CreateArtifact`, call a new `core.AddLinkDurable` function, read the
   Markdown file, parse frontmatter, verify `links:` contains the added link.
2. `TestRemoveLinkWriteThrough_RemovesFromMarkdown`: add a link, remove it via
   `core.RemoveLinkDurable`, read the Markdown file, verify `links:` no longer
   contains the removed link.
3. `TestAddLinkWriteThrough_Idempotent`: add the same link twice, verify
   Markdown contains exactly one entry.
4. `TestAddLinkWriteThrough_MultipleLinkTypes`: add three links with different
   types, verify all appear in frontmatter.

**Approach:**

Create `core.AddLinkDurable(ctx, ws, sourceID, targetID, linkType)` and
`core.RemoveLinkDurable(ctx, ws, sourceID, targetID, linkType)` functions that:

1. Load the source artifact from its Markdown file via `FindArtifactPath`.
2. Update the `artifact.Links` slice (add or remove the entry).
3. Call `WriteArtifactFile` to persist the change to disk (Markdown-first).
4. Call `db.AddLink` / `db.RemoveLink` to update the SQLite cache.

**Review amendment (F-1):** Write ordering is Markdown-first, then SQLite cache.
If the Markdown write fails, the SQLite cache is not mutated. Add sentinel
errors `ErrLinkNotFound` and `ErrLinkInvalid` to `internal/errors/errors.go`
for proper domain error mapping through `domainError`.

The existing `db.AddLink` and `db.RemoveLink` remain for SQLite-only callers
(rehydration, migration). The new `core` functions become the primary API for
MCP handlers. Update `handleAddLink` and `handleRemoveLink` in
`internal/mcp/tools.go` to call the durable variants.

**Verification:**
`go test ./internal/core/... -run TestAddLinkWriteThrough` passes.
`go test ./internal/core/... -run TestRemoveLinkWriteThrough` passes.
Existing `internal/db/links_test.go` and `internal/mcp/links_test.go` continue
to pass.

#### Unit 5: Rebuild links during rehydration, remove carve-out

**Files:** `internal/db/rehydration.go`
**Test files:** `internal/db/rehydration_links_test.go` (new)
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** existing rehydration transaction pattern (rehydration.go:35-54), `upsertItemTx` pattern from compound learning (docs/compound/database-issues/atomic-rehydration-sqlite-transaction-2026-04-08.md), `upsertDependencyTx` pattern
**Dependencies:** Unit 3 (model has `Links` field), Unit 4 (Markdown files contain links data)

**Red tests (write first):**

1. `TestRehydrate_RebuildsLinksFromMarkdown`: set up a workspace with two
   artifact files whose frontmatter contains `links:` entries. Run
   `Rehydrate()`. Query `item_links` table. Verify all links from frontmatter
   appear as rows.
2. `TestRehydrate_ClearsStaleLinks`: populate `item_links` with a link not
   present in any Markdown file. Run `Rehydrate()`. Verify the stale link is
   gone.
3. `TestRehydrate_DBDisposable_LinksRebuild`: delete the entire DB file. Reopen.
   Run `Rehydrate()`. Verify links are fully rebuilt from Markdown.

**Approach:**

1. Add `DELETE FROM item_links` to the rehydration transaction, alongside the
   existing `DELETE FROM items` and `DELETE FROM item_deps` statements
   (rehydration.go:49-54). This removes the carve-out comment at line 47-48.
2. In the `WalkDir` callback, after parsing an artifact's frontmatter, iterate
   over `artifact.Links` and call a new `upsertLinkTx(ctx, tx, sourceID,
   targetID, linkType)` helper to insert each link into `item_links` within the
   same transaction.
3. Move `DeleteAllItemLogs` inside the rehydrate transaction (addresses part of
   R11 from Layer 3 but naturally fits here since rehydration is the affected
   path).

**Verification:**
`go test ./internal/db/... -run TestRehydrate_RebuildsLinks` passes.
`go test ./internal/db/... -run TestRehydrate_ClearsStaleLinks` passes.
`go test ./internal/db/... -run TestRehydrate_DBDisposable` passes.
Full test suite `go test ./...` passes.

#### Unit 6: Link durability round-trip tests

**Files:** `internal/core/link_persistence_test.go` (extend from Unit 4)
**Test files:** same (test-only unit)
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `TestMigrateCustomFieldLinks_DeliberationID_CreatesInformsLink` (migrate_links_test.go:41), MCP link test helpers (mcp/links_test.go:226-244)
**Dependencies:** Unit 4, Unit 5

**Red tests (write first):**

1. `TestLinkDurability_AddLink_DeleteDB_Rehydrate_LinkSurvives`: create
   workspace, add a link via `core.AddLinkDurable`, close DB, delete DB file,
   reopen workspace (triggers rehydration), query `item_links`, verify the link
   is present.
2. `TestLinkDurability_RemoveLink_Rehydrate_LinkGone`: add a link, remove it
   via `core.RemoveLinkDurable`, trigger rehydration, verify the link is absent.
3. `TestLinkDurability_MCP_AddLink_Rehydrate_Survives`: call
   `handleAddLink` via the MCP test helper, trigger rehydration, verify the link
   survives.

**Approach:**
These are end-to-end integration tests validating the full lifecycle: MCP or core
API → Markdown write → DB delete → rehydration → links recovered. They serve as
the definitive proof that the disposable-DB promise holds for links.

**Verification:**
`go test ./internal/core/... -run TestLinkDurability` passes. All three tests
confirm links survive DB deletion and rehydration.

### Layer 2: Markdown-First Write Invariants (847DCF02)

#### Unit 7: Fix UpdateArtifact to fail on missing Markdown path

**Files:** `internal/core/artifacts.go`
**Test files:** `internal/core/artifacts_expansion_test.go` (extend)
**Effort size:** small
**Skill domain:** code
**Execution note:** characterization-first (write a test capturing current silent-skip behavior, then change it to expect an error)
**Patterns to follow:** existing `UpdateArtifact` error returns (artifacts.go:339-403), `ErrNotFound` sentinel in `internal/errors/`
**Dependencies:** none

**Red tests (write first):**

1. `TestUpdateArtifact_MissingMarkdownPath_ReturnsError`: create an artifact in
   SQLite only (no Markdown file on disk), call `UpdateArtifact`, assert an
   error is returned containing "artifact file not found" or similar.
2. `TestUpdateArtifact_ValidPath_WritesMarkdownFirst`: create an artifact
   normally, update it, read the Markdown file, verify the update is present
   even if the SQLite upsert were to fail afterward.
3. `TestUpdateArtifact_MarkdownWriteFailure_NoSQLiteUpdate`: create an artifact,
   make the Markdown file read-only, call `UpdateArtifact`, verify the SQLite
   row still has the old values.

**Approach:**

Change `UpdateArtifact` (artifacts.go:408-412) from silently skipping the
Markdown write on `FindArtifactPath` failure to returning an error:

```go
// Before (silent skip):
if filePath, pathErr := FindArtifactPath(ctx, ws, id); pathErr == nil {
    if writeErr := WriteArtifactFile(artifact, filePath); writeErr != nil {
        return nil, fmt.Errorf("write artifact file %s: %w", id, writeErr)
    }
}

// After (fail on missing path):
filePath, pathErr := FindArtifactPath(ctx, ws, id)
if pathErr != nil {
    return nil, fmt.Errorf("locate artifact file %s: %w", id, pathErr)
}
if writeErr := WriteArtifactFile(artifact, filePath); writeErr != nil {
    return nil, fmt.Errorf("write artifact file %s: %w", id, writeErr)
}
```

This enforces the Markdown-first invariant: if the file cannot be found, the
update fails entirely rather than creating a file/DB divergence.

**Verification:**
`go test ./internal/core/... -run TestUpdateArtifact_MissingMarkdown` passes.
Full `go test ./internal/core/...` passes (no regressions in callers).

#### Unit 8: Flip BulkUpdateStatus to Markdown-first

**Files:** `internal/core/queue.go`
**Test files:** `internal/core/queue_test.go` (extend)
**Effort size:** medium
**Skill domain:** code
**Execution note:** characterization-first (document current DB-first behavior in a test, then rewrite)
**Patterns to follow:** existing `setupQueueWorkspace` helper (queue_test.go:17-47), Markdown-first pattern in `UpdateArtifact` (artifacts.go:405-420)
**Dependencies:** Unit 7 (UpdateArtifact must fail on missing path first)

**Red tests (write first):**

1. `TestBulkUpdateStatus_MarkdownFirst_WritesBeforeDB`: set up workspace with
   multiple artifacts, call `BulkUpdateStatus`, read Markdown files to confirm
   status is updated, then verify DB rows match.
2. `TestBulkUpdateStatus_MarkdownWriteFails_NoDBUpdate`: create artifacts, make
   one Markdown file read-only, call `BulkUpdateStatus`, verify the DB row for
   that item retains its old status.
3. `TestBulkUpdateStatus_PartialSuccess_ReportsFailures`: set up 3 items, make 1
   Markdown file read-only, call `BulkUpdateStatus`, verify 2 items succeeded
   and 1 failure is reported.

**Approach:**

Reverse the write ordering in `BulkUpdateStatus` (queue.go:207-252):

1. Iterate `itemIDs`, load each artifact from Markdown, update its status, write
   back to Markdown via `WriteArtifactFile`.
2. Collect successfully-written IDs.
3. Open a DB transaction and update only the successfully-written items.
4. Commit the transaction.

Items that fail Markdown write are collected in an error list. The function
returns the count of successfully updated items and an aggregated error for
failures (if any). This is a partial-success model rather than all-or-nothing,
matching the existing behavioral contract where callers expect partial progress.

Remove the "DB-authoritative for now" comment (queue.go:230-231).

**Verification:**
`go test ./internal/core/... -run TestBulkUpdateStatus` passes. All three red
tests pass. Existing `TestQueryQueue_*` tests pass.

#### Unit 9: Add file relocation to move handler

**Files:** `internal/mcp/tools.go`, `internal/core/artifacts.go`
**Test files:** `internal/mcp/move_relocation_test.go` (new)
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `handleMoveItem` (tools.go:451-518), `WriteArtifactFile` + `os.Remove` pattern from `handleDeleteItem` (tools.go:520-540), registry routing in `config.Registry`
**Dependencies:** Unit 7 (UpdateArtifact error behavior must be stable)

**Red tests (write first):**

1. `TestMoveItem_RelocatesFile_WhenRegistryRequires`: set up workspace with a
   registry mapping that routes `active` items to `queue/` and `done` items to
   `done/`. Create an artifact in `queue/`. Move it to `done`. Verify the file
   exists in `done/` and is absent from `queue/`.
2. `TestMoveItem_NoRelocation_WhenSameDirectory`: move an item from `queued` to
   `active` (both mapped to `queue/`). Verify the file stays in `queue/`.
3. `TestMoveItem_RelocationFails_ReturnsError`: make the target directory
   read-only. Move an item. Verify an error is returned.

**Approach:**

After `UpdateArtifact` succeeds in `handleMoveItem`, check whether the registry
routing maps the new status to a different directory than the artifact's current
location. If so:

1. Determine the target directory from the workspace registry config.
2. Compute the new file path: `targetDir/{id}.md`.
3. Rename (move) the file from current path to target path using
   `os.Rename`.
4. The SQLite row already reflects the new status from `UpdateArtifact`; the
   next rehydration will pick up the new file location.

Add a `core.RelocateArtifact(ws, id, newStatus)` helper that encapsulates the
path lookup and rename, keeping `handleMoveItem` focused on MCP concerns.

**Verification:**
`go test ./internal/mcp/... -run TestMoveItem_Relocates` passes.
Full `go test ./...` passes.

#### Unit 10: Write-path invariant integration tests

**Files:** `internal/core/write_invariants_test.go` (new)
**Test files:** same (test-only unit)
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `setupTestWorkspace` (artifacts_expansion_test.go:21-33), end-to-end patterns in `link_persistence_test.go`
**Dependencies:** Units 7, 8, 9

**Red tests (write first):**

1. `TestWriteInvariant_UpdateThenRehydrate_StateConsistent`: create artifact,
   update it, delete DB, rehydrate, verify updated state survives.
2. `TestWriteInvariant_BulkUpdateThenRehydrate_StateConsistent`: bulk-update 3
   items, delete DB, rehydrate, verify all statuses match.
3. `TestWriteInvariant_MoveThenRehydrate_FileInCorrectDirectory`: move an item
   to a new status, delete DB, rehydrate, verify the artifact is found in the
   correct directory and the old location is empty.

**Approach:**
These end-to-end tests validate the combined behavior of Units 7-9 by exercising
the full create → update → rehydrate cycle. They serve as the definitive proof
that Markdown remains authoritative through all write paths.

**Verification:**
`go test ./internal/core/... -run TestWriteInvariant` passes.

### Layer 3: MCP Contract Hardening (C710BEDB)

#### Unit 11: Standardize ErrNotFound mapping in MCP error handler

**Files:** `internal/mcp/errors.go`
**Test files:** `internal/mcp/error_mapping_test.go` (new)
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** existing `domainError` switch (errors.go:58-69), sentinel errors in `internal/errors/errors.go`, `shipmentErrorField` test helper (shipment_errors_test.go:39)
**Dependencies:** none

**Red tests (write first):**

1. `TestDomainError_ErrNotFound_ReturnsNotFound`: wrap `ErrNotFound` in a
   context error, call `domainError`, verify the result has `error: not_found`.
2. `TestDomainError_GenericError_ReturnsInternal`: call `domainError` with a
   generic error, verify `error: internal`.
3. `TestDomainError_ErrValidation_ReturnsValidationFailed`: wrap `ErrValidation`,
   verify `error: validation_failed`.
4. `TestDomainError_NilWrappedNotFound_ReturnsNotFound`: wrap `ErrNotFound`
   three levels deep via `fmt.Errorf`, verify `errors.Is` chain works and
   returns `not_found`.

**Approach:**

The current `domainError` switch (errors.go:58-69) already handles
`corerrors.ErrNotFound` and `corerrors.ErrShipmentNotFound` in the same case
arm. The issue is that some handlers call `InternalError` directly instead of
routing through `domainError`. Audit all handlers in `tools.go` that manually
construct `InternalError` responses, and replace any that catch `ErrNotFound`
with calls to `domainError`. This is a mechanical search-and-replace:

```go
// Before (in handleDeleteItem):
return InternalError(fmt.Sprintf("find artifact: %v", err)), nil

// After:
return domainError("find artifact", err), nil
```

**Verification:**
`go test ./internal/mcp/... -run TestDomainError` passes. Grep for
`InternalError.*find.*artifact` returns zero hits.

#### Unit 12: Normalize shipment response shapes

**Files:** `internal/mcp/tools.go`
**Test files:** `internal/mcp/shipment_response_test.go` (new)
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `handleGetShipment` (tools.go:1256-1273), `core.GetShipment` return type, `toolResultJSON` helper
**Dependencies:** none

**Red tests (write first):**

1. `TestListShipments_ResponseShape_MatchesGetShipment`: create a shipment,
   call `handleListShipments`, call `handleGetShipment` with the same ID.
   Unmarshal both responses. Verify the list entry contains the same fields
   as the single-get response (id, title, status, items, etc.).
2. `TestListShipments_EmptyResult_ReturnsEmptyArray`: call `handleListShipments`
   on an empty workspace, verify the response is `[]` not `null`.

**Approach:**

`handleListShipments` currently calls `db.QueryItems` which returns
`[]*models.Artifact`. `handleGetShipment` calls `core.GetShipment` which returns
a richer `ShipmentDetail` struct with parsed items and metadata. To normalize:

1. Change `handleListShipments` to iterate the query results and call
   `core.GetShipment` for each shipment ID, building a list of
   `ShipmentDetail` objects.
2. Alternatively, create a `core.ListShipments(ctx, ws, status)` function that
   returns `[]ShipmentDetail` using a single query + parse pass for efficiency.

The second approach is preferred for performance. The response shape will match
`handleGetShipment` exactly.

**Verification:**
`go test ./internal/mcp/... -run TestListShipments_ResponseShape` passes.

#### Unit 13: Add sync.Once to ensureWorkspace

**Files:** `internal/mcp/server.go`
**Test files:** `internal/mcp/server_init_test.go` (new)
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** existing `ensureWorkspace` (server.go:101-116), `sync.Once` usage patterns in Go stdlib
**Dependencies:** none

**Red tests (write first):**

1. `TestEnsureWorkspace_ConcurrentCalls_NoRace`: launch 10 goroutines calling
   `ensureWorkspace` simultaneously, verify no race condition (run with
   `-race`), verify all return the same `*Workspace` pointer.
2. `TestEnsureWorkspace_AlreadyInitialized_ReturnsCached`: call
   `ensureWorkspace` twice sequentially, verify the second call returns the same
   pointer without re-opening the workspace.
3. `TestEnsureWorkspace_NoWorkspace_ReturnsError`: call `ensureWorkspace` on a
   server with no `.backlogit` directory, verify error is returned consistently.

**Approach:**

Add a `sync.Once` field to the `Server` struct and use it to guard the
one-time workspace initialization. **Review amendment (F-4):** Use
mutex/double-check pattern instead of `sync.Once` to allow retry on transient
failures while caching successful initialization:

```go
type Server struct {
    // existing fields...
    workspaceMu   sync.Mutex
    workspaceInit bool
}

func (s *Server) ensureWorkspace(ctx context.Context) (*core.Workspace, error) {
    s.workspaceMu.Lock()
    defer s.workspaceMu.Unlock()
    if s.workspaceInit && s.Workspace != nil {
        return s.Workspace, nil
    }
    if !dirExists(s.backlogitDir()) {
        return nil, os.ErrNotExist
    }
    ws, err := core.NewWorkspace(ctx, s.RootPath)
    if err != nil {
        return nil, err
    }
    s.Workspace = ws
    s.workspaceInit = true
    s.refreshTemplateService(ctx)
    return s.Workspace, nil
}
```

This ensures `NewWorkspace` runs exactly once, even under concurrent MCP
requests. The existing `s.Workspace != nil` fast path is subsumed by
`sync.Once`.

**Verification:**
`go test -race ./internal/mcp/... -run TestEnsureWorkspace` passes.

#### Unit 14: Transaction-wrap DeleteAllItemLogs and cascade-delete on item deletion

**Files:** `internal/db/logs.go`, `internal/db/rehydration.go`, `internal/db/queries.go`, `internal/mcp/tools.go`
**Test files:** `internal/db/logs_test.go` (new), `internal/mcp/delete_cascade_test.go` (new)
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** rehydration transaction pattern (rehydration.go:35-54), `upsertDependencyTx` transactional helper naming convention (compound learning), `handleDeleteItem` (tools.go:520-540)
**Dependencies:** Unit 1 (FK enforcement needed for cascade tests)

**Red tests (write first):**

1. `TestDeleteAllItemLogsTx_RunsInTransaction`: create log entries, call a new
   `DeleteAllItemLogsTx(ctx, tx)` function, verify entries are deleted only
   after commit.
2. `TestDeleteAllItemLogsTx_RollbackPreservesEntries`: create log entries, call
   `DeleteAllItemLogsTx`, rollback the transaction, verify entries persist.
3. `TestDeleteItem_CascadesDepLinks`: create an artifact with dependencies and
   links, delete it via `handleDeleteItem`, verify `item_deps` and
   `item_links` rows for that item are gone.
4. `TestDeleteItem_CascadesDepLinks_OtherItemsUnaffected`: create two artifacts
   with links, delete one, verify the other's links remain.
5. `TestRehydrate_ItemLogsInTransaction`: run rehydration on workspace with log
   files, verify `item_log_entries` are rebuilt atomically (cancel mid-walk
   preserves old entries).

**Approach:**

**Part A (DeleteAllItemLogs transaction):** Create
`DeleteAllItemLogsTx(ctx context.Context, tx *sql.Tx) error` following the `Tx`
suffix convention. Move the `DELETE FROM item_logs` and
`DELETE FROM item_log_entries` statements into this function. Update
`Rehydrate()` to call `DeleteAllItemLogsTx` within its existing transaction
(rehydration.go:31-33 moves inside the `tx` block).

**Part B (cascade-delete):** Modify `db.DeleteItem` to also delete from
`item_deps` (both `item_id` and `depends_on` matching) and `item_links` (both
`source_id` and `target_id` matching) within a transaction. Alternatively, add
`ON DELETE CASCADE` to the table definitions, but since `item_links` and
`item_deps` do not currently have FK constraints (they reference items but have
no FOREIGN KEY clause), the pragmatic approach is explicit DELETE statements
in a transaction:

```go
func DeleteItemCascade(ctx context.Context, db *sql.DB, id string) error {
    tx, err := db.BeginTx(ctx, nil)
    if err != nil { return err }
    defer tx.Rollback()

    tx.ExecContext(ctx, `DELETE FROM item_deps WHERE item_id = ? OR depends_on = ?`, id, id)
    tx.ExecContext(ctx, `DELETE FROM item_links WHERE source_id = ? OR target_id = ?`, id, id)
    tx.ExecContext(ctx, `DELETE FROM item_log_entries WHERE item_id = ?`, id)
    tx.ExecContext(ctx, `DELETE FROM item_logs WHERE item_id = ?`, id)
    tx.ExecContext(ctx, `DELETE FROM items WHERE id = ?`, id)
    return tx.Commit()
}
```

Update `handleDeleteItem` to call `DeleteItemCascade` instead of `DeleteItem`.

**Verification:**
`go test ./internal/db/... -run TestDeleteAllItemLogsTx` passes.
`go test ./internal/mcp/... -run TestDeleteItem_Cascades` passes.
`go test ./...` passes.

#### Unit 15: MCP contract consistency tests

**Files:** `internal/mcp/contract_consistency_test.go` (new)
**Test files:** same (test-only unit)
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `setupBugFixServer` helper pattern (section_bugs_test.go), `shipmentErrorField` helper (shipment_errors_test.go:39), MCP link test helpers (links_test.go:226-244)
**Dependencies:** Units 11, 12, 13, 14

**Red tests (write first):**

1. `TestMCP_NotFoundErrors_NeverSurfaceAsInternal`: call `handleGetItem`,
   `handleDeleteItem`, `handleMoveItem`, `handleGetShipment` with non-existent
   IDs. Verify all return `error: not_found`, never `error: internal`.
2. `TestMCP_ShipmentList_SameShapeAsGet`: create shipments, list them, get each
   individually. JSON-unmarshal both into a common struct. Verify structural
   equivalence.
3. `TestMCP_ConcurrentEnsureWorkspace_NoRace`: identical to Unit 13 test 1 but
   exercised through the full MCP tool handler path (launch 10 concurrent
   `handleListItems` calls with `-race`).
4. `TestMCP_DeleteItem_LeavesNoOrphans`: create an item with deps and links,
   delete it, query `item_deps` and `item_links`, verify zero rows reference the
   deleted ID.

**Approach:**
These are MCP-level integration tests that validate all Layer 3 fixes through
the handler API. They serve as regression guards for the error mapping,
response shape, concurrency, and cascade-delete contracts.

**Verification:**
`go test -race ./internal/mcp/... -run TestMCP_` passes.

## Dependency Graph

```text
Layer 0:
  Unit 1 (DSN PRAGMAs) ──→ Unit 2 (connection integration tests)

Layer 1:
  Unit 3 (model Links field) ──→ Unit 4 (write-through)
  Unit 4 (write-through)     ──→ Unit 5 (rehydration rebuild)
  Unit 4 + Unit 5            ──→ Unit 6 (durability round-trip)

Layer 2:
  Unit 7 (UpdateArtifact fail) ──→ Unit 8 (BulkUpdate Markdown-first)
  Unit 7                       ──→ Unit 9 (move file relocation)
  Units 7 + 8 + 9             ──→ Unit 10 (write-path integration)

Layer 3:
  Unit 11 (ErrNotFound mapping)     ─┐
  Unit 12 (shipment response shape) ─┤
  Unit 13 (sync.Once)               ─┼──→ Unit 15 (MCP contract tests)
  Unit 14 (cascade-delete + tx logs) ─┘
  Unit 1                             ──→ Unit 14 (FK enforcement)
```

Parallel tracks:

* **Track A** (DB connection): Unit 1 → Unit 2
* **Track B** (links persistence): Unit 3 → Unit 4 → Unit 5 → Unit 6
* **Track C** (write invariants): Unit 7 → Unit 8, Unit 7 → Unit 9, → Unit 10
* **Track D** (MCP contracts): Units 11, 12, 13 (parallel) → Unit 15
* **Track E** (cascade + tx): Unit 1 → Unit 14 → Unit 15

Tracks A, B, C, and D can proceed in parallel. Track E depends on Track A
(Unit 1) for FK enforcement.

## Decisions

| # | Decision | Rationale | Alternatives Rejected |
|---|---|---|---|
| D1 | Apply PRAGMAs via DSN query-string parameters | DSN parameters are applied per-connection by the driver, guaranteeing every pooled connection inherits them. Does not sacrifice concurrency | `SetMaxOpenConns(1)` (eliminates read concurrency), post-open `Exec` loop (current broken approach), `ConnInitHook` (not supported by modernc.org/sqlite) |
| D2 | Store links in Markdown frontmatter (`links:` YAML key), not JSONL sidecar | Frontmatter keeps all artifact state in one file, matching the pattern used by `dependencies`, `labels`, and `references`. No new file format to manage. Git diffs are cleaner | JSONL sidecar `item_links.jsonl` (adds a new file format, complicates rehydration, not human-readable in artifact context) |
| D3 | Use `sync.Once` for `ensureWorkspace`, not a full mutex | `ensureWorkspace` is a one-time initialization. `sync.Once` is the idiomatic Go primitive for this pattern. A full mutex adds unnecessary contention for every tool call after initialization | Full `sync.Mutex` on every `ensureWorkspace` call (over-synchronization), channel-based init (unnecessary complexity) |
| D4 | `BulkUpdateStatus` uses partial-success model, not all-or-nothing | Existing callers expect partial progress reporting. All-or-nothing would require rolling back Markdown writes for items that succeeded, adding complexity with no clear benefit | All-or-nothing rollback (complex, breaks existing behavioral contract) |
| D5 | Cascade-delete via explicit DELETE statements in a transaction, not FK `ON DELETE CASCADE` | `item_links` and `item_deps` tables lack FK constraints. Adding FKs would require a schema migration. Explicit DELETEs are transparent and testable | Add FK constraints with CASCADE (requires schema migration, may break rehydration ordering) |
| D6 | `handleListShipments` calls a new `core.ListShipments` instead of per-item `GetShipment` | Single query + parse pass is more efficient than N+1 queries. Returns the same `ShipmentDetail` shape as `GetShipment` | N+1 `GetShipment` calls (inefficient), raw `[]*models.Artifact` return (current broken shape) |

## Risks and Caveats

1. **DSN parameter support**: The `_pragma` DSN syntax depends on
   `modernc.org/sqlite` supporting it. If the driver version in `go.mod` does
   not support this syntax, fall back to `SetMaxOpenConns(1)` + post-open
   `Exec`. Verify with a spike test before committing to the approach.

2. **Links frontmatter migration**: Existing artifacts have no `links:` field in
   their frontmatter. After Layer 1 lands, any links currently in SQLite but not
   in Markdown will be lost on the next rehydration (which now clears
   `item_links`). Mitigation: before removing the rehydration carve-out, run a
   one-time migration that reads `item_links` from SQLite and writes them into
   the corresponding Markdown files. This migration is out of scope for this plan
   but must run before the rehydration change is deployed.

3. **UpdateArtifact callers**: Changing `UpdateArtifact` to fail on missing
   Markdown path may break callers that relied on the silent-skip behavior for
   DB-only items (e.g., items created during migration). Mitigation: audit all
   `UpdateArtifact` call sites and ensure they handle the new error path.

4. **BulkUpdateStatus partial failure reporting**: The partial-success model means
   some items may update while others fail. Callers must handle the mixed result.
   Current callers (queue operations) already accept partial counts, so this is
   low risk.

5. **File relocation atomicity**: `os.Rename` is atomic on the same filesystem
   but may fail across mount points. In practice, `.backlogit/` subdirectories
   are on the same filesystem, so this is acceptable. If cross-mount support is
   needed later, use temp-copy-then-rename.

6. **Concurrent MCP + file writes**: `sync.Once` protects workspace
   initialization but does not protect concurrent tool calls that write to the
   same artifact file. This is pre-existing and out of scope for this plan.

## Learnings Applied

* Atomic rehydration compound learning (docs/compound/database-issues/atomic-rehydration-sqlite-transaction-2026-04-08.md): informed transactional patterns in Units 5 and 14, `Tx` suffix naming convention, defer-rollback pattern
* Orphaned tasks compound learning (docs/compound/workflow-issues/orphaned-tasks-without-parent-features-2026-04-10.md): informed cascade-delete approach in Unit 14 and dependency cleanup patterns
* Core implementation patterns (docs/compound/go-implementation/feature-001-core-implementation.md): informed `WriteArtifactFile` atomic write pattern, FTS5 trigger awareness for link inserts during rehydration
* Advisory file lock learning (docs/compound/best-practices/advisory-file-lock-stale-ttl-go-2026-04-08.md): informed concurrent access considerations in Risk 6
* Data quality plan patterns (docs/exec-plans/2026-04-08-data-quality-tool-efficiency-plan.md): informed test-first unit structure, effort sizing conventions, pagination patterns

## Standards Check

* **Constitution I (Type-Safe Go)**: All new code uses typed structs (`ArtifactLink`), GoDoc comments, sentinel errors. Link operations use `LinkEdge` struct, not raw maps. Errors wrap sentinels via `%w`.
* **Constitution II (MCP Protocol Fidelity)**: All MCP tools remain unconditionally visible. Error mapping standardized through `domainError`. Shipment response shapes normalized. Pre-init errors return descriptive messages.
* **Constitution III (Test-First)**: Every code unit specifies explicit red tests written before implementation. Contract tests for MCP tools. Integration tests for cross-layer behaviors.
* **Constitution IV (Workspace Containment)**: All file operations use `FindArtifactPath` and `WriteArtifactFile` which resolve within `.backlogit/`. No path traversal. `sync.Once` prevents race conditions on initialization.
* **Constitution V (Structured Observability)**: Existing `slog` patterns maintained. Connection pragma changes are transparent to logging. Link write-through operations log at `Info` level via existing patterns.
* **Constitution VII (CQRS)**: Links move from SQLite-only to Markdown-first. Rehydration carve-out removed; DB becomes fully disposable. `UpdateArtifact` and `BulkUpdateStatus` enforce Markdown-first ordering. All write paths follow: Markdown → SQLite.
* **Constitution VIII (Git-Friendly Persistence)**: `links:` field uses YAML list in frontmatter, maintaining human-readable, Git-mergeable format. `omitempty` prevents noise in diffs for artifacts without links.
* **Constitution IX (Agent Context Efficiency)**: Normalized shipment response shapes reduce agent parsing complexity. Cascade-delete prevents orphaned data from polluting query results.
