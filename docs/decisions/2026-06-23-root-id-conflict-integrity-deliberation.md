---
chunk_strategy: h1-h2-h3
description: Deliberation on preventing and detecting undetected top-level work-item ID conflicts between queue and archive
doc_type: decision
docline:
    decision_status: decided
    depth: deep
    linked_artifacts:
        - docs/exec-plans/2026-06-23-root-id-conflict-integrity-plan.md
    promoted_to: plan
    stash_ids:
        - 0F65FBC9
    tags:
        - id-allocation
        - archive-safety
        - data-integrity
        - doctor-audit
        - rehydration
    topic: Top-level work-item ID conflicts go undetected until archive time (stash 0F65FBC9)
ingested_at: "2026-06-26T02:33:47Z"
schema_version: "1.0"
source: docs/decisions/2026-06-23-root-id-conflict-integrity-deliberation.md
title: 'Root-ID Conflict Integrity: Detection, Allocation, and Archive Safety'
---

## Problem Frame

Two top-level work items (feature / chore / spike) can be assigned the **same
root ID** — one living in `.backlogit/queue/`, the other already in
`.backlogit/archive/`. No error is raised at creation. The collision only
surfaces at **archive time**, when the queued item is archived and its file
would land on the same-named file already in the flat archive directory.

The failure is silent and lossy: `ArchiveItem` writes `archive/<basename>` via
`tmp + rename`, which **replaces** any existing destination of the same name
(`internal/core/archive.go:146-156`), so archiving the queue copy can overwrite
a *different* archived item that shares the filename.

**Who cares:** harness operators and the Ship agent rely on stable, unique root
IDs and on archive being a non-destructive, audit-preserving operation. A
silently overwritten archive file is unrecoverable data loss in a Git-tracked
backlog.

**Constraints:** Go 1.24, test-first (Constitution II), workspace containment,
Git-mergeable file state (Constitution IX), minimal new dependencies
(Constitution VI), and the 2-hour / single-skill-domain task granularity rule.

**Success criteria:**
1. A duplicate root ID across queue+archive is *detectable* on demand
   (`doctor`) from canonical files, not the PK-collapsed index.
2. Creation *cannot* silently mint an ID that already exists on the canonical
   filesystem (queue or archive).
3. Archiving *refuses* to overwrite a different existing archive destination and
   returns an actionable error instead of clobbering.
4. Rehydration *warns* when two source files map to the same ID instead of
   silently collapsing them with `INSERT OR REPLACE`.
5. A reproducing test pins the end-to-end scenario.

**Scope boundary (explicit non-goals):**
- Repairing the *existing* 060/061/062 shipment-manifest drift (see "Manifest
  Drift Decision" below) — that is a one-time data repair, a different problem
  class, routed to a separate follow-up.
- Redesigning the flat archive directory layout.
- Cross-branch merge-time ID reconciliation policy beyond detection + guards.

## Research Findings (grounded in current code)

### Finding 1 — Allocation reads only the index, per type, no guard

`core.NextID` (`internal/core/naming.go:55-77`) runs
`SELECT id FROM items WHERE artifact_type = ?` and returns `MAX(ordinal)+1`. It
reads only the SQLite index, is per-artifact-type, has **no** persistent
monotonic counter and **no** pre-write uniqueness check. `CreateArtifact`
(`internal/core/artifacts.go:97-175`) uses `NextTypedHierarchicalID` for
layout-typed IDs and falls back to `NextID`; neither validates the resolved ID
against the canonical filesystem before writing.

### Finding 2 — The index masks duplicates (PK collapse)

`items.id` is a PRIMARY KEY. `Rehydrate` (`internal/db/rehydration.go:48-160`)
walks the whole `.backlogit` tree (queue **and** archive), then batch-inserts
via `upsertItemTx` → `INSERT OR REPLACE INTO items`. Two source `.md` files that
share an ID **collapse into one row**, so every index-based check (`list`,
`query`, `NextID`, `db.FindDuplicates`) is blind to the duplicate. In a
fully-synced single workspace `NextID` is actually correct because the archived
file *is* indexed; the duplicate enters through a window where the cache does
not yet reflect the archived (or other-branch) file — i.e. cross-branch /
worktree / multi-agent creation, or allocation against a stale cache.

### Finding 3 — Archive overwrite is unguarded

`ArchiveItem` (`internal/core/archive.go:146-166`) computes
`archivePath = archive/<base(currentPath)>` and `tmp+rename`s onto it. There is
careful handling for the *same-path* in-place case (lines 157-166, 99-107) but
**no** guard for the *different-item-same-filename* case. A collision overwrites
the prior archived item.

### Finding 4 — Detection already partly exists (key correction to the steer)

The stash entry proposes "add a doctor audit that scans canonical .md files."
**This is already partially implemented.** `core.Doctor` with
`opts.CheckDuplicates` (`internal/core/doctor.go:108-136, 221+`) walks canonical
`.md` files across `artifactSearchDirs(ws)` — which includes the archive
directory via registry directory rules (`artifacts.go:494-540`) plus the queue
root — and emits `FindingDuplicateID` when one ID maps to ≥2 files. There is
also `db.FindDuplicates` (`internal/db/duplicates.go`), but that one is
index-based (title-grouped) and therefore *does* suffer the PK-collapse blind
spot. So the work for "detection" is to **extend and surface** the existing
file-based audit (add an explicit level-1 root-ID collision finding; ensure it
runs/visible by default in the `doctor` CLI), not to build a new package from
scratch.

### Finding 5 — Prior learning on rehydration atomicity

`docs/compound/database-issues/atomic-rehydration-sqlite-transaction-2026-04-08.md`
established that `Rehydrate` must keep its clear+rebuild inside a single
transaction (now honored at `rehydration.go:83-160`). Implication for the
rehydrate-warning work: add duplicate detection at the **collection** phase
(the `collected` slice) or at insert time **without** disturbing the
transaction boundary or the `upsertItemTx` path.

### Finding 6 — Durable counter has real cross-branch subtlety

A persistent per-type high-water-mark only helps the exact root-cause window
(an item not yet visible in this workspace's cache/files). But a counter file
that is **committed** to Git inherits the same cross-branch visibility problem
(branch A's bump is invisible to branch B until merge) *and* introduces a
new high-churn merge-conflict surface (Constitution IX). A counter that is
**not** committed cannot see other-branch allocations at all. The pre-write
filesystem guard (Finding 1/3 remediation) already closes the single-workspace
reuse case. So the durable counter's marginal value is narrow and its design is
non-trivial — it is the highest-blast-radius element of the steer.

## Options Evaluated

### Option A — Detection hardening + write-time guards (recommended core)

Extend the existing file-based doctor audit (root-ID collision finding, default
surfacing); add a **pre-write uniqueness guard** in `CreateArtifact`/`NextID`
authoritative over the canonical queue+archive filesystem; add an **archive
overwrite refusal** in `ArchiveItem`; add a **rehydrate duplicate-source
warning**; pin it all with a reproducing integration test. Defers the durable
counter.

- **Pros:** Fixes the acute, lossy failure (silent archive overwrite) and the
  creation reuse path with low, well-contained blast radius. Each piece is an
  independent, testable, single-domain unit. Mostly extends existing code.
- **Cons:** Does not address the narrow cross-branch-not-yet-merged window
  beyond *detecting* and *refusing* (rather than *preventing allocation*).
- **Effort:** medium. **Fit:** high.

### Option B — Option A **plus** durable per-type high-water-mark counter

Everything in A, plus persist a per-type counter so archived ordinals are never
reused even when out of view.

- **Pros:** Strongest guarantee against ordinal reuse within a single
  persistent workspace; matches the full 6-point steer.
- **Cons:** Highest blast radius (new persistent state on the central allocation
  path); committed-counter has unresolved cross-branch visibility + merge
  semantics; risk of the counter and canonical files disagreeing. Needs careful
  initialization (seed from canonical max across queue+archive).
- **Effort:** high. **Fit:** medium (powerful but design-incomplete).

### Option C — Detection-only (doctor audit), defer all guards

Ship only the audit hardening; leave create/archive paths unguarded.

- **Pros:** Lowest risk, smallest change.
- **Cons:** Leaves the actual data-loss path (silent archive overwrite) and the
  silent-reuse path open. Fails success criteria 2 and 3. Unacceptable.
- **Effort:** low. **Fit:** low.

## Trade-off Comparison

| Criterion | Option A (core + guards) | Option B (+ durable counter) | Option C (detect only) |
|---|---|---|---|
| Closes silent archive overwrite | Yes | Yes | No |
| Closes silent create reuse (single ws) | Yes | Yes | No |
| Closes cross-branch reuse window | Detect + refuse | Attempted (design-incomplete) | No |
| Blast radius | Low–medium | High | Low |
| Constitution IX merge risk | None | Elevated (counter file) | None |
| Task granularity fit (2h/single-domain) | Clean | Clean but riskiest task | Clean |
| Matches operator 6-point steer | 5 of 6 | 6 of 6 | 1 of 6 |

## Decision

**Adopt Option A as the committed core, and additionally include the durable
counter (Option B's increment) as a final, dependency-gated, explicitly
risk-flagged task — sequenced last and treated as the part most likely to be
deferred by the operator.**

Rationale: Option A's pre-write filesystem guard + archive refusal eliminate the
real data-loss and silent-reuse paths with contained blast radius, satisfying
all five hard success criteria. The operator gave a strong six-point steer and
explicitly asked that detection, allocation hardening, durable counter, archive
safety, rehydrate integrity, and TDD repro each be split appropriately; so the
durable counter is **kept in the decomposition** (point 3 of the steer) rather
than dropped — but it is sequenced last, depends on the allocation hardening,
and carries an explicit open design question (cross-branch visibility + merge
semantics) for the operator / executor to resolve before building. If the
operator prefers, the durable-counter task can be returned to backlog without
blocking the rest of the shipment.

Decomposition (each unit ≤2h, single skill domain, test-first per Constitution
II):

1. **Detection** — harden the canonical file-based doctor audit: add a level-1
   root-ID collision finding and ensure queue+archive duplicate detection is
   surfaced by the `doctor` CLI. (extends `internal/core/doctor.go`)
2. **Allocation guard** — pre-write uniqueness guard making
   `CreateArtifact`/`NextID` authoritative over the canonical queue+archive
   filesystem; regenerate or fail on a resolved-ID collision. (`artifacts.go`,
   helper in `naming.go`) — reuses the canonical-ID scanner introduced in (1).
3. **Archive safety** — `ArchiveItem` refuses to overwrite a *different*
   existing archive destination; returns an actionable collision error;
   preserves the existing same-path in-place behavior. (`archive.go`)
4. **Rehydrate integrity** — detect and warn when two source files map to the
   same ID during `Rehydrate`, without disturbing the transaction boundary.
   (`internal/db/rehydration.go`)
5. **Durable counter (risk-flagged, last)** — persist a per-type high-water-mark
   consulted by allocation so archived ordinals are not reused; depends on (2).
6. **TDD repro (integration)** — place two same-ID files across queue+archive,
   sync, and assert: doctor flags it, create will not reuse the ID, and
   archiving the queue copy fails clearly instead of overwriting.

## Manifest Drift Decision (060 / 061 / 062)

Live evidence verified this session:

| ID | Title | Status | `custom_fields.items` |
|---|---|---|---|
| `060-S` | Shipment State Integrity | queued | `["061-F","061.002-T","061.001-T"]` |
| `061-S` | Metadata and Section Sync Integrity | queued | `["062-F","062.001-T"…"062.005-T"]` |
| `061-F` | Shipment State Integrity | queued | (title matches 060-S) |
| `062-F` | Metadata and Section Sync Integrity | queued | (title matches 061-S) |
| `060-F` | Archive and Hierarchy Rollback Integrity | archived/done | — |

Each shipment's `items`/title is shifted by one feature (an off-by-one drift in
manifest membership and naming).

**Decision: route the data repair to a SEPARATE follow-up stash entry; do NOT
include it as a task in this feature, and do NOT mutate the manifests in this
Stage run.** Rationale:

- **Different problem class.** This feature is *code hardening* to prevent and
  detect future root-ID conflicts. The drift is a *one-time data repair* of
  three existing, already-`queued` shipment manifests. Bundling them violates
  width isolation (Constitution: single skill domain per unit) and mixes a
  code-fix shipment with live-state mutation.
- **Different risk profile.** Re-aligning `060-S`/`061-S` `items` and titles
  mutates live shipment state that the Ship agent may already be scheduling; it
  warrants its own plan, its own review gate, and possibly Ship-executed
  application — not a side-effect of a code feature.
- **Stage role boundary (P-010).** Stage must not mutate the 060/061/062
  manifests itself. A reviewed data-repair task is possible, but it belongs to
  its own work item so it can be planned and reviewed independently.
- **Complementary, not coupled.** Detection unit (1) of *this* feature will help
  *surface* such drift going forward, but it does not repair existing drift.

Action taken: a new high-priority bug stash entry — **`B8FF7590`** (kind=bug,
priority=high) — was created describing the drift precisely and
cross-referencing this deliberation and stash `0F65FBC9`. **Recommendation
flagged for the operator either way.** Stage did not mutate the 060/061/062
manifests.

## Rejected Alternatives

- **Option C (detect-only):** leaves the data-loss path open — rejected.
- **Repairing 060/061/062 inside this feature:** rejected for width-isolation
  and blast-radius reasons (see Manifest Drift Decision).
- **Committed durable counter as the primary fix:** rejected as the *primary*
  mechanism because of cross-branch visibility and merge-conflict concerns; kept
  only as a risk-flagged, dependency-gated final task.

## Unresolved Questions

1. **Durable counter semantics (for operator/executor):** should the counter
   file be Git-committed (visible but merge-conflict-prone) or local-only
   (conflict-free but blind to other branches)? This must be resolved before
   building unit 5, or unit 5 should be deferred.
2. **Archive collision policy:** on refusal, do we (a) hard-fail with an
   actionable error only, or (b) also quarantine the conflicting file to a
   `.backlogit/quarantine/` path? Plan assumes (a) hard-fail first; quarantine
   is an optional follow-up.
3. **Manifest-drift repair ownership:** should the follow-up be Stage-planned +
   Ship-executed, or executed directly by the operator given it is pure data?

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| Pre-write guard adds a filesystem scan to the hot create path | Scan the full canonical `artifactSearchDirs(ws)` set (parity with the doctor audit, to prevent detection/prevention divergence across registry-routed dirs), reuse the doctor walk helper, and guard at the single post-resolution chokepoint covering both the hierarchical-root and standalone allocators |
| Archive refusal breaks the legitimate same-path in-place archive | Unit 3 must preserve the existing `currentPath == archivePath` handling and only refuse on a *different* item sharing the filename |
| Rehydrate change risks the atomic transaction (prior incident) | Unit 4 adds detection at collection time only; transaction boundary untouched; regression test asserts row count preserved |
| Durable counter diverges from canonical files | Seed/repair the counter from the canonical max across all `artifactSearchDirs` (queue + archive + routed dirs); doctor audit can reconcile |
| Over-scoping into the 060/061/062 repair | Explicitly routed to a separate stash entry; Stage does not mutate manifests |
