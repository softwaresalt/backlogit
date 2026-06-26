---
chunk_strategy: h1-h2-h3
description: Implementation plan for fixing stale cross-artifact frontmatter references after AdoptItem ID rewrites
doc_type: plan
docline:
    date: 2026-04-20T00:00:00Z
    origin: .backlogit/queue/035-F.md
    status: draft
ingested_at: "2026-06-26T02:33:21Z"
schema_version: "1.0"
source: docs/exec-plans/2026-04-20-adoptitem-cross-reference-rewrite-plan.md
title: AdoptItem Cross-Reference Rewrite
---

## Problem Frame

When `AdoptItem` reassigns an orphaned or unparented artifact to a new parent
feature, it rewrites the artifact's hierarchical ID (e.g., `012-T` becomes
`025.012-T`). The function correctly updates:

* The artifact's own frontmatter (ID, parent_id)
* DB edge tables: `item_deps`, `item_links`, `commit_links`, `stash_links`,
  `item_logs`, `item_log_entries`
* File system: renames `.md` and `.jsonl` files

However, **other artifacts** whose Markdown frontmatter references the old ID
are not updated. Three frontmatter fields can contain cross-references:

* `parent_id` — child artifacts whose parent was the adopted item
* `dependencies` — artifacts listing the old ID as a dependency
* `links` — artifacts with semantic links targeting the old ID

Because Markdown is the source of truth (Constitution VII), the next
`backlogit sync` (rehydration) rebuilds the DB from stale Markdown, reverting
the corrected DB edges to their pre-adoption state. The fix is ephemeral; the
bug is permanent.

### Scope Boundary

This plan addresses cross-artifact frontmatter reference rewrites during
`AdoptItem` only. It does not address:

* Shipment manifest item lists (already handled by `RewriteAncillaryReferences`
  for DB rows; shipment `custom_fields.items` is a JSON blob that may need
  separate handling — deferred to implementation discovery)
* External references outside `.backlogit/` (out of scope per AdoptItem's
  existing contract: "external references are the caller's responsibility")

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | After AdoptItem, no Markdown file in `.backlogit/` contains the old artifact ID in `parent_id`, `dependencies`, or `links` fields | Stash C00AA592 / 035-F |
| R2 | Cross-reference rewrites survive rehydration: `backlogit sync` preserves corrected edges | Stash C00AA592 (rehydration staleness) |
| R3 | Cross-reference rewrites are atomic with the AdoptItem operation (all-or-nothing) | Constitution VII (CQRS), VIII (atomic writes) |

## Scope Boundaries

### In Scope

* New cross-artifact reference scanner/rewriter in `internal/core/artifact_references.go`
* Integration into `AdoptItem` transaction window (split-phase: collect outside tx, write inside short tx)
* Frontmatter field rewrites: `parent_id`, `dependencies`, `links[].target_id`
* Unit tests covering all cross-reference scenarios
* Integration test proving rehydration consistency

### Non-Goals

* Changing AdoptItem's existing ID generation, file rename, or DB edge logic
* Fixing the separate `archived_from` self-referential bug (future stash item)
* Cross-artifact reference rewrites for operations other than AdoptItem
* Shipment `custom_fields.items` rewriting (deferred to follow-up — not proven
  to participate in the rehydration corruption path, and introduces type-safety
  complexity from `[]any` vs `[]string` JSON round-trips per review F-004)

### Deferred to Implementation

* Whether `custom_fields` values beyond `items` can contain artifact ID
  references (should be assessed and stashed as follow-up if found)
* Optimal batch size for cross-artifact file writes in large workspaces

## Implementation Units

Each unit MUST be scoped to roughly 2 hours of human-equivalent effort.

### Unit 1: Cross-Artifact Reference Scanner and Rewriter

**Files:** `internal/core/artifact_references.go` (new file)
**Test files:** `internal/core/artifact_references_test.go` (new file)
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `artifactSearchDirs` (shipment_lifecycle.go:494) for
workspace scanning, `WriteArtifactFile` (artifacts.go:579) for atomic file
writes, `findArtifact` (artifacts.go:634) for artifact loading, existing
AdoptItem file-rollback pattern (shipment_lifecycle.go:652-658)
**Dependencies:** none

**Approach:**

Create two new unexported functions in a new file `internal/core/artifact_references.go`
(per review F-009, this is a workspace-level concern, not shipment-specific):

**Phase 1 — Collect (outside transaction, no DB lock):**

```go
type crossRefUpdate struct {
    artifact *models.Artifact
    filePath string
    snapshot []byte // file content before rewrite, for rollback
}

func findCrossArtifactReferences(ctx context.Context, ws *Workspace, oldID, newID string) ([]crossRefUpdate, error)
```

This function:

1. Calls `artifactSearchDirs(ws)` to get all search directories
2. Walks each directory for `.md` files, parsing each via `parseFile`
3. For each parsed artifact (excluding the adopted artifact itself and `newID`):
   a. Checks `parent_id == oldID` → marks for rewrite to `newID`
   b. Checks each entry in `dependencies` slice for exact match `oldID` → marks
   c. Checks each entry in `links` slice for `target_id == oldID` → marks
4. For each modified artifact:
   a. Snapshots the current file content (for rollback)
   b. Rewrites the in-memory struct fields
   c. Sets `UpdatedAt` to `time.Now()`
5. Returns the collected updates WITHOUT writing anything

The collect phase holds no DB lock and does no file I/O writes.

**Phase 2 — Apply (inside short transaction window):**

```go
func applyCrossArtifactRewrites(ctx context.Context, ws *Workspace, tx *sql.Tx, updates []crossRefUpdate) error
```

This function:

1. For each update:
   a. Writes the artifact via `WriteArtifactFile` (atomic tmp+rename)
   b. Upserts the artifact row via `bldb.UpsertItemTx(ctx, tx, artifact)`
   c. Deletes and reinserts dep/link rows for this artifact within the
      transaction (per review F-002, `UpsertItemTx` only updates the `items`
      row — dep/link rows must be explicitly refreshed)
2. On any failure:
   a. Restores all previously written files from their snapshots (per review F-001)
   b. Returns the error (the caller's `defer tx.Rollback()` handles DB)

The apply phase is short: no filesystem walks, no parsing, only targeted writes.

**Verification:**

* Test: create 3 artifacts where B has `parent_id: A`, C has
  `dependencies: [A]`, D has `links: [{target_id: A}]`. Call
  `findCrossArtifactReferences`. Assert all 3 are found with correct rewrites.
* Test: artifact with no references to old ID is not in results
* Test: adopted artifact itself (matching newID) is excluded
* Test: exact-match only — ID "001-T" does not match "001-T" substring
  in "0001-T" (prevent false-positive substring replacement)
* Test: `applyCrossArtifactRewrites` writes files and verifies frontmatter
* Test: `applyCrossArtifactRewrites` restores snapshots on write failure

### Unit 2: AdoptItem Integration

**Files:** `internal/core/shipment_lifecycle.go`
**Test files:** `internal/core/shipment_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** characterization-first (existing tests must still pass)
**Patterns to follow:** existing AdoptItem transaction flow (lines 566-660),
existing rollback pattern (lines 652-658)
**Dependencies:** Unit 1

**Approach:**

Modify AdoptItem to use the two-phase approach:

**Before the transaction opens** (between line 558 and 566):
Call `findCrossArtifactReferences(ctx, ws, oldID, newID)` to collect all
cross-reference updates. This runs outside the transaction, holding no DB lock.

**Inside the transaction** (after the adopted artifact's file write at line 636):
Call `applyCrossArtifactRewrites(ctx, ws, tx, updates)` to apply the file
writes and DB upserts. This runs within the existing short transaction window.

**On transaction commit failure** (existing rollback at lines 652-658):
Extend the rollback to restore cross-artifact file snapshots.

**Guard**: Only call when `newID != oldID`. The no-ID-change branch (line 661-671)
does not need cross-reference rewrites because the ID hasn't changed (per F-007).

**Extend `AdoptItemResult`:**
Add `RewrittenArtifactIDs []string` with `json:"rewritten_artifact_ids,omitempty"`
(per review F-008, F-010). Add GoDoc comment.

Add rewritten artifact IDs to the `adopted` event delta at line 673.

Update the `backlogit_adopt_item` MCP tool description to note that adoption
may rewrite cross-artifact references automatically (per review F-011).

**Verification:**

* All existing `TestAdoptItem_*` tests pass without modification
* New test: adopt with cross-references, verify `RewrittenArtifactIDs` populated
* Verify MCP tool response includes `rewritten_artifact_ids` when non-empty,
  omits when empty (contract test)

### Unit 3: Comprehensive Test Suite

**Files:** `internal/core/artifact_references_test.go`, `internal/core/shipment_test.go`
**Test files:** (same files)
**Effort size:** medium
**Skill domain:** tests
**Execution note:** test-first
**Patterns to follow:** `TestShipment_RehydrationConsistency` (shipment_test.go:562)
for rehydration test pattern, table-driven tests per Go conventions
**Dependencies:** Unit 1, Unit 2

**Approach:**

**In `artifact_references_test.go` — Unit-level tests for the scanner/rewriter:**

Table-driven tests covering each rewritable field independently:

| Test Case | Setup | Assert |
|---|---|---|
| parent_id rewrite | B.parent_id = A; adopt A → A' | B.parent_id = A' |
| dependencies rewrite | C.dependencies = [A]; adopt A → A' | C.dependencies = [A'] |
| links rewrite | D.links = [{target: A}]; adopt A → A' | D.links = [{target: A'}] |
| no-op (no references) | E has no refs to A; adopt A → A' | E untouched, not in results |
| self-exclusion | adopted artifact excluded from scan | not in results |
| exact-match only | F.parent_id = "001-T"; rewrite "01-T" | F.parent_id unchanged |
| multiple references | G.dependencies = [A, X, A] | G.dependencies = [A', X, A'] |
| rollback on write failure | inject write error mid-apply | all prior files restored |
| dep/link DB rows refreshed | after apply, query item_deps | correct rows present |

**In `shipment_test.go` — Integration tests:**

`TestAdoptItem_CrossReferenceRehydrationConsistency`:
1. Set up workspace with feature F, tasks T1 and T2 under F, where T2 has
   `dependencies: [T1.ID]`
2. Create a new feature G
3. Adopt T1 under G (T1 gets a new ID)
4. Assert T2's Markdown file now references T1's new ID in `dependencies`
5. Run `Rehydrate` to rebuild the DB from Markdown
6. Query `item_deps` for T2 — assert it lists T1's new ID, not the old one
7. Query `items` for T2 — assert `dependencies` field contains new ID

`TestAdoptItem_CrossReference_NoIDChange`:
1. Set up workspace where adoption doesn't change ID (no config/queue layout)
2. Verify no cross-reference scan occurs

`TestAdoptItem_CrossReference_ResultPopulated`:
1. Adopt with cross-references present
2. Verify `RewrittenArtifactIDs` in result contains correct IDs

**Verification:**

* All tests pass with `go test -race ./internal/core/...`
* No races detected
* Coverage of cross-reference rewrite paths ≥ 90%

## Dependency Graph

```text
Unit 1 (scanner/rewriter) ← Unit 2 (AdoptItem integration) ← Unit 3 (comprehensive tests)
```

Units are sequential: Unit 2 cannot integrate what Unit 1 hasn't built, and
Unit 3 validates the full stack including integration paths.

## Decisions

| # | Decision | Rationale | Alternatives Rejected |
|---|---|---|---|
| D1 | Filesystem scan (walk `.md` files) rather than DB query for finding references | Markdown is the source of truth. A DB query would miss references in artifacts not yet indexed. Walking the filesystem is authoritative. | DB `SELECT WHERE parent_id = ? OR dependencies LIKE ?` — fast but not authoritative; could miss unindexed artifacts |
| D2 | New file `artifact_references.go` rather than extending `shipment_lifecycle.go` | Cross-reference rewriting is a workspace-level concern, not shipment-specific. Separate file improves cohesion and testability (per review F-009). | Inline in `shipment_lifecycle.go` — grows an already large file with unrelated responsibility |
| D3 | Two-phase approach: collect outside tx, apply inside short tx | Follows Rehydrate()'s phased design (per review F-006). Collect phase holds no DB lock. Apply phase is short and targeted. | Single-phase scan+write inside tx — holds DB lock during filesystem walk |
| D4 | Defer `custom_fields.items` to follow-up | Not proven to participate in the rehydration corruption path. Introduces `[]any` vs `[]string` JSON round-trip complexity (per review F-004, LR-006). | Include in V1 — increases scope and risk without clear rehydration benefit |
| D5 | Extend `AdoptItemResult` with `RewrittenArtifactIDs` (omitempty) | Gives callers and event logging visibility into the cascade without a separate query. `omitempty` keeps the response clean when no rewrites occur. | Separate function to query rewritten refs — unnecessary indirection |
| D6 | File snapshot/restore rollback for cross-artifact writes | Follows existing AdoptItem rollback pattern (lines 652-658). Ensures all-or-nothing file writes on failure (per review F-001). | No rollback — would leave workspace in inconsistent state on partial failure |

## Risks and Caveats

1. **Performance on large workspaces.** The collect phase scans all `.md`
   files in `.backlogit/`. For workspaces with hundreds of artifacts, this adds
   latency to AdoptItem. Mitigation: the scan is read-only and runs outside the
   transaction, adding no DB lock time. Only artifacts that actually reference
   the old ID are written. For the typical backlogit workspace (dozens of
   artifacts), this is negligible.

2. **Partial file write failure.** If writing a cross-referenced artifact's file
   fails mid-apply, some files will be updated and others won't. Mitigation:
   each file write is atomic (tmp+rename). The apply function tracks which files
   succeeded and restores from snapshots on failure. The DB transaction rolls
   back automatically via `defer tx.Rollback()`.

3. **Compound learning: orphaned tasks.** The learning at
   `docs/compound/workflow-issues/orphaned-tasks-without-parent-features-2026-04-10.md`
   documents that AdoptItem previously didn't rewrite IDs at all. That was fixed.
   This plan addresses the next layer: cross-artifact references. The learning's
   "systemic prevention" section (item 2) explicitly calls for this fix.

4. **`custom_fields.items` deferred.** Shipment manifests store item IDs in this
   field. V1 does not rewrite them. If a shipped shipment's manifest still
   references old IDs, the shipment display may show stale items. This is
   tracked as a follow-up stash item. Rehydration correctness is not affected
   because `custom_fields` is not used to rebuild dep/link DB edges.

5. **Collect-apply race window.** Between the collect phase and the apply phase,
   another process could modify the workspace. Mitigation: AdoptItem is a
   single-agent operation (not called concurrently). The file-lock protocol
   protects against multi-agent scenarios when concurrency is active.

## Plan Hardening Signals (REQUIRED)

* public API, schema, or contract change: **absent** — no exported API changes;
  `AdoptItemResult` gains a field but is not a breaking change
* security, auth, permission, or compliance-sensitive behavior: **absent**
* migration, backfill, destructive data/config action, or irreversible step:
  **absent** — rewrites are corrective, not destructive
* external integration, operator checkpoint, or external dependency: **absent**
* high runtime, rollout, or rollback risk: **absent** — changes are internal
  to a single function with existing rollback patterns

Requires plan hardening: no

## Runtime Verification and Closure

### Changed Runtime Surface

AdoptItem (MCP tool `backlogit_adopt_item` and CLI `backlogit adopt`) gains
cross-reference rewrite behavior. This is a behavioral enhancement to an
existing tool, not a new surface.

### Runtime Verification

After implementation, verify:

1. `backlogit adopt <item> <new-parent>` rewrites cross-references in
   frontmatter (inspect `.md` files after adoption)
2. `backlogit sync` after adoption preserves corrected edges (query
   `item_deps` and `item_links` tables)
3. MCP tool `backlogit_adopt_item` returns `rewritten_artifact_ids` in the response

### Operational Closure

* No monitoring plan needed (single-binary CLI tool, no background service)
* Rollback: revert the commit. AdoptItem without cross-reference rewrite
  returns to pre-fix behavior (stale edges on rehydration, which is the
  current known-broken state)
* No deployment or rollout risk (library code change, not a service)

## Learnings Applied

* **Orphaned tasks compound learning**
  (`docs/compound/workflow-issues/orphaned-tasks-without-parent-features-2026-04-10.md`):
  Section "Systemic prevention" item 2 explicitly calls for AdoptItem to
  "update all references in shipment manifests" and "trigger an index sync."
  This plan implements that recommendation plus broader cross-reference
  coverage (parent_id, dependencies, links).

* **Source artifact archival pattern**
  (`docs/compound/workflow-issues/source-artifact-archival-pattern-2026-04-20.md`):
  Documents `custom_fields.source_stash_id` provenance tracking. Confirms
  that `custom_fields` is used for structured metadata that should be
  inspected during cross-reference rewrites.

* **Rehydration memory** (stored fact): "Rehydration rebuilds item_deps and
  item_links from Markdown frontmatter fields dependencies and links." This
  confirms that fixing Markdown is the only durable fix — DB-only rewrites
  are reverted on rehydration.

## Standards Check

| Standard | Compliance |
|---|---|
| GoDoc on all exported symbols | ✅ `AdoptItemResult.RewrittenArtifactIDs` will have GoDoc |
| `golangci-lint run` zero warnings | ✅ Will verify |
| Test-first development | ✅ Unit 3 written first; Units 1-2 are test-first |
| Atomic file writes (tmp+rename) | ✅ Uses existing `WriteArtifactFile` pattern |
| `path/filepath` for all paths | ✅ Follows existing `artifactSearchDirs` pattern |
| `log/slog` structured logging | ✅ Will log cross-reference rewrites at Info level |
| No `panic()` in library code | ✅ Error returns only |
| Parameterized DB queries | ✅ Uses existing `UpsertItemTx` |
| Workspace containment | ✅ Scans only within `.backlogit/` via `artifactSearchDirs` |

## Plan Review

**Gate Decision: FAIL → Revised → ADVISORY**

Six P1 findings required plan revision. All P1 findings have been addressed
through in-place plan revisions. Remaining P2/P3 findings are advisory and
do not block harvest.

### Reviewers

| Persona | Model | Findings |
|---|---|---|
| Constitution Reviewer | claude-opus-4.6 | 0 |
| Go Quality Reviewer | claude-opus-4.6 | 7 (1→0 P0, 3 P1, 2 P2, 1 P3) |
| Scope Boundary Auditor | claude-opus-4.6 | 5 (2 P1, 2 P2, 1 P3) |
| Learnings Researcher | claude-haiku-4.5 | 8 relevant learnings |
| Architecture Strategist | gpt-5.4 | 6 (1→0 P0, 2 P1, 3 P2) |
| Agent-Native Parity Reviewer | gpt-5.4 | 3 (2 P2, 1 P3) |

### P0 — Critical

None. Original P0s (GQ-002, AS-001) downgraded to P1 after cross-analysis:
both follow existing patterns in AdoptItem that need extending, not inventing.

### P1 — High (addressed in plan revision)

**F-001** (GQ-002 + SB-004): File rollback strategy for cross-artifact writes.
Cross-referenced artifact files are rewritten before the DB transaction commits.
If a later step fails, those files stay mutated while the DB rolls back.
*Resolution*: Plan revised to snapshot cross-artifact files before rewrite and
restore on failure, following the existing AdoptItem rollback pattern (lines
652-658).

**F-002** (AS-001): Partial DB cache update. `UpsertItemTx` only updates the
`items` row; it does not rebuild `item_deps` or `item_links` for cross-referenced
artifacts. After rewrite, their DB dep/link rows would be stale until next
rehydration.
*Resolution*: Plan revised to delete+reinsert dep/link rows for each
cross-referenced artifact within the same transaction, or alternatively trigger
a targeted rehydrate after commit.

**F-003** (GQ-001 + AS-005): Nullable `*sql.Tx` encodes two behaviors in one
signature. Passing `nil` is a brittle mode switch.
*Resolution*: Plan revised to split into two functions: a pure file rewriter
returning affected artifacts, and caller-side DB persistence.

**F-004** (SB-001 + AS-003 + GQ-003): `custom_fields.items` adds scope creep,
shipment-specific coupling, and type-safety risk from `[]any` vs `[]string`.
*Resolution*: Deferred to follow-up. V1 limited to canonical reference fields:
`parent_id`, `dependencies`, `links`.

**F-005** (GQ-006 + SB-003): One integration test is insufficient. Missing
per-field tests, failure paths, no-op cases, substring false-positive tests.
*Resolution*: Plan revised to expand Unit 3 test matrix.

**F-006** (AS-002): Transaction window holds DB write lock during filesystem
walk and parse. Rehydrate() already separates collect from write phases.
*Resolution*: Plan revised to split into collect phase (outside tx) and write
phase (short tx window).

### P2 — Moderate (advisory)

**F-007** (GQ-004): No-ID-change branch should not call rewrite helper. If
`oldID == newID`, there is nothing to rewrite.
*Note*: Addressed by the split-function design — caller only invokes when
`newID != oldID`.

**F-008** (GQ-005 + SB-005): `RewrittenRefs` field name is underspecified.
*Note*: Renamed to `RewrittenArtifactIDs []string` with `json:"rewritten_artifact_ids,omitempty"`.

**F-009** (AS-004): `shipment_lifecycle.go` is not the right long-term home.
*Note*: Accepted. New function placed in `artifact_references.go` within
`internal/core/`.

**F-010** (AP-001): MCP response backward compatibility.
*Note*: `omitempty` tag ensures field is absent when empty. Contract test added
to Unit 3.

**F-011** (AP-002): Agent-visible side-effect expansion needs documentation.
*Note*: Accepted as advisory. Tool description update added to Unit 2.

### P3 — Low (advisory)

**F-012** (AP-003): CLI parity needs test coverage.
**F-013** (GQ-007): Error wrapping needs path context.

### Learnings Incorporated

* LR-001 (atomic rehydration transaction pattern): Reinforces F-001/F-006.
  Addressed by the split-phase design.
* LR-005 (batch failure propagation): Reinforces F-001. Errors propagated
  immediately, no log-and-continue.
* LR-006 (CustomFields type normalization): Reinforces F-004. Deferred
  `custom_fields.items` handling avoids the type-safety risk for now.
