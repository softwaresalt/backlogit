---
chunk_strategy: h1-h2-h3
description: 'Two durable storage-contract facts graduated from 105-S / feature 125-F documentation reconciliation. (1) The SQLite index is NOT rebuilt automatically on read; NewWorkspace only opens the DB and ensures schema, PersistentPreRunE does no auto-sync, and read paths query the DB immediately, so a missing or stale backlogit.db yields empty/outdated results until an explicit rebuild runs (the backlogit sync CLI command or the backlogit_sync_index MCP tool, both calling db.Rehydrate). (2) telemetry-sessions.jsonl is a materialized summary rewritten via temp-file-then-rename on each harvest (writeTelemetryJSONL; atomic on POSIX, remove-then-rename on Windows) and reset by --force, so it is NOT append-only like the persistent per-item logs and telemetry.jsonl, which use os.O_APPEND. Docs that call the whole JSONL model "append-only" or promise automatic cache recovery are factually wrong against the code.'
doc_type: learning
docline:
    category: best-practices
    component: storage-contract
    date: 2026-07-25T00:00:00Z
    file_path: internal/core/workspace.go
    message: Docs must not claim the SQLite cache auto-rebuilds on read or that telemetry-sessions.jsonl is append-only; rebuild is explicit (backlogit sync) and the sessions file is materialized (temp-rename).
    problem_type: best_practice
    resolution_type: documentation
    resolved: true
    root_cause: README architecture prose asserted automatic cache recovery and a uniform append-only JSONL model, but the code rebuilds the index only via explicit paths (backlogit sync CLI / backlogit_sync_index MCP / migrate) and rewrites the session-summary file via temp-file-then-rename on each harvest (atomic on POSIX; remove-then-rename on Windows).
    severity: low
    tags:
        - storage-contract
        - db-cache
        - rehydrate
        - backlogit-sync
        - telemetry
        - append-only
        - materialized-file
        - documentation-drift
        - cqrs
schema_version: "1.0"
source: docs/compound/best-practices/storage-contract-index-rebuild-and-telemetry-sessions-materialization-2026-07-25.md
title: 'backlogit storage contract: the SQLite index is not auto-rebuilt on read (use backlogit sync), and telemetry-sessions.jsonl is a materialized summary, not append-only (125-F)'
---

# backlogit Storage Contract: Explicit Index Rebuild and Materialized Session Summary

Two durable, code-verified facts graduated from shipment 105-S (feature 125-F,
"Documentation consistency and README brochure rework for v1.7.0", PR #298, merge
`ae0f205377bf0843ce3f11307c53187538459c02`). Both were surfaced by a Copilot review
of relocated README architecture prose and confirmed against source before the doc
was corrected. Capture them so future documentation, agent guidance, and operator
runbooks describe the storage contract accurately instead of re-drifting.

## Problem

The README "Overview" prose made two convenient-but-false claims about backlogit's
CQRS storage model:

1. "The cache rebuilds automatically from the Markdown source whenever it is missing
   or stale."
2. "A JSONL event model records state transitions, comments, and telemetry in
   **append-only** files" — listing per-item logs, `telemetry.jsonl`, **and**
   `telemetry-sessions.jsonl` together under one append-only banner.

Both are inaccurate against the v1.7.0 code.

## Symptoms

- Deleting or externally staling `backlogit.db` and then running a read (`backlogit
  list`, MCP queries) returns empty or outdated results — no silent auto-rebuild.
- `telemetry-sessions.jsonl` does not grow monotonically like an append-only log; it
  is fully rewritten on each harvest and truncated/reset by `--force`.
- Documentation and agent mental models that assume "the cache always heals itself"
  or "every JSONL file is append-only" produce wrong operational guidance.

## What Did Not Work

A version-pinned fact audit (badges, dependency versions, command flags, file paths)
passed the README as accurate. That audit was too shallow: it checked that the named
files exist and the versions match, but not the deeper *semantic* claims about
rebuild timing and write discipline. Code-level verification was required to catch
the drift.

## Solution

Verify storage-contract claims against the actual read/write paths, then state them
precisely.

### Fact 1 — index rebuild is explicit, not automatic

- `core.NewWorkspace` opens the DB and ensures schema only; it does **not** rehydrate
  from Markdown (`internal/core/workspace.go`). It registers an *informational*
  "index stale" log hook (`hooks.RegisterLogIndexStale`) — logging, not rebuilding.
- The root command's `PersistentPreRunE` sets log level and JSON-RPC wrapping only —
  no auto-sync (`internal/cli/root.go`).
- Read paths query the DB immediately, e.g. `list` calls `db.QueryItems` right after
  `NewWorkspace` (`internal/cli/list.go`).
- The actual rebuild is `db.Rehydrate`, invoked only by explicit rebuild paths: the
  `backlogit sync` CLI command (`internal/cli/root.go` `newSyncCommand`), the
  `backlogit_sync_index` MCP tool (`internal/mcp/tools.go` `handleSyncIndex`), and
  `migrate`. The MCP server additionally reconciles incrementally via
  `MergeSync`/`RehydrateWithManifest` on git operations. None of these fire on an
  ordinary read.

Accurate phrasing: *"Because the cache is disposable, you rebuild it from the Markdown
source with `backlogit sync` (or the `backlogit_sync_index` MCP tool) whenever it is
missing or out of date."*

### Fact 2 — telemetry-sessions.jsonl is materialized, not append-only

- `writeTelemetryJSONL` writes prior + new records to a `*.tmp` file via `os.Create`
  then renames it over `telemetry-sessions.jsonl` (`internal/telemetry/harvest.go`) —
  atomic on POSIX; on Windows the destination is removed first, a narrow non-atomic
  window the code accepts for regenerable telemetry. The checkpoint `--force` path
  reprocesses from offset 0 and overwrites the file (`internal/telemetry/checkpoint.go`).
- By contrast, the persistent event streams are genuinely append-only via
  `os.O_APPEND`: per-item event logs `.backlogit/logs/{item-id}.jsonl` and
  agent-operation `telemetry.jsonl`. The tool-calls / session-facts harvest files
  (`internal/telemetry/events_harvest.go`) also append with `os.O_APPEND`, but `--force`
  deletes and recreates them, so they are incremental-append rather than strictly
  append-only over their lifecycle.

Accurate phrasing: *"Work-item history is appended per item to
`.backlogit/logs/{item-id}.jsonl`, and agent-operation telemetry is appended to
`.backlogit/telemetry.jsonl`. Harvested Copilot CLI session summaries are materialized
to `.backlogit/telemetry-sessions.jsonl`, which backlogit rewrites via
temp-file-then-rename on each harvest."*

## Why This Works

The two write disciplines exist for different reasons: per-item/operation logs are
**event streams** (append preserves history and keeps concurrent writers cheap via a
shared `EventWriter` mutex), while `telemetry-sessions.jsonl` is a **derived
projection** (a rebuilt summary that must stay internally consistent, hence the
temp-file-then-rename rewrite — atomic on POSIX, with a narrow remove-then-rename
window on Windows). Likewise the SQLite index is a *disposable
projection* of the Markdown source of truth — rebuild is a deliberate operator/agent
action (`sync`), not an implicit side effect of reading, which keeps reads fast and
predictable. Describing both precisely prevents operators from trusting a rebuild that
never fires and from treating a rewritten summary as an immutable log.

## Prevention

- When documenting a projection/cache, state the **trigger** for its rebuild
  explicitly (which command, on which event) rather than implying automatic recovery.
- When listing files as "append-only," verify each writer's open flags: only files
  opened with `os.O_APPEND` qualify; temp-file-then-rename writers are materialized
  rewrites.
- Treat canonical-fact audits as *semantic*, not just version/path checks: confirm
  timing and write-discipline claims against the read/write code paths, not only that
  the named artifacts exist.
- Cross-check both the prose and any adjacent tables (e.g. a "Technology Stack" row)
  so a corrected claim does not leave an inconsistent duplicate elsewhere in the doc.
