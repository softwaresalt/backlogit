---
chunk_strategy: h1-h2-h3
description: "Implementation plan for CC0EBB59 — persist a universal scheduler-baseline marker on every claim-activated manifest task so claim-activated tasks are not misclassified as active residuals (record-only claim mode dropped in favor of the universal marker)"
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-13-shipment-claim-scheduler-reconciliation-plan.md
title: "Implementation Plan: Shipment-claim activation reconciliation (universal scheduler-baseline marker)"
docline:
    stash_id: CC0EBB59
    status: approved
    created_at: 2026-09-13T17:31:00Z
---

## Objective

Add an additive, backward-compatible **enabling precondition** to backlogit
shipment-claim activation that lets a claim-activated manifest task be
distinguished from an organic active residual. This shipment delivers the in-repo
marker + accessor **only**; the observable P-002.6 scheduler-misclassification
defect is fully resolved only once the autoharness scheduler consumes the marker
(out-of-workspace follow-up). This shipment is therefore an **enabling
precondition, not full defect resolution**. In-repo
(`internal/core/shipment_lifecycle.go`) only.

Source: `docs/decisions/2026-09-13-shipment-claim-scheduler-reconciliation-deliberation.md`.

## Requires plan hardening: yes

Touches shipment lifecycle state and claim semantics consumed by an external
scheduler across a trust/workspace boundary. See `## Plan Hardening`.

## Implementation Units

* **U1** Persist the **scheduler-baseline marker** on **every** claim-activated
  manifest item, stored as a single additive `custom_fields` key. The marker MUST
  be written **atomically within the same `setArtifactStatus` activation write**
  (not a second, separately-tracked write) so it is covered by the existing
  all-or-nothing claim invariant and the `activatedIDs` rollback tracking. The
  write seam is a **gated, off-by-default option threaded into
  `setArtifactStatus`** so that ONLY claim-driven activation writes the marker;
  all other `setArtifactStatus` callers (gate transitions, queue ops) stay
  byte-identical. **custom_fields semantics**: `omitempty` applies to the map as a
  whole (empty/nil map omitted), NOT per key; U1 therefore sets the key
  explicitly and does not rely on per-key omit behavior. **Marker contract
  (PINNED — machine-readable governance-field contract, prior art
  `docs/compound/2026-07-23-machine-readable-governance-field-contract.md`):** the
  marker is the single `custom_fields` key `scheduler_baseline_claim` whose value
  is the activating shipment ID (a non-empty string); presence of a non-empty
  value ⇒ claim-activated, absence ⇒ organic-active. This exact key name and value
  shape are authoritative and are the stable cross-workspace contract the
  out-of-repo P-002.6 scheduler matches literally; the same literal key MUST be
  used by U0a/U0b, U2, and published in U5 so producer and consumer bind to one
  auditable contract. **Parent-cascade safety (prior art
  `docs/compound/2026-08-01-n-independent-pair-test-design-for-go-map-iteration-nondeterminism.md`):**
  `setArtifactStatus` cascades to parent status via `cascadePersistedParentStatuses`;
  U1 MUST write the marker ONLY on the activated manifest items and MUST NOT let
  the parent-status cascade recompute or propagate the marker onto parent artifacts.
  Domain: code
  (`shipment_lifecycle.go` + the gated seam).
  * AC: the marker key is set exactly on items `ClaimShipment` transitions
    `StatusQueued`→`StatusActive`; NON-claim `setArtifactStatus` callers produce
    byte-identical frontmatter; a consumer that ignores the marker observes no
    behavior change (marker is advisory; unmarked items carry a nil/absent
    `custom_fields`); parent artifacts do NOT receive the marker via the status cascade.
* **U2** Expose the claim-activated-vs-organic-active distinction through a
  **supported read surface the out-of-workspace scheduler can actually consume**,
  not an `internal/core`-only Go accessor (which an external workspace cannot
  call). **The marker rides the ALREADY-SUPPORTED generic `custom_fields`
  projection** (verified in the codebase: `internal/cli/get.go` `buildDetailMap`
  copies all frontmatter keys into `get --format json`, and MCP
  `backlogit_get_item` via `internal/db/queries.go` `scanArtifactRow` rehydrates
  `custom_fields` from the JSON column persisted by `UpsertItem`), so U2 adds NO
  bespoke marker-specific projection field — a divergent side channel is forbidden.
  U2 is therefore primarily **verification** that the pinned `scheduler_baseline_claim`
  key materializes identically on both read paths, plus an optional thin read-only
  Go convenience helper explicitly labeled non-contract. **Index freshness:** the
  claim-activation write MUST upsert `custom_fields` into the SQLite index within
  the same operation so the DB-backed MCP surface is not stale relative to the
  frontmatter-backed CLI surface. **Authoritative external transport:** `backlogit
  get --format json` and MCP `backlogit_get_item` (both materialize
  `custom_fields.scheduler_baseline_claim`); the SQL `query` surface exposes the
  marker only inside the opaque `custom_fields` JSON blob (requires `json_extract`)
  and is a SECONDARY path, not the primary contract. (Refinement vs the
  deliberation: because U1 marks every claim
  activation, a separate "record-only claim mode" is unnecessary and is dropped
  to avoid a divergent claim path.) Domain: code. Depends on U1.
  * AC: the pinned `scheduler_baseline_claim` marker is present in BOTH the CLI
    `get --format json` (frontmatter-backed) and MCP `get_item` (DB-backed)
    projections for a claim-activated item and absent for an organic-active item;
    the read surface mutates no state; no bespoke projection field and no
    `internal/core`-only accessor is exposed as the external contract.
* **U3** Add symmetric **rollback-clears-marker** handling:
  `rollbackShipmentClaim` (`shipment_lifecycle.go:100-133`) MUST clear the marker
  for every id it reverts by **deleting the `custom_fields` key** (restoring an
  emptied map to nil), **atomically within the same revert
  `setArtifactStatus(...StatusQueued...)` write** (lines ~104-107), so a
  partially-failed rollback never leaves a torn queued-but-marked item and a
  reverted item is byte-identical to a never-marked item. Domain: code. Depends
  on U1.
  * AC: after a claim that fails partway and rolls back, no reverted item retains
    the baseline marker key and no empty-map artifact remains (no torn state),
    even if rollback itself fails midway.
* **U0a (RED harness + characterization — lifecycle)** Write the lifecycle unit
  tests BEFORE U1/U3 implementation. TWO are genuine RED (fail against the
  pre-implementation code because the behavior does not exist yet): (1) claim
  activation sets the marker key on each activated item; (3) `rollbackShipmentClaim`
  clears the marker key on every reverted id and restores an emptied map to nil.
  ONE is a PASSING characterization/regression baseline, NOT a required RED: (2)
  non-claim `setArtifactStatus` callers produce byte-identical frontmatter (seam
  isolation) — non-claim callers already emit their current frontmatter, so this
  test passes before AND after and only locks in that the gated off-by-default
  seam leaves them unaffected. Domain: tests. No deps (predecessor). ≤3 scenarios.
  * AC: scenarios (1) and (3) exist and fail against the pre-implementation code
    (marker/rollback absent), demonstrating a genuine RED baseline; scenario (2)
    passes as a characterization/regression baseline both before and after and is
    NOT treated as a required RED.
* **U0b (RED harness + characterization — read surface)** Write the read-surface
  tests BEFORE U2, asserting the pinned `scheduler_baseline_claim` key on BOTH
  read transports independently (they use different code paths with different
  freshness guarantees). TWO are genuine RED (fail pre-impl because the marker is
  not yet produced): (1) the persisted marker appears in the frontmatter-backed
  CLI `get --format json` projection for a claim-activated item; (2) the persisted
  marker appears identically in the DB-backed MCP `get_item` projection for the
  same item. TWO are PASSING characterization/regression baselines, NOT required
  RED: (3) it is absent for an organic-active item (organic items carry no marker
  before AND after); (4) a consumer ignoring the marker (absent-marker consume
  path) is unchanged. Domain: tests. No deps (predecessor). ≤4 scenarios.
  * AC: the marker-projection scenarios (1)/(2) fail against the pre-implementation
    read path (genuine RED) and prove the marker materializes on both the CLI
    (frontmatter) and MCP (DB) surfaces so the two paths cannot silently drift; the
    absence/ignore scenarios (3)/(4) pass as characterization/regression baselines
    both before and after and are NOT treated as required RED.
* **U1** (impl) — see above. Depends on **U0a**.
* **U2** (impl) — see above. Depends on **U1**, **U0b**.
* **U3** (impl) — see above. Depends on **U1**, **U0a**.
* **U4 (concurrency/integration verification)** After impl, add the `-race`
  regression suite that cannot be expressed as a deterministic pre-impl RED unit:
  claim-activation marker atomicity under `-race`, and rollback-clears-marker
  under a partially-failed rollback; assert existing shipment lifecycle tests stay
  green. Domain: tests. Depends on **U1**, **U2**, **U3**. ≤3 scenarios. (The unit
  behaviors themselves are covered test-first by U0a/U0b; this unit is the
  post-impl concurrency regression net only.)
* **U5** Operator docs: scheduler-baseline marker + the supported CLI/MCP read
  surface semantics and the cross-workspace consumption contract; explicitly
  labels this as an enabling precondition. Must publish a concrete, copy-pasteable
  external read recipe naming the pinned key: `backlogit get <id> --format json`
  and MCP `backlogit_get_item`, both yielding `.custom_fields.scheduler_baseline_claim`
  (value = activating shipment ID); the claim-activated (non-empty value) vs
  organic-active (absent) decision rule; a note that the SQL `query` surface exposes
  the marker only inside the `custom_fields` JSON payload (needs `json_extract`) and
  is secondary; and a note on MCP/DB index freshness vs the frontmatter-backed CLI
  path. Domain: docs.
  * AC: docs publish the exact pinned key `scheduler_baseline_claim`, the concrete
    CLI + MCP read recipe, the advisory-vs-authoritative decision rule (backlogit
    claim gate remains authoritative), and state that full defect resolution
    requires the autoharness follow-up.

## Constitution Check

* **Test-first ordering (NON-NEGOTIABLE)**: RED harnesses (U0a lifecycle, U0b
  read-surface) are written and observed failing BEFORE the impl units (U1/U2/U3)
  that make them pass; dependency edges enforce the ordering. U4 is a post-impl
  concurrency regression net only. Pass.
* **Single-domain tasks**: tests / tests / code / code / code / tests / docs.
  Pass.
* **2-hour rule**: each unit < 3 files / < 5 functions / ≤ 3 scenarios. Pass.
* **Backward compatibility**: default claim behavior unchanged; unmarked items
  byte-identical. Pass.
* **Workspace containment (P-017)**: in-repo only; autoharness consumption is a
  documented follow-up. Pass.

Constitution Check: pass

## Plan Hardening

* **ProposedAction**: persist a new lifecycle marker at claim activation.
  **ActionRisk**: medium (lifecycle-state schema addition). Mitigation: additive
  field with absent-default; U3 asserts existing paths unchanged.
* **ProposedAction**: set the marker on **every** claim activation via a gated,
  off-by-default `setArtifactStatus` seam (no second claim path). **ActionRisk**:
  medium (touches the shared status-write seam). Mitigation: the seam is threaded
  as an off-by-default option so ONLY claim-driven activation writes the marker;
  all other `setArtifactStatus` callers stay byte-identical; the read accessor
  (U2) is pure and mutates nothing. The record-only claim mode considered in the
  deliberation is **dropped** — the universal marker makes a divergent claim path
  unnecessary.
* **Cross-boundary safety**: the marker is advisory data for the scheduler; the
  backlogit claim gate remains authoritative. No autoharness behavior is assumed
  to change for this shipment to be correct in-repo.
* **custom_fields marker representation**: the marker is a single `custom_fields`
  key. `omitempty` on the `custom_fields` map itself only omits the map when it
  is empty/nil — it does NOT give per-key omit semantics. Therefore U1 sets the
  key explicitly on claim activation, and U3 clears it by **deleting the map key**
  on revert; when the deletion empties the map, the writer restores it to a nil
  map so the reverted frontmatter is byte-identical to a never-marked item (no
  empty-map artifact left behind).
* **Rollback**: (a) mid-claim rollback — `rollbackShipmentClaim` clears the
  marker for **every** reverted id atomically within the same revert
  `setArtifactStatus(...StatusQueued...)` write (U3), leaving no torn
  queued-but-marked item. (b) Feature-level revert — reverting the shipment
  removes the marker/seam entirely; because the marker is advisory-only and never
  gated any authoritative decision, removing it restores no forgeable or unsafe
  behavior (the authoritative claim gate is unchanged). Marker state is cleared
  exactly (key deleted, empty map restored to nil) in both paths.

## Verification

* `go test ./internal/core/... -race` including U4.
* Existing shipment lifecycle tests remain green.

## Follow-ups (out of this shipment)

* Autoharness P-002.6 wave scheduler to consult the baseline marker and exclude
  claim-activated tasks from residual classification (cross-workspace).

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
