---
title: "Archive/shipment re-persist drops modeled frontmatter fields"
description: "Deliberation on covering-feature scope for two re-persist data-loss bugs sharing one re-persist seam root cause"
source: docs/decisions/2026-07-28-archive-repersist-projection-drop-deliberation.md
doc_type: decision
chunk_strategy: h1-h2-h3
schema_version: "1.0"
topic: "Archive/shipment re-persist drops item_links and archive provenance (D04D63D0 + 7A965F8A)"
depth: "standard"
decision_status: "decided"
promoted_to: "plan"
linked_artifacts:
  - "docs/exec-plans/2026-07-28-archive-repersist-projection-drop-plan.md"
tags:
  - "bug"
  - "shipment"
  - "archive"
  - "db-projection"
  - "stage"
stash_ids:
  - "D04D63D0"
  - "7A965F8A"
---

## Problem Frame

Two medium-priority bug stash entries were surfaced during shipment closure work
(108-S / 109-S) and are triaged together as **Group A**:

* **D04D63D0** — `ship_shipment` aborts when a shipment's archive candidates
  include an already-archived linked deliberation (e.g. `054-DL`).
  `collectArchiveCandidateIDs` unconditionally pulls in `linkedDeliberationIDs`;
  `attachCommitToItems` then loads each candidate via the DB fast-path and
  re-persists it. Because the fast-path load drops top-level `archived_from` /
  `archived_status`, the write-boundary guard
  (`refusing to write archived artifact ... without provenance`,
  `internal/core/artifacts.go:761`) refuses the write and ship aborts **after**
  stamping members but **before** archiving the manifest.
* **7A965F8A** — `ArchiveItem`/shipment archive re-persist silently drops
  modeled `item_links` (observed: `123-F -> 120-F` `spike_ref` link dropped
  during shipment `109-S` archive; PR #309 restored it by hand).

**Who cares**: the Ship agent and operators — both bugs corrupt state or abort a
release mid-flight, requiring manual recovery (`backlogit archive <shipment-id>`
for partial ships; hand-editing frontmatter to restore links).

**Success criteria**: shipping a shipment whose feature links an already-archived
deliberation completes and archives the manifest; modeled `item_links` survive
every archive/shipment re-persist path. Both proven by regression tests written
first (Constitution Principle II).

**Out of scope**: the `50471E28` durable-writes second-layer hardening
(`ErrWriteIndeterminate` retry idempotency) — a separate concern deferred to a
future session; branch-protection config (`918BCDAF`) and external autoharness
tasks (`7F0A6E89`, `6FA0829B`) — not in-repo actionable.

## Research Findings

Grounded in current code:

* `internal/db/queries.go:28` — `selectCols` projection omits `links`,
  `archived_from`, `archived_status`. `scanArtifactRow` (queries.go:35+) never
  populates `Artifact.Links`, `ArchivedFrom`, or `ArchivedStatus`.
* `internal/models/artifact.go:45-56` — `Links`, `ArchivedFrom`, `ArchivedStatus`
  are modeled fields; `ToFrontmatterMap` (artifact.go:74+) emits `links` when
  non-empty and archive provenance only while `Status == archived`.
* `internal/core/shipment.go:337` — `loadArtifact` prefers `bldb.GetItem`
  (DB fast-path) and only falls back to `findArtifact` (Markdown) on
  `ErrNotFound`. So a load→`persistArtifact` round-trip on an indexed item loses
  any field the projection omits.
* `internal/core/shipment_lifecycle.go:292` — `collectArchiveCandidateIDs`
  filters archived features/descendants but appends `linkedDeliberationIDs`
  **without** an archived filter (line ~331-335).
* `internal/core/shipment_lifecycle.go:341` — `attachCommitToItems` load→set
  commit→`persistArtifact` is the exact re-persist path that trips the guard /
  drops links.
* `internal/core/archive.go:97` — `ArchiveItem` itself operates on **raw**
  frontmatter (read→parse→stamp→serialize) and preserves links; the observed
  link drop therefore originates in the DB-fast-path re-persist flow, not
  `ArchiveItem`'s own raw path.

**Precedent**: `internal/core/serializer_provenance_hardening_test.go` documents
the same class of defect for `MoveInQueue`, whose fix reloads each item from
Markdown rather than trusting the DB projection. This is a known, previously-fixed
pattern.

## Options Evaluated

### Option A: One covering feature, shared root-cause fix at the DB projection layer + symptom guards

Single covering feature. Primary fix carries `links` + archive provenance through
the `GetItem` projection so every fast-path load→persist round-trip is lossless.
Add symptom-level correctness (skip already-archived linked deliberations — it is
semantically wrong to stamp a shipment merge commit onto a pre-existing archived
deliberation) and per-symptom regression tests.

* **Pros**: fixes the true shared root cause once; defense-in-depth; each bug
  keeps its own regression test; matches the `MoveInQueue` precedent.
* **Cons**: touches the DB layer (broader blast radius than a pure core guard).
* **Effort**: medium. **Fit**: strong.

### Option B: Per-symptom core-only fixes (no DB projection change)

Fix each symptom in the core layer only (guard archived deliberations for
D04D63D0; reload-from-Markdown before re-persist for 7A965F8A), leaving the
projection lossy.

* **Pros**: smaller blast radius; no DB change.
* **Cons**: leaves the shared root cause latent — the next fast-path re-persist
  path re-introduces the same drop; higher long-term regression risk.
* **Effort**: low-medium. **Fit**: weaker (treats symptoms).

## Trade-off Comparison

| Criterion | Option A | Option B |
|---|---|---|
| Fixes shared root cause | Yes (projection) | No (latent) |
| Blast radius | Medium (db + core) | Low (core only) |
| Regression durability | High | Lower |
| Aligns with MoveInQueue precedent | Yes | Partial |

## Decision

**Single covering feature** grouping both bugs, because they share one seam
(the shipment/archive re-persist path) and one release concern
(re-persist field-integrity).

**Fix seam (revised after plan-review attempt 1).** The original Option A
proposed widening the `GetItem` projection. Plan-review (Go Reviewer,
Architecture Strategist, Learnings Researcher — all P1) established this premise
is factually wrong: the `items` table has **no** `links`/`archived_from`/
`archived_status` columns (links live in the separate normalized `item_links`
table; provenance is unindexed). Widening the projection would require a schema
migration, create a links dual-source-of-truth, and leave the same class of bug
latent for any future modeled field. The corrected, precedent-aligned decision is
**reload-from-Markdown at the core re-persist seam** (the `MoveInQueue` /
`serializer_provenance_hardening` precedent — Markdown is the source of truth,
the index is a rebuildable cache).

Decompose into **two independent** test-first tasks:

1. **D04D63D0 correctness** — skip already-archived linked deliberations in the
   shipment archive flow so `attachCommitToItems` never re-persists a pre-existing
   archived deliberation; regression test asserts ship-with-linked-archived-
   deliberation succeeds and archives the manifest.
2. **7A965F8A correctness** — reload each candidate from Markdown before
   `persistArtifact` in the shipment re-persist seam so `item_links` (`spike_ref`)
   and archive provenance survive for every stamped member; regression test
   asserts frontmatter links + provenance survive (populated AND empty cases).

The two tasks are independent (no dependency). Stage only plans; Ship implements.

## Rejected Alternatives

- **Widen the DB `GetItem` projection** (original Option A) — rejected at
  plan-review: no such columns exist; it would add a schema migration, a links
  dual-source-of-truth against `item_links`, and leave the defect class latent.
- **Option B (per-symptom core-only, projection untouched)** — superseded: the
  reload-from-Markdown seam IS the core fix and addresses the whole dropped-field
  class, not just two symptoms.
- **Splitting into two independent features** — rejected: the bugs share the
  re-persist seam and belong in one coordinated release unit.

## Unresolved Questions

* Whether the cleanest D04D63D0 fix is skipping archived deliberations in
  `collectArchiveCandidateIDs` vs. guarding in `attachCommitToItems` — left to
  the implementation plan / Ship. Both are viable; skipping upstream is
  semantically cleaner.

## Risks and Mitigations

* **Risk**: reloading from Markdown adds a file read per re-persisted candidate.
  **Mitigation**: shipment archive sets are small (feature + tasks + linked
  deliberations); cost is negligible and matches the accepted `MoveInQueue`
  precedent. Full `go test ./...` gate at Ship time.
* **Risk**: empty-vs-populated `links` round-trip false-green (omitempty).
  **Mitigation**: the Unit 2 regression test asserts BOTH the populated
  (`spike_ref` survives) and empty/nil cases
  (`docs/compound/2026-07-21-omitempty-defeats-arrays-always-json-contract.md`).
* **Risk**: weakening the ship-gate's fail-closed provenance read.
  **Mitigation**: the plan preserves the Markdown-based `archived_status`
  fail-closed read (`docs/compound/2026-07-20-ship-gate-descoped-archived-member-exemption.md`);
  it is not relaxed.
* **Risk**: partial-ship recovery path already documented
  (`backlogit archive <shipment-id>`) — not regressed by these fixes.
