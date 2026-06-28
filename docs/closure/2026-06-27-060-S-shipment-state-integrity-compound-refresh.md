---
chunk_strategy: h1-h2-h3
description: 'Compound-refresh review for the 060-S Shipment State Integrity shipment — classifies overlapping rollback/atomicity entries (filesystem rename rollback, SQLite txn rehydration, shipment/stash patterns) as keep (distinct mechanisms) and records the new atomic-claim-rollback + stale-blocked-clearing learning'
doc_type: closure
docline:
    ms.date: 2026-06-27T00:00:00Z
    ms.topic: reference
ingested_at: "2026-06-28T00:22:00Z"
schema_version: "1.0"
source: docs/closure/2026-06-27-060-S-shipment-state-integrity-compound-refresh.md
title: 060-S Shipment State Integrity — Compound Refresh
---

# Compound Refresh — Shipment 060-S (Shipment State Integrity)

- **Scope**: `recent` + entries overlapping the rollback / atomicity / shipment-lifecycle surface
- **Mode**: propose (no rewrites required; one new entry authored separately)
- **Context**: Post-merge closure of 060-S (PR #143, merge `7a51904b`)

## Entries reviewed

| Entry | Overlap with 060-S | Classification | Rationale |
|---|---|---|---|
| `docs/compound/best-practices/crash-safe-delete-rename-rollback-go-2026-04-23.md` | Shares the *roll back a partially completed mutation on a later failure* theme | **keep** | Distinct mechanism: filesystem-level rename-back in a `rename→DB→remove` sequence on a **single** path. The new 060-S entry is about **multi-item, in-memory state-machine** rollback via pre-claim snapshot + activated-set tracking, and about eliminating a fallible post-mutation read-back. Complementary, not superseding; the new entry cross-links it. |
| `docs/compound/database-issues/atomic-rehydration-sqlite-transaction-2026-04-08.md` | Shares the *atomicity / no torn intermediate state on crash* theme | **keep** | Distinct mechanism: wrapping a `DELETE`+rebuild in a single **SQLite transaction**. 060-S is application-level state-transition atomicity (no DB transaction wraps the multi-item activation). Both express "atomic by construction," different layers. |
| `docs/compound/go-patterns/f015-shipment-stash-patterns.md` | Same domain (shipment/stash) | **keep** | Covers stash migration / SQLite decoding / regression-test patterns for F015. Does not touch claim atomicity or blocked-metadata clearing. No overlap in the specific learning. |
| `docs/compound/db-reliability/archived-from-invertible-unarchive-2026-06-27.md` | Adjacent prior closure (067-S), archive-safety theme | **keep** | About `archived_from` invertibility on (un)archive — orthogonal to claim atomicity and blocked metadata. The 060-S ship re-confirmed that fix (dogfood), but it is a different invariant. |

## New entry authored

- `docs/compound/best-practices/atomic-multi-item-claim-rollback-and-stale-blocked-clearing-2026-06-27.md`
  — captures two intertwined durable learnings: (1) atomic multi-item state transitions
  need a pre-mutation snapshot + activated-set tracking + rollback, and must **not** rely on
  a fallible post-mutation read-back; (2) derived/stale metadata (`blocked_reason`) must be
  cleared at **all** re-entry choke points, because the lifecycle `blocked→queued` path
  bypasses the `validate_status_transition` hook (which only allows `blocked→active`). This
  is a **new** learning (`compound`), not a refresh of an existing one.

## Evidence used

- Source: `internal/core/shipment_lifecycle.go` (`ClaimShipment`, `rollbackShipmentClaim`),
  `internal/core/shipment.go` (`clearStaleBlockedReason`), call sites at `artifacts.go:481`,
  `shipment_lifecycle.go:484`, `:523`.
- Tests: `shipment_atomic_test.go`, `shipment_state_integrity_test.go` (5 tests, all PASS).
- Closure + runtime verification artifacts for 060-S (this batch).

## Decision

**No stale/superseded entries.** All overlapping entries describe distinct mechanisms at
different layers and remain accurate against current source. One new best-practices entry
added. No consolidate / replace / delete actions taken.
