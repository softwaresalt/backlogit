---
chunk_strategy: h1-h2-h3
description: Compacted Stage and Ship memory for shipment 077-S shipment item normalization consolidation.
doc_type: memory
docline:
    ms.date: 2026-07-10T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/memory/compacted/2026-07-10-077-S-shipment-items-normalization-compacted.md
title: Compacted memory - 077-S shipment items normalization
---
## Summary

Shipment `077-S` consolidated duplicate shipment-item normalization logic. Stage promoted stash `17D29DDC` into `077-F` and `077.001-T`; Ship later resumed post-merge closure after a Windows update interrupted the original session.

## Archived originals

* `docs/archive/memory/2026-07-03-stage-17D29DDC-normalize-shipment-items-consolidation.md`
* `docs/archive/memory/2026-07-03-ship-077-S-post-merge-closure-session.md`

## Decisions and outcomes

* The true duplicate was the `[]any` to `[]string` mapping shared by core and MCP; the core mutator stayed separate because it wraps read normalization with writeback.
* `core.shipmentItems` became exported `core.NormalizeShipmentItems` and was hardened to never return nil, including empty `[]string{}` input.
* MCP `normalizeShipmentItems` was deleted; `handleListShipments` delegates to core while retaining the end-to-end never-null guard.
* PR #168 had already merged at `c8487407d5ddb19d26c754ce82606df929e35f46`; post-merge closure shipped `077-S` and archived `077.001-T`, `077-F`, and `077-S`.

## Files and verification

* `internal/core/shipment.go` exported `NormalizeShipmentItems` and became the single mapping source.
* Unit tests moved to `internal/core/shipment_normalize_test.go` with the empty-slice non-nil case.
* MCP response tests retained list never-null coverage.
* Merged-code gates passed on resume; gofmt noise was CRLF-only and not reformatted.
* No new compound entry was created because never-null and single-shaper invariants were already covered by existing learnings and 075-S closure.
