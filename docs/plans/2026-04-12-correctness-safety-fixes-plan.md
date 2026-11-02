---
chunk_strategy: h1-h2-h3
description: ""
doc_type: plan
docline:
    date: 2026-04-12T00:00:00Z
    origin: Stash entries 042C1812, 6D175713, 3F71C20D, D6E4B181, 53EC8F4C
    status: draft
ingested_at: "2026-06-26T02:33:21Z"
schema_version: "1.0"
source: docs/exec-plans/2026-04-12-correctness-safety-fixes-plan.md
title: Correctness & Safety Fixes
---

## Overview

Five correctness and safety fixes that harden existing backlogit surfaces. These
items are independent bug fixes and containment improvements with no
cross-dependencies, making them suitable for parallel implementation within a
single shipment.

Source stash entries:

* `042C1812` (high) — Constrain export_command_map path to `.backlogit/`
* `6D175713` (high) — Fix adopt_item atomic ID rewrite and file rename
* `3F71C20D` (medium) — Tighten query-gate semicolons and section-write error handling
* `D6E4B181` (medium) — Fix stash harvest TOCTOU and status blocking invariants
* `53EC8F4C` (medium) — Fix instructions referencing index.db instead of backlogit.db

## Constitution Check

| Principle | Compliance | Notes |
|---|---|---|
| I. Type-Safe Go | Compliant | All changes are in typed Go code with GoDoc |
| II. MCP Protocol Fidelity | Compliant | No tool visibility changes |
| III. Test-First Development | Compliant | Tests written before implementation for each unit |
| IV. Workspace Containment | Directly addressed | Units 1 and 2 strengthen containment |
| V. Structured Observability | Compliant | Existing slog patterns maintained |
| VI. Single-Binary Simplicity | Compliant | No new dependencies |
| VII. CQRS Data Architecture | Compliant | Markdown-first writes maintained |
| VIII. Git-Friendly Persistence | Compliant | Atomic temp-file-then-rename pattern used |
| IX. Agent Context Efficiency | Compliant | No tool response changes |

## Unit 1: Constrain export_command_map Path (042C1812)

### Problem

`WriteCommandMap` in `internal/core/metadata_catalog.go:305` resolves paths via
`SafeResolve(workspaceRoot, targetPath)` where `workspaceRoot` is the repo root
(passed as `s.RootPath` from `internal/mcp/metadata.go:82`). This allows writing
anywhere under the repo root, bypassing the `.backlogit/` write-only containment
boundary. The existing test `TestWriteCommandMap_WritesInsideWorkspace` only
verifies the file stays within the repo root, not within `.backlogit/`.

### Chosen Direction

Constrain `WriteCommandMap` to resolve paths within the `.backlogit/` workspace
storage root rather than the repo root. The MCP handler should pass the
`.backlogit/` directory path instead of `s.RootPath`. Update the existing test to
verify `.backlogit/` containment and add a test that rejects paths escaping
`.backlogit/` but still within the repo root (e.g., `.github/instructions/file.md`).

### Implementation

**Files changed:**

* `internal/mcp/metadata.go` — Change `s.RootPath` to `s.backlogitDir()` in
  the `handleExportCommandMap` handler (line 82)
* `internal/core/metadata_catalog_test.go` — Update
  `TestWriteCommandMap_WritesInsideWorkspace` to use a `.backlogit/` subdirectory
  as the root; add test case for paths that stay in repo root but escape
  `.backlogit/`

**Test plan:**

* `TestWriteCommandMap_WritesInsideBacklogit` — Verify output resolves under
  `.backlogit/`
* `TestWriteCommandMap_RejectsEscapingBacklogit` — Pass a repo-relative path
  like `../instructions/map.md` that would land in `.github/` and verify
  rejection
* Existing `TestWriteCommandMap_RejectsEscapingWorkspace` stays as-is

### Risk

Low. Single call-site change. Existing SafeResolve handles the boundary check.

## Unit 2: Atomic Adopt Item with ID Rewrite (6D175713)

### Problem

`AdoptItem` in `internal/core/shipment_lifecycle.go:461` only updates
`parent_id` and records `origin_feature` in custom fields. It does not:

* Rewrite the hierarchical ID (e.g., `012-T` should become `025.012-T` when
  adopted under `025-F`)
* Rename the Markdown file to match the new ID
* Rename the JSONL log file under `.backlogit/logs/`
* Update the SQLite index with the new ID
* Update references in shipment manifests or dependency edges

The compound learning in
`docs/compound/workflow-issues/orphaned-tasks-without-parent-features-2026-04-10.md`
documents this gap and the resulting inconsistencies.

### Chosen Direction

Extend `AdoptItem` to perform an atomic adoption that includes ID rewrite, file
rename, log rename, dependency edge update, and index sync. The operation should:

1. Generate the new hierarchical ID using the existing `NextID` mechanism
2. Rename the Markdown file to the new ID-based filename
3. Rename the log file if it exists
4. Update frontmatter `id` field in the Markdown file
5. Update all `item_deps` rows referencing the old ID (both `item_id` and
   `depends_on` columns)
6. Update `item_links` rows referencing the old ID
7. Delete the old index row and upsert the new one
8. Record the rename in the event log with both old and new IDs

**Execution ordering (reviewed):** Execute in this order to allow clean
rollback:

1. Validate inputs and generate the new hierarchical ID
2. Begin a DB transaction — rewrite all dep edges, link edges, delete old index
   row, upsert new index row
3. If DB transaction succeeds, perform file operations (rename `.md`, rename
   `.jsonl`, update frontmatter in-place)
4. If any file operation fails, roll back the DB transaction and restore any
   partially renamed files
5. Commit the DB transaction only after all file operations succeed

This ordering ensures the reversible operation (DB tx) happens before the
harder-to-reverse operations (file renames), and the return value includes the
new ID so callers can update their own references.

**Blast radius note:** Adoption rewrites internal backlogit references only
(dependency edges, link edges, index). External references in `docs/memory/`,
`.copilot-tracking/`, or agent conversation context are the caller's
responsibility.

### Implementation

**Files changed:**

* `internal/core/shipment_lifecycle.go` — Rewrite `AdoptItem` to include ID
  generation, file rename, log rename, frontmatter update, dep/link edge
  rewrite, and index sync
* `internal/core/naming.go` — May need to expose `NextID` or a helper that
  generates the next child ID under a given parent
* `internal/db/queries.go` — Add `UpdateDependencyItemID` and
  `UpdateLinkItemID` functions for edge rewriting
* `internal/core/shipment_test.go` — Expand existing adopt tests to verify ID
  rewrite, file rename, log rename, dependency edge updates, and index
  consistency

**Test plan:**

* `TestAdoptItem_RewritesID` — Adopt a root task under a feature; verify new
  hierarchical ID format
* `TestAdoptItem_RenamesFiles` — Verify old `.md` and `.jsonl` files removed,
  new files created with correct names
* `TestAdoptItem_UpdatesDependencyEdges` — Create deps referencing old ID;
  verify they point to new ID after adoption
* `TestAdoptItem_UpdatesLinkEdges` — Same for `item_links`
* `TestAdoptItem_RollsBackOnFailure` — Simulate write failure; verify original
  state preserved
* Existing tests `TestAdoptItem_RejectsArchivedItem` and
  `TestAdoptItem_RejectsMissingParent` stay as-is

### Risk

Moderate. This changes the semantics of an existing MCP tool. Callers that
depend on ID stability after adoption need to be aware. The compound learning
already documents the expectation. Atomic rollback on failure mitigates
corruption risk.

## Unit 3: Query Gate Semicolon Fix and Section-Write Error Handling (3F71C20D)

### Problem A: Query Gate Semicolons

The SQL gate in `internal/db/gate.go:15` uses the pattern
`(?m);.*\S+.*$` to reject semicolons followed by non-whitespace. This rejects
valid queries containing semicolons inside string literals, such as:

```sql
SELECT * FROM items WHERE description LIKE '%key;value%'
```

The intent is to prevent multi-statement injection (e.g., `SELECT 1; DROP TABLE
items`), but the regex operates on the raw query without respecting string
boundaries.

### Problem B: Section-Write Error Handling

`writeSectionsToFile` in `internal/mcp/tools.go:822` passes all section
updates to `parser.WriteSections` as a batch. If any section is missing (returns
error), the fallback on lines 843-849 treats ALL sections as missing and appends
ALL of them, even sections that already existed. This can duplicate existing
sections on partial match failures.

### Chosen Direction

**Gate fix:** Replace the raw semicolon regex with a string-literal-aware check.
Strip single-quoted string literals from the query before applying the semicolon
pattern. This preserves the multi-statement injection protection while allowing
semicolons inside SQL string values.

**String-literal stripping rules:**
1. Replace paired escaped quotes (`''`) with a placeholder first
2. Replace content between remaining `'...'` pairs with empty strings
3. If an unterminated string literal remains (odd number of `'`), reject the
   query conservatively rather than stripping incorrectly

**Section fix:** Change `writeSectionsToFile` to process sections individually.
For each section, attempt `WriteSections` with that single section. If it fails
with "not found", append the section markers. If it fails with any other error,
propagate the error. This prevents duplication of existing sections.

### Implementation

**Files changed:**

* `internal/db/gate.go` — Add a `stripStringLiterals` helper that removes
  single-quoted string content before applying `forbiddenPatterns`. Update
  `ValidateQuery` to use it.
* `internal/db/gate_test.go` — Add test cases for queries with semicolons in
  string literals (should pass) and multi-statement queries (should still fail)
* `internal/mcp/tools.go` — Rewrite `writeSectionsToFile` to process sections
  one at a time, distinguishing "not found" from structural errors
* `internal/mcp/tools_test.go` or equivalent — Add test for section writing
  with mixed existing/new sections

**Test plan:**

* `TestValidateQuery_AllowsSemicolonInStringLiteral` — Verify
  `SELECT * FROM items WHERE x LIKE '%a;b%'` passes the gate
* `TestValidateQuery_RejectsMultiStatement` — Verify
  `SELECT 1; DROP TABLE items` still fails
* `TestValidateQuery_RejectsTrailingSemicolon` — Verify trailing semicolons with
  content after them still fail
* `TestWriteSectionsToFile_MixedExistingAndNew` — Verify that when one section
  exists and another doesn't, only the missing one is appended
* `TestWriteSectionsToFile_PropagatesStructuralError` — Verify non-"not found"
  errors propagate instead of triggering fallback append

### Risk

Low to moderate. The gate change must not weaken injection protection. The
string-stripping approach is conservative: it removes content that cannot
contain SQL keywords, leaving the structural query intact for pattern matching.

## Unit 4: Stash Harvest TOCTOU and Terminal Status Gaps (D6E4B181)

### Problem A: Harvest TOCTOU

`HarvestStashByPriority` in `internal/core/stash.go:343` calls `FetchStash` to
read the batch, then iterates and calls `HarvestStashEntry` for each entry.
Between the fetch and each harvest, another process could modify the stash file
(add, remove, or edit entries), causing spurious failures or harvesting stale
data.

### Problem B: Terminal Status Gap

`TerminalStatuses` in `internal/core/blocking_cascade.go:14` includes `done`,
`accepted`, `archived`, `shipped`, `abandoned` but NOT `rejected`. This means
rejecting a parent artifact does not trigger the child-blocking guard that
prevents children from being worked on when their parent is terminal.

Additionally, `filterByResolvedDependencies` in `internal/core/queue.go:249`
uses a hardcoded subset (`done`, `accepted`) instead of the canonical
`TerminalStatuses` list, creating inconsistency.

### Chosen Direction

**TOCTOU fix:** Extract a `harvestStashEntryLocked` internal variant of
`HarvestStashEntry` that assumes the caller already holds the stash lock. Then
in `HarvestStashByPriority`, acquire the lock once before `FetchStash`, call
`harvestStashEntryLocked` in the loop, and release after the loop completes.
The public `HarvestStashEntry` continues to acquire and release the lock for
single-entry callers. This avoids deadlock on the non-reentrant `sync.Mutex` in
`stash_lock.go` while closing the TOCTOU window.

**Terminal status unification rationale:** `queue.go` currently uses only
`done` and `accepted` when deciding if a blocking dependency is resolved. This is
too narrow — if a dependency is `archived`, `shipped`, `abandoned`, or
`rejected`, the dependent item should be visible in the queue. Unifying to
`TerminalStatuses` provides a single source of truth for "is this item finished?"
across both the blocking cascade and queue visibility.

### Implementation

**Files changed:**

* `internal/core/stash.go` — Extract `harvestStashEntryLocked` (unexported) from
  `HarvestStashEntry` that skips lock acquisition. `HarvestStashEntry` calls
  `harvestStashEntryLocked` inside its own lock scope. In
  `HarvestStashByPriority`, acquire stash lock before `FetchStash`, call
  `harvestStashEntryLocked` in the loop, defer unlock after the loop completes.
* `internal/core/blocking_cascade.go` — Add `"rejected"` to `TerminalStatuses`
* `internal/core/queue.go` — Replace hardcoded `terminalStatuses` map in
  `filterByResolvedDependencies` (lines 249-252) with a set built from
  `TerminalStatuses`
* `internal/core/blocking_cascade_test.go` — Add test verifying `rejected`
  parent blocks children
* `internal/core/queue_test.go` — Add test verifying `rejected` dependency
  satisfies the queue filter

**Test plan:**

* `TestHarvestStashByPriority_HoldsLockDuringBatch` — Verify lock is held
  during the entire batch operation (can test by checking lock file existence
  during harvest)
* `TestTerminalStatuses_IncludesRejected` — Verify `rejected` is in the list
* `TestBlockingCascade_RejectedParent` — Reject a parent; verify children
  are blocked
* `TestQueueFilter_RejectedDependency` — Mark a blocking dependency as
  `rejected`; verify the dependent item appears in the queue

### Risk

Low. Adding `rejected` to terminal statuses is a semantic correction. The lock
scope change matches the existing locking pattern.

## Unit 5: Fix Stale index.db References in Instructions (53EC8F4C)

### Problem

Three instructions files reference `index.db` instead of `backlogit.db`:

* `.github/instructions/constitution.instructions.md` (6 references)
* `.github/instructions/backlogit-sql-schema.instructions.md` (1 reference)
* `.github/instructions/go-mcp-server.instructions.md` (3 references)

These are teaching and reference artifacts that guide agents. Stale references
cause agents to look for the wrong database file.

### Chosen Direction

Find-and-replace `index.db` with `backlogit.db` in all three files. Verify each
replacement is contextually correct (some may refer to a generic concept rather
than the backlogit database file).

### Implementation

**Files changed:**

* `.github/instructions/constitution.instructions.md` — Replace 6 occurrences
* `.github/instructions/backlogit-sql-schema.instructions.md` — Replace 1
  occurrence
* `.github/instructions/go-mcp-server.instructions.md` — Replace 3 occurrences

**Test plan:**

* `grep -r "index\.db" .github/instructions/` returns zero results after the
  change
* Also search `.github/agents/` and `.github/skills/` for stale `index.db`
  references (widen scope from instructions-only)
* No code changes, so no Go tests needed

### Risk

Minimal. Pure documentation fix.

## Dependency Graph

```text
Unit 1 (export_command_map containment) — independent
Unit 2 (adopt_item atomic rewrite)      — independent
Unit 3 (gate + sections)                — independent
Unit 4 (harvest TOCTOU + terminal)      — independent
Unit 5 (instructions fix)               — independent
```

All five units are independent and can be implemented in parallel.

## Verification Strategy

After all units are implemented:

1. `go test ./...` — All tests pass including new tests
2. `go vet ./...` — No findings
3. `golangci-lint run` — Zero warnings
4. `gofmt -l .` — No unformatted files
5. `grep -r "index\.db" .github/instructions/` — Zero results

## Shipment Assembly Guidance

All five units belong in a single shipment titled "Correctness & Safety Fixes".
No subtask decomposition is needed; each unit maps to one task under one parent
feature. The recommended execution order matches the unit numbering, but any
order is valid since there are no cross-dependencies.

Requires plan hardening: no

## Plan Review

**Gate Decision: ADVISORY**

Reviewed by 5 personas (constitution-reviewer, go-quality-reviewer,
scope-boundary-auditor, learnings-researcher, architecture-strategist).

Plan hardening was not required and this was confirmed by the review.

### P0: Critical (addressed in plan revision)

1. **Unit 4 Deadlock (go-quality, architecture):** `HarvestStashEntry` acquires
   the stash lock via `lockStashFile()` which calls `stashMu.Lock()`. Go's
   `sync.Mutex` is non-reentrant. The original plan proposed holding the lock in
   `HarvestStashByPriority` then calling `HarvestStashEntry`, which would
   deadlock. **Fixed:** Plan revised to extract `harvestStashEntryLocked`
   internal variant.

### P1: High (addressed in plan revision)

2. **Unit 2 Scope Asymmetry (scope-boundary, architecture):** Unit 2 is a
   feature-scale change (atomic ID rewrite, file rename, edge rewrite, rollback)
   bundled with 4 small bug fixes. Risk profile mismatch with the rest of the
   shipment. **Accepted:** Unit 2 is a correctness fix for an existing tool, not
   new feature work. It remains in the shipment but is clearly marked as the
   critical-path item.

3. **Unit 2 Rollback Ordering (go-quality):** Original plan did not specify
   ordering between DB transaction and file operations. If file rename succeeded
   but DB failed, rollback would be complex. **Fixed:** Plan revised with
   explicit ordering: DB tx first (reversible), file ops second, commit tx last.

### P2: Moderate (advisory)

4. **Unit 3 Escaped Quotes (go-quality):** `stripStringLiterals` must handle
   SQL escaped single quotes (`''`) and unterminated strings. **Fixed:** Plan
   revised with explicit string-literal stripping rules.

5. **Unit 4 Queue Filter Semantics (architecture):** `queue.go` uses narrower
   terminal set than `blocking_cascade.go`. Could be intentional. **Resolved:**
   Analysis shows the narrow set is a bug — archived, shipped, abandoned, and
   rejected blockers should all unblock dependents. Plan updated with rationale.

6. **Unit 1 MCP Tool Description (constitution):** Tool description says
   "workspace" but change constrains to `.backlogit/`. Update tool description to
   match new behavior. **Noted for implementation.**

7. **Unit 2 Return Value (architecture):** `AdoptItemResult` should include the
   new ID so callers can update their references. **Noted for implementation.**

8. **Unit 2 Blast Radius (architecture):** External references to old ID in
   `docs/memory/`, `.copilot-tracking/`, agent context. **Fixed:** Plan revised
   to document scope boundary.

### P3: Low (advisory)

9. **Unit 5 Search Scope (learnings):** Plan should also search `.agent.md` and
   `SKILL.md` files for stale `index.db` references. **Fixed:** Search scope
   widened in plan.

### Reviewer Attribution

| Finding | Reviewer | Model |
|---|---|---|
| 1. Deadlock | go-quality-reviewer, architecture-strategist | claude-sonnet-4 |
| 2. Scope asymmetry | scope-boundary-auditor, architecture-strategist | claude-sonnet-4 |
| 3. Rollback ordering | go-quality-reviewer | claude-sonnet-4 |
| 4. Escaped quotes | go-quality-reviewer | claude-sonnet-4 |
| 5. Queue semantics | architecture-strategist | claude-sonnet-4 |
| 6. Tool description | constitution-reviewer | claude-sonnet-4 |
| 7. Return value | architecture-strategist | claude-sonnet-4 |
| 8. Blast radius | architecture-strategist | claude-sonnet-4 |
| 9. Search scope | learnings-researcher | claude-sonnet-4 |

### Next Steps

All P0 and P1 findings addressed in plan revision. P2 findings noted for
implementation. Proceed to `harvest` to decompose the plan into backlogit work
items.
