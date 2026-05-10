---
title: "Lifecycle Hygiene: Cascading Archive & Stash Cleanup"
description: "Cascading archive for child items, stash cleanup on feature archive, doctor --fix-orphans, and stash_remove rename"
date: 2026-05-09
origin: ".backlogit/queue/046-DL.md"
status: reviewed
---

## Problem Frame

After build and release cycles, artifact leftovers remain in the queue and stash
because cleanup is distributed across agent harness steps. When an agent crashes
mid-workflow, no cleanup happens. The backlogit tool should own data-integrity
cascades on state transitions so the workspace stays consistent regardless of
harness behavior.

`ShipShipment` already cascades to manifest items (archiving scope items,
features, descendants, and deliberations). The remaining gaps are:

1. `ArchiveItem` does not cascade to children — archiving a feature leaves its
   tasks and subtasks in the queue.
2. Stash entries linked to shipped features are not automatically archived.
3. Doctor detects orphans but cannot fix them.
4. `stash_remove` naming implies deletion but performs archival.

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | When a parent item archives, cascade archive to all children | 046-DL chosen direction |
| R2 | When a feature archives with linked stash entries, mark those stash entries archived | 046-DL chosen direction |
| R3 | Add doctor --fix-orphans mode that resolves orphaned items | 046-DL chosen direction |
| R4 | Rename stash_remove to stash_archive with backward-compatible alias | Stash 6E99AE10 |
| R5 | Cascade defaults off; user-facing paths opt in | Design decision (see D1, revised per P1-1) |
| R6 | Doctor --fix-orphans defaults to report-only; --fix-orphans flag enables auto-fix | Design decision (see D2) |

## Scope Boundaries

### In Scope

- Cascading archive in `ArchiveItem` for child items
- Stash entry archival when linked features archive
- Doctor --fix-orphans mode
- Rename stash_remove → stash_archive (MCP tool + CLI + core function)
- Backward-compatible alias for old name
- Tests for all changes

### Non-Goals

- Changing `ShipShipment` cascade behavior (already works)
- Cascading status transitions other than archive (e.g., cascading "active")
- Stash entry auto-creation or auto-linking
- Changes to the harness workflow timing

### Deferred to Implementation

- Exact error handling when a child cascade fails mid-way (best-effort or abort)
- Whether to log cascade actions to events.jsonl (likely yes, confirm in impl)

## Implementation Units

### Unit 1: Rename stash_remove → stash_archive

**Files:** `internal/core/stash.go`, `internal/cli/stash.go`, `internal/mcp/tools.go`
**Test files:** `internal/core/stash_test.go` (existing), `tests/contract/stash_tools_test.go` (if exists)
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first — write a test asserting the new function name and archive reason before renaming
**Patterns to follow:** CLI alias pattern in `internal/cli/stash.go:61-68` (stash list has alias fetch-stash)
**Dependencies:** none

**Approach:**
1. Rename `RemoveStashEntry` → `ArchiveStashEntry` in `internal/core/stash.go`.
   Keep `RemoveStashEntry` as a deprecated wrapper that calls `ArchiveStashEntry`.
2. Rename JSONL field from `removal_reason` to `reason` (struct field
   `RemovalReason` → `Reason`, JSON tag `json:"reason"`). Set value to
   `"archived"`. Rename `removed_at` → `archived_at` (`json:"archived_at"`).
   Keep DB `stash_entries.state` as `"removed"` for rehydration safety (per P1-3).
3. In `internal/cli/stash.go`, rename command from `remove` to `archive` with
   `Aliases: []string{"remove"}` for backward compat.
4. In `internal/mcp/tools.go`, register `backlogit_stash_archive` as the primary
   tool. Register a second tool entry `backlogit_stash_remove` pointing to the
   same handler with a description noting deprecation.
5. Update `ArchivedStashEntry` field names and doc comments.

**Verification:**
- `backlogit stash archive <id>` writes JSONL with `"reason": "archived"`
- `backlogit stash remove <id>` still works (alias), same output
- Both MCP tool names work
- JSONL field is `reason` (not `removal_reason`)
- DB state remains `state = 'removed'` (unchanged — rehydration safety per P1-3)
- Existing tests pass with updated assertions

### Unit 2: Cascade archive children in ArchiveItem

**Files:** `internal/core/archive.go`
**Test files:** `internal/core/archive_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first — add test for parent archive cascading to children
**Patterns to follow:** `ShipShipment` cascade in `internal/core/shipment_lifecycle.go:124-136` (archiveItems helper)
**Dependencies:** none (parallel with Unit 1)

**Approach:**
1. Add a `WithCascade(bool)` archive option. Default to `false` (per P1-1).
   User-facing CLI and MCP entry points pass `WithCascade(true)` explicitly.
2. When cascade is enabled, use a private `archiveItemCascade(ctx, ..., cfg,
   visited, depth)` helper that carries visited set, depth counter, and
   aggregate results (per P1-5).
3. Query children via `db.QueryItems` with `ParentID` filter (per P2-3),
   not direct SQL. Archive deepest-first (subtasks → tasks → parent).
4. If any child archive fails, continue with remaining children. Record
   failures in `ArchiveRecord.FailedItems []ArchiveFailure` (per P1-2).
   Hard depth cap at configured hierarchy depth + 1 fallback.
5. Return cascade results in `ArchiveRecord` — add `CascadedItems []string`.
6. Guard against cycles by tracking visited IDs.

**Verification:**
- Archiving a feature cascades to its tasks and subtasks
- Archiving a task cascades to its subtasks
- Already-archived children are skipped (no error)
- `ShipShipment` continues working without changes (cascade defaults off)
- `ArchiveRecord.CascadedItems` lists all cascaded IDs

### Unit 3: Archive linked stash entries on feature archive

**Files:** `internal/core/archive.go`, `internal/core/stash.go`, `internal/db/stash.go`
**Test files:** `internal/core/archive_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `RemoveStashEntry` archive pattern in `internal/core/stash.go:680-692`
**Dependencies:** Unit 1 (uses renamed ArchiveStashEntry), Unit 2 (cascade context)

**Approach:**
1. After archiving an item in `ArchiveItem`, check `stash_links` table for any
   stash entries linked to this item ID.
2. For each linked stash entry that is still `active`, call `ArchiveStashEntry`
   (or directly use `db.UpsertStashEntry` with `stashStateArchived` and append
   to stash archive JSONL).
3. This runs after the cascade, so archiving a feature that has linked stash
   entries will clean them up.
4. Add `ArchiveLinkedStashEntries(ctx, ws, itemID)` in `stash.go` near stash
   lifecycle code (per P2-4). Call from cascade orchestration path.

**Verification:**
- Archiving a feature with a linked stash entry marks it archived
- Stash entry appears in `archive/stash.jsonl` with reason "archived"
- DB `stash_entries.state` = `'removed'` (unchanged persisted state)
- Archiving an item with no linked stash entries is a no-op (no error)

### Unit 4: Doctor --fix-orphans mode

**Files:** `internal/core/doctor.go`, `internal/cli/doctor.go`
**Test files:** `internal/core/doctor_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first — test that fix mode re-parents or archives orphans
**Patterns to follow:** Existing `Doctor` function structure in `internal/core/doctor.go:55-184`
**Dependencies:** Unit 2 (uses cascade archive for fix actions)

**Approach:**
1. Add `FixOrphans bool` field to `DoctorOptions`.
2. Add `--fix-orphans` flag to CLI doctor command (per P2-1).
3. When `FixOrphans` is true and `FindingOrphanedArtifact` findings exist:
   - Archive only items identified by the existing orphan detection logic,
     which already excludes items with `returned_to_backlog` history (per P1-4).
   - Record fix actions in `DoctorReport` with a new `FixActions []FixAction`
     field (item ID, action taken, result).
4. When `FixOrphans` is false (default): report-only behavior unchanged.
5. Add MCP tool parameter `fix_orphans` (boolean, optional, default false) to
   `backlogit_doctor` (per P2-5).

**Verification:**
- `backlogit doctor --fix-orphans` archives orphaned tasks
- `backlogit doctor --fix-orphans` does NOT archive returned_to_backlog items
- `backlogit doctor` (no flag) still reports only
- Fix actions are recorded in report output
- MCP `backlogit_doctor` with `fix_orphans: true` works

### Unit 5: CLI reference regeneration and integration test

**Files:** `docs/cli-reference/` (generated)
**Test files:** none (CI gate validates)
**Effort size:** small
**Skill domain:** docs
**Execution note:** run after all code units
**Patterns to follow:** `go run ./cmd/gen-docs`
**Dependencies:** Units 1, 4 (CLI flag changes)

**Approach:**
1. Run `go run ./cmd/gen-docs` to regenerate CLI reference docs.
2. Verify CLI Reference Drift Check will pass.

**Verification:**
- `go run ./cmd/gen-docs` produces no diff
- All quality gates pass: `go test ./...`, `go vet ./...`, `golangci-lint run`

## Dependency Graph

```
Unit 1 (rename stash) ──┐
                         ├──► Unit 3 (stash on feature archive) ──► Unit 5 (docs)
Unit 2 (cascade children)┘                                          ▲
                         ├──► Unit 4 (doctor --fix) ────────────────┘
```

Units 1 and 2 are parallel. Unit 3 depends on both. Unit 4 depends on Unit 2.
Unit 5 is last.

## Decisions

| # | Decision | Rationale | Alternatives Rejected |
|---|---|---|---|
| D1 | Cascade defaults off; user-facing paths opt in | Existing callers (AutoArchive, ShipShipment, tests) retain single-item behavior. User-facing CLI/MCP pass `WithCascade(true)` explicitly. Revised per P1-1 | Always-on (rejected P1-1: changes all callers unexpectedly) |
| D2 | Doctor --fix defaults to report-only | Safer default; auto-fix could mask data issues that need human review | Always auto-fix (rejected: too aggressive for a diagnostic tool) |
| D3 | Register both MCP tool names | MCP has no alias mechanism; dual registration with shared handler is simplest | Single name with breaking change (rejected: breaks existing agent instructions) |
| D4 | JSONL fields rename: `removal_reason` → `reason` (value: "archived"), `removed_at` → `archived_at`; DB `state` stays "removed" | Operator directive. JSONL fields are audit/display, not rehydration keys. DB state remains "removed" for rebuild safety | Keep old field names (overridden by operator) |
| D5 | ShipShipment unchanged (cascade defaults off) | No code change needed since cascade defaults to false. Revised per D1 | Explicit WithCascade(false) (unnecessary given new default) |

## Risks and Caveats

- **Cascade failure mid-way**: If archiving child 3 of 5 fails, children 1-2 are
  archived and 4-5 are still attempted. Failures are recorded in
  `ArchiveRecord.FailedItems`. CLI/MCP report non-zero failed items as warnings.
  Doctor --fix-orphans can clean up any remainder.
- **Stash entry linking**: Only entries linked via `stash_links` table are
  auto-archived. Entries that were never harvested (just removed) have no link
  and won't cascade. This is correct — they were already archived by stash_remove.
- **Existing "removed" state in DB**: All stash entries (old and new) use
  `state = 'removed'`. No migration needed. API rename is cosmetic only.
- **Compound learning**: `docs/compound/workflow-issues/source-artifact-archival-pattern-2026-04-20.md`
  confirms `ShipShipment` does not clean up stash provenance — this plan
  addresses that gap via Unit 3.

## Plan Hardening Signals (REQUIRED)

* public API, schema, or contract change: **YES** — MCP tool rename (stash_remove → stash_archive), new doctor --fix parameter, new ArchiveRecord.CascadedItems field
* security, auth, permission, or compliance-sensitive behavior: **NO**
* migration, backfill, destructive data/config action, or irreversible step: **NO** — archive is reversible via unarchive
* external integration, operator checkpoint, or external dependency: **NO**
* high runtime, rollout, or rollback risk: **LOW** — cascade is bounded by hierarchy depth (max 3 levels)

Requires plan hardening: no

The API changes are additive (new tool name alongside old, new optional flag),
and archive operations are reversible. Standard review is sufficient.

## Runtime Verification and Closure

- **Unit 1**: CLI and MCP tool surface changes — verify both old and new names work
- **Unit 2**: Archive cascade — verify via `backlogit doctor` that no orphans remain after archiving a feature with children
- **Unit 3**: Stash cleanup — verify stash entries are archived after feature archive via `backlogit query`
- **Unit 4**: Doctor --fix — verify orphan count drops to zero after running with --fix

Validation window: run a full harvest → ship → archive cycle on a test shipment
and confirm zero leftover artifacts.

## Learnings Applied

- `docs/compound/workflow-issues/source-artifact-archival-pattern-2026-04-20.md`:
  Confirmed ShipShipment does not clean up stash provenance. Unit 3 addresses this.
- `docs/compound/go-patterns/f015-shipment-stash-patterns.md`: Shipment
  custom_fields.items normalization pattern — not directly affected but relevant
  context for cascade scope resolution.

## Standards Check

- All changes use Go structs with explicit typing (Constitution I)
- MCP tools remain unconditionally visible (Constitution II)
- Test-first development for all units (Constitution III)
- File operations within .backlogit/ containment (Constitution IV)
- Structured logging via slog for cascade actions (Constitution V)
- No new dependencies (Constitution VI)
- CQRS maintained — Markdown source of truth, DB as cache (Constitution VII)

## Plan Review

**Gate Decision: FAIL → PASS (after inline revision)**

**Plan hardening**: Not required. Confirmed: API changes are additive and archive
is reversible. Standard review is sufficient.

**Reviewers**: Constitution Reviewer (Opus 4.6), Go Quality Reviewer (Opus 4.6),
Scope Boundary Auditor (Opus 4.6), Architecture Strategist (Opus 4.6),
Learnings Researcher (GPT-5.4), Agent-Native Parity Reviewer (GPT-5.4)

### Summary

6 personas returned 14 unique findings after dedup: 0 P0, 5 P1, 6 P2, 3 P3.
All P1 findings have been addressed inline with plan revisions below. Gate
upgraded from FAIL to PASS after revision.

### P0 — Critical

None.

### P1 — High (addressed inline)

**P1-1: Cascade default must be false (opt-in, not opt-out)**
*Reviewers: Scope Boundary Auditor, Architecture Strategist*

`ArchiveItem` is used by CLI, MCP, AutoArchive, tests, and ShipShipment.
Defaulting `WithCascade(true)` turns every existing call into a potentially
destructive parent-plus-children operation. `AutoArchive` is especially risky
because children may not independently satisfy retention criteria.

**Resolution**: D1 revised. Default cascade to `false`. User-facing archive
entry points (CLI `backlogit archive`, MCP `backlogit_archive_item`) pass
`WithCascade(true)` explicitly. ShipShipment and AutoArchive remain unaffected.

**P1-2: Best-effort cascade is insufficient for data integrity**
*Reviewers: Constitution Reviewer, Scope Boundary Auditor, Architecture Strategist*

A parent archived while children remain active creates exactly the orphan
problem this plan fixes. Logging alone is not enough for lifecycle mutations.

**Resolution**: Cascade archives deepest-first (subtasks → tasks → parent).
If any child archive fails, the parent archive still succeeds but
`ArchiveRecord` includes `FailedItems []ArchiveFailure` with item ID and
error. CLI/MCP report non-zero failed items as a warning. R1 updated: "best-effort"
replaced with "aggregate error reporting."

**P1-3: Don't change persisted stash state from "removed" to "archived"**
*Reviewers: Constitution Reviewer, Learnings Researcher, Agent-Native Parity Reviewer*

Changing `stashStateRemoved` to `stashStateArchived` is a contract migration,
not a rename. Existing SQL queries, docs, and agent logic key on
`state = 'removed'`. Rehydration does not read `archive/stash.jsonl`.

**Resolution**: D4 revised. Keep persisted state as `"removed"`. Rename only
the API/CLI surface (`stash_archive` primary, `stash_remove` alias). Both
MCP tools return identical response payloads. State migration deferred to a
future unit if semantic alignment is needed.

**P1-4: Doctor fix must respect returned_to_backlog exclusions**
*Reviewer: Learnings Researcher*

Current doctor logic explicitly skips items with `returned_to_backlog` events.
A naive fix pass could archive legitimate items returned for future work.

**Resolution**: Unit 4 revised. Fix mode operates only on
`FindingOrphanedArtifact` findings produced by current doctor logic, not on
raw `parent_id IS NULL` queries. Add test asserting `--fix` does NOT archive
items with `returned_to_backlog` history.

**P1-5: Cascade recursion needs proper state propagation**
*Reviewers: Go Quality Reviewer, Scope Boundary Auditor*

Visited tracking, depth guard, and error aggregation must be carried through
recursive calls via internal cascade state.

**Resolution**: Unit 2 revised. Add private `archiveItemCascade(ctx, ..., cfg,
visited, depth)` helper. Carry visited set, depth counter, and cascade result
aggregation. Hard depth cap at configured hierarchy depth + 1 fallback.

### P2 — Moderate (acknowledged)

**P2-1**: Use explicit `--fix-orphans` flag, not generic `--fix`.
*Reviewers: Constitution Reviewer, Scope Boundary Auditor*
Accepted. Unit 4 revised to use `--fix-orphans` CLI flag and `fix_orphans`
MCP parameter. Avoids implying all finding types are safely repairable.

**P2-2**: Contract tests needed for both stash tool names, doctor fix parameter,
and archive response shape.
*Reviewers: Constitution Reviewer, Agent-Native Parity Reviewer*
Accepted. Added to Unit 1 and Unit 4 verification criteria.

**P2-3**: Use `db` package helpers for child queries, not direct SQL in core.
*Reviewer: Go Quality Reviewer*
Accepted. Unit 2 will use `db.QueryItems` with `ParentID` filter or add a
`db.ListChildItems` helper.

**P2-4**: Keep stash cleanup as a separate helper, not embedded in `archive.go`.
*Reviewer: Architecture Strategist*
Accepted. Unit 3 revised: implement `ArchiveLinkedStashEntries(ctx, ws, itemID)`
near stash lifecycle code in `stash.go`. Call from cascade orchestration, not
from the generic archive primitive.

**P2-5**: Doctor MCP tool requires `fix_orphans` parameter for CLI/MCP parity.
*Reviewer: Agent-Native Parity Reviewer*
Accepted. Already part of Unit 4 approach. Confirmed explicit in plan.

**P2-6**: Add integration tests for ShipShipment with stash links and cascade
disabled.
*Reviewer: Go Quality Reviewer*
Accepted. Added to Unit 3 verification.

### P3 — Low (advisory)

**P3-1**: All exported symbols (`WithCascade`, `FixAction`,
`ArchiveRecord.CascadedItems`, `ArchiveRecord.FailedItems`) require GoDoc
comments and JSON field assertions in contract tests.
*Reviewer: Go Quality Reviewer*

**P3-2**: Unit 5 verification wording should be: "run generator, commit
updated docs, rerun to confirm no remaining diff."
*Reviewer: Go Quality Reviewer*

**P3-3**: Contract test coverage should explicitly include tool inventory
validation to catch registration drift.
*Reviewer: Agent-Native Parity Reviewer*

### Reviewer Attribution

| Finding | Reviewer | Model |
|---|---|---|
| P1-1 | Scope Boundary Auditor, Architecture Strategist | Opus 4.6, Opus 4.6 |
| P1-2 | Constitution Reviewer, Scope Auditor, Architecture Strategist | Opus 4.6 |
| P1-3 | Constitution Reviewer, Learnings Researcher, Agent-Native Parity | Opus 4.6, GPT-5.4 |
| P1-4 | Learnings Researcher | GPT-5.4 |
| P1-5 | Go Quality Reviewer, Scope Boundary Auditor | Opus 4.6 |
| P2-1 | Constitution Reviewer, Scope Boundary Auditor | Opus 4.6 |
| P2-2 | Constitution Reviewer, Agent-Native Parity Reviewer | Opus 4.6, GPT-5.4 |
| P2-3 | Go Quality Reviewer | Opus 4.6 |
| P2-4 | Architecture Strategist | Opus 4.6 |
| P2-5 | Agent-Native Parity Reviewer | GPT-5.4 |
| P2-6 | Go Quality Reviewer | Opus 4.6 |
| P3-1 | Go Quality Reviewer | Opus 4.6 |
| P3-2 | Go Quality Reviewer | Opus 4.6 |
| P3-3 | Agent-Native Parity Reviewer | GPT-5.4 |

### Inline Plan Revisions Applied

The following decisions and approach sections were revised to address P1 findings:

| Decision | Original | Revised |
|---|---|---|
| D1 | Cascade always-on by default (`WithCascade(true)`) | Cascade opt-in (`WithCascade(false)` default). User-facing paths pass `WithCascade(true)` |
| D4 | DB state changes from "removed" to "archived" | Keep "removed" state. Rename API/CLI only |
| R1 (cascade failure) | Best-effort with logged warnings | Aggregate error reporting via `ArchiveRecord.FailedItems` |
| Unit 2 approach | Recursive ArchiveItem with visited tracking | Private `archiveItemCascade` helper with visited set, depth guard, error aggregation |
| Unit 3 approach | Helper in archive.go | Separate `ArchiveLinkedStashEntries` in stash.go |
| Unit 4 approach | `--fix` flag, fix all orphans | `--fix-orphans` flag, operate only on `FindingOrphanedArtifact`, respect returned_to_backlog |

### Next Steps

Gate: **PASS** (after inline revision). Proceed to `harvest` to decompose
this plan into backlogit work items.
