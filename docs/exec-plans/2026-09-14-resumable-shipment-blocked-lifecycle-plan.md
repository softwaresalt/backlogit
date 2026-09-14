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
**Source spec:** `docs/product-specs/2026-09-14-resumable-shipment-blocked-lifecycle-status.md` (SBLK-R1…R26).
**Source deliberation:** `docs/decisions/2026-09-14-resumable-shipment-blocked-lifecycle-status-deliberation.md` (Option A, decided).
**Intake:** stash `808E4323`. **Informing defect (out of scope):** stash `7AA35A39`.
**Supersedes:** S12 parked-state unit `164.001-T` (see deliberation §4/§6).

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
  lock (U5b), SQLite projection (U7b), doctor checks (U13), CLI/MCP surface shapes (U9/U11). These
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
* **U2b (SBLK-R3) — Close ALL ungoverned status-write choke points.** Make EVERY status-write
  path that bypasses `isValidShipmentTransition` refuse a shipment transition whose
  `newStatus=="blocked"` (block direction) OR `oldStatus=="blocked"` (unblock direction),
  gated on `ArtifactType=="shipment"`, with NO forgeable exemption flag: the generic
  `move_item`/`update_item` path, **`BulkUpdateStatus`** (today only special-cases
  archived/shipped), **`setArtifactStatus`**, and **`cascadePersistedParentStatuses`**. Also
  gate the parent cascade so it can never move a shipment out of `blocked`. The governed
  `BlockShipment`/`UnblockShipment` functions (U2c) are exempt BY CONSTRUCTION (separate
  functions), never via a flag. Acceptance: each named choke point refuses both directions
  regardless of any flag and with the formal gate OFF; member `blocked` items are unaffected.
  Depends on U2a.
* **U2d (SBLK-R3) — Consumer-inventory proof-of-completeness.** Enumerate every
  `ShipmentStatus` consumer (guards, queue/ready-work, index projection, dependency
  eligibility, ship/abandon/reconcile, archive, reporting, event consumers) and assert each
  routes through the central non-terminal/terminal classifier or fails closed on the
  unrecognized value — "no consumer default-allows blocked" is a testable acceptance. This is
  the S12 taxonomy-integration P1 closure, split from U2b to keep each unit within the 2-hour
  envelope. Acceptance: consumer-inventory test enumerates the full consumer set and every
  entry classifies-or-fails-closed. Depends on U2b.
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
  Source-shape harness for the new signatures. Depends on U2b.
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
* **U6 (SBLK-R9, R10) — Evidence & member preservation on block.** `active → blocked` preserves
  member task statuses verbatim (no cascade to member `blocked`, no `return-blocked`
  invocation) and leaves branch association and `resume_checkpoint_ref` intact. All `blocked_*`
  predicates gate on `ArtifactType=="shipment"`. Acceptance: members unchanged after block;
  checkpoint ref preserved; a member `blocked` (return-blocked) satisfies NO shipment-blocked
  predicate and vice versa. Depends on U2c.

### SE-B — Active-slot & concurrency

* **U5b (SBLK-R4, R6) — Workspace-global active-slot serialization.** Provide a single
  workspace-scoped serialization (a global claim lock keyed on the workspace root under
  `.locks/`, or an atomic active-slot compare-and-swap) that guards the active-slot
  check-then-set. This is NOT the per-artifact mutation lock: two concurrent claims of
  DIFFERENT shipments must contend on the same key so both cannot read "no active" and both
  activate. Acceptance: two concurrent `ClaimShipment` calls on different shipments serialize;
  at most one activates. Depends on U1.
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
* **U7b (SBLK-R11) — Index projection + status filter.** The SQLite index carries the
  `blocked` value transactionally (rebuildable), `shipment list --status blocked` returns
  them, and an index rebuild after `sync` retains the value. Acceptance: status filter returns
  blocked; row count stable across a mid-walk-cancel rehydration test. Depends on U1.
* **U10 (SBLK-R12) — Dependency eligibility.** A `blocked` shipment as a dependency remains
  execution-blocking (NOT added to the `IsNoLongerBlockingStatus` set). Acceptance: a dependent
  stays gated while its dependency shipment is `blocked`; unblock+terminal releases it. Depends on U1.

### SE-D — Operator surfaces (CLI / MCP)

* **U9 (SBLK-R13) — CLI block/unblock.** `backlogit shipment block <id> --reason <text>
  [--resume-checkpoint <ref>]` and `backlogit shipment unblock <id> --to queued|active
  [--confirm]`, routing through the governed seam; naming distinct from `return-blocked`.
  Acceptance: both verbs route the governed path; `--reason` required for block; unblock-to-active
  honors the readiness gate (U12); help/usage updated. Depends on U2c, U3, U4, U5, U12.
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
  SBLK-R25) with a remediation message pointing at the U18 normalizer. Depends on U2c, U5, U5b.
* **U13 (SBLK-R15, R16) — Doctor integrity checks.** `backlogit doctor`, over
  `ArtifactType=="shipment"` only, verifies: (a) ≤1 active shipment; (b) every blocked shipment
  has non-empty `blocked_reason` + valid `blocked_at`; (c) no shipment is blocked AND terminal
  — justified as EXTERNAL-CORRUPTION detection (the governed matrix makes it unreachable in-band,
  but doctor's role is to catch out-of-band edits); (d) unrecognized shipment status fails closed.
  Reads `blocked_*` from the Markdown source (the index does not project them). Gating on
  `ArtifactType=="shipment"` prevents false positives on member `return-blocked` items (which
  correctly have no `blocked_reason`). Acceptance (≤4 scenarios): flags a malformed blocked
  shipment; flags a two-active condition; ignores a member `blocked` item; fails closed on an
  unknown status. Depends on U1, U5.
* **U14 (SBLK-R17) — Ship/abandon guard integration.** `shipment ship` refuses a `blocked`
  shipment; the guard prevents `blocked → shipped/archived/abandoned`; `reconcile-shipped`
  (archived-only) is unaffected; terminal abandonment path is `blocked → active → abandoned`.
  Acceptance: ship of blocked refused with clear error; reconcile path unchanged. Depends on U2a.

### SE-F — Back-compat & docs

* **U15 (SBLK-R18) — Back-compat (no fail-open marker).** The additive enum requires no data
  migration. A pre-existing multi-active condition is reported by doctor (U13a) as a HARD finding
  with explicit remediation (unblock/park one shipment) — NOT a resettable "first-run" warning
  and never a fail-open flag; claim enforcement (U5) is forward-only, so it refuses creating a new
  second active without retroactively mutating existing state. Acceptance: `sync` + `doctor` are
  clean over the existing corpus (exactly one active, `154-S`); an injected second active is a hard
  doctor finding with remediation text. Depends on U13.
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

  Also documents the **one-time `154-S` bootstrap-migration runbook** (SBLK-R24/R25/R26): the
  temporary generic-move compatibility seam, pre/post verification, status-only rollback, the
  U18 debt-normalization step required before any unblock, and the autoharness topology-gate
  caveat (bootstrap frees only backlogit's active-slot scan; the external gate must be verified
  separately). Additional acceptance: bootstrap runbook + autoharness-gate caveat present.
  (Also depends on U18.)
* **U17 (SBLK-R20, R21 — [shared] contract conformance) — Portable-contract conformance test.**
  A test that PINS the shared cross-repo contract so backlogit-local work cannot silently diverge:
  asserts the status token is exactly the string `blocked`; the shipment transition set is exactly
  the three shared edges; the audit field names are exactly
  `blocked_reason`/`blocked_at`/`blocked_by`/`resume_checkpoint_ref`; the event name is
  `shipment_status_changed`; and that NO conflicting synonym token (`parked`, `paused`, `on-hold`)
  is introduced for the same concept. Domain: tests. Acceptance: the conformance test enumerates
  the shared tokens/edges/field-names and fails if any is renamed, removed, or a synonym is added.
  Depends on U1, U2a, U3.
* **U18 (SBLK-R24, R25 — bootstrap-migration debt discharge) — Normalize bootstrap-migrated
  blocked shipments.** Provide a governed normalizer (`backlogit shipment normalize-blocked <id>
  --reason <text> [--resume-checkpoint <ref>]`, routed through the U2c seam) that backfills the
  governed `blocked_reason`/`blocked_at`/`blocked_by`/`resume_checkpoint_ref` and emits the
  governed `shipment_status_changed` event for a shipment whose status token was set to `blocked`
  by the temporary generic-move bootstrap seam (SBLK-R24) and therefore lacks governed metadata.
  Idempotent: a no-op on an already-governed blocked shipment. Also: `doctor` (U13) flags a
  status=`blocked` shipment with no `blocked_reason` as a **hard finding** naming this normalizer;
  and the unblock readiness gate (U12) refuses unblock until normalization completes. Domain: code.
  Acceptance: normalizer backfills all four fields + emits the event; is idempotent; a
  bootstrap-migrated shipment cannot be unblocked until normalized; doctor flags the un-normalized
  case. Depends on U2c, U3, U12, U13.

## Dependency graph

```
U1 ──┬─► U2a ─► U2b ─┬─► U2c ─► U3 ─► U4
     │              │         └─► U6
     │              └─► U2d
     │                 (governed seam U2c consumed by U9, U11)
     ├─► U5b ─► U5 ─► U8
     ├─► U7a
     ├─► U7b
     ├─► U10
     └─► U2a ─► U14
U5b ─► U12 ; U5 ─► U12
U2c ─► U12
{U2c,U3,U4,U5,U12} ─► U9 ; {U2c,U3,U4,U5,U12} ─► U11
{U1,U2a,U3} ─► U17
{U2c,U3,U12,U13} ─► U18
{U1,U5} ─► U13 ─► U15
{U9,U14,U15,U17,U18} ─► U16
```

Execution order (topological): U1 → {U2a, U5b, U7a, U7b, U10} → {U2b, U5, U14} →
{U2c, U2d, U8, U13} → {U3, U12, U15} → {U4, U6, U18} → {U9, U11, U17} → U16.

## Runtime verification & closure

Runtime surfaces: shipment lifecycle core, claim path, queue/ready-work selection, CLI,
MCP, doctor. Verification: the transition guard matrix, single-active claim refusal,
metadata set/clear at every choke point, queue/index exclusion, dependency gating, doctor
checks, and CLI/MCP parity are each covered by unit tests; a `backlogit sync` + `doctor`
pass over the existing corpus proves back-compat. Closure: operator docs + rollback runbook
(U16); blocked-status audit metadata schema documented.

## Rollback / verification (feature-level)

The feature is additive and code-only. Rollback reverts the enum, guards, seam functions,
and surfaces; no destructive data migration occurs. Pre-rollback runbook: `shipment list
--status blocked` then unblock each to `queued` so no artifact holds a status value the
prior binary cannot recognize.

## Plan Hardening

| ProposedAction | ActionRisk | Mitigation |
|---|---|---|
| New shipment lifecycle status + transition edges | Medium — taxonomy/gate integration (the exact S12 parked P1) | Single canonical `blocked` (Option A); closed guard matrix; **consumer-inventory proof** that no `ShipmentStatus` consumer default-allows blocked (U2b); fail-closed on unrecognized; doctor asserts well-formedness (U13) |
| Governed block/unblock transitions | Medium — ungoverned bypass via generic move/update, **BulkUpdateStatus, setArtifactStatus, cascade** | Unconditional refusal at EVERY status-write choke point keyed on old/new status=='blocked' AND ArtifactType=='shipment' (U2b); parent cascade gated so it cannot move a shipment out of blocked; governed `BlockShipment`/`UnblockShipment` exempt by construction; no forgeable flag (U2c) |
| Single-active enforcement (operator-requested; P-001 was convention-only) | Medium — race could double-activate; could surface pre-existing multi-active | Workspace-GLOBAL active-slot serialization / CAS, not the per-artifact lock (U5b); atomic claim with member-set rollback (U5); pre-existing multi-active is a HARD doctor finding with remediation, not a fail-open marker (U15) |
| Blocked audit metadata lifecycle & non-repudiation | Medium — stale `blocked_*` after hook-bypassing `blocked→queued`; audit erased on clear | Single seam-owned clear helper (U4); durable `shipment_status_changed` event with actor+reason+resume_ref emitted on BOTH block and unblock BEFORE clearing (U2c/U3); `blocked_by` documented as advisory, events authoritative |
| Member vs shipment `blocked` conflation | Medium — shared "blocked" string on shared status field | Distinct `ShipmentStatus` Go type (U1); every blocked_* predicate/doctor/guard gates on ArtifactType=='shipment' (U6/U13); cross-axis negative test |
| Atomic writes across MD + SQLite + JSONL | Medium — torn state / stale index on the new value | Transactional index rebuild with mid-walk-cancel test (U7b); commit-then-surface durability; shared EventWriter for event append (U2c) |
| Unblock readiness is not an authorization boundary | Medium — self-asserted `--confirm`/TTY is spoofable (S12 precedent) | `--confirm` documented as NON-authoritative; the authoritative gate is the backlogit-side free-slot check under U5b (U12); MCP rejects a bare client boolean as sufficient and records actor trust level (U11) |
| Downgrade with blocked shipments present | Low — unknown status to old binary | Additive enum; documented pre-rollback unblock runbook (U16) |
| Cross-repo contract divergence (backlogit-local drift from the shared autoharness contract) | Medium — a local synonym token, extra edge, or renamed field would break the eventual autoharness supersession | Explicit shared-vs-local boundary (spec §2.5); portable-contract conformance test pins token/edges/field-names and forbids synonyms (U17); non-destructive additive-only migration with a documented 1:1 mapping note (U16, SBLK-R22); external informing reference recorded without inventing an ID (SBLK-R23) |
| One-time `154-S` bootstrap via generic `move --status blocked` before the governed seam exists | Medium — degraded `blocked` shipment lacks governed audit metadata/event; could be mistaken for the final contract; external gate may not honor `blocked` | Bootstrap is a **temporary compatibility seam only**, status-only + reversible, never presented as the contract (SBLK-R24); missing metadata is tracked **debt** that U18 must normalize and U12 refuses unblock until discharged (SBLK-R25); doctor flags the un-normalized case (U13); pre/post verification + rollback documented (deliberation §6); autoharness topology-gate treatment of `blocked` flagged as an unconfirmed external assumption requiring separate verification (SBLK-R26) |

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
