---
chunk_strategy: h1-h2-h3
description: Compacted Stage and Ship memory for shipment 075-S covering-feature display in shipment views.
doc_type: memory
docline:
    ms.date: 2026-07-10T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/memory/compacted/2026-07-10-075-S-covering-feature-display-compacted.md
title: Compacted memory - 075-S shipment covering feature display
---
## Summary

Shipment `075-S` shipped a read-only covering-feature projection for shipment list/get surfaces. Stage promoted stash `D070FD3C` into `075-F` plus core, CLI, and MCP tasks; Ship implemented all three tasks, resolved an inherited Stage-commit halt, merged PR #164, and completed post-merge closure.

## Archived originals

* `docs/archive/memory/2026-07-02-stage-D070FD3C-covering-feature-display.md`
* `docs/archive/memory/2026-07-02-075-S-task3-mcp-checkpoint.md`
* `docs/archive/memory/2026-07-02-075-S-HALT-inherited-stage-commit.md`
* `docs/archive/memory/2026-07-02-ship-075-S-post-merge-closure-session.md`

## Decisions and outcomes

* The governing decision was forward-only display; existing shipment manifests and titles were never rewritten.
* Final shape is a top-level `covering_feature: {id,title}` object on JSON responses, not derived keys inside `custom_fields`, preventing write-path echo from persisting derived data.
* Resolution is read-only and uses `bldb.GetItem`, not `loadArtifact`, to avoid upserts on cache misses.
* Covering feature is the parent-first dotless root feature; zero-feature shipments omit the object everywhere.
* PR #164 merged as true merge commit `842e8883899ba25ce9c31840c89806ed2e032549`; post-merge closure shipped and archived `075-S` with clean reconcile.

## Files, failures, and verification

* Core shared shaper, CLI list/get, generated CLI docs, and MCP `handleGetShipment` / `handleListShipments` all surfaced the projection.
* MCP tests covered get/list shape, zero-feature omission, no custom-field leak, read-only invariant, and CLI/MCP parity.
* Plan review rejected the earlier custom-fields injection approach because agents could echo derived data into writes.
* Ship halted when PR #164 inherited unpushed Stage commit `f316dfd`, invalid docline frontmatter, and backlog state; the operator resolved the Stage issue before merge.
* Runtime verification confirmed projection present, zero-feature omit, and storage/index hashes unchanged. Follow-up stash `17D29DDC` captured normalization consolidation.
