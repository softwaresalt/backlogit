---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: "A governed backlogit archive that errors on the git launch is recovered by the approved CLI with a process-scoped git on PATH, not by a retry or a hand move"
description: "When the backlogit MCP server cannot launch git (its PATH resolves a broken Git for Windows launcher), governed archive operations error before any move. Do not retry the same MCP call and do not hand-move files. Verify the shipped event and the served roots, then run the governed CLI once with a process-scoped PATH prefix (no system change), then verify read-only."
doc_type: learning
source: docs/compound/2026-10-14-ship-governed-archive-git-launch-fallback.md
docline:
    date: 2026-10-14T04:44:00Z
    severity: medium
    tags:
        - shipment
        - post-merge-closure
        - mcp
        - git
        - windows
---

# A governed archive that errors on the git launch

## Symptom

`backlogit_archive_item` returned `internal: archive item: plan archive move: detect git worktree: git rev-parse --show-toplevel: exit status 1: error launching git: The system cannot find the path specified`. The error occurs before any file moves, so the shipment record and the archive were unchanged.

## Cause

The backlogit MCP server resolves git from the PATH it inherited. On this Windows host, PATH resolved `C:\Program Files\Git\cmd\git.exe`, a launcher whose `mingw64\bin` and `bin` directories were missing, so the launch failed. Other git installations existed (Visual Studio Team Explorer, Copilot), but the server was not configured to use them.

## What worked

1. Verify the served roots before any fallback: the metadata catalog root and storage root match, and the database is a direct child of the storage root.
2. Verify the state the archive depends on. For shipment closure, confirm the shipped event in the item log (`.backlogit/logs/{id}.jsonl`) and the doctor finding (`backlogit doctor --check-shipped-event-completeness`).
3. Take the shipment lock, then run the governed CLI operation once, with a process-scoped PATH prefix to a working git. Do not change the system PATH or the git installation.
4. Verify read-only: the archive frontmatter, the removed queue record, the doctor finding cleared, and the index synced (`backlogit_sync_index`, which did not need git).
5. Release the lock and record a P-005 note that names the MCP error and the approved CLI route.

## What did not work

* Retrying the same MCP call. Each failure is the same failure, and a retry adds a second write attempt against a shipment whose state is already partly changed.
* Hand-moving the queue record to the archive. That bypasses the governed lifecycle and its audit events.
* Calling the shipment safe-close again. A shipment that is already `shipped` is not admissible to safe-close.

## Rule

A governed backlog operation that errors on an environment dependency may be retried only through an approved fallback that keeps the same governed operation, and only for an archive operation proven to have failed before any move. Verify the state before and after. This does not authorize any mutation after an unknown or partial shipment timeout: an incomplete quiescent `backlogit_ship_shipment` outcome halts for operator reconciliation (see the 2026-10-06 timeout learning). Never synthesize an event, and never edit a shipment record directly.
