---
chunk_strategy: h1-h2-h3
description: Documents the gap in backlogit's stash command surface that forced an agent to use custom PowerShell scripting to identify and remove stale stash entries, and prescribes the metadata and tooling additions needed to make staleness management a native capability.
doc_type: learning
docline:
    category: workflow_issue
    component: stash
    date: 2026-04-09T00:00:00Z
    file_path: internal/core/stash.go
    message: No native backlogit command or metadata supports stash staleness detection or removal. Agents must resort to custom scripting.
    problem_type: workflow_issue
    resolution_type: documentation
    resolved: true
    root_cause: missing_tooling
    severity: high
    tags:
        - stash
        - staleness
        - grooming
        - agent-tooling
        - command-surface
        - metadata-gap
        - custom-scripting
ingested_at: "2026-06-26T02:32:58Z"
schema_version: "1.0"
source: docs/compound/workflow-issues/stash-staleness-requires-custom-scripting-2026-04-09.md
title: Stash Staleness Detection and Removal Requires Custom Scripting
---

## Problem

When reviewing a backlogit stash containing 23 entries, 13 had been superseded
by features already shipped and archived (015-F through 019-F). Identifying
those stale entries and removing them required custom PowerShell scripting
because backlogit provides no native tooling for either operation.

The stash command surface today offers three operations: `add`, `fetch-stash`
(list), and `harvest` (promote to artifact). None of these address the
inevitable lifecycle question: what happens to stash entries that are no longer
relevant?

## Symptoms

> [!NOTE]
> Items 1-4 below are resolved as of shipment 014-S. The stash command
> surface now exposes `created_at`, `stash remove`, `stash edit`, and `age_days`
> as native operations. Item 5 (agent guidance) is resolved by the Stage agent
> hygiene protocol added in the same shipment.

* ~~No way to distinguish a fresh idea from a year-old entry (no `created_at`)~~
  **Resolved:** `stash.Entry.CreatedAt` field added.
* ~~No `stash remove` or `stash delete` command in CLI or MCP~~ **Resolved:**
  `backlogit_stash_remove` MCP tool and `backlogit stash remove` CLI exposed.
* ~~`removeStashEntry` exists but is private~~ **Resolved:** wrapped into
  `RemoveStashEntry` (public) wired to CLI and MCP surfaces.
* ~~`backlogit_fetch_stash` provides no staleness indicators~~ **Resolved:**
  `age_days` is now computed from `CreatedAt` and included in `StashEntryView`.
* ~~Agent instructions contain no guidance for stash pruning~~ **Resolved:**
  Stage agent now includes Step 0 (Stash Hygiene) using `age_days`,
  `backlogit_stash_remove`, and `backlogit_stash_edit`.
* Stash accumulates entries that have already been addressed by shipped features
* No way to cross-reference stash text against shipped feature scopes
* Agents fall back to raw file manipulation (PowerShell filtering of
  `stash.jsonl`) instead of using structured backlogit operations

## What Did Not Work

The agent attempted to use backlogit's existing tool surface to clean stale
stash entries. The available options were:

1. **`backlogit_harvest_stash`**: Promotes a stash entry to a backlog artifact.
   This is semantically wrong for entries that should be discarded, not
   promoted. Harvesting a stale entry would create a backlog artifact for work
   that has already been completed.

2. **`backlogit_fetch_stash`**: Lists entries with optional priority filter.
   Useful for viewing but provides no mutation capability and no staleness
   indicators.

3. **`backlogit_stash`**: Adds new entries. No edit, update, or delete path.

With no delete operation available, the agent wrote a PowerShell one-liner to
filter `stash.jsonl` directly:

```powershell
$keep = @('C50CB316','174A4EB9','834CCDB7','078E58F2','8699071E',
          '2CDA43BF','60EF697D','93A77D46','F51BAEC0','4A87BF86')
$lines = Get-Content .backlogit\stash.jsonl | Where-Object {
    $id = ($_ | ConvertFrom-Json).id
    $keep -contains $id
}
$lines | Set-Content .backlogit\stash.jsonl -Encoding utf8NoBOM
```

This bypasses the stash lock mechanism (`internal/core/stash_lock.go`), the
index synchronization pipeline, and the event logging system.

## Solution: Required Additions

### 1. Add `created_at` to the `stash.Entry` struct ✅ Implemented

The `Entry` struct in `internal/stash/stash.go` now includes a `CreatedAt`
field populated by `AddStashEntry`. Existing entries without `created_at`
default to `nil` and are treated as unknown age rather than new.

### 2. Expose `stash remove` as a CLI command and MCP tool ✅ Implemented

`RemoveStashEntry` is public and wired to both the CLI (`backlogit stash remove`)
and MCP (`backlogit_stash_remove`). Removals update the DB index and support an
optional reason string for auditability.

### 3. Add `stash edit` for metadata corrections ✅ Implemented

`EditStashEntry` is public and exposed via `backlogit stash edit` (CLI) and
`backlogit_stash_edit` (MCP). Supports updating priority, kind, and text.

### 4. Add staleness indicators to fetch output ✅ Implemented

`StashEntryView` now includes `age_days` computed from `CreatedAt`. The field
is `nil` for entries predating `created_at` support. `backlogit_fetch_stash`
includes `age_days` in every response.

### 5. Add durable archival for removed and harvested entries ✅ Implemented

`appendToStashArchive()` appends an `ArchivedStashEntry` record to
`.backlogit/archive/stash.jsonl` whenever a stash entry is removed or harvested.
Archive entries preserve the original fields plus removal metadata:
`removed_at`, `removal_reason`, and (for harvested entries) `harvested_artifact_id`.

### 6. Update agent instructions for stash lifecycle ✅ Implemented

The Stage agent now includes **Step 0: Stash Hygiene** that runs before triage:

* Reviews entries with `age_days >= 30` for operator confirmation
* Calls `backlogit_stash_remove` for confirmed-stale entries with a reason
* Calls `backlogit_stash_edit` for entries with stale metadata (wrong priority or kind)
* Broadcasts each removal via agent-intercom
* Escalates entries with unknown age to the operator

## Why This Works

Stash entries are transient ideas that may or may not graduate to backlog
artifacts. The existing command surface assumed all entries either get harvested
(promoted) or remain forever. In practice, most stash entries in a healthy
project become stale as features ship and priorities shift.

Native removal with event logging preserves the full lifecycle (created,
considered, removed with reason) without forcing agents into raw file
manipulation that bypasses safety mechanisms.

The durable archive at `.backlogit/archive/stash.jsonl` provides a complete
history of every stash entry that has been removed or harvested, including the
reason and any linked artifact ID.

## Prevention

* When adding a new intake surface (stash, queue, inbox), always design the
  full CRUD lifecycle from the start: create, read, update, delete
* Transient data stores need explicit removal paths; "harvest or keep forever"
  is not a complete lifecycle
* Agent instructions should address data hygiene, not just forward-flow
  operations
* If an agent must use custom scripting to perform a common operation, treat
  that as a tooling gap, not a successful workaround

## Related Solutions

* `best-practices/advisory-file-lock-stale-ttl-go-2026-04-08.md` covers the
  stash lock mechanism that custom scripting bypasses
* `go-patterns/f015-shipment-stash-patterns.md` documents JSONL append patterns
* Stash archive implemented in `internal/core/stash.go` (`appendToStashArchive`,
  `ArchivedStashEntry`) as part of shipment 014-S

