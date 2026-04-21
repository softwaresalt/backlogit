---
title: "Stage handoff context for MCP merge sync"
description: "Context for a Stage deliberation about adding an MCP merge sync feature that detects .backlogit drift and updates the SQLite cache without a full rehydrate"
ms.date: 2026-04-20
session_type: stage-stash-handoff
topic: mcp-merge-sync
---

## Summary

We want a new MCP feature that can detect `.backlogit/` additions, updates, relocations, and deletions during a long-running agent session and perform a targeted merge sync without forcing a full SQLite rehydrate in the middle of that session.

## Recommended Direction

* Add a new MCP tool, `backlogit_merge_sync`, alongside the existing `backlogit_sync_index` tool
* Scope detection to `.backlogit/` rather than `docs/`
* Use a lightweight manifest keyed by relative path, file kind, size, mtime, and parsed `item_id` for artifact Markdown files
* Avoid content hashes in v1 unless later evidence shows mtimes are unreliable in practice
* For changed or added artifact Markdown files, parse the file and update the cache with `UpsertItem`, then reconcile dependency and link rows for that item
* For deleted artifact Markdown files, use `DeleteItemCascade` when the `item_id` is truly absent rather than merely relocated
* Rebuild stash index tables only when stash source files or stash provenance-related artifact fields change
* Rebuild item log index tables only when `.backlogit/logs/*.jsonl` changes
* Fall back to full `Rehydrate` when the delta is ambiguous, too large, or otherwise unsafe to merge incrementally

## Why This Shape

The current architecture treats SQLite as a disposable cache backed by `.backlogit/` source files. A merge sync should preserve that model instead of turning the cache into a second source of truth.

This hybrid approach keeps the common case cheap while remaining safe:

* artifacts get incremental updates
* stash and log tables can be rebuilt narrowly when their source files drift
* full rehydrate remains the safety net for complex or suspicious cases

## Scope Guardrails

* Do not include `docs/` in the first version of drift detection
* Do not start with full-content hashing across the workspace
* Do not attempt per-line incremental reconciliation for item logs in v1
* Do not remove or replace the existing full `backlogit_sync_index` path

## Suggested Deliberation Questions

* Should the manifest live only in MCP server memory in v1, or also persist to SQLite for cross-restart continuity?
* Should `backlogit_merge_sync` be explicit only, or should the MCP server also trigger it lazily after a stale check?
* What changed-file threshold should trigger fallback to full rehydrate?
* Should the first version support a dry-run mode that only reports detected drift?
* What response payload would be most useful for agents: counts only, or a list of touched IDs and fallback reasons?

## Candidate Success Criteria

* An agent can call a new MCP tool to refresh cache state mid-session without requiring full rehydration in the normal case
* Added, changed, relocated, and deleted artifact files under `.backlogit/` are handled correctly
* Stash and item log indexes stay consistent after targeted sync operations
* The implementation has an explicit and safe fallback to full rehydrate when merge sync confidence is low

## Relevant Code Paths

* `internal/mcp/tools.go`
* `internal/db/rehydration.go`
* `internal/db/queries.go`
* `internal/db/stash.go`
* `internal/db/logs.go`
* `internal/core/stash.go`
