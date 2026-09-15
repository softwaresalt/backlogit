---
chunk_strategy: h1-h2-h3
description: "Product/requirements spec for a resumable, non-terminal shipment `blocked` lifecycle status that preserves resumption evidence and is excluded from the single active execution slot."
doc_type: spec
schema_version: "1.0"
source: docs/product-specs/2026-09-14-resumable-shipment-blocked-lifecycle-status.md
title: "Product Spec — Resumable shipment `blocked` lifecycle status"
docline:
    stash_id: 808E4323
    informing_defect_stash_id: 7AA35A39
    status: proposed
    created_at: 2026-09-14T13:41:59Z
---

# Product Spec — Resumable shipment `blocked` lifecycle status

**Owner:** Stage (planning/decomposition only — no application code written here).
**Primary intake:** stash `808E4323` (feature): *"Add support for a `blocked` state
for shipments, including a `blocked_reason` attribute that explains why the shipment
is blocked, such as unmet dependencies on other shipments."*
**Informing workflow defect (NOT in scope, linked only):** stash `7AA35A39` — the
RED-deliverable zero-delta baseline vs. mandatory Ship task-claim mutation conflict
that stalled shipment `154-S`.
**Reconciled precedent:** feature `164-F` / shipment `146-S` (S12 "parked state"),
whose plan is currently `FAIL` at plan-review attempt 2 and pending re-plan.

---

## 0. CURRENT AUTHORITATIVE REVISION — rev2 (2026-09-15)

> This §0 is the **authoritative current revision** and supersedes the detailed
> requirement/decomposition text in the sections below, which are **retained as
> historical context and audit trail** (including all prior `## Plan Review`
> records in the companion plan). Requirement IDs `SBLK-R1…R27` remain valid as
> the shared contract vocabulary; §0 re-states the delivery shape concisely.

### 0.1 Scope split — shared contract vs. backlogit-local implementation

**(a) Shared, portable contract (STABLE — must not diverge from autoharness).**
The cross-repository contract is shipment lifecycle status **`blocked`**: non-terminal;
preserves shipment/member/reconciliation/resumption evidence; **excluded from the single
*active* execution slot** so exactly one corrective shipment may run; cannot be
claimed/executed while blocked; governed transitions `active → blocked` and
`blocked → {queued, active}` only after blockers clear and topology/readiness gates pass.
Multiple `blocked` shipments may coexist; at most one `active`. Governed audit metadata:
`blocked_reason`, `blocked_at`, `blocked_by`, `resume_checkpoint_ref`. On block, active/
in-flight member tasks are captured in a **machine-readable snapshot** and returned to
`queued`; on governed unblock they are **exactly restored** (CAS/drift refusal). Every
transition uses a **correlated intent→commit** record so an append-only event alone can
never assert a completed transition. This contract is the autoharness-stashed capability;
the local implementation is expected to be superseded by the upstream one and MUST remain
compatible (no backlogit-specific semantics that conflict with autoharness).

**(b) backlogit-local implementation (REPLACEABLE).** The central governed writer/envelope,
`.locks/` workspace-global active-slot lock, SQLite projection, doctor checks, and CLI/MCP
surface shapes are local and may be re-implemented upstream; they MUST NOT leak
backlogit-specific semantics into (a) nor introduce a conflicting synonym token.

### 0.2 Concise replacement decomposition (feature 174-F / shipment 155-S, 9 tasks)

The prior 29-task decomposition (`174.001-T…174.029-T`) is **superseded** and parked at
status `blocked` (preserved, not deleted). Replacement, all test-first and ≤2h:

| Task | Req | Scope | Domain |
|---|---|---|---|
| `174.030-T` | R1 | RED harness: transitions, metadata, intent/commit, member snapshot/disposition | tests |
| `174.031-T` | R2 | Core governed `BlockShipment` (lock-held snapshot → member→queued → persist → intent/commit → rollback) | code |
| `174.032-T` | R3 | Core governed `UnblockShipment` (`--confirm`, free-slot check, snapshot CAS/drift refusal, exact restore, metadata clear/event) | code |
| `174.033-T` | R4a | Central private governed writer + envelope (correlation/intent-commit) | code |
| `174.034-T` | R4b | Route all bypasses through the writer (generic move/update, `MoveShipmentStatus`, bulk/cascade, create-as-blocked/active, public `WriteArtifactFile`) | code |
| `174.035-T` | R5 | CLI+MCP parity for block/unblock + read/list status | code |
| `174.036-T` | R6 | 154-S bootstrap + normalizer (machine-readable snapshot, no free-form memory) + startup/governed crash recovery from durable intent | code |
| `174.037-T` | R7 | Crash/reopen subprocess integration tests (block/unblock/bootstrap) | tests |
| `174.038-T` | R8 | Minimal docs + doctor verification (active-count, malformed-blocked, torn-intent) | docs |

Dependency order (parent-first): `174-F → 174.030 → 174.031 → 174.032 → 174.033 →
174.034 → 174.035 → 174.036 → 174.037 → 174.038`.

### 0.3 External autoharness topology compatibility (VERIFIED 2026-09-15)

Direct inspection of the installed external gate
`autoharness gate pipeline-topology` (`autoharness/gates/topology.py`) establishes:

* **The current gate REJECTS a `blocked` shipment token.** Queue-folder shipment parsing
  (`topology.py:553`) raises `BacklogUnavailableError("missing or unsupported status")`
  for any status not in `_VALID_LIVE_SHIPMENT_STATUSES = {queued, active, shipped,
  abandoned}` — `blocked` is absent, so a `blocked` shipment fails the gate closed.
* **Downstream logic already treats `blocked` correctly once admitted:** `_active_shipments`
  counts only `live_status == "active"` (so `blocked` is non-active — matches contract (a)),
  and `_detect_before_consistency` (`_NOT_YET_CLAIMED_STATUSES = {queued, blocked}`) flags
  `SHIPMENT_STATE_INCONSISTENT` if a `blocked` shipment still has an `active`/`done` member —
  **external corroboration** of the member-disposition-to-`queued` requirement.
* **Smallest compatibility change** = add `"blocked"` to `_VALID_LIVE_SHIPMENT_STATUSES`
  (one line) in **external autoharness** `topology.py`. This lives upstream (Python,
  external dependency), NOT in backlogit Go source, so it is recorded here as an
  **external informing/ratification item** (see `informing_defect` + §0.4), not implemented
  by this release unit.
* **Until upstream ratifies:** the bootstrap uses an **operator-only, audited temporary
  gate override** (`autoharness gate pipeline-topology --force`) scoped to phases
  `pre_claim`/`post_claim`/`lifecycle` for `155-S`'s bootstrap window only, recorded in the
  bootstrap runbook (companion decision §6). No standing override; removed after migration.

### 0.4 Bootstrap source-of-truth handoff (154-S) — authoritative

The staging HEAD (`chore/stage-155`, based on `origin/main`) carries `154-S` and its member
`173.006-T` as `queued`, but the **intact Ship branch `feat/shipment-claim-scheduler-baseline-marker-enabling-precondition`
@ `dd9f01a1`** carries the *active* provenance/checkpoint. The bootstrap's machine-readable
snapshot MUST be captured from the **Ship-branch truth**, not the staging projection.
Full one-time, operator-approved, single-worktree handoff runbook (no parallel worktrees,
no loss of Ship commits) is authoritative in companion decision §6 and plan U-BOOT.

---

## 1. Problem statement

A shipment can reach a state where it cannot make forward progress but MUST NOT be
abandoned or archived because its branch, wave harnesses, commits, checkpoints, and
intent are still valuable and resumable. The concrete trigger observed in this
workspace is `154-S`: its covering feature `173-F` ("shipment-claim scheduler-baseline
marker") halted because the build-feature Step 0.5 RED-deliverable pre-claim zero-delta
baseline contract conflicts with mandatory Ship task-claim bookkeeping mutations
(deferred workflow defect `7AA35A39`; Ship checkpoint `checkpoint-20260914-070735.json`,
`blocker_token: RED_DELIVERABLE_DELTA_OUT_OF_SURFACE`).

Today the shipment lifecycle enum is `queued → active → shipped | abandoned`
(`internal/core/shipment.go:24-36`). There is **no non-terminal parking state**. A
stalled shipment must either sit in the single `active` slot (P-001 — at most one
active shipment) and block ALL other delivery, or be `abandoned` (terminal, evidence
lost). Neither is acceptable for a recoverable blocker.

`blocked` already exists as a **member/task** artifact status
(`internal/models/artifact.go:10-21`) and in the lifecycle hooks transition map
(precedent 124-F added `blocked→queued`/`active→queued` for member items). It is NOT a
shipment lifecycle status. This spec introduces a **shipment-lifecycle** `blocked`
status, deliberately distinct from the member/task status and from dependency gating.

---

## 2. Goals / non-goals

### Goals

* Introduce a non-terminal, resumable shipment lifecycle status `blocked`.
* Free the single `active` execution slot so ONE corrective shipment may run while a
  blocked shipment waits.
* Preserve all resumption evidence — branch, checkpoints, reconciliation records, and a
  governed **member-status snapshot** — while blocked, so a later governed unblock restores
  the exact pre-block member statuses. (Active/in-flight members are dispositioned to
  `queued` on block — NOT left unchanged — so the shipment holds no active execution; the
  snapshot is what preserves their prior state for exact restore. See SBLK-R9/R10.)
* Provide governed, auditable block/unblock transitions with operator-supplied reason
  and a resume-checkpoint reference.

### Non-goals

* **NOT** fixing the RED-baseline/claim-bookkeeping contract conflict (`7AA35A39`).
  That is a separate Ship/build-feature workflow change; this feature only provides the
  parking mechanism that lets `154-S` wait for that fix.
* **NOT** modifying shipment `154-S`'s manifest, its branch, or any application
  source/tests (Stage role boundary; P-010).
* **NOT** changing the external `autoharness` numeric-predecessor topology gate
  (out-of-repo; see `docs/decisions/2026-09-06-queued-shipment-ordered-scope-decision.md`).
* **NOT** a generic force-transition / state-override framework.
* **NOT** introducing `blocked → abandoned` as a direct edge (terminal abandonment is
  reached via `blocked → active → abandoned`, keeping the transition set minimal).
* **NOT** diverging from the shared cross-repository `blocked` contract, and **NOT**
  introducing backlogit-specific semantics that would conflict with the autoharness
  implementation that is expected to eventually supersede this one (see §2.5).

---

## 2.5. Cross-repository contract boundary (shared vs. backlogit-local)

This capability is **already stashed in the upstream `autoharness` backlog**. The backlogit
implementation planned here is a **local first-mover** that is **expected to be eventually
superseded by the autoharness implementation**. It must therefore remain consistent with the
shared end goal and approach. To keep the local work portable, this spec separates two
layers explicitly:

**(a) Shared contract — MUST remain stable and portable across repositories.** This is the
cross-repo agreement the autoharness implementation is also expected to honor. It MUST NOT be
renamed, re-scoped, or given backlogit-specific meaning:

* The lifecycle status **token** is the literal string `blocked` on the shipment artifact.
* The **transition set** is exactly `active → blocked`, `blocked → queued`, `blocked → active`
  (non-terminal; no direct `blocked → {shipped,archived,abandoned}`).
* The **behavioral semantics**: `blocked` is non-terminal and resumable; it is excluded from
  the single `active` execution slot; a `blocked` shipment cannot be claimed or executed; all
  resumption evidence is preserved (branch, checkpoints, reconciliation records, and a governed
  member-status snapshot enabling exact member restore — active members are dispositioned to
  `queued` on block, not left in place); a `blocked` shipment used as a dependency remains
  execution-blocking.
* The **audit field names** `blocked_reason`, `blocked_at`, `blocked_by`,
  `resume_checkpoint_ref`, and the **event** `shipment_status_changed` carrying actor + reason +
  resume reference as the authoritative record.
* The **distinction** from member/task `blocked` (`return-blocked`) and from dependency gating
  (§3) — these three axes stay independent in any conforming implementation.

**(b) Backlogit-local implementation — MAY be replaced when autoharness supersedes.** These are
mechanisms internal to backlogit that sit *behind* the shared contract and carry no cross-repo
commitment: the specific status-write choke-point seams and refusal wiring; the `.locks/`
workspace-global lock vs. an atomic CAS; the SQLite index projection and `list --status`
filter; the frontmatter/`custom_fields` storage shape; the `doctor` integrity checks; and the
exact CLI verb / MCP tool surface shapes. Any of these may be re-implemented differently
upstream, as long as layer (a) is preserved.

**Requirement classification.** Each requirement in §4 is tagged `[shared]` (part of layer (a))
or `[local]` (layer (b) implementation detail) so a future migration can tell what must stay
stable from what may be swapped.

---

## 3. Definitions (disambiguation is a first-class requirement)

| Concept | Scope | Meaning | Storage |
|---|---|---|---|
| **Shipment lifecycle `blocked`** (THIS spec) | shipment artifact | non-terminal, resumable, excluded from the active slot | shipment `status` field + `blocked_*` metadata |
| Member/task `blocked` (`return-blocked`) | member task | a single work item is blocked; set by `shipment return-blocked` | member `status` field |
| Dependency gating | edge computation | a `blocks` dependency is unresolved; computed on read | not a status |

A shipment being `blocked` does **not** imply its members are `blocked`, and a member
being `blocked` does **not** block the shipment lifecycle. These are independent axes.

---

## 4. Requirements (stable IDs)

Priority key: **M** = must, **S** = should, **C** = could.
Layer key (per §2.5): **[shared]** = portable cross-repo contract that must stay stable;
**[local]** = backlogit-local implementation detail that may be replaced by autoharness.

Layer classification of the requirements below:

| Layer | Requirements |
|---|---|
| **[shared]** — portable contract, must stay stable | SBLK-R1 (token value), R2 (transition set), R4 (single-active exclusion invariant), R5 (unblock-readiness *semantics*: blocker-resolution assertion + free slot), R6 (non-claimable/non-executable while blocked), R7 (audit **field names** + authoritative event), R9 (member-disposition semantics: no active release-unit execution while blocked), R10 (evidence/branch/checkpoint/member-snapshot preserved), R11 (ready-work exclusion + kept-in-queue *behavior*), R12 (dependency stays execution-blocking), R17 (no direct `blocked→terminal`), R20, R22, R27 (partial-failure durability semantics) |
| **[local]** — backlogit implementation detail, replaceable | SBLK-R3 (choke-point seams/wiring), R5 (`--confirm` flag + `.locks/` free-slot *mechanism*), R8 (clear-helper mechanism), R13 (CLI verb shape), R14 (MCP tool shape), R15 (SQLite projection), R16 (`doctor` checks), R18 (backlogit enum/index additive handling), R24/R25 (bootstrap seam + normalization *mechanism*) |
| **informing / assessment (non-contract)** | SBLK-R19 (RED-baseline defect provenance), R21 (no-conflicting-local-semantics governance), R23 (upstream informing item), R26 (external autoharness topology-gate assessment) |

R21 and R23 are cross-cutting contract-governance requirements (see *Portability & migration*).

### Lifecycle & transitions

* **SBLK-R1 (M)** — Add shipment lifecycle status value `blocked` to the shipment
  status enum and status validator, additively, classified **non-terminal**.
* **SBLK-R2 (M)** — Governed transition set is exactly: `active → blocked`,
  `blocked → queued`, `blocked → active`. Every other transition into or out of
  `blocked` (e.g. `queued → blocked`, `blocked → shipped`, `blocked → abandoned`,
  `blocked → archived`) is refused fail-closed.
* **SBLK-R3 (M)** — Governed block/unblock transitions are enforced **unconditionally at
  every status-write choke point** — the generic `move_item`/`update_item` path,
  `BulkUpdateStatus`, `setArtifactStatus`, `cascadePersistedParentStatuses`, the **exported
  core `MoveShipmentStatus`** entry point, and the shipment **`create` / `add` / `create_item`**
  entry points — each of which REFUSES a shipment transition with `newStatus=="blocked"`
  (block direction) or `oldStatus=="blocked"` (unblock direction), gated on
  `ArtifactType=="shipment"`, with no forgeable exemption flag. `create`/`create_item`
  additionally cannot originate a shipment directly in `blocked` (no `queued→blocked` or
  bare-create-into-`blocked`). The parent cascade is additionally gated so it can never move a
  shipment out of `blocked`. Dedicated functions (`BlockShipment` / `UnblockShipment`) are
  the ONLY legitimate write path and are exempt by construction (separate functions, not a
  flag), writing through a seam-private primitive that lives BENEATH the guarded choke points.
  A **consumer-inventory** test asserts every `ShipmentStatus` consumer (including
  `MoveShipmentStatus` and the create/add/create_item entry points) routes through
  the central terminal/non-terminal classifier or fails closed on the unrecognized value.
  Precedent: `docs/compound/2026-08-18-shipment-shipped-prevention-envelope.md`.
* **SBLK-R5 (M)** — `blocked → active` is permitted ONLY when (a) the operator passes an
  explicit `--confirm` flag asserting blocker resolution AND (b) the active slot is free
  (concrete backlogit-side check under the SBLK-R4 serialization). `blocked → queued`
  requires `--confirm` only. The `--confirm` flag is a **non-authoritative** confirmation
  (backlogit cannot verify an external blocker such as `RED_DELIVERABLE_DELTA_OUT_OF_SURFACE`;
  it is never treated as an authorization credential — a TTY/self-asserted token is
  spoofable). The external autoharness numeric-predecessor topology gate is acknowledged as
  an out-of-repo constraint and is NOT relied upon or implemented here.

### Single-active slot & concurrency

* **SBLK-R4 (M)** — Invariant: at most one shipment is `active` at any time. `blocked`
  shipments are **excluded** from the active slot. `shipment claim` (`queued → active`)
  is refused fail-closed when another shipment is already `active`; blocked shipments do
  NOT count against the slot. **Active-slot source of truth:** the authoritative active
  count is derived from an **authoritative scan of the contained shipment Markdown records**
  (canonical state), OR an equivalent real compare-and-swap on that canonical state — NOT
  from the SQLite index, which is a rebuildable non-authoritative projection. The
  check-then-set **fails closed** when the index is stale or missing (the scan is the source
  of truth) and when the scanned state is malformed or shows a duplicate/ambiguous active
  condition (refuse rather than guess). The active-slot check-then-set is serialized on a
  **workspace-global** lock/compare-and-swap (NOT the per-artifact mutation lock, which
  would not serialize two concurrent claims of different shipments). Claim remains
  atomic-by-construction with rollback of the activated member set on any mid-flight
  failure. (Operator-requested; P-001 was previously convention-only — this adds
  forward-only enforcement.) Precedent:
  `docs/compound/best-practices/atomic-multi-item-claim-rollback-and-stale-blocked-clearing-2026-06-27.md`.
* **SBLK-R6 (M)** — A `blocked` shipment cannot be claimed or executed concurrently.
  There are **two distinct lock scopes**, and the spec fixes both their roles and their
  acquisition order:
  1. **One workspace-global active-slot lock** (a single lock keyed on the workspace root, or
     an atomic CAS on the active-slot) that serializes the active-slot check-then-set shared by
     `claim` (`queued → active`) and `unblock --to active` (`blocked → active`). This is the
     ONLY lock that guarantees INV-1 (single-active): two concurrent claims/unblocks of
     DIFFERENT shipments must contend on this one key so both cannot read "no active" and both
     activate.
  2. **Per-artifact mutation locks** keyed on a **stable synthetic identifier** (workspace root
     + artifact ID, never the artifact's current path) that serialize writes to an individual
     shipment/member record.
  These are NOT interchangeable: the per-artifact lock alone does not serialize two different
  shipments and cannot enforce INV-1. **Fixed acquisition order to prevent deadlock:** when an
  operation needs both, it acquires the **workspace-global active-slot lock FIRST, then the
  per-artifact lock(s)**, and releases in reverse order; per-artifact locks are acquired in a
  deterministic artifact-ID order when more than one is held. The free-slot check and the
  activating write occur inside the SAME held global critical section (no release between check
  and set, closing the TOCTOU window).
  Precedent: `docs/compound/concurrency-issues/2026-08-09-stable-lock-keys-and-heartbeat-refresh-for-unbounded-holds.md`.

### Audit metadata & evidence

* **SBLK-R7 (M)** — On `active → blocked`, record audit metadata: `blocked_reason`
  (required, non-empty), `blocked_at` (RFC3339 timestamp), `blocked_by` (best-effort actor
  attribution — **advisory, NOT an authorization credential**; its trust level is
  documented), and `resume_checkpoint_ref` (optional pointer to the Ship-owned resume
  checkpoint, e.g. `154-S`'s `checkpoint-20260914-070735.json`). Metadata is persisted in
  shipment frontmatter/`custom_fields`. The append-only `shipment_status_changed` event is
  the **authoritative** non-repudiation record and carries actor + reason +
  `resume_checkpoint_ref`.
* **SBLK-R8 (M)** — `blocked_*` audit metadata is cleared on `blocked → queued` AND
  `blocked → active` through a **single governed-seam-owned helper** (not independent
  blocked-aware branches embedded in generic mutation code). Because generic status-write
  paths (`move_item`/`update_item`, `BulkUpdateStatus`, `setArtifactStatus`,
  `cascadePersistedParentStatuses`) REFUSE shipment `active↔blocked` transitions (SBLK-R3),
  the governed `UnblockShipment` seam is the sole legitimate unblock write path. A durable
  `shipment_status_changed` event (actor + reason + prior `resume_checkpoint_ref`) is
  emitted on the unblock edge BEFORE frontmatter is cleared, preserving the audit trail.
  Precedent: the atomic-claim/stale-blocked-clearing compound learning.
* **SBLK-R10 (M)** — Blocking MUST NOT sever or mutate branch association or
  resume-checkpoint linkage, and MUST capture a **governed member-status snapshot** (the
  pre-block status of every member) as resumption evidence. Branch, checkpoint, and the
  member snapshot are preserved verbatim so a later unblock can restore the exact pre-block
  member state and resume from the recorded point.

### Member semantics on block

* **SBLK-R9 (M)** — When a shipment transitions `active → blocked`, the governed block
  performs a **member disposition** that removes active release-unit execution while
  preserving full resumption evidence: it records the governed member-status snapshot
  (SBLK-R10), then returns any **`active`/in-flight member** to `queued` so the blocked
  shipment holds **no active execution** (this is what actually frees the single active slot
  — a blocked shipment with a still-`active` member would otherwise keep an execution in
  flight and P-001 would still be contended). Non-active member statuses are left unchanged.
  On `blocked → active` (governed unblock/resume), the snapshot **restores** each member to
  its recorded pre-block status. The block still does NOT cascade a member `blocked` status
  and does NOT invoke `return-blocked`; member `blocked` and shipment `blocked` remain
  independent concepts (Definitions §3). The member requeue is performed by the governed seam
  through its seam-private primitive, is captured in the audit event, and is reversible via
  the snapshot — it is a governed disposition, never an ungoverned member mutation.

### Queue, dependency, index surfaces

* **SBLK-R11 (M)** — `blocked` shipments are **excluded from ready-work selection**, are
  NOT archived, and remain in the queue directory. `shipment list --status blocked`
  lists them. The SQLite index carries the `blocked` status value and supports filtering
  on it; index writes for the new value are transactional/rebuildable
  (`docs/compound/best-practices/atomic-rehydration-sqlite-transaction-2026-04-08.md`).
* **SBLK-R12 (M)** — A `blocked` shipment used as a dependency remains
  **execution-blocking** (it is NOT a dependency-satisfying / no-longer-blocking status)
  until it reaches an eligible/terminal state. This is distinct from member/task status
  gating (Definitions §3).

### Operator surfaces

* **SBLK-R13 (M)** — CLI: `backlogit shipment block <id> --reason <text>
  [--resume-checkpoint <ref>]` and `backlogit shipment unblock <id> --to
  queued|active --confirm`. The `--confirm` flag is required on both unblock targets (a
  non-authoritative confirmation of blocker resolution per SBLK-R5). Command naming is
  deliberately distinct from the existing member-level `shipment return-blocked` to avoid
  semantic collision.
* **SBLK-R14 (M)** — MCP: block/unblock tools at parity with the CLI, plus `blocked`
  supported in shipment status filters on `list`/`get`. JSON-RPC error mapping is
  consistent with existing shipment tools.

### Schema / index projection

* **SBLK-R15 (M)** — Any gate/doctor/decision logic that must read `blocked_*` fields
  the SQLite index does not project MUST read the Markdown source directly
  (FindArtifactPath + ParseFrontmatter) and fail closed on missing/unrecognized status.
  Precedent: `docs/compound/2026-07-20-ship-gate-descoped-archived-member-exemption.md`.

### Integrity, ship, abandon, reconcile

* **SBLK-R16 (M)** — `backlogit doctor`, evaluated over `ArtifactType=="shipment"` only
  (so member `return-blocked` items are never swept in), gains checks: (a) at most one
  shipment is `active`; (b) every `blocked` shipment carries a non-empty `blocked_reason` +
  a valid `blocked_at` (a bootstrap-migrated shipment lacking `blocked_reason` is a **hard
  finding** naming the U18 normalizer, SBLK-R25); (d) unrecognized shipment status fails
  closed. The former "blocked AND terminal" scalar check is **removed**: `status` is a
  single scalar, so a record cannot simultaneously hold `blocked` and a terminal value —
  the check was unsatisfiable as written and is not a meaningful corruption detector.
  **Finding contract:** each check declares a **severity** (`error` for one-active-violation,
  malformed-blocked, and unrecognized-status; these are hard findings), a deterministic
  process **exit code** (non-zero when any `error`-severity finding is present), and an
  **MCP result contract** at parity with the CLI (findings returned as structured items with
  `severity`, `code`, `artifact_id`, `message`). Unrecognized shipment status fails closed.
* **SBLK-R17 (M)** — `shipment ship` refuses a `blocked` shipment (must unblock to
  `active` first). `reconcile-shipped` is unaffected (it operates on already-archived
  shipments). There is no direct `blocked → abandoned`; terminal abandonment is reached
  via `blocked → active → abandoned` (existing `active → abandoned` edge).

### Backward compatibility & migration

* **SBLK-R18 (M)** — The change is additive: existing shipments are unaffected and no
  data migration is required for the shipment status enum (shipment transitions are
  code-level, not the persisted hooks transition map). The member/task hooks transition
  map (124-F) is a SEPARATE surface and is not modified. A pre-existing multi-active
  condition is surfaced by doctor as a **hard finding with explicit remediation** (unblock
  one shipment) — NOT a resettable "first-run" warning and never a fail-open flag.
  Claim enforcement (SBLK-R4) is **forward-only**: it refuses creating a new second active
  without retroactively mutating existing state, so the new invariant does not silently
  break existing workspaces (the current corpus has exactly one active, `154-S`).

### Cross-repository contract & migration (shared-vs-local, per §2.5)

* **SBLK-R20 (M) [shared]** — The shared cross-repo `blocked` contract — the `blocked` status
  token, the exact three-edge transition set, the non-terminal/resumable/single-active-exclusion
  behavior, the audit **field names** (`blocked_reason`, `blocked_at`, `blocked_by`,
  `resume_checkpoint_ref`), and the authoritative `shipment_status_changed` event — MUST remain
  **stable and portable**. It is the contract the upstream `autoharness` implementation is also
  expected to honor. Changing any of these tokens/names in a backlogit-local way is a contract
  break and is prohibited without a corresponding cross-repo decision.
* **SBLK-R21 (M)** — The backlogit-local implementation MUST NOT introduce semantics that would
  **conflict** with the autoharness contract: no alternate status token (e.g. `paused`,
  `parked`, `on-hold`) for the same concept, no additional *required* transition edge, no
  local-only terminal reinterpretation of `blocked`, and no local-only required field that a
  conforming implementation could not satisfy. Backlogit-local mechanisms (SBLK-R3/R8/R13/R14/
  R15/R16/R18 and the `.locks/` global lock) sit **behind** the shared contract and are
  replaceable; they must not leak backlogit-specific meaning into layer (a).
* **SBLK-R22 (M) [shared]** — Compatibility & migration: the shared contract defined here is
  **backlogit-proposed / assumed, pending autoharness ratification** (no upstream contract is
  locally identifiable — SBLK-R23), so the enforceable guarantee is **one-sided**: backlogit
  keeps ITS OWN tokens/field-names/event stable, additive-only, and never renamed. When the
  autoharness implementation supersedes this one, migration MUST be **non-destructive and
  forward-compatible** — existing `blocked` shipments, their `blocked_*` metadata, and their
  `shipment_status_changed` events map cleanly (target: 1:1) onto whatever the ratified
  autoharness contract turns out to be, with **no data loss and no backlogit-side token rename**.
  A documented migration/compatibility note (delivered with the docs unit) describes the
  backlogit-side field/event shape and the supersession path, and explicitly flags any field
  whose upstream counterpart is not yet known. Changes remain **additive-only** with respect to
  the backlogit-side shared tokens.
* **SBLK-R23 (M, informing only)** — This capability is tracked upstream in the **`autoharness`
  backlog**. **No autoharness stash/backlog ID is identifiable from local evidence** (searched
  `.autoharness/` config, registry, manifests, and backups — no blocked-shipment backlog
  reference found), so it is recorded as an **external informing backlog item (upstream
  autoharness backlog) without inventing an ID**. The local intake was backlogit stash
  `808E4323` (consumed → feature `174-F`). If a concrete autoharness reference later becomes
  available, link it via `link add 174-F <ref> informs`.

### One-time, operator-approved, pre-governance bootstrap for `154-S`

* **SBLK-R24 (M) [local, exceptional, one-time] — Pre-governance bootstrap runbook that resolves
  the rollout circularity via manual member disposition + bounded pre-claim rollback; it is NOT
  the shared contract and is removed after migration.** Three facts are already true in local
  code (verified 2026-09-14): (i) `models.StatusBlocked` (`= "blocked"`) is a valid
  `ArtifactStatus` (`internal/models/artifact.go:17`); (ii) the generic
  `backlogit move <id> --status blocked` path routes through `core.UpdateArtifactWithGate`
  (`internal/cli/move.go:62`) and sets a shipment artifact's status token to `blocked`;
  (iii) `backlogit list --type shipment --status blocked` already surfaces such a shipment.
  **Rollout circularity:** the governed `BlockShipment` seam (U2c) with member disposition (U6)
  is the correct steady-state way to block `154-S`, but it does not exist until `155-S` ships —
  and `155-S` cannot claim the single active slot until `154-S` is blocked. A naive single
  generic move cannot resolve this because (1) it does **no member disposition**, so member
  `173.006-T` would remain `active` and P-001 stays contended, and (2) once the SBLK-R3 guards
  land it has **no valid governed rollback**. The resolution is an **explicitly exceptional,
  one-time, operator-approved pre-governance bootstrap runbook** that manually reproduces the two
  guarantees the governed seam would provide:
  1. **Manual member disposition FIRST.** Snapshot every member status durably, then move the
     active member to `queued` (`backlogit move 173.006-T --status queued`) BEFORE moving the
     shipment token (`backlogit move 154-S --status blocked`). This removes the in-flight
     release-unit execution so P-001 is genuinely uncontended and the slot is truly free — the
     manual equivalent of SBLK-R9/R10, without a policy override.
  2. **Bounded rollback, valid ONLY before any other claim.** Rollback (`move 154-S --status
     active` + restore each member to its exact snapshot status) is permitted **only if no other
     shipment has become active** in the interim (verify `list --status active` is empty first);
     otherwise it would create a second active and MUST **fail closed** (forward-fix only). This
     bounds the window the missing governed rollback would otherwise leave open.
  The bootstrap appends a **durable comment/memory audit** naming the migration debt (absent
  governed `blocked_reason`/`blocked_at`/`blocked_by`/`resume_checkpoint_ref` and event), runs the
  autoharness topology pre-claim check for `155-S` (SBLK-R26) before relying on the freed slot,
  and is **removed after migration**. It is **exceptional, operator-approved, and one-time**;
  **Stage does NOT execute it** (operator-owned; `154-S` not edited by Stage). The steady-state
  sanctioned path remains the governed `shipment block 154-S` once the capability ships; the
  generic-move primitive otherwise survives only as the U18a/U18b normalizer's internal
  remediation target for out-of-band `blocked` records, never as an operator-facing steady-state
  admission or rollback path.
* **SBLK-R25 (M)** — **Migration debt & normalization.** IF a `blocked` shipment ever exists
  without governed metadata (e.g. an externally-migrated or test-seeded record), it carries
  MISSING `blocked_reason` / `blocked_at` / `blocked_by` / `resume_checkpoint_ref` and NO
  governed `shipment_status_changed` event. This is explicit, tracked migration **DEBT**, not an
  acceptable terminal state. **Before such a shipment may be unblocked**, the governed normalizer
  (unit U18a) MUST backfill the governed metadata and emit the governed event through the real
  seam. The unblock readiness gate (SBLK-R5 / U12) MUST **refuse to unblock** a shipment whose
  governed `blocked_*` metadata is absent (outstanding debt), failing closed with a remediation
  message naming U18a/U18b. `doctor` (SBLK-R16) surfaces such a shipment (status `blocked`, no
  `blocked_reason`) as a **hard finding** naming U18a normalization as the fix. The normalizer's
  own governed transition, and the governed block/unblock it complements, are subject to the
  partial-failure durability semantics of **SBLK-R27**.
* **SBLK-R26 (M, external assessment)** — **Autoharness topology-gate compatibility.** The
  external `autoharness` pipeline-topology gate (out-of-repo, numeric-predecessor ordering) does
  NOT necessarily interpret the new `blocked` token. Setting `154-S` to `blocked` frees
  *backlogit's own* active-slot scan (status-keyed), but whether the external gate treats a
  `blocked` shipment as **non-active** (releasing the slot for `155-S`) or still counts its
  queue position as an unmet predecessor is **not established** and MUST be independently
  verified. If the external gate keys off queue position or does not recognize `blocked`, a
  **separate external compatibility gate / configuration** is required before relying on the
  bootstrap to admit `155-S`. Until verified, treat "external gate recognizes `blocked` as
  non-active" as an **unconfirmed assumption** and fail closed (do not assume admission).

### Partial-failure durability

* **SBLK-R27 (M) [shared]** — **Partial-failure durability & event intent/commit protocol** for
  the multi-write block / unblock / normalize operations. Each governed transition performs an
  ordered set of writes: (1) the `shipment_status_changed` **event**, (2) the shipment
  **frontmatter** (`status` + `blocked_*`), (3) the **member-snapshot + member-status (requeue)**
  writes per SBLK-R9/R10, (4) the SQLite **index** projection. Durability classification: the
  **event log is the authoritative durable record**; **frontmatter + member state are the
  canonical persisted state**; the **index is a rebuildable, non-authoritative projection**.
  **Intent/commit protocol (append-first MUST NOT falsely assert completion):** an appended event
  alone can NEVER be read as a completed transition. The governed seam records an **INTENT** entry
  carrying a **correlation id** for the transition, performs the frontmatter + member writes, and
  then records a **COMMITTED** entry (same correlation id) that marks the transition durable;
  reconciliation treats an INTENT without a matching COMMITTED (and without corresponding
  frontmatter/member state) as an **incomplete transition to be rolled back or forward, never as a
  completed one**. (An equivalent **persist-first with classified reconciliation** design — write
  canonical state first, then a committed event, with the same reconciliation classification — is
  an acceptable alternative; the invariant is that no single append can assert completion.) On a
  partial failure (crash between any two steps) the operation MUST **fail closed** and leave a
  **recoverable, non-torn** state: either the transition is fully committed (committed event +
  frontmatter + member disposition) or it is refused/rolled-back. **Doctor/recovery:** `doctor`
  flags any torn state — a committed frontmatter without a committed event, a committed event
  without frontmatter, a `blocked` shipment with a still-`active` member (member-requeue not
  applied), or an un-reconciled INTENT — and `sync`/rebuild self-heals a divergent index from
  canonical Markdown. **Failure-injection tests** (unit U19b, `174.029-T`) simulate a crash
  between each write step — including event-without-frontmatter and event-without-member-requeue —
  and assert no partial governed state is observable as valid.

### Provenance

* **SBLK-R19 (M, informing only)** — The RED-baseline/claim-bookkeeping defect
  (`7AA35A39`) is captured as the motivating scenario and linked as an informing defect.
  It is explicitly OUT OF SCOPE for this feature and is NOT harvested or absorbed here.

---

## 5. Lifecycle state model

```
                 claim (SBLK-R4: slot must be free)
   queued ─────────────────────────────► active
     ▲                                    │  │
     │ unblock --to queued (SBLK-R5)      │  │ ship (governed) ─► shipped (terminal)
     │  clears blocked_* (SBLK-R8)        │  │ abandon ─────────► abandoned (terminal)
     │                                    │  │
     │           block --reason (SBLK-R7) │  │
     └──────────── blocked ◄──────────────┘  │
                     │  unblock --to active   │
                     └───(SBLK-R5 + free slot)┘
```

**Invariants**

* **INV-1** at most one shipment in `active` (SBLK-R4).
* **INV-2** `blocked` is non-terminal and never occupies the active slot (SBLK-R1/R4).
* **INV-3** `blocked ⇒ blocked_reason ≠ "" ∧ blocked_at valid` (SBLK-R7/R16).
* **INV-4** leaving `blocked` (to `queued` or `active`) clears all `blocked_*` metadata
  at every choke point (SBLK-R8).
* **INV-5** blocking captures a governed member-status snapshot and returns active members to
  `queued` (no active release-unit execution while blocked), preserving branch/checkpoint/
  snapshot evidence verbatim; unblock restores members from the snapshot (SBLK-R9/R10).
* **INV-6** transitions into/out of `blocked` occur only through the governed seam
  (SBLK-R3); the generic path refuses them.
* **INV-7** no partial/torn governed state is ever valid: event and frontmatter commit
  together or not at all; the index is a rebuildable projection (SBLK-R27).

---

## 6. Success criteria (acceptance at the capability level)

1. `154-S` can be moved `active → blocked` **through the governed seam** with a reason and a
   `resume_checkpoint_ref`, freeing the active slot via **governed member disposition** (active
   members returned to `queued` under a preserved member-status snapshot), WITHOUT losing branch
   association, the member-status snapshot, or checkpoint; a later governed unblock restores
   members from the snapshot. *(Operational execution on `154-S` is deferred to the operator/
   Ship once the capability ships — Stage does not mutate `154-S` here; the generic-move
   bootstrap is NOT an admission path, SBLK-R24.)*
2. While `154-S` is `blocked` **and holds no active member** (per member disposition), a
   different queued shipment can be claimed (`queued → active`) and no second shipment can be
   `active` simultaneously.
3. `154-S` (blocked) does not appear in ready-work selection but appears in
   `shipment list --status blocked`.
4. `blocked → active` is refused while another shipment is active and refused when the
   operator has not asserted blocker resolution / readiness gates fail.
5. `blocked → queued` and `blocked → active` clear `blocked_*` metadata.
6. `shipment ship` refuses a blocked shipment; `doctor` flags a blocked shipment missing
   `blocked_reason` and flags two active shipments.
7. Generic `move_item`/`update_item` cannot force a shipment into/out of `blocked`.
8. Existing shipments and existing member/task `blocked` semantics are unchanged;
   `backlogit sync` + `doctor` are clean on a **dedicated fixture workspace** (deterministic
   seeded state, NOT dependent on live corpus counts) that asserts the one-active invariant.

---

## 7. Verification & rollback

* **Verification:** unit tests per implementation unit (declaration → RED → GREEN),
  covering the transition guard matrix, the single-active claim refusal, metadata
  set/clear at every choke point, queue/index exclusion, dependency gating, doctor
  checks, and CLI/MCP parity. A `backlogit sync` + `doctor` pass over the existing
  workspace corpus proves back-compat.
* **Rollback:** the feature is additive and confined to shipment lifecycle code plus
  CLI/MCP/doctor surfaces. Reverting the change restores the prior enum and guards; no
  destructive data migration occurs, so revert is a code-only rollback. Any shipment
  that was set to `blocked` under the feature would, on rollback, hold a status value
  the old binary does not recognize — mitigated by a documented pre-rollback step:
  unblock all `blocked` shipments to `queued` first (surfaced by
  `shipment list --status blocked`).

---

## 8. Scope boundary (Stage)

Stage produces this spec, the deliberation, the implementation plan, the reviewed
backlog hierarchy, and a **queued** shipment. Stage does NOT write application
source/tests, does NOT modify `154-S`, does NOT claim or execute anything, and does NOT
invoke Ship. Implementation is Ship's responsibility once the queued shipment is routed
by the operator.
