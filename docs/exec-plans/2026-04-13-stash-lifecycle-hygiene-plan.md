---
title: "Stash Lifecycle & Hygiene"
date: 2026-04-13
origin: "013-DL, 014-DL"
status: approved
---

## Overview

Two complementary improvements to the stash subsystem: durable archival of
removed/harvested entries, and a structured hygiene protocol for the Stage agent.

## Scope

### In scope

* Append removed and harvested stash entries to `.backlogit/archive/stash.jsonl`
* Preserve original fields plus removal metadata (timestamp, reason, harvested
  artifact ID)
* Add stash hygiene step to `stage.agent.md` using native `age_days`,
  `stash remove`, and `stash edit` tools
* Refresh compound doc to reflect current tooling state

### Out of scope

* Querying or searching the stash archive
* Automated purge triggers or scheduled cleanup
* New CLI commands beyond what already exists

## Task Decomposition

### Task 1: Stash archive append function

Add `appendToStashArchive()` to `internal/core/stash.go` that appends an
archived entry record to `.backlogit/archive/stash.jsonl`.

Archive entry schema:

```json
{
  "id": "46CC1C9D",
  "priority": "medium",
  "kind": "feature",
  "text": "original stash text",
  "created_at": "2026-04-10T20:12:47Z",
  "deliberation_id": "013-DL",
  "removed_at": "2026-04-13T02:00:00Z",
  "removal_reason": "harvested",
  "harvested_artifact_id": "030.001-T"
}
```

Implementation notes:

* Define `ArchivedStashEntry` struct extending `stash.Entry` with removal
  metadata fields
* Use atomic temp-file-then-append pattern consistent with existing JSONL
  writes in `internal/events/stream.go`
* Create `.backlogit/archive/stash.jsonl` on first write if it does not exist

### Task 2: Integrate archive into remove and harvest paths

Wire `appendToStashArchive()` into two call sites:

* `RemoveStashEntry()` at `stash.go:583` — call before rewriting stash file,
  with `removal_reason: "removed"`
* `harvestStashEntryLocked()` at `stash.go:271` — call after artifact creation
  succeeds, with `removal_reason: "harvested"` and `harvested_artifact_id` set

### Task 3: Tests for stash archive

* Unit test: `appendToStashArchive` creates file on first call, appends on
  subsequent calls, preserves all fields
* Integration test: `RemoveStashEntry` writes to archive before deletion
* Integration test: `HarvestStashEntry` writes to archive with artifact ID
* Edge case: concurrent archive writes (lock already held by caller)

### Task 4: Stage agent stash hygiene protocol

Add a hygiene step to `.github/agents/stage.agent.md` that:

* Runs `backlogit_fetch_stash` and checks `age_days` on all active entries
* Flags entries older than 30 days for operator review
* Uses `backlogit_stash_remove` for confirmed-stale entries
* Uses `backlogit_stash_edit` to update priority/kind when entries need
  adjustment rather than removal
* Broadcasts stale entry removals via agent-intercom

### Task 5: Refresh compound doc

Update `docs/compound/workflow-issues/stash-staleness-requires-custom-scripting-2026-04-09.md`:

* Set `resolved: true`
* Mark items 1-4 (created_at, stash remove, stash edit, staleness indicators)
  as implemented
* Add reference to the new stash archive and hygiene protocol
* Update symptoms section to reflect current state

## Constitution Check

* **Type-Safe Go**: Archive struct uses typed fields with JSON tags ✓
* **Test-First**: Tests defined before implementation ✓
* **Workspace Containment**: Archive writes to `.backlogit/archive/` ✓
* **CQRS**: Archive is a durable JSONL stream, not ephemeral cache ✓
* **Git-Friendly**: JSONL format minimizes merge conflicts ✓

## Risk Assessment

Low risk. Changes are additive (new archive file, new agent protocol step).
No existing behavior is modified beyond adding archive-append calls to
existing remove/harvest paths. Plan hardening is not required.
