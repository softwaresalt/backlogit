---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for the durable fix to core.ShipShipment over-archiving covering features on partial-feature shipments: gate all THREE covering-feature mutation seams (the feature status->done transition at :191, the archival collector plus its linked-deliberation append at :292, and the transitive parent-status rollup where completeReleaseScope marks shipped children done and cascadePersistedParentStatuses rolls the covering feature to done and relocates it) on explicit shipment manifest membership — gating the two direct seams and neutralizing the rollup for non-member covering features via a narrow pre-ship status/location snapshot+restore (or cascade suppression); add a check-only doctor audit for the invariant, update the three existing children-only-manifest ship tests to the membership contract and land the 114-S partial-feature regression test, and correct the .ship.agent.md Step 6.1.b drift.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-07-31-shipshipment-partial-feature-archive-cascade-plan.md
title: 'ShipShipment: stop over-archiving covering features on partial-feature shipments'
---

## Source

* Deliberation: `docs/decisions/2026-07-31-shipshipment-partial-feature-archive-cascade-deliberation.md`
  (decision: Option A — archive/close the covering feature strictly by explicit
  shipment manifest membership; add a check-only `doctor` audit + partial-feature
  regression tests; correct `.ship.agent.md` Step 6.1.b drift; defer the keep-open flag).
* Stash: `C0909DB5` (high, bug).
* Prior art: `docs/compound/2026-07-31-p015-single-artifact-safe-close-for-partial-feature-shipments.md`,
  `docs/decisions/2026-07-29-ship-sequence-manifest-spike.md`,
  `docs/compound/2026-07-20-ship-gate-descoped-archived-member-exemption.md`,
  `docs/compound/2026-07-28-attach-commit-repersist-must-reload-from-markdown.md`,
  `docs/decisions/2026-06-26-archive-archived-from-self-reference-deliberation.md`.

## Problem Frame

In `internal/core/shipment_lifecycle.go`, `ShipShipment` (`:135`) resolves
`explicitScope = NormalizeShipmentItems(shipment)` (the manifest — the canonical
membership authority), then calls `featureScopeRoots(ctx, ws, explicitScope)`
(`:423`), which walks `parent_id` upward from **every** manifest item and returns
**all** ancestor `feature` IDs. `collectArchiveCandidateIDs` (`:292`) then appends
each ancestor feature to the archive set with only an already-archived guard
(`if feature.Status != models.StatusArchived { candidates = append(...) }`) — **no
terminal-children / membership gate**, unlike the descendant-item branch gated by
`isTerminalReleaseStatus`. Because `returnUnreleasedFeatureItems` (`:260`) has
already detached (requeued + cleared `parent_id`) nonterminal siblings before this
runs, shipping the first sub-shipment of a multi-cycle feature **always** archives
the covering feature. Observed on `114-S`: `archived_ids` included `106-F` though
the manifest `items` were only `[106.001-T, 106.002-T]`.

**Three mutation seams, not two.** Before `collectArchiveCandidateIDs` runs,
`ShipShipment`'s `featureIDs` loop already calls `setArtifactStatus(featureID,
models.StatusDone, "feature released")` (`:191`) on **every** covering feature, and
`returnUnreleasedFeatureItems` (`:260`) has already cleared `parent_id` on
non-terminal siblings. So (a) the covering feature is marked **done** and relocated
out of `.backlogit/queue/` independently of the archival gate, and (b) a post-detach
`parent_id` / `descendantItems` query can no longer see the very siblings that
should keep the feature open. **A third, indirect seam precedes both.**
`completeReleaseScope` (`:171`, body `:241`) marks each shipped child
`StatusDone` via `setArtifactStatus` (`:254`), and `setArtifactStatus`
**unconditionally** calls `cascadePersistedParentStatuses` (`:519`), which computes
`ComputeParentStatus(parentID)` (`internal/core/harness_status.go:48`) and — when
**every recorded child** is `done` (`SELECT status FROM items WHERE parent_id = ?`,
`harness_status.go:82`) — persists the covering feature as `StatusDone` **with
relocation** (`shouldRelocateOnStatusChange`, `:550`) and recurses upward, with **no
membership or feature-boundary guard**. So on a children-only ship that terminates
all of the feature's *recorded* children, the covering feature is rolled to `done`
and relocated **before** the `:191`/`:292` seams ever run — gating only those two is
insufficient. The durable fix therefore neutralizes **all three** covering-feature
seams: it gates the status→`done` transition (`:191`) and the covering-feature
archival append **together with its `linkedDeliberationIDs` append** (`:292`) on
**explicit shipment manifest membership** (`featureID ∈ explicitScope`, the
`NormalizeShipmentItems` set resolved at `:158`, *before* any mutation), and it
**suppresses the transitive parent-status rollup for non-member covering features** —
capturing each `featureScopeRoots(explicitScope)` feature's pre-ship status + queue
location at `:158` and restoring any `featureID ∉ explicitScope` feature that the
cascade rolled to `done`/relocated (equivalently, a cascade-suppression variant that
stops the upward rollup at a non-member feature boundary). A derived-but-unlisted
covering feature is thus never closed, relocated, or archived; a feature closes only
when the operator lists it in the manifest.

Membership plus rollup-suppression resolves the hard case: because a children-only
ship never lists the covering feature, gating the two direct seams keeps it out of the
archival/close set, and suppressing the transitive rollup keeps `completeReleaseScope`
from closing it when the shipped tasks are its only *recorded* children — exactly the
body-planned gap the 114-S learning proved a descendant-only gate could not close.
Note the rollup is the **only** path that can close a body-planned covering feature,
and it is precisely the case where `ComputeParentStatus` sees all recorded children
`done`; so R3 is **not** "handled by construction" by membership alone — the rollup
must be actively neutralized. The full-feature close path is preserved by listing the
feature ID in the manifest (a feature-inclusive shipment), which the existing tests
are updated to do; a listed feature is a member, so both the direct seams and the
rollup legitimately close it. `completeReleaseScope` (`:171`) **is** therefore a
covering-feature seam, but only **transitively**, via the child→parent status rollup —
not through `releaseScope` membership: `releaseScopeItemIDs` (`:399`) expands only
**downward** (descendants), so a derived ancestor feature never enters `releaseScope`,
yet the cascade fired by marking its children `done` still rolls it up. The fix
neutralizes that cascade for non-member features rather than changing
`releaseScopeItemIDs`. Authoritative status is still read from Markdown
(`findArtifact`), not the DB fast-path, per the re-persist seam learning, and
already-archived items are skipped.

## Requirements Trace

| Requirement (from deliberation success criteria) | Implementation action |
|---|---|
| R1: Partial-feature ship never archives the covering feature or unreleased siblings, and never marks the covering feature `done` | Gate **all three** covering-feature seams: the `:191` status→`done` transition and the `:292` archival + linked-deliberation append on explicit manifest membership, and **suppress the transitive parent-status rollup** (`completeReleaseScope`→`setArtifactStatus`→`cascadePersistedParentStatuses`) for non-member covering features; a derived-but-unlisted feature is left untouched (Unit 2) |
| R2: Full-feature ship still closes/archives the feature | The feature is closed/archived when it is an explicit manifest member (a feature-inclusive shipment); the updated + new characterization tests lock it (Unit 1, Unit 2) |
| R3: Survive planned-but-unharvested hard case | The body-planned covering feature (its only records are the shipped children) is the exact case the parent-status rollup would close; membership gating of the direct seams **plus** rollup suppression for the non-member feature keeps it in `queued`. Not "by construction" from membership alone — the rollup is actively neutralized (Unit 2) |
| R4: Regression test reproduces the 114-S partial-feature scenario | Red harness reproducing partial-feature over-archival (Unit 1) |
| R5: Retire the P-015 manual workaround / correct `.ship.agent.md` Step 6.1.b drift | Point Ship Step 6.1.b at native `backlogit_ship_shipment` now that the code is safe by construction (Unit 4) |
| R6 (durability): detect the invariant | Check-only `doctor` audit for over-archived/over-closed covering features (Unit 3) |

## Implementation Units

Each unit obeys the 2-Hour Rule (fewer than 3 files, fewer than 5 functions,
fewer than 4 test scenarios), width isolation (single domain), and produces an
atomic, verifiable milestone.

### Unit 1 — Ship-archival test contract: update the three children-only-manifest tests + add the 114-S partial regression (tests)

* **Domain:** tests. **Execution posture:** test-first (red) / characterization-first.
* **Existing tests updated to the membership contract** — these currently encode the
  bug (a children-only manifest that archives/closes the covering feature) and must
  be updated deliberately (incident-traced to 114-S / P-015), not left "green":
  1. `TestShipShipment_QueuePathAbsentAfterShip` (`internal/core/025_archive_harness_test.go`)
     — its real regression target is the **archive-deletion** bug (archive file present
     after ship), not feature archival. Change its manifest to **feature-inclusive**
     (`[]string{feature.ID, task.ID}`) so the archive-deletion assertions still hold
     with the feature legitimately a member.
  2. `TestShipShipment_CleansReleasedFeatureScope` (`internal/core/shipment_test.go`)
     — this is the **partial** scenario (feature + two tasks, ships only `releasedTask`,
     `futureTask` returned). Flip its assertions to the correct contract: the covering
     feature **and its linked deliberation remain in `.backlogit/queue/` (not archived,
     not `done`)**, `releasedTask` is archived, `futureTask` is returned to `queued`
     with `parent_id` cleared. This becomes the primary partial-feature regression.
  3. `TestShipShipment_SkipsAlreadyArchivedLinkedDeliberation`
     (`internal/core/shipment_repersist_test.go`) — under membership gating a
     children-only manifest skips the feature block, so `linkedDeliberationIDs` would
     never be reached and the skip-already-archived path (the test's purpose) would go
     unexercised. Change its manifest to **feature-inclusive** so the linked-deliberation
     collection path is still exercised and the skip assertion remains meaningful.
* **New tests added:** a **feature-inclusive full-feature characterization** (manifest
  lists the feature ID with all children terminal → feature set `done` and archived —
  locks the full path), and a **body-planned-only** regression (covering feature whose
  only records are the shipped tasks, ships children-only → feature stays in `queued`).
  The body-planned regression is the one that exercises the **transitive parent-status
  rollup** seam, so it MUST assert the covering feature's `findArtifact` status is **not
  `done`** **and** that the feature file is still under `.backlogit/queue/` — asserting
  only the **absence** of the feature from `result.ArchivedIDs` **false-greens**,
  because the rollup path closes and relocates the feature *without* routing it through
  the archival collector (it never appears in `ArchivedIDs`).
* **Files:** the three existing test files + optionally 1 new test file.
  **Functions:** ~3 updated + ~2 new test funcs (+ small helpers).
* **Scenarios (harvest may split into 1a "update existing tests" and 1b "new tests" to
  respect the 2-Hour Rule / <4-scenario heuristic):** children-only partial ship keeps
  feature+deliberation+sibling in queue; feature-inclusive full ship archives feature;
  body-planned-only children ship keeps feature open; feature-inclusive ship still
  exercises the linked-deliberation skip path.
* **Acceptance criteria:**
  * The partial-feature / body-planned tests fail against current `main` (feature
    wrongly `done`/archived) and pass after Unit 2; the feature-inclusive
    characterization passes both before and after (baseline preserved).
  * Assertions are filesystem/Markdown-based (queue dir + status via `findArtifact`),
    distinguishing "covering feature still in queue / not `done`" from "manifest items
    archived" to avoid a false-green (per the re-persist learning). Specifically, the
    body-planned regression asserts covering-feature status (**not `done`**) **and**
    queue-directory location — **not** merely `result.ArchivedIDs` absence, which
    false-greens against the transitive parent-status rollup path.
  * Every existing ShipShipment test that asserted covering-feature archival on a
    children-only manifest is enumerated and updated; none is left silently regressed.
  * Tests compile and run under `go test ./internal/core/...`.

### Unit 2 — Neutralize all three covering-feature seams on explicit manifest membership (code)

* **Domain:** code. **Execution posture:** test-first (make Unit 1 green).
* **Changes:** In `internal/core/shipment_lifecycle.go`, thread the already-computed
  `explicitScope` (the `NormalizeShipmentItems` manifest set resolved at `:158`) to the
  covering-feature seams and gate/neutralize them on membership (`featureID ∈
  explicitScope`):
  1. In the `featureIDs` loop, only call `setArtifactStatus(featureID,
     models.StatusDone, "feature released")` (`:191`) when the feature is a manifest
     member; otherwise leave its pre-ship status and queue location untouched.
  2. In `collectArchiveCandidateIDs` (`:292`), gate the **entire per-feature block**
     on membership — the feature append, the `linkedDeliberationIDs` append, and the
     per-feature descendant sweep. Add `explicitScope` (a set) to the function
     signature and update the single `ShipShipment` callsite. A derived-but-unlisted
     feature contributes nothing to the archive set; the shipped manifest items are
     still archived by the existing releaseScope-terminal branch at the top of the
     collector (unchanged).
  3. **Suppress the transitive parent-status rollup for non-member covering features.**
     `completeReleaseScope` (`:171`) marks each shipped child `done` via
     `setArtifactStatus` (`:254`), which unconditionally cascades
     (`cascadePersistedParentStatuses`, `:519`) and rolls the covering feature to
     `done` + relocates it when all its *recorded* children are done
     (`ComputeParentStatus`, `harness_status.go:82`). **Primary approach:** capture a
     pre-ship snapshot of each `featureScopeRoots(explicitScope)` feature's status +
     queue-file location at `:158` (before any mutation), then after
     `completeReleaseScope` and the `featureIDs` loop, **restore** any `featureID ∉
     explicitScope` feature that the cascade moved to `done`/relocated back to its
     snapshotted status and location. **Alternative approach:** thread a
     non-member-feature boundary set into the cascade (a `cascadePersistedParentStatuses`
     variant / `setArtifactStatus` option used only by `completeReleaseScope`) so the
     upward rollup stops at a covering feature that is not a manifest member — avoiding
     the restore, at the cost of a narrowly-scoped new parameter on a shared function.
     Ship chooses the mechanism under test-first; the **requirement** is invariant: a
     non-member covering feature must retain its pre-ship status and queue location.
* **Why membership + rollup-suppression suffices:** `explicitScope` is available before
  any mutation, so the two direct seams (`:191`, `:292`) need only a set-membership test
  and no inference. The transitive rollup, however, fires from `completeReleaseScope`
  marking shipped children `done`, so it **does** require a narrow pre-ship snapshot
  (each covering feature's status + queue location) or a cascade-suppression parameter —
  this is **not** the descendant-terminal snapshot rejected earlier (no per-descendant
  terminality is inferred and **no new terminal-status predicate** is introduced); it is
  a targeted status/location capture used only to revert an unwanted parent rollup for a
  non-member feature. A children-only ship never lists the covering feature, so R1/R3
  hold once the rollup is suppressed; a feature-inclusive ship lists it, so R2 holds.
  Optionally, as defense-in-depth, additionally require the per-feature `returned` slice
  from `returnUnreleasedFeatureItems` (already in scope at `:188`, before `:191`) to be
  **empty** before closing a listed feature — this keeps a listed-but-still-incomplete
  feature open **without introducing any new terminal predicate**. `blocking_cascade.go`
  and the five `isTerminalReleaseStatus` release-progression callsites are left
  untouched, and there is no dependence on the imprecise/`core.TerminalStatuses`
  distinction (there is no exported `core.TerminalStatuses` — the cascade set is the
  unexported `terminalCascadeStatuses` reached via `IsCascadeTerminalStatus` /
  `CascadeTerminalStatuses`).
* **Re-persist discipline:** the archival collector continues to read authoritative
  status from Markdown (`findArtifact`, not the DB fast-path) and skip already-archived
  items, per the attach-commit-repersist learning. Wrap all errors with `%w` and
  nil-guard `findArtifact` results (`ErrNotFound`).
* **`completeReleaseScope` is a transitive seam (neutralized, not ignored):** verified
  that `releaseScopeItemIDs` (`:399`) expands only downward, so `completeReleaseScope`
  (`:171`) never lists a derived covering feature in `releaseScope` — but marking that
  feature's shipped children `done` cascades
  (`setArtifactStatus`→`cascadePersistedParentStatuses`) and rolls the covering feature
  to `done`/relocated regardless. The fix neutralizes that cascade for non-member
  features (snapshot+restore in `ShipShipment`, or a cascade-suppression variant); when
  the feature is an explicit member the rollup close is correct and proceeds.
* **Files:** 1 (`shipment_lifecycle.go`; membership is a set-membership test — a tiny
  `isManifestMember` helper is acceptable but no new file is required).
  **Functions:** the `featureIDs` loop gate + `collectArchiveCandidateIDs` (signature +
  per-feature block gate) + its `ShipShipment` callsite + the pre-ship snapshot/restore
  of non-member covering-feature status+location inside `ShipShipment` (≤4 functions;
  the alternative cascade-suppression variant touches
  `cascadePersistedParentStatuses`/`setArtifactStatus` instead — still ≤4, but on a
  shared function, so snapshot+restore is the lower-blast-radius default).
* **Acceptance criteria:**
  * All Unit 1 tests pass; the `shipment_gate_*` tests and all other existing tests
    stay green.
  * On a children-only ship the covering feature is neither set to `done` nor archived
    and remains in `.backlogit/queue/`, and its linked deliberation is not archived; on
    a feature-inclusive ship the feature is set `done` and archived.
  * On a **body-planned** children-only ship (the covering feature's only recorded
    children are the shipped tasks), the feature is left at its pre-ship status (not
    `done`) and in `.backlogit/queue/` **even though `completeReleaseScope` marked all
    its recorded children `done`** — i.e. the transitive parent-status rollup is
    neutralized for the non-member feature.
  * `collectArchiveCandidateIDs` receives explicit membership; no covering-feature
    disposition is inferred from post-detach `parent_id`.
  * No change to the five unrelated `isTerminalReleaseStatus` callsites or to
    `blocking_cascade.go`.

### Unit 3 — doctor audit (check-only) for over-archived covering features (code)

* **Domain:** code. **Execution posture:** test-first.
* **Changes:** Add a **check-only** `doctor` audit that flags a covering feature
  archived (or marked `done`) while it still has non-terminal / returned descendant
  work. Detection must reconstruct membership from the shipment manifest and the
  `returned_to_backlog` provenance (NOT from `parent_id`, which
  `returnUnreleasedFeatureItems` clears). No mutation in this unit. Add a focused
  unit test for detection (crafted wrongly-archived fixture reported; clean
  workspace reports clean).
* **Deferred:** the destructive `doctor --fix` remediation is **out of scope for
  this release unit** (YAGNI — no wrongly-archived records confirmed in the wild;
  the forward fix prevents new ones). If later warranted it lands as a separate,
  operator-approved unit and MUST be **CLI-only (not exposed on the
  `backlogit_doctor` MCP tool)** per Constitution VII, body-preserving, with the
  clobber-refuse guard, restoring to the correct pre-archival status.
* **Files:** 1–2 (doctor check + test). **Functions:** ≤2.
* **Acceptance criteria:**
  * `doctor` reports a wrongly-archived / over-closed covering feature in a crafted
    fixture and reports clean when none exist.
  * The check performs no mutation; detection uses manifest + returned provenance,
    not `parent_id`.

### Unit 4 — Retire the P-015 partial-feature workaround in .ship.agent.md Step 6.1.b (docs/config)

* **Domain:** docs/config (agent definition). **Execution posture:** doc edit.
* **Changes:** In `.github/agents/.ship.agent.md`, correct Step 6.1.b so the Ship
  agent **uses the native `backlogit_ship_shipment` for both full- and
  partial-feature shipments** — because Unit 2 makes the covering-feature cascade
  safe by construction, the P-015 single-artifact procedural workaround is **no
  longer required** for partial shipments and is retired so the user command and the
  agent workflow stay at parity. Keep a short note that P-015 is now the invariant
  the code enforces (not a manual step). Do not alter unrelated pre-existing edits
  already present in that file.
* **Files:** 1. **Functions:** n/a.
* **Acceptance criteria:**
  * Step 6.1.b no longer instructs a manual single-artifact safe-close for partial
    shipments; it points at native `backlogit_ship_shipment` and references the
    now-code-enforced P-015 invariant.
  * No unrelated content in `.ship.agent.md` is modified.

## Dependency Graph

```text
Unit 1 (red harness) ──► Unit 2 (core fix, turns harness green)
                                   │
                                   ├──► Unit 3 (doctor audit reflects fixed invariant)
                                   └──► Unit 4 (agent doc aligns with new safe behavior)
```

* Unit 2 depends on Unit 1 (test-first).
* Unit 3 depends on Unit 2 (audit encodes the now-enforced invariant).
* Unit 4 depends on Unit 2 (doc describes the corrected behavior). No cycles.
* The **explicit-membership close-intent contract** (see Decisions) is resolved in
  planning and encoded by Unit 1's updated tests + feature-inclusive characterization
  before Unit 2 changes behavior, so behavior is pinned by test rather than left implicit.

## Decisions and Rationale

* **Explicit shipment membership over a keep-open flag** — the manifest is already the
  canonical membership authority (`NormalizeShipmentItems`), so gating reuses an owned
  source of truth with no new persisted schema, avoiding the typed codec / lossy
  `custom_fields` traps documented in prior art. Rationale detailed in the deliberation.
* **Membership needs no terminal-status predicate** — because the gate is pure
  set-membership over `explicitScope`, the fix introduces **no** new terminal-status
  classification, does not touch the five `isTerminalReleaseStatus` release-progression
  callsites, and does not depend on the (non-existent) exported `core.TerminalStatuses`
  vs. the unexported `terminalCascadeStatuses` / `IsCascadeTerminalStatus` distinction.
  An optional defense-in-depth "`returned` slice is empty" check reuses data already
  computed by `returnUnreleasedFeatureItems` — still no new predicate. The rollup
  suppression is likewise a status/location restore (or a boundary-set cascade stop),
  not a terminal-status predicate.
* **Gate all three mutation seams on membership** — the covering feature is rolled to
  `done` transitively by `completeReleaseScope` (`:171`) marking its shipped children
  done, then set `done` directly (`:191`) and detached (`:260`), before archival
  (`:292`); membership (available at `:158`, pre-mutation) gates the direct status→`done`
  transition and the entire per-feature archival block — feature append,
  `linkedDeliberationIDs` append, and descendant sweep — while a pre-ship
  status/location snapshot+restore (or cascade suppression) neutralizes the transitive
  rollup for non-member features, so partial ships neither close, relocate, nor archive
  the covering feature nor its linked deliberation.
* **Membership is the close-intent authority** — a covering feature closes only when it
  is an explicit manifest member. A genuinely complete feature whose manifest omits the
  parent stays open by design (pinned by the updated tests); the full-feature path is
  driven by a feature-inclusive manifest. This matches P-015 "archive only what is
  listed" and fixes the planned-but-unharvested hard case by construction.
* **`completeReleaseScope` is a transitive covering-feature seam** — `releaseScopeItemIDs`
  expands only downward (descendants), so the derived covering feature never enters
  `releaseScope`; but `completeReleaseScope` marking the shipped children `done`
  cascades (`setArtifactStatus`→`cascadePersistedParentStatuses`→`ComputeParentStatus`)
  and rolls the covering feature to `done`/relocated when all its *recorded* children
  are done. The fix neutralizes this cascade for non-member features via a narrow
  status/location snapshot+restore (or a cascade-suppression variant), rather than
  editing `releaseScopeItemIDs`; a listed (member) feature is legitimately closed by it.
* **doctor audit is check-only this release; destructive `--fix` deferred and
  CLI-only** — the forward fix prevents new corruption; remediation of any
  pre-existing wrongly-archived records is a separate operator-approved, CLI-only
  (not MCP) unit per Constitution VII.
* **Read authoritative status from Markdown (`findArtifact`)** — the DB fast-path
  omits `archived_status`/`item_links`; the re-persist seam learning mandates
  Markdown-authoritative reads and skipping already-archived items.
* **Conservative keep-open default** — a feature with body-planned-only remaining
  work (no records) stays open; this is the safe behavior P-015 protected and
  removes the agent-memory dependency.
* **Ship a `doctor` audit** — mirrors the archived-from delivery vehicle so the
  invariant is detectable/repairable and cannot silently regress.

## Risks and Caveats

* **Deliberate contract change to existing tests.** Three existing tests assert
  covering-feature (and linked-deliberation) archival on a **children-only** manifest —
  they encode the 114-S bug. Mitigation: Unit 1 updates all three explicitly
  (feature-inclusive manifest where the intent was a full ship; flipped keep-open
  assertions where the intent was partial), traceable to P-015; none is left silently
  regressed.
* **Regressing the full-feature close path.** Mitigation: a feature-inclusive
  characterization test (Unit 1) locks full-feature archival; membership makes the full
  path explicit (list the feature) rather than implicit.
* **False-green regression test.** The transitive rollup closes and relocates the
  covering feature **without** adding it to `result.ArchivedIDs`, so a test that only
  checks `ArchivedIDs` absence false-greens. Mitigation: assert both retention of the
  covering feature (queue dir + `findArtifact` status ≠ `done`) and archival of manifest
  items (skip-happened vs reload-masked-abort); never rely on `ArchivedIDs` alone for
  the body-planned case.
* **All three seams must be neutralized.** The covering feature is rolled to `done`
  transitively by `completeReleaseScope` (`:171`, via `cascadePersistedParentStatuses`)
  and set `done` directly at `:191`, both before the collector at `:292`; gating only
  the collector — or only the two direct seams — would still leave it `done` and out of
  queue. Mitigation: gate the `:191` transition and the whole per-feature collector
  block on membership **and** suppress the transitive rollup (snapshot+restore or
  cascade suppression) for non-member features (Unit 2).
* **Linked deliberations and the descendant sweep.** They live inside the per-feature
  collector block; gating only the feature append would still archive an open feature's
  deliberation / descoped descendants. Mitigation: gate the entire per-feature block on
  membership.
* **Scope re-expansion.** `releaseScopeItemIDs` expands manifest items to descendants
  with `IncludeArchived:true`; verify the shipped-item archival branch operates on the
  intended candidate set and that membership gating does not accidentally archive a
  manifest-excluded sibling.
* **doctor audit scope.** This release ships a check-only audit; destructive `--fix` is
  deferred to a separate operator-approved, CLI-only unit.

## Constitution Check

Mapping against `.github/instructions/constitution.instructions.md`:

* **I. Safety-First Go (MUST):** pass — all changes in Go 1.24.0; errors
  wrapped with `%w`; no `unsafe`; `go vet` / `golangci-lint` gates apply at Ship.
* **II. Test-First Development (NON-NEGOTIABLE):** pass — Unit 1 lands a failing
  harness (red) before Unit 2's production fix (green); Unit 3 test-first.
* **III. Workspace Isolation and Security Boundaries:** pass — all file operations
  resolve within the workspace; no secrets; no path traversal.
* **IV. CLI Workspace Containment (NON-NEGOTIABLE):** pass — every change is inside
  this repository tree; the external-autoharness template follow-ups (`7F0A6E89`,
  `6FA0829B`) are explicitly out of scope.
* **V. Structured Observability:** pass — `doctor` audit + conventional commits +
  test evidence provide traceability.
* **VI. Single Responsibility:** pass — no new dependencies; reuses existing
  archive/doctor infrastructure.
* **VII. Destructive Command Approval (NON-NEGOTIABLE):** pass — no destructive
  terminal commands; this release ships a **check-only** `doctor` audit. Any future
  `doctor --fix` remediation would be CLI-only (not MCP), scoped, reversible
  (unarchive), and operator-approved.
* **VIII. Explicit Safety Modes for Elevated Risk:** pass — this is a hardening
  plan; the `## Plan Hardening` section performs careful-mode risk enumeration and
  freeze-scope protected invariants, and classifies risky actions via strict-safety
  `ProposedAction`/`ActionRisk`.
* **IX. Git-Friendly Persistence:** pass — archival persists Markdown + YAML
  frontmatter; the fix reads Markdown-authoritatively via `findArtifact`.
* **X. Agent Context Efficiency:** pass — the `doctor` audit is a targeted check; no
  bulk scanning is introduced.
* **XI. Merge Commit History Preservation (NON-NEGOTIABLE):** pass — ships via a
  merge commit (Ship-owned); no squash/rebase.

Constitution Check: pass

## Plan Hardening Signals

* **public API, schema, or contract change:** present — `ShipShipment` archival
  behavior is a behavioral contract; which items get archived on ship changes.
* **security, auth, permission, or compliance-sensitive behavior:** absent.
* **migration, backfill, destructive data/config action, or irreversible step:**
  present — archival is a data-lifecycle action affecting live backlog state; an
  optional `doctor --fix` remediates existing records (reversible via unarchive).
* **external integration, operator checkpoint, or external dependency:** absent.
* **high runtime, rollout, or rollback risk:** present (moderate) — a regression
  could archive or fail to archive the wrong items, corrupting backlog state for
  every multi-cycle feature.

Requires plan hardening: yes

## Runtime Verification and Closure

* **Changed runtime surface:** the `backlogit shipment ship` / `backlogit_ship_shipment`
  path and the `doctor` command. No API/browser surface.
* **Runtime verification (Ship):** after the fix, run a real partial-feature ship in
  a scratch fixture and confirm `archived_ids` contains only manifest items + the
  shipment record, and the covering feature + unshipped siblings remain in
  `.backlogit/queue/`; run a full-feature ship and confirm the feature archives;
  run `doctor` to confirm a clean report and correct detection on a crafted
  wrongly-archived fixture.
* **Operational closure:** record a compound-learning update (or supersede note) on
  `docs/compound/2026-07-31-p015-single-artifact-safe-close-for-partial-feature-shipments.md`
  once the durable code fix lands, since the cascade is then safe by construction;
  note the `doctor` audit as the ongoing detection surface. Rollback trigger:
  wrong `archived_ids` on a partial-feature ship in verification → revert the
  shipment-lifecycle change. Because one hardening signal (and specifically a
  data-lifecycle/contract signal) is present, `plan-harden` must tighten the
  verification, rollback, and remediation-scoping detail before review.

## Plan Hardening

**Hardening required: yes.** Triggered by two signals — a behavioral **contract
change** to `core.ShipShipment` archival, and a **data-lifecycle/irreversible-step**
signal (archival mutates live backlog state; an optional `doctor --fix` remediates
existing records). Blast radius is broad: every multi-cycle feature that ships in
more than one shipment is affected by the current defect and by any regression in
the fix.

### Protected invariants (must not regress)

1. **Full-feature close path** — a feature explicitly listed in the manifest still
   closes and archives on ship. Locked by the Unit 1 feature-inclusive characterization
   test; the three existing children-only-manifest tests are updated to the membership
   contract (not left silently regressed).
2. **Unrelated terminal-predicate callsites** — the fix is pure set-membership over
   `explicitScope` and introduces **no** new terminal-status predicate; the five
   existing `isTerminalReleaseStatus` release-progression callsites and
   `blocking_cascade.go` remain unchanged. (No exported `core.TerminalStatuses` exists;
   the cascade set is the unexported `terminalCascadeStatuses` via
   `IsCascadeTerminalStatus` / `CascadeTerminalStatuses` — unused by this fix.)
3. **Markdown authority at the re-persist seam** — status reads for the archival
   decision use `findArtifact` (Markdown), and already-archived items are skipped;
   never re-stamp an archived artifact (`attach-commit-repersist` learning).
4. **Manifest-exclusion is not scope-exclusion** — verify `releaseScopeItemIDs`
   descendant expansion (`IncludeArchived:true`) does not re-pull manifest-excluded
   siblings into the archival candidate set.
5. **Pre-existing `.ship.agent.md` edits preserved** — Unit 4 edits only Step 6.1.b;
   it must not revert or overwrite unrelated pre-existing changes in that file.
6. **Transitive parent-status rollup neutralized for non-member covering features** —
   `completeReleaseScope` marking shipped children `done`
   (`setArtifactStatus`→`cascadePersistedParentStatuses`→`ComputeParentStatus`) must not
   leave a `featureID ∉ explicitScope` covering feature at `done` or relocated out of
   `.backlogit/queue/`. Locked by the Unit 1 body-planned regression, which asserts
   status (≠ `done`) and queue-directory location, not merely `ArchivedIDs` absence.

### Learnings and instructions consulted

* `docs/compound/2026-07-31-p015-single-artifact-safe-close-for-partial-feature-shipments.md`
  (root cause + why a descendant-only gate fails),
  `docs/compound/2026-07-20-ship-gate-descoped-archived-member-exemption.md`
  (terminal-predicate imprecision; scope re-expansion),
  `docs/compound/2026-07-28-attach-commit-repersist-must-reload-from-markdown.md`
  (re-persist seam), `docs/decisions/2026-07-29-ship-sequence-manifest-spike.md`
  (manifest is canonical membership),
  `docs/decisions/2026-06-26-archive-archived-from-self-reference-deliberation.md`
  (`doctor --fix` delivery vehicle).
* `.github/instructions/constitution.instructions.md` (P-II test-first, P-VII
  destructive approval, P-XI merge commit), `.github/policies/workflow-policies.md`
  (P-015 single-artifact safe-close), `.github/instructions/strict-safety.instructions.md`.

### Risky actions (strict-safety classification)

| ProposedAction | change_kind | ActionRisk | Approval | Rollback | ActionResult |
|---|---|---|---|---|---|
| Gate covering-feature close (`:191`) + the per-feature archival block (`:292`) on explicit manifest membership **and** suppress the transitive parent-status rollup (`completeReleaseScope`→`cascadePersistedParentStatuses`) for non-member features via pre-ship status/location snapshot+restore or cascade suppression (Unit 2) | contract change / behavior | high | Prefer operator approval before merge (contract + broad blast radius) | Revert the `shipment_lifecycle.go` change; behavior returns to prior cascade | planned |
| Add `doctor` audit detection (Unit 3, check-only) | local code + read-only check | low | Not required | Revert doctor change | planned |
| `doctor --fix` remediation that unarchives wrongly-archived covering features | destructive data/config action | destructive | **Required** — operator approval; CLI-only (not MCP) | Re-archive affected features (reversible); scope limited to detected records | deferred — out of scope this release |
| Edit `.ship.agent.md` Step 6.1.b (Unit 4) | config/doc change | moderate | Not required | Revert doc edit | planned |

The `doctor --fix` remediation is the only `destructive` action and MUST NOT run
without explicit operator approval (Constitution VII / strict-safety). The default
`doctor` run is audit-only and non-mutating. The Unit 2 contract change is `high`
risk; approval is preferred before merge and is naturally gated by the P-014
operator-approved merge that Ship performs.

### Deepened runtime verification (carried into Ship's runtime-verification)

1. **Environment precheck:** operate in a disposable scratch backlog fixture, never
   the live `.backlogit/` workspace, when exercising real ship/doctor mutations.
2. **Partial-feature scenario:** ship a manifest of only child tasks; assert
   `archived_ids` = manifest items + shipment record only, and the covering feature
   plus unshipped siblings remain in `.backlogit/queue/`.
3. **Full-feature scenario:** ship a manifest whose feature is explicitly released
   with all descendants terminal; assert the covering feature archives.
4. **Planned-but-unharvested hard case:** covering feature whose only recorded children
   are the shipped tasks stays open after a partial (children-only) ship — assert its
   status is not `done` **and** its file remains under `.backlogit/queue/` (proving the
   transitive parent-status rollup was neutralized); do not rely on `archived_ids`
   absence, which false-greens for this path.
5. **doctor detection:** crafted wrongly-archived fixture reported; clean workspace
   reports clean; `--fix` (if run, post-approval) touches only affected records.

### Deepened operational closure

* **Monitoring/detection signal:** the `doctor` audit is the durable detection
  surface for the invariant; run it after multi-cycle-feature ships.
* **Rollback trigger:** any partial-feature ship in verification whose `archived_ids`
  includes the covering feature (or an unreleased sibling) → revert the
  `shipment_lifecycle.go` change immediately and halt.
* **Rollback procedure:** `git revert` the Unit 2 commit; if a wrong archival was
  already applied to real state, use the existing unarchive path (or
  `doctor --fix`, post-approval) scoped to the affected feature.
* **Owner / validation window:** Ship agent owns verification during the PR window;
  closure records the outcome and updates the P-015 compound learning to note the
  code fix makes the cascade safe by construction.

### Resolved decisions and deferred scope

* **Close semantics — RESOLVED:** the covering feature closes/archives/relocates **iff
  it is an explicit manifest member**. Membership gates the two direct seams (`:191`,
  `:292`) and, because `completeReleaseScope`'s child-completion cascade would otherwise
  roll a non-member feature to `done`, the transitive parent-status rollup is suppressed
  for non-member features (pre-ship status/location snapshot+restore, or cascade
  suppression). No descendant-terminal inference and no new terminal predicate are used.
  The full-feature path is a feature-inclusive shipment, and the three existing
  children-only-manifest tests are updated to this contract (Unit 1). The complete seam
  set is enumerated (three seams); this is not left open for build-time reinterpretation.
* **`doctor --fix` remediation — DEFERRED (scope):** this release ships the check-only
  audit; destructive remediation of any pre-existing wrongly-archived records is a
  separate operator-approved, CLI-only unit. Recommended: audit-only first; add `--fix`
  only if wrongly-archived records are found in the wild.

## Plan Review

dispatch_mode: multi-agent-dispatch
TOOL_OK: reviewer-subagent-dispatch
TOOL_DEGRADED: backlogit_docs_lint (MCP path-Rel bug; CLI `backlogit docs lint --profile authoring` fallback used for frontmatter/style validation)
review-fix cycles: 3 (circuit-breaker limit reached)
decision: ADVISORY
operator_authorization: approved

Multi-persona plan review dispatched reviewer sub-agents across tiers over three
review-fix cycles (circuit-breaker limit = 3). Each cycle surfaced progressively deeper,
*different* findings (convergent refinement, not thrashing): cycle 1 the seam/predicate
shape, cycle 2 the membership-vs-test and linked-deliberation gaps, cycle 3 the transitive
parent-status rollup. Findings by cycle:

### Cycle 1 — FAIL
Personas (parallel): Go Reviewer, Architecture Strategist, Scope Boundary Auditor,
Agent-Native Parity Reviewer, SQLite Reviewer, Constitution Reviewer.

* **P0 (Go):** the initial descendant-terminal predicate was imprecise and coupled the
  fix to `isTerminalReleaseStatus`, risking the 5 release-progression callsites.
* **P1 (Architecture):** the fix relied on a post-detach `parent_id` query that
  `returnUnreleasedFeatureItems` had already cleared; disposition could not be inferred
  after detach.
* **P1 (Parity):** user-command vs agent-workflow drift around Step 6.1.b (P-015 manual
  workaround) not addressed.
* **Resolution:** 13 edits — replaced the terminal predicate with a **membership-primary**
  gate over `explicitScope`; added Unit 4 to retire the P-015 workaround for parity.

### Cycle 2 — FAIL
Blocking reviewers: Go Reviewer, Architecture Strategist, Agent-Native Parity Reviewer.

* **Parity P1:** RESOLVED (verified) — Unit 4 aligns command/workflow.
* **P0 (Go):** the membership conjunct as written regressed the green test
  `TestShipShipment_QueuePathAbsentAfterShip` (children-only manifest that legitimately
  archives on a single-task feature).
* **P1 (Architecture):** stale "Unresolved" sections; the descendant/linked-deliberation
  sweep inside the collector was still un-gated; first claim of an additional seam.
* **Resolution:** cycle-3 rewrite — gate BOTH direct covering-feature seams (`:191`
  status→done and the whole `:292` per-feature block incl. `linkedDeliberationIDs`) on
  membership; Unit 1 explicitly OWNS updating the three existing children-only-manifest
  tests to the membership contract; rewrote stale "Unresolved" sections to "Resolved".

### Cycle 3 — findings 1 & 2 RESOLVED; finding 3 (third seam) VERIFIED + INCORPORATED
Reviewers (parallel): Go Reviewer, Architecture Strategist.

* **Finding 1** (cycle-2 Go P0 test regression): **RESOLVED** — verified; Unit 1 now owns
  the three test updates with feature-inclusive manifests where a full close was intended.
* **Finding 2** (cycle-2 Architecture P1s: stale sections, un-gated sweep): **RESOLVED** —
  verified; the whole per-feature collector block is gated and stale sections rewritten.
* **Finding 3 (NEW — both reviewers independently converged, HIGH confidence, Go=P0 /
  Architecture=P1):** a **third, indirect mutation seam**. `completeReleaseScope` (`:171`,
  body `:241`) marks each shipped child `StatusDone` via `setArtifactStatus` (`:254`),
  which **unconditionally** calls `cascadePersistedParentStatuses` (`:519`) →
  `ComputeParentStatus` (`internal/core/harness_status.go:82`), rolling the covering
  feature to `StatusDone` **with relocation** out of `.backlogit/queue/` whenever **all
  its recorded children** are done — the exact body-planned 114-S case — **before** the
  `:191`/`:292` seams run. Gating only the two direct seams is therefore insufficient
  (violates R1/R3/R4). A regression asserting only `result.ArchivedIDs` absence
  false-greens, because the rollup path closes+relocates the feature without routing it
  through the archival collector.
  * **VERIFIED against source by the Stage agent** (not taken on reviewer assertion):
    `completeReleaseScope` `:241-258`, `setArtifactStatus` `:498-522` (unconditional
    cascade at `:519`), `cascadePersistedParentStatuses` `:525-559` (parent persist +
    relocation `:550`, upward recursion `:558`, no membership/feature guard),
    `ComputeParentStatus` `:48-83` (`SELECT status FROM items WHERE parent_id = ?`;
    returns `StatusDone` when all recorded children done — body-planned units invisible).
  * **INCORPORATED into this plan revision** (no fourth review-dispatch, per the
    circuit-breaker limit): Problem Frame (two→three seams + the indirect path);
    Requirements Trace R1 & R3 (rollup suppression, not "by construction"); Unit 1
    (body-planned regression asserts `findArtifact` status ≠ `done` AND queue-directory
    location, with an explicit false-green callout); Unit 2 (item 3 — neutralize the
    rollup for non-member features via pre-ship status/location snapshot+restore, or a
    cascade-suppression variant; corrected the "no snapshot needed" language); Decisions
    (all-three-seams + `completeReleaseScope` is a transitive seam); Risks (all-three
    seams + rollup false-green); Plan Hardening (protected invariant #6, risky-actions
    row, deepened runtime-verification #4).

### Circuit-breaker disposition and residual risk
Cycle 3 is the final review-fix cycle (circuit-breaker limit = 3). The cycle-3 third-seam
finding was **verified against source and incorporated into the plan specification**; per
the limit **no fourth review-dispatch is performed**. The plan now enumerates the complete
**three-seam** set and specifies neutralization for each, so no P0/P1 finding remains
un-addressed *in the plan text*. **Residual risk (documented, carried to Ship):** the
third-seam neutralization mechanism (snapshot+restore vs cascade-suppression) is a
plan-level specification incorporated after the final review cycle and was **not itself
re-reviewed**; its implementation correctness is verified at **Ship build time under
test-first discipline** — Unit 1's body-planned regression MUST fail on current `main`
(feature wrongly `done`/relocated) and pass only after Unit 2 neutralizes all three seams.
This residual is a **mandatory Ship-phase test-first verification item** and is backed by
the rollback trigger already recorded in Plan Hardening (any partial-feature ship whose
disposition closes/relocates/archives the covering feature → revert the Unit 2 commit).

### Gate decision
decision: **ADVISORY** — the plan is architecturally sound and complete as a
specification; all P0/P1 findings across three review cycles are addressed in the plan
text, with one documented residual (third-seam mechanism unverified by review, to be
proven by Ship test-first). Not PASS because a P0/P1-severity finding was resolved in the
final cycle without a confirming re-review; not FAIL because the plan text contains no
un-addressed P0/P1. Harvest may proceed under operator authorization.
operator_authorization: approved (autopilot `Stage next` authorization; degraded-visibility
mode — intercom/engram MCP not exposed this session).
