---
title: "Deliberation: Shipment-claim / wave-scheduler convergence (claim semantic model)"
description: "Authoritative decision on the shipment-claim semantic model that reconciles core.ClaimShipment activation with the P-002.6 wave scheduler; deliberation over deferred stash 6434A4D7"
topic: "Shipment-claim / wave-scheduler convergence — claim semantic model for backlogit core"
depth: "deep"
decision_status: "decided"
promoted_to: "queue"
linked_artifacts:
  - "docs/decisions/2026-09-13-shipment-claim-scheduler-reconciliation-deliberation.md"
  - "docs/exec-plans/2026-09-13-shipment-claim-scheduler-reconciliation-plan.md"
  - "docs/decisions/2026-09-19-baseline-convergence-decomposition.md"
  - "docs/exec-plans/2026-09-17-baseline-convergence-plan.md"
source_stash_id: "6434A4D7"
source_refs:
  pr: "#449"
  feature: "175-F"
  shipments: ["156-S", "149-S", "157-S..169-S"]
  capture_review_threads: ["PRRT_kwDORzozKM6kE5FR", "PRRT_kwDORzozKM6kE5F0"]
  cycle_review_threads: ["PRRT_kwDORzozKM6kLn0r", "PRRT_kwDORzozKM6kLn1C", "PRRT_kwDORzozKM6kLn1M"]
tags:
  - "shipment-lifecycle"
  - "wave-scheduler"
  - "claim-semantics"
  - "deferred-scope-expansion"
  - "P-021"
---

## Deliberation: Shipment-claim / wave-scheduler convergence

**Depth**: deep. **Route**: `deliberate` (forced by the `DEFERRED SCOPE
EXPANSION` marker on stash `6434A4D7`; P-021 C6). **Scope posture**:
investigate-first + freeze-scope. This is a **planning-only** cycle: no Go core,
script, workflow, or config source is modified, no shipment is claimed, and no
implementation PR is created (Stage role boundary; P-010).

### Deferred-expansion triage record (P-021 C5/C6)

* **Entry**: `6434A4D7` (high, feature) — `DEFERRED SCOPE EXPANSION`: backlogit
  core enforces neither shipped-only dependency readiness nor a claim-time
  dependency guard, and exposes no Stage-scope governed non-claimable disposition
  for a superseded shipment.
* **Duplicate detection (A) — UNCONDITIONAL**: ran over all 60 active stash
  entries. Result: **CLEAN — no duplicate.** `6434A4D7` is the sole entry
  covering claim-time dependency / shipped-only readiness / non-claimable
  disposition. (`48FC866D` is wave-scheduler *test-fixture* drift; `4DB1DFF1` is
  lint remediation — neither describes the same expansion.)
* **Late-identifier reconciliation (B)**: NOT TRIGGERED — every source-ref field
  is populated (PR `#449`; feature `175-F`; shipments `156-S,149-S,157-S..169-S`;
  capture review-threads `PRRT_kwDORzozKM6kE5FR`, `PRRT_kwDORzozKM6kE5F0`). No
  `N/A` field exists, so reconciliation is a no-op. Recorded explicitly per the
  "record all four cases" rule.
* **Disposition**: reconciled in place under Stage stash authority; entry
  retained (its implementation is out of scope for this cycle) and re-linked to
  this decision. No second entry created.

### Problem Frame

PR #449 packages the baseline-convergence topology as a prerequisite shipment
`176-S` (RS-W(-1)) followed by the wave-aligned replacement sequence
`157-S`..`169-S` (RS-W00..RS-W12). Three unresolved Copilot review threads on
#449 expose that this topology, while structurally well-formed, is **not
executable** under the current shipment-claim and wave-scheduler contracts:

* **`PRRT_kwDORzozKM6kLn0r`** (`176-S.md:8`) — `core.ClaimShipment`
  (`internal/core/shipment_lifecycle.go:76-85`) transitions **every** queued
  manifest member to `active` on claim, while Ship wave admission
  (`.github/agents/_ship.agent.md:526-535`) **halts on any active member** and
  computes `ready_k` only from `queued` tasks. Claiming `176-S` therefore makes
  all three tasks `active` and immediately yields `WAVE_NO_PROGRESS`; the same
  applies to `157-S`..`169-S`.
* **`PRRT_kwDORzozKM6kLn1C`** (`175.099-T.md:53`) — the bootstrap tasks'
  declared governed-claim dirty-path footprint omits the mutations
  `ClaimShipment` actually makes: the shipment manifest, all sibling members, the
  feature (status cascade), and the append-only event journals. Because the
  contract fails closed on any non-allowlisted dirty path, ordinary claim intake
  invalidates the bootstrap task before implementation begins.
* **`PRRT_kwDORzozKM6kLn1M`** (`176-S.md:8`) — the PR body / Local Review
  Readiness still asserts a superseded 99-task, single-bootstrap topology; the
  feature, plan, and decomposition already report 101 (98 remediation + 3
  bootstrap). Evidence-only; no repository change.

This is the same contract mismatch that already blocked shipment `140-S`
(checkpoint `ship-140-s-2026-09-10`, phase `wave-admission-blocked`,
resume_hint: "resolve the ClaimShipment-all-members-active versus P-002.6
active-residual contract mismatch"). It is exactly the reliability defect
deferred as `6434A4D7`.

The question this deliberation answers: **what is the authoritative claim
semantic model** that (a) reconciles claim activation with dependency-gated wave
admission, (b) yields an explicit lifecycle-mutation cleanliness boundary, and
(c) does not break the existing `ClaimShipment` contract?

### Research Findings (code / history / graph evidence)

Engram code-graph + history evidence (`engram map-code`/`search`, git history):

1. **`ClaimShipment` activates all members with no dependency check.**
   `internal/core/shipment_lifecycle.go:46-98` sets the shipment `active`, then
   iterates `NormalizeShipmentItems` and calls `setArtifactStatus(... Active,
   "shipment claimed")` on every `queued` member. There is **no** claim-time
   dependency validation.
2. **The all-members-active behavior is a locked, documented, tested contract.**
   `tests/integration/shipment_workflow_test.go` →
   `TestShipmentWorkflow_ClaimActivatesIncludedItems` asserts member activation
   and documents it as "the side effect that distinguishes `ClaimShipment` from a
   bare `MoveShipmentStatus`." `internal/core/shipment_state_integrity_test.go` →
   `TestClaimShipment_SuccessActivatesAllItems`; `internal/core/shipment_test.go`
   → `TestClaimShipment_ActivatesIncludedScope`. Across the tree there are ~60
   `ClaimShipment(...)` call sites (many downstream tests claim a shipment and
   then drive members to `done`, relying on their being `active`), plus **two
   production callers**: `internal/cli/shipment.go:277` (CLI `shipment claim`)
   and `internal/mcp/tools.go handleClaimShipment` (MCP `backlogit_claim_shipment`).
3. **The dependency cascade already treats non-shipped terminals as satisfying
   an edge.** `internal/core/queue.go filterByResolvedDependencies` (L441-490)
   clears a `blocks`/`parent_of`/`relates_to` edge whenever the upstream reaches
   any of the 6-status cascade (`done, accepted, archived, shipped, abandoned,
   rejected`) via `IsNoLongerBlockingStatus`. So `archived`/`abandoned`/`rejected`
   satisfy a dependency identically to `shipped` — the root of `6434A4D7`'s (a)
   and (b): `156-S` cannot be held hard non-claimable behind `169-S`, and `149-S`
   can become eligible without `169-S` having *shipped*.
4. **The wave scheduler assumes members enter `queued` and are admitted
   per-wave** (`.github/agents/_ship.agent.md:505-536`): partition M; halt on any
   `active` residual (Step 4); `ready_k = { t in queued : deps terminal_success }`
   (Step 6). This directly contradicts `ClaimShipment` eagerly activating all
   members.
5. **Prior art (2026-09-13, feature `173-F` / stash `CC0EBB59`).** The
   `docs/decisions/2026-09-13-shipment-claim-scheduler-reconciliation-deliberation.md`
   deliberation already evaluated this exact reconciliation and chose an
   **additive persisted scheduler-baseline marker** on every claim activation
   (Option A), explicitly **dropping a record-only / second claim path** (Option
   B). Its plan (`...-reconciliation-plan.md`, tasks 173.001-T..007-T) delivers
   the **in-repo marker + accessor only**; scheduler *consumption* of the marker
   is an out-of-workspace autoharness follow-up (P-017). `173-F`/`154-S` is queued
   and reviewed PASS.

### Options Evaluated

**Model M1 — In-place semantics change: `ClaimShipment` activates only the
shipment, leaves members `queued`.**
Members then enter wave admission as `queued`, and the scheduler admits them
per-wave. Directly matches the scheduler's assumption.
* Pros: conceptually clean; no marker or scheduler special-casing.
* Cons: **backward-incompatible.** Breaks `TestShipmentWorkflow_ClaimActivatesIncludedItems`,
  `TestClaimShipment_SuccessActivatesAllItems`, `TestClaimShipment_ActivatesIncludedScope`,
  and the ~60 downstream test call sites that claim then complete members; changes
  the observable contract of both production callers (CLI + MCP). High blast
  radius on a shared core surface.

**Model M2 — Preserve `ClaimShipment` all-members-active semantics; realize
dependency-gated wave admission additively (CHOSEN).**
Keep claim activating shipment + all members. Add, additively:
(i) the scheduler-baseline **marker** (`173-F`/`154-S`, in-repo, reviewed PASS)
so claim-activated members are distinguishable from organic active residuals;
(ii) wave-admission **consumption** of the marker (Ship-agent contract + external
P-002.6 scheduler) — treat marked-active members as the wave-0 admissible
baseline, compute `ready_k` over `queued ∪ marked-active` in dependency order,
and reserve the `WAVE_NO_PROGRESS` active-residual halt for **unmarked** active
members (genuine orphaned prior-wave claims);
(iii) an additive **claim-time dependency guard** + **shipped-only readiness
gate** + **governed non-claimable disposition** (`6434A4D7`) for the dependency
axis.
* Pros: backward-compatible (unmarked behavior byte-identical); preserves
  dependency-gated wave admission; the claim-time guard is a precondition check
  that does not touch member-activation semantics; extends, rather than forks,
  the existing lifecycle; consistent with the already-decided `173-F` marker.
* Cons: full observable fix requires the external autoharness scheduler to
  consume the marker (out-of-workspace follow-up); the dependency-axis hardening
  (`6434A4D7`) is a separate future core release unit.

**Model M3 — New "record-only" claim operation (distinct verb that activates the
shipment only).**
* Pros: strongest separation between wave-scheduled and eager claims.
* Cons: **already deliberated and dropped 2026-09-13 (Option B).** Introduces a
  divergent second claim lifecycle and diverging consumers for no benefit the
  marker does not already provide.

**Model M4 — "Reinterpret all active members as pending."**
* Pros: would let the scheduler ingest a just-claimed shipment.
* Cons: **unsafe; rejected on the operator's explicit guard.** No evidence it is
  safe. It contradicts the locked all-members-active contract and makes
  gate-evidence, `done`/`archived` cascade, and rollback semantics ambiguous for
  members the claim already transitioned `active`. A blanket active→pending
  reinterpretation cannot distinguish a claim-activated member from a genuine
  orphaned residual — which is precisely the distinction the marker exists to
  make.

### Trade-off Comparison

| Criterion | M1 in-place | M2 additive (CHOSEN) | M3 record-only verb | M4 active→pending |
|---|---|---|---|---|
| Backward compatibility | Broken (~60 tests, 2 callers) | Preserved (unmarked identical) | Preserved but forks lifecycle | Broken (contract + evidence semantics) |
| Preserves dependency-gated wave admission | Yes | Yes | Yes | Ambiguous |
| Lifecycle-mutation cleanliness boundary | Implicit | Explicit (marker + claim-time guard) | Explicit but duplicated | None |
| Blast radius | High (shared core) | Low (additive seam) | Medium (second path) | High |
| Evidence of safety | Contradicted | Supported | Prior-dropped | None |

### Decision

Adopt **Model M2**. `core.ClaimShipment` **retains** its all-members-active
semantics; dependency-gated wave admission is realized **additively**:

1. **Scheduler-baseline marker (in-repo, OWNED by `173-F`/`154-S`, reviewed
   PASS):** written on every claim-activated member so the wave scheduler can
   distinguish claim-activated members from organic active residuals. Additive,
   off-by-default seam; unmarked artifacts are byte-identical to today.
2. **Wave-admission marker consumption (Ship-agent contract + external
   autoharness P-002.6 scheduler; P-017 out-of-workspace):** marked-active
   members are treated as the wave-0 admissible baseline and the active-residual
   `WAVE_NO_PROGRESS` halt is reserved for **unmarked** active members (genuine
   orphaned prior-wave claims). The exact admission algorithm (e.g. computing
   `ready_k` over `queued ∪ marked-active` in dependency order) is a
   **recommendation to the owning workspace**, not a Stage-authored spec; it
   SHOULD be single-sourced as one versioned marker-consumption contract that
   both the Ship-agent contract and the external scheduler cite, so the two
   consumers cannot diverge.
3. **Dependency-axis hardening (`6434A4D7`; deferred Go-core release unit):**
   an additive **claim-time dependency precondition guard** inside
   `ClaimShipment` (reject a claim whose own dependencies are not `shipped`), a
   **shipped-only readiness gate** at the claim/queue boundary implemented as a
   **NEW release-gating predicate** that MUST NOT mutate or unify the shared
   `IsNoLongerBlockingStatus` / `IsCascadeTerminalStatus` taxonomy —
   `internal/core/status_taxonomy.go` documents the cascade and releasable sets
   as **deliberately divergent** (`archived`/`abandoned`/`done`/`accepted`/
   `rejected` legitimately satisfy ordinary edges and MUST continue to) — and a
   **governed non-claimable/superseded disposition** reachable from `queued`
   without activation. The three concerns cross distinct module boundaries
   (`shipment_lifecycle`, `queue` resolution, a new disposition state) and SHOULD
   be decomposed into dependency-ordered sub-units in the future core cycle rather
   than shipped as one oversized unit. This item is **deferred design guidance**,
   not a Stage-authored implementation contract; the only binding constraint from
   this decision is Model M2: **additive, no in-place change to `ClaimShipment`
   member-activation semantics.**

Together these preserve dependency-gated wave admission and yield the required
**explicit lifecycle-mutation cleanliness boundary**: a claim mutates
(shipment status + all-member activation + feature cascade + event journal)
**only when its dependencies are satisfied** (guard), and **every mutation it
makes is self-describing** (marker). `M4` ("reinterpret all active members as
pending") is **rejected** — no evidence proves it safe, per the operator guard.

### Reuse Determination (Objective 3)

* **`154-S` / `173-F` already OWNS the in-repo enabling precondition** (the
  scheduler-baseline marker). It is **REUSED**, not duplicated. No new
  prerequisite release unit is created for the marker.
* The **marker consumption** is external autoharness (P-017) — recorded as a
  cross-workspace follow-up; not harvestable in this workspace.
* **`6434A4D7`'s dependency-axis hardening is a distinct, deferred Go-core
  release unit** (P-021 C1: internal/core lifecycle + queue semantics on a shared
  surface, beyond this planning-only cycle). It is **not harvested here**; it is
  reconciled, re-linked to this decision, and left queued for a future core
  cycle. Its eventual implementation MUST follow Model M2 (additive guard, no
  in-place activation-semantics change).

### Bootstrap Resolution (Objective 3)

**The bootstrap problem:** the enabling-precondition shipment `154-S`/`173-F`
cannot itself be executed through the marked-aware wave scheduler, because the
marker its own tasks produce does not exist at its own claim time. `173-F` is
multi-wave internally (`173.001-T`..`173.007-T` form a dependency chain, e.g.
`173.002-T depends_on 173.001-T, 173.007-T`). Claiming `154-S` therefore
activates all seven members **unmarked** at once → strict wave admission halts
`WAVE_NO_PROGRESS`. So the fix cannot be shipped by the very contract it repairs.

**Resolution — operator-authorized single-shipment bootstrap execution, without
relying on the broken contract:** `154-S`/`173-F` is executed as a direct,
operator-supervised bootstrap in which Ship claims the shipment and then drives
the claim-activated members green **in their declared dependency order**
(`173.006-T`/`173.007-T` → `173.001-T` → `173.002-T`/`173.003-T` → `173.004-T` →
…), **without** running the strict active-residual admission halt — because for
the just-claimed prerequisite the active members ARE the intended wave-0 working
set, not orphaned residuals. This bootstrap exception is:

* **Scoped to exactly one shipment** (`154-S`); every subsequent shipment uses
  the normal marked-aware scheduler with no exception.
* **Independent of the not-yet-built marker** — it needs no scheduler
  marker-consumption to complete.
* **Operator-authorized** — the strict P-002.6 active-residual halt is a safety
  check against *orphaned prior-wave* claims; waiving it for a single supervised
  prerequisite whose members are all backlogit-core marker work is bounded and
  auditable.

Sequencing that unblocks #449:
`154-S`/`173-F` (bootstrap execution) → external autoharness scheduler consumes
the marker → `176-S` (runner bootstrap) → `157-S`..`169-S` (baseline waves) via
the normal marked-aware scheduler → `6434A4D7` dependency-axis hardening lands
separately to convert the advisory `156-S`/`149-S` edges into hard guards.

### Rejected Alternatives

* **M1 (in-place activate-only-shipment):** backward-incompatible; breaks locked
  tests and both production callers.
* **M3 (record-only claim verb):** already dropped 2026-09-13; divergent second
  lifecycle.
* **M4 (reinterpret active as pending):** unsafe; no supporting evidence;
  contradicts the locked contract and the marker's purpose.

### Unresolved Questions / Follow-ups

* External autoharness P-002.6 scheduler marker-consumption (out-of-workspace,
  P-017) — tracked cross-workspace follow-up; gating for #449 execution.
* `6434A4D7` Go-core dependency-axis hardening — deferred core release unit;
  governs converting the `156-S`/`149-S` advisory edges into hard shipped-only
  guards and adding the governed non-claimable disposition.
* Whether the single-shipment bootstrap exception should be formalized as an
  explicit Ship-contract clause (vs remaining an operator-authorized manual
  bootstrap) — Ship-owned; out of Stage scope.

### Risks and Mitigations

| Risk | Sev | Mitigation |
|---|---|---|
| Baseline sequence treated as executable while claim/scheduler unresolved | High | #449 decomposition + plan rewritten to readiness `BLOCKED`; advisory edge `154-S blocks 176-S` recorded. |
| Bootstrap exception over-generalized into a standing scheduler bypass | Med | Exception scoped to `154-S` only, operator-authorized, single-shipment; all later shipments use the marked-aware scheduler. |
| Eventual `6434A4D7` implementer changes `ClaimShipment` in place | Med | This decision fixes Model M2 (additive guard only) as the authoritative model. |
| Marker never consumed (external work stalls) | Med | Readiness stays `BLOCKED`; no baseline wave is claimed until consumption lands (Orchestrator claim routing). |

### Scope Boundary

Planning-only. This decision and the coupled #449 rewrites modify **decision,
plan, and backlog contract artifacts only** — no Go core, script, workflow, or
config source. No shipment is claimed. No GitHub thread is replied to or resolved
by Stage. The marker implementation (`173-F`) and the dependency-axis hardening
(`6434A4D7`) are executed by Ship in later cycles.

## Constitution Check

- Role separation (P-001/P-010): Stage produces planning artifacts only; no
  source edits, no claims, no PRs.
- Investigate-first / freeze-scope: decision grounded in code, history, and
  code-graph evidence; no scope beyond the claim-model decision and the coupled
  #449 truth-rewrite.
- Deferred-expansion governance (P-021 C5/C6): unconditional duplicate scan
  (clean), reconciliation no-op recorded, deliberate route honored.
- Backward-compatibility first: the chosen model preserves the locked
  `ClaimShipment` contract.

Constitution Check: pass
