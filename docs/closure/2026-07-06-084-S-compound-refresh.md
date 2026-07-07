---
chunk_strategy: h1-h2-h3
description: 'Compound-refresh report for shipment 084-S (ancestor-aware shipment-gate staleness, merge f49ce3c). Scope: recent gate-broker / exec-timeout learnings adjacent to the 084-S change. Two new entries added (ancestor-aware staleness + fail-closed merge-base exit-code handling; bounded-helper-timeout hard cap). One existing entry classified UPDATE (082-S external-process-timeout-before-probe: forward cross-ref added to the hard-cap entry). No consolidations, replacements, or deletions — the new entries are complementary refinements, not supersessions. All touched files docline-clean, LF-normalized.'
doc_type: closure
docline:
    ms.date: 2026-07-06T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-06T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-06-084-S-compound-refresh.md
title: 084-S ancestor-aware shipment-gate staleness — Compound Refresh Report
---

# Compound Refresh Report — 084-S

**Scope**: `recent` (gate-broker + exec/timeout learnings adjacent to the 084-S change).
**Context**: shipment 084-S closure, feature merge `f49ce3c37b460afce81591ca6e354b8de3a14a17`.
**Mode**: `apply`.

## Evidence gathered

- Changed runtime surface: `internal/core/shipment_gate.go` (ancestor-aware member
  staleness, bounded/fail-closed git helpers, hard-cap timeout).
- Closure artifacts: 084-S adversarial review, runtime verification, feature-PR and
  post-merge operational closure.
- Overlapping existing entries by tag (`timeout`, `exec`, `gate`, `dos`, `locking`,
  `core`): the 082-S gate-broker series.

## Classifications

| Entry | Outcome | Rationale |
|---|---|---|
| `2026-07-06-external-process-timeout-before-probe.md` (082-S) | **update** | Still accurate. Added a forward `Related` cross-ref to the new hard-cap entry: 084-S refines it — bounding is necessary but the bound must fit the workload (a 600s command timeout on a near-instant metadata read is still a DoS). |
| `2026-07-06-ancestor-aware-shipment-gate-staleness.md` | **new** | Brand-new pattern (ancestor-vs-equality staleness + fail-closed `git merge-base --is-ancestor` exit-code handling incl. Windows ctxErr-before-ExitError). No prior entry covered it. |
| `2026-07-06-bounded-helper-timeout-hard-cap.md` | **new** | New reliability rule (hard-cap a bounded helper's deadline). Distinct from — and cross-linked to — the 082-S probe-timeout lesson. |
| `2026-07-06-exec-binary-config-must-be-bare-path-validated.md` (082-S) | **keep** | RCE/exec-seam lesson; unrelated to the staleness/timeout change. Accurate. |
| `2026-07-06-autoharness-gate-broker-integration-contract.md` (082-S) | **keep** | Contract doc; unaffected. Accurate. |

## Actions taken (mode: apply)

- Created `docs/compound/2026-07-06-ancestor-aware-shipment-gate-staleness.md`.
- Created `docs/compound/2026-07-06-bounded-helper-timeout-hard-cap.md`.
- Updated `docs/compound/2026-07-06-external-process-timeout-before-probe.md`
  Related section with a forward cross-ref.

## Summary

- new: 2 · update: 1 · keep: 2 · consolidate: 0 · replace: 0 · delete: 0
- No stale or superseded entries. The 084-S learnings extend (do not contradict) the
  082-S gate-broker series; cross-references now connect the cluster.
