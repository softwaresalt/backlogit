---
chunk_strategy: h1-h2-h3
description: Compacted Stage and Ship memory for shipment 084-S ancestor-aware shipment-gate staleness.
doc_type: memory
docline:
    ms.date: 2026-07-10T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/memory/compacted/2026-07-10-084-S-ancestor-aware-staleness-compacted.md
title: Compacted memory - 084-S ancestor-aware shipment-gate staleness
---
## Summary

Shipment `084-S` fixed the false-staleness bug exposed by 083-S by making shipment-gate member evidence ancestor-aware. Stage promoted stash `885A7F65` after an operator-authorized plan-review confirmation cycle; Ship merged PR #182, shipped and archived `084-S`, and opened closure PR #183.

## Archived originals

* `docs/archive/memory/2026-07-06-stage-885A7F65-ancestor-staleness-checkpoint.md`
* `docs/archive/memory/2026-07-06-084-S-ship-session-complete.md`

## Decisions and outcomes

* Member evidence is fresh when `head_sha` is equal to or an ancestor of the shipment head, checked with `git merge-base --is-ancestor`.
* Git object names are restricted to 40- or 64-hex strings before use in git exec calls.
* Ancestor checks are bounded, use argv-array execution, `cmd.Dir=ws.RootPath`, `gate.MinimalEnv`, stderr capture, and fail-closed trichotomy.
* Shipment head is resolved once before evaluation, then re-resolved as the final read before success to catch drift.
* PR #182 merged by true merge commit `f49ce3c37b460afce81591ca6e354b8de3a14a17`; the fixed binary then shipped its own `084-S` closure successfully.

## Files and verification

* `internal/core/shipment_gate.go` gained ancestor-or-equal lineage checks and bounded head resolution.
* Tests covered included, divergent, malformed, timeout/cancel, head-drift, and no-repo/legacy skip behavior.
* Plan review required four attempts; H1 fixed `headSHABounded` so bounded-read timeout/cancel fails closed.
* Copilot found a bounded-helper hard-cap issue and dedup opportunities; fixed and re-review was clean.
* Runtime verification passed five real-subprocess scenarios. New compound learnings captured ancestor-aware staleness and bounded helper timeout hard cap.
