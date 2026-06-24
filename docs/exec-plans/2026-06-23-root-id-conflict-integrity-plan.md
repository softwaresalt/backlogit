---
title: "Root-ID Conflict Integrity: Detection, Allocation, and Archive Safety"
date: 2026-06-23
origin: "docs/decisions/2026-06-23-root-id-conflict-integrity-deliberation.md"
status: reviewed
stash_ids:
  - "0F65FBC9"
related_stash:
  - "B8FF7590"
  - "C55C5158"
---

## Problem Frame

Top-level work items can be minted with a duplicate root ID — one in
`.backlogit/queue/`, one in `.backlogit/archive/` — with no error at creation.
The collision is invisible to every index-based check because `items.id` is a
PRIMARY KEY and `Rehydrate` collapses duplicate source files via
`INSERT OR REPLACE` (`internal/db/rehydration.go`). It first becomes *destructive*
at archive time: `ArchiveItem` (`internal/core/archive.go:146-166`) `tmp+rename`s
onto `archive/<basename>`, overwriting any different archived item that shares
the filename.

Three code paths must change, plus one detection surface and one regression net:

* `internal/core/naming.go` — `NextID` reads only the index, no canonical guard.
* `internal/core/artifacts.go` — `CreateArtifact` writes the resolved ID without
  a pre-write filesystem uniqueness check.
* `internal/core/archive.go` — `ArchiveItem` overwrites archive destinations.
* `internal/db/rehydration.go` — silent `INSERT OR REPLACE` collapse.
* `internal/core/doctor.go` — file-based duplicate audit exists but lacks an
  explicit level-1 root-ID collision finding and default CLI surfacing.

## Requirements Trace

| # | Requirement (from stash 0F65FBC9 / deliberation) | Origin | Unit |
|---|---|---|---|
| R1 | Detect duplicate root IDs + level-1 root-ID collisions from canonical queue+archive files (not the PK-collapsed index) | Fix #1 | U1 |
| R2 | Make `NextID`/`CreateArtifact` authoritative over canonical source; pre-write uniqueness guard that regenerates or fails on a resolved-ID collision | Fix #2 | U2 |
| R3 | `ArchiveItem` refuses to overwrite a *different* existing archive destination; actionable collision error; same-path in-place behavior preserved | Fix #4 | U3 |
| R4 | `Rehydrate` detects and warns when two source files map to one ID, without disturbing the atomic transaction | Fix #5 | U4 |
| R5 | Durable per-type high-water-mark so archived ordinals are not reused even when out of view | Fix #3 | **Externalized** → stash `C55C5158` (design-gated; see Plan Review) |
| R6 | Reproducing test: two same-ID files across queue+archive → doctor flags, create won't reuse, archive fails clearly | Fix #6 | U6 |

**Foundational decision (applies to U2/U3/U6): typed sentinel errors.** Define
exported sentinels — `ErrIDCollision` (resolved ID already exists on the
canonical filesystem) and `ErrArchiveDestinationOccupied` (archive destination
already holds a *different* item) — in the existing `internal/errors` package,
wrap with `fmt.Errorf("context: %w", err)`, and have U6 assert via `errors.Is`
rather than brittle string matching (Constitution I sentinel-error requirement).

## Scope Boundaries

### In Scope

* Canonical-file root-ID collision detection (extend existing doctor audit).
* Pre-write ID uniqueness guard on the create path (single chokepoint, fail-loud).
* Archive overwrite refusal.
* Rehydrate duplicate-source warning.
* Typed sentinel errors for the new collision/refusal failure modes.
* End-to-end reproducing integration test.

### Non-Goals

* **Durable per-type high-water-mark counter** — externalized to stash
  `C55C5158`. The acute data-loss/silent-reuse paths are closed without it by U2
  + U3; it is design-gated on an unresolved persistence-model decision.
* Repairing the existing 060/061/062 shipment-manifest drift (routed to stash
  `B8FF7590`).
* Redesigning the flat archive directory layout.
* Cross-branch merge-time ID reconciliation policy beyond detect + guard.
* Quarantine-on-collision UX (optional follow-up; see U3 acceptance).

## Implementation Units

### Unit 1: Canonical root-ID collision doctor audit + shared scanner

**Files:** `internal/core/canonical_scan.go` (new, neutrally-named), `internal/core/doctor.go`
**Test files:** `internal/core/canonical_scan_test.go`, `internal/core/doctor_*_test.go`
**Effort size:** medium **Skill domain:** code **Execution posture:** test-first
**Dependencies:** none

The existing `Doctor` with `CheckDuplicates` already walks canonical `.md` files
across `artifactSearchDirs(ws)` (a **recursive** `filepath.WalkDir` over queue +
archive + registry-routed dirs) and already emits `FindingDuplicateID` by default
(`cli/doctor.go` sets `--check-duplicates` default `true`). **Correction vs. the
original framing:** detection is NOT gated behind a rarely-set flag, so no CLI
default-flip (and no `cmd/` change) is needed — this keeps U1 inside `core`
(width isolation preserved).

This unit therefore: (a) extracts a reusable, **recursive** canonical-ID scanner
into a neutral file `internal/core/canonical_scan.go` returning
`map[string][]artifactRef` where `artifactRef` carries the file path **plus the
already-parsed fields** (id, artifactType, parentID, status, level) so a single
walk serves the doctor's orphan + duplicate checks and U2's allocation guard
without re-parsing; (b) refactors `Doctor` to consume the shared scanner; (c)
adds an explicit level-1 **root-ID collision** signal — implemented either as a
distinct `FindingRootIDCollision` finding **or** as enriched metadata on the
existing `FindingDuplicateID` (implementer chooses the lower-maintenance option,
but the collision-between-queue-and-archive-at-level-1 case MUST be explicitly
distinguishable in the report).

**Acceptance criteria**
* A failing test is authored and observed red before implementation (Constitution II).
* A reusable recursive canonical-ID scanner exists in `core/canonical_scan.go`,
  returns parsed `artifactRef`s, and is unit-tested.
* `Doctor` reports a distinguishable root-ID-collision signal when the same
  level-1 ID exists in both queue and archive.
* Pre-existing `FindingDuplicateID`, orphan, and CLI-default behavior is
  unchanged (regression test green).
* `go test ./internal/core/...` passes.

### Unit 2: Pre-write ID uniqueness guard in CreateArtifact

**Files:** `internal/core/artifacts.go`, `internal/core/naming.go`, `internal/errors` (sentinel)
**Test files:** `internal/core/*_test.go` (create-guard cases)
**Effort size:** medium **Skill domain:** code **Execution posture:** test-first
**Dependencies:** Unit 1 (consumes the shared canonical scanner)

**Single chokepoint (P1 fix).** Top-level root items are minted via
`NextTypedHierarchicalID(parentID="")` (`artifacts.go:137-148`), NOT via `NextID`
— `NextID`/`ResolveName` only fires for types with no hierarchy level
(`artifacts.go:151-157`). To cover the exact bug site (the hierarchical root
path) **and** the standalone path, place the guard **once** after `artifactID`
is fully resolved (between `artifacts.go:157` and the file write at ~`:264`), so
both resolution branches are guarded by one check.

**Full canonical scope (P1 fix).** Scan the **full `artifactSearchDirs(ws)` set**
(parity with U1/doctor detection), not just queue+archive — status routing can
place a top-level item in other registry dirs (e.g. `review`), and detection and
prevention must not disagree (protected invariant #4).

**Deterministic fail-loud (P2 fix).** Behavior is pinned, not implementer's
choice: compute the canonical max ordinal once from the shared scanner and let
normal allocation advance past it for **whichever allocator is active** — both
the hierarchical-root path (`NextTypedHierarchicalID`) and the standalone path
(`NextID`) — i.e. `next = max(DB max, canonical max) + 1`, single pass, no
re-scan loop. If the *resolved* ID nonetheless already exists as a canonical
file (the cross-window case), **fail loud** with `ErrIDCollision` rather than
silently regenerating. The sentinel is the safety backstop regardless of which
allocator produced the ID. Auto-regeneration is explicitly out of scope.

**Residual TOCTOU (acknowledged).** Scan+write is not atomic, so a race between
two concurrent `CreateArtifact` calls remains theoretically possible; this guard
closes the single-synced-workspace reuse path but is NOT claimed to be a full
serialization mechanism. Closing the concurrent-create race is deferred to the
externalized durable-counter work (stash `C55C5158`) or a future workspace lock.

**Acceptance criteria**
* A failing test is authored and observed red before implementation.
* The guard is a single check after ID resolution, covering both the
  hierarchical-root and standalone allocation branches.
* The canonical scan covers the full `artifactSearchDirs` set.
* A resolved-ID collision returns `ErrIDCollision` (assertable via `errors.Is`);
  no silent reuse.
* Normal create path (no collision) is unchanged and remains a single-pass
  allocation.
* `go test ./internal/core/...` passes.

### Unit 3: Archive overwrite refusal in ArchiveItem

**Files:** `internal/core/archive.go`, `internal/errors` (sentinel)
**Test files:** `internal/core/*archive*_test.go`
**Effort size:** medium **Skill domain:** code **Execution posture:** test-first
**Dependencies:** none

**Hoist before hooks (P2 fix).** `archivePath` is fully determined from
`currentPath` by `archive.go:107`, before the pre-archive hooks fire
(`:124-138`). Compute `archivePath` and run the collision check **before**
`FirePre` so a refusal does not leave pre-archive hook side effects executed for
an archive that never completes.

**Discriminator is PATH equality, not ID equality (correctness fix).** The bug
scenario is two *different* top-level items sharing the same root ID — e.g.
`queue/X.md` (item B) and `archive/X.md` (item A). When archiving B, the
destination `archive/X.md` carries the same frontmatter id `X`, so an
ID-equality discriminator would mis-classify the foreign collision as an
"in-place" update and overwrite A — the exact data loss this unit prevents. The
existing code already keys idempotent archive on **path** equality
(`archive.go:161`). Therefore:

* `filepath.Clean(archivePath) == filepath.Clean(currentPath)` → legitimate
  in-place / terminal-status update; proceed and reuse the already-parsed `fm`
  (`:110-117`). (Preserves `:99-107, 157-166`.)
* `archivePath` exists AND is a **distinct file** from `currentPath` → a foreign
  item occupies the destination → **refuse** with `ErrArchiveDestinationOccupied`;
  do not write; leave the existing archive file intact. Refuse-on-parse-error of
  the occupying file is conservative by design (data-loss prevention).
* Narrow exception — the existing 060.002-T same-item half-archive recovery
  (where a prior interrupted archive left a copy of the **same logical item**):
  only allow the overwrite when the occupying file matches the same item by id
  **and** content/title; if the id matches but content differs (the
  duplicate-root-ID bug), refuse. Cover this exception explicitly with a test so
  the recovery path is not broken by the new guard.

**Acceptance criteria**
* A failing test is authored and observed red before implementation.
* Archiving a queue item whose basename collides with a *different* archived
  item returns `ErrArchiveDestinationOccupied` (assertable via `errors.Is`) and
  leaves the existing archive file intact.
* The check runs before pre-archive hooks; no partial hook side effects on refusal.
* The legitimate same-path in-place archive (terminal-status routing) still works.
* The cascade and DB-rollback paths remain consistent on refusal.
* `go test ./internal/core/...` passes.

### Unit 4: Rehydrate duplicate-source warning

**Files:** `internal/db/rehydration.go`
**Test files:** `internal/db/rehydration_*_test.go`
**Effort size:** small **Skill domain:** code **Execution posture:** test-first
**Dependencies:** none

During the collection phase (`rehydration.go:51-81`), maintain a local
`map[string]string` (id → first source path) inside the walk closure and emit
exactly one structured `slog.Warn` (id + both paths) on the second occurrence of
an ID, instead of silently collapsing via `INSERT OR REPLACE`. `collectedArtifact`
currently stores only the parsed artifact, so the path is tracked in the closure
map rather than by widening the struct. The atomic clear+rebuild transaction
boundary (`rehydration.go:83-160`) MUST remain untouched (per
`docs/compound/database-issues/atomic-rehydration-sqlite-transaction-2026-04-08.md`).
This unit stays entirely within `internal/db` and does NOT import `core` (the
deliberate `core → db` layering / cycle-avoidance at `rehydration.go:195-197`).

**Acceptance criteria**
* A failing test is authored and observed red before implementation.
* Rehydrating a workspace with two same-ID source files logs exactly one
  structured warning naming both paths.
* The transaction boundary is unchanged; the indexed row count after rehydrate
  matches the **pre-fix collapse behavior** (one surviving row per ID — the
  warning does not change collapse semantics, only surfaces it).
* `go test ./internal/db/...` passes.

### Unit 6: End-to-end reproducing integration test

**Files:** `tests/` (new integration test)
**Test files:** the test itself
**Effort size:** medium **Skill domain:** tests **Execution posture:**
repro-first (authored red before/alongside U1)
**Dependencies:** Unit 1, Unit 2, Unit 3

Place two `.md` files sharing a top-level ID — one in queue, one in archive —
sync, then assert: (1) `doctor` flags the root-ID collision; (2) `CreateArtifact`
for that type does not reuse the ID and returns `ErrIDCollision` on a forced
collision; (3) archiving the queue copy fails with `ErrArchiveDestinationOccupied`
instead of overwriting the archived file. Author the failing repro before/
alongside U1 so the end-to-end red phase is observed in the working tree, not by
a later manual `git checkout`.

**Acceptance criteria**
* The integration test reproduces the scenario and asserts all three behaviors
  via `errors.Is` (no brittle string matching).
* The test fails against pre-fix `main` (red) and passes after U1–U3 (green).
* `go test ./tests/...` passes.

## Dependency Graph

```text
U1 (detection + shared scanner) ──▶ U2 (create guard, fail-loud)
U3 (archive refusal)         (independent)
U4 (rehydrate warning)       (independent)
U1, U2, U3 ──▶ U6 (integration repro)
```

No cycles. Suggested order: author U6 repro red first, then U1 → U2, with U3/U4
in parallel, then land U6 green. Externalized: durable counter (stash
`C55C5158`) is not in this graph.

## Decisions and Rationale

* **Extend, not rebuild, detection.** The file-based audit already exists in
  `doctor.go` (and already runs by default); rebuilding a new package would
  duplicate the walk and registry-aware search-dir logic. Rationale: minimal
  change, Constitution VI.
* **Pre-write filesystem guard is the primary reuse defense.** It closes the
  single-synced-workspace silent-reuse path; the externalized durable counter is
  belt-and-suspenders for the narrow out-of-view window.
* **Single guard chokepoint over the full canonical surface.** One check after
  ID resolution covers both the hierarchical-root and standalone branches and is
  scoped to the same `artifactSearchDirs` set the doctor audits, so detection and
  prevention cannot disagree.
* **Fail-loud, not auto-regenerate.** A resolved-ID collision returns a typed
  sentinel rather than silently advancing — deterministic and testable.
* **Typed sentinel errors.** New known failures get sentinels (`ErrIDCollision`,
  `ErrArchiveDestinationOccupied`) so callers and tests use `errors.Is`
  (Constitution I).
* **Archive refusal over silent overwrite, hoisted before hooks.** Data loss is
  unacceptable for a Git-tracked backlog; the check runs before pre-archive hooks
  to avoid partial side effects.
* **Rehydrate: detect at collection, not at insert.** Keeps the proven atomic
  transaction boundary intact (prior incident learning); stays within `db` (no
  `core` import).
* **Durable counter externalized (stash `C55C5158`).** Highest blast radius,
  design-unresolved, and unable to fully close the cross-branch window by the
  plan's own analysis; the plan-review gate recommended removing it from the
  critical path. Kept as a tracked follow-up pending the operator's
  persistence-model decision.

## Risks and Caveats

* **Hot-path cost (U2):** the pre-write scan adds I/O to create. Mitigation:
  reuse U1's single recursive walk + parsed `artifactRef`s (no re-parse), compute
  canonical max once, single-pass allocation.
* **TOCTOU on create (U2):** scan+write is not atomic; concurrent creates can
  still race. Mitigation: documented as a residual; the guard is not claimed to
  serialize; the concurrent-create race is deferred to the externalized counter
  (`C55C5158`) or a future workspace lock.
* **Same-path regression (U3):** must not break terminal-status in-place
  archiving. Mitigation: compare IDs, preserve existing same-path branch, reuse
  parsed `fm`, cover with a regression test.
* **Transaction regression (U4):** prior incident showed a crash mid-walk can
  empty the index. Mitigation: detection only at collection phase; transaction
  untouched; assert row count matches pre-fix collapse behavior.
* **Detection/prevention divergence:** Mitigation — U2 scans the same
  `artifactSearchDirs` set the doctor audits.
* **Over-scoping into the 060/061/062 repair or the durable counter:** both
  explicitly externalized (`B8FF7590`, `C55C5158`); Stage does not mutate
  manifests.

## Plan Hardening Signals (REQUIRED)

* public API, schema, or contract change — **absent** (internal helpers; CLI
  `doctor` output gains a finding type, additive).
* security, auth, permission, or compliance-sensitive behavior — **absent**.
* migration, backfill, destructive data/config action, or irreversible step —
  **present**: U3 changes archive write semantics (today an overwrite =
  irreversible data loss). Archive is a destructive surface. (The durable
  counter's persistent-state risk is externalized to stash `C55C5158`.)
* external integration, operator checkpoint, or external dependency —
  **present**: the externalized durable-counter follow-up carries an unresolved
  operator design decision (counter persistence model); it is recorded as an
  operator checkpoint even though it is no longer a unit of this plan.
* high runtime, rollout, or rollback risk — **present**: `NextID`/`CreateArtifact`
  and `ArchiveItem` are central, high-blast-radius allocation/lifecycle paths.

Requires plan hardening: yes

## Runtime Verification and Closure

| Unit | Runtime surface | Verification before "absorbed" | Closure artifact |
|---|---|---|---|
| U1 | `doctor` CLI | Run `backlogit doctor` against a workspace with a seeded duplicate; confirm the root-ID collision finding appears | Doctor output sample in PR / test fixture |
| U2 | create (CLI + MCP) | Attempt create against a seeded canonical collision; confirm no reuse | Unit test + manual `backlogit add` probe |
| U3 | archive (CLI + MCP) | Attempt archive into a colliding basename; confirm refusal + intact archive file | Unit test + manual `backlogit archive` probe |
| U4 | sync/rehydrate | Run `backlogit sync` with two same-ID files; confirm warning, row count intact | Log sample + regression test |
| U6 | full flow | Red on pre-fix `main`, green after U1–U3 | Integration test in `tests/` |

## Constitution Check

| Principle | Compliance |
|---|---|
| I. Safety-First Go | All new errors wrapped with `fmt.Errorf("context: %w", err)`; typed sentinels (`ErrIDCollision`, `ErrArchiveDestinationOccupied`) for the new known failures, asserted via `errors.Is`; no `unsafe`; `golangci-lint`/`go vet` clean required before merge. |
| II. Test-First (NON-NEGOTIABLE) | Every unit authors a failing test observed red before implementation; U6 is a dedicated end-to-end red→green repro authored before/alongside U1. |
| III. Workspace Isolation | All file ops resolve within `.backlogit`; the canonical scanner reuses workspace-rooted search dirs; no path traversal. |
| IV. CLI Containment | No writes outside cwd tree; archive writes stay under `.backlogit`. (No new persistent store in this plan — durable counter externalized.) |
| V. Structured Observability | U4 adds structured `slog.Warn`; doctor findings are structured; commits/PR carry traceability. |
| VI. Single Responsibility | No new external dependencies; extends existing doctor/allocation/archive code; standard library only. |
| VII. Destructive Approval | U3 touches a destructive surface — no destructive *terminal command* is run by this work; archive semantics become *safer* (refuse overwrite). |
| VIII. Safety Modes | High-blast-radius allocation/archive paths → plan-harden applied (investigate-first + freeze-scope to the named files). |
| IX. Git-Friendly Persistence | This plan adds no new persistent file state (durable counter externalized to `C55C5158`, where the IX constraints are recorded). |
| X. Context Efficiency | Detection prefers a single targeted canonical scan reused by both consumers; doctor returns structured findings. |
| XI. Merge Commit Preservation | Downstream Ship merge MUST use a merge commit (no squash/rebase) — recorded for the executor. |

No principle requires a justified violation. The IX persistence-model question is
carried with the externalized durable-counter follow-up (`C55C5158`), not this
plan.

## Plan Hardening

**Hardening required:** yes. Triggered by destructive/irreversible archive write
semantics (U3) and the high blast radius of the central `NextID` /
`CreateArtifact` / `ArchiveItem` paths. (The durable-counter persistent-state
risk was externalized to stash `C55C5158` during plan-review.)

**Learnings & instructions consulted:**
* `docs/compound/database-issues/atomic-rehydration-sqlite-transaction-2026-04-08.md`
  — rehydrate transaction boundary must not be disturbed (constrains U4).
* `.github/instructions/constitution.instructions.md` — principles I, II, VII,
  VIII, IX, XI.
* `.github/instructions/strict-safety.instructions.md` — action classification
  vocabulary below.

**Protected invariants:**
1. Archive is non-destructive: archiving item X MUST NOT overwrite a *different*
   archived item Y that shares a filename.
2. The legitimate same-path / terminal-status in-place archive flow MUST keep
   working (`archive.go:99-107, 157-166`).
3. `Rehydrate`'s atomic clear+rebuild transaction MUST remain a single
   transaction; the warning is detection-only and does not change collapse
   semantics (one surviving indexed row per ID).
4. No silent ID reuse: a resolved ID that already exists on the canonical
   filesystem MUST NOT be written.

### Risky actions (ProposedAction / ActionRisk)

**PA-1 — Change archive write to refuse foreign-destination overwrite (U3)**
* targets: `internal/core/archive.go` (ArchiveItem write path ~146-166)
* change_kind: behavior change on a destructive surface (archive write)
* ActionRisk: **high** (archive today silently overwrites = potential data loss;
  the change makes it safer but alters a central lifecycle path)
* rollback: revert the guard commit; behavior returns to prior overwrite (test
  coverage prevents accidental regression in either direction)
* approval_required: standard plan-review gate (no live data mutated by this
  Stage run)

**PA-2 — Add pre-write canonical uniqueness guard to create (U2)**
* targets: `internal/core/artifacts.go`, `internal/core/naming.go`
* change_kind: behavior change on the allocation hot path
* ActionRisk: **high** (every create flows through here)
* rollback: revert commit; allocation returns to index-only `NextID`
* approval_required: plan-review gate

**PA-3 — (EXTERNALIZED) Durable per-type high-water-mark counter**
* status: removed from this plan during plan-review; tracked as stash
  `C55C5158`. Recorded here for traceability.
* targets (future): `internal/core/naming.go` + a durable file store under
  `.backlogit` (NOT the ephemeral SQLite index)
* change_kind: new persistent state on the allocation path
* ActionRisk: **high** (persistent state that can diverge from canonical files;
  unresolved Git-commit-vs-local design question)
* approval_required: **operator decision required** on the persistence model
  before the follow-up is built

**PA-4 — Add rehydrate duplicate-source warning (U4)**
* targets: `internal/db/rehydration.go` (collection phase only)
* change_kind: additive structured logging; no schema or transaction change
* ActionRisk: **moderate** (touches rehydrate, but detection-only)
* rollback: revert commit
* approval_required: plan-review gate

**PA-5 — Extend doctor canonical audit + reproducing test (U1, U6)**
* targets: `internal/core/doctor.go`, `tests/`
* change_kind: additive finding type + new test
* ActionRisk: **low** (additive, read-only audit + test)
* rollback: revert commit
* approval_required: none beyond plan-review

### Added verification, rollback, and operator checkpoints

* **Operator checkpoint (externalized follow-up `C55C5158`):** decide the
  durable-counter persistence model (Git-committed vs. local-only) before that
  follow-up is planned/built. This plan ships U1–U4 + U6 without it.
* **Pre-merge verification:** full quality gate (`go test ./...`, `go vet ./...`,
  `golangci-lint run`, `gofmt -l .`) plus the U6 red→green demonstration.
* **Rollback coupling:** every unit is an independent, revertible commit.
* **Runtime probes:** `backlogit doctor`, `backlogit add`, `backlogit archive`,
  and `backlogit sync` manual probes per the Runtime Verification table.
* **Owner / validation window:** Ship agent owns execution; validation window =
  one full CI run + doctor probe on the staging branch before merge.

**Unresolved operator decision (does not block this plan):** the durable-counter
persistence model, carried with follow-up `C55C5158`.

<!-- plan-review-attempt: 2 -->

## Plan Review

**Gate decision: PASS** (after one FAIL→revise cycle; attempt 2).

Multi-persona review (Constitution Reviewer, Go Reviewer, Scope Boundary
Auditor, Architecture Strategist; Learnings Researcher folded into planning).
No P0 findings in either pass.

### Attempt 1 — FAIL

A convergent **P1** (Go Reviewer + Architecture Strategist) found the U2 create
guard mis-framed: it targeted only the `NextID` branch (top-level roots are
actually minted via `NextTypedHierarchicalID(parentID="")`) and scoped the scan
to queue+archive only (the doctor audits the full `artifactSearchDirs` set),
leaving the exact bug-site path unguarded and allowing detection and prevention
to disagree. Any P1 blocks harvest → FAIL.

High-value P2s also raised: define typed sentinel errors; pin U2 to deterministic
fail-loud; hoist U3's collision check before pre-archive hooks; correct U4's
row-count assertion vs. collapse semantics and keep it within `internal/db`;
recursive shared scanner returning parsed `artifactRef`s in a neutral file;
correct the inaccurate "detection gated behind a rarely-set flag" framing; and —
from four reviewers — **externalize the durable counter (U5)** as design-gated
and off the critical path.

### Revisions applied

* U2: single guard chokepoint after ID resolution covering both allocator
  branches; full `artifactSearchDirs` scope; deterministic fail-loud with
  `ErrIDCollision`; residual TOCTOU acknowledged. (resolves P1)
* Typed sentinels `ErrIDCollision` / `ErrArchiveDestinationOccupied` added to
  U2/U3/U6 and Constitution Check Row I.
* U3: hoisted before pre-archive hooks; refuse-on-parse-error; reuse parsed `fm`.
* U4: id→path closure map; row-count assertion aligned to collapse semantics;
  stays in `internal/db` (no `core` import).
* U1: recursive shared scanner in `internal/core/canonical_scan.go` returning
  parsed refs; corrected the detection-already-on-by-default framing (no `cmd/`
  change → width isolation preserved).
* U5 durable counter externalized to stash `C55C5158`.

### Attempt 2 — re-review

Confirmed the P1 and four of five structural P2s RESOLVED. One **new P2** was
found in U3: keying the in-place/refuse decision on **ID equality** would
mis-handle the core scenario (two different items sharing root ID `X`; the
destination shares the id and would be overwritten). Corrected in-plan to key on
**path equality** (refuse on any distinct occupied destination; narrow same-item
half-archive recovery requires id **and** content match). Remaining items are P3
polish only.

### Hardening verification

Plan declared `Requires plan hardening: yes`; a `## Plan Hardening` section is
present with protected invariants and `ProposedAction`/`ActionRisk` classification
(strict-safety vocabulary). The hardening requirement is satisfied.

### Outstanding (non-blocking)

* P3: hierarchical allocator gets the canonical-max advance + sentinel backstop
  (documented in U2).
* Operator decision (externalized, non-blocking this plan): durable-counter
  persistence model — carried with stash `C55C5158`.

Ready for harvest.

## Harvest Outcome

Harvested 2026-06-23 by the Stage agent.

* **Source stash:** `0F65FBC9` (bug, high) → archived as consumed.
* **Covering feature:** `066-F` — "Root-ID Conflict Integrity: Detection,
  Allocation, and Archive Safety" (queued).
* **Tasks** (all queued, parent `066-F`):
  * `066.001-T` — U1 doctor canonical root-ID audit + shared recursive scanner
  * `066.002-T` — U2 single-chokepoint pre-write ID uniqueness guard (`ErrIDCollision`)
  * `066.003-T` — U3 ArchiveItem distinct-destination overwrite refusal (`ErrArchiveDestinationOccupied`)
  * `066.004-T` — U4 rehydrate duplicate-source warning (no transaction change)
  * `066.005-T` — U6 end-to-end repro (queue + archive same root ID)
* **Dependency edges:** `066.002-T → 066.001-T`; `066.005-T → 066.001-T`,
  `066.005-T → 066.002-T`, `066.005-T → 066.003-T`. `066.003-T` and `066.004-T`
  are independent.
* **Shipment:** `066-S` — "Root-ID Conflict Integrity" (queued), items
  `[066-F, 066.001-T, 066.002-T, 066.003-T, 066.004-T, 066.005-T]`. Handoff token
  to the Ship agent.
* **Externalized follow-ups (not in this shipment):**
  * `C55C5158` (task, medium) — durable per-type high-water-mark counter (U5),
    design-gated on the persistence model.
  * `B8FF7590` (bug, high) — manifest-drift data repair for shipments
    `060-S` / `061-S` / `062-F` (separate one-time remediation; see deliberation).
