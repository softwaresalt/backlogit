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

This shipment delivers only:

* the in-repo marker;
* its claim-crash-recovery compatibility;
* verification over the existing generic read surface;
* the published consumer contract.

The observable P-002.6 scheduler-misclassification defect is resolved only
once the autoharness scheduler consumes the marker, which is out-of-workspace
follow-up work. This shipment is an **enabling precondition, not full defect
resolution**.

**In-repo scope (revised 2026-09-28, finding F8):**

* `internal/core/shipment_lifecycle.go`: the claim loop and the marker write.
* `internal/core/shipment_recovery.go`: the claim recovery candidates (U1b).
* Tests in `internal/core`, `internal/cli` and `internal/mcp`.
* One operator doc.

No other shipment lifecycle operation (Block, Unblock, ReturnBlocked, Ship,
normalize) is changed. That follows from the option A decision below.

**Sources:**

* `docs/decisions/2026-09-13-shipment-claim-scheduler-reconciliation-deliberation.md`
  (original).
* `docs/decisions/2026-09-28-173f-marker-lifecycle-option-a-decision.md`
  (F2 marker lifecycle, option A).

**Review status:** attempt 5 recorded FAIL against post-`174-F` `main`. This
revision is the re-plan for plan-review attempt 6. The attempt-2..4 PASS
verdicts are superseded.

## Marker lifecycle decision (option A)

The operator decided this on 2026-09-28T22:22 -07:00: "Decision: option A".
The full artifact is
`docs/decisions/2026-09-28-173f-marker-lifecycle-option-a-decision.md`.

* **Contract (pinned, unchanged):** a single `custom_fields` key,
  `scheduler_baseline_claim`. Its value is the ID of the activating shipment,
  as a non-empty string. The prior art is
  `docs/compound/2026-07-23-machine-readable-governance-field-contract.md`.
* **Producer:** only `ClaimShipment` writes the key. It writes it on every
  manifest member whose **preimage status was `queued`**, independent of
  manifest order (F5). A re-claim overwrites a stale value. Members whose
  preimage status is not `queued` are left byte-identical.
* **Kept, never cleared:** Block, Unblock (to `active` or `queued`),
  `ReturnBlockedItem`, Ship and normalize leave the key as it is.
  * They already preserve `custom_fields` through `cloneArtifact` +
    `persistArtifact`.
  * Their recovery candidates are clones of preimages taken at operation time,
    so they already carry any marker. No new write paths or candidate shapes
    are needed.
* **Removed only by rollback:** in-process snapshot restore, or journal
  recovery persisting the preimage clones, restores the exact pre-claim
  bytes.
* **Consumer predicate (normative, published by U5):** an item is
  claim-activated if and only if all three hold:
  1. `status == active`;
  2. `custom_fields.scheduler_baseline_claim` == the ID of the **currently
     active shipment** (there is at most one);
  3. the item is in that shipment's current `items` manifest.

  Otherwise the item is organic-active. When no shipment is active, no item is
  claim-activated. "Non-empty ⇒ claim-activated" is **withdrawn**.
* **Accepted residuals, documented by U5:**
  * R1: stale markers persist on returned, blocked and terminal items. The
    predicate neutralizes them.
  * R2: a member that was organically activated while its shipment was
    blocked or unblocked to `queued` keeps that shipment's earlier marker
    across a re-claim.
  * R3: after a double-fault partial compensation, CLI and MCP can disagree
    until recovery runs and then `backlogit sync`.

## Requires plan hardening: yes

This work touches shipment lifecycle state, the claim crash-recovery CAS, and
a cross-workspace contract. See `## Plan Hardening`.

## Implementation Units

Execution order (the dependency edges enforce it):

1. U0a, U0b and U0c: the RED harnesses.
2. U1b: recovery compatibility.
3. U1: the marker write.
4. U2 and U3: verification.
5. U4: the regression net.
6. U5: the docs.

**U1b lands before U1.** At no commit may the marker be written while claim
crash recovery still rejects marked members, because that is the F1 wedge.
Every unit whose gate runs `./internal/core/...` also depends on the
UR3 crash-ready flake fix (`BDA56ED8`, see `## Verification`).

* **U0a: RED harness + characterization, claim marking (task 173.006-T).**
  * Domain: tests. File: `internal/core/shipment_claim_marker_test.go` (new).
  * Written before U1. No 173-F dependencies. 3 scenarios.
  * **(1) RED.** `ClaimShipment` sets `scheduler_baseline_claim = <shipmentID>`
    on every member whose preimage status is `queued`. Members that are not
    queued (for example, already `active` or `done`) and the shipment
    artifact are byte-identical to their preimage, apart from shipment
    status.
  * **(2) RED, ordering (F5).** The manifest lists a queued child before its
    queued member-parent (`[child, parent]`), so the bounded cascade activates
    the parent before the loop reaches it. Both child and parent end up
    marked.
  * **(3) Characterization, not RED.** A non-claim `setArtifactStatus`
    caller, such as a queue move, produces byte-identical frontmatter before
    and after.
  * AC:
    * (1) and (2) fail against pre-implementation code.
    * (3) passes both before and after the implementation and is not counted
      as RED.
    * The rollback-clears-marker scenario is not here; it moved to U3 as
      verification (F4).
* **U0b: RED harness + characterization, external read surface (task
  173.007-T).**
  * Domain: tests. Files: one test file in `internal/cli` and one in
    `internal/mcp`.
  * Written before U2. No 173-F dependencies. 3 scenarios, down from 4 (F7).
  * **(1) RED.** After a real claim, CLI `get --format json`
    (frontmatter-backed) shows `custom_fields.scheduler_baseline_claim ==
    <shipmentID>`.
  * **(2) RED.** MCP `backlogit_get_item` (index-backed) shows the identical
    value for the same item.
  * **(3) Characterization.** The key is absent on both transports for an
    organic-active item.
  * The old scenario (4), an "ignoring consumer", is **dropped** because there
    is no in-repo consumer (YAGNI). Option A's stale-marker behavior is
    covered by U4 (1).
  * AC:
    * (1) and (2) fail before the implementation.
    * (3) passes before and after.
    * There are fewer than 4 scenarios.
* **U0c: RED harness + characterization, claim crash recovery with marked
  members (new task, F1/F4).**
  * Domain: tests. File: `internal/core/shipment_claim_marker_recovery_test.go`
    (new).
  * No 173-F dependencies. 3 scenarios.
  * The tests fabricate the crashed state directly: a `shipment-operation/v1`
    claim intent journal with its preimage, plus current member files. They
    do not depend on U1.
  * **(1) RED.** Crash after marking. The shipment is `active` and every
    preimage-queued member is `active` + marked with the journal's shipment
    ID.
    * The next lifecycle operation, or `recoverPendingShipmentOperations`,
      rolls back successfully.
    * Every member and the shipment are byte-identical to the preimage: no
      marker key, and `custom_fields` is nil if it was nil before the claim.
    * A following `ClaimShipment` of the same shipment succeeds, so nothing
      is wedged.
    * A direct assertion checks that the last claim candidate returned by
      `memberRecoveryCandidates` carries the marker.
  * **(2) RED.** Double-fault partial compensation. The shipment is already
    restored to its `queued` preimage. One member is restored and another is
    still `active` + marked. Recovery succeeds and converges to the preimage.
  * **(3) Characterization, not RED.** A member marked with a **different**
    shipment ID, or otherwise diverged, still fails closed with
    `ErrShipmentConflict`. This guards against over-broad acceptance.
  * AC:
    * (1) and (2) fail today with `ErrShipmentConflict`, which is the
      genuine RED for the recovery surface.
    * (3) passes before and after.
    * The test does not run in parallel (`t.Parallel`) when it uses
      package-global seams. Prior art:
      `docs/compound/2026-07-29-durable-writes-test-seam-patterns.md`.
* **U1b: claim-recovery marker compatibility (new task, F1).**
  * Domain: code. Files: `internal/core/shipment_recovery.go`, plus at most
    one shared constant.
  * Depends on **U0c**.
  * In `memberRecoveryCandidates`, the `rollback`+`claim` case for a
    preimage-`queued` member returns three candidates, in this order:
    1. the preimage;
    2. preimage + `Status=Active`, unmarked. This is the intermediate
       member-parent cascade window, kept so rollback CAS accepts it.
    3. preimage + `Status=Active` + `custom_fields.scheduler_baseline_claim
       = journal.ShipmentID`, **last**. A nil map is lazily initialized.
  * The marked target must be the **last** candidate, because
    `validateShipmentLifecycleRecoveryOutcome` uses `candidates[len-1]` for
    committed evidence and `candidates[0]` for compensated evidence. That
    keeps the CAS path and the outcome path consistent.
  * Nothing changes for non-queued preimage members, for the shipment
    candidates, or for the block, unblock and normalize cases (option A).
  * Define the key once as an unexported constant,
    `schedulerBaselineClaimKey = "scheduler_baseline_claim"`, which U1 reuses.
  * AC:
    * U0c (1) and (2) turn GREEN and U0c (3) stays GREEN.
    * Existing recovery and UR3 tests stay green.
    * A marker with any value other than `journal.ShipmentID` is never
      accepted.
* **U1: persist the marker on claim activation (task 173.001-T).**
  * Domain: code. File: `internal/core/shipment_lifecycle.go`.
  * Depends on **U0a** and **U1b**.
  * **Seam form (F5/P3).** Use an **explicit per-call parameter, not a context
    value.**
    * Extract the body of `setArtifactStatus` into an unexported writer that
      takes a `claimMarker string` argument.
    * `setArtifactStatus` calls it with `""` and its behavior is unchanged.
    * The claim loop in `ClaimShipment` calls it with the shipment ID.
    * The seam is never visible to `MoveShipmentStatus` or other downstream
      writes.
  * **Ordering-independent marking (F5).**
    * For each member whose preimage status is `queued`, the write sets
      `Status=Active` and the key in **one** `persistArtifact` call.
    * If the member is already `active` (a member-parent the bounded cascade
      activated earlier in the loop), the writer still persists the key. That
      is a marker-only write: no `status_changed` event and no status
      transition.
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
    **`activatedIDs` no longer exists and is not referenced.**
  * AC:
    * U0a (1) and (2) turn GREEN and U0a (3) stays GREEN.
    * The key is set exactly on preimage-queued manifest members.
    * Non-claim `setArtifactStatus` callers are byte-identical.
    * Non-member parents are not marked.
    * Ignoring consumers see no behavior change.
* **U2: verification of the generic read surface (task 173.002-T; PASSed in
  attempt 5).**
  * Domain: code, verification-first.
  * Depends on **U1** and **U0b**.
  * The marker rides the existing generic `custom_fields` projection, and no
    bespoke field is added:
    * CLI `get --format json`: `internal/cli/get.go` `buildDetailMap`.
    * MCP `backlogit_get_item`: `internal/db/queries.go` `scanArtifactRow`,
      through `UpsertItem`.
  * **Index freshness:** file then index within **one** `persistArtifact`
    call (attempt-5 P3 wording).
  * The authoritative transports are CLI `get --format json` and MCP
    `backlogit_get_item`. SQL `query` (`json_extract`) is secondary.
  * An optional thin read-only helper is allowed, labeled as non-contract.
  * AC:
    * The pinned key is present on both transports for a claim-activated item
      and absent for an organic-active item.
    * The projection is the generic `custom_fields` copy, with no
      marker-specific filtering, so option A stale markers project as they
      are. The core-level behavior is covered by U4 (1); no extra transport
      scenario is added.
    * No bespoke projection field is added, and nothing mutates state.
* **U3: verification of both claim rollback paths (task 173.003-T, converted
  to tests-only, F3/F4).**
  * Domain: tests. File: `internal/core/shipment_claim_marker_rollback_test.go`
    (new).
  * Depends on **U1** and **U1b**.
  * 2 scenarios. Both are verification or characterization of existing
    exact-restore machinery, **not RED**, and no production code changes.
  * **(1)** In-process snapshot rollback. A real `ClaimShipment` fails after
    at least one member is marked; the failure is injected through the
    `persistArtifactWriteFn` seam on a later member. Every member and the
    shipment are byte-identical to the preimage, with no key and the nil map
    restored.
  * **(2)** Journal-recovery rollback after a real double-fault. Claim
    compensation also fails and leaves at least one member marked; the
    pattern is `TestP1C6_ClaimCompensationFailureIsClassifiedAndRecoverable`.
    The next lifecycle operation recovers through the journal, converges to
    the preimage and is not wedged. Afterwards, `backlogit sync` makes CLI
    and MCP agree (R3).
  * The stale references (`:100-133`, `~104-107`) and the "revert write"
    mechanism are removed.
  * AC: both scenarios pass. No production file is modified.
* **U4: option A lifecycle + end-to-end regression net (task 173.004-T).**
  * Domain: tests. File: `internal/core/shipment_claim_marker_lifecycle_test.go`
    (new).
  * Depends on **U1**, **U2** and **U3**, and on the UR3 flake fix.
  * 3 scenarios.
  * **(1) Option A lifecycle invariant.** This replaces the old "no
    queued-but-marked" invariant, which is false under Block.
    * The test runs claim → block → unblock(`active`) → `ReturnBlockedItem`
      of one member.
    * The marker equals the shipment ID on every claim-marked member
      throughout, including queued-but-marked while blocked.
    * Applying the published three-part predicate as an in-repo test helper:
      restored-active members are claim-activated. The returned item is not,
      because it is not in the manifest. After the item is organically
      activated, it is organic-active.
  * **(2) Crash-recovery end to end.**
    * A real `ClaimShipment` is interrupted after at least one member is
      marked. The interruption is a persisted intent journal plus a
      failpoint. The subprocess crash-child harness from
      `shipment_blocked_recovery_harness_test.go` may be used only after the
      flake fix has landed.
    * On reopen, both recovery paths converge:
      * rollback CAS, then preimage restore;
      * terminal evidence, where a compensated journal awaiting removal
        validates against the preimage.
    * This replaces the old concurrent claim + rollback `-race` scenario,
      which mostly exercised the global lifecycle lock (F6).
  * **(3) Regression gate.** `go test -race ./internal/core/...
    ./internal/cli/... ./internal/mcp/... ./internal/db/...` is green (F6:
    widened from `./internal/core/...`).
  * AC:
    * All three pass.
    * No U0a/U0b/U0c scenario is duplicated.
    * The flake prerequisite has shipped. There is **no** known-flake rerun
      policy.
* **U5: operator docs, the marker contract (task 173.005-T).**
  * Domain: docs. File: `docs/design-docs/scheduler-baseline-marker-contract.md`
    (new, hand-written). Generated `docs/cli-reference/*` is not edited.
  * Depends on **U2**.
  * Publish:
    * the pinned key and value shape;
    * a copy-pasteable read recipe:
      * find the active shipment: `backlogit shipment list --status active`
        or MCP `backlogit_list_shipments` with `status: active`;
      * read its manifest: `items`;
      * read each item: `backlogit get <id> --format json` or MCP
        `backlogit_get_item`, reading `.custom_fields.scheduler_baseline_claim`;
    * the **three-part option A predicate** (normative). "Non-empty ⇒
      claim-activated" is explicitly withdrawn;
    * that the marker is advisory, while the backlogit claim gate is
      authoritative;
    * the option A lifecycle table: kept through block, unblock, return and
      ship; removed by rollback; overwritten by re-claim;
    * residuals R1–R3. R3 is the CLI/MCP divergence after partial
      compensation (F8); the remedy is to trigger recovery with any lifecycle
      operation and then run `backlogit sync`;
    * that SQL `query` is secondary (`json_extract`);
    * that full defect resolution needs the autoharness follow-up.
  * AC: the doc contains every item above, and the predicate text matches the
    decision artifact verbatim.

## Constitution Check

* **Test-first ordering (non-negotiable).** Each implementation unit has a
  genuine RED predecessor:
  * U0a (1)/(2) → U1;
  * U0c (1)/(2) → U1b;
  * U0b (1)/(2) → U2.

  Characterization scenarios are labeled and not counted as RED. U3 and U4
  are post-implementation verification. The dependency edges enforce the
  order, and U1b precedes U1. Pass.
* **Single-domain tasks.** The domains are: tests (U0a, U0b, U0c, U3, U4),
  code (U1b, U1, U2), and docs (U5). Pass.
* **2-hour rule.** Every unit touches at most 2 files, fewer than 5 functions,
  and at most 3 scenarios. U0b's two files are one test per transport package.
  Pass.
* **Backward compatibility.** Default claim behavior is unchanged apart from
  the additive key on preimage-queued members. Non-claim writers are
  byte-identical. Recovery still fails closed on every divergence except this
  journal's own marker. Pass.
* **Workspace containment (P-017).** All work is in-repo. Autoharness
  consumption is a documented follow-up. Pass.

Constitution Check: pass

## Plan Hardening

**Required: yes.** The triggers are:

* the lifecycle-state schema addition;
* the shared status-write seam;
* the claim crash-recovery CAS (a fail-closed gate on every lifecycle
  operation);
* a cross-workspace consumer contract.

Re-hardened on 2026-09-28 for the post-`174-F` code. The attempt-5 rollback
bullet described a revert write that no longer exists.

**Protected invariants**

* Claim stays all-or-nothing through the journaled preimage and exact
  snapshot restore.
* Recovery never accepts a state it cannot prove came from this journal.
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
  `persistArtifactWriteFn` failure injection, and no `t.Parallel` with
  package-global seams.

**Risky actions**

* **ProposedAction:** widen the claim recovery candidates to accept this
  journal's marker (U1b).
  * **ActionRisk: high.** A mistake either wedges every lifecycle operation
    (too strict) or accepts a forged or divergent state (too loose).
  * Mitigation:
    * The only extra value accepted is exactly `journal.ShipmentID`.
    * U0c (3) keeps foreign or other divergence fail-closed.
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
      `status_changed` event.
* **ProposedAction:** publish the three-part consumer predicate (U5).
  * **ActionRisk: medium.** It is a cross-workspace contract.
  * Mitigation:
    * The predicate is copied verbatim from the decision artifact.
    * The residuals are published.
    * The marker is advisory-only.

**Verification added**

* The U0c recovery RED.
* U3 exact-restore on both rollback paths.
* U4 (1) option A lifecycle with the predicate as an in-repo consumer.
* U4 (2) end-to-end crash recovery.
* The widened `-race` gate.

**Rollback**

* **Mid-claim:** unchanged, exact restore. Journal recovery restores the
  preimage, including through U1b for marked members.
* **Feature-level revert:**
  * Revert U1 first, then U1b. A U1 revert alone is safe, because U1b only
    widens acceptance.
  * Markers already persisted on items stay as inert advisory data. The
    predicate and ignoring consumers are unaffected, and a later re-claim
    overwrites them.
  * Reverting U1b while marked claim **intent** journals exist on disk would
    re-wedge. **Operator checkpoint:** confirm that `.backlogit/ops/` has no
    pending claim lifecycle journal before reverting U1b.

**Monitoring and closure**

* After ship, `backlogit doctor` reports no shipment lifecycle evidence
  conflicts.
* The first real claim after merge shows the key on its members through both
  transports.
* The owner is Ship for `154-S`. The validation window is the `154-S`
  supervised bootstrap.

**Review-gate markers**

* Plan review must emit the literal `dispatch_mode:` and `decision:` fields.
* If sub-agent dispatch is unavailable, declare
  `single-agent-declared-degradation` (P-012 principle).

**Unresolved operator decisions:** none. F2 is decided (option A).

## Verification

* Per unit, as listed in the ACs.
* Shipment gate: `go test -race ./internal/core/... ./internal/cli/...
  ./internal/mcp/... ./internal/db/...`. `-race` runs on this Windows host:
  CGO is on and MinGW gcc is available.
* **Flake prerequisite (operator, 2026-09-28):**
  `TestUR3_ReopenRollsBackInterruptedBlockFromCompletePreimage`
  (stash `BDA56ED8`, duplicate `46A898B8`) is fixed in its own shipment
  **before** any `./internal/core/...` gate here runs. Enforcement:
  * a shipment `blocks` edge from `154-S`;
  * task-level `blocks` edges from U0a, U0c, U1b, U1, U3 and U4.

  No known-flake rerun policy is used.

  **Harvested (2026-09-28).** The fix is feature `181-F`, task
  `181.001-T`, in shipment `182-S`.
  * Recorded: `154-S` blocks-depends on `182-S`.
  * Recorded: task edges onto `181.001-T` from `173.006-T` (U0a),
    `173.001-T` (U1), `173.002-T` (U2), `173.003-T` (U3) and
    `173.004-T` (U4). The U2 edge is added because its gate runs
    `./internal/core/...`. It is redundant through U2→U1, but harmless.
  * Not created: the U0c and U1b tasks, because attempt 6 FAILed. Add their
    edges when they are harvested.
* Existing shipment lifecycle, recovery and UR3 tests stay green.

## Follow-ups (out of this shipment)

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
