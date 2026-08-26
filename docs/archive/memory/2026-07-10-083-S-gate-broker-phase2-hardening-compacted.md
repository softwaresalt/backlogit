---
chunk_strategy: h1-h2-h3
description: Compacted Stage and Ship memory for shipment 083-S gate broker phase-2 hardening and re-closure.
doc_type: memory
docline:
    ms.date: 2026-07-10T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/memory/compacted/2026-07-10-083-S-gate-broker-phase2-hardening-compacted.md
title: Compacted memory - 083-S gate broker phase 2 hardening
---
## Summary

Shipment `083-S` delivered gate-broker phase-2 hardening and later re-closed after 084-S fixed ancestor-aware shipment staleness. Stage bundled five deferred findings into `083-F`; Ship implemented F1, F4, F5, F7, and Q3 indexed gate evidence, merged PR #180, hit a real post-merge closure wall, then completed re-closure after the 084-F fix landed.

## Archived originals

* `docs/archive/memory/2026-07-06-stage-gate-broker-phase2-session.md`
* `docs/archive/memory/2026-07-06-083-S-ship-feature-complete-checkpoint.md`
* `docs/archive/memory/2026-07-06-083-S-copilot-iter1-resolved-checkpoint.md`
* `docs/archive/memory/2026-07-06-083-S-post-merge-closure-BLOCKED-checkpoint.md`
* `docs/archive/memory/2026-07-07-083-S-reclosure-RESOLVED-checkpoint.md`

## Decisions and outcomes

* F4 accepts forced evidence unconditionally but requires `ran==true` for `passed` evidence.
* Q3 introduced a disposable `gate_evidence` projection table sourced from logs during rehydration; logs remain source of truth.
* Doctor uses the indexed projection with log-scan fallback when projection rows are absent or stale.
* PR #180 merged by true merge commit `ac41bb1d2611fadd0fae6ccc49b3a8233468622d`; feature value reached `main` but shipment closure initially refused under strict head-sha equality.
* After 084-S shipped ancestor-aware staleness, rerunning the same `shipment ship 083-S --sha ac41bb1d...` succeeded and archived 11 artifacts.

## Files, failures, and verification

* Core and DB gained shared gate-evidence predicate/constants, `gate_evidence` table/index, sync population, and doctor projection lookup.
* Rehydration was optimized after Copilot flagged double parsing: `rehydrateItemLogs` returns parsed events and `rehydrateGateEvidence` consumes them.
* Closure stopped rather than forcing because strict equality was a real semantic mismatch: member evidence heads were ancestors of the merge commit, not equal to it.
* Q3 sync idempotency was verified with repeated `backlogit sync` and stable doctor output.
* Compound learning `2026-07-06-ancestor-aware-shipment-gate-staleness.md` was updated with the 083-S exposure cross-reference.
