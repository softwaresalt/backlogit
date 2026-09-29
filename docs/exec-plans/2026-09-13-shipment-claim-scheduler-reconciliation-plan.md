---
chunk_strategy: h1-h2-h3
description: "Implementation plan for CC0EBB59 — persist the scheduler_baseline_claim marker on every claim-activated manifest member (option A lifecycle: kept through block/unblock/return, three-part consumer predicate) and make claim crash recovery accept marked members, so claim-activated tasks are distinguishable from organic active residuals"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-13-shipment-claim-scheduler-reconciliation-plan.md
title: "Implementation Plan: Shipment-claim activation reconciliation (universal scheduler-baseline marker)"
docline:
    stash_id: CC0EBB59
    status: revised
    created_at: 2026-09-13T17:31:00Z
---

## Objective

Add an additive, backward-compatible **enabling precondition** to backlogit
shipment-claim activation. After it lands, a claim-activated manifest task can
be told apart from an organic active residual.

The bootstrap slice (`173-F`, shipped by `154-S`) delivers only:

* the in-repo marker;
* its claim-crash-recovery compatibility;
* verification of both claim rollback paths and of the existing generic read
  surface;
* the published consumer contract.

The option A lifecycle regression net (U4) is **deferred** to a follow-on
feature. No follow-on shipment is created now; a later Stage session packages
it after `154-S` ships (see `## Size Validation and Decomposition`).

The observable P-002.6 scheduler-misclassification defect is resolved only
once the autoharness scheduler consumes the marker, which is out-of-workspace
follow-up work. This shipment is an **enabling precondition, not full defect
resolution**.

**In-repo scope of the bootstrap slice (revised 2026-09-29, attempt 7):**

* `internal/core/shipment_lifecycle.go`: the claim loop, the marker write and
  the one shared key constant.
* `internal/core/shipment_recovery.go`: the claim recovery candidates (U1b).
* Tests in `internal/core`, `internal/cli` and `internal/mcp`.
* One operator doc.

No other shipment lifecycle operation (Block, Unblock, ReturnBlocked, Ship,
normalize, Abandon) and no generic write path (`UpdateArtifact`, `move`,
`update`) is changed. That follows from the option A decision below.

**Sources:**

* `docs/decisions/2026-09-13-shipment-claim-scheduler-reconciliation-deliberation.md`
  (original).
* `docs/decisions/2026-09-28-173f-marker-lifecycle-option-a-decision.md`
  (F2 marker lifecycle, option A).

**Review status:** attempt 6 recorded FAIL (one P1, ten P2s). The operator
authorized one additional attempt past the circuit breaker on 2026-09-28
("Yes on additional plan review; validate the size of the plan; if too large,
decompose."). This revision applies every attempt-6 remediation, adds the
operator-required size validation, and decomposes the plan. It is the input to
plan-review attempt 7. If attempt 7 fails, Stage stops and escalates; there is
no attempt 8 without new operator direction.

## Marker lifecycle decision (option A)

The operator decided this on 2026-09-28T22:22 -07:00: "Decision: option A".
The full artifact is
`docs/decisions/2026-09-28-173f-marker-lifecycle-option-a-decision.md`.

* **Contract (pinned, unchanged):** a single `custom_fields` key,
  `scheduler_baseline_claim`. Its value is the ID of the activating shipment,
  as a non-empty string. The prior art is
  `docs/compound/2026-07-23-machine-readable-governance-field-contract.md`.
  The key is **reserved but not write-protected**: a generic `update` can
  still write it, and consumers must rely on the predicate, never on the raw
  value alone.
* **Producer:** only `ClaimShipment` writes the key. It writes it on every
  manifest member whose **preimage status was `queued`**, independent of
  manifest order (F5). A re-claim overwrites a stale value, including a stale
  value from a different shipment. Members whose preimage status is not
  `queued` are left byte-identical.
* **Kept, never cleared:** Block, Unblock (to `active` or `queued`),
  `ReturnBlockedItem`, Ship, Abandon, normalize, and the generic
  `UpdateArtifact`, `move` and `update` paths leave the key as it is.
  * The shipment operations preserve `custom_fields` through
    `cloneArtifact` (which uses `maps.Clone`) + `persistArtifact`. The generic
    paths load, mutate named fields and persist the same map.
  * Their recovery candidates are clones of preimages taken at operation time,
    so they already carry any marker. No new write paths or candidate shapes
    are needed.
  * **Learning waiver (attempt 6 P2-10).**
    `docs/compound/best-practices/atomic-multi-item-claim-rollback-and-stale-blocked-clearing-2026-06-27.md`
    teaches that claim-scoped state must be cleared when the item leaves the
    claimed state. That rule is **explicitly waived for this advisory
    marker**: option A keeps it and neutralizes staleness through the
    three-part predicate, because clearing would add writes and recovery
    candidates to four operations (option B, rejected). The learning's
    all-or-nothing claim rollback rule is **not** waived; U3 verifies it.
* **Removed only by rollback:** in-process snapshot restore, or journal
  recovery persisting the preimage clones, restores the exact pre-claim
  bytes, including a stale value the preimage carried.
* **Consumer predicate (normative, published by U5):** an item is
  claim-activated if and only if all three hold:
  1. `status == active`;
  2. `custom_fields.scheduler_baseline_claim` == the ID of the **currently
     active shipment** (there is at most one);
  3. the item is in that shipment's current `items` manifest.

  Otherwise the item is organic-active. When no shipment is active, no item is
  claim-activated. "Non-empty ⇒ claim-activated" is **withdrawn**.
  **Active-shipment cardinality guard (attempt 6 P2-6):** if the consumer's
  active-shipment read returns more than one active shipment, the state is
  an invariant violation and is **indeterminate** (fail closed under the R3
  consumer rule).
* **Accepted residuals (summarized here; the decision artifact text is
  normative and U5 copies it verbatim):**
  * R1: stale markers persist on returned, blocked, shipped, abandoned and
    archived items. The predicate neutralizes them.
  * R2 (restated per attempt 6 P2-3): any activation route for an item that
    carries the active shipment's marker and is in its manifest reads as
    claim-activated, however it became active. Organic activation *while
    blocked* is guarded and cannot happen. Examples: a member unblocked to
    `queued` and activated organically before the re-claim; an item returned
    with `ReturnBlockedItem` (which leaves it `blocked`), later activated
    organically, then added back to the active shipment.
  * R3: after a double-fault partial compensation, CLI and MCP can disagree
    until recovery runs, and `shipment list` is index-backed on both
    transports. Split into a consumer rule (indeterminate → defer to the claim
    gate) and an operator-only remediation (fresh-process CLI `backlogit
    sync`, then `backlogit_sync_index`). See U5.

## Requires plan hardening: yes

This work touches shipment lifecycle state, the claim crash-recovery CAS, and
a cross-workspace contract. See `## Plan Hardening`.

## Size Validation and Decomposition

Operator requirement (2026-09-28): validate the plan's size and decompose it if
it is too large. Rules: the 2-hour rule (fewer than 3 files, fewer than 5
functions, fewer than 4 test scenarios per task), width isolation (one skill
domain per task), atomic milestone, and an overall release unit that one
supervised bootstrap shipment can carry.

**Attempt-6 plan, as submitted:** 9 units (U0a, U0b, U0c, U1b, U1, U2, U3, U4,
U5) plus the feature, so 9 tasks. At least 7 dependency waves:
{U0a, U0b, U0c} → U1b → U1 → U2 → U3 → U4 → U5. 19 test scenarios, about 12
files. Every unit individually met the 2-hour rule and width isolation, but
the release unit was over the ~8-task ceiling for a single supervised
bootstrap, and `154-S` is the one shipment that runs **without** the
active-residual halt, so every extra wave is a supervised wave.

**Verdict: too large for one bootstrap release unit. Decomposed.**

**Decomposition.**

1. **Retire U2 (`173.002-T`), folded into U0b.** Attempt 6 P1-1 showed U2
   changes no production code and U1, not U2, turns U0b green. U2's
   acceptance criteria (generic projection, no bespoke field, no mutation)
   become U0b's post-implementation AC, and U2's optional helper is dropped.
   `173.002-T` is closed as superseded (not implemented), with provenance.
2. **Defer U4 (`173.004-T`) to a follow-on feature.** U4 is a regression net
   over operations this shipment does **not** change (Block, Unblock,
   `ReturnBlockedItem`, Ship). The bootstrap's own safety surface (claim
   write, recovery, rollback) is fully verified by U0a, U0c, U1b and U3. U4
   moves (reparented, not re-created) to a new follow-on feature and leaves
   the `154-S` manifest.
   * **No follow-on shipment now (attempt 7 P2-A).** The operator asked for no
     new shipment unless needed, and one is not needed: U4 keeps task-level
     `blocks` edges onto `173.001-T` (U1) and `173.003-T` (U3), so it cannot
     start before the bootstrap code exists. A later Stage session packages
     it (as a single-member shipment, one wave, claimable without a second
     bootstrap waiver, the `182-S` precedent) after `154-S` ships.
   * **Ordering relative to 074-DL.** U4 sits behind `154-S` and follows the
     existing 074-DL rule like every other item behind it (shipped provenance
     **and** the operator's autoharness-consumption attestation). The plan
     does **not** require U4 to ship before the attestation, which avoids the
     attestation ⇄ U4 loop the attempt-7 review found. The rows U4 would pin
     are published as "by construction; regression net tracked by the
     follow-on U4 task" (see U5), a wording that never needs a later edit.
3. **Keep U5 in the bootstrap.** The contract must ship with its producer.
   Deferring U5 would put a marker on disk with no published predicate, which
   invites the withdrawn "non-empty ⇒ claim-activated" reading. U5 depends
   on U3 (the last bootstrap verification) instead of U4 (attempt 6 P2-5 is
   satisfied for everything the bootstrap verifies; see U5 for the
   verification-status column that covers the deferred rows).
4. **Move the widened regression gate** from U4 (3) to an AC of U3, the last
   bootstrap test task (attempt 6 P2-2 "gate as an AC").

**Bootstrap slice (`173-F`, `154-S`):**

| Unit | Task | Domain | Files | Functions (approx.) | Scenarios | Wave |
|---|---|---|---|---|---|---|
| U0a | `173.006-T` | tests | 1 | 3 test funcs + 1 fixture helper | 3 | 1 |
| U0b | `173.007-T` | tests | 2 (one per transport package) | 2 test funcs + 2 fixture helpers (one per package: `cli_test`, `mcp`) | 3 | 1 |
| U0c | new | tests | 1 | 3 test funcs + 1 journal-fixture helper | 3 | 1 |
| U1b | new | code | 2 (`shipment_recovery.go`; one constant line in `shipment_lifecycle.go`) | 1 changed | per U0c | 2 |
| U1 | `173.001-T` | code | 1 | 3 (new extracted writer, `setArtifactStatus` as a delegating wrapper, `ClaimShipment` loop) | per U0a/U0b | 3 |
| U3 | `173.003-T` | tests | 1 | 2 test funcs | 2 | 4 |
| U5 | `173.005-T` | docs | 1 | n/a | n/a | 5 |

Totals: 7 tasks (down from 9), 5 waves (down from at least 7), 11 test
scenarios (6 RED, 3 characterization, 2 verification), 9 file touches over 8
distinct files. Every task meets
the 2-hour rule and has a single domain and an atomic, testable milestone.

**Deferred slice (new follow-on feature; no shipment until a later Stage
session):**

| Unit | Task | Domain | Files | Scenarios | Gate |
|---|---|---|---|---|---|
| U4 | `173.004-T`, adopted under the follow-on feature (new ID) | tests | 1 | 2 | task `blocks` edges onto U1 and U3; routed only under the 074-DL rule after `154-S` has shipped provenance |

**Why this is the minimal bootstrap slice.** Removing any remaining unit
breaks a stated invariant: without U1b the marker wedges recovery (F1);
without U0a/U0b/U0c there is no test-first RED; without U3 the claim
all-or-nothing invariant is unverified for marked members; without U5 the
marker ships with no published predicate.

## Implementation Units

Execution order (the dependency edges enforce it):

1. U0a, U0b and U0c: the RED harnesses.
2. U1b: recovery compatibility.
3. U1: the marker write.
4. U3: rollback verification and the shipment regression gate.
5. U5: the docs.

**U1b lands before U1.** At no commit may the marker be written while claim
crash recovery still rejects marked members, because that is the F1 wedge.
The UR3 crash-ready flake prerequisite (`BDA56ED8`) has shipped (`182-S`,
merge `70d72044`), so every `./internal/core/...` gate here runs without a
known-flake rerun policy.

Test-seam rule for every test unit below: no `t.Parallel` when a test swaps a
package-global seam such as `persistArtifactWriteFn`; restore the seam with
`t.Cleanup` (prior art:
`docs/compound/2026-07-29-durable-writes-test-seam-patterns.md`).

RED-compile rule (attempt 7 P3): the wave-1 harnesses (U0a, U0b, U0c) use the
**literal string** `"scheduler_baseline_claim"`, never the
`schedulerBaselineClaimKey` constant (which only arrives in U1b). Each RED
therefore compiles and fails on behavior, and the tests pin the contract
string independently of the constant.

* **U0a: RED harness + characterization, claim marking (task 173.006-T).**
  * Domain: tests. File: `internal/core/shipment_claim_marker_test.go` (new).
  * Written before U1. No 173-F dependencies. 3 scenarios.
  * Fixture rule: no non-queued member-parent that has queued member children
    (that shape is not needed and muddies byte-identity).
  * **(1) RED.** `ClaimShipment` sets `scheduler_baseline_claim = <shipmentID>`
    on every member whose preimage status is `queued`.
    * Fixture includes a queued member whose preimage already carries a
      **stale foreign marker** (`"OLD-S"`, a different shipment); after the
      claim its value is the new shipment ID (re-claim overwrite, attempt 6
      P2-1).
    * Fixture includes a queued member whose preimage has other non-empty
      `custom_fields`; those keys survive unchanged. Key presence is asserted
      with the two-value form (`v, ok := m[key]`).
    * Members that are not queued (for example, already `active` or `done`)
      are byte-identical to their preimage. The shipment artifact differs
      from its preimage only in `status` and `updated_at`.
  * **(2) RED, ordering (F5).** The manifest lists a queued child before its
    queued member-parent (`[child, parent]`), so the bounded cascade activates
    the parent before the loop reaches it. Both child and parent end up
    marked. One pair is enough because the claim loop walks the ordered
    manifest slice, not a map; if a marking loop ever ranges over a map or
    set, use at least 5 independent pairs (per the cited N-independent-pair
    learning).
  * **(3) Characterization, not RED.** A non-claim `setArtifactStatus`
    caller, such as a queue move, produces byte-identical frontmatter before
    and after. The fixture's preimage carries a stale `blocked_reason`, so the
    existing `clearStaleBlockedReason` behavior is characterized too.
  * AC:
    * (1) and (2) fail against pre-implementation code.
    * (3) passes both before and after the implementation and is not counted
      as RED.
* **U0b: RED harness + characterization, external read surface and consumer
  recipe (task 173.007-T; absorbs retired U2).**
  * Domain: tests. Files: one test file in `internal/cli` and one in
    `internal/mcp`.
  * Written before U1 (attempt 6 P1-1). No 173-F dependencies. 3 scenarios.
  * Each RED scenario walks the **exact U5 recipe paths** on one transport,
    after a real claim of a shipment with one queued member:
  * **(1) RED, CLI (frontmatter-backed item read).**
    `backlogit shipment list --status active --format json` returns a JSON
    array of length 1; `.[0].id` is the shipment ID; `.[0].custom_fields.items[]`
    contains the member ID; `backlogit get <member> --format json` has
    `.custom_fields.scheduler_baseline_claim == <shipmentID>`.
  * **(2) RED, MCP (index-backed).** `backlogit_list_shipments`
    `{"status":"active"}` returns an array of length 1 with the same
    `.[0].custom_fields.items[]`; `backlogit_get_item` for the member has
    the identical `.custom_fields.scheduler_baseline_claim` value.
  * **(3) Characterization.** Before any claim, `shipment list --status
    active` returns a present empty array `[]` (not `null`, not a missing
    value) on both transports, and the key is absent on both transports for
    an organic-active item (two-value presence assertion). Written as
    `t.Run` subtests in each package's file.
  * AC:
    * (1) and (2) fail before U1 (the marker is absent) and pass after it.
    * (3) passes before and after.
    * **Post-implementation AC (from retired U2):** the marker reaches both
      transports through the existing generic `custom_fields` projection
      (CLI `internal/cli/get.go` `buildDetailMap`; MCP
      `internal/db/queries.go` `scanArtifactRow`, fed by `UpsertItem`, which
      `persistArtifact` calls after the file write in the same call). No
      bespoke projection field, no marker-specific filter, no read helper, and
      no state mutation is added.
    * There are fewer than 4 scenarios.
* **U0c: RED harness + characterization, claim crash recovery with marked
  members (new task, F1/F4).**
  * Domain: tests. File: `internal/core/shipment_claim_marker_recovery_test.go`
    (new).
  * No 173-F dependencies. 3 scenarios.
  * The tests fabricate the crashed state directly: a `shipment-operation/v1`
    claim **intent** journal (the only phase recovery reconciles,
    `shipment_recovery.go` `reconcileShipmentLifecycleIntent`) with its
    preimage, plus current member files. Mechanism: in-package calls to
    `writeShipmentLifecycleJournalForWorkspace` and `persistArtifact` under
    `withShipmentOperation` (governed ctx for the shipment's active write).
    Close the original workspace before reopening with `NewWorkspace`
    (Windows SQLite and temp-dir cleanup). They do not depend on U1.
  * **(1) RED.** Crash after marking. The shipment is `active` and every
    preimage-queued member is `active` + marked with the journal's shipment
    ID.
    * Fixture includes one member whose **preimage** carries a stale foreign
      marker (`"OLD-S"`); after rollback its value is `"OLD-S"` again,
      byte-for-byte (attempt 6 P2-1).
    * `recoverPendingShipmentOperations`, invoked through a fresh
      `NewWorkspace` open, rolls back successfully.
    * Every member and the shipment are byte-identical to the preimage: no
      marker key where the preimage had none, and `custom_fields` is nil if it
      was nil before the claim.
    * A following `ClaimShipment` of the same shipment succeeds, so nothing
      is wedged.
    * A direct assertion checks that the last claim candidate returned by
      `memberRecoveryCandidates` carries the journal's marker.
  * **(2) RED.** Double-fault partial compensation. The shipment is already
    restored to its `queued` preimage. One member is restored and another is
    still `active` + marked. Recovery succeeds and converges to the preimage.
  * **(3) Characterization, not RED.** A member carrying a marker whose value
    differs from **both** the preimage's own value and `journal.ShipmentID`
    (or otherwise diverged) still fails closed with `ErrShipmentConflict`.
    This guards against over-broad acceptance.
  * AC:
    * (1) and (2) fail today with `ErrShipmentConflict`, which is the
      genuine RED for the recovery surface.
    * (3) passes before and after.
* **U1b: claim-recovery marker compatibility (new task, F1).**
  * Domain: code. Files: `internal/core/shipment_recovery.go`, plus the one
    shared constant line in `internal/core/shipment_lifecycle.go`.
  * Depends on **U0c**.
  * Declare `const schedulerBaselineClaimKey = "scheduler_baseline_claim"` in
    `internal/core/shipment_lifecycle.go`, next to `ClaimShipment` (the
    producer). U1 reuses it. No other change to that file in U1b.
  * In `memberRecoveryCandidates`, the `rollback`+`claim` case for a
    preimage-`queued` member returns three candidates, in this order:
    1. the preimage (it may carry a stale marker, R1);
    2. preimage + `Status=Active`, marker as in the preimage. This is the
       intermediate member-parent cascade window, kept so rollback CAS
       accepts it;
    3. preimage + `Status=Active` + `custom_fields.scheduler_baseline_claim
       = journal.ShipmentID`, **last**. Build `custom_fields` as a fresh map
       copy of the preimage's map (nil → new one-entry map).
  * The marked target is **last**. `validateShipmentLifecycleRecoveryOutcome`
    uses `candidates[len-1]` for committed evidence and `candidates[0]` for
    compensated evidence. For claim, the committed-evidence path is
    **defensive only**: claim emits no correlation-tagged evidence events and
    claim recovery rolls back before any terminal evidence is appended, so the
    ordering keeps the two paths consistent without being reachable today.
  * Nothing changes for non-queued preimage members, for the shipment
    candidates, or for the block, unblock and normalize cases (option A).
  * AC:
    * U0c (1) and (2) turn GREEN and U0c (3) stays GREEN.
    * Existing recovery and UR3 tests stay green.
    * The marker values accepted on a member are **only** the preimage's own
      value (candidates 1 and 2) or `journal.ShipmentID` on the marked-last
      candidate (candidate 3). Any other value fails closed.
* **U1: persist the marker on claim activation (task 173.001-T).**
  * Domain: code. File: `internal/core/shipment_lifecycle.go`.
  * Depends on **U0a**, **U0b** (attempt 6 P1-1) and **U1b**.
  * **Seam form (F5/P3).** Use an **explicit per-call parameter, not a context
    value.**
    * Extract the body of `setArtifactStatus` into an unexported writer that
      takes a `claimMarker string` argument.
    * `setArtifactStatus` calls it with `""` and its behavior is unchanged.
    * The claim loop in `ClaimShipment` calls it with the shipment ID.
    * The seam is never visible to `MoveShipmentStatus` or other downstream
      writes.
    * A shared `applyClaimActivation` helper for U1 and U1b was considered and
      rejected: U1b builds an in-memory candidate, U1 persists an artifact;
      only the key constant is shared.
  * **Ordering-independent marking (F5).**
    * For each member whose preimage status is `queued`, the write sets
      `Status=Active` and the key in **one** `persistArtifact` call.
    * If the member is already `active` (a member-parent the bounded cascade
      activated earlier in the loop), the writer still persists the key. That
      is a **marker-only write**: no status transition, no parent cascade, and
      no `status_changed` event. It is therefore **unaudited** in the event
      log; this is documented in U5.
    * If the key already equals the shipment ID, the marker-only write is a
      no-op (no persist). The comparison uses the artifact as `loadArtifact`
      returns it (index-first), which is consistent in-process because the
      cascade upserts before the loop reaches the member.
    * A marker-only write bumps `UpdatedAt`; recovery candidates 2 and 3
      already set `ignoreUpdatedAt`.
    * The extracted writer keeps every existing side effect of
      `setArtifactStatus`, including `clearStaleBlockedReason`, the event
      append and the bounded cascade (U0a (3) characterizes it).
    * `custom_fields` semantics: `omitempty` applies to the whole map, not
      per key. The writer sets the key explicitly and lazily initializes a nil
      map.
  * **Parent-cascade safety.**
    * The cascade (`cascadePersistedParentStatuses`, bounded by
      `shipmentMemberCascadeBoundaryContextKey`) never writes the key.
    * Non-member parents are neither mutated nor marked.
    * Member-parents are marked only by the loop.
  * **Rollback coupling.** The claim is covered by the existing journaled
    preimage plus `snapshotShipArtifacts` / `restoreShipArtifactsDetailed`,
    which restore exact bytes. No compensating marker write is added (F3).
    `activatedIDs` no longer exists and is not referenced.
  * AC:
    * U0a (1) and (2) and U0b (1) and (2) turn GREEN; U0a (3) and U0b (3)
      stay GREEN.
    * The key is set exactly on preimage-queued manifest members.
    * Non-claim `setArtifactStatus` callers are byte-identical.
    * Non-member parents are not marked.
    * Consumers that ignore the key see no behavior change. This is pinned by
      U0a (3) byte-identity and U0b (3) key absence; there is no separate
      ignoring-consumer test.
* **U2: retired (task 173.002-T closed as superseded).** Folded into U0b's
  post-implementation AC per attempt 6 P1-1. The optional helper is dropped.
* **U3: verification of both claim rollback paths + shipment regression gate
  (task 173.003-T, tests-only).**
  * Domain: tests. File: `internal/core/shipment_claim_marker_rollback_test.go`
    (new).
  * Depends on **U1** and **U1b**.
  * 2 scenarios. Both verify existing exact-restore machinery, are **not
    RED**, and change no production code.
  * Fixture for both scenarios: a shipment with at least 2 preimage-queued
    members, `member1` before `member2` in the manifest.
  * **(1)** In-process snapshot rollback. A real `ClaimShipment` fails after
    `member1` is marked. The failure is injected through the
    `persistArtifactWriteFn` seam with the predicate
    `artifact.ID == member2.ID && artifact.Status == models.StatusActive`, and
    a counter in the seam proves `member1` was written marked before the
    failure. The injected error is a **non-indeterminate** error (a plain error or `ErrWriteNotApplied`); an
    `ErrWriteIndeterminate` must never be used, because the durable-writes
    contract forbids rolling it back (attempt 6 P2-8,
    `docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md`).
    Every member and the shipment are byte-identical to the preimage, with no
    key and the nil map restored.
  * **(2)** Journal-recovery rollback after a real double-fault. The fixture
    is specified exactly (attempt 7 P2-C), adapting the seam style of
    `TestP1C6_ClaimCompensationFailureIsClassifiedAndRecoverable`:
    * the forward write fails on `member2`'s activation with a plain error
      (same predicate as (1));
    * `restoreShipmentSnapshotFn` fails on `member1`'s **artifact file path**
      (`snapshot.file.Path`), not its event log, so `member1`'s file keeps the
      marked, active bytes while its index row is restored.
    * Before recovery, assert R3 at the core level deterministically:
      `member1`'s frontmatter **is** active and marked while `bldb.GetItem`
      **does** show the queued preimage without the key. The test uses the
      injected member ID it already knows; it does not parse the
      `MutationPartialError` cause string.
    * A fresh `NewWorkspace` open recovers through the journal (through U1b),
      converges to the preimage, and a following claim is not wedged.
    * Distinct from U0c: U0c fabricates the crashed state; U3 (2) produces it
      with a real `ClaimShipment` run.
  * AC:
    * Both scenarios pass. No production file is modified.
    * **Shipment regression gate (moved from U4):** `go test -race
      ./internal/core/... ./internal/cli/... ./internal/mcp/...
      ./internal/db/...` is green, with no known-flake rerun policy.
    * A gate failure caused by U1 or U1b **reopens that task**; U3 never
      patches production code.
    * No U0a/U0b/U0c scenario is duplicated. U3 (2)'s distinct value over
      U0c (2) is the real-claim path and the deterministic core-level R3
      disagreement check.
* **U4: DEFERRED to the follow-on feature (task 173.004-T, adopted).**
  Option A lifecycle regression net. Out of the `154-S` manifest.
  * Domain: tests. File: `internal/core/shipment_claim_marker_lifecycle_test.go`
    (new). 2 scenarios. Depends on U1 (`173.001-T`) and U3 (`173.003-T`)
    through task `blocks` edges; the dependency on the retired U2 is removed.
    Routed only under the 074-DL rule after `154-S` ships.
  * **(1) Option A lifecycle invariant.** claim → block → unblock(`active`)
    → `ReturnBlockedItem` of one member. The marker equals the shipment ID on
    every claim-marked member throughout, asserted via **both** the Markdown
    frontmatter and the index row (`bldb.GetItem`). Applying the three-part
    predicate as an in-repo test helper: restored-active members are
    claim-activated; the returned item (`blocked`, out of the manifest) is
    not.
  * **(2) Terminal and generic paths keep the marker.** `ShipShipment`, a
    shipment move to `abandoned`, and a generic `update`/`move` of a marked
    item leave the key byte-identical; with no active shipment, the predicate classifies
    nothing as claim-activated.
  * The attempt-6 U4 (2) claim-crash scenario is **dropped** (attempt 6
    P2-2): the claim terminal-evidence path is unreachable, and the rollback
    path is covered by U0c (fabricated) and U3 (2) (real double-fault).
  * AC: both pass; no U0a/U0b/U0c/U3 scenario duplicated; the `-race` gate of
    U3 is green. U4 stays tests-only: it does **not** edit the U5 doc, whose
    row is worded so it never needs a later edit (see U5).
* **U5: operator docs, the marker contract (task 173.005-T).**
  * Domain: docs. File: `docs/design-docs/scheduler-baseline-marker-contract.md`
    (new, hand-written). Generated `docs/cli-reference/*` is not edited.
  * Depends on **U0b**, **U1** and **U3** (attempt 6 P2-5 as far as the
    bootstrap verifies; U4 is deferred, see the verification-status column).
  * Publish:
    * the pinned key and value shape; the key is reserved but not
      write-protected;
    * a **stability note**: contract version `scheduler-baseline-marker/v1`;
      any change to the key, value shape or predicate is a breaking change
      that needs a new version and a consumer notice;
    * the copy-pasteable read recipe with **exact JSON paths** (the same paths
      U0b asserts):
      1. active shipment: `backlogit shipment list --status active --format
         json` (or MCP `backlogit_list_shipments` with `{"status":"active"}`)
         returns a JSON array. Length 0 → no item is claim-activated. Length
         greater than 1 → indeterminate, fail closed. Length 1 → the shipment
         ID is `.[0].id`;
      2. its manifest: `.[0].custom_fields.items[]` (there is **no**
         top-level `items`);
      3. each item: `backlogit get <id> --format json` or MCP
         `backlogit_get_item`, reading `.status` and
         `.custom_fields.scheduler_baseline_claim`;
    * the **three-part option A predicate** (normative), verbatim from the
      decision artifact. "Non-empty ⇒ claim-activated" is explicitly
      withdrawn;
    * that the marker is advisory, while the backlogit claim gate is
      authoritative;
    * the option A lifecycle table with a **verification-status column**:
      * claim write, re-claim overwrite (including a stale foreign value):
        verified by U0a;
      * removed by rollback (in-process and journal): verified by U3 and
        U0c;
      * marker-only write on an already-active member-parent: unaudited (no
        event);
      * kept through block, unblock, return, ship, abandon, normalize and
        generic update/move: by construction (code-cited); the regression net
        is tracked by the follow-on feature's U4 task, named by ID, and
        consumers read that task's status rather than this doc (so the row
        never needs a later edit);
    * residuals R1–R3, text identical to the decision artifact. R3 is
      published as the split rule (attempt 6 P2-7, refined by attempt 7
      P2-D):
      * **transport side effects:** every CLI read (`get`, `shipment list`)
        opens a recovering workspace (`core.NewWorkspace` runs
        `recoverPendingShipmentOperations`), so a CLI read can roll back a
        pending claim journal as sanctioned, fail-closed convergence, and a
        fresh CLI read almost never shows the frontmatter/index split. MCP
        server **startup** (`openMCPServer` → `core.NewWorkspace`) also
        recovers; only reads on an already-running server do not. Consumers
        that must avoid recovery side effects read through an MCP server that
        is already running and that they did not start;
      * the consumer rule: disagreement between the two reads, a pending claim
        journal under `<storage-root>/ops/` (detect portably with `backlogit
        doctor`), a doctor journal finding, more than one active shipment, any
        non-zero exit or error from a recipe read (in particular an `open
        workspace: recover shipment operations` error), or any MCP tool error
        means indeterminate, so defer to the claim gate; a consumer never
        invokes a lifecycle command;
      * the recipe reads are not atomic: if a re-read of the active shipment's
        ID or manifest differs from the first read, treat the result as
        indeterminate. MCP results arrive as JSON text content that is parsed
        before the paths apply. A missing `custom_fields` or a missing key
        means "not marked";
      * the operator-only remediation: a fresh-process CLI `backlogit sync`
        (its workspace open runs journal recovery), then
        `backlogit_sync_index` if an MCP server is running; on a recovery
        error, `backlogit doctor` and escalate;
    * the mixed-binary caveat: **any** binary that opens this workspace, CLI
      or MCP server, including one a consumer invokes or starts for a read,
      must include U1b;
    * that the autoharness consumer should not treat the contract as
      operative before the rollout checkpoint is recorded (per
      `docs/compound/workflow-issues/stable-contract-before-two-agent-adoption-2026-04-05.md`);
    * that SQL `query` is secondary (`json_extract`);
    * that full defect resolution needs the autoharness follow-up.
  * AC:
    * The doc contains every item above.
    * The predicate and residual text match the decision artifact verbatim.
    * The recipe JSON paths are identical to the paths U0b (1) and (2)
      assert.

## Constitution Check

* **Test-first ordering (non-negotiable).** Each implementation unit has a
  genuine RED predecessor, enforced by a dependency edge:
  * U0a (1)/(2) → U1;
  * U0b (1)/(2) → U1 (attempt 6 P1-1; edge `173.001-T` → `173.007-T`);
  * U0c (1)/(2) → U1b.

  Characterization scenarios are labeled and not counted as RED. U3 (and the
  deferred U4) are post-implementation verification. U1b precedes U1. The
  retired U2 no longer claims a RED. Pass.
* **Single-domain tasks.** Tests: U0a, U0b, U0c, U3 (and deferred U4). Code:
  U1b, U1. Docs: U5. Pass.
* **2-hour rule.** Every unit touches at most 2 files, fewer than 5 functions,
  and at most 3 scenarios (see `## Size Validation and Decomposition`). U1b's
  second file is a one-line constant. Pass.
* **Release-unit size.** The bootstrap slice is 7 tasks in 5 waves, under the
  ~8-task ceiling; the deferred U4 moves to a follow-on feature with no
  shipment until a later Stage session. Pass.
* **Backward compatibility.** Default claim behavior is unchanged apart from
  the additive key on preimage-queued members. Non-claim writers are
  byte-identical. Recovery still fails closed on every divergence except the
  preimage's own marker or this journal's marker. Pass.
* **Workspace containment (P-017, Principle IV).** All agent work is in-repo.
  The only out-of-repo step, the binary install in the rollout checkpoint, is
  an **operator action, not an agent action** (Principle IV exception).
  Autoharness consumption is a documented follow-up. Pass.
* **Other principles (attempt 7 P3).** Errors wrap with `%w` and U1b keeps the
  `ErrShipmentConflict` sentinel (I). The unaudited marker-only write is
  documented in U5 (V). The U1b recovery change and the rollout checkpoint
  run in careful mode (VII/VIII). The marker lives in Git-friendly
  frontmatter (IX). Pass.

Constitution Check: pass

## Plan Hardening

**Required: yes.** The triggers are:

* the lifecycle-state schema addition;
* the shared status-write seam;
* the claim crash-recovery CAS (a fail-closed gate on every lifecycle
  operation);
* a cross-workspace consumer contract;
* a mixed-binary rollout window (new in attempt 7).

Re-hardened on 2026-09-29 for attempt 7 against `main` `7e4041ee`.

**Protected invariants**

* Claim stays all-or-nothing through the journaled preimage and exact
  snapshot restore.
* Recovery never accepts a state it cannot prove came from the preimage or
  from this journal.
* Non-claim writers are byte-identical.
* There is at most one active shipment.
* The backlogit claim gate stays authoritative, and the marker stays
  advisory.

**Learnings consulted**

* `docs/compound/2026-07-23-machine-readable-governance-field-contract.md`:
  pinned key.
* `docs/compound/2026-08-01-n-independent-pair-test-design-for-go-map-iteration-nondeterminism.md`:
  cascade and ordering.
* `docs/compound/2026-07-29-durable-writes-test-seam-patterns.md`:
  `persistArtifactWriteFn` failure injection, no `t.Parallel` with
  package-global seams, `t.Cleanup` restore.
* `docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md`:
  inject only non-indeterminate errors when testing rollback (U3).
* `docs/compound/2026-08-01-self-hosted-cli-version-skew-merged-fix-not-yet-operative.md`:
  mixed-binary rollout checkpoint.
* `docs/compound/best-practices/atomic-multi-item-claim-rollback-and-stale-blocked-clearing-2026-06-27.md`:
  all-or-nothing claim rollback kept; clear-on-state-exit waived for the
  advisory marker (see the option A section); `clearStaleBlockedReason` kept
  by U1's extracted writer.
* `docs/compound/2026-07-20-ship-gate-descoped-archived-member-exemption.md`
  and
  `docs/compound/2026-07-31-p015-single-artifact-safe-close-for-partial-feature-shipments.md`:
  how the retired and deferred tasks leave `154-S` (see `## Harvest
  Checklist`).
* `docs/compound/2026-07-13-post-merge-lifecycle-requires-fresh-binary.md`:
  prove the installed binary's commit by ancestry, not by file time.
* `docs/compound/workflow-issues/stable-contract-before-two-agent-adoption-2026-04-05.md`:
  versioned contract, not operative before the checkpoint.

**Risky actions**

* **ProposedAction:** widen the claim recovery candidates to accept this
  journal's marker (U1b).
  * **ActionRisk: high.** A mistake either wedges every lifecycle operation
    (too strict) or accepts a forged or divergent state (too loose).
  * Mitigation:
    * The only marker values accepted are the preimage's own value and
      exactly `journal.ShipmentID` on the marked-last candidate.
    * U0c (3) keeps foreign or other divergence fail-closed.
    * U0c (1) proves a stale foreign preimage value is restored
      byte-for-byte.
    * The candidate order is pinned: the marked target is last.
    * U1b lands before U1.
  * No operator approval is needed beyond plan review.
* **ProposedAction:** write the marker from the claim loop through an
  explicit per-call parameter (U1).
  * **ActionRisk: medium.** It touches the shared status-write body.
  * Mitigation:
    * `setArtifactStatus`'s signature and behavior are unchanged.
    * There is no context-value inheritance.
    * U0a (3) characterizes non-claim callers.
    * A marker-only write on an already-active member emits no
      `status_changed` event and does not cascade.
* **ProposedAction:** publish the three-part consumer predicate (U5).
  * **ActionRisk: medium.** It is a cross-workspace contract.
  * Mitigation:
    * The predicate and residuals are copied verbatim from the decision
      artifact.
    * The recipe paths are test-pinned by U0b.
    * The contract is versioned (`scheduler-baseline-marker/v1`) and
      advisory-only.
* **ProposedAction:** run the first claim after the `154-S` implementation
  merge (the `154-S` claim itself happens **before** that merge, with a
  pre-U1 binary, so it writes no marker; the first marker-writing claim is the
  next one, most likely the later U4 follow-on shipment).
  * **ActionRisk: high** while binaries are mixed. A post-U1 binary that
    crashes mid-claim, followed by recovery from a pre-U1b binary, re-creates
    the F1 wedge.
  * Mitigation: the rollout checkpoint below. **Operator approval required**
    (the existing `154-S` supervised-bootstrap condition).

**Rollout checkpoint (attempt 6 P2-9; attempt 7 P2-E).** After the `154-S`
implementation merge, the change is **merged but not operative** until every
binary that can touch this workspace has been rebuilt from a commit that
contains both U1b and U1. Steps 1–3 are performed **by the operator only**
(they write outside the repository and replace installed binaries); no agent
runs them:

1. stop the MCP server;
2. rebuild and install the PATH CLI and the MCP server binary;
3. prove each binary's stamped commit descends from the merge with `git
   merge-base --is-ancestor <merge-sha> <stamped-commit>`; a dirty or missing
   stamp fails closed; file modification time is never evidence;
4. only then may any `shipment claim` run.

Ship (or the Orchestrator) only checks and records the operator-reported
version strings and ancestry result on `154-S` closure. Until it is recorded,
the marker contract is not operative and no consumer may rely on it.

**Verification added**

* The U0c recovery RED, including the stale-foreign preimage restore.
* U0a re-claim overwrite of a stale foreign value.
* U0b recipe-path assertions on both transports.
* U3 exact-restore on both rollback paths, with the correct error class, and
  the core-level R3 assertion.
* The widened `-race` gate (U3 AC).
* Deferred: U4 option A lifecycle net (follow-on).

**Rollback**

* **Mid-claim:** unchanged, exact restore. Journal recovery restores the
  preimage, including through U1b for marked members.
* **Feature-level revert (corrected per attempt 6 P2-4):**
  * Revert U1 first, then U1b. A U1 revert alone is safe for recovery,
    because U1b only widens acceptance.
  * A U1 revert **withdraws the contract**: later claims no longer write
    markers, so nothing is overwritten, and markers already on disk become
    **unreliable**, not inert (an organically re-activated item could match a
    later shipment ID only by coincidence, but the predicate can no longer be
    trusted to find claim-activated items).
  * **Operator step:** before or with the revert, notify the autoharness
    consumer that `scheduler-baseline-marker/v1` is withdrawn, so it stops
    consuming the marker.
  * Reverting U1b while marked claim **intent** journals exist on disk would
    re-wedge. **Operator checkpoint:** stop the MCP server first, then confirm
    that `.backlogit/ops/` contains no pending claim lifecycle journal
    (`shipment-operation/v1` files with `operation: claim` and
    `phase: intent`) before reverting U1b.

**Monitoring and closure**

* After ship, `backlogit doctor` reports no shipment lifecycle evidence
  conflicts.
* The first claim after the rollout checkpoint shows the key on its members
  through both transports.
* The owner is Ship for `154-S`. The validation window is the `154-S`
  supervised bootstrap plus the first post-merge claim.
* The deferred U4 follow-on is routed under the 074-DL rule like everything
  else behind `154-S`; it is **not** a precondition of the attestation.

**Review-gate markers**

* Plan review must emit the literal `dispatch_mode:` and `decision:` fields.
* If sub-agent dispatch is unavailable, declare
  `single-agent-declared-degradation` (P-012 principle).

**Unresolved operator decisions:** none for the design. F2 is decided
(option A); the size decomposition follows the operator's 2026-09-28
instruction. Proceeding to harvest on the attempt-7 ADVISORY gate requires
the operator's `operator_authorization: approved`.

## Verification

* Per unit, as listed in the ACs.
* Shipment gate (U3 AC): `go test -race ./internal/core/... ./internal/cli/...
  ./internal/mcp/... ./internal/db/...`. `-race` runs on this Windows host:
  CGO is on and MinGW gcc is available.
* **Flake prerequisite: satisfied.**
  `TestUR3_ReopenRollsBackInterruptedBlockFromCompletePreimage` (stash
  `BDA56ED8`) was fixed by feature `181-F` / task `181.001-T` in shipment
  `182-S`, which **shipped** (`archived_status: shipped`, merge `70d72044`).
  * `154-S` blocks-depends on `182-S` (retained as provenance).
  * Existing task edges onto `181.001-T` from `173.006-T`, `173.001-T`,
    `173.003-T` and `173.004-T` are retained as provenance. New tasks (U0c,
    U1b) do not need an edge, because the prerequisite has shipped.
  * No known-flake rerun policy is used.
* Existing shipment lifecycle, recovery and UR3 tests stay green.
* **Shipment merge gate (constitution quality gates):** `go test ./...`,
  `go vet ./...`, `golangci-lint run` and `gofmt -l .` (empty) all pass before
  the `154-S` implementation PR merges, in addition to the U3 `-race` gate.

## Harvest Checklist (on gate authorization)

Performed by Stage only after the gate is satisfied (PASS, or ADVISORY with
`operator_authorization: approved`), and **before** the `154-S` hold label is
removed (attempt 7 P2-B):

1. Create the follow-on feature ("Scheduler-baseline marker: option A
   lifecycle regression net (173-F follow-on)"), referencing this plan.
2. On `173.004-T`, **before** adopting it, remove its `173.002-T` edge
   (its `173.001-T` and `173.003-T` edges stay). Then reparent it under the
   follow-on feature with `backlogit adopt`, which assigns a new hierarchical
   ID and rewrites its remaining edges. Update the adopted task's contract to
   the deferred U4 text above. No follow-on shipment.
3. Retire `173.002-T`: remove its edge from `173.005-T` (the only remaining
   dependent after step 2), update its body with the supersession
   provenance, then archive it **directly from `queued`** (so
   `archived_status: queued` qualifies for the ship-gate descope
   exemption).
4. Create U0c and U1b as tasks under `173-F`. Edges: U1b → U0c;
   `173.001-T` → U1b; `173.001-T` → `173.007-T` (P1-1).
5. Rewrite `173.005-T` dependencies to `173.007-T`, `173.001-T` and
   `173.003-T`; rewrite `173.003-T` to depend on `173.001-T` and U1b. Update
   the contracts of `173.001-T`, `173.003-T`, `173.005-T`, `173.006-T` and
   `173.007-T` to this plan's text, and the `173-F` description.
6. Edit the `154-S` manifest to exactly: `173-F`, `173.006-T`, `173.007-T`,
   U0c, U1b, `173.001-T`, `173.003-T`, `173.005-T` (remove `173.002-T` and
   `173.004-T`).
7. Pre-ship check, recorded in the `154-S` banner for Ship: at ship time no
   child of `173-F` outside the manifest may be non-terminal (the adopted U4
   is no longer a child; `173.002-T` is archived from `queued`).
8. Archive stash `C29EBEE5` with provenance; remove only
   `do-not-claim-until-convergence` from `154-S`; keep
   `bootstrap-bypass-approved-conditional`; update the banner.

## Follow-ups (out of this shipment)

* **Follow-on feature (created at harvest):** the option A lifecycle
  regression net (U4, `173.004-T` adopted under it). No shipment now; a later
  Stage session packages it after `154-S` ships, under the 074-DL rule.
* The autoharness P-002.6 wave scheduler should apply the three-part
  predicate and exclude claim-activated tasks from residual classification
  (cross-workspace).
* Label-aware claim refusal: `AF1E5075`, already deferred.

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: FAIL

Attempt 1. Personas: Correctness + Architecture Strategist. Coverage complete.

**Gate rationale**: one P1 finding → FAIL per rubric.

Findings:
* **P1 (Correctness)** — marker/rollback invariant gap: `rollbackShipmentClaim`
  reverts status but would not clear a claim-activated marker, leaving a torn
  queued-but-marked item that misleads the scheduler accessor.
* **P2** — record-only semantics ambiguous (redundant marker-set vs a rejected
  second claim path). Pin down as a flag on `ClaimShipment`.
* **P2** — marker persistence must be atomic within the `setArtifactStatus`
  activation write, inside `activatedIDs` rollback tracking.
* **P2** — per-unit acceptance criteria missing.
* **P3** — Objective overstated the delivered outcome (enabling precondition,
  not defect resolution).

Plan hardening required: yes; present. Remediation applied in this revision
(added U3 rollback-clears-marker, atomic-write requirement in U1, record-only as
a flag in U2, per-unit AC, enabling-precondition labeling). Re-review below.

<!-- plan-review-attempt: 1 -->

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: PASS

Attempt 2. Personas: Correctness + Architecture Strategist (re-review). Coverage
complete. Prior P1 (marker/rollback torn state) confirmed resolved via U3
(atomic rollback-clears-marker). The attempt-2 P2 (U1/U2 contradiction on when
the marker is written) is resolved in this revision: U1 now sets the marker on
EVERY claim activation via a gated off-by-default `setArtifactStatus` seam; the
redundant "record-only mode" is dropped and U2 reduced to the read accessor;
rollback clear is atomic within the revert write; U5 gained an AC.

**Gate rationale**: no P0/P1/P2 remain; residual items were folded into the
revision. Plan hardening required: yes; present and adequate.

<!-- plan-review-attempt: 2 -->

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: PASS

Attempt 3 (review-fix cycle 1). Personas dispatched (7, coverage complete):
Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Learnings Researcher,
Architecture Strategist, Security Lens Reviewer, Agent-Native Parity Reviewer.

**Findings and remediation:**
* **P1 (Learnings Researcher)** — the claim-activation marker ignored two prior
  solutions: (a) machine-readable governance-field contract
  (`docs/compound/2026-07-23-machine-readable-governance-field-contract.md`) — the
  exact `custom_fields` key/format consumed by the out-of-repo P-002.6 scheduler
  was unpinned; (b) the `setArtifactStatus`→`cascadePersistedParentStatuses`
  parent cascade
  (`docs/compound/2026-08-01-n-independent-pair-test-design-for-go-map-iteration-nondeterminism.md`)
  was unaccounted. **REMEDIATED**: U1 (and task 173.001-T) pin the marker to the
  single key `scheduler_baseline_claim` (value = activating shipment ID) referenced
  by U0a/U0b/U2/U5, and require the marker be written ONLY on activated manifest
  items with no cascade propagation to parents.
* **P2 (Agent-Native Parity — codebase-verified)** — the marker rides the EXISTING
  generic `custom_fields` projection (CLI `get --format json` `buildDetailMap`;
  MCP `get_item` `scanArtifactRow`), so no bespoke projection field must be added;
  the two transports have different freshness guarantees. **REMEDIATED**: U2
  reworded to verification over the existing projection (no side channel) + index
  freshness (upsert `custom_fields` in the same op); U0b/173.007-T assert the
  pinned key on BOTH the frontmatter-backed CLI and DB-backed MCP surfaces; U5/
  173.005-T publish the concrete read recipe and mark SQL `query` as secondary.

**Gate rationale**: after remediation no P0/P1/P2 remain. The plan ships a genuine,
supported cross-workspace read surface (verified in `internal/cli/get.go`,
`internal/mcp/tools.go`, `internal/db/queries.go`) with a pinned field contract and
both-transport parity; unmarked items stay byte-identical. Plan hardening required:
yes; present and adequate. Honestly scoped as an enabling precondition (full defect
resolution needs the autoharness follow-up).

<!-- plan-review-attempt: 3 -->

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: PASS

Review-fix cycle 2 (staging PR #442). Re-reviewed after reconciling Copilot
comments 5, 6, and 10. Item 5 (U0a / 173.006-T): marker activation and rollback
are genuine RED; the seam byte-identical / non-claim-caller case is explicitly
passing characterization/regression, not required RED. Item 6 (U0b /
173.007-T): CLI/MCP marker projection cases are genuine RED; absence/ignore
cases are explicitly passing characterization. Item 10 (173-F): summary uses the
existing generic CLI/MCP `custom_fields` projection with no bespoke read
accessor. RED-vs-characterization labeling internally consistent. No P0/P1
findings.

<!-- plan-review-attempt: 4 -->

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: FAIL

Attempt 5: the `C29EBEE5` re-validation (Stage, 2026-09-28, branch
`stage/c29ebee5-154s-revalidation`). The plan and the 173-F task contracts
were re-checked against `main` `ba303ee2`, i.e. after 174-F shipped (`155-S`,
PR #450, merge `2c8759c3`).

Six personas were dispatched and all returned: Architecture Strategist, Go
Reviewer, Scope Boundary Auditor, Learnings Researcher, Constitution Reviewer,
and Agent-Native Parity Reviewer. Security Lens was not triggered, since the
plan touches no auth, secrets, or external integration. Each persona returned
FAIL. Where personas disagreed, the more conservative severity was kept.

**Gate rationale.** There are P1 findings that need a design decision and new
work, not just contract-text fixes, so the gate is FAIL. The attempt-2..4 PASS
verdicts were given against the pre-174-F `ClaimShipment` and no longer hold.

* Plan hardening is still required. The `## Plan Hardening` section is
  present, but its rollback bullet describes a mechanism that no longer exists,
  so it has to be re-hardened.
* The `Constitution Check: pass` verdict is stale:
  * U0a scenario (3) cannot be a genuine RED.
  * The recovery path has no RED.
  * Backward compatibility is broken on the interrupted-claim path (see F1).

**What changed in the code (verified on `ba303ee2`)**

* `ClaimShipment` (`internal/core/shipment_lifecycle.go:51-178`) now:
  * records a preimage and a `shipment-operation/v1` intent journal;
  * snapshots the file, the index row, and the event log of every locked
    artifact (`snapshotShipArtifacts`).
* `activatedIDs` no longer exists.
* `rollbackShipmentClaim` (`:183-225`) restores those snapshots exactly
  (`restoreShipArtifactsDetailed`). There is no
  `setArtifactStatus(StatusQueued)` revert write any more.
* The cascade out of the claim loop is bounded to manifest members by
  `shipmentMemberCascadeBoundaryContextKey`.
* `BlockShipment` / `UnblockShipment` (`internal/core/shipment.go:382`, `:627`)
  and `ReturnBlockedItem` (`:1572`) write members with
  `cloneArtifact` + `persistArtifact`. That preserves `custom_fields`.
* `ShipShipment` now always returns `ReturnedIDs: []`
  (`shipment_lifecycle.go:692`).

**Findings**

* **F1 (P1, new work; 173.001-T, 173.006-T): crash-recovery CAS wedge.**
  * `memberRecoveryCandidates` (`internal/core/shipment_recovery.go:~938`)
    builds the claim target as the preimage clone with only `Status=Active`.
    `recoveryArtifactMatchesAny` (`:~758`) compares full JSON and ignores only
    `UpdatedAt`.
  * So a member that was marked before a crash, or a partial compensation,
    matches no candidate and recovery returns `ErrShipmentConflict`. The same
    happens in `validateShipmentLifecycleRecoveryOutcome`.
  * Claim, Block, Unblock, ReturnBlocked and Ship all run
    `recoverPendingShipmentOperations` first. One interrupted marked claim
    would therefore block the whole shipment lifecycle until an operator
    repairs it by hand.
  * No 173-F unit names `shipment_recovery.go`. Adding it to U1 breaks the
    2-hour file budget, so this needs a new unit (U1b) with its own RED
    harness. The fix: for preimage-queued members, the claim recovery target
    also carries `scheduler_baseline_claim = journal.ShipmentID`, in both the
    CAS path and the outcome path.
* **F2 (P1, design decision; 173-F, 173.005-T, 173.007-T): the marker
  lifecycle and the consumer rule are undefined.**
  * Because block, unblock and return preserve `custom_fields`, the marker
    survives:
    * Block leaves members queued but still marked.
    * Unblock to active restores them active and still marked. That refutes
      the C29EBEE5 hypothesis that restored members would be "active +
      unmarked".
    * Unblock to queued leaves them queued and still marked.
    * `ReturnBlockedItem` leaves a stale marker on an item that is no longer
      in the named shipment.
  * The published rule "non-empty marker ⇒ claim-activated" (U5) then
    misclassifies organic activations of stale-marked items. That is the
    inverse of the P-002.6 defect.
  * Pick one:
    * **(A)** Keep the marker through block, unblock and return (no new write
      paths), and harden the consumer rule to: `status == active` AND marker
      == the currently active shipment ID AND the item is in that shipment's
      current manifest.
    * **(B)** Clear the marker on block and return, and re-mark on unblock to
      active. This changes Block, Unblock and ReturnBlocked and their
      recovery candidates.
  * Scope Boundary Auditor and Parity Reviewer recommend (A).
* **F3 (P1/P2, text fix plus re-scope; 173.003-T).** The references
  (`:100-133`, `~104-107`) and the revert-write mechanism are stale. Snapshot
  restore, and the journal-recovery rollback that persists preimage clones,
  already clear the marker and restore a nil map byte-for-byte.
  * Make U3 a verification-only check of both rollback paths, or retire it.
  * Do not add a second compensating write.
  * "Even if rollback fails midway" now depends on journal recovery, so U3
    must depend on U1b.
* **F4 (P1, test-first; 173.006-T).** Scenario (3), "rollback clears the
  marker", passes both before and after the implementation. It cannot be a
  genuine RED, so reclassify it as characterization. The genuine RED for the
  rollback/recovery surface is F1's recovery of a marked member, which fails
  today.
* **F5 (P2, design; 173.001-T): the marker is order-dependent for
  member-parents.**
  * When a child member comes before its member-parent in the manifest,
    `cascadePersistedParentStatuses` activates the parent first
    (`ComputeParentStatus`: an active child makes the parent active).
  * The claim loop's `setArtifactStatus` then returns early on equal status
    (`:~965`), so the gated option never writes the marker.
  * Mark by "was queued in the preimage and is a manifest member" instead of
    relying on the status-write seam, or require parent-first manifests, and
    add an AC for this.
  * `154-S` itself lists `173-F` first, so the 154-S bootstrap does not hit
    this.
  * Seam form (P3): use a per-call option that is not inherited, rather than
    a context value that `MoveShipmentStatus` or other downstream writes would
    also see.
* **F6 (P2; 173.004-T).**
  * The "no queued-but-marked" invariant is false under current block
    semantics. Restate it once F2 is decided.
  * Scenario (1) (concurrent claim + rollback) mostly exercises the global
    lifecycle lock. Replace it with a crash-recovery scenario: an intent
    journal where some members are already marked, run through both the
    rollback path and the terminal-evidence path.
  * The verification command `go test ./internal/core/... -race` does not
    cover the U0b/U2 tests in `./internal/cli/...`, `./internal/mcp/...` and
    `./internal/db/...`. Widen it.
  * AC(3), "existing lifecycle tests stay green", runs the same package as
    the flaky `TestUR3_ReopenRollsBackInterruptedBlockFromCompletePreimage`
    (stash `46A898B8`; probable duplicate `BDA56ED8`). Fix it first, or
    declare a known-flake rerun policy.
* **F7 (P2; 173.007-T).** The task has 4 scenarios, which breaks the
  "fewer than 4" rule; the 2026-09-14 review missed this. Scenario (4)
  (ignoring consumer) has no in-repo consumer. Drop it, or swap in a
  stale-marker case whose shape depends on F2.
* **F8 (P2; 173-F, plan Objective).**
  * "In-repo (`shipment_lifecycle.go`) only" and "Reviewed PASS (attempt 2)"
    are stale.
  * Partial compensation restores the file and the index row independently,
    so the CLI and MCP transports can disagree for the IDs it reports. U5
    must document this, with `sync` as the remedy.
* **173.002-T: PASS.** These references are still valid:
  * `internal/cli/get.go:97` `buildDetailMap`;
  * `internal/db/queries.go:38` `scanArtifactRow`;
  * `:129` `UpsertItem`;
  * `persistArtifact`, which writes the file and then upserts the index.

  Advisory P3: say "file then index within one `persistArtifact` call", and add
  a stale-marker AC once F2 is decided.

**Executability**

* Stash `24D693E1` (GOOS=linux harness commands) does not affect 173-F. No
  173-F task or plan section uses `GOOS=linux`.
* Stash `46A898B8` does affect 173.004-T AC(3); see F6.
* `-race` runs on this Windows host: CGO is enabled and MinGW gcc is on PATH.

**Installed binary (C29EBEE5 condition): satisfied.**

* MCP server: `v1.10.0-1023-g2c8759c3-dirty-debug` at commit `2c8759c3`.
* PATH CLI: `C:\Tools\backlogit.exe` v1.11.0 at commit `131577c`, which
  descends from `2c8759c3`.
* Caveat: the MCP server is a local dirty debug build.

**Required before the next review attempt**

1. Deliberate F2, the marker lifecycle, as an operator decision (Step 2,
   P-021 C6).
2. Revise U1: remove `activatedIDs`, define the ordering-independent marking
   rule (F5), and choose the seam form.
3. Add U1b plus its RED (F1).
4. Re-scope U3 (F3), U0a (F4), U4 (F6), U0b (F7) and U5 (F2/F8).
5. Re-harden, re-run the Constitution Check, and re-run plan-review.
6. Update the harvest: task text, new task(s), dependency edges, and the
   `154-S` manifest.

The `154-S` hold label stays until a later attempt passes.

<!-- plan-review-attempt: 5 -->

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: FAIL

Attempt 6 was run by Stage on 2026-09-28 on branch
`stage/173f-option-a-replan`, with code at `main` `a7462173`. It reviewed
the option A re-plan, which is the body above.

Six personas were dispatched and all six returned:

| Persona | Verdict |
|---|---|
| Constitution Reviewer | FAIL (1 P1) |
| Go Reviewer | ADVISORY |
| Scope Boundary Auditor | ADVISORY |
| Learnings Researcher | ADVISORY |
| Architecture Strategist | ADVISORY |
| Agent-Native Parity Reviewer | ADVISORY |

Security Lens was not triggered: the plan touches no auth, secrets, or new
external integration. Where personas disagreed, the more conservative
severity was kept.

**Gate rationale.** There is one P1, a test-first ordering violation against
a non-negotiable constitutional principle, so the gate is FAIL.

* Plan hardening was required and is present. It needs the P2 corrections
  below.
* The `Constitution Check: pass` verdict is wrong because of P1-1.
* **All attempt-5 findings F1–F8 were confirmed resolved in substance** by
  every persona that checked them. F6 is only partly resolved: its
  replacement scenario U4 (2) is not feasible (P2-2).

**Circuit breaker.** Attempts 5 and 6 are two consecutive FAILs, so the
Stage re-entry limit is reached. Stage halted re-planning for `173-F` and
escalated (P-013.6). No further attempt is made without operator
intervention.

The harvest was **not** updated:

* the `173.00x-T` contracts are unchanged;
* no U0c or U1b tasks were created;
* the `154-S` manifest is unchanged;
* `do-not-claim-until-convergence` stays on `154-S`;
* `C29EBEE5` stays active.

### P1

**P1-1 (Constitution Reviewer): U0b's RED is not ordered before the code that
turns it GREEN.**

* U2 changes no production code. The marker reaches both transports as soon
  as U1's `persistArtifact` runs, so U1, not U2, is what makes U0b (1) and (2)
  pass.
* U1 depends only on U0a and U1b. Nothing forces U0b to be written first, so
  the U0b AC "fails before the implementation" may never be observable.
* The Constitution Check mapping "U0b → U2" is therefore wrong.

**Remediation:**

* Add U0b (`173.007-T`) as a dependency of U1 (`173.001-T`).
* Map U0b (1)/(2) → U1.
* Relabel U2 as tests-only verification, or fold its AC into U0b's
  post-implementation AC.
* Drop U2's optional helper, which has no RED and no consumer.

### P2 (merged)

* **P2-1 (Constitution, Architecture, Go): U1b's "never accepted" AC is too
  strict.**
  * Candidates 1 and 2 are clones of the preimage. Under option A, a
    preimage can legitimately carry a stale marker (R1), for example an item
    returned from shipment A and later claimed in shipment B.
  * Taken literally, the AC would re-wedge re-claims.
  * **Remediation:**
    * Reword to "accepts only the preimage's own value, or
      `journal.ShipmentID` on the marked-last candidate".
    * U0c (3)'s foreign marker must differ from the preimage's value.
    * Add a stale-foreign-marker preimage fixture to U0a (1), to test the
      re-claim overwrite, and to U0c or U3, to test that recovery restores
      the stale value byte-for-byte.
* **P2-2 (Go, Scope, Architecture): U4 (2) cannot be executed as written.**
  * The claim terminal-evidence path cannot be reached:
    * claim emits no `correlation_id`-tagged events (`shipment.go` ~:1150-1167);
    * claim recovery returns before appending terminal evidence;
    * `recoverPendingShipmentOperations` and
      `reconcileShipmentLifecycleIntent` skip non-`intent` journals
      (`shipment_recovery.go:128`).
  * The UR3 crash-child harness only drives `block` and `unblock`, and it
    fires on the first persisted write, which for a claim is the shipment
    move.
  * **Remediation:**
    * Re-specify U4 (2) as an in-process claim that fails at the Nth member
      write, followed by `NewWorkspace` reopen and recovery. Distinguish it
      from U3 (2).
    * Otherwise, drop it.
    * State that U1b's marked-last `[len-1]` is defensive.
    * Add U3 to U4's no-duplication AC.
    * Move the regression gate from a scenario to an AC (Scope N2).
* **P2-3 (Architecture, Go, Scope): the residuals are incomplete and
  partly inaccurate.**
  * `ReturnBlockedItem` leaves the item `blocked` (`shipment.go` ~:1622-1627),
    not `queued`.
  * The organic activation in R2 cannot happen "while blocked", because
    blocked members are guarded.
  * The general residual is: any activation route for an item that carries
    the active shipment's marker and is in its manifest reads as
    claim-activated. One such route is `ReturnBlockedItem`, then organic
    activation, then `AddItemToShipment` back onto the active shipment.
  * **Remediation:** fix the decision table and restate R2.
* **P2-4 (Architecture): the revert guidance is wrong.**
  * After a U1 revert, re-claims no longer write markers, so nothing gets
    overwritten.
  * The revert withdraws the contract, and any markers already on disk become
    unreliable, not inert.
  * **Remediation:** add an operator step to notify the autoharness consumer.
* **P2-5 (Architecture):** U5 must depend on U4, since U5 publishes what U3
  and U4 verify.
* **P2-6 (Parity): the U5 recipe has an incorrect path.**
  * The manifest is at `[].custom_fields.items[]`, not a top-level `items`.
    A literal consumer would classify everything as organic.
  * **Remediation:**
    * Pin exact JSON paths for all three reads.
    * Add the 0 / more-than-1 active-shipment rule (fail closed).
    * Add an AC that the recipe paths match the transport output.
* **P2-7 (Parity): R3 understates the freshness split.**
  * `shipment list` is index-backed on both transports.
  * "Run any lifecycle operation" is not safe to give an external agent as
    an instruction.
  * **Remediation:**
    * Split R3 into a consumer rule and an operator remediation.
    * Consumer rule: treat disagreement, a pending claim journal under
      `.backlogit/ops/`, or a doctor conflict as indeterminate; defer to the
      claim gate.
    * Operator remediation: name one specific lifecycle trigger, then
      `backlogit sync` / `backlogit_sync_index`.
* **P2-8 (Learnings,
  `docs/compound/2026-07-28-durable-writes-two-class-contract-commit-then-surface.md`):**
  U3 (1) must inject a non-indeterminate error (plain, or
  `ErrWriteNotApplied`). `ErrWriteIndeterminate` must never be rolled back.
* **P2-9 (Learnings,
  `docs/compound/2026-08-01-self-hosted-cli-version-skew-merged-fix-not-yet-operative.md`):
  mixed binaries can re-create the F1 wedge.**
  * A post-U1 binary that crashes mid-claim, followed by a pre-U1b binary
    (the MCP server and PATH CLI currently differ), re-creates the wedge.
  * **Remediation:**
    * Add a rollout checkpoint: upgrade every binary before the first claim.
    * Record the change as "merged but not operative" until that is done.
* **P2-10 (Learnings,
  `docs/compound/best-practices/atomic-multi-item-claim-rollback-and-stale-blocked-clearing-2026-06-27.md`):**
  cite this learning and explicitly waive its clear-on-state-exit rule for
  the advisory marker. Add the generic `UpdateArtifact`, `move` and `update`
  paths to the "kept" table.

### P3 (advisory)

* **U0a (1) wording.**
  * The shipment also gets a new `UpdatedAt`.
  * Fixtures must avoid non-queued member-parents that have queued member
    children.
* **U1.**
  * The marker-only write skips the cascade and emits no event (documented
    as unaudited).
  * It is a no-op when the key already equals the shipment ID.
  * Consider one helper, `applyClaimActivation`, shared by U1 and U1b, with
    the constant next to the producer.
* **U1b.** Name the file for the constant.
* **Candidate 3.** Build it from a fresh map copy.
  * Stage verified that `cloneArtifact` already uses `maps.Clone`
    (`shipment.go:2274`), so the top-level aliasing concern is moot.
  * Still, add a fixture with a non-empty preimage `custom_fields`, and use
    two-value key-presence assertions.
* **Index projection.** Stage verified that `projectedCustomFields`
  (`internal/db/upsert_projection.go:113`) only fills dedicated columns and
  does not filter the `custom_fields` JSON. Assert via Markdown and via the
  index in U4 (1). Cite why Ship and normalize are out of scope.
* **Parallel-safe seams.** State "no `t.Parallel`, restore via
  `t.Cleanup`" in U3 and U4.
* **U3 (2).** Assert R3 at the core level, frontmatter against
  `bldb.GetItem`, not through CLI or MCP.
* **U5.**
  * Say the key is reserved but not write-protected.
  * Add a stability or version note for the cross-workspace contract.
  * Cover the Abandon row and multiple-active-shipment edge cases.
  * Optionally, add a transport-level predicate test.
* **Revert checkpoint wording.** Name the journal files, and stop the MCP
  server first.
* **Frontmatter.** `docline.status` was `approved` while the latest review
  was FAIL. It is now `revised`.

### Operator decision required to continue

Authorize attempt 7, which is past the circuit breaker. The remediation is
mechanical:

* one dependency edge plus a Constitution Check mapping (P1-1);
* U4 (2) re-specification or drop (P2-2);
* text corrections for P2-1 and P2-3 through P2-10.

It needs no new design decision. Option A stands.

<!-- plan-review-attempt: 6 -->

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: ADVISORY

Attempt 7 was run by Stage on 2026-09-29 on branch
`stage/173f-plan-review-attempt-7`, with code at `main` `7e4041ee`. The
operator authorized it past the circuit breaker on 2026-09-28 ("Yes on
additional plan review; validate the size of the plan; if too large,
decompose."). It reviewed the attempt-7 revision: every attempt-6 remediation
applied, the new `## Size Validation and Decomposition` (9 tasks → a 7-task,
5-wave bootstrap slice; U2 retired into U0b; U4 deferred to a follow-on
feature), and re-hardening (mixed-binary rollout checkpoint, corrected revert
guidance).

Six personas were dispatched and all six returned:

| Persona | Verdict |
|---|---|
| Constitution Reviewer | ADVISORY (1 P2) |
| Go Reviewer | ADVISORY (2 P2) |
| Scope Boundary Auditor | ADVISORY (1 P2) |
| Learnings Researcher | ADVISORY (1 P2) |
| Architecture Strategist | ADVISORY (2 P2) |
| Agent-Native Parity Reviewer | ADVISORY (1 P2) |

Security Lens was not triggered: no auth, secrets, or new external
integration.

**Gate rationale.** No P0 or P1 findings; P2 findings only, so the gate is
ADVISORY. The Constitution Check verdict is present and was confirmed correct.
Plan hardening was required and is present and adequate.

**Attempt-6 findings: all resolved.** Every persona that checked them
confirmed P1-1 (U0b → U1 edge; U2 retired into U0b) and P2-1 through P2-10
resolved, and the attempt-6 P3s addressed or dispositioned. P2-5 is resolved
through the decomposition (U5 depends on U0b, U1 and U3, with a
verification-status column for the rows the deferred U4 pins).

**Size assessment: confirmed.** The Scope Boundary Auditor agreed that the
attempt-6 plan (9 tasks, at least 7 waves) was too large for the one
shipment that runs without the active-residual halt, that retiring U2 and
deferring U4 are correct, that nothing left in the bootstrap is removable,
and that keeping U5 in the bootstrap is justified. The Constitution Reviewer
confirmed the table arithmetic (7 tasks, 5 waves, 11 scenarios: 6 RED,
3 characterization, 2 verification; 9 file touches over 8 files) and that no
task breaks the 2-hour rule or width isolation.

### P2 (merged) — remediation applied in this revision

* **P2-A (Scope, Architecture): follow-on vs the 074-DL gate.** A follow-on
  shipment with a `blocks` edge onto `154-S` is "behind `154-S`" and needs the
  074-DL attestation, while the plan required U4 to ship before that
  attestation: a loop. **Applied:** no follow-on shipment is created now
  (operator: no new shipment unless needed). U4 keeps task edges onto U1 and
  U3, is routed under the 074-DL rule like everything else behind `154-S`,
  and is not a precondition of the attestation. U5 publishes the affected
  rows as "by construction; regression pin pending", and U4's AC flips that
  row when it ships.
* **P2-B (Architecture, Learnings; Scope P3): harvest steps unpinned.** How
  `173.002-T` and `173.004-T` leave `154-S`, and which terminal status keeps
  the ship gate satisfiable, were unspecified
  (`docs/compound/2026-07-20-ship-gate-descoped-archived-member-exemption.md`).
  **Applied:** new `## Harvest Checklist`: reparent `173.004-T` before any
  claim; archive `173.002-T` directly from `queued`; exact manifest; exact
  edge rewrites; pre-ship check for non-terminal non-manifest children.
* **P2-C (Go; Constitution P3): U3 (2) fixture could not produce its
  assertions.** The cited P1C6 pattern fails on the only member's event-log
  restore, so nothing is marked and file and index agree. **Applied:** exact
  fixture (2 members; forward failure on `member2`; restore failure on
  `member1`'s artifact file path) and a deterministic "does disagree"
  assertion keyed on the known member ID.
* **P2-D (Go, Parity): CLI reads run journal recovery.** Every CLI read opens
  `core.NewWorkspace`, which runs `recoverPendingShipmentOperations`, so a
  consumer's CLI read performs rollback and the "never runs a lifecycle
  operation" rule was inaccurate. **Applied** in U5 and in the decision
  artifact's R3 (so the verbatim copy stays consistent): transport
  side-effect note, MCP recommended for strictly read-only consumers, read
  errors and non-atomic re-read mismatches are indeterminate,
  `<storage-root>/ops/`, and the mixed-binary caveat extended to any CLI.
* **P2-E (Constitution): rollout checkpoint actor.** Rebuilding and installing
  binaries writes outside the repository. **Applied:** steps 1–3 are
  operator-only, Ship only records the reported result, and the containment
  line records the operator-action exception.

### P3 (advisory) — applied unless noted

* RED harnesses use the literal key string, not the U1b constant (compile
  safety).
* Size-table counts corrected: U1 changes 3 functions; U0b needs 2 fixture
  helpers.
* U1: `UpdatedAt` bump on the marker-only write; the index-first no-op
  comparison; `clearStaleBlockedReason` and the other side effects kept (U0a
  (3) characterizes it).
* U0a (2): one pair suffices because the loop walks the ordered manifest
  slice.
* U0b (3): the zero-active `[]` case on both transports.
* U0c: fabrication mechanism named; close before reopen.
* U3: seam predicate plus counter; a gate failure reopens U1/U1b; the full
  constitution quality gates added to `## Verification`.
* Ancestry proof (`git merge-base --is-ancestor`) for binaries; the
  stable-contract learning cited.
* The risky-action wording now says the `154-S` claim itself precedes the
  merge and writes no marker.
* Other principles (I, V, VII/VIII, IX) are mapped in the Constitution Check.
* Not applied (optional): splitting U0a (1) into table-driven subtests is left
  to the implementer.

**Operator decision required to continue.** An ADVISORY gate is satisfied
only with `operator_authorization: approved` in this section. The P2
remediation above is text-only and introduces no new design decision (option A
stands; the decomposition follows the operator's instruction), so no further
review attempt is proposed. On authorization, Stage runs the `## Harvest
Checklist`. Until then the harvest is **not** updated and `154-S` stays held.

**Post-review corrections (PR #463, Copilot review cycle 1, text only).**

* MCP server startup also runs journal recovery (`openMCPServer` →
  `core.NewWorkspace`). The R3 transport note and the mixed-binary caveat now
  cover any binary that opens the workspace, CLI or MCP server.
* U4 no longer edits the U5 doc, which keeps U4 tests-only (width
  isolation). The U5 row is worded so it never needs a later edit. This
  supersedes the "U4's AC flips that row" wording under P2-A above.
* Harvest Checklist steps 2–3: the `173.002-T` edge is removed from
  `173.004-T` before `adopt` renames it, and step 3 touches only
  `173.005-T`.

<!-- plan-review-attempt: 7 -->
