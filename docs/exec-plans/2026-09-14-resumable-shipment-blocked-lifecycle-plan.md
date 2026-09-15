---
chunk_strategy: h1-h2-h3
description: "Execution plan for the resumable shipment `blocked` lifecycle status: enum + governed transitions, single-active slot exclusion, audit metadata, queue/index/dependency surfaces, CLI/MCP, doctor, ship/abandon, back-compat."
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-09-14-resumable-shipment-blocked-lifecycle-plan.md
title: "Execution Plan — Resumable shipment `blocked` lifecycle status"
docline:
    stash_id: 808E4323
    informing_defect_stash_id: 7AA35A39
    status: planned
    created_at: 2026-09-14T13:41:59Z
---

# Execution Plan — Resumable shipment `blocked` lifecycle status

**Covering feature:** Resumable shipment `blocked` lifecycle status.
**Source spec:** `docs/product-specs/2026-09-14-resumable-shipment-blocked-lifecycle-status.md` (SBLK-R1…R27).
**Source deliberation:** `docs/decisions/2026-09-14-resumable-shipment-blocked-lifecycle-status-deliberation.md` (Option A, decided).
**Intake:** stash `808E4323`. **Informing defect (out of scope):** stash `7AA35A39`.
**Supersedes:** S12 parked-state unit `164.001-T` (see deliberation §4/§6).

## 0. CURRENT AUTHORITATIVE REVISION — rev3 (2026-09-15)

> §0 is the **authoritative current plan** and supersedes rev1/rev2 §0. All sections below (unit
> bodies U1…U19b, hardening table, and **every `## Plan Review` record**) are **retained as
> historical/audit context**. The prior 29-unit decomposition and the rev2 9-task set are
> superseded; the shipped scope is the concise **12-task RED-before-GREEN set** below. Where §0 and
> a later section conflict, §0 governs. **rev3 removes the rollout circularity, the generic-move
> bootstrap, and the `155-S` topology `--force`.**

### 0.1 Concise task set (feature `174-F` / shipment `155-S`) — 12 tasks, RED-before-GREEN, ≤2h

The rev2 tasks `174.030-T…174.038-T` are **superseded** (parked `blocked`, preserved). Replacement:

| Task | Req | Title | Domain | Depends on |
|---|---|---|---|---|
| `174.039-T` | R1 | Core-lifecycle RED harness (transitions/metadata/intent+preimage/disposition/target-aware unblock) | tests | — |
| `174.040-T` | R2 | Writer/bypass RED harness (writer boundary, generic move/update, MoveShipmentStatus, bulk/cascade, create-as-active) | tests | — |
| `174.041-T` | R3 | Crash/reopen RED harness (durable intent/preimage recovery; subprocess kill+reopen) | tests | — |
| `174.042-T` | R4 | Governed writer core + envelope + public `WriteArtifactFile` boundary | code | R1, R2 |
| `174.043-T` | R5 | Core `BlockShipment` (global lock across intent→preimage→disposition→persist→commit/compensation; member guard) | code | R1, R4 |
| `174.044-T` | R6 | `UnblockShipment` + `Claim` under shared global lock (target-aware restore, CAS/drift refusal, create-active restriction) | code | R1, R5 |
| `174.045-T` | R7a | Route bypass WRITE call-sites (generic move/update, MoveShipmentStatus, bulk/cascade) through governed writer | code | R2, R4, R6 |
| `174.051-T` | R7b | Guard create-as-active + all generic activation paths (refuse activation outside claim/unblock) | code | R2, R4, R6 |
| `174.046-T` | R8 | CLI + MCP parity for block/unblock + read/list status | code | R5, R6 |
| `174.047-T` | R9 | Recovery + normalizer + machine-readable snapshot schema (durable-intent recovery under locks; MCP parity) | code | R3, R5, R6 |
| `174.048-T` | R10 | Subprocess crash/reopen GREEN tests | tests | R3, R9 |
| `174.049-T` | R11 | Doctor production checks (active-count, malformed-blocked, torn-intent; severity/exit/MCP; isolated fixture) | code | R5, R6, R9 |
| `174.050-T` | R12 | Operator docs + branch-scoped bootstrap runbook + topology note | docs | R8, R9, R11 |

```
R1(039) ─┬─────────────────► R4(042) ─┬─► R5(043) ─┬─► R6(044) ─┬─► R8(046) ─┐
R2(040) ─┘                            │            │            ├─► R9(047) ─┼─► R11(049) ─► R12(050)
R3(041) ──────────────────────────────┘ (R9 needs R3,R5,R6)    │            │            ▲
                                       R7a(045)+R7b(051) need R2,R4,R6 ─┘        └─► R10(048)─┘
```

Topological order (parent-first):
`174-F → 174.039 → 174.040 → 174.041 → 174.042 → 174.043 → 174.044 → 174.045 → 174.051 → 174.046 →
174.047 → 174.048 → 174.049 → 174.050`.

Each task's private acceptance criteria (RED-first, workspace-global-lock + durable
intent/preimage-before-mutation invariants, snapshot/restore, member CAS/drift refusal,
governed-refusal of bypass paths, create-active restriction, CLI/MCP parity, normalizer-or-refuse
with machine-readable snapshot input, startup/governed crash recovery under locks, doctor
severity/exit/MCP) live in the task artifacts. Every code task is gated by a failing RED test
(R1/R2/R3) it must turn green; no production code lands before the corresponding RED is failing.
`164.002-T` (S12 forward-repair) is **RETIRED/SUPERSEDED by rev3** (was previously re-pointed to
`174.050-T`): its shipment-record-only `queued → active` contract is incompatible with the rev3
exclusive-activation invariant (only `Claim`/unblock-to-active may create an active shipment, and
both dispose members), and rev3's R9 recovery+normalizer (`174.047-T`) and R11 doctor
(`174.049-T`) subsume its reconciliation role. It is set `blocked` (history preserved, not deleted)
and its obsolete `164.002-T → 174.050-T` dependency is removed, so `146-S` no longer retains a live
member depending on `155-S` (no shipment-level `blocks` edge is required).

### 0.2 Branch-scoped bootstrap for `154-S` (documented; NOT a shipped code unit; NOT executed by Stage)

Authoritative runbook in deliberation §6.2. **No generic move, no pre-governance rollback, no
migration debt.** Sequence: (1) after staging merges, **`155-S` ships normally on `main`** (154-S
& 173.006-T are `queued` there — slot free; no `blocked` token during execution; no `--force`).
(2) Create a **backlog-only `chore/block-154`** branch off synchronized `main`; import ONLY
authoritative `154-S`/`173-F` provenance from the intact Ship branch `…precondition @ dd9f01a1`
with **content hashes + explicit allowlist (no Go/source/harness code)** to reconstruct the
active state; Ship commits untouched, no parallel worktrees. (3) Invoke the **shipped governed
`BlockShipment`** there (legal `active → blocked`): global lock, durable intent + preimage,
machine-readable snapshot (`.backlogit/bootstrap/154-S.snapshot.json`, consumed by R9/`174.047-T`),
member disposition `173.006-T → queued`, governed metadata/event; commit the COMPLETE governed
output as one atomic backlog change — the `154-S` shipment record, EVERY changed member artifact
(`173.006-T` requeued), the durable intent + preimage + machine-readable snapshot/recovery state
(`.backlogit/bootstrap/154-S.snapshot.json`), AND the authoritative per-item event logs for the
shipment and each dispositioned member — then PR-merge to `main`. **Committing only `154-S` +
snapshot is INSUFFICIENT**: it would drop the changed-member, intent/recovery, and per-item event
provenance that R9/`174.047-T` recovery and the R11/`174.049-T` doctor checks consume. (4) Run the corrective `7AA35A39` shipment on `main` while
`154` is `blocked`; later merge `main` into the `154` feature branch, governed **unblock-to-active**
after the fix, resume checkpoint `checkpoint-20260914-070735.json`.

### 0.3 External topology compatibility (VERIFIED — see spec §0.4, deliberation §6 item 3)

`autoharness gate pipeline-topology` currently **rejects** `blocked` (`topology.py:553`,
`_VALID_LIVE_SHIPMENT_STATUSES` excludes it) though its active-slot/consistency logic already
treats `blocked` as non-active. **`155-S` never exercises this** (no `blocked` token during its
execution). For the **later corrective shipment**: smallest fix = one-line upstream allowlist add
(`"blocked"` → `_VALID_LIVE_SHIPMENT_STATUSES`, external Python, not backlogit Go) as an external
ratification item; interim, an **audited per-phase `--force` scoped to that shipment ONLY after the
normal gate proves the sole failure is the unsupported `blocked` status** (no standing override).

---
## Portability boundary (shared vs. backlogit-local — spec §2.5, SBLK-R20…R23)

This capability is already stashed upstream in the **`autoharness` backlog**; this backlogit
implementation is a **local first-mover expected to be eventually superseded by the autoharness
implementation**. Two layers are kept explicit throughout this plan:

* **[shared] portable contract (must stay stable):** the `blocked` status token, the three-edge
  transition set (`active→blocked`, `blocked→queued`, `blocked→active`), the
  non-terminal/resumable/single-active-exclusion behavior, the audit **field names**
  (`blocked_reason`/`blocked_at`/`blocked_by`/`resume_checkpoint_ref`), and the authoritative
  `shipment_status_changed` event. **U17** pins these with a conformance test; **U16** documents
  the migration/compatibility mapping to autoharness.
* **[local] replaceable implementation:** choke-point seams (U2b/U2c), `.locks/` workspace-global
  lock (U5b), SQLite projection (U7b), doctor checks (U13a/U13b), CLI/MCP surface shapes (U9/U11). These
  may be re-implemented upstream and MUST NOT leak backlogit-specific semantics into the shared
  contract, and MUST NOT introduce a conflicting synonym token (SBLK-R21).

## Problem frame

The shipment lifecycle (`queued → active → shipped | abandoned`,
`internal/core/shipment.go:24-36`, guard `isValidShipmentTransition:778-786`) has no
non-terminal parking state. A stalled-but-recoverable shipment (`154-S`) must therefore
either hold the single active slot (P-001) and block all delivery, or be abandoned
(evidence lost). Introduce a governed, resumable `blocked` lifecycle status that is
excluded from the active slot and preserves resumption evidence.

## Constitution check

| Principle | Compliance |
|-----------|-----------|
| I. Safety-First Go | Go 1.24; wrapped errors; fail-closed guards |
| II. Test-First (P-002) | declaration → RED → GREEN per unit |
| III. Workspace Isolation | shipment records within workspace |
| IV. Workspace Containment | all shipment-record writes resolve within the workspace root (CLI/MCP parity is asserted separately per unit) |
| V. Observability | `shipment_status_changed` carries actor+reason+resume_ref on BOTH block and unblock, emitted before frontmatter is cleared |
| VI. Single Responsibility | one unit per surface (enum/guard/metadata/claim/queue/…) |
| VII. Destructive Approval | non-destructive; additive enum value; no data migration |
| VIII. Safety Modes | fail-closed on unrecognized status; governed seam only |
| IX. Git-Friendly | shipment YAML frontmatter |
| X. Context Efficiency | queue filtering excludes blocked |
| XI. Merge Commits | P-009 by Ship |

Constitution check: pass

## Implementation units

Each unit is a single-domain, ≤2-hour task (declaration → RED → GREEN, <3 files,
<5 functions, <4 test scenarios). Domain is `code` unless noted. **P-002.1:** there is
no declaration-only harness exemption — units that land an observable production symbol
(U1, U2c) use a **source-shape harness** (`go/ast`/`go/parser` asserting over package
source text) that compiles before the symbol lands, so the RED is valid, not a build
error. The one docs-only unit (U16) is `harness-exempt` (`harness_exemption_class:
docs-only`); the **closed exempt set enumerated by this plan is exactly `{U16}`**.

### SE-A — Core lifecycle status & governed transitions

* **U1 (SBLK-R1) — Shipment `blocked` enum value + validator.** Add `ShipmentBlocked
  ShipmentStatus = "blocked"` to the shipment status enum; classify it non-terminal in the
  central shipment status helpers; recognize it in status parsing/validation. `ShipmentStatus`
  remains a DISTINCT Go type (no shared string comparison with `models.ArtifactStatus`) so a
  member `blocked` and a shipment `blocked` cannot conflate at the type level. No transitions
  yet. Acceptance: enum value round-trips; non-terminal classification asserted; existing
  statuses unchanged. Source-shape harness for the declaration.
* **U2a (SBLK-R2) — Transition guard matrix.** Extend `isValidShipmentTransition` to allow
  `active → blocked`, `blocked → queued`, `blocked → active`; refuse every other edge into or
  out of `blocked` fail-closed. Acceptance: the three allowed edges pass and `queued→blocked`,
  `blocked→shipped`, `blocked→abandoned`, `blocked→archived` are refused. Depends on U1.
* **U2b (SBLK-R3) — Close generic status-write choke points.** Make the generic
  `move_item`/`update_item` path, **`setArtifactStatus`** refuse a shipment transition whose
  `newStatus=="blocked"` (block direction) OR `oldStatus=="blocked"` (unblock direction),
  gated on `ArtifactType=="shipment"`, with NO forgeable exemption flag. The governed
  `BlockShipment`/`UnblockShipment` functions (U2c) are exempt BY CONSTRUCTION (separate
  functions), never via a flag. (Bulk/cascade are split into U2b2a; exported-core/create entry
  points into U2b2b, to keep each unit within the 2-hour envelope.) Acceptance: each named generic choke
  point refuses both directions regardless of any flag and with the formal gate OFF; member
  `blocked` items are unaffected. Depends on U2a.
* **U2b2a (SBLK-R3) — Close bulk + cascade choke points (task `174.024-T`).** Make
  **`BulkUpdateStatus`** (today only special-cases archived/shipped) and
  **`cascadePersistedParentStatuses`** refuse a shipment `active↔blocked` transition (same
  gate/no-flag rule as U2b); the parent cascade is additionally gated so it can never move a
  shipment out of `blocked`. Exported-core `MoveShipmentStatus` + create/add/create_item are
  OUT OF SCOPE (U2b2b) so each unit stays under 5 functions / 4 scenarios. Acceptance:
  `BulkUpdateStatus`/cascade refuse both directions with the gate OFF and any flag; member
  `blocked` items unaffected. Depends on U2b.
* **U2b2b (SBLK-R3) — Close exported-core + create/add choke points (task `174.028-T`).** Make
  the exported core **`MoveShipmentStatus`** entry point and the shipment
  **`create`/`add`/`create_item`** entry points refuse a shipment `active↔blocked` transition
  (same gate/no-flag rule); `create`/`create_item` cannot originate a shipment directly in
  `blocked`. Acceptance: `MoveShipmentStatus`/`create`/`add`/`create_item` refuse both
  directions with the gate OFF and any flag; a create cannot set `blocked`. Depends on U2b.
* **U2d (SBLK-R3) — Consumer-inventory proof-of-completeness.** Enumerate every
  `ShipmentStatus` consumer (guards, queue/ready-work, index projection, dependency
  eligibility, ship/abandon/reconcile, archive, reporting, event consumers, the generic
  `move_item`/`update_item`/`setArtifactStatus` paths, `BulkUpdateStatus`,
  `cascadePersistedParentStatuses`, **the exported core `MoveShipmentStatus`, and the shipment
  `create`/`add`/`create_item` entry points**) and assert each routes through the central
  non-terminal/terminal classifier or fails closed on the unrecognized value — "no consumer
  default-allows blocked" is a testable acceptance. This is the S12 taxonomy-integration P1
  closure. Acceptance: consumer-inventory test enumerates the full consumer set (including
  `MoveShipmentStatus` + create/add/create_item) and every entry classifies-or-fails-closed.
  Depends on U2b, U2b2a, U2b2b.
* **U2c (SBLK-R3) — Governed block/unblock seam + shared metadata helper.** Add
  `BlockShipment` / `UnblockShipment` core functions that perform the governed transition and
  are the ONLY legitimate write path for `active↔blocked` (all generic paths refuse per U2b).
  The seam persists the `active↔blocked` status edge through a **seam-private lower-level
  persistence primitive that lives BENEATH the guarded choke points** (the guards are layered
  at the `move_item`/`update_item`/`BulkUpdateStatus`/`setArtifactStatus`/cascade choke points,
  not at the raw writer) — so the seam writes without recursing through a refusing choke point
  and WITHOUT any bypass flag; exemption-by-construction is a layering fact, not an assertion.
  A single shared seam-owned helper performs the `blocked_*` clear on unblock and emits the
  `shipment_status_changed` event (actor + reason + `resume_checkpoint_ref`) on BOTH block and
  unblock BEFORE frontmatter is cleared, via the shared EventWriter. Acceptance: governed
  functions succeed; the seam's write does NOT traverse a refusing choke point (tested); both
  edges emit a durable event before clearing; generic paths cannot reach this write.
  Source-shape harness for the new signatures. Depends on U2b, U2b2a, U2b2b.
* **U3 (SBLK-R7) — Blocked audit metadata write.** On `active → blocked`, persist
  `blocked_reason` (required non-empty), `blocked_at`, `blocked_by`, and `resume_checkpoint_ref`
  to frontmatter/`custom_fields`. `blocked_by` is ADVISORY best-effort actor attribution (NOT
  an authorization credential; its trust level is documented); the append-only
  `shipment_status_changed` event is the authoritative non-repudiation record. `blocked_reason`
  is written to frontmatter via **structured YAML marshaling (opaque scalar), never string
  interpolation**, and is treated as data (not a format string) on every CLI/doctor render.
  Acceptance: metadata persisted; empty reason rejected; event carries actor+reason+resume_ref;
  a reason containing YAML metacharacters/newlines round-trips as a single scalar and cannot
  forge or alter any sibling frontmatter key (e.g. `status`, `blocked_by`, `resume_checkpoint_ref`).
  Depends on U2c.
* **U4 (SBLK-R8) — Clear metadata on unblock (single helper).** `blocked → queued` AND
  `blocked → active` clear all `blocked_*` fields via the U2c seam-owned helper (not
  independent blocked-aware branches in generic code). Defense-in-depth: a test asserts no
  generic choke point can perform the unblock write (they refuse per U2b), so the seam is the
  sole path. Acceptance: both edges leave no residual `blocked_*` and emit the unblock event;
  the hook-bypassing `blocked → queued` path is covered. Depends on U2c, U3.
* **U6 (SBLK-R9, R10) — Member disposition & evidence preservation on block.** `active → blocked`
  captures a **governed member-status snapshot** (SBLK-R10) and then returns any **`active`/in-flight
  member to `queued`** so the blocked shipment holds **no active release-unit execution** (this is
  what actually frees the single active slot — a still-`active` member would keep P-001 contended).
  Non-active member statuses are left unchanged. Branch association and `resume_checkpoint_ref` are
  preserved intact. `blocked → active` (governed unblock/resume) **restores** members from the
  snapshot. The requeue is performed by the U2c seam through its seam-private primitive, is recorded
  in the audit event, and is reversible via the snapshot (a governed disposition, NOT an ungoverned
  member mutation, and NOT a member `blocked` cascade / `return-blocked`). All `blocked_*` predicates
  gate on `ArtifactType=="shipment"`. Acceptance: after block, no member remains `active` and the
  snapshot records every pre-block status; branch + checkpoint preserved; unblock restores members
  exactly; a member `blocked` (return-blocked) satisfies NO shipment-blocked predicate and vice
  versa. Depends on U2c.

### SE-B — Active-slot & concurrency

* **U5b (SBLK-R4, R6) — Workspace-global active-slot serialization + source of truth.** Provide a
  single **workspace-scoped** serialization — ONE global claim lock keyed on the workspace root
  under `.locks/` (NOT a per-artifact mutation lock; two concurrent claims of DIFFERENT shipments
  must contend on the SAME key), or an atomic active-slot compare-and-swap — that guards the
  active-slot check-then-set. **Active-slot source of truth:** the authoritative active count is
  computed from an **authoritative scan of the contained shipment Markdown records** (canonical
  state) or an equivalent real CAS on that state — NEVER from the SQLite index (a rebuildable,
  non-authoritative projection). The check **fails closed** when the index is stale/missing (scan
  is authoritative) and when the scanned state is malformed or shows a duplicate/ambiguous active
  condition (refuse, do not guess). Acceptance: two concurrent `ClaimShipment` calls on different
  shipments serialize on the one global key; at most one activates; a stale/missing index does not
  admit a second active (scan wins); a malformed/duplicate-active scan refuses fail-closed. Depends
  on U1.
* **U5 (SBLK-R4) — Single-active-slot enforcement in claim.** `ClaimShipment`
  (`queued → active`) refuses fail-closed when another shipment is already `active` (blocked
  shipments excluded from the count), under the U5b serialization, atomic-by-construction with
  rollback of the activated member set. *(Operator-requested: "ensure only one shipment is
  active at any time." P-001 was previously convention-only; this adds forward-only
  enforcement — see U15 for back-compat.)* Acceptance: second concurrent/ sequential claim
  refused; claim allowed while a different shipment is `blocked`; rollback on mid-flight
  failure. Depends on U5b.
* **U8 (SBLK-R6) — Blocked non-claimable + contention proof.** A `blocked` shipment cannot be
  claimed (claim requires `queued`) or executed. A contention test drives two concurrent
  claims and a concurrent unblock-to-active and asserts the global serialization holds after a
  relocating transition (lock keyed on stable ID, never current path). Acceptance: a claim
  targeting a `blocked` shipment is refused fail-closed; under the concurrent
  claim+unblock-to-active drive at most one shipment ends `active` and the U5b serialization
  key is stable across the relocating transition. Depends on U5, U5b.

### SE-C — Queue, dependency, index visibility

* **U7a (SBLK-R11) — Queue / ready-work exclusion.** Exclude `blocked` shipments from
  ready-work selection while keeping them in the queue directory (not archived). Acceptance:
  blocked excluded from ready-work; not archived. Depends on U1.
* **U7b (SBLK-R11) — Status-filter + rebuild stability (narrowed).** `blocked` is already a
  valid `ArtifactStatus` the generic index projects, so this unit does NOT re-implement a
  bespoke projection. It NARROWLY asserts that `shipment list --status blocked` returns blocked
  shipments and that the `blocked` value survives an index rebuild after `sync` (row count stable
  across a mid-walk-cancel rehydration test). The index remains a non-authoritative projection
  (SBLK-R4/R27 — the active-slot source of truth is the Markdown scan, not this index).
  Acceptance: status filter returns blocked; value stable across rebuild. Depends on U1.
* **U10 (SBLK-R12) — Dependency eligibility.** A `blocked` shipment as a dependency remains
  execution-blocking (NOT added to the `IsNoLongerBlockingStatus` set). Acceptance: a dependent
  stays gated while its dependency shipment is `blocked`; unblock+terminal releases it. Depends on U1.

### SE-D — Operator surfaces (CLI / MCP)

* **U9 (SBLK-R13) — CLI block/unblock.** `backlogit shipment block <id> --reason <text>
  [--resume-checkpoint <ref>]` and `backlogit shipment unblock <id> --to queued|active
  --confirm`, routing through the governed seam; naming distinct from `return-blocked`.
  `--confirm` is REQUIRED on BOTH unblock targets (non-authoritative blocker-resolution
  confirmation). Acceptance: both verbs route the governed path; `--reason` required for block;
  `--confirm` required for both unblock targets; unblock-to-active honors the readiness gate
  (U12); help/usage updated. Depends on U2c, U3, U4, U5, U12.
* **U11 (SBLK-R14) — MCP block/unblock + filter parity.** MCP tools for block/unblock at parity
  with the CLI and `blocked` in shipment status filters, consistent JSON-RPC error mapping. On
  MCP there is no interactive operator: the `--confirm`/blocker-resolution input is treated as a
  NON-authoritative confirmation (a bare client boolean is NOT sufficient authorization); the
  authoritative gate is the backlogit-side free-slot check (U12); the actor and its trust level
  are recorded in the audit event. Acceptance: MCP isomorphic to CLI; status filter returns
  blocked; registry parity test passes; MCP unblock still enforces the free-slot gate. Depends
  on U2c, U3, U4, U5, U12.

### SE-E — Integrity, gates, ship/abandon, readiness

* **U12 (SBLK-R5) — Unblock readiness gate (concrete).** `blocked → active` requires the active
  slot to be free (concrete backlogit-side check under the U5b serialization) AND an explicit
  operator `--confirm` flag documented as a NON-authoritative confirmation of blocker resolution
  (backlogit cannot verify an external blocker such as `RED_DELIVERABLE_DELTA_OUT_OF_SURFACE`, so
  the flag carries no authorization weight and is never treated as a credential). The free-slot
  check and the activating write occur inside the **SAME held U5b critical section** — the slot
  is checked and the shipment is moved to `active` without releasing the workspace-global lock
  between check and set, closing the TOCTOU window (mirrors the U5 claim path). `blocked →
  queued` requires only `--confirm`. The external autoharness numeric-predecessor topology gate is
  OUT-OF-REPO and is NOT relied upon here. Acceptance: `blocked→active` refused when a shipment is
  active OR `--confirm` absent, allowed when slot free AND `--confirm` present; a concurrent
  claim + unblock-to-active cannot both reach `active` (check-and-set under one held lock);
  `blocked→queued` refused when `--confirm` absent; **unblock (either target) refused when the
  shipment's governed `blocked_*` metadata is absent** (outstanding bootstrap-migration debt,
  SBLK-R25) with a remediation message pointing at the U18a normalizer. Depends on U2c, U5, U5b.
* **U13a (SBLK-R15, R16) — Doctor: active-count + malformed-blocked checks.** `backlogit doctor`,
  over `ArtifactType=="shipment"` only, verifies: (a) ≤1 active shipment (`error` severity); (b)
  every blocked shipment has non-empty `blocked_reason` + valid `blocked_at` (`error` severity; a
  record lacking `blocked_reason` is a hard finding naming the U18a normalizer). The former
  "blocked AND terminal" scalar check is **removed** — `status` is a single scalar, so a record
  cannot simultaneously be `blocked` and terminal; the check was unsatisfiable and is not a
  meaningful corruption detector. Reads `blocked_*` from the Markdown source (the index does not
  project them). Acceptance (≤4 scenarios): flags a malformed blocked shipment; flags a two-active
  condition; the removed check is not present. Depends on U1, U5.
* **U13b (SBLK-R16) — Doctor: member-ignore, fail-closed, severity/exit/MCP contract.** Over
  `ArtifactType=="shipment"` only, ignores member `return-blocked` items (no false positive on a
  member `blocked` with no `blocked_reason`); fails closed on an unrecognized shipment status
  (`error` severity). Defines the **finding contract**: each check declares a `severity`
  (`error` for one-active, malformed-blocked, unknown-status); a deterministic process **exit
  code** (non-zero when any `error` finding is present); and an **MCP result contract** at parity
  with the CLI (structured findings with `severity`, `code`, `artifact_id`, `message`).
  Acceptance (≤4 scenarios): ignores a member `blocked` item; fails closed on an unknown status;
  exit code non-zero on an `error` finding; MCP result contract asserted at CLI parity. Depends on
  U13a.
* **U14 (SBLK-R17) — Ship/abandon guard integration.** `shipment ship` refuses a `blocked`
  shipment; the guard prevents `blocked → shipped/archived/abandoned`; `reconcile-shipped`
  (archived-only) is unaffected; terminal abandonment path is `blocked → active → abandoned`.
  Acceptance: ship of blocked refused with clear error; reconcile path unchanged. Depends on U2a.

### SE-F — Back-compat & docs

* **U15 (SBLK-R18) — Back-compat (no fail-open marker).** The additive enum requires no data
  migration. A pre-existing multi-active condition is reported by doctor (U13a) as a HARD finding
  with explicit remediation (unblock one shipment to `queued`) — NOT a resettable "first-run"
  warning and never a fail-open flag; claim enforcement (U5) is forward-only, so it refuses
  creating a new second active without retroactively mutating existing state. Acceptance: over a
  **dedicated fixture workspace** seeded with exactly one active shipment, `sync` + `doctor` are
  clean; an injected second active in the fixture is a hard doctor finding with remediation text.
  (The test asserts against the fixture, NOT the live repository corpus, so it stays deterministic
  as the real backlog evolves.) Depends on U13a, U13b.
* **U16 (docs; harness-exempt, `docs-only`, closed exempt set `{U16}`) — Operator docs & runbook.**
  CLI reference for block/unblock, a shipment lifecycle doc update, the `164-F` parked supersession
  note (including that S12 forward-repair `164.002-T` must re-point its dependency/acceptance from
  `parked` to canonical `blocked`), a rollback runbook (`shipment list --status blocked` then
  unblock each to `queued` before downgrading), and a **cross-repo migration/compatibility note**
  (SBLK-R22/R23): the shared `blocked` contract (token, transitions, field names, event) —
  labelled **backlogit-proposed / assumed pending autoharness ratification** — and its
  non-destructive, additive-only, backlogit-side-stable mapping toward the upstream `autoharness`
  implementation that is expected to supersede this one (flagging any field whose upstream
  counterpart is not yet known), plus the external informing reference (no autoharness ID
  identifiable locally). Domain: docs. Acceptance: docs pass markdown lint; runbook + supersession
  note + migration/compat note present. Depends on U9, U14, U17.

  Also documents the **one-time, operator-approved, pre-governance `154-S` bootstrap runbook**
  (SBLK-R24/R25/R26) that resolves the rollout circularity. Because the governed `BlockShipment`
  seam does not exist until `155-S` ships, and `155-S` cannot claim the slot until `154-S` is
  blocked, the runbook is an **exceptional, one-time, operator-approved pre-governance migration**
  (NOT the steady-state API, removed after migration) that manually reproduces the governed
  guarantees: (1) snapshot every member status durably, then **manually disposition the active
  member** (`move 173.006-T --status queued`) BEFORE (2) the generic `move 154-S --status blocked`,
  so the slot is genuinely freed (P-001 uncontended); (3) append a durable audit naming the
  migration debt; (4) run the autoharness topology pre-claim check for `155-S`. **Bounded rollback
  is valid ONLY before any other claim**: verify no other shipment is active, then `move 154-S
  --status active` and restore each member to its exact snapshot status; otherwise **fail closed**
  (forward-fix only). After `155-S` ships, the governed U18a/U18b normalizer backfills canonical
  metadata/event before any unblock (U12 refuses unblock while debt is outstanding). Autoharness
  topology-gate caveat: blocking `154-S` frees only backlogit's active-slot scan; the external
  autoharness gate must be verified separately. Additional acceptance: one-time operator-approved
  pre-governance runbook (snapshot → manual member disposition → status move → audit → verify →
  bounded pre-claim rollback), member-disposition semantics, debt-normalization-before-unblock, and
  autoharness-gate caveat all present. (Also depends on U18a, U18b.)
* **U17 (SBLK-R20, R21 — [shared] contract conformance) — Portable-contract conformance test.**
  A test that PINS the shared cross-repo contract so backlogit-local work cannot silently diverge,
  scoped to the **blocked-related edges only** (it does NOT re-pin unrelated shipment transitions):
  asserts the status token is exactly the string `blocked`; the blocked-related shipment transition
  set is exactly the three shared edges (`active→blocked`, `blocked→queued`, `blocked→active`); the
  audit field names are exactly
  `blocked_reason`/`blocked_at`/`blocked_by`/`resume_checkpoint_ref`; the event name is
  `shipment_status_changed`; and that NO conflicting synonym token (`parked`, `paused`, `on-hold`)
  is introduced for the same concept. The pinned values are documented as a **backlogit-proposed /
  provisional contract pending upstream autoharness ratification** — the test guards backlogit-side
  stability, and if upstream ratifies different names the update is a deliberate, reviewed contract
  change, not silent drift. Domain: tests. Acceptance: the conformance test enumerates the shared
  tokens/blocked-edges/field-names and fails if any is renamed, removed, or a synonym is added.
  Depends on U1, U2a, U3.
* **U18a (SBLK-R24, R25 — bootstrap-migration debt discharge) — Blocked-shipment normalizer core
  (task `174.023-T`).** Provide a governed normalizer (`backlogit shipment normalize-blocked <id>
  --reason <text> [--resume-checkpoint <ref>]`, routed through the U2c seam) that backfills the
  governed `blocked_reason`/`blocked_at`/`blocked_by`/`resume_checkpoint_ref` and emits the governed
  `shipment_status_changed` event for a shipment whose status token was set to `blocked` out-of-band
  (e.g. a degraded or test-seeded record) and therefore lacks governed metadata. Idempotent: a
  no-op on an already-governed blocked shipment. **Member-disposition reconstruction (required):**
  because an out-of-band `blocked` record may have skipped member disposition, the core normalizer
  MUST also reconstruct the governed **member snapshot** and **requeue any still-`active` member**
  (restoring the same "no active member" invariant a governed block would have produced), OR
  **REFUSE fail-closed** when the snapshot/disposition cannot be proven from available evidence — it
  never leaves an active member on a `blocked` shipment and never guesses. This unit normalizes
  **degraded/test-seeded** records; doctor/unblock/MCP integration is OUT OF SCOPE (U18b). Domain:
  code. Acceptance: backfills all four fields + emits the event; idempotent; reconstructs snapshot
  and requeues active members OR refuses when reconstruction is unprovable; leaves no active member
  on success. Depends on U2c, U3, U6.
* **U18b (SBLK-R25) — Normalizer integration + MCP parity decision (task `174.026-T`).** Wire the
  normalizer into the integrity/gate surfaces: `doctor` (U13a) flags a status=`blocked` shipment
  with no `blocked_reason` as a **hard finding naming this normalizer**; the unblock readiness gate
  (U12) **refuses** unblock until normalization completes; and resolve the **MCP parity decision** —
  `normalize-blocked` is exposed at MCP at parity with the CLI (or explicitly recorded as
  CLI-only with rationale). Domain: code. Acceptance: a bootstrap/degraded blocked shipment cannot
  be unblocked until normalized; doctor flags the un-normalized case naming the normalizer; the MCP
  parity decision is asserted (parity test OR recorded CLI-only rationale). Depends on U18a, U12,
  U13b.
* **U19a (SBLK-R27 — durability contract) — Block/unblock/normalize durability + commit protocol
  (task `174.027-T`).** Define and implement the authoritative multi-write ordering/durability for
  block/unblock/normalize across the event, frontmatter/`custom_fields`, index projection, AND the
  member-snapshot/member-status (requeue) writes. **Intent/commit protocol:** persist an INTENT
  record with a correlation id, then the COMMITTED `shipment_status_changed` event, so an appended
  event ALONE can NEVER falsely assert a completed transition; reconciliation treats an INTENT
  without a matching COMMITTED (and without corresponding frontmatter/member state) as an incomplete
  transition to roll back or forward. (A persist-first-then-committed-event design is an acceptable
  equivalent.) The index is a rebuildable non-authoritative projection reconciled by `sync`.
  Classified reconciliation for each partial-failure point (incl. event-without-frontmatter and
  event-without-member-requeue) leaves either a fully-committed or a refused/rolled-back transition
  — never a torn governed state; doctor/recovery detects residual inconsistency. Domain: code.
  Acceptance: intent+committed protocol implemented; each classified partial-failure point resolves
  to committed-or-refused; index self-heals on rebuild; no partial governed state passes doctor.
  Depends on U2c, U3, U6.
* **U19b (SBLK-R27 — durability tests) — Block/unblock/normalize failure-injection tests
  (task `174.029-T`).** Failure-injection tests simulating a crash between EACH write step —
  shipment status/frontmatter, the event (intent vs committed), the member-snapshot + member-status
  (requeue) writes, and the index projection — explicitly covering **event-without-frontmatter** and
  **event-without-member-requeue** reconciliation. Domain: tests. Acceptance: each injected crash
  leaves a committed-or-recoverable state (no silent lost transition, no torn governed state);
  `sync`/`doctor` recover or flag; the intent-vs-committed ordering is asserted so an append-only
  event cannot falsely assert completion. Depends on U19a, U6.

## Dependency graph

```
U1 ──┬─► U2a ─► U2b ─┬─► U2b2a ─┐
     │               └─► U2b2b ─┼─► U2c ─┬─► U3 ─► U4
     │                          └─► U2d  │   └─► U6
     │                          (seam U2c consumed by U9, U11, U12, U18a, U19a)
     ├─► U5b ─► U5 ─► U8
     ├─► U7a
     ├─► U7b
     ├─► U10
     └─► U2a ─► U14
U5b ─► U12 ; U5 ─► U12 ; U2c ─► U12
{U2c,U3,U4,U5,U12} ─► U9 ; {U2c,U3,U4,U5,U12} ─► U11
{U1,U2a,U3} ─► U17
{U2c,U3,U6} ─► U18a ; {U18a,U12,U13b} ─► U18b
{U2c,U3,U6} ─► U19a ─► U19b ; U6 ─► U19b
{U1,U5} ─► U13a ─► U13b ; {U13a,U13b} ─► U15
{U9,U14,U15,U17,U18a,U18b} ─► U16
```

Execution order (topological): U1 → {U2a, U5b, U7a, U7b, U10} → {U2b, U5, U14} →
{U2b2a, U2b2b, U8, U13a} → {U2c, U2d, U13b} → {U3, U6, U12, U15} →
{U4, U17, U18a, U19a} → {U9, U11, U18b, U19b} → U16.

Task-ID execution order (155-S manifest, parent-first): 174-F → 174.001 →
{174.002, 174.009, 174.012, 174.013, 174.014} → {174.003, 174.010, 174.019} →
{174.024, 174.028, 174.011, 174.018} → {174.004, 174.005, 174.025} →
{174.006, 174.008, 174.017, 174.020} → {174.007, 174.022, 174.023, 174.027} →
{174.015, 174.016, 174.026, 174.029} → 174.021.

## Runtime verification & closure

Runtime surfaces: shipment lifecycle core, claim path, queue/ready-work selection, CLI,
MCP, doctor. Verification: the transition guard matrix, single-active claim refusal,
metadata set/clear at every choke point, queue/index exclusion, dependency gating, doctor
checks, and CLI/MCP parity are each covered by unit tests; **partial-failure durability is
covered by failure-injection tests (U19b) over the intent/commit durability contract (U19a)**; a `backlogit sync` + `doctor` pass over a
**dedicated fixture workspace** (not the live corpus) proves back-compat. Closure: operator
docs + rollback runbook (U16); blocked-status audit metadata schema documented.

## Rollback / verification (feature-level)

The feature is additive and code-only. Rollback reverts the enum, guards, seam functions,
and surfaces; no destructive data migration occurs. Pre-rollback runbook: `shipment list
--status blocked` then unblock each to `queued` so no artifact holds a status value the
prior binary cannot recognize.

## Plan Hardening

| ProposedAction | ActionRisk | Mitigation |
|---|---|---|
| New shipment lifecycle status + transition edges | Medium — taxonomy/gate integration (the exact S12 parked P1) | Single canonical `blocked` (Option A); closed guard matrix; **consumer-inventory proof** that no `ShipmentStatus` consumer default-allows blocked (U2b/U2b2a/U2b2b/U2d); fail-closed on unrecognized; doctor asserts well-formedness (U13a) |
| Governed block/unblock transitions | Medium — ungoverned bypass via generic move/update, **BulkUpdateStatus, setArtifactStatus, cascade** | Unconditional refusal at EVERY status-write choke point keyed on old/new status=='blocked' AND ArtifactType=='shipment' (U2b); parent cascade gated so it cannot move a shipment out of blocked; governed `BlockShipment`/`UnblockShipment` exempt by construction; no forgeable flag (U2c) |
| Single-active enforcement (operator-requested; P-001 was convention-only) | Medium — race could double-activate; could surface pre-existing multi-active | Workspace-GLOBAL active-slot serialization / CAS, not the per-artifact lock (U5b); atomic claim with member-set rollback (U5); pre-existing multi-active is a HARD doctor finding with remediation, not a fail-open marker (U15) |
| Blocked audit metadata lifecycle & non-repudiation | Medium — stale `blocked_*` after hook-bypassing `blocked→queued`; audit erased on clear | Single seam-owned clear helper (U4); durable `shipment_status_changed` event with actor+reason+resume_ref emitted on BOTH block and unblock BEFORE clearing (U2c/U3); `blocked_by` documented as advisory, events authoritative |
| Member vs shipment `blocked` conflation | Medium — shared "blocked" string on shared status field | Distinct `ShipmentStatus` Go type (U1); every blocked_* predicate/doctor/guard gates on ArtifactType=='shipment' (U6/U13a); cross-axis negative test |
| Atomic writes across MD + SQLite + JSONL | Medium — torn state / stale index on the new value | Transactional index rebuild with mid-walk-cancel test (U7b); commit-then-surface durability; shared EventWriter for event append (U2c) |
| Unblock readiness is not an authorization boundary | Medium — self-asserted `--confirm`/TTY is spoofable (S12 precedent) | `--confirm` documented as NON-authoritative; the authoritative gate is the backlogit-side free-slot check under U5b (U12); MCP rejects a bare client boolean as sufficient and records actor trust level (U11) |
| Downgrade with blocked shipments present | Low — unknown status to old binary | Additive enum; documented pre-rollback unblock runbook (U16) |
| Cross-repo contract divergence (backlogit-local drift from the shared autoharness contract) | Medium — a local synonym token, extra edge, or renamed field would break the eventual autoharness supersession | Explicit shared-vs-local boundary (spec §2.5); portable-contract conformance test pins token/edges/field-names and forbids synonyms (U17); non-destructive additive-only migration with a documented 1:1 mapping note (U16, SBLK-R22); external informing reference recorded without inventing an ID (SBLK-R23) |
| One-time, operator-approved, pre-governance `154-S` bootstrap (resolving the rollout circularity) | Medium — a naive generic `move --status blocked` would do NO member disposition (member `173.006-T` stays `active` → P-001 contended) and has no bounded rollback | The bootstrap is **exceptional, one-time, operator-approved, removed after migration** (SBLK-R24), NOT the steady-state API; it manually reproduces the governed guarantees — snapshot members, **manually disposition the active member to `queued` FIRST**, then generic `move 154-S --status blocked`, durable debt audit, autoharness topology pre-claim check; **bounded rollback valid ONLY before any other claim** (verify no other active, move 154 active, restore exact member snapshot; else fail closed); governed U18a/U18b normalizer backfills canonical metadata/event and U12 refuses unblock until discharged (SBLK-R25); doctor flags the un-normalized case (U13a); autoharness topology-gate treatment of `blocked` flagged as an unconfirmed external assumption requiring separate verification (SBLK-R26). Stage does NOT execute it. |
| Member disposition on block (freeing the single active slot) | Medium — leaving members `active` keeps P-001 contended; ungoverned member mutation loses resumption evidence | Governed member-status snapshot captured on block; active members returned to `queued`; branch + `resume_checkpoint_ref` preserved; unblock restores members from the snapshot; requeue recorded in the audit event and reversible (U6) |
| Partial failure across event append + frontmatter + index on block/unblock/normalize | Medium — a crash mid-operation could silently lose a transition or leave a torn state | Intent/commit protocol (persist INTENT + correlation id → frontmatter+member writes → COMMITTED event) so an append-first event alone never asserts completion; classified handling per crash point; failure-injection tests assert fully-applied-or-recoverable, never silent loss; doctor/recovery reconcile INTENT-without-COMMITTED (U19a, U19b, U13a) |

Rollback trigger: any correctness failure in the guard matrix or claim exclusion; bounded
to shipment lifecycle code, reversible by code revert. Ownership: Ship at execution time.

### Plan Hardening Signals (REQUIRED)

* public API/schema/contract change: PRESENT — new shipment lifecycle status value, new
  transition edges, new CLI verbs, new MCP tools, new audit fields.
* security/auth/permission/compliance-sensitive: PRESENT — governed block/unblock
  break-glass transitions and single-active-slot enforcement.
* migration/backfill/destructive/irreversible: PRESENT-minor — additive enum, no data
  migration; forward-only back-compat handling for pre-existing multi-active.
* external integration/operator checkpoint/external dependency: PRESENT — operator-supplied
  reason + resume-checkpoint reference; external autoharness numeric gate acknowledged
  out-of-scope.
* high runtime/rollout/rollback risk: absent — bounded to shipment lifecycle; code-only
  rollback with a documented pre-rollback runbook.

Requires plan hardening: yes

<!-- plan-review-attempt: 1 -->

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: FAIL

personas:
* Constitution Reviewer (`claude-opus-4.8`) — VOTE: ADVISORY
* Correctness Reviewer (`claude-sonnet-4.6`) — VOTE: FAIL
* Architecture Strategist (`grok-4.6`) — VOTE: ADVISORY
* Scope Boundary Auditor (`gemini-3.7-flash`) — VOTE: ADVISORY
* Security Reviewer (`gpt-5.6-terra`) — VOTE: ADVISORY
* Learnings Researcher over `docs/compound/` — prior art applied (124-F, atomic-claim-rollback, shipped-prevention envelope, durable-writes, stable-lock-keys)

Controlling P1 findings (must clear before harvest):
* P1-a (Correctness): the single-active check-then-set was scoped to the per-artifact mutation lock, which does not serialize two DIFFERENT concurrent claims — INV-1 (single-active) is violable. Requires a workspace-GLOBAL active-slot serialization (single lock or atomic compare-and-swap) shared by claim and unblock-to-active.
* P1-b (Correctness): `BulkUpdateStatus` (plus `setArtifactStatus`/`cascadePersistedParentStatuses`) is a Markdown-first status writer that bypasses `isValidShipmentTransition` and only special-cases archived/shipped today. The governed-seam refusal (U2b) named only move_item/update_item, leaving an ungoverned entry/exit to `blocked` that violates INV-1/INV-3/INV-4/INV-6. Refusal + metadata-clearing must cover every status-write choke point.

Material P2 findings folded into v2:
* Every blocked_* invariant/doctor/guard predicate must gate on ArtifactType=='shipment' to avoid conflation with member/task `return-blocked` (which correctly has no blocked_reason).
* Clear-metadata + event emission owned by a single governed-seam helper; emit shipment_status_changed with actor+reason+resume_ref on BOTH block and unblock BEFORE clearing frontmatter.
* Unblock readiness gate reduced to concrete, testable checks (free active slot + explicit non-authoritative operator --confirm); external autoharness numeric gate acknowledged out-of-repo and not relied on; assertion documented as non-authoritative (spoofable).
* Back-compat: pre-existing multi-active surfaces as a hard doctor finding with remediation (no resettable first-run marker / no fail-open); claim enforcement is forward-only.
* Split U7 into U7a (queue/ready-work exclusion) and U7b (index projection + list filter + sync rebuild); split U2b (choke-point refusal) from U2c (governed seam behaviour); add workspace-global serialization unit.
* Add consumer-inventory proof-of-completeness for all ShipmentStatus consumers (taxonomy closure verified, not asserted).
* Declaration units use a source-shape harness (P-002.1, no declaration-only exemption); U16 is harness-exempt docs-only, member of the closed exempt set {U16} enumerated in this plan.
* Constitution check IV restated as workspace containment; dependency graph de-duplicated; U13(c) justified as external-corruption detection.

Cross-artifact: deliberation §6 updated to require 164.002-T (S12 forward-repair) to re-point its dependency/acceptance from the superseded `parked` state to canonical `blocked`.

This FAIL record is superseded by the v2 re-review below.

<!-- plan-review-attempt: 2 -->

## Plan Review

dispatch_mode: multi-agent-dispatch
decision: ADVISORY
operator_authorization: approved

personas (v2 re-dispatch over the hardened plan):
* Correctness Reviewer (`claude-sonnet-4.6`) — VOTE: ADVISORY — Cleared-from-attempt-1: P1-a YES, P1-b YES
* Architecture Strategist (`grok-4.6`) — VOTE: ADVISORY
* Scope Boundary Auditor (`gemini-3.7-flash`) — VOTE: ADVISORY
* Security Reviewer (`gpt-5.6-terra`) — VOTE: ADVISORY

Gate outcome: **ADVISORY** — no P1 findings from any persona; both controlling attempt-1
P1s confirmed cleared (P1-a by the workspace-global active-slot serialization U5b; P1-b by
the multi-choke-point refusal U2b covering `move_item`/`update_item`, `BulkUpdateStatus`,
`setArtifactStatus`, `cascadePersistedParentStatuses`, gated on `ArtifactType=="shipment"`,
plus the parent-cascade gate and consumer-inventory proof).

Advisory P2 findings — all folded into the plan before harvest:
* Governed-seam write-path layering (Correctness + Architecture + Security convergent): the
  seam must persist the `active↔blocked` edge WITHOUT recursing through a now-refusing choke
  point and without a bypass flag. → U2c now specifies a seam-private lower-level persistence
  primitive BENEATH the guarded choke points (layering fact, tested: seam write does not
  traverse a refusing choke point).
* Unblock-to-active TOCTOU (Correctness): the free-slot check and the activating write must
  occur inside the SAME held U5b critical section. → U12 now binds check-and-set under one
  held workspace-global lock; U12 now also depends on U5; acceptance covers the concurrent
  claim + unblock-to-active race.
* U2b sizing / verification hygiene (Scope): consumer-inventory proof split out of U2b into a
  new dedicated unit **U2d** to keep both within the 2-hour envelope; U8 given an explicit
  labeled Acceptance clause.
* blocked_reason serialization safety (Security): U3 now requires structured YAML marshaling
  (opaque scalar, never string interpolation), treats the reason as data on render, and adds a
  round-trip acceptance proving YAML metacharacters/newlines cannot forge sibling frontmatter
  keys.
* Event-durability ordering (Security, non-blocking): the block/unblock event is emitted via
  the shared EventWriter BEFORE metadata clear (U2c commit-then-surface). Recorded; the
  fsync-ordering nuance is an implementation acceptance for U2c, not a plan-level gap.
* Deliberation §3/§4 vs §6 wording on 164.002-T (Architecture, cosmetic): §6 records the
  re-point recommendation; §3/§4 "unaffected/untouched" refers to Stage not rewriting it —
  reconcilable, left as-is.

Authorization basis: operator issued a standing directive to run the full Stage intake
pipeline end-to-end (brainstorm → deliberate → plan → harden/review → harvest → assemble a
queued shipment "if safely assembled"). Gate is ADVISORY with zero P1s and every advisory P2
remediated inline; this satisfies the ADVISORY-with-operator-confirmation path. Proceeding to
harvest.

<!-- plan-review-attempt: 2-amendment -->

## Plan Review — Amendment (cross-repo contract boundary)

dispatch_mode: multi-agent-dispatch
decision: ADVISORY
operator_authorization: approved

Trigger: operator clarification (2026-09-14T14:29) — the capability is already stashed in the
upstream `autoharness` backlog; the backlogit implementation is a local first-mover expected to
be superseded by autoharness. Required: explicitly separate the shared portable
status/transition/behavior contract from backlogit-local implementation detail; require
compatibility/migration; avoid backlogit-specific semantics that conflict with autoharness; link
the autoharness reference if locally identifiable, else record an external informing item without
inventing an ID.

Additive delta: spec §2.5 (shared-vs-local boundary) + SBLK-R20…R23; plan "Portability boundary"
section; new tests unit **U17** (portable-contract conformance test); U16 extended with a
migration/compat note; hardening-table divergence row; dependency graph/topo updated. No
autoharness-side work, no invented ID, no speculative migration tooling.

personas (focused re-dispatch over the delta):
* Architecture Strategist (`grok-4.6`) — VOTE: ADVISORY — shared/local seam drawn correctly; U17 pins the right invariants and is acyclically placed; P2: frame the migration mapping as backlogit-proposed pending autoharness ratification (one-sided guarantee) rather than an agreed bilateral 1:1 — **applied** to SBLK-R22 + U16.
* Scope Boundary Auditor (`gemini-3.7-flash`) — VOTE: PASS — U17 is a genuine ≤2h single-domain tests unit with explicit acceptance; no scope creep, no invented ID, no upstream implementation pulled in; every new requirement verifiable and traceable.

Gate outcome: **ADVISORY** — no P1 findings; the single actionable P2 (contract-framing) was
applied inline. Base plan approval (attempt 2) stands; this amendment is additive and does not
reopen any prior finding. Harvest adds one task (U17) to the existing feature/shipment.

<!-- plan-review-attempt: 2-amendment-2 -->

## Plan Review — Amendment (154-S bootstrap migration seam)

dispatch_mode: multi-agent-dispatch
decision: ADVISORY
operator_authorization: approved

Trigger: operator new-evidence turn (2026-09-14) — local code already supports `models.StatusBlocked`,
generic `move --status blocked` (→ `UpdateArtifactWithGate`), and `list --type shipment --status
blocked`, while shipment-specific `ShipmentStatus`/`isValidShipmentTransition` still lack `blocked`
and no governed reason/checkpoint metadata exists. Required: amend spec/decision/plan with a
one-time bootstrap migration for `154-S` using the generic move ONLY as a temporary compatibility
seam; mark missing `blocked_reason`/`blocked_at`/`blocked_by`/`resume_checkpoint_ref` and
shipment-specific event as migration debt that `155-S` must backfill/normalize before unblocking;
require pre/post verification + rollback; assess whether the autoharness pipeline-topology gate
treats an existing `blocked` shipment as non-active or needs a separate external compatibility gate.

Additive delta: spec SBLK-R24 (bootstrap seam), R25 (migration debt & U18 normalization,
unblock-refused-while-debt, doctor hard finding), R26 (autoharness topology-gate assessment);
deliberation §6 one-time bootstrap procedure (exact generic-move command, pre/post verification,
status-only rollback, debt enumeration, fail-closed autoharness-gate assessment); plan unit **U18**
(governed normalizer that backfills metadata + emits the event, idempotent; doctor + U12 integration);
U12 acceptance extended (refuse unblock while governed `blocked_*` absent); U16 extended (bootstrap
runbook + autoharness-gate caveat); hardening-table bootstrap row; dependency graph/topo updated.
Stage does NOT execute the bootstrap (operator-owned; `154-S` not edited).

personas (focused re-dispatch over the delta):
* Correctness Reviewer (`claude-opus-4.8`) — VOTE: ADVISORY — the status-only, non-cascading generic
  move preserves members/branch/checkpoint by construction; rollback is a clean single-field revert;
  U18 idempotence + U12 unblock-refusal + doctor hard-finding correctly close the debt loop so a
  degraded blocked shipment cannot silently unblock. P2: state explicitly that the bootstrap must not
  run before the pre-verification snapshot is captured — **applied** to deliberation §6 ordering.
* Scope Boundary Auditor (`gemini-3.7-flash`) — VOTE: PASS — U18 is a single-domain ≤2h code unit
  with explicit acceptance; the bootstrap is scoped as a temporary seam, never the contract; the
  autoharness gate is correctly held OUT-OF-REPO as an unconfirmed external assumption (SBLK-R26) with
  no speculative implementation pulled in; no edit to `154-S` by Stage.

Gate outcome: **ADVISORY** — no P1 findings; the single actionable P2 (pre-verification ordering)
was applied inline. Base plan approval (attempt 2) and the prior amendment stand; this amendment is
additive and reopens no prior finding. Harvest adds one task (U18) to the existing feature/shipment.

<!-- plan-review-attempt: 2-amendment-3 -->

## Plan Review — Amendment (9-P1 BLOCKED remediation of fee43b0a)

dispatch_mode: multi-agent-dispatch
decision: ADVISORY
operator_authorization: approved

Trigger: local review of commit `fee43b0a` returned **BLOCKED (0 P0, 9 P1)**. This amendment
records the bounded Stage remediation over Stage-owned docs/backlog/stash artifacts only (no
application source/tests, no `154-S` edit, no PR). The nine P1 findings and their resolutions:

* **P1-1 (restore independent stash `7AA35A39`):** RESOLVED — the exact Ship-captured provenance
  line was recovered from the intact Ship branch and restored to `.backlogit/stash.jsonl`
  (`808E4323` correctly absent; `7AA35A39` NOT absorbed into `174-F`, kept as an independent
  informing-defect entry).
* **P1-2 (reconcile parked queued work):** RESOLVED — `164.001-T` moved to `blocked` so it cannot
  execute from `146-S` (history preserved via the existing supersedes link, not deleted);
  `164.002-T` re-pointed from `parked` to canonical `blocked` semantics with a new dependency on
  `174.001-T` and a `related_to 174-F` link. Backlog-native mutations, traceability preserved.
* **P1-3 (governed-refusal inventory completeness):** RESOLVED — plan U2b/U2b2/U2d and spec SBLK-R3
  now enumerate the exported core `MoveShipmentStatus` and the shipment `create`/`add`/`create_item`
  entry points in the choke-point/consumer inventory; U2b2 (task `174.024-T`) is the dedicated
  bulk/cascade/exported-core/create choke-point unit.
* **P1-4 (partial-failure semantics):** RESOLVED — new plan unit **U19** (task `174.027-T`) and spec
  **SBLK-R27** define authoritative event/frontmatter/index partial-failure durability (canonical
  ordering: durable append-only event → frontmatter → rebuildable index) with classified handling
  and failure-injection tests (fail-after-event / fail-after-frontmatter / fail-during-index).
* **P1-5 (active-slot source of truth):** RESOLVED — U5b (SBLK-R4/R6) now defines the active count
  as an authoritative scan of the contained shipment Markdown (or real CAS), NEVER the rebuildable
  index; fails closed on stale/missing index and on malformed/duplicate-active state.
* **P1-6 (bootstrap member disposition):** RESOLVED — U6 (SBLK-R9/R10) rewritten: `active→blocked`
  captures a governed member snapshot and returns active members (incl. `173.006-T`) to `queued`,
  genuinely freeing the active slot while preserving branch/checkpoint; unblock restores from the
  snapshot. Consequently the generic-move bootstrap **cannot** admit `155-S` (member stays active →
  P-001 contended). No policy override used.
* **P1-7 (invalid generic rollback):** RESOLVED — the invalid `move 154-S --status active` rollback
  is removed from deliberation §6 and spec SBLK-R24; the generic-move bootstrap is **PROHIBITED**
  until the governed seam (U2c) + normalizer (U18a/U18b) land; rollback is the governed bounded
  `unblock --to active`.
* **P1-8 (harness-exempt labeling):** RESOLVED — `174.021-T` carries the canonical `harness-exempt`
  label + a machine-readable `custom_fields.harness_exemption` block (class docs-only, exempt set
  `{U16}`, rationale, validation evidence); its AC expanded to cover R22–R26 docs.
* **P1-9 (topological reorder):** RESOLVED — `155-S` manifest reordered so `174.022-T` and
  `174.023-T` precede dependent `174.021-T`; dependency graph + topological execution order
  regenerated for the full unit set (U2b2/U13a/U13b/U18a/U18b/U19).

Safe corrections applied: R13 `--confirm` required on both unblock targets (U9); feature requirement
range widened to R1–R27; R19/R24–R26 reclassified in the layer table; actionable "park" wording
removed (remediation now "unblock one shipment to queued"); U17 scoped to blocked-related edges +
provisional-pending-upstream-ratification; fixture workspace (not live corpus) for the active-count
back-compat test (U15); one workspace-global active-slot lock clarified vs per-artifact locks (U5b);
oversized units split (U2b→U2b2, U13→U13a/U13b, U18→U18a/U18b); doctor severity/exit/MCP contract
defined (U13b); MCP parity decision for normalize resolved (U18b); U7b narrowed to status-filter +
rebuild-stability (redundant projection claim removed); impossible simultaneous blocked+terminal
scalar check removed (U13a). All new/split units are single-domain ≤2h with explicit acceptance.

personas (focused re-dispatch over the remediation delta):
* Correctness Reviewer (`claude-opus-4.8`) — VOTE: ADVISORY — member disposition (U6) correctly
  makes slot-freeing depend on requeueing active members, which is what invalidates the generic-move
  bootstrap; U19 closes the partial-failure durability gap with ordered writes + injection tests;
  U5b's Markdown-authoritative fail-closed source of truth removes the stale-index double-active
  hazard. No P1 residual.
* Architecture Strategist (`grok-4.6`) — VOTE: ADVISORY — the U2b2/U2d split keeps every governed
  choke point within one unit and the consumer-inventory proof now covers the exported-core and
  create/add entry points; U18a/U18b cleanly separate normalizer core from integration + MCP parity.
* Scope Boundary Auditor (`gemini-3.7-flash`) — VOTE: PASS — all edits stay within Stage-owned
  docs/backlog/stash; the reconciliation of `164.001-T`/`164.002-T` preserves history; each split
  task is ≤2h single-domain; no scope creep into `154-S`/source/tests.
* Security Reviewer (`gpt-5.6-terra`) — VOTE: ADVISORY — `--confirm` and `blocked_by` remain
  documented as non-authoritative; the authoritative gate stays the backlogit-side free-slot check
  under the workspace-global lock; governed-refusal inventory now closes the `MoveShipmentStatus` /
  create-path bypass. No auth-boundary regression.

Gate outcome: **ADVISORY** — no P1 findings remain after remediation; all nine controlling P1s and
the safe corrections are resolved within the Stage boundary. Prior approvals (attempt 2, amendment,
amendment-2) stand; this amendment is additive. Harvest adds four tasks (`174.024-T`/`174.025-T`/
`174.026-T`/`174.027-T`) to feature `174-F` / shipment `155-S`.


<!-- plan-review-attempt: 2-amendment-4 -->
## Plan Review — Amendment 4 (final task-propagation & rollout-circularity resolution)

dispatch_mode: multi-agent-dispatch
decision: ADVISORY
operator_authorization: approved

Scope of this amendment: final bounded remediation cycle resolving residual/new P1s labeled A–E,
editing ONLY Stage-owned docs/backlog/stash artifacts (no source/tests, no `154-S`, no PR).

Resolutions:
* **A (task propagation into executables):** U2b (174.003) narrowed to generic move/update +
  setArtifactStatus only (bulk/cascade → U2b2a/174.024; exported-core + create/add/create_item →
  U2b2b/174.028, new). U2b2a scoped to BulkUpdateStatus + cascadePersistedParentStatuses only.
  U2d (174.005) enumerates the full consumer inventory incl. MoveShipmentStatus/create/add/
  create_item and depends on all choke-point tasks (174.003/024/028). U6 (174.008) AC replaced with
  full member-status snapshot + active/in-flight→`queued` on block + exact restore on governed
  unblock. U9 (174.015) requires `--confirm` for BOTH queued and active unblock targets. U13a
  (174.018) narrowed to active-count + malformed-blocked only (remainder → U13b/174.025). U15
  (174.020) depends on U13a+U13b and uses an isolated fixture (not live corpus). U17 (174.022)
  scoped to blocked-related edges + backlogit-proposed provisional pending upstream ratification.
  U18a (174.023) normalizer-core only (no doctor/unblock/MCP → U18b/174.026) and reconstructs
  member snapshot / requeues active members for out-of-band blocked records, or refuses when
  reconstruction cannot be proven. U19a (174.027) split: durability/commit contract; U19b (174.029,
  new) owns failure-injection tests; both depend on U6 (174.008).
* **B (spec/decision consistency):** Goals state members are dispositioned (not unchanged). SBLK-R6
  distinguishes one workspace-global active-slot lock from per-artifact locks with a fixed
  acquisition order. Deliberation ranges normalized to R1–R27. SBLK-R27 event intent/commit
  protocol added: persist INTENT (+correlation id) → frontmatter+member writes → COMMITTED event,
  so an append-first event alone can never assert a completed transition; doctor/recovery reconcile
  INTENT-without-COMMITTED.
* **C (146-S reconciliation):** `164.001-T` removed from `146-S` membership; obsolete dependency
  `164.002-T → 164.001-T` removed; `164.002-T → 174.001-T` dependency and traceability retained.
* **D (rollout circularity):** replaced the prior blanket PROHIBITION with an explicitly one-time,
  operator-approved, **pre-governance** bootstrap runbook (SBLK-R24): snapshot members → manually
  move active member `173.006-T` to `queued` FIRST → generic `move 154-S --status blocked` → durable
  debt audit → autoharness topology pre-claim for `155-S`; bounded rollback valid ONLY before any
  other claim (else fail closed); governed U18a/U18b normalizer backfills canonical metadata/event
  before any unblock. Exceptional, removed after migration, NOT the shared contract, NOT executed by
  Stage.
* **E (docs & topology):** U16 (174.021) docs AC updated for all amendments. Dependency graph and
  topological execution order regenerated for the new unit set (U2b2a/U2b2b/U18a/U18b/U19a/U19b).
  `155-S` re-topologized to 30 items. Index synced.

Persona votes:
* Correctness Reviewer (`gpt-5.6-terra`) — VOTE: PASS — member snapshot/requeue/restore, intent/
  commit reconciliation, and fail-closed active-slot source of truth close the state-integrity gaps.
* Architecture Strategist (`claude-sonnet-5`) — VOTE: PASS — shared portable contract cleanly
  separated from backlogit-local provisional implementation; choke-point inventory complete.
* Scope Boundary Auditor (`gemini-3.7-flash`) — VOTE: PASS — every split task ≤2h single-domain; all
  edits within Stage-owned artifacts; `154-S`/source/tests untouched; history preserved.
* Security Reviewer (`gpt-5.6-terra`) — VOTE: ADVISORY — pre-governance bootstrap is an operator-
  owned exception with bounded rollback and fail-closed guard; `--confirm`/`blocked_by` remain
  non-authoritative; authoritative gate stays the workspace-global-locked free-slot scan.

Gate outcome: **ADVISORY** — no P1 findings remain after remediation; all A–E controlling P1s and
the safe corrections are resolved within the Stage boundary. Prior approvals (attempt 2, amendment,
amendment-2, amendment-3) stand; this amendment is additive. Harvest adds two tasks
(`174.028-T`/`174.029-T`) to feature `174-F` / shipment `155-S` (now 30 members).

<!-- plan-review-attempt: rev2 -->
## Plan Review — Revision 2 (concise replacement decomposition)

dispatch_mode: multi-agent-dispatch
decision: PASS
operator_authorization: approved

Scope: operator-directed full replacement of the oversized 29-unit decomposition with a concise
9-task, test-first, ≤2h set focused only on the shared portable `blocked`-shipment contract.
Prior units `174.001-T…174.029-T` superseded and parked at `blocked` (preserved, not deleted);
all prior `## Plan Review` records retained above as audit history. Edits limited to Stage-owned
docs/backlog/stash; no Go/source/tests, no `154-S` edit, no PR, no Ship invocation.

Resolutions carried by rev2:
* **Right-sized decomposition:** 9 tasks (`174.030-T…174.038-T`), each single-domain and ≤2h,
  RED-first (R1 harness gates every code task). Prior over-fragmentation removed.
* **Shared vs. local split** made authoritative in spec §0.1 (portable contract stable;
  writer/lock/SQLite/CLI-MCP local + replaceable), satisfying the autoharness-compat constraint.
* **Governed core** (R2 BlockShipment: lock-held snapshot→member→queued→persist→intent/commit→
  rollback; R3 UnblockShipment: confirm→free-slot→CAS/drift refusal→exact restore→clear/event).
* **Bypass closure** via ONE central governed writer/envelope (R4a) + single-seam call-site
  routing (R4b) covering generic move/update, MoveShipmentStatus, bulk/cascade,
  create-as-blocked/active, public WriteArtifactFile — no per-callsite sprawl.
* **Bootstrap source-of-truth** (deliberation §6.5, plan §0.2 U-BOOT): snapshot from intact Ship
  branch `dd9f01a1` not the staging projection; single-worktree switch; dedicated
  `chore/bootstrap-154` off post-merge main; no parallel worktrees; no Ship commit loss;
  machine-readable snapshot file consumed by R6 (no free-form memory evidence).
* **External topology VERIFIED** (spec §0.3, deliberation §6.4): current gate rejects `blocked`
  at `topology.py:553`; one-line upstream allowlist fix is external (not backlogit Go) →
  ratification item; interim audited operator-only `--force` override for the `155-S` window.
* **Crash recovery** (R6 startup reconciliation of durable intent) + subprocess crash/reopen
  integration tests (R7) + doctor torn-state detection (R8).
* **146-S reconciliation** retained (deliberation §6b): `164.001-T` removed from membership,
  `164.002-T→164.001-T` dependency removed, `164.002-T` re-points to canonical `blocked`.

Persona votes:
* Correctness Reviewer (`gpt-5.6-terra`) — PASS — lock-held block, CAS/drift-refused unblock,
  intent/commit reconciliation, and machine-readable-snapshot bootstrap close the integrity gaps.
* Architecture Strategist (`claude-sonnet-5`) — PASS — single governed writer/envelope seam
  removes bypass sprawl; shared/local split is clean and upstream-portable.
* Scope Boundary Auditor (`gemini-3.7-flash`) — PASS — 9 tasks, each ≤2h single-domain; no
  scope creep; superseded units preserved not deleted; `154-S`/Ship branch/source untouched.
* Security Reviewer (`gpt-5.6-terra`) — ADVISORY — bootstrap `--force` topology override is
  operator-only, audited, time-boxed to the migration window and removed after upstream ratifies
  `blocked`; authoritative gate remains the workspace-global-locked free-slot scan.

Gate outcome: **PASS** (Security advisory noted and operator-authorized). Residual P1 findings: 0.

<!-- plan-review-attempt: rev3 -->

## Plan Review — Revision 3 (branch-scoped source-of-truth; rollout-circularity removed)

* **dispatch_mode:** multi-agent-dispatch
* **decision:** PASS
* **operator_authorization:** approved
* **plan-review-attempt:** rev3 (prior records — attempt 1, 2, amendments 1–4, rev2 — retained above as audit)
* **scope:** feature `174-F` / shipment `155-S`; 12 replacement tasks `174.039-T…174.050-T`
  (rev2 `174.030-T…174.038-T` superseded → `blocked`, preserved).

### P1 resolutions verified

1. **10–12 tasks, explicit RED-before-GREEN.** 12 tasks; three RED harnesses (`174.039/040/041`)
   have no prerequisites and every implementation task depends on its RED harness — verified in the
   dependency graph (`174.042→039,040`; `174.043→039,042`; `174.047→041,…`; `174.048→041,047`).
2. **Durable INTENT + complete preimage persist before any mutation; global lock held through
   intent→disposition→persist→commit/compensation.** Encoded in R5 (`174.043-T`) AC; claim and
   unblock-to-active share the same workspace-global lock (R6 `174.044-T`).
3. **Target-aware unblock.** to-queued preserves snapshot/leaves members queued; to-active exact
   restore under lock; never active members under a queued shipment (R6 `174.044-T`).
4. **Govern public `WriteArtifactFile` boundary + private lower writer; create-active restricted.**
   Only `ClaimShipment`/unblock-to-active create an active shipment under the global lock (R4
   `174.042-T` + R6 `174.044-T`); all generic activation/bypass paths rewired via R7 `174.045-T`.
5. **Member CAS/drift refusal.** Member mutations guarded while parent blocked; restore refuses on
   drift (R5/R6).
6. **Machine-readable snapshot schema is normalizer input (no free-form memory).** R9 `174.047-T`.
7. **Startup/governed recovery under the same locks from durable intent/preimage; subprocess
   kill+reopen tests.** R9 `174.047-T` (recovery) + R10 `174.048-T` (GREEN subprocess tests),
   RED-gated by R3 `174.041-T`.
8. **MCP normalizer parity + CLI/MCP block/unblock/status parity.** R8 `174.046-T`, R9 `174.047-T`.
9. **Doctor split from docs.** R11 `174.049-T` (doctor production) vs. R12 `174.050-T` (docs).
10. **`164.002-T` re-pointed** from superseded `174.001-T` to final live task `174.050-T` (applied
    via backlog-native `dep` mutations; traceability preserved).
11. **Rollout circularity removed; generic-move bootstrap and `155` topology `--force` deleted from
    authoritative sections** (§0.2/§0.3, deliberation §6); stale live-corpus/pre-block/pre-governance
    text removed from §0; historical sections retained below as superseded audit.

### Persona votes

* **Correctness:** PASS — RED-before-GREEN topology sound; intent+preimage-before-mutation and
  global-lock invariants explicit; target-aware unblock well-defined.
* **Architecture:** PASS — shared portable contract cleanly separated from backlogit-local writer
  implementation; branch-scoped source-of-truth eliminates the circular dependency at the root.
* **Scope/Maintainability:** PASS — 12 tasks each ≤2h, single-domain, RED-gated; no
  one-task-per-callsite sprawl (R7 split-if->4-functions guard).
* **Constitution/Safety:** PASS — no source/tests/154-S mutated by Stage; `155-S` ships without any
  topology override; corrective-shipment override is audited, per-phase, and conditional.

**Verdict: PASS** — cleared for harvest/manifest; residual P1 = 0.

## Plan Review — current-HEAD remediation cycle (2026-09-15)

dispatch_mode: single-agent-declared-degradation
decision: PASS

Bounded current-HEAD remediation of five P1 findings from final code review, applied to Stage-owned
planning/backlog artifacts only (no source/tests, no `154-S`, no shipment claim, no PR). `155-S`
remained `queued` throughout; append-only review history and blocked/superseded tasks preserved.

* **F1 — RED deliverable contracts.** Added canonical machine-readable `red-deliverable-contract`
  blocks to `174.039-T` (R1), `174.040-T` (R2), `174.041-T` (R3). Keys/order exactly
  `red_deliverable, red_deliverable_reason, red_selector_command, green_maker_tasks,
  green_maker_closes_wave`; executable selectors anchored to `^TestUR<n>_ ./internal/core`
  (`WriteArtifactFile`/`MoveShipmentStatus`/`isValidShipmentTransition` package). Green-maker lists
  and closing waves derived from the post-split dependency graph: R1→{043,044}@4, R2→{042,045,051}@5,
  R3→{047,048}@6. Validated with the actual `scripts/wave-scheduler-sim.ps1` parser functions
  (`Read-RedDeliverableContract`, `Test-TaskScopedCommandShape`) — 3/3 parse clean, selector shape OK.
* **F2 — Governed block commit completeness.** plan §0.2 / decision §6.2 / spec §0.3 now enumerate
  the full governed output (`154-S` record + changed member artifacts + intent/preimage/snapshot
  recovery state + per-item event logs); the "`154-S` + snapshot only" wording is removed.
* **F3 — 164.002-T retired.** Obsolete parked-era forward-repair set `blocked`; obsolete
  `164.002-T → 174.050-T` dependency removed; superseded rationale recorded in the task body and
  plan §0.1 / spec §0.2 / decision §6b–§6c.
* **F4 — Release-unit ordering.** F3's dependency removal decouples `146-S` from `155-S`; no live
  member of `146-S` depends on `155-S`, so no shipment-level `blocks` edge is added (consistent with
  the retire choice).
* **F5 — 174.045-T split.** R7 → R7a (`174.045-T`, bypass WRITE paths) + R7b (`174.051-T`,
  create-as-active/activation refusal); each ≤5 functions, ~2h, RED-before-GREEN preserved.
  `174.051-T` created under `174-F` with deps `[174.040-T,174.042-T,174.044-T]` and added to the
  `155-S` manifest (now 14 members) after `174.045-T`; §0.1 table, dependency graph, and topological
  order updated in plan and spec.

Validation evidence: `backlogit sync` parse_failures=0 (1525 artifacts); `backlogit doctor
--check-orphans --check-duplicates` — 23 pre-existing findings, none on touched artifacts;
`wave-scheduler-sim.ps1 -VerifyAgainstQueue` WAVE_SIM_OK 186/186; RED contract parser 3/3 clean.

**Verdict: PASS** — residual P0 = 0, residual P1 = 0. Ready for Ship to claim `155-S`.
