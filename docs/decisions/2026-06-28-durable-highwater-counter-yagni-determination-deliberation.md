---
chunk_strategy: h1-h2-h3
description: Determination on whether a durable per-artifact-type high-water-mark counter is genuinely additive over the shipped 066-S canonical pre-write uniqueness guard, or premature abstraction (stash C55C5158)
doc_type: decision
docline:
    decision_status: decided
    depth: deep
    linked_artifacts:
        - docs/decisions/2026-06-23-root-id-conflict-integrity-deliberation.md
        - docs/exec-plans/2026-06-23-root-id-conflict-integrity-plan.md
    promoted_to: none
    stash_ids:
        - C55C5158
    tags:
        - id-allocation
        - archive-safety
        - data-integrity
        - yagni
        - premature-abstraction
        - high-water-mark
    topic: Is a durable per-type high-water-mark counter genuinely additive over the shipped canonical pre-write guard, or YAGNI? (stash C55C5158)
ingested_at: "2026-06-28T20:43:41Z"
schema_version: "1.0"
source: docs/decisions/2026-06-28-durable-highwater-counter-yagni-determination-deliberation.md
title: 'Durable high-water-mark counter: YAGNI determination over the shipped 066-S canonical pre-write guard'
---

## Problem Frame

Stash `C55C5158` (design-gated enhancement, medium) proposes a **durable
per-artifact-type high-water-mark counter** — a persistent file under
`.backlogit/` recording the highest ordinal ever allocated per artifact type — so
that an archived ordinal can never be reused even when the file that carries it is
"temporarily out of the index's view." It was split out of the 066-S
root-ID-conflict-integrity feature during Stage plan-review on 2026-06-23 as the
explicitly highest-blast-radius, design-incomplete "belt-and-suspenders" element,
and carried forward with a **blocking operator design decision**:

* **Gate 2 (persistence model):** Git-committed counter (visible across clones but
  merge-conflict-prone on every allocation, still branch-local until merge) vs.
  local-only counter (conflict-free but blind to other branches).

The 066-S parent deliberation (`docs/decisions/2026-06-23-root-id-conflict-integrity-deliberation.md`,
Finding 6) and its plan
(`docs/exec-plans/2026-06-23-root-id-conflict-integrity-plan.md`, Decisions and
Risks) both flagged that the durable counter's marginal value is narrow and its
design non-trivial. The stash itself imposes a **gating reviewer constraint that
must be answered before any persistence-model debate**:

> Confirm it is genuinely additive over a single-pass canonical-max scan before
> building; document the precise out-of-view window it protects (avoid premature
> abstraction).

This determination answers that gate **first** (Gate 1), grounded in the current
shipped code, and treats Gate 2 as conditional on a "build" outcome.

**Who cares:** harness operators and the Ship agent rely on stable, unique root
IDs and on a non-destructive, audit-preserving archive. The acute failure (silent
overwrite of a distinct archived item sharing a filename) was the original 066-F
data-loss path.

**Constraints:** Go 1.24, test-first (Constitution II), Git-mergeable file state
with minimal merge-conflict surface (Constitution IX), minimal new dependencies
and abstractions (Constitution VI / YAGNI), 2-hour / single-skill-domain task
granularity.

**Success criteria for THIS determination:**

1. State, with code citations, the exact window (if any) a durable counter would
   uniquely close that the shipped 066-S canonical pre-write guard does not.
2. Decide whether that window justifies building the counter now, or whether the
   counter is premature abstraction (YAGNI) given the acute paths are already
   closed.
3. If building, resolve Gate 2 (persistence model). If not, settle the question
   thoroughly so it is not re-raised, and archive `C55C5158` with rationale.

**Scope boundary (non-goals):** this is not a re-litigation of the 066-S guards
(shipped, merged in PR #132); not a perf review of the canonical scan (tracked
separately as stash `D6B44FF6`); not a cross-branch merge-policy redesign.

## Research Findings (grounded in the current shipped code)

All findings verified against the working tree at `main` @ `03e73637` (066-S
merged via PR #132).

### Finding 1 — Allocation derives the ordinal from the index, then a fresh filesystem guard overrides it

`core.NextID` (`internal/core/naming.go:55-77`) runs
`SELECT id FROM items WHERE artifact_type = ?` and returns `MAX(ordinal)+1` from
the **SQLite index only**. Top-level roots are minted via
`NextTypedHierarchicalID`; standalone types fall back to `NextID`
(`internal/core/artifacts.go:135-157`). The index can lag the filesystem — this
is the exact window the stash describes ("archived ordinals temporarily out of the
index's view").

**But allocation does not stop at the index.** Immediately after resolving the ID,
`CreateArtifact` runs a **pre-write canonical uniqueness guard**
(`internal/core/artifacts.go:159-172`):

```go
canonical, scanErr := scanCanonicalArtifacts(ws)   // full filesystem scan
...
if existing := canonical[artifactID]; len(existing) > 0 {
    return nil, fmt.Errorf("create artifact %q: %w", artifactID, blerrors.ErrIDCollision)
}
```

A second defense stats the concrete destination path and also fails loud if it is
occupied — covering even unparseable/mismatched files the parsed scan would skip
(`internal/core/artifacts.go:279-290`).

### Finding 2 — The canonical scan ALWAYS sees the archive, by construction

`scanCanonicalArtifacts` (`internal/core/canonical_scan.go:38-116`) walks the full
`artifactSearchDirs(ws)` set and **force-includes the fixed
`.backlogit/archive` directory even when the registry does not route it**
(`canonical_scan.go:44-65`):

```go
// Force the canonical archive directory into the scan set so the collision
// guard and doctor audit never go blind to already-archived IDs.
archiveDir := filepath.Join(WorkspaceStorageRoot(ws.RootPath), "archive")
... if !archivePresent { dirs = append(dirs, archiveDir) }
```

`artifactSearchDirs` (`internal/core/artifacts.go:523-569`) composes
**registry-routed dirs + queue-layout root**, and the archive force-include
guarantees the archive is never dropped. This is exactly the
queue + archive + routed-dirs surface the stash says the counter must be
"seeded/reconciled from."

**Consequence:** within a single workspace, an archived ordinal is *never* out of
the allocator's pre-write view, regardless of SQLite index state. The "index lag"
window is closed at allocation time by a live filesystem scan, which **fails loud**
rather than reusing.

### Finding 3 — Archive overwrite is already refused; collisions are detected and warned

* `ArchiveItem` refuses to overwrite a *distinct* occupied destination and returns
  `ErrArchiveDestinationOccupied` (066.003-T; pinned by
  `internal/core/066_archive_refusal_test.go`). The acute, lossy path (silent
  overwrite of a different archived item sharing a filename) is closed.
* `doctor` emits a distinguishable `FindingRootIDCollision` for a level-1 ID
  present in both queue and archive (066.001-T;
  `internal/core/066_doctor_rootid_collision_test.go`).
* `Rehydrate` warns on duplicate source IDs instead of silently collapsing them
  (066.004-T).

### Finding 4 — The acute paths are pinned by shipped tests

* `TestCreateArtifact_RefusesCanonicalIDCollision`
  (`internal/core/066_create_guard_test.go`) — create fails loud with
  `ErrIDCollision` when the canonical queue+archive filesystem already holds the
  resolved ID.
* `TestCreateArtifact_NoCollisionPathUnchanged` — normal sequential allocation
  still advances collision-free.
* `internal/core/canonical_scan_test.go:144` — explicitly asserts that an archived
  `003-F` is discovered "even when registry.yaml does not route the archive dir, or
  the collision guard goes blind to the archive."

So the precise window the stash names — *archived ordinal not visible to the index
at allocation* — is already covered and regression-tested. Verification here was
done by reading the shipped code and its tests (the behavior is already pinned), so
no throwaway spike prototype was warranted; a spike would only re-assert
`canonical_scan_test.go:144` and `066_create_guard_test.go`.

### Finding 5 — A counter is, at best, a cache of the canonical max it would be seeded from

The stash's own reviewer constraint requires the counter to be
"seeded/reconciled from the canonical max across ALL `artifactSearchDirs`" and to
"degrade safely (fall back to canonical scan) if missing/stale." That is precisely
what the create guard already computes **fresh on every allocation**. A durable
counter would therefore be a *cached copy* of a value the guard already derives
live — it can never see more than the scan, because both are branch-local snapshots
of the same Git working tree.

## The precise out-of-view window analysis (Gate 1 core)

There are exactly two candidate windows. Neither justifies the counter.

### Window A — same workspace, stale SQLite index (the window the stash names)

* **Is it real?** The index *can* lag the archive (e.g., a Rehydrate that has not
  yet run, or a cache built without the archive).
* **Does the counter uniquely close it?** **No.** The shipped canonical scan
  (Findings 1–2, force-including the archive) already sees the archived ordinal and
  **fails loud with `ErrIDCollision`** at allocation. The only behavioral
  difference a counter would introduce is *auto-advancing* past the high-water-mark
  to a fresh ordinal instead of failing loud. That is a **UX/retry-ergonomics
  preference, not a data-integrity gain** — and it directly contradicts the shipped,
  deliberately chosen "fail-loud, not auto-regenerate" design
  (`...-plan.md`, Decisions and Rationale). The acute data-loss path (silent
  overwrite) is closed either way.

### Window B — cross-branch / not-yet-merged (the only window beyond the scan)

* **Is it real?** Yes — a file created on branch B is absent from branch A's
  working tree, so branch A's canonical scan cannot see it. This is the *only*
  window genuinely outside the canonical scan.
* **Does a Git-committed counter close it?** **No, not pre-merge.** Branch B's
  counter bump lives in branch B's commit; branch A does not see it until merge —
  the committed counter is **branch-local exactly like the canonical files**. It
  provides *no better* cross-branch visibility than scanning that same branch. At
  merge time, the counter file itself becomes a per-allocation merge-conflict
  surface (Constitution IX regression), and the colliding canonical files are
  already caught by the shipped **detect (doctor) + refuse (create guard / archive
  refusal) + warn (rehydrate)** trio.
* **Does a local-only counter close it?** **No.** Never committed ⇒ blind to all
  other branches *and* empty on a fresh clone ⇒ it must be re-seeded from the
  canonical max — the very scan it was meant to replace.

**Therefore NEITHER persistence model closes the only window (B) that would justify
the counter, and Window A is already closed by the shipped guard.** The counter
adds new persistent state on the central allocation hot path, a merge-conflict
surface or clone-blindness, and a seed/reconcile obligation that re-derives the
canonical max — in exchange for, at most, swapping a deterministic fail-loud for a
silent auto-advance. That is textbook premature abstraction.

## Options Evaluated

### Option A — Do not build; archive C55C5158 as YAGNI (recommended)

Document the determination thoroughly (this artifact) so the question is settled,
then archive the stash entry with a consumed/won't-do rationale. No code change, no
shipment.

* **Pros:** Honors the stash's own gating constraint (confirm additive before
  building). Avoids new persistent state on the allocation hot path, avoids the
  Constitution IX merge-conflict surface, avoids coupling allocation correctness to
  a cache that must be reconciled from the canonical max anyway. The acute paths
  remain closed and tested by 066-S.
* **Cons:** Same-workspace stale-index collisions surface as a fail-loud
  `ErrIDCollision` the operator must retry past, rather than an automatic advance.
  (This is the intended, shipped behavior — not a regression.)
* **Effort:** none. **Fit:** high.

### Option B — Build a local-only durable counter

Persist a per-type high-water-mark under `.backlogit/`, not committed.

* **Pros:** Conflict-free; could auto-advance in the stale-index window.
* **Cons:** Blind to other branches (does nothing for Window B); empty on clone ⇒
  must seed from the canonical max (redundant with the live scan); adds persistent
  state and a reconcile path on the hot allocation chokepoint; converts a
  deterministic fail-loud into a silent advance, reducing testability/determinism.
  Closes no window the scan leaves open.
* **Effort:** high. **Fit:** low.

### Option C — Build a Git-committed durable counter

Persist and commit the per-type high-water-mark.

* **Pros:** Visible across clones after merge.
* **Cons:** Still branch-local until merge ⇒ does NOT close Window B pre-merge;
  introduces a per-allocation merge-conflict surface (Constitution IX) on the
  highest-churn path in the system; at merge, redundant with the canonical scan +
  collision guards that already fire. Highest blast radius for the least marginal
  value.
* **Effort:** high. **Fit:** very low.

## Trade-off Comparison

| Criterion | A — Do not build (YAGNI) | B — Local-only counter | C — Committed counter |
|---|---|---|---|
| Closes same-ws stale-index reuse | Already closed (fail-loud) | "Closes" via auto-advance (UX only) | Same (UX only) |
| Closes cross-branch window (B) | No (no model can pre-merge) | No | No (branch-local until merge) |
| Acute silent-overwrite path | Closed by 066-S | Closed by 066-S | Closed by 066-S |
| New persistent state on hot path | None | Yes | Yes |
| Constitution IX merge-conflict surface | None | None | Elevated (per allocation) |
| Determinism / testability of allocation | Preserved (fail-loud) | Reduced (silent advance) | Reduced (silent advance) |
| Marginal integrity value over scan | None | None | None |
| Premature-abstraction risk | Avoided | High | High |

## Decision

**DO NOT BUILD. The durable per-artifact-type high-water-mark counter is YAGNI
relative to the shipped 066-S canonical pre-write uniqueness guard. Archive
`C55C5158` as consumed (won't-do), with this artifact as the settled rationale.**

**Gate 1 determination — NOT genuinely additive.** The counter closes no window
that the shipped guard leaves open:

* The same-workspace "archived ordinal out of the index's view" window the stash
  names is **already closed** by the live canonical scan, which force-includes the
  archive (`canonical_scan.go:44-65`) and fails loud with `ErrIDCollision`
  (`artifacts.go:159-172`), pinned by `066_create_guard_test.go` and
  `canonical_scan_test.go:144`.
* The only window genuinely beyond the canonical scan is **cross-branch /
  not-yet-merged**, and that window is closed by **neither** persistence model
  pre-merge (committed is branch-local until merge; local-only is blind to other
  branches). At merge, the existing detect + refuse + warn trio handles collisions.
* The counter's sole behavioral effect would be to convert a deterministic
  fail-loud into a silent auto-advance in the stale-index window — a UX preference
  that *regresses* the deliberately chosen fail-loud design, in exchange for new
  hot-path persistent state and (for the committed variant) a Constitution IX
  merge-conflict surface.

**Gate 2 determination — MOOT.** Because Gate 1 resolves to "do not build," the
persistence-model decision (Git-committed vs. local-only) does not need to be made.
For completeness, the analysis above shows the decision would be unresolvable on its
own terms: the cross-branch visibility the committed variant is meant to buy does
not materialize pre-merge, so neither model can justify the cost. There is no
persistence model that makes the counter additive.

## Rejected Alternatives

* **Option B (local-only counter):** rejected — blind to other branches, redundant
  with the canonical max it must be seeded from, regresses fail-loud determinism for
  no integrity gain.
* **Option C (committed counter):** rejected — branch-local until merge (does not
  close Window B), adds a per-allocation merge-conflict surface (Constitution IX),
  redundant with the scan + collision guards at merge time.
* **Re-opening the 066-S guards:** out of scope; shipped and tested in PR #132.

## Unresolved Questions

1. **Audit-stable monotonic ordinals as a distinct product requirement.** This
   determination addresses *integrity* (preventing reuse of archived ordinals that
   are still on disk and therefore scanned). If a future, *different* requirement
   emerges — "ordinals must be globally monotonic and never reused even after a hard
   `delete --force`, for external audit/traceability reasons" — that is a separate
   problem class (a product/UX guarantee, not an integrity gap) and should be
   re-opened under that explicit framing, not via this stash. Today no such
   requirement exists; hard-deleting an item intentionally frees its ordinal, and
   there is no surviving file to collide with.
2. **Hot-path scan cost** of the per-create canonical scan is already tracked
   separately as stash `D6B44FF6` (066-S review P2). A counter would *add* writes,
   not remove the scan, so it is not a perf remedy.

## Risks and Mitigations

| Risk | Mitigation |
|---|---|
| Question gets re-raised later from a different angle | This artifact records the code-grounded Gate 1 analysis and the precise reason no persistence model is additive; it is linked from the 066-S deliberation/plan lineage and from the archived stash entry. |
| A future workflow genuinely needs auto-advance-on-collision UX | Track as a new, narrowly-scoped UX stash if/when it arises; it does not require a durable counter — the create guard already knows the colliding ID and could advance in-process. Keep that decision separate from this integrity determination. |
| Cross-branch collisions at merge | Already handled by the shipped detect (doctor `FindingRootIDCollision`) + refuse (`ErrIDCollision`, `ErrArchiveDestinationOccupied`) + warn (rehydrate duplicate-source) trio. No new mechanism needed. |
