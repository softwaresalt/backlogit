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
> superseded; the shipped scope is the concise **16-task RED-before-GREEN set** (`174.039-T…174.050-T`
> plus the R7b split task `174.051-T`, the core stateful-seam split tasks `174.052-T`
> (declaration-only compile-green) and `174.053-T` (core-lifecycle behavior RED), and the R4g
> pre-edge guard task `174.054-T` (generic `MoveShipmentStatus` block/unblock refusal, landed before
> the R5/R6 transition-table edges); 17 `155-S` members incl. `174-F`) below. Where §0 and
> a later section conflict, §0 governs. **rev3 removes the rollout circularity, the generic-move
> bootstrap, and the `155-S` topology `--force`.**

### 0.1 Concise task set (feature `174-F` / shipment `155-S`) — 16 tasks (17 `155-S` members incl. `174-F`), RED-before-GREEN, ≤2h

The rev1/rev2 tasks `174.001-T…174.038-T` are **superseded and archived** (terminal,
`archived_status: blocked`, history/events preserved). Under the flat-manifest rule (§0.4, spec
§0.5 / SBLK-R28) they are OUTSIDE `155-S` scope **because their IDs were never explicit members of
the `155-S` manifest**, not because they were archived — archival is not required to descope them.
The core `BlockShipment`/`UnblockShipment` stateful seam follows **source-shape RED →
declaration-only compile-green → behavior RED → implementation** (R1 split into R1s/Rd/R1b).
Replacement:

| Task | Req | Title | Domain | Depends on |
|---|---|---|---|---|
| `174.039-T` | R1s | Core-lifecycle SOURCE-SHAPE RED harness (go/ast: `ShipmentBlocked` + EXACT `BlockShipment`/`UnblockShipment` signatures + `BlockOptions`/`UnblockOptions` + `blerrors.ErrNotImplemented` + `blerrors.ErrShipmentBlockedRequiresEnvelope` sentinels; no transition-table) | tests | — |
| `174.052-T` | Rd | Core-lifecycle declaration-only stubs (compile-green): `ShipmentBlocked` const + `BlockOptions`/`UnblockOptions` + block/unblock stub signatures returning `blerrors.ErrNotImplemented`; declares BOTH the `blerrors.ErrNotImplemented` and `blerrors.ErrShipmentBlockedRequiresEnvelope` sentinels (no transition-table) | code | R1s |
| `174.053-T` | R1b | Core-lifecycle BEHAVIOR RED harness (transitions/metadata/intent+preimage/disposition/target-aware unblock) | tests | Rd |
| `174.040-T` | R2 | Writer/bypass BEHAVIOR RED harness (writer boundary, generic move/update, MoveShipmentStatus, bulk/cascade, create-as-active) | tests | Rd |
| `174.041-T` | R3 | Crash/reopen BEHAVIOR RED harness (durable intent/preimage recovery; subprocess kill+reopen) | tests | Rd |
| `174.042-T` | R4 | Governed writer core + envelope + public `WriteArtifactFile` boundary | code | R2, Rd |
| `174.054-T` | R4g | Pre-edge generic `MoveShipmentStatus` block/unblock top-level refusal (`blerrors.ErrShipmentBlockedRequiresEnvelope`, placed above the transition-table check; lands before any R5/R6 edge) | code | R2, Rd |
| `174.043-T` | R5 | Core `BlockShipment` (global lock across intent→preimage→disposition→persist→commit/compensation; member guard; enable governed `active→blocked` transition, generic move still refused via R4g) | code | R1b, Rd, R4, R4g |
| `174.044-T` | R6 | `UnblockShipment` + `Claim` under shared global lock (target-aware restore, CAS/drift refusal, create-active restriction; enable governed `blocked→{queued,active}` transitions, generic move still refused via R4g) | code | R1b, Rd, R5, R4g |
| `174.045-T` | R7a | Route REMAINING bypass WRITE call-sites (generic move/update, bulk/cascade; MoveShipmentStatus writer delegation) through governed writer — MoveShipmentStatus block/unblock refusal now owned by R4g | code | R2, R4, R6 |
| `174.051-T` | R7b | Guard create-as-active + all generic activation paths (refuse activation outside claim/unblock) | code | R2, R4, R6 |
| `174.046-T` | R8 | CLI + MCP parity for block/unblock + read/list status | code | R5, R6 |
| `174.047-T` | R9 | Recovery + normalizer + machine-readable snapshot schema (durable-intent recovery under locks; MCP parity) | code | R3, R5, R6 |
| `174.048-T` | R10 | Subprocess crash/reopen GREEN tests | tests | R3, R9 |
| `174.049-T` | R11 | Doctor production checks (active-count, malformed-blocked, torn-intent; severity/exit/MCP; isolated fixture) | code | R5, R6, R9 |
| `174.050-T` | R12 | Operator docs + branch-scoped bootstrap runbook + topology note | docs | R8, R9, R11 |

```
R1s(039) ─► Rd(052) ─┬─► R1b(053) ─────────────┐
                     ├─► R2(040) ─► R4(042) ──┐ │
                     │            └► R4g(054) ─┤ │   (R4g refusal lands @W4, before edges)
                     └─► R3(041)              ├─►R5(043) ─► R6(044) ─┬─► R8(046) ─┐
                                              │  (R5/R6 need R1b,Rd,R4g) ├─► R9(047) ─┼─► R11(049) ─► R12(050)
                                              │                      │            │            ▲
                                       R7a(045)+R7b(051) need R2,R4,R6             └─► R10(048)─┘
```

Waves (dependency layers): W1 `039`; W2 `052`; W3 `053`,`040`,`041`; W4 `042`,`054`; W5 `043`;
W6 `044`; W7 `045`,`051`,`046`,`047`; W8 `048`,`049`; W9 `050` — 9 waves. RED-deliverable closing
waves: R1s (`174.039-T`)@2, R1b (`174.053-T`)@6, R2 (`174.040-T`)@7, R3 (`174.041-T`)@8. The R4g
guard (`174.054-T`)@W4 is a green-maker for R2 (the `MoveShipmentStatus` block/unblock-refusal
portion) and lands strictly BEFORE R5/R6 enable the `isValidShipmentTransition` edges, so no
intermediate wave exposes an ungoverned generic block/unblock.

Topological order (parent-first):
`174-F → 174.039 → 174.052 → 174.053 → 174.040 → 174.041 → 174.042 → 174.054 → 174.043 → 174.044 →
174.045 → 174.051 → 174.046 → 174.047 → 174.048 → 174.049 → 174.050`.

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

### 0.4 Scope-correction task addendum (flat-manifest shipment scope — rev4, 2026-09-15)

Delivers spec §0.5 / **SBLK-R28** (flat explicit shipment membership) as part of `155-S`, RED-first,
layered on the finalized `blocked`-lifecycle seam. Root cause (direct code inspection): the shipment
lifecycle derives release scope by expanding a listed feature into all descendants —
`releaseScopeItemIDs` (`internal/core/shipment_lifecycle.go:1142`) → `descendantItems`
(`:1219`, BFS on `ParentID`, `IncludeArchived:true`) — and `compositionMemberIDs`
(`internal/core/size_composition.go:291`) expands feature→children for the member projection, so
`155-S`'s 17-entry explicit manifest projects to 54 members (pulling in archived `174.001-T…038-T`).
A second and third independent expansion survive even a flat `releaseScope`: (a) the ship/closure cleanup
re-expands a member feature's descendants directly via `descendantItems` in `collectArchiveCandidateIDs`
(`:800`, archives unlisted terminal descendants **and appends the feature's linked deliberations** via
`linkedDeliberationIDs`) and `returnUnreleasedFeatureItems` (`:735`, returns-to-backlog and
`clearParentID`-orphans unlisted non-terminal descendants) — flattened by `174.058-T`; and (b) the
rollback/snapshot path (`:603-612`) independently re-expands the artifact lock/snapshot/restore set
(`rollbackIDs`/`snapshotShipArtifacts`) with covering-feature ancestors and every `descendantItems` —
flattened by `174.057-T` (the covering-feature status-rollup revert via `nonMemberFeatureSnapshots` is a
separate mechanism and is preserved). The flat rule makes release scope equal the explicit
`custom_fields.items` manifest.

| Task | Role | Title | Domain | Depends on |
|---|---|---|---|---|
| `174.055-T` | SCOPE-RED-A | Behavior RED harness: release scope == flat explicit manifest (task-only manifest scopes to its items; listed feature does NOT expand to unlisted descendants; explicitly-listed descendant IS in scope; asserts over the retained `releaseScopeItemIDs` seam) — <4 scenarios | tests | `174.044-T` |
| `174.059-T` | SCOPE-RED-C | Behavior RED harness: rollback lock/snapshot/restore set == `{shipment} ∪ flat manifest` (unlisted ancestor feature + unlisted descendant absent and non-restorable; pins the independent `:603-612` expansion) — <4 scenarios | tests | `174.044-T` |
| `174.056-T` | SCOPE-RED-B | Behavior RED harness (regression): unlisted **blocked** descendant excluded from projection; unlisted (incl. **archived**) descendants excluded; **feature-only** manifest scopes to `{feature}` and its ship/closure frees the active slot — <4 scenarios | tests | `174.044-T` |
| `174.060-T` | SCOPE-RED-D | Behavior RED harness: `collectArchiveCandidateIDs` excludes an unlisted **terminal-but-not-archived** (`done`/`accepted`) descendant **and** an unlisted **linked deliberation** from `ArchivedIDs`; no archived artifact restored for scope — <4 scenarios | tests | `174.044-T` |
| `174.057-T` | SCOPE-IMPL-1 | Flatten release-scope **derivation**: `releaseScopeItemIDs` (`:1142`) flattened **in place** (seam retained, not bypassed); evidence set/`validateMemberGateEvidence` + `completeReleaseScope` flatten transitively; **AND** neutralize the independent rollback/snapshot expansion at `:603-612` (lock/snapshot/restore set == `{shipment} ∪ flat manifest`; non-member feature rollup revert preserved); parent-first preserved as ordering — makes `174.055-T` + `174.059-T` green | code | `174.055-T`, `174.059-T`, `174.045-T`, `174.051-T` |
| `174.058-T` | SCOPE-IMPL-2 | Flatten **projection** + **ship/closure feature cleanup**: `compositionMemberIDs` (listed task members, feature excluded → `155-S` **22**) + the independent re-expansions in `collectArchiveCandidateIDs` (`:800`, unlisted **terminal-but-not-archived** descendants **and linked deliberations** left untouched) and `returnUnreleasedFeatureItems` (`:735`, no archival/orphan of unlisted descendants); feature-only ship/closure frees the active slot; update coupled legacy tests — makes `174.056-T` + `174.060-T` green | code | `174.056-T`, `174.060-T`, `174.057-T` |

```
174.044(R6) ─┬─► 174.055(RED-A) ─┬─► 174.057(IMPL-1) ─► 174.058(IMPL-2)
             ├─► 174.059(RED-C) ─┘
             ├─► 174.056(RED-B) ─┬─────────────────────► 174.058
             └─► 174.060(RED-D) ─┘
174.045(R7a)+174.051(R7b) ─► 174.057
```

**Integrated waves (still 9):** `174.055`,`174.059`,`174.056`,`174.060`@**W7**; `174.057`@**W8**;
`174.058`@**W9**. RED-deliverable closing waves add: SCOPE-RED-A (`174.055-T`) + SCOPE-RED-C
(`174.059-T`) closed by `174.057-T`@W8; SCOPE-RED-B (`174.056-T`) + SCOPE-RED-D (`174.060-T`) closed
by `174.058-T`@W9. Sub-graph acyclic; no existing wave changes.

**`155-S` manifest → 22 tasks (23 members incl. `174-F`)**; appended in dependency order
`… 174.050 → 174.055 → 174.059 → 174.056 → 174.060 → 174.057 → 174.058`. **No public API introduced**
(internal behavior change only; `NormalizeShipmentItems` unchanged). External topology/wave gate is independent
(reads neither seam); **no `--force` authorized or applied**.

> **rev6 update (2026-09-16, branch `chore/stage-155-flat-shipment-scope`) — SUPERSEDES the two lines
> above for counts/waves.** The rev4/rev5 addendum PRESERVED the non-member covering-feature
> status-rollup revert (`nonMemberFeatureSnapshots`/`restoreRolledUpNonMemberFeatures`). That preserved
> path is a **P1 concurrency defect** once `174.057-T` drops unlisted ancestors from the outer artifact
> lock (`:603-612`): the ancestor is snapshotted and later restored **without a lock**, so a mutation
> to it between snapshot and restore is overwritten (lost update). Per SBLK-R28 (§0.5 point 8) shipment
> ship/rollback must not mutate/snapshot/restore/lock any artifact absent from the explicit manifest;
> the fix **eliminates** the non-member ancestor rollup entirely rather than re-locking it (re-locking
> would re-introduce the hierarchy expansion `174.057-T` removed). **+2 tasks (scope-correction delta
> now +8); concurrency fix-cycle-3 adds `174.063-T` (delta now +9):** `174.061-T` (SCOPE-RED-E,
> `^TestUNonMemberRollupSafe_`, dep `174.044-T`) and `174.063-T` (SCOPE-RED-F,
> `^TestUPostShipHookCascadeGlobal_`, dep `174.044-T`) precede
> `174.062-T` (SCOPE-IMPL-3, dep `174.061-T,174.063-T,174.057-T,174.058-T`) which bounds the
> `completeReleaseScope`→`cascadePersistedParentStatuses` rollup to explicit members **via a separate
> in-closure boundary context** (never the escaping outer `ctx` that reaches the post-ship `FirePost`
> hook at `:704`) and deletes `snapshotNonMemberFeatureStatuses`/`restoreRolledUpNonMemberFeatures`
> (+ their `ShipShipment` and `classifyShippedEventAppendFailure` call-sites). Explicitly-listed
> feature members keep governed `done` handling (the `:648` member-guarded path). `174.057-T`/`174.059-T`
> updated: they no longer claim the status-rollup revert is "preserved" — they leave it in place pending
> `174.062-T`, and their transactional-set terminology now reads `{shipment control record ID} ∪ explicit
> manifest IDs` (shipment record = sole non-member exemption), never "manifest only".
>
> ```
> 174.044(R6) ─┬─► 174.061(SCOPE-RED-E) ─┐
>              └─► 174.063(SCOPE-RED-F) ─┤
> 174.057(IMPL-1),174.058(IMPL-2) ───────┴─► 174.062(SCOPE-IMPL-3)
> ```
>
> **Waves 9 → 10:** `174.061`@**W7** (dep `174.044`@W6), `174.063`@**W7** (dep `174.044`@W6);
> `174.062`@**W10** (deps `174.061`@W7, `174.063`@W7, `174.057`@W8, `174.058`@W9). RED-deliverable
> closing wave adds: SCOPE-RED-E (`174.061-T`) and SCOPE-RED-F (`174.063-T`) closed by
> `174.062-T`@**W10**. **`155-S` manifest → 25 tasks (26 members incl. `174-F`)**; appended
> `… 174.058 → 174.061 → 174.063 → 174.062`. No public/exported API introduced (unexported context boundary key
> only). Topology/wave gate still independent; no `--force`.

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
  lock (U5b), SQLite projection (U7b), doctor checks (U13a/U13b), CLI/MCP surface shapes (U9/U11),
  **and the exact Go API shape — `BlockShipment(ctx, ws, shipmentID string, opts BlockOptions) (*models.Artifact, error)` / `UnblockShipment(ctx, ws, shipmentID string, opts UnblockOptions) (*models.Artifact, error)`, the `BlockOptions`/`UnblockOptions` structs, the `blerrors.ErrNotImplemented` declaration-only sentinel, and the `blerrors.ErrShipmentBlockedRequiresEnvelope` generic-refusal sentinel**. These
  may be re-implemented upstream and MUST NOT leak backlogit-specific semantics into the shared
  contract, and MUST NOT introduce a conflicting synonym token (SBLK-R21). The source-shape RED
  (`174.039-T`) pins this [local] Go shape via `go/ast`; the [shared] token/edge-set/field-names/
  event conformance is asserted SEPARATELY by U17 — the two assertion surfaces do not overlap, so
  the local API shape can evolve toward the upstream implementation without touching the shared
  contract test.

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
  `shipment_status_changed` event is the authoritative durable correlated audit record (durable
  audit evidence — NOT a non-repudiation/tamper-evident record: the JSONL is a mutable fsynced
  append with no signing, and `blocked_by` is advisory). `blocked_reason`
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
| Blocked audit metadata lifecycle & durable audit evidence | Medium — stale `blocked_*` after hook-bypassing `blocked→queued`; audit erased on clear | Single seam-owned clear helper (U4); durable `shipment_status_changed` event with actor+reason+resume_ref emitted on BOTH block and unblock BEFORE clearing (U2c/U3); `blocked_by` documented as advisory, events authoritative (durable correlated audit record, not a non-repudiation/tamper-evident guarantee) |
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

## Plan Review — current-HEAD remediation cycle 2 (2026-09-15, branch `chore/stage-155`)

dispatch_mode: single-agent-declared-degradation
decision: PASS

Bounded current-HEAD remediation of two remaining P1 findings. The prior cycle removed the
obsolete `164.002-T → 174.050-T` dependency and updated the retirement narrative, but two gaps
remained: (a) the retired task `164.002-T` was still a **member of the live queued `146-S`
manifest** (WAVE_MEMBER_BLOCKED), with `147-S` depending on `146-S` (permanent downstream block);
and (b) the authoritative Rev3 §0 headings/summaries still declared **12 tasks** while the actual
post-R7b-split topology is **13 live tasks / 14 `155-S` members**. Stage-owned artifacts only — no
source/tests, no `154-S`, no shipment claim, no PR; `146-S`/`147-S`/`155-S` remained `queued`.

* **P1-1 — Parked-era release unit reconciled (smallest valid manifest mutation).** Governed
  `backlogit shipment return-blocked --shipment 146-S --item 164.002-T` removed the retired member
  from the executable `146-S` manifest while keeping it `blocked` (history preserved via the
  return-blocked journal + `shipment_item_returned_blocked`/`item_blocked` events; `related_to
  174-F` link retained). `146-S` manifest is now `[164-F]` — a covering feature with only
  `blocked` children (`164.001-T`, `164.002-T`), i.e. **no executable task work** (features are
  excluded from the wave member set `M`). Because `146-S` has no executable work, the downstream
  `147-S → 146-S` `blocks` dependency was **removed** (`backlogit dep remove 147-S 146-S`) so no
  live queued shipment is permanently blocked by the retired empty release unit. No unsupported
  shipment status invented; `146-S`/`147-S` stay `queued`.
* **P1-2 — Rev3 §0 topology corrected to actual post-split state.** Spec §0.2 heading + topological
  note, decision §0 decomposition bullet, and plan §0 header + §0.1 heading now state **13 live
  tasks (`174.039-T…174.050-T` plus `174.051-T`)** and **14 `155-S` members including `174-F`**, and
  explicitly state `164.002-T` is **retired with no dependency on `174.050-T`**. The stale decision
  §0 "`164.002-T` re-pointed … to the final live task `174.050-T`" line was corrected. Old
  `12-task`/re-point wording survives only inside the superseded `## Plan Review — Revision 3`
  audit record.

Validation evidence (current HEAD): `backlogit sync` 1525 artifacts, parse_failures=0;
`backlogit docs lint` on all three docs — `valid: true`, 0 violations each; `backlogit doctor` — 23
pre-existing findings (unchanged), **none on touched artifacts** (`146-S`/`147-S`/`164-F`/
`164.001-T`/`164.002-T`/`155-S`/`165-F`); scheduler verification — `164.002-T` is a member of **no**
shipment manifest and **no** shipment depends on `146-S`; `147-S` sits in the `queued` frontier
(not `blocked`); `wave-scheduler-sim.ps1 -VerifyAgainstQueue` WAVE_SIM_OK 186/186 (scheduler
contract incl. WAVE_MEMBER_BLOCKED detection intact).

**Verdict: PASS** — residual P0 = 0, residual P1 = 0.

## Plan Review — Copilot PR #444 remediation (2026-09-15, branch `chore/stage-155`)

dispatch_mode: single-agent-declared-degradation
decision: PASS

Bounded remediation of two valid, in-scope Copilot review findings on PR #444 (HEAD `35912a0b`),
Stage-owned backlog/docs only — no source/tests, no `154-S`, no shipment claim, no PR/thread reply.
`155-S` remained `queued`; append-only history and all superseded artifacts preserved.

* **Finding 1 (thread `PRRT_kwDORzozKM6itxgT`) — superseded descendants inside live release scope.**
  `releaseScopeItemIDs` expands the covering feature `174-F` to ALL descendants (`IncludeArchived:
  true`), so the 38 superseded, `blocked`, non-terminal tasks `174.001-T…174.038-T` sat in `155-S`
  release scope and would fail `validateMemberGateEvidence` (`is blocked (not completed through the
  gate)`). Remediation: archived all 38 via the supported non-destructive `backlogit archive`
  lifecycle op (queue → archive; `status: archived`, `archived_status: blocked` preserved; parent
  `174-F` and per-item event logs intact). Archiving from a descope-eligible in-flight status
  (`blocked`) makes each a GENUINELY DESCOPED member that `validateMemberGateEvidence` skips, so they
  no longer block the ship while remaining fully auditable. Result: `174-F` descendants = 38 archived
  (descope-eligible) + 15 live queued; **0 blocked non-terminal descendants** remain in release scope.

* **Finding 2 (thread `PRRT_kwDORzozKM6itxgz`) — source-shape RED jumped straight to declare+implement.**
  The stateful core seam (`ShipmentBlocked` enum + `BlockShipment`/`UnblockShipment`, absent from
  `internal/core`) violated the harness chain **source-shape RED → declaration-only compile-green →
  behavior RED → implementation**. Remediation: R1 (`174.039-T`) split into
  (a) **R1s** `174.039-T` source-shape RED (go/ast pins `ShipmentBlocked` + block/unblock signatures
  + `isValidShipmentTransition` blocked shape; `^TestUR1S_`; green-maker `174.052-T`; closes wave 2),
  (b) **Rd** `174.052-T` NEW declaration-only compile-green task (lands the `ShipmentBlocked` const +
  block/unblock stub signatures + transition shape; ≤5 functions; gated by R1s — NOT a declaration-
  only exemption per P-002.1), and (c) **R1b** `174.053-T` NEW behavior RED harness (`^TestUR1B_`;
  green-makers `174.043-T,174.044-T`; closes wave 6). R2 (`174.040-T`) and R3 (`174.041-T`) now
  depend on Rd (`174.052-T`) so they compile-green before asserting behavior (closing waves 7 and 8).
  Implementation deps rewired: R4 `[R2,Rd]`, R5 `[R1b,Rd,R4]`, R6 `[R1b,Rd,R5]`. Manifest updated to
  **16 members** (`174-F` + 15 tasks) in parent-first dependency order; §0 topology in spec/plan/
  decision updated to 15 tasks / 9 waves. Every task ≤2h / ≤5 functions; RED-before-GREEN preserved.

Validation evidence (current HEAD, branch `chore/stage-155`):
`backlogit sync` — 1527 artifacts, parse_failures=0.
`wave-scheduler-sim.ps1 -VerifyAgainstQueue` — **WAVE_SIM_OK 186/186** (scheduler contract intact).
Actual RED-contract parser (`Read-RedDeliverableContract`) over the 4 live 174 RED harnesses —
**4/4 parse clean (0 errors)**; selectors `^TestUR1S_/^TestUR1B_/^TestUR2_/^TestUR3_`; closing waves
R1s@2, R1b@6, R2@7, R3@8 (match the recomputed 9-wave dependency layering); Rd `174.052-T` carries no
red-deliverable block (correct).
`backlogit docs lint` — spec/plan/decision `valid: true`, 0 violations each.
`backlogit doctor` target-mode — 9/9 touched live files exit 0; global doctor 23 pre-existing
findings (unchanged), **none on any 174.\* artifact** (archived or live).
Release-scope check — manifest task set (15) == live queued `174-F` descendant set (15); all 38
archived carry `archived_status: blocked` (descope-eligible); 0 blocked non-terminal descendants.

**Verdict: PASS** — residual P0 = 0, residual P1 = 0. `155-S` ready for Ship to claim.

## Plan Review — current-HEAD remediation cycle 3 (2026-09-15, branch `chore/stage-155`)

dispatch_mode: single-agent-declared-degradation
decision: PASS

Bounded current-HEAD remediation of two same-contract P1 findings on PR #444 (HEAD `380fa1e9`),
Stage-owned backlog/docs only — no source/tests, no `154-S`, no shipment claim, no PR/thread reply.
`155-S` remained `queued`; append-only history and all superseded records preserved (the cycle-2
and 052/053-split review records above retain their original counts/wording as audit context).

* **P1-1 — `174.052-T` (Rd) was not truly declaration-only.** The declaration wave also added the
  `isValidShipmentTransition` blocked-transition entries (active->blocked, blocked->{queued,active}).
  Populating that table is FUNCTIONAL transition-enablement: it would let generic
  `MoveShipmentStatus` persist a governed blocked transition inside the declaration wave, before any
  behavior RED or governed path exists. Remediation: `174.052-T` now lands ONLY the `ShipmentBlocked`
  enum const + the not-implemented `BlockShipment`/`UnblockShipment` stub signatures (the symbols the
  later behavior tests compile against) and explicitly MUST NOT touch `isValidShipmentTransition`,
  which continues to REJECT every blocked transition after Rd (fail-closed). `174.039-T` (R1s
  source-shape RED) correspondingly no longer asserts the transition-table shape — it pins the
  enum + block/unblock signatures only. The transition-table enablement moved to the governed
  implementation tasks AFTER behavior RED: `174.043-T` (R5) enables `active->blocked`, `174.044-T`
  (R6) enables `blocked->{queued,active}`, each gated to the governed path so generic
  `MoveShipmentStatus` remains UNABLE to persist a blocked transition (ungoverned refusal still
  asserted by R2 `174.040-T` and enforced by R7a `174.045-T`). No dependency edges, waves, or
  manifest membership changed — Rd still green-makes R1s@2; R1b (`174.053-T`) behavior RED still
  closes @6; 043/044 remain in waves 5/6, strictly after behavior RED.

* **P1-2 — stale §0 count in the authoritative plan intro.** The §0 introductory paragraph still
  declared a **13-task / 14-member** scope. Corrected to the actual **15 live tasks / 16 `155-S`
  members**, explicitly naming the core stateful-seam split tasks `174.052-T` (declaration-only
  compile-green) and `174.053-T` (core-lifecycle behavior RED). All other authoritative current
  sections (plan §0.1 heading, spec §0.2, decision §0) already stated 15/16; the only remaining
  13/14 references live inside clearly-labeled superseded `## Plan Review`/`§6d` audit records.

Validation evidence (current HEAD, branch `chore/stage-155`):
`backlogit sync` — 1527 artifacts, parse_failures=0.
`backlogit docs lint` — spec/plan/decision `valid: true`, 0 violations each.
`wave-scheduler-sim.ps1 -VerifyAgainstQueue` — **WAVE_SIM_OK 186/186** (scheduler contract intact).
Actual RED-contract parser (`Read-RedDeliverableContract`) over the 4 live 174 RED harnesses —
**4/4 parse clean**; selectors `^TestUR1S_/^TestUR1B_/^TestUR2_/^TestUR3_`; `174.052-T` (Rd) carries
NO red-deliverable block (correct). Dependency graph — acyclic, 9 waves; order R1s(039)@1 ->
Rd(052)@2 -> R1b(053)@3, and the transition-enablement tasks R5(043)@5 / R6(044)@6 fall STRICTLY
after behavior RED (053)@3. Manifest/release-scope — `155-S` 16 members (parent-first); task set
(15) == live queued `174-F` descendant set (15); 38 archived `174.001-T..174.038-T` carry
`archived_status: blocked`. Targeted text/contract check — `174.052-T` proven declaration-only (no
`isValidShipmentTransition` entries; fail-closed rejection preserved); transition enablement present
only in 043/044 after behavior RED. `backlogit doctor` — 23 pre-existing findings (unchanged),
**none on any 174.\* artifact**.

**Verdict: PASS** — residual P0 = 0, residual P1 = 0. `155-S` ready for Ship to claim.

## Plan Review — Copilot PR #444 remediation cycle 4 (2026-09-15, branch `chore/stage-155`)

dispatch_mode: single-agent-declared-degradation
decision: PASS

Bounded remediation of the two OPEN Copilot review threads on PR #444 (HEAD `409f3c54`),
Stage-owned backlog/docs only — no source/tests, no `154-S`, no shipment claim; caller replies/
resolves the threads. `155-S` remained `queued`; append-only history and all superseded records
above preserved verbatim as audit context (their prior 13/14 and 15/16 counts are intentionally
retained).

* **P1-A — `BlockShipment`/`UnblockShipment` had no objective API shape (thread
  `PRRT_kwDORzozKM6iuWEe`, `174.039-T`).** The source-shape RED and companion Rd/behavior/parity
  tasks spelled the seam only as `(...)`, leaving the harness nothing to pin and permitting CLI/MCP
  divergence. Remediation grounds the shape in existing core conventions —
  `(ctx context.Context, ws *Workspace, shipmentID string, …)` as on `MoveShipmentStatus`/
  `AddItemToShipment`/`ReturnBlockedItem`, and `(*models.Artifact, error)` return as on
  `ClaimShipment`/`CreateShipment` — and pins the EXACT [local] API shape:
  `BlockShipment(ctx, ws, shipmentID string, opts BlockOptions) (*models.Artifact, error)` and
  `UnblockShipment(ctx, ws, shipmentID string, opts UnblockOptions) (*models.Artifact, error)`, with
  `BlockOptions{Reason (required), BlockedBy, ResumeCheckpointRef}` and
  `UnblockOptions{Target (ShipmentQueued|ShipmentActive), Confirm, UnblockedBy}`. The
  declaration-only sentinels are `blerrors.ErrNotImplemented = errors.New("backlogit: not implemented")`
  and `blerrors.ErrShipmentBlockedRequiresEnvelope` in `internal/errors` (both follow the package
  `backlogit:` convention). BOTH sentinels are declared declaration-only by Rd (`174.052-T`);
  `ErrShipmentBlockedRequiresEnvelope` is WIRED (not introduced) by the R4g guard (`174.054-T`),
  so the R2 behavior harness (`174.040-T`) compiles against an already-declared symbol and there is
  no declare-after-use cycle (see P1-C). Updated: `174.039-T` (R1s
  asserts exact signatures + option struct names + sentinel via `go/ast`), `174.052-T` (Rd lands
  exactly those declarations returning the sentinel), `174.053-T` (R1b behavior RED exercises the
  exact option-carrying calls), `174.043-T`/`174.044-T` (implementations read the exact opts),
  `174.046-T` (R8 CLI+MCP parity maps surface inputs 1:1 to the option fields over the same core),
  plus spec §0.2/§2.5 and this plan's Portability boundary. The [local] Go API shape is pinned by
  R1s; the [shared] token/edge-set/field-names/event conformance stays SEPARATE (U17), so the two
  do not overlap.

* **P1-B — R5/R6 enabled `isValidShipmentTransition` edges before the generic
  `MoveShipmentStatus` refusal (thread `PRRT_kwDORzozKM6iunWq`, `174.045-T`).** R7a's generic
  refusal was scheduled at wave 7, but R5 (`174.043-T`)@W5 enabled `active→blocked` and R6
  (`174.044-T`)@W6 enabled `blocked→{queued,active}` in the table that exported `MoveShipmentStatus`
  reads directly (`internal/core/shipment.go:159-183,778-786`), exposing an ungoverned generic
  block at W5 and ungoverned unblock at W6. Remediation moves the specific `MoveShipmentStatus`
  block/unblock refusal into a new smallest task **R4g `174.054-T`**@**W4** — a top-level fail-closed
  guard placed ABOVE the transition-table check (unlike the existing `shipped` guard which sits
  after it), returning the `blerrors.ErrShipmentBlockedRequiresEnvelope` sentinel (declared
  declaration-only by Rd `174.052-T` alongside `ErrNotImplemented`; R4g WIRES it, does not
  introduce it — mirrors
  `ErrShipmentShippedRequiresEnvelope`). Because it sits above the table it refuses identically
  before AND after the edges land, so the guard is active from W4, strictly before R5/R6.
  R5 and R6 now carry a hard dependency on `174.054-T`; the governed `BlockShipment`/`UnblockShipment`
  seam writes the edges through the U2c seam-private primitive BENEATH the choke point (never the
  top-level entry point), so exemption is by construction. R7a (`174.045-T`) retains the remaining
  bypass routing (generic move/update, bulk/cascade, MoveShipmentStatus writer delegation). R2
  (`174.040-T`) green-maker list gains `174.054-T`; its close-wave stays 7. A dedicated new task
  (rather than folding into R4 `174.042-T`) was chosen because the top-level entry-point
  transition-refusal is a different layer/responsibility from R4's writer-envelope-beneath-choke-
  points concern and keeps each task's R2-portion green-mapping and ≤5-function budget clean.

Scope delta: +1 task (`174.054-T`), `155-S` now **16 tasks / 17 members incl. `174-F`** (was
15/16). Every intermediate commit remains safe (guard-before-edge); all tasks remain ≤2h / ≤5
functions / <4 scenarios.

Validation evidence (current HEAD, branch `chore/stage-155`):
`backlogit sync` — parse_failures=0.
`backlogit docs lint` — spec/plan/decision `valid: true`.
`wave-scheduler-sim.ps1 -VerifyAgainstQueue` — **WAVE_SIM_OK** (scheduler contract intact; R2
green-maker mapping resolves with `174.054-T` added).
Actual RED-contract parser (`Read-RedDeliverableContract`) over the live 174 RED harnesses — parse
clean; `174.054-T` (guard, code) carries NO red-deliverable block (correct, it is a green-maker).
Dependency graph — acyclic, 9 waves; `174.054-T`@W4 strictly before `174.043-T`@W5 / `174.044-T`@W6
(guard-before-edge verified); R5/R6 both depend on `174.054-T`. Manifest/release-scope — `155-S` 17
members (parent-first, `174.054-T` inserted after `174.042-T`); live task set (16) == live queued
`174-F` descendant set (16). Targeted contract check — exact signatures/sentinel present in
039/052/053/043/044/046; guard-refusal placed above transition table in `174.054-T`; R7a no longer
owns the MoveShipmentStatus block/unblock refusal. `backlogit doctor` — pre-existing findings only,
none on any 174.\* artifact.

**Verdict: PASS** — residual P0 = 0, residual P1 = 0. `155-S` ready for Ship to claim.

### Copilot PR #444 remediation cycle 5 (2026-09-15, HEAD `8fb4e8b5`)

Bounded remediation of two OPEN Copilot review threads; Stage-owned backlog/docs only — no
source/tests, no `154-S`, no shipment claim; caller replies/resolves the threads. Prior records
above are preserved verbatim as append-only audit history.

* **P1-C — declare-after-use compile cycle on the refusal sentinel (thread
  `PRRT_kwDORzozKM6ivJeO`, `174.040-T`/`174.054-T`).** The R2 behavior harness `174.040-T`
  references `blerrors.ErrShipmentBlockedRequiresEnvelope` (asserting the generic
  `MoveShipmentStatus` refusal for BOTH directions) and depends only on Rd `174.052-T`, but the
  sentinel was previously INTRODUCED by R4g `174.054-T`, which in turn depends on `174.040-T` —
  a declare-after-use cycle that left `174.040-T` unable to compile and coupled the two tasks in
  a logical cycle. Remediation makes the refusal sentinel a **declaration-only symbol landed by
  Rd `174.052-T`** alongside `blerrors.ErrNotImplemented` (a bare `errors.New(...)` var — no
  behavior, so Rd stays declaration-only), and **source-shape-gates its exact name in R1s
  `174.039-T`** (go/ast asserts `internal/errors` declares it). `174.040-T` now compiles against
  the already-declared sentinel and fails on BEHAVIOR only. R4g `174.054-T` now **WIRES/uses** the
  already-declared sentinel in the top-level guard rather than introducing it (its function budget
  drops the sentinel decl). No dependency-graph edge changed; the graph stays acyclic (9 waves)
  and the compile-green wave semantics hold — R1s@W1 red until Rd@W2 lands both sentinels + stubs,
  then green; R2 behavior-red compiling against Rd; R4g@W4 turns the refusal portion green.
* **P1-D — `non-repudiation` overclaim on the mutable JSONL event (thread
  `PRRT_kwDORzozKM6ivJej`).** The plan (U3 SBLK-R7 narrative + risk table) and spec (SBLK-R7)
  called the fsynced-but-mutable, unsigned `shipment_status_changed` JSONL record the
  "non-repudiation record" even though `blocked_by` is advisory and no signing/tamper-evidence
  exists. Replaced with accurate wording — **"authoritative durable correlated audit record" /
  "durable audit evidence"** — in all authoritative current docs, with an explicit note that it is
  NOT a non-repudiation/tamper-evident guarantee. No auth/signing scope added; no active acceptance
  criterion claims non-repudiation. Superseded audit history retained verbatim.

Validation evidence (HEAD after commit): `backlogit sync` parse_failures=0; `backlogit docs lint`
spec/plan/decision `valid: true`; `wave-scheduler-sim -VerifyAgainstQueue` WAVE_SIM_OK; actual
RED-contract parser clean (`174.054-T` carries no red block); dependency graph acyclic, 9 waves,
`174.054-T`@W4 strictly before `174.043-T`@W5 / `174.044-T`@W6; manifest/release-scope `155-S` 17
members (16 live == live `174-F` descendant set); targeted contract check — both sentinels declared
by Rd `174.052-T`, asserted by R1s `174.039-T`, referenced (not introduced) by R2 `174.040-T` and
R4g `174.054-T`; no `non-repudiation` claim remains in any authoritative current doc/live task.

**Verdict: PASS** — residual P0 = 0, residual P1 = 0. `155-S` remains ready for Ship to claim.

## Plan Review — Revision 4 (flat-manifest shipment scope; SBLK-R28) (2026-09-16, branch `chore/stage-155-flat-shipment-scope`)

dispatch_mode: single-agent-declared-degradation
decision: PASS

Bounded Stage amendment implementing the operator's clarified authoritative product rule: a
shipment is a **FLAT manifest of explicitly-listed deliverables** (`custom_fields.items`) — including
a feature does not implicitly include its descendants; dependencies govern execution ordering, not
membership. Stage-owned backlog/docs/planning artifacts only — NO source or test code written by
Stage; the rule is delivered as RED-first tasks so Ship implements it test-first inside `155-S`.
`155-S` remained `queued` throughout; append-only history and all superseded records above are
preserved verbatim as audit context. **ENGRAM_DEGRADED**: the agent-engram daemon was unavailable
after the required retry, so discovery used bounded local read-only code inspection — this justifies
`single-agent-declared-degradation` for this cycle, consistent with the prior cycles above.

**Scope correction added (spec §0.5/§0.6 + SBLK-R28; plan §0.4; decision §6h):** the shipment
lifecycle derived release scope by expanding a listed feature into all descendants
(`releaseScopeItemIDs` → `descendantItems`, `shipment_lifecycle.go:1142/1219`, BFS on `ParentID`,
`IncludeArchived:true`) and the member projection did the same (`compositionMemberIDs`,
`size_composition.go:291`). Direct inspection of live `155-S` confirmed the defect: a 17-entry
explicit manifest (`174-F` + 16 tasks) projected to 54 `size_composition.members` (58 after adding
the four scope tasks), pulling in the archived `174.001-T…174.038-T` band that was never an explicit
member. The flat rule makes release scope equal the explicit `custom_fields.items` manifest.

**Review findings — dispatched code-review, both RESOLVED before this verdict:**

* **P0 — the two independent-expansion consumers were unassigned.** A flat `releaseScope`
  (line 550) is consumed transitively by the evidence set/`validateMemberGateEvidence`,
  `completeReleaseScope`, and the snapshot/rollback lock set — those need no logic change. But
  `returnUnreleasedFeatureItems` (`:735`, which returns-to-backlog and `clearParentID`-**orphans**
  unlisted non-terminal descendants) and the covering-feature descendant loop in
  `collectArchiveCandidateIDs` (`:800`, which **archives** unlisted terminal descendants) call
  `descendantItems(featureID)` **directly** and are NOT flattened by changing line 550. Both are
  member-feature-guarded (133.004-T) and `174-F` IS a member, so they fire. Resolution: `174.058-T`
  now explicitly owns neutralizing both; `174.057-T` explicitly scopes them OUT and documents the
  transitive-vs-independent distinction. Spec §0.5/§0.6, plan §0.4, and decision §6h were reconciled
  to name both functions.
* **P1 — member-count overclaim.** `compositionMemberIDs` excludes the covering feature
  (non-sizable) and counts tasks only, so the flat `size_composition.members` for `155-S` is **20
  task members** (manifest `items` stays 21 incl. `174-F`), not 21 and not the expanded 58. All four
  docs and the `174.058-T` AC now state 20.

**Task delta (+4, RED-before-GREEN, each ≤2h / ≤5 functions / <4 scenarios):** `174.055-T`
(SCOPE-RED-A, derivation harness, `^TestUReleaseScopeFlat_`) and `174.056-T` (SCOPE-RED-B,
projection+ship-flow regression harness, `^TestUReleaseScopeRegression_`) precede `174.057-T`
(SCOPE-IMPL-1, flatten the `releaseScopeItemIDs` derivation) and `174.058-T` (SCOPE-IMPL-2, flatten
the `compositionMemberIDs` projection + the independent `collectArchiveCandidateIDs` /
`returnUnreleasedFeatureItems` re-expansions, plus feature-only/zero-executable-task active-slot
safety and coupled legacy-test updates). The four operator-required regression cases are allocated
so each harness stays <4 scenarios: explicitly-listed child + unlisted `blocked` descendant →
`174.055-T`; archived descendants + feature-only manifest → `174.056-T`. No new exported/public API
(internal behavior change only; source-shape ordering not triggered). Archived `174.001-T…174.038-T`
are NOT restored and were NOT required to descope `155-S` — they were never explicit members;
archival preserved history but non-membership, not archival, is the descoping mechanism.

Validation evidence (branch `chore/stage-155-flat-shipment-scope`): `backlogit docs lint`
spec/plan/decision all `valid: true` (0 violations); `doctor --target` on `174.055-T…174.058-T` all
exit 0; RED-contract blocks well-formed (green_maker `174.057-T`@close-wave 8 / `174.058-T`@close-wave
9); `wave-scheduler-sim` fixture WAVE_SIM_OK 164/164 and `-VerifyAgainstQueue` WAVE_SIM_OK 186/186
(both decoupled — bound to `130-S`/`147-F`, unperturbed by `155-S`); dependency graph acyclic, **9
waves unchanged** (`055`,`056`@W7; `057`@W8; `058`@W9) with RED strictly before its green-maker;
dep edges verified `055→044`, `056→044`, `057→{055,045,051}`, `058→{056,057}`; `155-S` manifest = 21
items (`174-F` + 20 tasks incl. `055–058`); live `size_composition.members` currently 58 (the
pre-fix expansion defect the tasks correct to 20). Topology/wave gate is independent of this
hierarchy correction (the scheduler does not call `releaseScopeItemIDs`); **no `--force` override
authorized or applied.**

**Verdict: PASS** — residual P0 = 0, residual P1 = 0. `155-S` ready for Ship to implement SBLK-R28
test-first. Unrelated residual: pre-existing `doctor` orphan findings in the `106.xxx-T` band
(present on baseline main, outside this amendment's scope).

## Plan Review — Revision 5 (flat-manifest scope hardening) (2026-09-16, branch `chore/stage-155-flat-shipment-scope`)

dispatch_mode: single-agent-declared-degradation
decision: PASS

Bounded Stage remediation of **four P1 findings** against the rev4 flat-manifest addendum. Stage-owned
backlog/docs only — NO source or test code written by Stage; `155-S` stays `queued`; no shipment claim,
no PR. **ENGRAM_DEGRADED**: the agent-engram daemon was not consulted for this cycle (bounded local
read-only code inspection of `internal/core/shipment_lifecycle.go` and `size_composition.go` was
sufficient to confirm each finding at exact line references), which justifies
`single-agent-declared-degradation`, consistent with the prior cycles above.

**Findings remediated (all four were real):**

* **P1-1 — rollback/snapshot path is an INDEPENDENT expansion, mis-scoped as "flatten transitively."**
  `rollbackIDs`/`snapshotShipArtifacts` are built at `shipment_lifecycle.go:603-612` by appending
  covering-feature ancestors (`featureIDs`) **and every** `descendantItems` on top of
  `{shipmentID} ∪ releaseScope`; flattening the derivation alone does not flatten the artifact
  lock/snapshot/restore set. **Fix:** re-scoped to `174.057-T` (neutralize `:603-612`) with a new RED
  harness `174.059-T` (SCOPE-RED-C, `^TestURollbackScopeFlat_`) asserting set-equality with
  `{shipmentID} ∪ flat manifest` and that unlisted ancestors/descendants are absent and non-restorable.
  The separate `nonMemberFeatureSnapshots`/`restoreRolledUpNonMemberFeatures` status-rollup revert is
  preserved (explicitly kept in `174.057-T` AC).
* **P1-2 — RED/GREEN contract inconsistent for `releaseScopeItemIDs`.** RED (`174.055-T`) pins the
  `releaseScopeItemIDs` seam; the rev4 `174.057-T` told the implementer to bypass it with direct
  `explicitScope` assignment. **Fix:** `174.057-T` now flattens `releaseScopeItemIDs` **in place**
  (`return uniqueNonEmptyStrings(itemIDs)`; seam + signature retained; line 549 still calls it) and all
  callers consume its flat result. The bypass instruction is removed. `174.055-T` clarified to state the
  seam is retained, so RED and GREEN target the same function.
* **P1-3 — `collectArchiveCandidateIDs` also appends unlisted linked deliberations.** Its covering-feature
  loop appends `linkedDeliberationIDs(feature)` unguarded. **Fix:** folded into `174.058-T` ownership;
  `174.060-T` (SCOPE-RED-D) asserts an unlisted linked deliberation is absent from `ArchivedIDs` and
  untouched.
* **P1-4 — archive RED used already-archived descendants the collector skips (a no-op).** **Fix:**
  `174.060-T` uses an unlisted **terminal-but-not-archived** descendant (`done`/`accepted`) — which the
  collector DOES append today — asserting it stays untouched and absent from `ArchivedIDs`;
  archived-descendant projection coverage retained separately (`174.056-T` scenario 2 as projection-only;
  `174.060-T` negative-control that no archived artifact is restored for scope).

**Task delta (rev5): +2 (scope-correction delta now +6).** New `174.059-T` (SCOPE-RED-C →
`174.057-T`@close-wave 8) and `174.060-T` (SCOPE-RED-D → `174.058-T`@close-wave 9), each `dep 174.044-T`,
≤2h / <4 scenarios / tests. Impl edges added `057→059`, `058→060`. Flat `size_composition.members`
target updated `20 → 22`; spec §0.5/§0.6, plan §0.4, and decision §6i reconciled. No new exported/public
API; no archived/superseded task restored.

**Validation evidence (branch `chore/stage-155-flat-shipment-scope`):** `backlogit sync` OK (1534
artifacts, 0 parse failures); `backlogit docs lint` spec/plan/decision all `valid: true` (0 violations);
`doctor --target` on `174.055-T`,`174.056-T`,`174.057-T`,`174.058-T`,`174.059-T`,`174.060-T`,`155-S` all
exit 0; RED-contract blocks well-formed (green_maker `174.057-T`@close-wave 8 for `055`/`059`;
`174.058-T`@close-wave 9 for `056`/`060`); `wave-scheduler-sim` fixture **WAVE_SIM_OK 164/164** and
`-VerifyAgainstQueue` **WAVE_SIM_OK 186/186** (both decoupled — bound to `130-S`/`147-F`, unperturbed by
`155-S`); markdownlint **0 issues**; dependency-graph check over the `155-S` manifest: **acyclic**,
manifest a valid **parent-first topological order**, **23 items (`174-F` + 22 tasks)**; dep edges
verified `059→044`, `060→044`, `057→{055,059,045,051}`, `058→{056,060,057}`; **9 waves unchanged**
(`055`,`059`,`056`,`060`@W7; `057`@W8; `058`@W9) with each RED strictly before its green-maker.
Topology/wave gate independent of this correction (the scheduler reads neither `releaseScopeItemIDs`,
the member projection, nor the rollback set); **no `--force` override authorized or applied.**

**Verdict: PASS** — residual P0 = 0, residual P1 = 0. `155-S` remains `queued` and ready for Ship to
implement SBLK-R28 test-first. Unrelated residual: pre-existing `doctor` orphan findings in the
`016.xxx`/`106.xxx-T` bands (present on baseline, outside this amendment's scope).

## Plan Review — Revision 6 (non-member ancestor rollup elimination; concurrency) (2026-09-16, branch `chore/stage-155-flat-shipment-scope`)

dispatch_mode: multi-agent-dispatch
decision: PASS

**Reviewer:** Concurrency Reviewer subagent (independent), read-only over `internal/core/shipment_lifecycle.go`, `internal/core/shipment.go`, and task contracts `174.057-T`/`174.058-T`/`174.059-T`/`174.061-T`/`174.062-T`. **Confidence: High.**

**Scope of this revision.** Resolves the residual P1 lost-update race that survived rev5: `174.057-T` removes unlisted covering-feature ancestors from the outer artifact-mutation lock set (`:603-612`), but the rev5 tasks explicitly PRESERVED the separate non-member covering-feature status-rollup path (`snapshotNonMemberFeatureStatuses` @`:598`; `restoreRolledUpNonMemberFeatures` in-line @`:684`, deferred fallback @`:496`, and inside `classifyShippedEventAppendFailure` @`:927`). Because the unlisted ancestor is no longer locked, a concurrent mutation to it between snapshot and restore is silently overwritten (P1 lost update). Root cause: `completeReleaseScope` → `setArtifactStatus` → `cascadePersistedParentStatuses` (`:1267`) walks UP the parent chain marking any ancestor `done` with no shipment-membership awareness.

**Resolution reviewed (authoritative flat-manifest rule / SBLK-R28):** eliminate the non-member ancestor rollup side effect entirely rather than re-lock/CAS-protect it. Two coordinated moves in ONE impl task (`174.062-T`): (1) bound `cascadePersistedParentStatuses` to explicit members via an unexported context boundary key sourced from `explicitScopeSet` (stop the up-walk at a non-member parent; absent key ⇒ unchanged global cascade for all non-ship callers); (2) delete `snapshotNonMemberFeatureStatuses` + `restoreRolledUpNonMemberFeatures` + `featureStatusSnapshot`/`nonMemberFeatureSnapshots` and drop them from `ShipShipment` and `classifyShippedEventAppendFailure`. RED harness `174.061-T` (`^TestUNonMemberRollupSafe_`, 3 scenarios) pins the contract using existing production seams (`persistArtifactPreLockHook`, `persistArtifactWriteFn`).

### Reviewer conclusions
* **Race fully closed at root.** Removing the unbounded upward cascade for non-members means the ancestor is never written by the ship, so there is nothing to snapshot or restore — the snapshot↔restore window that hosted the lost update ceases to exist. Enumerated status-rollup sites (`:598`,`:684`,`:496`,`:927`,`:1267`,`:648`) confirmed COMPLETE against source; `collectArchiveCandidateIDs` already membership-guards non-members (`:790`), not a missed site.
* **Boundary-key cascade correct/safe.** Stopping the up-walk at a non-member parent cannot strand a member descendant (cascade propagates upward only); "absent key ⇒ global behavior" holds (only `ShipShipment` sets the key; context values immutable; concurrent writer uses its own ctx — no cross-goroutine leak). **[SUPERSEDED by rev7]** — this conclusion missed a SAME-goroutine leak: the boundary-bearing outer `ctx` also flows into the post-ship `FirePost` hook (`:704`), so the boundary must be carried on a separate in-closure context (see rev7).
* **Seams verified present** and fire at the claimed boundaries (`shipment.go:~840`); tests are RED today and GREEN only after `174.062-T`.
* **No residual P0/P1.** Listed member features remain locked (per `174.057-T`) and get governed `done` directly via `:648`; deferred-fallback removal leaves no uncompensated failure path. Atomicity (both moves in one task) confirmed necessary — move-2-before-move-1 would leave non-members rolled-up-but-unreverted.

### Advisory findings folded into the task contracts before this PASS
* **F1 (was P2) → `174.062-T`:** the context boundary key MUST be injected on the exact `ctx` threaded into every in-closure `setArtifactStatus` caller (`completeReleaseScope`, the `:648` member loop, and `returnUnreleasedFeatureItems` `:735`), AFTER the `lockArtifactMutations` ctx reassignment (`:610`) and BEFORE `completeReleaseScope`. Contract now states this injection point explicitly. **[SUPERSEDED by rev7]** — injecting on the function-scope `ctx` leaks the boundary into the escaping post-closure/post-ship-hook path (`FirePost` `:704`); rev7 replaces this with a SEPARATE in-closure `scopedCtx` threaded only to the three governed callers, leaving the outer `ctx` boundary-free.
* **F3 (was P2) → `174.061-T`:** scenarios 2 & 3 now anchor the concurrent-writer injection to a LISTED MEMBER's persist boundary (fires on both current and fixed code) rather than to `F` (which never persists post-fix), keeping the assertions RED-now / GREEN-after instead of vacuously green.
* **F2 (P3) → `174.062-T` AC(2):** the `returnUnreleasedFeatureItems` (`:735`) return-to-backlog cascade is now named explicitly as a second non-member path covered by the bounded-cascade fix.
* **F4 (P3) → `174.062-T`:** the member-above-non-member invariant (`T`→`F`(non-member)→`E`(member): `E` gets `done` via the DIRECT `:648` write, not via cascade) is pinned so a future change cannot silently reintroduce a stale-member-ancestor bug.
* **F5 (P3):** atomicity + dependency order confirmed correct (`174.062-T` deps `{174.061-T, 174.057-T, 174.058-T}`; the exploitable window opens at `174.057-T`@W8 and the fix lands @W10, never released to Ship independently).

**Validation evidence (branch `chore/stage-155-flat-shipment-scope`):** `backlogit sync` OK (1536 artifacts, 0 parse failures); `backlogit docs lint` on plan/spec/decision all `valid: true` (0 violations); `doctor --target` on `174.057-T`,`174.058-T`,`174.059-T`,`174.061-T`,`174.062-T`,`155-S` all exit 0 (`ok: true`, `kind: pass`); RED-contract block well-formed (`174.061-T` green_maker `174.062-T`@close-wave 10); `wave-scheduler-sim` fixture **WAVE_SIM_OK 164/164** and `-VerifyAgainstQueue` **WAVE_SIM_OK 186/186**; dependency-graph check over the `155-S` manifest: **acyclic**, manifest a valid **parent-first topological order**, **25 items (`174-F` + 24 tasks)**; dep edges verified `061→044`, `062→{061,057,058}`; **waves 9 → 10** (`061`@W7; `062`@W10) with the RED strictly before its green-maker. Topology/wave gate independent of this correction; **no `--force` override authorized or applied.**

**Verdict: PASS** — residual P0 = 0, residual P1 = 0. `155-S` remains `queued` and ready for Ship to implement SBLK-R28 test-first (RED `174.061-T` → GREEN `174.062-T`). Unrelated residual: pre-existing `doctor` orphan findings in the `016.xxx`/`106.xxx-T` bands (present on baseline, outside this amendment's scope).

## Plan Review — Revision 7 (post-ship-hook cascade isolation; terminology) (2026-09-16, branch `chore/stage-155-flat-shipment-scope`)

dispatch_mode: multi-agent-dispatch
decision: PASS

**Reviewer:** independent read-only Concurrency/code-review pass over `internal/core/shipment_lifecycle.go`, `internal/core/shipment_verify.go`, `internal/core/archive.go`, and task contracts `174.057-T`/`174.059-T`/`174.061-T`/`174.062-T`/`174.063-T`. **Confidence: High.** This is **concurrency fix-cycle-3** — the third and final bounded review-fix cycle — resolving exactly **two same-contract P1 findings** against the rev6 non-member-rollup-elimination addendum. Stage-owned backlog/docs only; `155-S` stays `queued`; no source/tests/PR by Stage.

**Finding 1 (P1) — member boundary leaks into post-ship hooks.** rev6 F1 instructed injecting the membership boundary key on the function-scope `ctx` that `lockArtifactMutations` reassigns. Verified against source: `shipment_lifecycle.go:615` is an `=` assignment (not `:=`) to the function-scope `ctx` (declared with `releaseArtifactLocks` at `:470`), so it ESCAPES the `ShipShipment` governed closure and is the same `ctx` handed to `collectArchiveCandidateIDs` (`:664`), `VerifyPostShipConsistency` (`:689`), and the top-level post-ship `FirePost` hook (`:704`). Setting the boundary there would SUPPRESS the legitimate GLOBAL parent-status cascade that an UNRELATED artifact update inside a post-ship hook callback must still trigger. **Resolution:** `174.062-T` now requires deriving a SEPARATE in-closure `scopedCtx := context.WithValue(ctx, <unexportedBoundaryKey>, explicitScopeSet)` AFTER the `:615` lock reassignment and BEFORE `completeReleaseScope`, threaded ONLY into the three governed callers (`completeReleaseScope` `:630`, the `:648` member loop, `returnUnreleasedFeatureItems` call `:637`/def `:735`); the function-scope `ctx` stays boundary-free for all post-closure work. A new RED harness `174.063-T` (SCOPE-RED-F, `^TestUPostShipHookCascadeGlobal_`, dep `174.044-T`, green-maker `174.062-T`) pins the contract.

**Finding 2 (P1) — impossible "manifest-only" transactional-set wording.** Corrected authoritative terminology across spec §0.5 point 3 / point 8 / SBLK-R28 / §0.6 rev6 addendum, plan rev6 update block, decision §6j, and tasks `174.057-T`/`174.059-T`/`174.061-T`/`174.062-T`/`174.063-T`: shipment RELEASE/MEMBER scope == exactly flat `custom_fields.items`; the transactional lock/snapshot/rollback set == `{shipment control record ID} ∪ explicit manifest IDs`; the shipment control record is the SOLE control-record exemption (never a `custom_fields.items` member, but locked/snapshotted/transitioned because ship/rollback mutates its own status); no other absent artifact may be mutated/snapshotted/restored/locked due to hierarchy. Verified against source: `rollbackIDs := append([]string{shipmentID}, releaseScope...)` (`:603`), so `{shipmentID} ∪ manifest` is the correct set and "manifest only" is retracted (`174.059-T` scenario 3, `174.061-T`).

### Reviewer conclusions
* **Finding 1 escape claim confirmed TRUE** against source — the `:615` `=` assignment mutates the function-scope `ctx`; no `ctx :=` shadow exists in the closure; `FirePost` (`:704`) receives it.
* **Separate-`scopedCtx` fix sound AND complete.** The only trigger of the bounded rollup is `setArtifactStatus → cascadePersistedParentStatuses` (`:1261`). All three governed callers get `scopedCtx`. Every post-closure call was verified boundary-safe: `collectArchiveCandidateIDs` is read-only; `attachCommitToItems` persists commit only; `archiveItems`→`ArchiveItem` cascades DOWNWARD only (no upward rollup in `archive.go`); `VerifyPostShipConsistency` takes `_ context.Context` (ignores ctx, structurally cannot cascade); `moveShipmentStatusWithHeadGuard` uses `persistArtifactWithGuard` with no cascade (shipment control record is the exempt transition). No other cascade-bearing post-closure path exists.
* **`174.063-T` is genuinely RED-now and a valid discriminator.** Its RED-now anchor pins the EVENT history (no `"child status rollup"`, no revert event on the non-member ancestor `F`) — correct, because the current snapshot/restore reverts `F`'s status to pre-ship so a status-only assertion would be vacuously green. Truth table: current code → RED (anchor); correct separate-`scopedCtx` fix → GREEN; naive boundary-on-escaping-`ctx` fix → RED (the hook's `C→P` up-walk stops at non-member `P`).
* **Finding 2 terminology internally consistent** and matches source; no contradictory statement about the shipment control record remains.
* **No residual P0/P1.** Atomicity, dependency order (`062 → {061,063,057,058}`; RED harnesses `061`/`063`@W7 strictly before green-maker `062`@W10), and RED-before-GREEN discipline all hold.

**Validation evidence (branch `chore/stage-155-flat-shipment-scope`):** `backlogit sync` OK (**1537 artifacts, 0 parse failures**); `backlogit docs lint` on plan/spec/decision all `valid: true` (0 violations); `doctor --target` on `174.057-T`,`174.058-T`,`174.059-T`,`174.061-T`,`174.063-T`,`174.062-T`,`155-S` all exit 0 (`ok: true`, `kind: pass`); RED-contract blocks well-formed (`174.061-T` and `174.063-T` both green_maker `174.062-T`@close-wave 10); `wave-scheduler-sim` fixture **WAVE_SIM_OK 164/164** and `-VerifyAgainstQueue` **WAVE_SIM_OK 186/186** across 21 scenarios; dependency-graph check over the `155-S` manifest: **acyclic (0 cycles)**, **parent-first (0 violations)**, in-manifest **topological order valid (0 violations)**, **26 members (`174-F` + 25 tasks)**, manifest order `… 174.061-T[23] → 174.063-T[24] → 174.062-T[25]` (RED strictly before green-maker); dep edges verified `061→044`, `063→044`, `062→{061,063,057,058}`. Topology/wave gate independent of this correction; **no `--force` override authorized or applied.**

**Verdict: PASS** — residual P0 = 0, residual P1 = 0. This was **fix cycle 3 (final)**; both in-scope P1 findings are resolved and no in-scope P0/P1 remains. `155-S` remains `queued` and ready for Ship to implement SBLK-R28 test-first (RED `174.061-T` + `174.063-T` → GREEN `174.062-T`). Unrelated residual: pre-existing `doctor` orphan findings in the `016.xxx`/`106.xxx-T` bands (present on baseline, outside this amendment's scope).

<!-- plan-review-attempt: rev8-wave3-verification-ownership -->

## Plan Review — Amendment (Wave 3 RED-harness verification-ownership correction) (2026-09-21, branch `feat/155-s-s14-resumable-shipment-blocked-lifecycle-status`)

dispatch_mode: multi-agent-dispatch
decision: PASS

**Trigger.** `155-S` is an ACTIVE shipment mid-Wave-3. The single authorized cycle-4
harness review (see `docs/memory/2026-09-21/155-s-wave-3-cycle-4-review-blocker.md`) passed
formatting, compile, vet, CI-pinned lint, and the expected assertion-RED selectors
(`^TestUR1B_`, `^TestUR2_`, `^TestUR3_`) but RETAINED three P1 findings because the plan/task
contracts assigned **verification ownership** incorrectly. This amendment corrects the
verification-ownership contracts on the EXISTING Wave-3 tasks. It is a Stage-owned
plan/backlog-contract change only: NO production or test code, NO status/dependency/manifest/
priority mutation, NO new task, and the four uncommitted Wave-3 harness files
(checkpoint `checkpoint-20260921-194021.json`, Ship-owned) are untouched. `155-S` stays
`active`; `154-S` and PR #449 untouched.

**Root-cause corrections encoded (cycle-4 P1 findings 1–3 + writer-control gap):**

1. **Deterministic workspace-global-lock contention proof relocated RED → GREEN.** A
   deterministic proof that recovery contends on the workspace-GLOBAL lock CANNOT be authored
   reliably in the PRE-implementation RED harness **R3 `174.041-T`**: the only observable hook
   available pre-implementation is immediately before the artifact lock, so a global-lock-less
   implementation passes whenever the competing goroutine is scheduled late (exactly cycle-4 P1
   finding 1). `174.041-T` now carries an explicit EXCLUSION disclaiming that proof; the
   deterministic, scheduler-independent contention proof is now OWNED by the POST-implementation
   GREEN task **R10 `174.048-T`**, which asserts it against the real recovery + lock mechanism
   defined by **R9 `174.047-T`**. `174.048-T` already depends on `[174.041-T, 174.047-T]`, so
   **no dependency edge is added or changed**.
2. **RED recovery responsibilities retained on `174.041-T`.** The RED harness keeps
   responsibility for: startup/governed recovery INVOCATION; **policy-aware terminal state**
   (compensated ⇒ exact preimage restored; committed ⇒ target fully applied); recovered
   CANONICAL authority (Markdown source + append-only `shipment_status_changed` event, never the
   index, as state of record); and correlated TERMINAL audit evidence (correlation id ties intent
   → committed/compensated outcome). The GREEN subprocess task `174.048-T` proves the same
   canonical-authority and terminal-audit outcomes end-to-end post-implementation (cycle-4 P1
   finding 3).
3. **SQLite index = sync/rebuild convergence, NOT immediate equality.** The SQLite index is a
   rebuildable, non-authoritative projection. Both `174.041-T` (RED) and `174.048-T` (GREEN) now
   express any index check as SYNC/REBUILD CONVERGENCE — equality asserted only AFTER `sync` —
   and explicitly state that immediate-projection equality during recovery is NOT a RED-harness
   requirement (cycle-4 P1 finding 3).
4. **`174.040-T` positive non-shipment controls expanded to every guarded surface.** The R2
   writer/bypass RED harness now requires POSITIVE non-shipment controls for EVERY named guarded
   surface — public `WriteArtifactFile` boundary, private lower writer, generic move/update,
   `MoveShipmentStatus`, bulk/cascade status updates, and create-as-active/generic activation —
   proving each guard scopes strictly on `ArtifactType=="shipment"` and preserving ordinary
   task/member transitions on the SAME surface, not only `UpdateArtifact` (cycle-4 P1 finding 2).

> **CORRECTION (rev9, 2026-09-21):** item #4 is SUPERSEDED IN PART. `MoveShipmentStatus` is
> REMOVED from the positive-non-shipment-control matrix — it is a shipment-only entry point through
> which no non-shipment artifact can legitimately flow, so a literal non-shipment success path there
> is impossible and permitting `queued→active` via `MoveShipmentStatus` to construct one would
> conflict with activation being restricted to claim/confirmed unblock. The matrix now applies ONLY
> to the polymorphic/generic surfaces (`WriteArtifactFile`, private lower writer, generic
> move/update, bulk/cascade, create-as-active/generic activation). `MoveShipmentStatus` instead
> carries shipment-specific same-surface controls. rev8 items #1–#3 are unchanged. See the rev9
> amendment section below.

**RED-before-GREEN discipline preserved.** The relocation moves a GREEN, post-implementation
assertion INTO a GREEN task (`174.048-T`) and OUT of a RED harness (`174.041-T`) — never the
reverse. Both `174.040-T` and `174.041-T` remain BEHAVIOR-RED-by-design (they still fail on
behavior until their unchanged green-makers land). The red-deliverable-contract blocks
(`174.040-T`: green_maker `174.054-T/174.042-T/174.045-T/174.051-T`, close-wave 7;
`174.041-T`: green_maker `174.047-T/174.048-T`, close-wave 8) are UNCHANGED. Scenario budgets
respected: `174.040-T`/`174.041-T` `<4 scenario groups`; `174.048-T` stays at 3 scenarios
(contention/convergence/audit folded in as assertions/post-conditions, no new scenario).

**Review dispatch (multi-agent).**
* Correctness Reviewer (`claude-opus-4.8`) — VOTE: PASS — all four findings encoded YES/YES/YES/YES;
  RED harnesses remain behavior-red-by-design; relocation direction correct; no new dependency edge.
* Scope Boundary Auditor (`gemini-3.8-flash`) — VOTE: ADVISORY — no P0/P1/P2 scope creep; two
  non-blocking P3 advisories: (a) the terminal-audit post-condition on `174.048-T` is a GREEN
  mirror of the relocated proof AND is directly mandated by cycle-4 P1 finding 3 (in-scope,
  dispositioned); (b) the six-surface control matrix on `174.040-T` sits at the upper edge of the
  2-hour rule (assertions folded into existing scenario groups; effort-edge advisory only, no
  structural change). Both advisories dispositioned; neither blocks.

**Aggregate: zero P1/P2 findings across both personas.** Both reviewers operated read-only and
could not run `git diff`; Stage independently verified the diff touches ONLY AC/description prose
plus the `updated_at` header on the three files, with no dependency/status/priority/id/parent/
membership/contract-block mutation.

**Validation evidence (branch `feat/155-s-s14-resumable-shipment-blocked-lifecycle-status`):**
`backlogit sync` OK (**1555 artifacts, 0 parse failures**); `doctor --target` on
`174.040-T`, `174.041-T`, `174.048-T` all `ok: true` / `kind: pass` (exit 0); shipment `155-S`
manifest unchanged (**26 members**, `174-F` + 25 tasks); dependencies unchanged
(`174.040-T→[174.052-T]`, `174.041-T→[174.052-T]`, `174.048-T→[174.041-T,174.047-T]`);
red-deliverable-contract blocks unchanged. The four Ship-owned uncommitted harness files were not
read for content, not edited, and are excluded from the amendment commit.

**Verdict: PASS** — residual P0 = 0, residual P1 = 0. Verification ownership now assigns the
deterministic global-lock contention proof to GREEN `174.048-T`, defines SQLite verification as
sync/rebuild convergence, preserves the RED recovery/terminal-state/canonical-authority/
correlated-audit assertions on `174.041-T`, and expands `174.040-T` non-shipment controls to
every guarded surface. No new tasks, dependencies, members, or status changes. Ready for Ship to
resume Wave-3 implementation against the corrected contracts.

<!-- plan-review-attempt: rev9-wave3-moveshipmentstatus-control-correction -->

## Plan Review — Amendment (Wave 3 `MoveShipmentStatus` non-shipment-control correction) (2026-09-21, branch `feat/155-s-s14-resumable-shipment-blocked-lifecycle-status`)

dispatch_mode: single-agent-declared-degradation
decision: PASS

**Trigger.** A fresh Ship harness review of the rev8-amended Wave-3 contracts returned two P1
observations. This bounded Stage-owned amendment resolves the first (a genuine contract defect) and
records the disposition of the second (NOT a contract defect). It is a plan/backlog-contract change
only: NO production or test code, NO status/dependency/manifest/priority mutation, NO new task.
`155-S` stays `active`; `154-S` and PR #449 untouched; the four uncommitted Ship-owned Wave-3
harness files are excluded from this commit.

**P1-1 (contract defect — CORRECTED): rev8 item #4 required an impossible non-shipment success path
through the shipment-only `MoveShipmentStatus`.** rev8 amendment item #4 (and `174.040-T` AC (4))
required POSITIVE non-shipment controls for EVERY named guarded surface, INCLUDING
`MoveShipmentStatus`. Fresh review correctly found this literally impossible/incorrect:
`MoveShipmentStatus` is inherently shipment-specific and cannot legitimately receive a non-shipment
artifact, so an ordinary non-shipment (task/member) status transition cannot succeed through it;
substituting `UpdateArtifact` does not satisfy the literal contract, and allowing `queued→active`
via `MoveShipmentStatus` to manufacture such a path would conflict with activation being restricted
to `ClaimShipment`/confirmed unblock-to-active. Correction:
* The positive-non-shipment-control matrix (rev8 #4 / `174.040-T` AC (4)) now applies ONLY to the
  POLYMORPHIC/GENERIC surfaces that can legitimately receive a non-shipment artifact: public
  `WriteArtifactFile`, the private lower writer, generic move/update, bulk/cascade status updates,
  and create-as-active/generic activation as applicable. `MoveShipmentStatus` is REMOVED from that
  matrix.
* `MoveShipmentStatus` instead carries the CORRECT shipment-specific SAME-SURFACE controls
  (`174.040-T` new AC (5)): shipment-only type enforcement / rejection of a non-shipment ID where
  the surface is callable with one; refusal of governed `blocked`/unblock bypasses (top-level
  `blerrors.ErrShipmentBlockedRequiresEnvelope` for BOTH directions via the R4g guard `174.054-T`);
  and preservation of every otherwise-allowed shipment transition EXCEPT that activation remains
  restricted to claim/confirmed unblock per this plan. NO non-shipment success path is asserted or
  required through the shipment-only API.

**P1-2 (NOT a contract defect — obligation PRESERVED explicitly): `174.041-T` correlated
`shipment_status_changed` terminal evidence for the compensated restored-preimage status.** The
fresh review's second P1 asks that `174.041-T` require correlated `shipment_status_changed` terminal
evidence for the compensated restored-preimage status. That obligation is ALREADY required by
`174.041-T` AC (2) (canonical authority = Markdown source + append-only `shipment_status_changed`
event) and AC (3) (correlated TERMINAL audit event for the committed/compensated outcome). This
amendment makes it EXPLICIT in AC (3) — the compensated restore's correlated terminal evidence is
specifically a `shipment_status_changed` event recording the restored preimage status — and records
that the obligation is RETAINED and NOT weakened. No RED→GREEN relocation; `174.041-T` remains
BEHAVIOR-RED-by-design.

**All other rev8 corrections unchanged.** rev8 items #1 (global-lock contention proof relocated
RED→GREEN to `174.048-T`), #2 (RED recovery responsibilities retained on `174.041-T`), and #3
(SQLite = sync/rebuild convergence) stand exactly as recorded. rev8 item #4 is superseded IN PART
only as to the `MoveShipmentStatus` surface (see the inline correction marker on rev8 item #4
above); its generic-surface positive controls are retained.

**Review dispatch.** dispatch_mode `single-agent-declared-degradation`: this bounded prose-only
contract correction was reviewed directly by Stage against the fresh-review findings and the plan's
activation/refusal invariants; no multi-agent dispatch was run for this narrow follow-up. Focused
correctness check: (a) corrected AC (4) no longer asserts an impossible non-shipment success path;
(b) AC (5) restates only same-surface shipment controls already consistent with the base-plan
governed-refusal inventory and the claim/unblock-restricted activation rule; (c) `174.041-T`
obligation strengthened-not-weakened; (d) no id/parent/status/priority/dependency/membership/
red-deliverable-contract mutation on any task.

**Validation evidence (branch `feat/155-s-s14-resumable-shipment-blocked-lifecycle-status`):**
`backlogit sync` OK (**1555 artifacts, 0 parse failures**); `doctor --target` on `174.040-T` and
`174.041-T` both `ok: true` / `kind: pass` (exit 0); shipment `155-S` manifest unchanged (**26
members**, `174-F` + 25 tasks); dependencies unchanged (`174.040-T→[174.052-T]`,
`174.041-T→[174.052-T]`); red-deliverable-contract blocks unchanged. The four Ship-owned uncommitted
harness files were not read for content, not edited, and are excluded from this commit.

**Verdict: PASS** — residual P0 = 0, residual P1 = 0. `MoveShipmentStatus` non-shipment control
requirement corrected to shipment-specific same-surface controls; generic-surface positive controls
retained; `174.041-T` correlated `shipment_status_changed` terminal-evidence obligation preserved
explicitly and not weakened. No new tasks, dependencies, members, or status changes. Ready for Ship
to resume Wave-3 implementation against the corrected contracts.

<!-- plan-review-attempt: rev10-wave3-adversarial-consensus-ownership -->

## Plan Review — Amendment (Wave 3 adversarial-consensus verification-ownership resolution) (2026-09-21, branch `feat/155-s-s14-resumable-shipment-blocked-lifecycle-status`)

dispatch_mode: multi-agent-dispatch
decision: PASS

**Trigger.** An adversarial four-reviewer consensus over the rev8/rev9-amended Wave-3 contracts
completed and returned a bounded set of verification-ownership decisions. This rev10 amendment
encodes those decisions EXACTLY onto the existing tasks and plan. It is a Stage-owned
plan/backlog-contract change only: NO production or test code, NO status/dependency/manifest/
priority mutation, NO new task, NO new scenario group. `155-S` stays `active`; `154-S` and PR #449
untouched; the four uncommitted Ship-owned Wave-3 harness files are excluded from this commit.

**Consensus decisions encoded:**

1. **`174.040-T` RED retains two HIGH/P1 obligations (added as AC (6)/(7), assertions folded into
   existing bypass-path scenario groups — no new group).** (a) Representative invalid blocked
   INGRESS/EGRESS coverage across EVERY polymorphic lower-writer surface enumerated in AC (4) —
   both directions refused on every such surface, not just one. (b) The BulkUpdateStatus /
   bulk-cascade harness MUST inspect item-level `BulkUpdateResult.Failed` entries (not only the
   top-level error) AND prove AGGREGATE IMMUTABILITY (all-or-nothing; no batch member left
   mutated on a refused blocked transition).

2. **Blocked-member mutation and drift/CAS proofs are GREEN-owned by existing `174.043-T` /
   `174.044-T`, NOT Wave-3 RED P1s.** Ownership clarified in place: the blocked-member-mutation
   guard is GREEN-owned by R5 `174.043-T`; the drift/CAS exact-snapshot-restore proof is
   GREEN-owned by R6 `174.044-T`. R1b `174.053-T` (RED) carries neither. No dependency-graph
   change.

3. **Deterministic Claim/unblock CONTENTION proof is GREEN-owned by `174.044-T`, paralleling the
   rev8 recovery-contention relocation to GREEN R10 `174.048-T`.** R1b `174.053-T` (RED) MAY assert
   the externally visible single-active-shipment invariant (blocked excluded) but MUST NOT require
   an operation-owned deterministic contention barrier before implementation — adversarial
   consensus found NO authentic pre-implementation RED-observable contention seam for Claim/unblock.
   No dependency-graph change.

4. **rev9 correlated compensated-status evidence REMAINS required (no `174.041-T` change).** The
   rev9 obligation on `174.041-T` AC (3) — a correlated `shipment_status_changed` event recording
   the restored preimage status for a COMPENSATED restore — stands unchanged. A blanket
   post-reopen SUFFIX requirement is NOT required and MUST NOT be imposed: `174.041-T` AC (2)/(3)
   already handle the committed roll-forward case ("committed ⇒ target fully applied") via the same
   canonical authority, and legitimate PRE-CRASH roll-forward evidence must NOT be rejected. No
   contract change to `174.041-T`; this disposition is recorded here so no reviewer or implementer
   reads a suffix-ordering requirement into AC (3). `174.041-T` therefore needs NO edit under rev10.

**Authorized Ship follow-up (bounded).** This amendment authorizes Ship's final bounded patch
ONLY in `internal/core/shipment_blocked_writer_harness_test.go` for exactly the two consensus
HIGH/P1 fixes in decision 1 (invalid blocked ingress/egress coverage across polymorphic
lower-writer surfaces; BulkUpdateStatus item-level `Failed` inspection + aggregate immutability).
It MUST NOT reopen decisions 2, 3, or 4 as RED blockers, and MUST NOT touch the other three
uncommitted harness files.

**Review dispatch (multi-agent — adversarial four-reviewer consensus).** The four-reviewer
adversarial consensus is the authoritative review of record for this amendment; Stage encoded its
decisions verbatim and independently verified the encoding: (a) AC (6)/(7) fold into existing
scenario groups with no new group and no scenario-count-limit breach; (b) decisions 2/3 are
ownership clarifications only, with every green-maker edge and red-deliverable-contract block left
byte-for-byte intact; (c) decision 4 imposes no `174.041-T` contract change and preserves the rev9
obligation while barring a suffix requirement; (d) no id/parent/status/priority/dependency/
membership mutation on any task.

**Validation evidence (branch `feat/155-s-s14-resumable-shipment-blocked-lifecycle-status`):**
`backlogit sync` OK (**1555 artifacts, 0 parse failures**); `doctor --target` on `174.040-T`,
`174.043-T`, `174.044-T`, and `174.053-T` all `ok: true` / `kind: pass` (exit 0); `docs lint` on
this plan valid (0 violations); shipment `155-S` manifest unchanged (**26 members**, `174-F` + 25
tasks); dependencies unchanged (`174.040-T→[174.052-T]`, `174.053-T→[174.052-T]`,
`174.043-T→[174.052-T,174.053-T,174.042-T,174.054-T]`,
`174.044-T→[174.052-T,174.053-T,174.043-T,174.054-T]`); all red-deliverable-contract blocks
unchanged; only `updated_at` and description/AC prose changed on the four edited task files. The
four Ship-owned uncommitted harness files were not read for content, not edited, and are excluded
from this commit.

**Verdict: PASS** — residual P0 = 0, residual P1 = 0. Verification ownership is now unambiguous:
`174.040-T` RED retains the two consensus HIGH/P1 writer obligations; blocked-member mutation,
drift/CAS, and Claim/unblock contention proofs are GREEN-owned by `174.043-T`/`174.044-T`; R1b
`174.053-T` asserts only the externally visible single-active invariant with no pre-implementation
contention barrier; and the rev9 compensated-status evidence obligation on `174.041-T` stands
without a post-reopen suffix requirement. No new tasks, dependencies, members, scenario groups, or
status changes. Ship's final bounded patch is authorized solely in the writer harness for the two
decision-1 fixes.

<!-- plan-review-attempt: rev11-wave7-red-deliverable-early-green-correction -->

## Plan Review — Amendment (Wave 7 `^TestUR3_` red-deliverable early-green mapping correction + 3-P1 P-021 classification) (2026-09-22, branch `feat/155-s-s14-resumable-shipment-blocked-lifecycle-status`)

dispatch_mode: single-agent-declared-degradation
decision: PASS

**Trigger.** Ship reported primary blocker `WAVE_RED_DELIVERABLE_EARLY_GREEN` (P-002.6): the
`^TestUR3_` red-deliverable selector greened after completed GREEN task R9 `174.047-T` (wave 7),
but the authoritative red-deliverable mapping on RED task `174.041-T` named BOTH `174.047-T` and
Wave-8 `174.048-T` as green-makers with `green_maker_closes_wave: 8`. Because the selector greened
at wave 7 — one wave before its declared closure — P-002.6 fails closed. This rev11 amendment is a
Stage-owned plan/backlog-contract change ONLY: NO production/test code, NO status/dependency/
manifest/priority/checkpoint mutation, NO new task, NO new scenario group. `155-S` stays `active`;
`154-S` and PR #449 untouched; completed GREEN task `174.047-T` keeps `status: done`; the four
Ship-owned harness files are excluded.

**Mapping correction (authoritative live source: `174.041-T` red-deliverable-contract block).**
`green_maker_tasks` corrected `174.047-T, 174.048-T` → `174.047-T`; `green_maker_closes_wave`
corrected `8` → `7`. Rationale confirmed against the actual contracts:
* `174.047-T` (R9) AC (1) EXPLICITLY "turns recovery tests of R3 GREEN (non-subprocess)" — it is the
  green-maker that closes `^TestUR3_`, and it lands at wave 7.
* The `^TestUR3_` functions (`TestUR3_ReopenRollsBackInterruptedBlockFromCompletePreimage`,
  `..._ReopenRollsBackInterruptedUnblockAndRestoresExactBlockedPreimage`,
  `..._BranchBootstrapReopenRollsForwardUsingMachineReadableSnapshot`) invoke recovery IN-PROCESS
  via `NewWorkspace(...)` reopen; the subprocess mechanism only PRODUCES the crash state. All
  behavior they assert is supplied by R9 `174.047-T` recovery + normalizer, so the selector is fully
  green at wave 7.
* `174.048-T` (R10) authors its OWN separate subprocess crash/reopen integration tests (a distinct
  selector) and owns the rev8-relocated deterministic global-lock contention proof + SQLite
  sync/rebuild convergence + terminal audit; it is a GREEN task with NO red-deliverable-contract
  block of its own — already an explicit non-red-deliverable gate. It is therefore NOT a green-maker
  of `^TestUR3_`; listing it was the mapping error that produced the early-green block.

This SUPERSEDES the rev8 statement above ("`174.041-T`: green_maker `174.047-T/174.048-T`,
close-wave 8 ... are UNCHANGED"), which is retained as audit history. The correction is
wording/mapping ONLY: `174.041-T`'s `dependencies` (`174.052-T`), `status: done`, priority, parent,
and shipment membership are UNCHANGED, and the dependency GRAPH is UNCHANGED (`174.048-T` still
depends on `[174.041-T, 174.047-T]`). A wording/mapping correction alone fully expresses the true
order, so — per the fail-closed dependency rule — NO dependency edge is changed. A green selector is
NOT made artificially red.

**`174.048-T` obligation status: RETAINED IN FULL, NOT weakened.** Removing `174.048-T` from
`^TestUR3_`'s green-maker list drops no obligation. `174.048-T` continues to own, under its own
non-red-deliverable GREEN gate (its own subprocess selector): (a) subprocess crash/reopen recovery
for block/unblock/branch-bootstrap; (b) the deterministic shared workspace-global-lock CONTENTION
proof relocated from RED `174.041-T` in rev8; (c) SQLite sync/rebuild projection convergence;
(d) correlated TERMINAL audit evidence — all folded into its existing 3 scenarios (no new scenario).
A rev11 confirmation note is recorded on `174.048-T`.

**P-021 C1 classification of the three report-only P1s (all SAME-CONTRACT / in-scope for Ship; none
out-of-scope; no deferred-capture; no AC amendment required — ownership already unambiguous).**

1. **Claim / membership-writer serialization → SAME-CONTRACT.** Owner: R6 `174.044-T` (makes
   `ClaimShipment` share the SAME workspace-global lock; rev10 assigned the deterministic
   Claim/unblock contention proof GREEN-owned here) together with R7a `174.045-T` (routes the
   REMAINING bypass write call-sites — generic move/update, bulk/cascade, membership writes —
   through the governed writer, which acquires the same workspace-global lock). Serializing the
   membership writer against Claim is completed ENTIRELY by finishing the already-authorized shared
   workspace-global-lock routing on `174.044-T`/`174.045-T` — the exact same contract surface. No
   new task, no AC amendment.

2. **Recovery CAS/drift protection → SAME-CONTRACT.** Owner: R9 `174.047-T` (recovery reconciles
   durable intent + preimage UNDER THE SAME LOCKS, rolling forward/back; normalizer REFUSES when
   reconstruction cannot be proven) together with R6 `174.044-T` (exact-snapshot restore UNDER LOCK
   with refusal on CAS/drift; rev10 assigned the drift/CAS proof GREEN-owned here). CAS/drift
   protection during recovery is completion of the exact recovery-under-locks + CAS-refusal surface
   already authorized. No new task, no AC amendment.

3. **Missing normalizer MCP registry mapping → SAME-CONTRACT.** Owner: R9 `174.047-T` AC (3) "MCP
   normalizer parity present" ("Provide MCP normalizer parity"). The MCP tool
   `backlogit_normalize_blocked_shipment` IS implemented and registered in the Go MCP server
   (`internal/mcp/tools.go`), but the abstract operation is absent from
   `.autoharness/backlog-registry.yaml`, whose SIBLING lifecycle operations (`block_shipment`,
   `unblock_shipment`, `claim_shipment`, `add_to_shipment`) ARE mapped. Adding the
   `normalize_blocked_shipment` registry entry to match its siblings is completion of the exact "MCP
   normalizer parity" already authorized on `174.047-T` AC (3) — same contract surface. No new task,
   no AC amendment.

Because all three P1s are SAME-CONTRACT completions of existing tasks, NONE is a deferred scope
expansion and Stage captures NOTHING (per P-021: capture only when conclusively out-of-scope AND
Stage owns the operation). All three remain in-scope for Ship remediation within the cited tasks'
existing contracts.

**Scope confirmation — machine-readable scheduler fixture untouched (out of scope).** The
`tests/simulation/wave-scheduler-contract.json` fixture is a frozen scheduler-ALGORITHM self-test
whose `source.shipment` is `130-S` and which references ZERO `174.` tasks; it does NOT encode the
`155-S`/`174.041-T` mapping and is NOT the authoritative source for this correction. It was NOT
touched. The authoritative `155-S` red-deliverable mapping is the live `174.041-T` task block, the
only file corrected for the mapping.

**Review dispatch.** dispatch_mode `single-agent-declared-degradation`: this bounded
mapping-wording + classification amendment carries no production/test delta; a single reviewer
(Stage) verified read-only that the diff touches ONLY the `174.041-T` red-deliverable-contract
mapping fields + amendment prose + `updated_at`, a confirmation note + `updated_at` on `174.048-T`,
and this plan section — with no id/parent/status/priority/dependency/membership/scenario-count
change and no edit to any RED-deliverable block other than `174.041-T`'s two `green_maker_*` fields
and reason.

**Validation evidence (branch `feat/155-s-s14-resumable-shipment-blocked-lifecycle-status`):**
`backlogit sync` OK (**1555 artifacts, 0 parse failures**); `doctor --target` on `174.041-T` and
`174.048-T` both `ok: true` / `kind: pass` (exit 0); `docs lint` on this plan valid (0 violations);
shipment `155-S` manifest unchanged (**26 members**, `174-F` + 25 tasks); dependency graph
unchanged (`174.041-T→[174.052-T]`, `174.048-T→[174.041-T,174.047-T]`); `174.047-T` remains
`status: done`. The four Ship-owned harness files were not read as harness code, not edited, and are
excluded from this commit.

**Verdict: PASS** — residual P0 = 0, residual P1 = 0. The `^TestUR3_` red-deliverable now closes at
wave 7 via `174.047-T`, clearing `WAVE_RED_DELIVERABLE_EARLY_GREEN`; `174.048-T`'s obligations are
retained under its own non-red-deliverable GREEN gate; and all three report-only P1s are
SAME-CONTRACT completions in-scope for Ship (Claim/membership serialization → `174.044-T`/
`174.045-T`; recovery CAS/drift → `174.047-T`/`174.044-T`; normalizer registry mapping →
`174.047-T`). No new tasks, dependencies, members, scenario groups, or status changes.

**Ship-ready directive.** Advance `155-S` past the early-green gate: `^TestUR3_` is legitimately
GREEN at wave 7 (closed by `174.047-T`) and the corrected mapping now agrees. Then complete the
three SAME-CONTRACT P1s in place — (1) route the membership writer through the governed shared
workspace-global lock in `174.044-T`/`174.045-T`; (2) enforce recovery CAS/drift refusal in
`174.047-T`/`174.044-T`; (3) add the `normalize_blocked_shipment` mapping to
`.autoharness/backlog-registry.yaml` to match its block/unblock/claim siblings under `174.047-T`
AC (3) — and complete `174.048-T`'s retained post-implementation subprocess/contention/convergence/
audit verification under its own selector. No new backlog items are required.

<!-- plan-review-attempt: rev12-corrective-lock-order-inversion-closure -->

## Corrective Wave — Wave 11 (Lock-order inversion closure) (2026-09-23, branch `feat/155-s-s14-resumable-shipment-blocked-lifecycle-status`, HEAD `fcbed9fc`)

**Trigger.** The final bounded R1–R5/R8 shared-serialization remediation (uncommitted at review time:
10 files, 418 insertions / 20 deletions; focused regressions + mechanical gates green) introduced a
canonical `shipment-lifecycle-global` lock (`lockShipmentLifecycleGlobal` →
`lockShipmentMembership(ctx, ws, shipmentLifecycleGlobalLockID)`). `updateArtifactUngated`
(`internal/core/artifacts.go:523`) and the shipment-lifecycle / membership mutators acquire that
global lock BEFORE `lockArtifactMutations` — **global → artifact**. Pre-existing persistence callers
reach the lifecycle pending-recovery barrier while ALREADY holding artifact locks via
`UpdateArtifact → updateArtifactUngated → persistArtifact → persistArtifactWithLinkPolicyAndGuard` —
**artifact → global**. The two orders form an ABBA inversion that deadlocked the core suite at
`TestCheckChildrenTerminal_NonTerminalChild_ReturnsBlockingError` (suite timeout). Naively releasing
the global lock before the artifact locks breaks generic-writer serialization
(`TestP021ClaimSerialization_MembershipAndGenericWritersUseGlobalLock/generic_update_waits`). Ship
correctly halted with no scope expansion applied.

**Decision.** This is a SAME-CONTRACT mechanical consequence of R1 / shared serialization, but a NEW
separately-planned corrective work unit — the prior review-fix cycle is exhausted and all owning
tasks (`174.044-T`, `174.045-T`, `174.047-T`) are already `done`. Completed task statuses are NOT
reopened. Fresh WIT/metadata discovery confirmed `task` (level 2) sits directly under `174-F`
(feature, level 1); no subtask is required, so the smallest unit is a single task.

**New task.** `174.064-T` — *"Canonicalize shipment-lifecycle/artifact lock order; close
inverse-caller paths"* (`artifact_type: task`, `parent_id: 174-F`, `status: queued`,
`priority: high`; task priority enum max is `high`). Full AC (canonical hierarchy; inverse-path
closure; serialization preserved; deterministic deadlock regression; lock-order audit; focused +
full core + full-repo validation; rollback/diagnostics) and implementation-notes are carried in the
task artifact. Scope ≈ ≤2h; internal/core lock-ordering only — no decomposition needed.

**Shipment membership.** `174.064-T` was added to ACTIVE shipment `155-S` through the GOVERNED
`AddItemToShipment` path (`backlogit shipment add 155-S 174.064-T` → `{status: "added"}`). This is
permitted because `shipmentMutationBlocked` (shipment.go:1887) blocks membership mutation only for
`blocked`/`shipped`/`abandoned`/`archived` — an `active` shipment accepts governed additions. No
`custom_fields` were hand-edited; the shipment frontmatter `items` list and `updated_at` were
mutated only by the governed operation.

**Dependencies / order.** `174.064-T depends_on {174.044-T, 174.045-T, 174.047-T}` (typed `blocks`;
all three `done` → the task is immediately eligible). The edges encode that the corrective runs AFTER
the implementation that exposed the inversion — `174.044-T` (global-lock serialization owner),
`174.045-T` (generic-writer routing through the governed writer), `174.047-T` (pending-recovery
barrier under locks). As an unfinished member of `155-S`, `174.064-T` gates the shipment's
PR/ship readiness until `done`, so the corrective necessarily precedes final review/PR readiness. NO
existing dependency edge is changed and NO completed status is altered — the ordering is expressed by
adding one new queued node with three satisfied predecessors, the minimal graph change.

**Canonical hierarchy decision (for the implementer).** The single permitted order is
**global (`shipment-lifecycle-global`) → artifact (`artifact-mutation`)**. Inverse-caller closure
HOISTS the pending-recovery barrier / global acquisition ahead of the artifact locks on the
persistence path (`persistArtifact` / `persistArtifactWithLinkPolicyAndGuard` /
`recoverPendingShipmentOperations`); it does NOT release the global lock early, which would reopen
the `generic_update_waits` serialization gap. The global lock is acquired first AND held
continuously spanning artifact-lock acquisition as one nested critical section (no
acquire/release/reacquire window). Because `updateArtifactUngated` already holds the global lock
before `persistArtifact` re-enters the barrier's own `lockShipmentLifecycleGlobal`, the barrier's
global acquisition must detect prior ownership via a ctx-carried held-lock token and become a
verified no-op when already held (erroring rather than silently skipping if the token is absent
while contended) — closing the single-goroutine re-entrant self-acquire mode as well as ABBA.
Generic/membership/Claim writers continue to contend on the same global lock.

**Scope guard.** internal/core lock-ordering only; no shipment-lifecycle semantic change beyond
acquisition order. Ship's uncommitted R1–R5/R8 implementation files are PRESERVED and NOT reverted or
staged by Stage; `154-S`, PR #449, and the active checkpoint
`.backlogit/checkpoints/checkpoint-20260923-033438.json` (unresolved/active) are untouched.

<!-- plan-review-attempt: rev12-corrective-lock-order-inversion-closure -->

## Plan Review — Amendment (Wave 11 lock-order inversion corrective task) (2026-09-23)

dispatch_mode: multi-agent-dispatch
decision: PASS

**Reviewers dispatched (parallel, planning-artifact review of `174.064-T` ACs + this corrective-wave section):**

- **Concurrency Reviewer** — initial ADVISORY. Three P1 wording gaps: (a) re-entrancy seam unspecified
  (`updateArtifactUngated` holds global before `persistArtifact` re-enters the barrier's own
  `lockShipmentLifecycleGlobal` → single-goroutine self-reacquire risk); (b) "hoist the barrier"
  ambiguous on lock HOLD span (acquire/release/reacquire window reopens the inversion); (c) caller
  enumeration must be transitive (an inverse path is "reaches the barrier while holding an artifact
  lock"), not only direct acquisition sites. **All three closed** in AC2/AC4/AC5 (rev12 hardening):
  ctx-carried held-lock token + verified no-op + error-not-skip; global held continuously across
  artifact-lock acquisition as one nested critical section; transitive persist→barrier + P-021
  claim/generic-writer trace added to the audit.
- **Correctness Reviewer** — initial ADVISORY. Four P1 gaps: AC4(a) a "returns blocking error"
  functional test does not deterministically exercise ABBA (timeout ≠ deterministic failure); AC4(b)
  "proves no ABBA" unfalsifiable / can pass vacuously; AC1-vs-AC5 scope inconsistency ("anywhere" vs
  "7 files"); AC4↔AC7 detection mechanism not linked. **All four closed:** AC4 pass/fail signal is now
  the AC7 out-of-order-acquisition assertion (not a hang), `-race` + repeated iterations + explicit
  single-goroutine re-entrant case; AC5 now requires proving the enumerated set is EXHAUSTIVE for the
  global-lock handle (repo-wide search) or widening repo-wide; AC3 keeps `generic_update_waits` a
  positive wait assertion.
- **Scope Boundary Auditor** — **PASS**, zero P0/P1. One task, no subtask, ACs map 1:1 to the required
  elements, one new queued node with three satisfied `blocks` edges (no existing edge or completed
  status changed), governed `155-S` membership, commit pathspec limited to the four Stage-owned files.

**Residual P0/P1 after hardening: NONE.** All ADVISORY P1 findings were resolved in-scope by tightening
`174.064-T`'s own acceptance criteria (same corrective contract — no new tasks, no scenario-group or
dependency changes). Task doctor re-run PASS after each edit. Decision: **PASS**.

<!-- plan-review-attempt: rev12-corrective-lock-order-inversion-closure (PASS after ADVISORY→hardened) -->

<!-- plan-review-attempt: rev13-corrective-faultline-golden-lf-materialization -->

## Corrective Wave — Wave 12 (Faultline parity golden LF materialization) (2026-09-23, branch `feat/155-s-s14-resumable-shipment-blocked-lifecycle-status`)

**Trigger.** The mandatory local full suite for `155-S` final readiness is blocked by an
ENVIRONMENT-ONLY failure: pre-existing test
`internal/faultline/evidence_conformance_test.go:TestU4aBehaviorCanonicalByteStable` byte-compares
`a.Canonical()` LF output to `internal/faultline/testdata/parity_v1.golden.json`. The repo blobs are
LF, but Windows `core.autocrlf=true` materializes the fixture as CRLF because `.gitattributes` carries
only `* text=auto`. Test, fixture, and `.gitattributes` are unchanged by `155-S`; there is no Linux CI
evidence and no waiver. The same-operation full-suite circuit is OPEN — no test re-run or fourth
attempt was performed.

**P-021 C1 classification — OUT OF SCOPE (deferred-scope-expansion captured).** Fixing the fixture
byte-stability / line-ending materialization contract does NOT complete the exact change authorized
for `155-S` (resumable shipment blocked-lifecycle status + the Wave 11 lock-order closure). It is a
DIFFERENT contract surface owned by the archived Faultline golden-parity harness task `156.006-T`
under archived feature `156-F`. same-file / same-PR / same-subsystem do not make it in-scope. The
mandatory `DEFERRED SCOPE EXPANSION` capture was performed BEFORE any planning: stash `18E587A0`
(kind bug, provisional priority high) → deliberation `067-DL`.

**Ownership decision (fail-closed parent).** `156-F` and `156.006-T` are ARCHIVED (terminal). Hosting
a live queued corrective under a terminal feature would soft-reopen closed scope and risks
hierarchical-ID ambiguity against archived ordinals. The corrective is therefore parented to the
ACTIVE covering feature `174-F` (the covering feature of `155-S`), with explicit C1 provenance to
`156.006-T` recorded in the task body and here. This parallels the accepted sibling corrective
`174.064-T`.

**New task.** `174.065-T` — *"Pin LF materialization for Faultline parity golden fixture (155-S
full-suite unblock)"* (status `queued`, priority `high`, parent `174-F`). Scope: add a path-specific
`.gitattributes` entry `internal/faultline/testdata/parity_v1.golden.json text eol=lf` ONLY —
NO global `* text eol=lf`, NO change to the existing `* text=auto`, NO test weakening, NO edit to the
in-repo golden bytes. Acceptance criteria (AC0–AC8, hardened per rev13 review) require: a committed-blob
LF precondition pre-check (AC0, guarding byte-identity); the path-specific `eol=lf` entry placed AFTER
`* text=auto` (order-based precedence); a `git check-attr eol` proof resolving `lf`; a FORCED
working-tree re-materialization (`git rm --cached`/delete + `git checkout`, not `git add --renormalize`
alone — which updates only the index and no-ops when the blob is already LF) followed by a no-CRLF byte
assertion; a narrow adjacent-golden audit (fix only proven-defective paths); the targeted
`TestU4aBehaviorCanonicalByteStable` passing with the `156.006-T` byte-stability contract preserved; no
comparison-path normalization; the circuit disposition below; and a rollback/diagnostic record. Scope ≈
≤2h; config/attributes only — no decomposition needed.

**Shipment membership.** `174.065-T` was added to ACTIVE shipment `155-S` through the GOVERNED
`AddItemToShipment` path (`backlogit shipment add 155-S 174.065-T` → `{status: "added"}`). Active
shipments are not membership-mutation-blocked (`shipmentMutationBlocked` excludes `active`);
`validateShipmentItemIDs` imposes no covering-feature-descendant requirement, so an explicit
single-task add of a `174-F`-parented task is governed-permitted. The covering feature `174-F` was
already a member; no parent feature was newly added and `custom_fields` was NOT hand-edited.

**Dependencies / order.** No dependency-graph change (fail-closed). Unlike the lock-order corrective
`174.064-T` — which legitimately depends on foundational serialization tasks it must follow — the
LF-materialization fix is a standalone config/attributes change with NO upstream implementation task;
adding a `blocks` edge onto already-done tasks would encode a false ordering and satisfy immediately,
providing no real gate. As an UNFINISHED member of active `155-S`, `174.065-T` already gates the
shipment's final readiness / PR readiness via membership (the same membership-as-gate mechanism 155-S
uses for all members). This is the minimal correct encoding and preserves the dependency graph
unchanged.

**Circuit disposition (KEY — do NOT run the full suite now).** The same-operation full-suite circuit
is OPEN against the PRE-FIX repository state. This corrective task, when implemented and COMMITTED by
Ship, CHANGES repository state (adds the `.gitattributes` entry + renormalizes the fixture in the
working tree). After that correction commit, the local full-suite invocation is a NEW,
SEPARATELY-AUTHORIZED final-gate operation at a NEW commit / NEW workflow phase — it is NOT a retry or
probe of the open pre-fix failing state, and does not count against the pre-fix circuit. Explicit
operator authorization is still required immediately before that post-fix full-suite run if policy
demands it. Ship MUST NOT run the full suite as part of this task's RED/GREEN beyond the targeted
`TestU4aBehaviorCanonicalByteStable`. No waiver is invented; if policy forbids ever running the full
suite even after a state-changing correction, the compliant alternative is a targeted per-package
verification of `./internal/faultline/` plus the affected packages at the new commit under explicit
authorization — never a skipped mandatory gate.

**Scope guard.** Faultline golden LF materialization only; no shipment-lifecycle, lock-order, or
product-code change; no test/source/fixture/`.gitattributes` edit under THIS Stage amendment (Ship
implements the `.gitattributes` change later under `174.065-T`). The active Ship checkpoint
`checkpoint-20260923-191544.json`, `154-S`, and PR #449 are untouched; the uncommitted Ship
implementation files are excluded from the Stage commit (explicit pathspec).

<!-- plan-review-attempt: rev13-corrective-faultline-golden-lf-materialization -->

## Plan Review — Amendment (Wave 12 Faultline golden LF materialization corrective task) (2026-09-23)

dispatch_mode: multi-agent-dispatch
decision: PASS

**Reviewers dispatched (parallel, planning-artifact review of `174.065-T` ACs + the Wave 12 corrective-wave section):**

- **Scope Boundary Auditor** — **PASS**, zero P0/P1/P2. Confirmed minimal scope (single path-specific
  `.gitattributes` line; no global change; no test/golden weakening), verifiable ACs, and that the
  "no dependency-graph change / membership-as-gate" decision is the anti-scope-creep choice (a `blocks`
  edge onto already-done tasks would encode a false, immediately-satisfied ordering) rather than a
  verification gap. One P3 advisory: keep AC4's adjacent-golden audit strictly limited to
  proven-defective faultline paths (no preemptive `eol=lf` on non-defective goldens). Encoded into AC4.

- **Correctness Reviewer** — **PASS**. Verified the failure diagnosis is technically correct
  (`canonical.Canonicalize` emits a single trailing LF; the raw `bytes.Equal` compare in
  `evidence_conformance_test.go` has no normalization; `.gitattributes` carries only `* text=auto`), and
  that `text eol=lf` is the correct and sufficient forcing mechanism that overrides `* text=auto`. One
  P2: `git add --renormalize` alone does NOT reliably rewrite the WORKING TREE (index-only; no-op when
  the blob is already LF; a plain `git checkout` may skip re-smudging) — the AC must force
  re-materialization. Two P3 advisories: place the path-specific entry AFTER `* text=auto`
  (order-based precedence), and pre-check that the committed blob is LF before renormalize (guards
  byte-identity). **All three resolved in-scope** by hardening `174.065-T`'s own ACs: new AC0
  (blob-LF precondition pre-check), AC1 ordering requirement (after `* text=auto`), and AC3 rewritten to
  force re-materialization (`git rm --cached`/delete + `git checkout`, not `--renormalize` alone) with
  the no-CRLF byte assertion retained as a falsifiable backstop.

**Residual P0/P1 after hardening: NONE** (P0=0, P1=0). The one P2 and both P3 findings were resolved
in-scope by tightening `174.065-T`'s own acceptance criteria — same corrective contract; NO new task,
scenario group, dependency edge, status, priority, or membership change. Task `doctor` re-run PASS after
the AC edits (see validation evidence). Decision: **PASS**.

**Ship-ready directive.** Implement `174.065-T` as the final corrective before `155-S` PR readiness:
add the single path-specific `.gitattributes` `eol=lf` entry (after `* text=auto`), force-re-materialize
and prove no-CRLF, run ONLY the targeted `./internal/faultline/ -run TestU4aBehaviorCanonicalByteStable`.
Do NOT run the mandatory full suite as part of this task. The post-fix full-suite run is a NEW
final-gate operation at the new (post-correction) commit — NOT a retry of the open pre-fix circuit — and
requires explicit operator authorization immediately before it if policy demands. Preserve the
uncommitted R1–R5/R8 Ship implementation and the `174.064-T` lock-order work; this corrective is
orthogonal and touches only `.gitattributes` + working-tree materialization of the one golden.

<!-- plan-review-attempt: rev13-corrective-faultline-golden-lf-materialization (PASS after ADVISORY→hardened) -->

---

## Wave 13 — Corrective: isolate shipped-event durability tests from the real gate/version subprocess (2026-09-23)

**Task:** `174.066-T` (queued, high) under active covering feature `174-F`; governed member of active
shipment `155-S` (added after `174.064-T`, `174.065-T`). Origin: P-021 deferred-scope-expansion stash
`FF1F3AC7` → deliberation `068-DL`.

**Trigger.** The authorized full suite `go test ./...` fails and then times out in UNCHANGED
`internal/core` durability tests. First failure:
`TestShipShipment_FailClosedShippedAppendSuppressesMoveStatusPostHook`
(`internal/core/shipment_shipped_event_durability_test.go:316`); the package then times out at 10m in
`TestShipShipment_ShippedEventAppendFailureLogsFixedShape/not_applied`. Terminal stack:
`gate.ExecVersionRunner.Version → gate.Probe → Broker.Evaluate → gateShipmentCompletion → ShipShipment`.

**Root cause (test-infrastructure, not product).** `newShipDurabilityFixture` builds its workspace via
`setupShipmentWorkspace → NewWorkspace`, which wires the REAL `buildGateBroker`
(`gate.Broker{Version: gate.ExecVersionRunner{...}}`, workspace.go:278 / gate_transition.go:71-76) and
never overrides it. `shipWithWatchdog` then runs `ShipShipment(context.Background(), …)` UNBOUNDED in a
goroutine, so `gateShipmentCompletion` spawns a real `autoharness version` subprocess whose latency,
under a loaded parallel full-suite run, exhausts the 10m package deadline. This is a gate/version
subprocess-isolation defect in the test harness, NOT the item-log/artifact/global-lock ordering surface
owned by `174.064-T`.

**P-021 C1 classification.** OUT OF SCOPE for `174.064-T` (lock-order canonicalization) — a different
contract surface (gate/version subprocess isolation vs. artifact/global lock ordering) — yet it BLOCKS
`174.064-T` AC6 / the full-repository gate, so it is a governed prerequisite for `155-S` final readiness.
Disposition: captured as `DEFERRED SCOPE EXPANSION` (`FF1F3AC7`), deliberated (`068-DL`), and planned as
its own minimal corrective task rather than folded into any completed task.

**Chosen correction (test-only; no production change).** Use the PRE-EXISTING injection seam — the public
`ws.GateBroker` field, the `gate.VersionRunner` / `gate.GateRunner` interfaces, and the
`injectBroker` / `fakeVersion` / `fakeGateRunner` helpers already in `gate_transition_test.go`:

1. Override the durability fixture's `ws.GateBroker` with a PASSING fake broker (fake version + fake gate
   runner) using the `gate.EnabledMode` that preserves the existing ship-completion path, so no
   `gate.ExecVersionRunner` subprocess is ever spawned and every durability assertion still runs.
2. Propagate a BOUNDED context (`context.WithTimeout`/`WithDeadline` + `defer cancel`) into `ShipShipment`
   and the ship goroutine in `shipWithWatchdog`, instead of `context.Background()`, so cancellation
   propagates into the ship path rather than relying solely on the package deadline.
3. Deterministically PROVE no `gate.ExecVersionRunner` invocation (call-recording fake and/or type
   assertion on `ws.GateBroker.Version`), preserve ALL append/compensation/post-hook-suppression/
   fixed-shape assertions verbatim, and demonstrate RED-before / GREEN-after on the targeted durability
   suite. No timeout inflation, no `t.Skip`, no weakened assertions, no production gate bypass.

Scope is confined to `internal/core/shipment_shipped_event_durability_test.go` (and, only if strictly
necessary, a sibling `_test.go` helper reusing the existing seam). No production (`non-_test.go`) change
is required; if one is discovered to be required, Ship MUST HALT and classify/bound it explicitly rather
than silently expand.

**Dependencies / ordering.** No dependency-graph edge is added (fail-closed): the standalone
test-harness corrective has no authentic upstream implementation task, and a `blocks` edge onto the
already-done tasks would encode a false, immediately-satisfied ordering. Ordering is enforced by
MEMBERSHIP — `174.066-T` is an unfinished member of active `155-S`, which gates PR readiness until it is
done. No completed task status is reopened.

**Circuit disposition (full-suite same-operation circuit is OPEN).** Do NOT run `go test ./...` now.
After the correction COMMIT changes repository state, the local full-suite invocation is a NEW,
SEPARATELY-AUTHORIZED final-gate operation at a NEW commit / NEW workflow phase — NOT a retry or probe of
the open pre-fix failing state, and it does not count against the pre-fix circuit. Explicit operator
authorization is still required immediately before that post-fix full-suite run if policy demands it.
Within this task Ship runs ONLY the targeted durability verification (e.g.
`go test -run TestShipShipment_ ./internal/core`). No waiver is invented; if policy forbids ever running
the full suite even after a state-changing correction, the compliant alternative is a targeted
per-package verification of `./internal/core/` plus the affected packages at the new commit under
explicit authorization — never a skipped mandatory gate.

**Scope guard.** Durability-test gate isolation only; no shipment-lifecycle, lock-order, product-code, or
`.gitattributes` change; no touch of the current uncommitted Ship implementation, the active checkpoint
`checkpoint-20260923-231731.json`, `154-S`, or PR #449. The uncommitted Ship implementation files are
excluded from the Stage commit (explicit pathspec).

<!-- plan-review-attempt: rev14-corrective-durability-gate-isolation -->

## Plan Review — Amendment (Wave 13 durability-test gate-isolation corrective task) (2026-09-23)

dispatch_mode: multi-agent-dispatch
decision: PASS

**Reviewers dispatched (parallel, planning-artifact review of `174.066-T` ACs + the Wave 13 corrective-wave section):** Concurrency Reviewer, Correctness Reviewer, Scope Boundary Auditor.

- **Correctness Reviewer — initial FAIL (P1), re-review PASS after hardening.** Traced the real gate broker, `gateShipmentCompletion`, `validateMemberGateEvidence`, and the durability fixture and found the original AC premise INVERTED: it directed an ENFORCED+PASSING fake broker, but the fixture's release-scope members are active and carry NO per-member gate-pass evidence and `formalGateEnforced()` is false, so an enforced broker makes `gateShipmentCompletion` run `validateMemberGateEvidence` and REFUSE the ship BEFORE the `ws.shipmentEventAppend` seam — short-circuiting the ship and breaking every durability assertion. The behavior-preserving configuration is a NOT-enforced / fail-open broker (`gate.EnabledAuto` + version-probe error, or `gate.EnabledFalse`), matching the current default-config CI `!ev.Enforced` early-return. **Resolved in-scope** by rewriting AC1 (not-enforced/fail-open, explicitly forbidding enforced+passing), plus P2 fixes: AC2 (assert `formalGateEnforced()==false` precondition), AC5 (prove the post-hook-suppression scenario's error is the shipped-event-append `MutationPartialError` via `requireShippedAppendPartial`/`errors.As`, closing a wrong-path masking gap), AC6 (structural TYPE ASSERTION on `ws.GateBroker.Version`/`.Runner` rather than an invocation spy that would pass vacuously under `EnabledFalse`), and P3 reframes (AC3 "600s-bounded + leaks" precision; AC7 pre-fix RED is a bounded timeout, not a clean assertion failure). Re-review verdict: **PASS** — the not-enforced direction is behavior-preserving under both `EnabledAuto` and `EnabledFalse`, the masking gap is closed, the type-assertion proof is sound, and no new correctness defect was introduced. Two residual P3 advisories (AC8 fail-open/kill-switch wording; AC3 bound must exceed the pre-Evaluate `ws.headSHABounded` git probe) were also folded into the ACs.

- **Concurrency Reviewer — ADVISORY, resolved in-scope.** Confirmed AC1 (fake broker) is the primary and sufficient fix and AC3 (bounded ctx) is correct defense-in-depth (`ExecVersionRunner.Version` honors `exec.CommandContext(ctx)`), with AC6's proof deterministic (per-`ws` broker, no `t.Parallel`). P2/P3 hardening folded in: AC3 bound strictly < watchdog and generous/unobservable on the happy path (prevents `DeadlineExceeded` from mutating `MutationPartialError.Class`/`CompensationState`); AC6 recorder concurrency-safe and read only after channel receive (avoids a `-race` hazard on a leaked goroutine), `-race` recommended for AC7; AC1 injection ordering pinned (after construction, before goroutine launch); AC9 explicitly acknowledges the residual leaked-goroutine/use-after-close on a GENUINE lock regression as a known OUT-OF-SCOPE limitation whose full fix (cooperative ctx cancellation inside `ShipShipment`) is a production change.

- **Scope Boundary Auditor — ADVISORY, resolved in-scope.** Affirmed strong anti-creep scope (single `_test.go` file, pre-existing seam reuse, verbatim-assertion preservation, HALT-on-production-change) and that the no-dependency-edge / membership-as-gate decision is the correct anti-scope-creep choice (a `blocks` edge onto already-done tasks is a vacuous, immediately-satisfied ordering), not a verification gap. P2/P3 hardening folded in: AC6 broadened to both seam interfaces (VersionRunner AND GateRunner); AC9 makes introducing any NEW production injection API OUT OF SCOPE BY DEFINITION (HALT, not "bound"); AC3 framed as defense-in-depth; AC7 accepts a bounded RED reproduction; AC4 scoped "verbatim" to assertion SEMANTICS so it does not forbid AC3's context threading. Plan-level clarification (this record): the fail-closed membership-as-gate control is INDEPENDENTLY OBSERVABLE — `174.066-T` is an unfinished member of active `155-S`, and shipment membership is what actually blocks `174.064-T` AC6 / the full-repository gate for `155-S` final readiness — so the absent dependency edge is compensated by an asserted control, not by assumption.

**Residual P0/P1 after hardening: NONE** (P0=0, P1=0). The single P1 (correctness AC1 inversion) and all P2/P3 findings were resolved in-scope by tightening `174.066-T`'s own acceptance criteria — same corrective contract; NO new task, scenario group, dependency edge, status, priority, or membership change. Decision: **PASS**.

**Ship-ready directive.** Implement `174.066-T` as a governed prerequisite for `155-S` final readiness, confined to `internal/core/shipment_shipped_event_durability_test.go` (+ a sibling `_test.go` helper only if strictly needed) using the PRE-EXISTING seam — NO production change:

1. Override the durability fixture `ws.GateBroker` with a NOT-enforced / fail-open fake broker (`gate.EnabledAuto` + a `fakeVersion` reporting the gate binary unavailable, or `gate.EnabledFalse`) via `injectBroker`, injected after `NewWorkspace` construction and before `shipWithWatchdog` launches the ship goroutine. Do NOT use an enforced+passing broker (it would trip `validateMemberGateEvidence` and refuse the ungated members). Assert `formalGateEnforced()==false`.
2. Replace `context.Background()` in `shipWithWatchdog` with a bounded context (bound strictly < watchdog, generous vs. the append/compensation window AND the pre-Evaluate `ws.headSHABounded` git probe, unobservable on the happy path).
3. Prove no real broker via a TYPE ASSERTION on `ws.GateBroker.Version` and `.Runner`; close the post-hook-suppression masking gap by asserting the error is the shipped-event-append `MutationPartialError`; keep any recorder concurrency-safe and read only after channel receive.
4. Preserve every append/compensation/post-hook-suppression/fixed-shape assertion by semantics. RED-before (bounded timeout reproduction acceptable) / GREEN-after on `go test -run TestShipShipment_ ./internal/core` (recommended `-race`).
5. Do NOT run `go test ./...` now — the full-suite same-operation circuit is OPEN. After the correction commit, the full suite is a NEW separately-authorized final-gate operation at a new commit/phase requiring explicit operator authorization; if policy forbids it even post-correction, use targeted per-package verification (`./internal/core` + affected packages) under explicit authorization. Preserve the uncommitted R1–R5/R8 Ship implementation, the `174.064-T` lock-order work, the active checkpoint `checkpoint-20260923-231731.json`, `154-S`, and PR #449; this corrective is orthogonal (test-harness gate isolation only). If any production (`non-_test.go`) change is found necessary, HALT and classify — introducing a new production injection API is out of scope by definition.

<!-- plan-review-attempt: rev14-corrective-durability-gate-isolation (PASS after correctness FAIL->hardened) -->

<!-- plan-review-attempt: rev15-corrective-final-review-blockers -->

## Wave 14 — Corrective: final-review consensus blockers (lock hierarchy + Windows directory TOCTOU) (2026-09-23)

At HEAD 5e4a04ca (`go test ./...` passed once at the corrected state), standard + adversarial final review blocked on three consensus-backed same-contract completion defects. Wave 14 splits them by domain under the 2-hour rule into two atomic corrective tasks, both governed members of active 155-S. Implementation commit provenance: 4d8a08fd; task-completion HEAD 5e4a04ca; active checkpoint checkpoint-20260924-011723.json.

### 174.067-T — Lock hierarchy: singular-writer barrier + Reconcile/Add inversion (concurrency domain)

Closes:
- R-A (HIGH/P1, 3/3): `internal/core/shipment.go:1293 lockArtifactMutation` lets singular typed writers mutate lifecycle members without the pending-intent recovery barrier that the plural `lockArtifactMutations` (:1304-1308) acquires.
- R-B (MEDIUM/P1, 2/3): `internal/core/shipment.go:1303-1308 lockArtifactMutations` context; `ReconcileShipmentToShipped` may hold membership/item-log locks before global, opposing `AddItemToShipment`'s global->membership order.

Objective closure conditions (consensus-hardened): R-A closed at the `persistArtifactWithLinkPolicyAndGuard` barrier-gate (broaden the `ArtifactType=="shipment"` gate to cover lifecycle members, preserving `ctx=lockedCtx` token propagation + reentrancy held-check) — NOT unconditionally in the generic `lockArtifactMutation` helper (would over-serialize all writes, run recovery on every write, and risk a `validateShipmentLifecycleGlobalReentry` fail-closed break); R-B closed by `reconcileShipmentToShippedImpl` Phase A / `lockShipmentReconcileCThenB` acquiring global before membership/item-log (extended global hold across reconcile Phase C is intentional); GLOBAL-FIRST invariant enforced on the implicated paths only (item-log↔artifact relative order is NOT constrained on the snapshot/ship path, dominated by the barrier); scoped lock-order audit; deterministic singular-member-wait test (targets a NON-shipment member to avoid a vacuous RED) and lock-layer Reconcile-vs-Add contention test (under -race); existing generic/membership-writer serialization (TestP021ClaimSerialization...) preserved; no NEW runtime lock-order detector. Scope (whitelisted): `internal/core/shipment.go` (persist barrier-gate) + `internal/core/shipment_reconcile_transaction.go` (Phase A) + `internal/core/shipment_reconcile_lock.go` (CThenB) + exact tests; no new production lock primitive (HALT+classify otherwise).

### 174.068-T — Windows shipment-ops directory-replacement TOCTOU (Windows filesystem containment domain)

Closes:
- R-C (MEDIUM/P1, 2/3): `internal/core/shipment_ops_windows.go:54` (and the read/write/remove helpers at :56/:79/:147) validate a directory handle then DISCARD it (`_ *os.File`) and re-resolve absolute paths for read/write/remove, leaving a directory-replacement TOCTOU.

Objective closure conditions (consensus-hardened): the validated directory handle is threaded through and USED via OBJECT-BOUND, RootDirectory-relative `NtCreateFile`/`NtOpenFile` (no-follow) for read/create-temp/remove AND a handle-relative `SetFileInformationByHandle` FILE_RENAME_INFO+RootDirectory atomic rename REPLACING the pathname `MoveFileEx` (the literal MoveFileEx leaves the rename target swappable — a fail-open payload leak — and is jointly unsatisfiable with the handle-relative requirement); no absolute-path re-resolution and no post-hoc re-open+identity-compare substitute; fail closed (ErrValidation) on directory-identity change; handle stays live across each dependent op incl. the rename commit; all other containment + durability invariants preserved (durability via temp fsync-before-rename + atomic replace-if-exists); Windows-guarded adversarial tests that swap the ops dir by OBJECT IDENTITY while preserving the canonical path (not a mere external reparse point, which existing checks already catch) for read, remove, AND the temp-create→rename window (asserting payload bytes never reach the swapped-in directory). Scope: `internal/core/shipment_ops_windows.go` + Windows-only tests; the minimal NT-native RootDirectory-relative primitives (`NtCreateFile`/`NtOpenFile`, `SetFileInformationByHandle` FILE_RENAME_INFO+RootDirectory) are EXPLICITLY AUTHORIZED as in-scope (parity with the Unix `Openat`/`Renameat` contract); any OTHER new production seam or cross-platform change is out of scope (HALT+classify).

### Shared Wave 14 closure / circuit disposition (NON-NEGOTIABLE)

- Dependencies: no `blocks` edge on either task; both are governed members of active 155-S and gate 155-S final readiness by membership-as-gate. Provenance link to 174.064-T (lock-order) is informational, not an ordering edge onto a done task.
- No completed task statuses are reopened.
- The same-operation full-suite circuit is OPEN. Neither task runs `go test ./...`. After BOTH corrective commits (174.067-T and 174.068-T) land, the full suite runs EXACTLY ONCE as a NEW separately-authorized final-gate operation at a new commit/phase (explicit operator authorization required immediately before it). Compliant alternative if a full-suite run is forbidden even post-correction: targeted per-package verification (./internal/core plus affected packages) under explicit authorization — never a skipped mandatory gate.
- Review is NOT open-ended: after the shared post-both-commits verification, exactly ONE standard review + ONE adversarial review over ONLY the changed surfaces of the two tasks. Exact stop gate: zero P0/P1. No further review-fix cycles are authorized by this amendment beyond closing genuine P0/P1 within those changed surfaces.

## Plan Review — Amendment (Wave 14 final-review-blockers corrective) (2026-09-23)

<!-- plan-review-attempt: rev15-corrective-final-review-blockers -->

dispatch_mode: multi-agent-dispatch
decision: PASS

Scope of amendment: two NEW atomic corrective tasks created under 174-F and governed-added to active shipment 155-S, closing the three consensus-backed final-review blockers at HEAD 5e4a04ca. No existing task status/dependency/priority/membership altered; no source/test/checkpoint mutation; 154-S and PR #449 untouched.

- 174.067-T (Task A, concurrency/lock hierarchy) — closes R-A (HIGH/P1 3/3) + R-B (MEDIUM/P1 2/3).
- 174.068-T (Task B, Windows filesystem containment) — closes R-C (MEDIUM/P1 2/3).

Reviewer dispatch (4 personas, parallel; then targeted re-dispatch of the 2 FAILs over hardened ACs):
- Concurrency Reviewer (Task A): ADVISORY -> hardened. R-A fix relocated from an unconditional `lockArtifactMutation` acquire to the `persistArtifactWithLinkPolicyAndGuard` barrier-gate (preserves ctx=lockedCtx reentrancy-token propagation; avoids over-serializing all writes and a `validateShipmentLifecycleGlobalReentry` fail-closed break). R-B fix whitelisted to `shipment_reconcile_transaction.go` Phase A + `shipment_reconcile_lock.go` (CThenB). Canonical invariant narrowed to GLOBAL-FIRST on implicated paths; no new runtime lock-order detector.
- Scope Boundary Auditor (both): ADVISORY -> hardened. Non-vacuous RED (singular test targets a NON-shipment member); scope pathspecs pinned; NT-native primitive authorization made explicit rather than an unbounded "new API".
- Correctness Reviewer (both): FAIL -> PASS. Prior FAIL: Task B AC1/AC2 (handle-relative) vs old AC3 (retain literal MoveFileEx) were jointly UNSATISFIABLE. Resolved: MoveFileEx replaced by handle-relative SetFileInformationByHandle FILE_RENAME_INFO+RootDirectory (Windows Renameat parity); durability via temp Sync()-before-rename; AC5 ErrValidation coupling relaxed to the object-binding security invariant (payload never reaches the swapped-in dir).
- Security Reviewer (Task B): FAIL -> PASS. Prior FAIL: AC7 forbade the very NT-native RootDirectory-relative primitives required to close the same-canonical-path window; AC1 allowed a re-open-then-verify-identity (check-after-use) substitute; AC5 tested only a reparse-to-external swap already caught vacuously. Resolved: AC7 explicitly authorizes the minimal NtCreateFile/NtOpenFile(RootDirectory) + SetFileInformationByHandle(FILE_RENAME_INFO,RootDirectory) primitives and documents the deliberate divergence from shipment_reconcile_fs_windows.go; AC1 strikes the verify-after substitute; AC5 tests a canonical-path-preserving object-identity swap covering read, remove, AND the temp-create->rename window.

Residual P0/P1: NONE. Remaining reviewer items (Correctness P2 AC5 error-coupling; Security P3 handle-relative stat) were encoded into 174.068-T AC1/AC4/AC5/AC6.

Ship-ready directive:
1. Implement 174.067-T (test+code): apply R-A at `persistArtifactWithLinkPolicyAndGuard` (broaden the `ArtifactType=="shipment"` barrier gate to lifecycle members, preserving reentrancy token + held-check); apply R-B by making `reconcileShipmentToShippedImpl` Phase A / `lockShipmentReconcileCThenB` acquire the global barrier before membership/item-log. Add the deterministic singular-member-wait test (target a non-shipment member) and the -race Reconcile-vs-Add contention test. Preserve TestP021ClaimSerialization... . No new lock primitive/detector. Files: shipment.go + shipment_reconcile_transaction.go + shipment_reconcile_lock.go + exact tests.
2. Implement 174.068-T (test+code): thread the validated ops directory handle through read/stat/create-temp/write/rename/remove via RootDirectory-relative NtCreateFile/NtOpenFile (no-follow) and REPLACE MoveFileEx with a handle-relative SetFileInformationByHandle FILE_RENAME_INFO+RootDirectory rename; add the Windows-guarded object-identity-swap TOCTOU tests (read, remove, temp-create->rename window). File: shipment_ops_windows.go + Windows-only tests.
3. Circuit disposition (shared): do NOT run `go test ./...` mid-task (same-operation circuit OPEN). After BOTH corrective commits land, run the full suite ONCE as a NEW separately-authorized final-gate operation (explicit operator authorization immediately before it); compliant alternative is targeted per-package verification. `go test ./...` passed once at HEAD 5e4a04ca before final review blocked.
4. Stop gate: after both commits + the single authorized full-suite pass, exactly ONE standard + ONE adversarial review over ONLY the changed surfaces of both tasks; ship-ready at zero P0/P1.
5. No dependency edges added (both are governed members of active 155-S; membership-as-gate blocks final readiness). Provenance link to done 174.064-T is informational only.

<!-- plan-review-attempt: rev15-corrective-final-review-blockers (PASS after 2 FAIL->hardened) -->

## Plan Review — Amendment (Wave 14 rev16: Windows rename API-boundary correction) (2026-09-23)

<!-- plan-review-attempt: rev16-windows-rename-api-boundary -->

dispatch_mode: multi-agent-dispatch
decision: PASS

Scope: surgical API-boundary correction to 174.068-T AC2/AC7 only. No graph/status/dependency/membership/priority change; no source/test/checkpoint mutation; 154-S and PR #449 untouched. 174.068-T remains active/high, governed member of 155-S.

Correction (read-only diagnosis by Ship): the Win32 `SetFileInformationByHandle(FileRenameInfo`, class 3`)` form rejects a non-null-RootDirectory relative rename with `ERROR_INVALID_PARAMETER` on this platform. Everything else in the handle-relative design is already correct (buffer/layout, source DELETE access, share flags, directory handle, UTF-16 byte length without terminator, no-follow source open). The information-class/API pairing is the only defect.

Amendment: AC2 now mandates `golang.org/x/sys/windows.NtSetInformationFile` with the native `FileRenameInformation` class (value 10) over the EXISTING `FILE_RENAME_INFORMATION`-compatible buffer and the live validated RootDirectory handle, converting NTSTATUS through the existing Windows error path. AC7 swaps the authorized rename primitive accordingly. Both ACs EXPLICITLY FORBID: `FileRenameInfoEx`/`FileRenameInformationEx` (any *_EX class), POSIX-semantics rename flags / class 65, `MoveFileEx`, absolute-path re-resolution, and any new syscall declaration or third-party dependency (`NtSetInformationFile` already exists in `x/sys/windows`).

Preserved unchanged: AC1, AC3–AC6, AC8; all security properties (object-binding, no verify-after substitute, fail-closed-or-genuine-object invariant), the adversarial TOCTOU tests (read/remove/rename-window object-identity swap), durability (temp `Sync()`-before-rename + atomic replace-if-exists), rollback/circuit disposition. Sibling `shipment_reconcile_fs_windows.go` divergence rationale preserved.

Focused review: Security PASS (native NT rename over the RootDirectory-relative buffer closes the same-canonical-path window the Win32 form left ERROR_INVALID_PARAMETER-blocked; no *_EX/POSIX/MoveFileEx/absolute-path fallback reintroduced; no post-hoc verify-after substitute). Correctness PASS (AC2/AC7 now name an API/class pairing that accepts the non-null RootDirectory buffer, so the contract is satisfiable; buffer, share flags, DELETE access, and no-follow open preserved as-is; NTSTATUS routed through the existing error path — no new error surface). Residual P0/P1: NONE.

Ship-ready directive: in `internal/core/shipment_ops_windows.go`, replace the `SetFileInformationByHandle(FileRenameInfo)` rename call with `NtSetInformationFile(FileRenameInformation, class 10)` using the same buffer + live RootDirectory handle; convert NTSTATUS via the existing Windows error path; keep the open primitives (`NtCreateFile`/`NtOpenFile`, no-follow) and every other invariant unchanged. Verification stays circuit-gated: no tests now; one post-change targeted Windows verification phase requires explicit operator authorization immediately before it, then the shared Wave 14 full-suite/stop-gate (AC8) applies.

<!-- plan-review-attempt: rev16-windows-rename-api-boundary (PASS) -->

## Wave 15 — Class-level ShipShipment gate-fixture isolation (155-S full-suite unblock) (2026-09-23)

<!-- plan-review-attempt: rev17-classlevel-gate-fixture-isolation -->

Corrective wave adding ONE class-level, test-only task closing the full-suite timeout root cause. Source stash DB48A817; deliberation 069-DL. No graph/status/priority/membership change to existing tasks; no production/config/default/API change.

- 174.069-T (queued/high, under 174-F, governed member of active 155-S) — default GateBroker=nil across shared ShipShipment fixture families so ordinary internal/core tests stop inheriting the real ExecVersionRunner/ExecRunner gate subprocess.

Root cause: 11 of 34 internal/core ShipShipment tests inherit the real default GateBroker via setupShipmentWorkspace or external setupTestWorkspace; latest full-suite failure shipment_test.go:TestShipShipment_RestoresNonMemberFeatureEvenWhenShipFailsAfterRollup times out on the real gate version subprocess.

Objective closure conditions: shared test-only disableExecGateForTest(t,ws) sets/asserts GateBroker=nil, defaulted in setupShipmentWorkspace and newGateTestWorkspace; intentional gate tests explicitly inject fakes afterward and assert no Exec runners; durability fail-open fake fixture/assertions preserved (touched only if consolidating structural assertions); local external archive wrapper for 025_archive_harness_test.go nils/asserts the broker without modifying broad core_test.setupTestWorkspace; inventory regression proves no ordinary ShipShipment fixture retains exec types while intentional gate tests remain behaviorally covered; no skipped tests / timeout inflation / weakened assertions / production changes. Scope: internal/core test files only. P-021 C1 OUT OF SCOPE re done 174.066-T (shared cross-family infra, not durability-only completion).

Circuit disposition (shared): do NOT run go test ./... within this task (same-operation full-suite circuit OPEN). After the corrective commit lands, run the full suite ONCE as a NEW separately-authorized final-gate operation (explicit operator authorization immediately before it); compliant alternative is targeted per-package verification (ShipShipment/gate/durability/archive suites + race). Then 155-S final review/stop-gate (zero P0/P1) applies.

Dependencies: none added (governed member of active 155-S; membership-as-gate blocks final readiness). Provenance link to done 174.066-T is informational.

## Plan Review — Amendment (Wave 15 GateBroker-nil test isolation) (2026-09-23)

dispatch_mode: multi-agent-dispatch
decision: PASS

Focused two-reviewer dispatch over corrective task 174.069-T (class-level, test-only gate-fixture isolation) ACs and the Wave 15 plan section.

- Correctness Reviewer: PASS. Verified against ground-truth code that gateShipmentCompletion nil-guards ws.GateBroker (skips the gate, returns nil — no nil-panic), NewWorkspace wires the real buildGateBroker only when the gate config is enabled, and intentional gate/durability fixtures re-inject fakes AFTER construction so nil defaulting is harmless. AC3 preserves behavioral gate coverage; AC4 leaves the durability fail-open fixture intact; AC6 inventory invariant is consistent with the gate-test exemption. Two P3 advisories applied (AC5 names the actual ShipShipment caller; AC6 asserts absence-of-real-Exec-types rather than strict nil so the durability fake re-injection is not false-flagged).
- Scope Boundary Auditor: PASS. Correction stays within the four authorized internal/core test files; broad core_test.setupTestWorkspace and all production/config/default/API surfaces are explicitly excluded (AC5/AC7); correction is genuinely class-level (shared helper + two fixture defaults + one inventory regression), not per-test patches; verification is bounded (AC8 targeted + race; AC9 defers full suite to a separately-authorized op under the zero-P0/P1 stop gate); P-021 C1 OUT OF SCOPE re done 174.066-T is correctly justified. One P3 advisory applied (AC6 regression pinned to gate_transition_test.go).

Residual P0/P1: NONE. All findings were P3 advisories; the two clarifying ones are applied to 174.069-T.

Ship-ready directive: implement 174.069-T as a test-only class-level fix in internal/core test files only (gate_transition_test.go shared helper/newGateTestWorkspace; shipment_test.go setupShipmentWorkspace; 025_archive_harness_test.go local wrapper; durability test only if consolidating). Do NOT run go test ./... during the task; use targeted per-package verification. After the commit lands, one post-fix full-suite run is a NEW separately-authorized final-gate operation requiring explicit operator authorization immediately before it.

<!-- plan-review-attempt: rev17-classlevel-gate-fixture-isolation-PASS -->

## Wave 16 — Pinned errcheck discard completion for durability helper (155-S lint unblock) (2026-09-23)

<!-- plan-review-attempt: rev18-durability-errcheck-discard -->

Corrective wave adding ONE one-line, test-only lint completion. errcheck pins internal/core/shipment_shipped_event_durability_test.go:359 because requireShippedAppendPartial returns *blerrors.MutationPartialError (satisfies error via Error() string) and its return is discarded at that single call site. Source: fresh pinned-lint finding that stopped the remaining 174.069-T gates after inventory/helper/race/compile/vet passed.

- 174.070-T (queued/high, under 174-F, governed member of active 155-S) — replace the bare call at line 359 with the repository-standard explicit discard `_ = requireShippedAppendPartial(t, err)`.

P-021 C1: SAME-CONTRACT completion of DONE 174.066-T (the helper + call site it authored). Created as a NEW task because 174.066-T is done and its review-fix authorization is exhausted; 174.066-T status is NOT reopened. The helper return is intentionally discardable at this one site: the helper's internal require.Error / ErrorAs / FailedStep assertions fully validate the structured partial, and TestShipShipment_FailClosedShippedAppendSuppressesMoveStatusPostHook needs only those plus the movePostHookFired==false assertion — no further Class/CompensationState assertion on the return.

Objective closure conditions: exactly one changed line (line 359); helper signature/return type and the three capturing call sites (198/261/325) unchanged; MutationPartialError semantics preserved; targeted durability suite GREEN; pinned errcheck cleared with no new lint; gofmt/goimports clean. No production/config/API change. Scope: internal/core/shipment_shipped_event_durability_test.go only.

Circuit disposition (shared): do NOT run go test ./... in this task. After the commit lands, Ship resumes the 174.069-T lint/build/format gates; the full suite runs later ONCE only as a NEW separately-authorized final-gate operation (explicit operator authorization immediately before). Then the 155-S zero-P0/P1 stop-gate applies.

Dependencies: none added (governed member of active 155-S; membership-as-gate). Provenance link to done 174.066-T is informational.

## Plan Review — Amendment (Wave 16 durability errcheck discard) (2026-09-23)

dispatch_mode: multi-agent-dispatch
decision: PASS

Focused Correctness review over corrective task 174.070-T (one-line, test-only errcheck discard at internal/core/shipment_shipped_event_durability_test.go:359).

- Correctness Reviewer: PASS (no findings). Verified against ground-truth code that requireShippedAppendPartial (helper line 173) performs all structured-error assertions internally (require.Error, ErrorAs into *MutationPartialError, FailedStep == shippedEventAppendStep), so discarding the return skips no validation. Line 359 sits in TestShipShipment_FailClosedShippedAppendSuppressesMoveStatusPostHook, whose only subsequent assertion is movePostHookFired==false — nothing consumes the returned partial, so the return is genuinely intentionally discardable there. The three capturing sites (198/261/325) make further Class/CompensationState assertions and are correctly left unchanged (AC2). `_ = f()` is the repo-standard explicit discard and preserves MutationPartialError semantics.

Residual P0/P1: NONE.

Ship-ready directive: apply the exact one-line change `_ = requireShippedAppendPartial(t, err)` at line 359 of internal/core/shipment_shipped_event_durability_test.go. Run the targeted durability suite GREEN, clear the pinned errcheck with no new lint, gofmt clean, confirm a one-line diff. Do NOT run go test ./... in the task; then resume the 174.069-T lint/build/format gates. The full suite runs later ONCE as a NEW separately-authorized final-gate operation (explicit operator authorization immediately before it).

<!-- plan-review-attempt: rev18-durability-errcheck-discard-PASS -->

## Wave 17 — Final two correctives: core_test recovery isolation + Ship allowlist restore (155-S) (2026-09-23)

<!-- plan-review-attempt: rev19-final-two-correctives -->

Two DISTINCT, domain-separated corrective tasks closing the last full-suite/contract blockers. Sources: stash 885263D2 (deliberation 070-DL) and 4A0B7BCF (deliberation 071-DL). No graph/status/priority change to existing tasks; both are governed members of active 155-S (membership-as-gate); no inter-task dependency (independent domains).

- 174.071-T (queued/high, under 174-F, member of 155-S) — TEST-ONLY Go fixture seam. Add package-core _test.go helper NewWorkspaceWithoutRecoveryForTest(ctx,root) delegating to newWorkspace(ctx,root,false); external core_test setupTestWorkspace (artifacts_expansion_test.go:24, 100+ callers) uses it so the broad fixture stops running recovery-enabled construction that hangs a hierarchy test in Windows FindFirstFile/EvalSymlinks during synthetic global-lock root canonicalization. Recovery-specific tests keep explicit NewWorkspace. NO production default/lock/recovery change; do NOT repurpose NewDiagnosticWorkspace. P-021 C1 OUT OF SCOPE for all done 174-F tasks (distinct test-infra seam).

- 174.072-T (queued/high, under 174-F, member of 155-S) — HARNESS/CONFIG restore. Restore the protected explicit Ship governed-lifecycle tool allowlist in .github/agents/_ship.agent.md that a generic renderer overwrote with backlogit/* (failing the integration contract requiring explicit block/unblock/normalize trio). Keep the explicit list incl. the trio + all intended tools, drop backlogit/*, preserve unrelated concurrent model-routing edits, and reconcile the .autoharness/harness-manifest.yaml _ship checksum + drift record. Do NOT change registry/plugin template here (registry trio already present via ac5ebd29); capture a separate upstream tune-preservation follow-up if the renderer will re-drift. P-021 C1 OUT OF SCOPE for all done 174-F tasks (harness/config surface, moderate mutation authority).

Circuit disposition (shared): do NOT run go test ./... in either task; use targeted per-task verification (hierarchy/expansion + recovery selectors for A; integration contract + harness verify for B). After BOTH commits land, the full suite runs ONCE as a NEW separately-authorized final-gate operation (explicit operator authorization immediately before it), followed by the final standard + adversarial review over the changed surfaces with a zero-P0/P1 stop-gate.

Dependencies: none added; both governed members of active 155-S. Provenance links to done tasks are informational.

## Plan Review — Amendment (Wave 17: final two correctives — core_test recovery isolation + Ship allowlist restore)

<!-- plan-review-attempt: rev19-final-two-correctives -->

- dispatch_mode: multi-agent-dispatch
- decision: PASS
- residual P0/P1: NONE
- scope: 174.071-T (test-only Go fixture seam) + 174.072-T (Ship harness/config allowlist restore); both governed members of active 155-S; no inter-task dependency (independent domains); no existing task status/priority/graph change.

Reviewer verdicts:
- 174.071-T — Correctness Reviewer: PASS (seam technically sound; recovery coverage preserved). Scope Boundary Auditor: PASS (strictly test-only; no production/NewDiagnosticWorkspace repurposing; <=2h single-domain).
- 174.072-T — Agent-Native Parity Reviewer: PASS. Template Integrity Reviewer: PASS. Security Reviewer (scope): PASS (dropping backlogit/* is a least-privilege improvement; no authority escalation; trio inside Ship role boundary).

Consensus P2 findings folded into the ACs (no P0/P1):
- 174.071-T: enumerate the exact recovery-agnostic fixtures to convert (setupTestWorkspace + setupTestWorkspaceWithBugLevel) and require confirmation that no recovery/journal test transitively uses a converted fixture (AC2/AC3); require a bounded-timeout RED reproduction + named targeted selectors (AC5); normalize the seam signature to (*Workspace, error) (AC1).
- 174.072-T: keep tools as a scalar comma-separated string with in-place wildcard removal so the contract .(string) assertion holds (AC1); preserve exact HEAD memory token + unrelated model-routing edits (AC2); recompute the manifest checksum LF-normalized (AC3/AC6); make require.NotContains(shipTools, "backlogit/*") a MANDATORY contract assertion (AC4); make the separate upstream tune-preservation follow-up capture MANDATORY, not conditional (AC5).

Circuit disposition: full-suite go test ./... circuit remains OPEN; neither task runs it. After BOTH commits land, one NEW separately-authorized full-suite run (explicit operator authorization immediately before), then the 155-S zero-P0/P1 stop-gate and final standard + adversarial review over the changed surfaces.

Ship-ready sequential directive: (1) apply 174.071-T test-seam (add package-core _test.go seam; switch the two enumerated fixtures; keep recovery tests on explicit NewWorkspace) with its targeted hierarchy/expansion + recovery selectors; (2) apply 174.072-T allowlist restore (in-place scalar tools restore incl. trio, drop backlogit/*, add mandatory NotContains assertion, LF-normalized manifest checksum + drift record) with verify-workspace + integration contract; (3) capture the mandatory upstream tune-preservation follow-up stash; (4) then request explicit authorization for the single post-fix full-suite operation; (5) final standard + adversarial review over changed surfaces, zero-P0/P1 stop-gate. Preserve the currently-uncommitted Ship implementation and the dirty _ship.agent.md / config / checkpoints; Stage committed only its own backlog/plan files.

<!-- plan-review-attempt: rev19-final-two-correctives-PASS -->

## Wave 18 — Corrective: size/complexity core_test fixture recovery isolation (62C6A469) (2026-09-24)

<!-- plan-review-attempt: rev20-size-complexity-fixture-recovery-isolation -->

This corrective wave adds ONE test-only task. Source: DEFERRED SCOPE EXPANSION stash `62C6A469` (P-021 C6 forced deliberate route); deliberation `072-DL`. It makes no graph, status, priority, or membership change to existing tasks, and no production, config, default, API, CI, Makefile, or timeout change. The branch is `feat/155-s-s14-resumable-shipment-blocked-lifecycle-status`; evidence HEAD is `228a28fe89def0566fcb8b96ef077e1ab0d5eb03`, and the implementation state before evidence was `eedc48a16711fc5e3cd5b94b5619c61e2f006659`. Where this section and `072-DL` differ in wording (helper responsibilities), THIS SECTION IS AUTHORITATIVE.

### Problem frame and root cause (evidence-bounded)

Three consecutive separately-authorized `go test ./...` runs failed ONLY in `internal/core`, at the default 10m package deadline (602.093s / 600.818s / 602.192s). Every other package passed, including `tests/integration`. At each deadline the running test was 0-1s old and sat in a different, unrelated, normal I/O stack:

| Capture | Blamed test (fixture) | Stack at deadline | Run-order position (current tree) |
|---|---|---|---|
| post-wave14 (`3588cad2`) | `TestShipShipment_RestoresNonMemberFeatureEvenWhenShipFailsAfterRollup` | gate `ExecVersionRunner` process wait (since fixed by 174.069-T) | about 59% (522/881) |
| post-174070 (`46145ed5`) | `TestCreateArtifact_RejectsLevel2WithoutParent` (`setupTestWorkspace`) | recovery `EvalSymlinks`/`FindFirstFile` (since fixed by 174.071-T) | about 69% (612/881) |
| post-wave17 (`eedc48a1`) | `TestSetArtifactSize_BusyLockReturnsErrTaskBusy` (`setupSizeWorkspace`) | `config.LoadTemplates` -> `os.ReadFile` | about 79% (697/881) |

No stack shows a lock waiter, holder cycle, or per-test hang. The in-capture test counts (879/880/881) are nearly constant, so the run-order positions are comparable. The root cause is the cumulative wall time of the single `internal/core` test binary (881 top-level tests: 590 `package core` + 291 `core_test`) on this Windows host. The blamed fixture is incidental to where the deadline fell. Logging stops at the same test (`TestGateBaseOverrideShadowed_WarnsAdvisory`) in two captures while the deadline moved, which confirms the 60-90s silence is a diagnostics artifact (the `slog.SetDefault` restore leaves `log` output redirected), not a hang; it is captured separately as `D8EF5443`. Paused `t.Parallel` tests (`workspace_dualroot_test.go`) run only after all sequential tests, so the about-760s linear estimate is a LOWER bound.

### Honest sufficiency disclosure and 62C6A469 split disposition (NON-NEGOTIABLE carry-forward)

This task removes the unrelated recovery cost from the fixture named by `62C6A469` (`setupSizeWorkspace`) plus its construction-identical sibling `setupComplexityWorkspace` (072-DL option A2). By itself it is NOT expected to bring `internal/core` under the 10m default: the savings are seconds, against an estimated shortfall of at least about 160s. `62C6A469` is therefore a SPLIT disposition. The fixture-family portion is resolved by `174.073-T`. The package runtime-budget residual (explicit test timeout vs partitioning vs broad fixture I/O reduction/consolidation into one canonical recovery-agnostic `core_test` constructor vs construction caching) is carried by DEFERRED SCOPE EXPANSION `11BE840F` (high, requires deliberation). `62C6A469` is archived as consumed, with forward refs to BOTH `174.073-T` and `11BE840F`; it is NOT recorded as fully resolved. Neither the task nor Ship may widen into `11BE840F` (P-021 C4). Because the 155-S final gate requires a passing full suite, 155-S cannot close until `11BE840F` is dispositioned and, if it yields work, harvested as a governed 155-S member.

### Implementation unit

- **U18 / 174.073-T** (queued/high, under `174-F`, governed member of active `155-S`). Domain: Go test infrastructure only. Safety modes: investigate-first plus freeze-scope (two files).
  - Files (exactly two, both `package core_test`): `internal/core/artifact_size_test.go`, `internal/core/artifact_complexity_test.go`.
  - Functions (four):
    1. NEW helper `newRecoveryFreeFixtureWorkspace(ctx context.Context, root string) (*core.Workspace, error)` in `artifact_size_test.go`. Its ONLY responsibility is construction: it calls the EXISTING `core.NewWorkspaceWithoutRecoveryForTest(ctx, root)` (`internal/core/workspace_test_seam_test.go`, unchanged) and wraps any error as `fmt.Errorf("new recovery-free fixture workspace: %w", err)`. It does NOT call `config.WriteDefaults`.
    2. `setupSizeWorkspace` rerouted to call the helper in place of `core.NewWorkspace`. Everything else stays exactly as before: `t.TempDir()`, queue `MkdirAll`, `config.WriteDefaults`, `require.NoError`, `t.Cleanup(ws.Close)`, golden file, and `db.UpsertItem` seed.
    3. `setupComplexityWorkspace`, rerouted identically.
    4. NEW harness `TestRecoveryFreeFixture_SkipsShipmentRecoveryButLoadsTemplates` in `artifact_size_test.go` (no `t.Parallel`; no `slog`/env mutation). Arrange: a `t.TempDir()` root with `.backlogit/queue` + `config.WriteDefaults`, then create `<root>/.backlogit/ops` as a REGULAR FILE. The implementer confirms the path against `shipmentOpsRootForWorkspace` (`internal/core/shipment_ops.go`, `filepath.Join(realStorageRoot, "ops")`; a non-directory is rejected with `blerrors.ErrValidation`). Before seeding, `require.NoFileExists`/`NoDirExists` on `ops` confirms `WriteDefaults` did not create it. Each constructor call uses its OWN distinctly named context, `recoveryCtx, cancelRecovery := context.WithTimeout(context.Background(), 2*time.Second)` / `isolatedCtx, cancelIsolated := ...`, each with `defer cancel...()`, following the naming precedent at `workspace_no_recovery_regression_test.go`. Result variables are also distinct (`recoveryWS, recoveryErr` / `isolatedWS, isolatedErr`), so there is no `:=` redeclaration. (i) Negative control: `recoveryWS, recoveryErr := core.NewWorkspace(recoveryCtx, root)`. If `recoveryWS != nil`, register a Close cleanup first (precedent `shipment_ops_security_test.go`). Then MUST give `require.Error`, `require.ErrorContains(recoveryErr, "recover shipment operations")`, `require.ErrorIs(recoveryErr, blerrors.ErrValidation)`, `require.Nil(t, recoveryWS)`. (ii) Positive: `isolatedWS, isolatedErr := newRecoveryFreeFixtureWorkspace(isolatedCtx, root)` MUST give `require.NoError`, register `t.Cleanup(func() { require.NoError(t, isolatedWS.Close()) })`, and `require.NotEmpty(t, isolatedWS.Templates)` (template loading retained).
  - Phase ownership (P-002/P-004; scoped red selectors because the full-suite circuit is OPEN, per branch precedent Waves 13-17; the compile-check narrowing is new and declared in the Quality Gates deviation):
    - RED phase (harness-architect): (a) characterization extract. Add the helper, TEMPORARILY calling `core.NewWorkspace` (TRANSITION-ONLY state; no permanent test pins it), and reroute both fixtures through it. The family selector (step 2) must be GREEN with 13 PASS, which proves the extract is behavior-preserving. (b) Add the harness. Compile check `go test -run '^$' -count=1 ./internal/core` exits 0. The harness selector (step 1) FAILS on assertion (ii) with the recovery error. Capture this under `logs/diagnostics/174073-red-harness.txt` (+ `.metadata.json`). Record `Compilation: PASS` / `Red Phase: CONFIRMED (scoped)` and apply `harness-ready`.
    - GREEN phase (build-feature): change ONLY the helper body to call `core.NewWorkspaceWithoutRecoveryForTest`. Steps 1-6 then pass. Capture under `logs/diagnostics/174073-*`.

### Verification contract (PowerShell-safe; single-quoted selectors; literal SHAs)

Before the first edit, Ship records the literal task-start SHA with `git rev-parse HEAD` and substitutes that literal text wherever `START_SHA` appears below. Never paste angle-bracket placeholders. Non-vacuity counts ONLY unindented top-level `--- PASS: Test` lines, and any `--- SKIP` or `--- FAIL` counts as failure.

1. RED/GREEN harness: `go test ./internal/core -run '^TestRecoveryFreeFixture_SkipsShipmentRecoveryButLoadsTemplates$' -count=1 -timeout=2m -v`. RED before the helper switch, GREEN after. Exactly 1 top-level PASS when green.
2. Family (exact-name anchored; a prefix selector would also match `TestSetArtifactSize_PreservesTopLevelDocline` in `docline_codec_roundtrip_test.go`, which is out of scope and keeps `core.NewWorkspace`): `go test ./internal/core -run '^(TestSetArtifactSize_PersistsAndPreservesIndexColumns|TestSetArtifactSize_RejectsInvalidValueBeforeWrite|TestSetArtifactSize_GoldenBodyPreserved|TestSetArtifactSize_Idempotent|TestSetArtifactSize_BusyLockReturnsErrTaskBusy|TestSetArtifactComplexity_PersistsAndPreservesBody|TestSetArtifactComplexity_RejectsInvalidValueBeforeWrite|TestSetArtifactComplexity_EmptyClearsField|TestSetArtifactComplexity_EmptyRejectsNonTask|TestSetArtifactComplexity_RejectsNonTaskEvenWithCustomSchema|TestSetArtifactComplexity_EmptyRequiresComplexitySchema|TestSetArtifactComplexity_GenericUpdatePreservesComplexity|TestSetArtifactComplexity_GenericCreateRejectsComplexity)$' -count=1 -timeout=2m -v`. Exactly 13 top-level PASS: the 5 `TestSetArtifactSize_*` + 8 `TestSetArtifactComplexity_*` functions declared in the two target files. `TestSetArtifactSize_BusyLockReturnsErrTaskBusy` still asserts `core.ErrTaskBusy`.
3. Recovery and seam negative controls: `go test ./internal/core -run '^(TestNewWorkspace_RecoversPendingReturnBlockedJournal|TestNewWorkspace_RemovesWriterTempResidueAndRecoversValidJournal|TestNewWorkspaceWithoutRecoveryForTest_IsolatesRecoveryState)$' -count=1 -timeout=2m -v`. Exactly 3 top-level PASS. If a named test does not resolve, STOP and report; do not substitute.
4. Template sanity (optional, non-gating; `internal/config` is untouched): `go test ./internal/config -run '^(TestLoadTemplates_|TestWriteDefaults_)' -count=1 -v`.
5. Pinned gates (Go 1.24.0): `go vet ./...`; `go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8 run --timeout=5m --new-from-rev=START_SHA ./internal/core/...` (or the installed `golangci-lint` v1.64.8 with the same arguments), 0 new findings; `go build ./cmd/backlogit`; scoped format `gofmt -l 'internal/core/artifact_size_test.go' 'internal/core/artifact_complexity_test.go'`, empty output; `git diff --check START_SHA HEAD -- 'internal/core/artifact_size_test.go' 'internal/core/artifact_complexity_test.go'`, clean (path-scoped committed delta, so unrelated dirty files cannot falsely fail it). The lint `--new-from-rev` result is judged ONLY for findings in the two target files; an unrelated-file finding from dirty-worktree state is recorded, not treated as a task failure.
6. Scope and reroute inventory, against the task commit:
   - `git diff --name-only START_SHA HEAD -- internal cmd` lists EXACTLY `internal/core/artifact_size_test.go` and `internal/core/artifact_complexity_test.go`. Allowed outside that filter: `.backlogit/**` lifecycle metadata for `174.073-T`, `logs/diagnostics/174073-*`, `docs/memory/**`. Pre-existing unrelated dirty files are neither staged nor committed.
   - `Select-String -CaseSensitive -SimpleMatch -Pattern 'core.NewWorkspace(' -Path 'internal/core/artifact_size_test.go','internal/core/artifact_complexity_test.go'` gives EXACTLY 1 match (the harness negative control). Counts include comments, so do not write either literal pattern in a comment.
   - `-Pattern 'newRecoveryFreeFixtureWorkspace('` gives EXACTLY 4 matches: 1 definition, plus calls in `setupSizeWorkspace`, `setupComplexityWorkspace`, and the harness.
   - `internal/core/workspace_test_seam_test.go` and every non-`_test.go` file are unchanged.

### Circuit disposition (NON-NEGOTIABLE)

- Do NOT run `go test ./...` within this task. Do NOT run the whole `internal/core` package without a `-run` selector; both are full-suite-equivalent because `internal/core` is the failing unit. The package-scoped `-run '^$'` compile check in the RED phase (`./internal/core` only) is the only unselected form allowed, and it runs no tests. `go test -run '^$' ./...` is NOT used, because it is itself a `go test ./...` invocation under the operator's stop rule; see the Quality Gates deviation.
- After the task commit lands, Ship STOPS. Before any next `go test ./...`, Ship MUST request a NEW, separate, explicit operator authorization immediately before the run. That request MUST cite either a recorded operator disposition of `11BE840F`, or an explicit operator waiver accepting a known-risk diagnostic run.
- The run form SHOULD be `go test -json ./...`, captured to `logs/diagnostics/`, so per-test `Elapsed` is available.
- The 155-S zero-P0/P1 stop-gate and the final standard + adversarial review then apply.

### Dependencies

None added. The task is a governed member of active `155-S` (membership-as-gate). Provenance links to done `174.071-T` (seam author) are informational. The 155-S final gate depends on the `11BE840F` disposition (operator decision; not yet harvestable).

### Plan Hardening Signals (Wave 18)

- Public API/schema/contract change: absent (test-only; the seam is reused unchanged).
- Security/auth/permission: absent.
- Migration/destructive/irreversible: absent (only `t.TempDir()` scratch writes).
- External integration / operator checkpoint / external dependency: PRESENT. The next full suite needs separate operator authorization, and `11BE840F` is an operator decision.
- High runtime/rollout/rollback risk: PRESENT (moderate). The Windows package deadline remains after this task, so a mis-sequenced full-suite run would waste another roughly 13-minute authorized operation.

Requires plan hardening: yes

### Constitution Check (Wave 18)

- I. Safety-First Go: pass except the lint-scope element, which is a DEVIATION (see Quality Gates below: `golangci-lint` runs in `--new-from-rev` mode). Test-only Go; no `unsafe`; the helper wraps errors with `%w`.
- II. Test-First Development (NON-NEGOTIABLE): pass. The characterization extract comes first, then a failing harness captured as RED before GREEN. "All tests pass via `go test ./...` before merge" is enforced at the 155-S merge gate under separate authorization, not waived. `11BE840F` must preserve this rule; no ad-hoc timeout workaround.
- III. Workspace Isolation and Security Boundaries: pass. All test-authored writes stay under each test's `t.TempDir()` root; no traversal; no secrets.
- IV. CLI Workspace Containment (NON-NEGOTIABLE): pass. The agent performs no file operation outside the working directory. Diagnostics go under `logs/diagnostics/`. The only out-of-cwd writes (`t.TempDir()`, GOCACHE) are made by the Go test/toolchain process itself, not by agent file operations, so they are not an exception to IV.
- V. Structured Observability: pass. RED/GREEN evidence is captured under `logs/diagnostics/174073-*` with metadata. The log-capture leak that degraded observability is captured as `D8EF5443`.
- VI. Single Responsibility: pass. The helper only constructs; fixtures keep seeding; the harness proves one contract.
- VII. Destructive Command Approval (NON-NEGOTIABLE): pass. No destructive steps; the full-suite run is gated by operator authorization.
- VIII. Explicit Safety Modes: pass. Investigate-first + freeze-scope, declared above.
- IX. Git-Friendly Persistence: pass. Text-only test changes; backlog changes go through governed backlogit operations.
- X. Agent Context Efficiency: pass. The task contract is self-contained, with exact selectors and paths.
- XI. Merge Commit History Preservation (NON-NEGOTIABLE): pass. The work ships inside the 155-S merge-commit PR; no squash/rebase.
- Quality Gates / Technical Constraints: DEVIATION (documented, not a NON-NEGOTIABLE principle). At task level, `go test ./...` is replaced by the scoped selectors above because the full-suite circuit is OPEN; it runs once at the 155-S final gate under separate authorization. `golangci-lint run` runs in `--new-from-rev` mode because the repo carries pre-existing lint debt (`4DB1DFF1`). `gofmt -l .` is scoped to the two touched files because the repo carries pre-existing format drift (`4DB1DFF1`). `go vet ./...` runs in full. P-004 red-phase mapping: P-004's precondition `go test -run=^$ -count=1 ./...` is NARROWED to `go test -run '^$' -count=1 ./internal/core`, because any `go test ./...` form is behind the operator's stop rule. Only `internal/core` changes, and `go vet ./...` + `go build ./cmd/backlogit` compile-check the other non-test packages. P-004's `go test ./...` exits non-zero clause is mapped to the scoped harness selector (P-002.6 per-member scoped commands). Branch precedent (Waves 13-17) supports scoped red selectors only; the compile-check narrowing is NEW and is declared here. Rejected simpler alternative: running the full gates per task, which would re-trip the known package deadline and pre-existing debt unrelated to this change.

Constitution Check: documented-deviations

## Plan Hardening — Wave 18 (size/complexity fixture recovery isolation)

Hardening required: yes (operator checkpoint + residual runtime risk). Consulted: `.github/policies/workflow-policies.md` P-002/P-004/P-021 (C1/C2/C4/C6); `.github/instructions/constitution.instructions.md`; `docs/compound/test-failures/go-analysistest-absolute-path-and-non-vacuity-2026-09-11.md` (non-vacuity of scoped `-run` selectors); `docs/compound/2026-07-29-durable-writes-test-seam-patterns.md` (seams must not leak process-global state; no `t.Parallel` alongside global seams); `docs/compound/best-practices/source-shape-harnesses-must-allow-lifecycle-successors-2026-09-11.md` (the RED extract is a transition-only state); `docs/memory/2026-09-24/ship-155s-174070-format-command-blocked.md` (a PowerShell-unsafe format command previously blocked Ship).

Protected invariants:
1. Recovery coverage: every recovery/journal test keeps explicit `core.NewWorkspace`, and the harness negative control proves recovery still engages on the seeded root (`ErrorIs` `blerrors.ErrValidation`).
2. Template coverage: the recovery-free fixture still loads templates (`require.NotEmpty(ws.Templates)`), and the complexity tests still create artifacts.
3. Lock semantics: `TestSetArtifactSize_BusyLockReturnsErrTaskBusy` still returns `core.ErrTaskBusy`.
4. Seam integrity: `NewWorkspaceWithoutRecoveryForTest` is unchanged and its regression test stays GREEN; `NewDiagnosticWorkspace` is not repurposed; no production file changes.
5. No global-state leakage: no `slog.SetDefault`, env, or `t.Parallel` changes in the touched files.
6. Reroute proof: static inventory (step 6) shows neither fixture calls `core.NewWorkspace`.

ProposedAction / ActionRisk:
- PA-1: edit two `_test.go` files (helper + two reroutes + harness). ActionRisk: low; reversible by `git revert`; no approval needed beyond task claim.
- PA-2: run targeted selectors 1-4, pinned gates 5, and inventory 6. ActionRisk: low; bounded (`-timeout=2m`) and scoped.
- PA-3: next `go test ./...` (OUTSIDE this task). ActionRisk: medium (a roughly 13-minute operation with a known residual deadline risk). REQUIRES a new explicit operator authorization immediately before it, citing the `11BE840F` disposition or an explicit known-risk waiver; the recommended form is `go test -json ./...`.

Blocked-path handling:
- If RED cannot be reproduced (`core.NewWorkspace` succeeds on the seeded root, or the harness passes before the switch), STOP. Do not substitute a weaker fingerprint. Record the observation and return to Stage, since that would falsify the recovery-cost premise.
- If the extract turns the family selector non-GREEN, STOP. The extract was not behavior-preserving.
- If any selector reports SKIP, fewer PASS than stated, or a named test does not resolve, treat it as FAIL.
- If a pinned gate fails, stop without retrying under a different command form, and record the evidence under `logs/diagnostics/`.

Rollback: `git revert` of the single task commit restores both fixtures; there is no data or config state to unwind. Owner: Ship (task execution); Stage (`11BE840F` and `D8EF5443` follow-ups). Validation window: the task closes on steps 1-6. The package-deadline outcome is measured only by the next separately-authorized full-suite run.

Review-gate capability carry-forward: plan review MUST emit literal `dispatch_mode:` and `decision:` markers. If reviewer sub-agent dispatch is unavailable or partial, it must declare `single-agent-declared-degradation` and `TOOL_DEGRADED: reviewer-subagent-dispatch` rather than issue a partial gate.

Unresolved operator decisions (non-blocking for this task; blocking for the 155-S final gate): the `11BE840F` package-budget disposition (or an explicit known-risk waiver), and authorization for the next full suite.

## Plan Review — Wave 18 attempt 1 (rev20, superseded)

- dispatch_mode: multi-agent-dispatch
- decision: ADVISORY
- operator_authorization: not requested (superseded by the in-cycle P2 revision below; not a gate-satisfying record)
- personas: Go Reviewer (PASS, 3 P2 / 5 P3), Scope Boundary Auditor (PASS, 4 P2 / 3 P3), Constitution Reviewer (PASS, 4 P2 / 5 P3), Architecture Strategist (PASS, 4 P2 / 3 P3), Learnings Researcher (PASS, 0 P2 / 4 P3, confidence high). Agent-Native Parity and Security Lens were not triggered (no MCP/agent-facing surface; no auth/secrets/trust boundary).
- residual P0/P1: NONE.
- Plan hardening required: yes; satisfied by `## Plan Hardening — Wave 18`.

Merged P2 findings (deduplicated), all C1-in-scope for this plan and resolved in place in the Wave 18 section above (rev20.1):
1. The harness proved the helper but not the fixture reroute (Go/Scope/Arch/Constitution). Resolved: step 6 static inventory (exactly 1 `core.NewWorkspace(`, exactly 4 helper matches) + invariant 6.
2. The scope-inventory command compared against the dirty worktree and used a PowerShell-unsafe `<placeholder>` (Go/Scope). Resolved: `git diff --name-only START_SHA HEAD -- internal cmd`, a literal SHA via `git rev-parse HEAD`, and an explicit allowed-metadata list.
3. The harness lacked close/timeout discipline (Go). Resolved: `t.Cleanup(require.NoError(ws.Close()))`, 2s `context.WithTimeout` per constructor, `require.Nil(ws)` on the negative path, and `ErrorIs(blerrors.ErrValidation)`.
4. `11BE840F` was only "advised" before the next full suite (Scope/Arch). Resolved: the authorization request MUST cite a recorded `11BE840F` disposition or an explicit known-risk waiver.
5. The `62C6A469` disposition was unstated (Scope). Resolved: SPLIT disposition recorded (family portion to `174.073-T`; residual to `11BE840F`; archive with both forward refs).
6. 155-S closure dependency on `11BE840F` was untracked (Arch). Resolved: recorded in Dependencies and the sufficiency section.
7. The DL vs plan helper wording conflicted (Scope/Arch). Resolved: plan declared authoritative; the helper only constructs; the 072-DL body wording was corrected, and a 072-DL comment event records the supersession.
8. The Constitution Check was incomplete, mislabeled III/IV, gave a wrong III/IV rationale, and left the scoped gates undeclared (Constitution). Resolved: all principles I-XI listed with correct NON-NEGOTIABLE labels, toolchain temp/cache exception stated, Quality Gates deviation documented; verdict now `documented-deviations`.
9. RED ownership under P-002/P-004 was unassigned (Constitution). Resolved: extract + harness + scoped red + `harness-ready` go to harness-architect; the helper switch goes to build-feature.

Stage static verification (post-revision): the `Templates` field (`workspace.go:36`), the `blerrors` alias precedent, the 5+8 family functions, all three negative-control names (`shipment_test.go:1244`, `shipment_p1_lifecycle_remediation_test.go:61`, `workspace_no_recovery_regression_test.go:17`), and the current 2 `core.NewWorkspace(` sites (`artifact_size_test.go:54`, `artifact_complexity_test.go:45`) were confirmed. A prefix collision (`TestSetArtifactSize_PreservesTopLevelDocline`) was found and removed by exact-name anchoring selector 2.

P3 items adopted: helper renamed `newRecoveryFreeFixtureWorkspace`; error wrap with `%w`; seam regression test added to step 3; family selector run after the extract; RED evidence path named; top-level-only PASS counting; `-timeout=2m` on selectors 1-3; RED extract marked transition-only; safety modes named; "family" wording narrowed; selector 4 made non-gating; `go vet ./...` run in full; lower-bound note on the estimate. P3 items recorded for `11BE840F` deliberation: consolidate recovery-free fixtures into one canonical `core_test` constructor; the paused-parallel test cost.

<!-- plan-review-attempt: rev20-attempt-1-ADVISORY-superseded -->

## Plan Review — Wave 18 final (rev20.2, attempt 2)

- dispatch_mode: multi-agent-dispatch
- decision: PASS
- personas (re-review of the revised section after the attempt-1 P2 resolutions): Go Reviewer PASS (0 P0/P1/P2; all 3 attempt-1 P2s RESOLVED); Scope Boundary Auditor PASS (0 P0/P1/P2; F1-F4 and the DL wording conflict RESOLVED); Constitution Reviewer PASS (0 P0/P1; all 5 attempt-1 P2s RESOLVED; 1 new P2). The Architecture Strategist's and Learnings Researcher's attempt-1 findings were covered by the same resolutions: the reroute proof, the `11BE840F` precondition, the 155-S closure dependency, and the DL wording. Neither re-review surfaced any finding in their areas.
- Plan hardening required: yes; satisfied by `## Plan Hardening — Wave 18`.
- Constitution Check: documented-deviations (Quality Gates / Principle I lint scope; P-004 compile-check narrowing). No NON-NEGOTIABLE principle is deviated.

Attempt-2 findings and disposition (all resolved in place; none deferred):
- P2 (Constitution): the P-004 compile check was narrowed to `./internal/core` without a declaration. RESOLVED: the narrowing and its rationale are now declared in the Quality Gates deviation. `go test -run '^$' ./...` is explicitly not used, because it is a `go test ./...` form under the operator's stop rule. `go vet ./...` and `go build ./cmd/backlogit` compile-check the rest.
- P3 (Constitution): Principle I was marked pass despite the lint scoping. RESOLVED: now marked "pass except lint-scope DEVIATION".
- P3 (Constitution): the IV "toolchain exception" wording. RESOLVED: reworded to "no agent file operation outside cwd; toolchain temp is process-managed".
- P3 (Constitution): the precedent overstated. RESOLVED: the text now says precedent covers scoped red selectors only and the compile-check narrowing is new.
- P3 (Go): `git diff --check` / lint compared against the dirty worktree. RESOLVED: `git diff --check START_SHA HEAD -- <two paths>`; lint judged only on the two target files.
- P3 (Go): harness redeclaration and lost-cancel risk. RESOLVED: distinct `recoveryCtx`/`isolatedCtx` and `recoveryWS`/`isolatedWS` names, each with `defer cancel`.
- P3 (Go): negative path could leak an open workspace. RESOLVED: Close cleanup registered if `recoveryWS != nil`.
- P3 (Go): comments could break the static counts. RESOLVED: "counts include comments" stated.
- P3 (Go): `WriteDefaults` might create `ops`. RESOLVED: `NoFileExists`/`NoDirExists` pre-seed check.
- P3 (Scope): the 072-DL supersession comment was missing. RESOLVED: 072-DL body corrected and a comment event appended.
- Stage static verification also found a prefix collision in selector 2 (`TestSetArtifactSize_PreservesTopLevelDocline`). RESOLVED before attempt 2 by exact-name anchoring.

Out-of-scope residuals (P-021 C2 captured; NOT widened into this task): `11BE840F` (package runtime-budget disposition; high; deliberate route) and `D8EF5443` (slog/log capture restore leak; medium; deliberate route).

Residual P0/P1/P2: NONE. Gate satisfied (`decision: PASS`). Harvest authorized for exactly one task (`174.073-T` or the CLI-assigned ID recorded below at harvest).

<!-- plan-review-attempt: rev20-attempt-2-PASS -->

### Wave 18 harvest record (2026-09-24)

- Harvested: `174.073-T` "Route size/complexity core_test fixtures through recovery-free seam" (task, queued, high, parent `174-F`; CLI-assigned ID matches this plan).
- Shipment: governed `backlogit shipment add 155-S 174.073-T` -> `added`. The 155-S manifest now has 36 items (`174-F` first, `174.073-T` last), status `active`. `154-S` is untouched.
- Stash (Stage authority): `62C6A469` edited with the SPLIT disposition and archived via `backlogit stash archive` (non-destructive). `11BE840F` and `D8EF5443` were late-reconciled from `task N/A` to `task 174.073-T`; both remain ACTIVE for operator-decided deliberation.
- Dependency edges: none added (membership-as-gate).

## Wave 19 — Governed adaptive go test runtime budget (11BE840F / 073-DL) — rev21.3

**Source.** DEFERRED SCOPE EXPANSION stash `11BE840F` (P-021 C6 forced the deliberate route) →
deliberation `073-DL` (`.backlogit/queue/073-DL.md`, option O11). Operator direction 2026-09-24:
"Deal with 11BE840F first. I don't know where the 10-minute timeout came from; it seems arbitrary.
The more tests we have, the longer they will take and we should adapt accordingly." The operator
delegated the decision to Stage ("decide and produce executable reviewed work"), so O11 is the
recorded decision.

**Authority.** This section is the authoritative Wave 19 contract. Where it differs from
`073-DL`, this section wins. rev21 failed plan review in attempt 1, rev21.1 failed in
attempt 2, and rev21.2 failed in attempt 3 (see the `## Plan Review — Wave 19` records). rev21.3
applies every attempt-3 disposition. The operator authorized one additional review pass, and
rev21.3 failed it in attempt 4 (one P0, B1). Harvest stays blocked pending a new operator
decision (see the attempt-4 record). Task IDs in the attempt-1 record use rev21.1 numbering. Differences from 073-DL:

* 18 tasks instead of 13. Every new Go surface is split into a declaration task and a behavior
  task (Principle II), and the wrapper is split into a run surface and a result-sink surface to
  stay within the 2-hour rule.
* The contract test lands before the surface migrations as a red deliverable, not last.
* At the ceiling, the wrapper clamps and still runs, then exits non-zero. It does not refuse to
  run.
* The constitution bump is MINOR `1.1.0`, not PATCH.
* The drift-ignore edits are dropped.
* Targeted `-run` selectors keep Go's default timeout (or the smaller timeout their task
  declares). They do not "keep explicit small timeouts".
* The 073-DL note "capture the next run with `-json`" is superseded (see 19.4.6).

### 19.1 Provenance finding (verified, no run)

* **Origin.** The 10-minute limit is `cmd/go`'s built-in default for `go test`:
  * `GOROOT/src/cmd/go/internal/test/testflag.go:69` has
    `cf.DurationVar(&testTimeout, "timeout", 10*time.Minute, "")`.
  * `testflag.go:385-389` injects `-test.timeout=10m0s` into every test binary when the caller
    passes no `-timeout`.
  * `cmd/go` adds a backup process kill at timeout + 1m (`test.go:841`).
  * The in-binary alarm is `testing.(*M).startAlarm` (`testing.go:2802`), the panic frame in
    every capture.
  * Host toolchain `go1.26.5`; `go.mod` declares `go 1.24.0`.
* **Scope.** The limit is **per package test binary** and **cumulative**. It is not per test and
  not per command. `internal/core` is one binary with 885 top-level tests (886 after
  `174.073-T`), so its whole runtime shares one 10-minute alarm.
* **Ownership.** Nobody in this repository owns it:
  * `GOFLAGS` is empty (env and `go env`) and there is no `go.work`.
  * No governed invocation passes `-timeout` (19.2).
  * The `.autoharness` `TEST_COMMAND` token is a bare `go test ./...`, so every rendered harness
    surface inherits the default implicitly.
* **Outer limits that are NOT the test timeout:**
  * The `ci.yml` `test` job has no `timeout-minutes` (GitHub default 360m).
  * The only 10-minute literal in CI is `timeout-minutes: 10` on the unrelated `topology-check`
    job. It is a coincidence.
  * `golangci-lint --timeout=5m` and `engram --timeout 300` are lint and index limits.
  * The agent shell `initial_wait` is a poll interval, not a kill.
  * The 803.433s overall wall time of the post-wave17 run comes from packages running in
    parallel. Only `internal/core`'s own binary alarm fired.

### 19.2 Governed invocation-surface inventory

"Relies on default" means the line runs a full-suite form with no `-timeout`, so Go's 10m default
applies per package.

| Surface | Line(s) | Current form | Relies on default | Migrating unit |
|---|---|---|---|---|
| `.github/workflows/ci.yml` step `Test` | 121 | `go test -race -coverprofile=coverage.out ./...` | yes | U19h `174.081-T` |
| `.github/workflows/release.yml` step `Test` | 40 | `go test ./...` | yes | U19h `174.081-T` |
| `scripts/pre-push-quality-gates.ps1` Test gate | 140 | `Invoke-Gate "Test" "go" "go test ./..."` | yes | U19i `174.082-T` |
| `scripts/pre-push-quality-gates.sh` Test gate | 129 | `run_gate "Test" "go" "go test ./..."` | yes | U19i `174.082-T` |
| `Makefile` `test:` | 20 | `go test -race -coverprofile=coverage.out ./...` | yes | U19j `174.083-T` |
| `make.ps1` `Step "test"` | 50, 65 | same as Makefile | yes | U19j `174.083-T` |
| `.github/agents/_ship.agent.md` final gate | 950, 989 | unfiltered full repository suite `go test ./...` | yes | U19k `174.084-T` |
| `.github/skills/build-feature/SKILL.md` | 355, 365 | full-suite gate | yes | U19k `174.084-T` |
| `.github/policies/workflow-policies.md` | 916, 925, 1149 | full-suite rule text | yes | U19l `174.085-T` |
| `.github/skills/fix-ci/SKILL.md` | 13, 72, 126, 189 | local reproduction | yes | U19m `174.086-T` |
| `.github/instructions/github-pr-automation.instructions.md` | 126 | pre-PR gate | yes | U19m `174.086-T` |
| `AGENTS.md` | 16 (version), 43, 239, 252 | build/test guidance | yes | U19n `174.087-T` |
| `.github/instructions/constitution.instructions.md` | 22, 209, 221, 328 (version) | quality gate | yes | U19n `174.087-T` |
| `.github/copilot-instructions.md` | 13, 47 | test guidance | yes | U19o `174.088-T` |
| `.github/instructions/go.instructions.md` | 35, 36 | test guidance | yes | U19o `174.088-T` |
| `.github/agents/subagents/go-engineer.agent.md` | 76 | validation loop (74 is compile-only and stays unchanged) | yes | U19p `174.089-T` |
| `README.md` | 283 | contributor test command | yes | U19p `174.089-T` |
| `.github/copilot-review-instructions.md` | 195 | reviewer guidance | yes | U19q `174.090-T` |
| `.github/instructions/copilot-code-review.instructions.md` | 53 | reviewer guidance | yes | U19q `174.090-T` |
| `.autoharness/harness-manifest.yaml` variables | 556 `TEST_COMMAND`, 596 `HARNESS_ENFORCED_SUMMARY`, 633 `TESTING_RULES` | render tokens | yes, for re-render | U19r `174.091-T` |
| `.autoharness/workspace-profile.yaml` | 27, 47 | discovered test command | yes, for re-render | U19r `174.091-T` |

**Left unchanged on purpose:**

* **Descriptive or historical text:**
  * `harness-manifest.yaml:461` (a `note:` recording what was installed 2026-08-31).
  * `.github/skills/doc-review/SKILL.md:200` (a list of commands whose absence makes docs
    stale; `go test` still exists).
  * `tests/simulation/wave-scheduler-contract.json:968-969` (a simulation fixture, not an
    invocation).
  * Workflow-policies amendment-log rows.
  * All prior plan and memory text.
* **Out of scope:** `plugin/**`. It is the product bundle shipped to consumer workspaces that do
  not have `cmd/test-budget`. Its parity is captured as a P-021 follow-up (19.12).
* **Targeted selectors** (`go test ./pkg -run '^TestX$' ...`) are not full-suite forms. They keep
  Go's default 10m, or the smaller `-timeout` their own task declares. This includes
  `ci.yml:165` (the `test-windows` job, `-run "ItemLog|ShipmentReconcile"`).
* **Compile-only gates** (`-run=^$`) stay unchanged: `scripts/pre-push-quality-gates.ps1:141` and
  `.sh:130` (Build gate `go test -run=^$ -count=1 ./...`). No test body runs under them, so they
  are not runtime-budget surfaces.
* **Does CI differ from dev/Ship?** Yes, but in flags only:
  * CI and make add `-race -coverprofile=coverage.out`.
  * release, pre-push, and Ship use the plain form.
  * All of them rely on the same unowned 10m default.

### 19.3 Runtime evidence, safe baseline, and headroom (existing transcripts only)

| Run (Windows, no `-race`) | `internal/core` elapsed | Result |
|---|---|---|
| final 2026-09-23 | 602.717s | TIMEOUT |
| post-`174.066-T` | 578.449s | PASS (96.4% of 600s) |
| post-wave14 `3588cad2` | 602.093s | TIMEOUT |
| post-`174.070-T` `46145ed5` | 600.818s | TIMEOUT |
| post-wave17 `eedc48a1` | 602.192s | TIMEOUT (overall wall 803.433s) |

* **Where the deadline fell.** At about 59%, 69%, and 79% of the 881-test run order. Each time
  the running test was 0-1s old and sat in ordinary I/O. That signature is cumulative growth,
  not a hang.
* **Measured per-test cost.** `600s / (0.79 × 881) = 0.862 s/test`. This is a lower bound,
  because paused `t.Parallel` tests run last. The linear projection is about 760s.
* **Linux CI with `-race`** (run 35462047185, 2026-09-19): `internal/core` took 239.878s, about
  0.29 s/test.
* **Growth.** Static top-level counts for `internal/core` were 800 at merge-base `37a5cba4` and
  885 at HEAD `0ffbdd42`: +85 in one shipment. Next largest packages: `internal/cli` 385,
  `internal/events` 253.
* **Safe baseline.** PerTestReference = 900ms. This is the Windows dev/Ship host measurement
  (the slowest governed platform), rounded up.
* **Headroom.** 2× over that reference. The expected ~760-800s run therefore uses about 45% of
  the 30m budget. That absorbs host noise and the unmeasured `t.Parallel` tail, and still leaves
  WARN (R1) meaningful.

### 19.4 Chosen policy — Governed Test Runtime Budget (O11)

#### 19.4.1 Single source of truth

The new package `internal/testbudget` is the single source of truth. Its package doc carries the
provenance, the formula, and R1-R5. Constants:

| Constant | Value | Why |
|---|---|---|
| `Floor` | `10 * time.Minute` | Never below Go's own default |
| `Step` | `5 * time.Minute` | Rounding step; limits `go test` cache-key churn (`-timeout` is a cached flag) |
| `PerTestReference` | `900 * time.Millisecond` | 19.3 measurement, rounded up |
| `Headroom` | `2` (untyped const) | 2× safety over the reference |
| `Ceiling` | `45 * time.Minute` | Upper bound that forces a Stage decision |
| `PolicyID` | `"073-DL"` | Links every printed budget to the governing decision |

**Formula:** `Compute(n) = max(Floor, ceil_to_Step(n × PerTestReference × Headroom))`

* `n < 0` returns an error.
* `n == 0` returns `Floor` from `Compute`. The caller fails closed on an empty repository count
  (19.4.3).
* **Overflow guard:** if `n > int(Ceiling / (PerTestReference * Headroom))` (that is, `n > 1500`),
  return `(Ceiling, fmt.Errorf("%w: ...", ErrCeilingExceeded))` **before** multiplying.

**Current value:** N_max = 885 (886 after `174.073-T`) → 1593s (1594.8s) → **`30m0s`**.

#### 19.4.2 Growth table (pinned in `TestCompute_Table`)

| N_max (largest package) | Budget |
|---|---|
| 0-333 | 10m (floor) |
| 334-500 | 15m |
| 501-666 | 20m |
| 667-833 | 25m |
| 834-1000 | 30m (today: 885/886) |
| 1001-1166 | 35m |
| 1167-1333 | 40m |
| 1334-1500 | 45m (ceiling) |
| > 1500 | `ErrCeilingExceeded` (clamped to 45m, run marked failed) |

The table test pins these rows: 0→10m, 1→10m, 333→10m, 334→15m, 500→15m, 501→20m, 885→30m,
886→30m, 1000→30m, 1001→35m, 1500→45m, 1501→(45m, `ErrCeilingExceeded`), -1→error.

#### 19.4.3 Static counter (deterministic per commit, identical on every machine)

**What `N_max` is.** The largest count, in any one package directory, of top-level test
functions in `*_test.go` files. Internal and external `_test` packages in one directory share one
binary, so they are counted together.

**What counts as a test function:**

* A `*ast.FuncDecl` with no receiver.
* Named `Test…` or `Fuzz…`, where the suffix does not start with a lowercase letter.
* Exactly one parameter, whose type is `*T`/`*testing.T` for `Test…` or `*F`/`*testing.F` for
  `Fuzz…`. The parameter list must have one field naming at most one parameter, so
  `func TestX(a, b *testing.T)` is rejected, as `cmd/go` rejects it. The type may be
  `*ast.StarExpr` over an `*ast.Ident` or an `*ast.SelectorExpr` whose `Sel` is `T`/`F`.
* No results.

**Consequences:**

* `TestMain(m *testing.M)` is excluded by the parameter rule.
* Benchmarks and examples are not counted.
* Build tags are ignored, so every file is counted. This is conservative: it overcounts across
  GOOS-specific files.
* Non-`_test.go` files are never read.

**Directory walk.** `Largest(root)` walks `root`. The skip rules apply only to directories other
than `root` itself (`path != root`), so `Largest(".")` is not skipped by the `.` prefix rule. It
skips:

* `testdata`, `vendor`, and `.`- or `_`-prefixed directories;
* any nested directory containing its own `go.mod`.

`Dir` is reported slash-separated relative to `root` (`.` for the root).

**Repository-wide on purpose.** The budget is derived from the whole module, whatever package
pattern the invocation selects. The printed value is therefore a function of the commit alone,
identical on dev, Ship, and CI.

**Fail closed.** `N_max == 0` on a real repository is an error, not the floor.

#### 19.4.4 Governed entry point `cmd/test-budget`

`cmd/test-budget` uses plain `os.Args`, not cobra, and is not part of the release artifact. It
must be run from the module root; a missing `go.mod` in the working directory gives exit 2.

**`go run ./cmd/test-budget print`** prints the header and exits 0. It never runs tests, so
verification can check the derivation without any suite run.

**`go run ./cmd/test-budget run <go test flags/packages>`:**

1. Rejects a caller `-timeout`, `--timeout`, `-test.timeout`, or `--test.timeout` (in `=` or
   separate-value form) with exit 2 and `TEST_BUDGET_ERROR: -timeout is governed by 073-DL`.
   This keeps one source of truth.
2. Prints the header (field order is pinned):
   `TEST_BUDGET: timeout=30m0s largest_package=internal/core top_level_tests=886 policy=073-DL`.
3. Emits these header warnings when they apply:
   * `TEST_BUDGET_WARN: largest_package_tests=<n> revisit=R3` when N_max ≥ 1200.
   * `TEST_BUDGET_WARN: race-on-windows uncalibrated revisit=R4` when `-race` is passed and
     `hostGOOS == "windows"` (`hostGOOS` is a package variable initialized from `runtime.GOOS`).
   * `TEST_BUDGET_CEILING_EXCEEDED: largest_package_tests=<n> ceiling=45m0s revisit=R3` when
     `Compute` returns `ErrCeilingExceeded`. The run continues clamped to 45m so the evidence R3
     needs is still produced. The final exit is non-zero even when every package passes.
4. Execs `go test` with the argument vector built by `buildGoTestArgs(d, args)`, which is exactly
   `["test", "-timeout=<d>", args...]`. `execGo` maps a `Start` failure to 2 inline and passes the
   `Wait` outcome to the pure helper `mapWaitResult(code, err)` (declared in 174.077-T, tested in
   174.078-T).
   * The child's stdout and stderr stream live through **one** writer,
     `w := io.MultiWriter(stdout, sink)`, passed as both `Stdout` and `Stderr`. `os/exec`
     therefore uses a single pipe and a single copy goroutine (it compares the two writers
     with `interfaceEqual`). `Write` is never called concurrently, and the wrapper starts no
     goroutine of its own.
   * `resultSink.Write` always returns `len(p), nil`. It buffers partial lines (capped at
     64 KiB; the rest of an over-long line is discarded), strips `\r`, and parses each
     completed line against `^(ok|FAIL)\s+(\S+)\s+([0-9]+(?:\.[0-9]+)?)s(?:\s|$)`. `(cached)`,
     `[no test files]`, and `[build failed]` lines do not match.
   * The terminal receives the child's raw bytes unchanged. Only the sink's private copy is
     line-normalized.
5. Prints the footer:
   `TEST_BUDGET_SUMMARY: timeout=<d> slowest_package=<pkg> slowest_elapsed=<raw>s budget_use=<pct>% largest_package_tests=<n> largest_package_elapsed=<raw>s per_test_cost=<s> ratio_to_reference=<x.xx>`.
   * The raw elapsed token is kept as printed by `go test`.
   * `per_test_cost` and the ratio use `%.3f` and `%.2f`.
   * The largest package's elapsed is found by an **exact** import-path match, where the path is
     the `go.mod` `module` path, plus `"/" + Dir` when `Dir != "."`. If there is no match, those
     fields print `n/a`.
   * If nothing was parsed (for example, a fully cached run), the summary prints
     `slowest_package=n/a` with no percentages and derives no WARN.
   * Footer warnings:
     * `TEST_BUDGET_WARN: package=<pkg> elapsed=<raw>s exceeds budget/Headroom revisit=R1` for
       each package over `budget / Headroom`.
     * `TEST_BUDGET_WARN: per_test_cost ratio=<x> revisit=R2` when `ratio_to_reference > 1.25`.
   * With `-json` in the arguments, the footer prints `TEST_BUDGET_SUMMARY: mode=json-unparsed`
     and no WARN is derived.

**Exit codes, returned by `run()`:**

| Code | Meaning |
|---|---|
| 0 | Child succeeded and the ceiling was not exceeded |
| child's code | Child failed; a negative `ExitCode()` (signal) maps to 1 |
| 1 | The child exited but a descendant held the output open past `WaitDelay` (`exec.ErrWaitDelay`), fail closed |
| 2 | Usage, configuration, or counter error, or the `go` process failed to start |
| 3 | Child succeeded but `ErrCeilingExceeded` |

`go run` collapses any non-zero program status to 1. Gates therefore classify outcomes by the
printed markers and zero/non-zero status, not by exact codes. Codes 2 and 3 matter only to the
direct `run()` unit tests.

**Signals.** The wrapper never kills or signals the child, and it starts no goroutine.

* `execGo` registers `signal.Notify(ch, os.Interrupt)` on a buffered channel of size 1, with
  `defer signal.Stop(ch)`. This only keeps the wrapper alive until the child exits and the
  footer is printed.
* The terminal or console delivers Ctrl+C to the child process directly (the process group on
  Unix, the attached console on Windows).
* `cmd/go`'s own budget + 1m kill stays the backstop, so the hang detector stays finite.

#### 19.4.5 Governed commands

**Canonical literals** (required exactly at the structural gate locations the coupling test
checks):

* **canonicalFull** (Ship final gate, release, pre-push, docs):
  `go run ./cmd/test-budget run ./...`
* **canonicalCI** (CI `Test` step, Makefile, make.ps1):
  `go run ./cmd/test-budget run -race -coverprofile=coverage.out ./...`

**Any other governed span** (`go run ./cmd/test-budget run <args>` with no timeout flag) is
accepted in prose. For example, `go.instructions.md:36` becomes
`go run ./cmd/test-budget run -coverprofile=coverage.out ./...`, which keeps that line's
no-`-race` intent.

#### 19.4.6 Hang detection and `-json` supersession

* **The hang detector stays finite.** Every test binary still gets a Go alarm (30m today) with a
  goroutine dump that names the stuck test. `cmd/go` kills the process at budget + 1m.
* **Detection latency.** It grows from 10m to the computed budget. This is accepted, because R5
  classifies a timeout whose running test is ≥ 60s old as a defect, never as a reason to raise
  the budget.
* **`-json`.** 073-DL/11BE840F's "capture the next run with `go test -json`" is superseded for
  the gate. The gate form is canonicalFull, whose standard package lines feed the summary.
  `-json` stays available as a pass-through for a separate diagnostic run
  (`mode=json-unparsed`).

### 19.5 Rejected alternatives

| ID | Alternative | Why rejected |
|---|---|---|
| O1 | Status quo (Go default 10m) | Already exceeded (~760s needed) |
| O2 | Scattered literal `-timeout=30m` | Unexplained constant drifting across ~20 surfaces; no growth adaptation |
| O3 | `-timeout=0` | Removes the finite hang detector |
| O4 | `GOFLAGS` / `go env -w` | Machine-local and unversioned; dev, Ship, and CI diverge |
| O5 | `TestMain` override of `test.timeout` | `cmd/go` always injects `-test.timeout` and kills at timeout + 1m; hidden and fragile |
| O6 | Partition `internal/core`, reduce fixture I/O, or cache templates | Performance refactor excluded by the operator's scope; does not adapt durably; captured as a trigger-gated follow-up |
| O7 | Committed measured-baseline ratchet | Commits machine-specific numbers, needs large `-json` captures, churns |
| O8 | Platform-specific budgets | A run could pass in CI and time out locally |
| O9 | Fail-fast guard against bare runs at runtime | Breaks legitimate bare runs on fast hosts; the static coupling test covers governed surfaces instead |
| O10 | `go test -json ./...` as the gate form | Capture size; the calibration uses standard package lines |

### 19.6 Revisit criteria (constants change only via a Stage deliberation citing ≥ 2 governed-run summaries)

* **R1** — `TEST_BUDGET_WARN ... revisit=R1` on any governed run. Stage re-measures
  PerTestReference from the summaries.
* **R2** — `ratio_to_reference > 1.25`. Investigate a per-test cost regression first; do not just
  raise the budget.
* **R3** — N_max ≥ 1200 (80% of the ceiling) or `TEST_BUDGET_CEILING_EXCEEDED`. Deliberate the
  partition/performance follow-up, or a reviewed ceiling amendment.
* **R4** — A new governed platform, or `-race` on Windows (make.ps1). Measure before treating the
  budget as calibrated.
* **R5** — A timeout whose running test is ≥ 60s old is a genuine hang or slow-test defect. Fix
  the test; never raise the budget for it.

### 19.7 Work units (18 tasks under `174-F`; each ≤ 2 files, < 5 functions, < 4 scenarios, single domain)

**Function limit.** It counts production functions and methods created or given a body in the
task. A table test is one scenario.

**RED selectors are scoped.** Every compile check is `go vet` on the named package plus a scoped
`-run '^$'`; see deviation D1.

**U19a `174.074-T` — Declare internal/testbudget policy surface**

* Tests domain first, then declaration. Non-exempt; source-shape harness per P-002.1. Wave 1.
  No dependencies.
* Files:
  * `internal/testbudget/budget_shape_test.go` (new; imports only `go/ast`, `go/parser`,
    `go/token`, `os`, `path/filepath`, `strings`, `testing`);
  * `internal/testbudget/budget.go` (new).
* Harness `TestBudgetSourceShape` asserts over `budget.go` source:
  * the package doc mentions `073-DL`;
  * the 6 constants exist with the 19.4.1 value expressions;
  * `var ErrCeilingExceeded` exists;
  * `type PackageCount struct { Dir string; Tests int }` exists;
  * these signatures exist: `func Compute(n int) (time.Duration, error)`,
    `func CountTopLevelTests(dir string) (int, error)`,
    `func Largest(root string) (PackageCount, error)`.
* RED: before `budget.go` exists, the harness compiles and fails on assertion ("budget.go not
  found / Compute not declared").
* Declaration bodies are non-behavioral: `return 0, errors.New("testbudget: not implemented")`,
  or the zero struct plus that error.
* AC:
  1. `go vet ./internal/testbudget` exits 0.
  2. `go test ./internal/testbudget -run '^TestBudgetSourceShape$' -count=1 -v` shows exactly 1
     top-level PASS.
  3. No behavior is implemented.
  4. `gofmt -l internal/testbudget` is empty.

**U19b `174.075-T` — Implement testbudget Compute, counter, and Largest**

* Tests first. Non-exempt. Wave 2. Depends on `174.074-T`.
* Files: `internal/testbudget/budget_test.go` (new), `internal/testbudget/budget.go`.
* Functions: `Compute`, `CountTopLevelTests`, `Largest`, plus at most one unexported helper
  (`isTestFunc`).
* Harness (compiles against the declarations, fails on assertion). All fixtures are written into
  `t.TempDir()`; no `testdata` files are committed.
  * `TestCompute_Table`: the 19.4.2 rows.
  * `TestCountTopLevelTests`: a fixture with
    * a counted `TestA(t *testing.T)`, `TestB(t *T)` (declared with a local `type T = testing.T`; cmd/go matches `*T` by name), and
      `FuzzC(f *testing.F)`;
    * non-counted `TestMain(m *testing.M)`, `Testlower(t *testing.T)`, `TestD()`,
      `TestE(t *testing.T) error`, `TestH(a, b *testing.T)`, and a method
      `(s S) TestF(t *testing.T)`;
    * a `helper.go` (non-test file) containing `func TestG(t *testing.T)`.
    * The count is exactly 3.
  * `TestLargest`, with subtests:
    * `fixture`: two directories plus skipped `testdata`, `vendor`, `_x`, `.y`, and a nested
      module. The largest is reported with a relative slash `Dir`.
    * `empty`: returns an error.
    * `dot-root`: under `t.Chdir(<fixture root with go.mod and one test file>)`, `Largest(".")`
      returns `Dir == "."` with the root's count (the `.`-prefix skip rule does not apply to the
      root).
    * `repository`: `Largest(<module root>)` has `Dir == "internal/core"` and `Tests >= 500`.
      This is a non-vacuity floor, not an exact count.
* AC:
  1. The RED evidence (assertion failures, compile PASS) is captured under
     `logs/diagnostics/174075-*`.
  2. GREEN: `go test ./internal/testbudget -count=1 -v` shows exactly 4 top-level PASS
     (including the shape test).
  3. `go vet ./internal/testbudget` exits 0.
  4. The package doc carries provenance, the formula, and R1-R5.

**Shape-harness imports (174.074-T, 174.076-T, 174.077-T).** Each harness imports exactly
`go/ast`, `go/parser`, `go/token`, `os`, `path/filepath`, `strings`, and `testing`. That goes
beyond the four packages P-002.1 lists as its example. `os` and `path/filepath` locate and read
the source file; `strings` checks doc comments and literals. None of them names an identifier
of the package under test, so the harness compiles before the declaration exists (a package
made only of test files compiles under `go vet` and `go test`).

**U19c `174.076-T` — Declare cmd/test-budget run surface**

* Source-shape harness first. Non-exempt. Wave 1. No dependencies.
* Files: `cmd/test-budget/main_shape_test.go` (new), `cmd/test-budget/main.go` (new).
* Harness `TestWrapperRunSourceShape` asserts these declarations in `main.go`:
  * `type goRunner func(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error)`;
  * `var defaultRunner goRunner = execGo`;
  * `var hostGOOS = runtime.GOOS`;
  * `func main()`, whose body calls `os.Exit(run(...))` with `defaultRunner`;
  * `func run(ctx context.Context, args []string, stdout, stderr io.Writer, runner goRunner) int`;
  * `func buildGoTestArgs(d time.Duration, args []string) ([]string, error)`;
  * `func execGo(ctx context.Context, args []string, stdout, stderr io.Writer) (int, error)`.
* Functions: 4 (`main`, `run`, `buildGoTestArgs`, `execGo`).
* Stub bodies are non-behavioral:
  * `buildGoTestArgs` returns `nil, errors.New("test-budget: not implemented")`;
  * `execGo` returns `2, errors.New(...)`;
  * `run` is exactly: `_ = hostGOOS`, then
    `if _, err := buildGoTestArgs(0, args); err != nil { return 2 }`, then `return 2`. The
    checked error keeps errcheck quiet.
  * This keeps golangci-lint `unused` (default linter set; there is no `.golangci.yml`) quiet
    without any directive.
* AC:
  1. `go vet ./cmd/test-budget` exits 0.
  2. `go test ./cmd/test-budget -run '^TestWrapperRunSourceShape$' -count=1 -v` shows exactly 1
     top-level PASS.
  3. `gofmt -l cmd/test-budget` is empty.
  4. `golangci-lint run ./cmd/test-budget/...` reports nothing.

**U19d `174.077-T` — Declare cmd/test-budget result sink and exit-mapping surface**

* Source-shape harness first. Non-exempt. Wave 1. No dependencies.
* Files: `cmd/test-budget/result_shape_test.go` (new), `cmd/test-budget/result.go` (new).
* Harness `TestResultSinkSourceShape` asserts these declarations in `result.go`:
  * `type resultSink struct{}` (no fields; 174.078-T adds them);
  * `func (s *resultSink) Write(p []byte) (int, error)`;
  * `func (s *resultSink) footer(budget time.Duration, largestImportPath string, largestTests int, jsonMode bool) []string`;
  * `func mapWaitResult(code int, err error) (int, error)`;
  * the anchors `var _ io.Writer = (*resultSink)(nil)`, `var _ = (*resultSink).footer`, and
    `var _ = mapWaitResult`. 174.078-T removes the last two once its tests call them
    (golangci-lint runs with tests included by default, so a test call counts as a use).
* Functions: 3 (`Write`, `footer`, `mapWaitResult`).
* Stub bodies are non-behavioral:
  * `Write` returns `len(p), nil` without parsing (the `io.Writer` contract only);
  * `footer` returns `nil`;
  * `mapWaitResult` returns `2, errors.New("test-budget: not implemented")`.
* AC:
  1. `go vet ./cmd/test-budget` exits 0.
  2. `go test ./cmd/test-budget -run '^TestResultSinkSourceShape$' -count=1 -v` shows exactly 1
     PASS.
  3. `gofmt -l cmd/test-budget` is empty.
  4. `golangci-lint run ./cmd/test-budget/...` reports nothing.

**U19e `174.078-T` — Implement test-budget result sink, footer, and exit mapping**

* Tests first. Non-exempt. Wave 2. Depends on `174.074-T` (constants) and `174.077-T`.
* Files: `cmd/test-budget/result_test.go` (new), `cmd/test-budget/result.go`.
* Functions given bodies: 3 (`Write`, `footer`, `mapWaitResult`). `footer` reads
  `testbudget.Headroom` and `testbudget.PerTestReference`. It adds the `resultSink` fields and
  deletes the `var _ = (*resultSink).footer` and `var _ = mapWaitResult` anchors; the
  `io.Writer` assertion stays.
* `mapWaitResult(code, err)` contract (pure; `code` is the caller's
  `cmd.ProcessState.ExitCode()`, or -1 when there is no state):

| Input | Returns |
|---|---|
| `(0, nil)` | `(0, nil)` |
| `(c, *exec.ExitError)` with c ≥ 0 | `(c, nil)` |
| `(c, *exec.ExitError)` with c < 0 (signal) | `(1, nil)` |
| `errors.Is(err, exec.ErrWaitDelay)` (any code) | `(1, err)`, fail closed |
| any other non-nil `err` | `(1, err)` |
* `Write` contract:
  * It **always returns `len(p), nil`**, even when it discards bytes. A short write or an error
    would stop the `os/exec` copy goroutine and break the child's pipe.
  * It buffers a partial line (capped at 64 KiB; the rest of an over-long line is discarded up
    to the next newline), strips a trailing `\r`, and matches each completed line against
    `^(ok|FAIL)\s+(\S+)\s+([0-9]+(?:\.[0-9]+)?)s(?:\s|$)`.
  * It records the raw elapsed token for each package.
* Harness:
  * `TestResultSink_Write` (table) covers:
    * a line split across two writes;
    * CRLF input;
    * `ok  pkg (cached)` ignored;
    * `?   pkg [no test files]` ignored;
    * `FAIL pkg 602.717s` parsed, with the raw token kept;
    * `FAIL pkg [build failed]` ignored;
    * `ok  pkg 0.123s  coverage: 50.0% of statements` parsed;
    * a 70 KiB line: discarded, `Write` still returns `len(p), nil`, and the next line parses.
  * `TestResultSink_Footer` (table) covers:
    * the slowest package and its raw elapsed;
    * `budget_use` percent;
    * `revisit=R1` when a package elapsed exceeds `budget / Headroom`;
    * `revisit=R2` when `ratio_to_reference > 1.25`;
    * the largest package matched by **exact** import path (`module` alone, or
      `module + "/" + Dir`), where `example.com/m/x/internal/core` does not match
      `internal/core`, and an unmatched package prints `n/a`;
    * nothing parsed (a fully cached run): `slowest_package=n/a`, no percentages, no WARN;
    * `jsonMode`: exactly `TEST_BUDGET_SUMMARY: mode=json-unparsed`.
  * `TestMapWaitResult` (table): `(0, nil)` → `(0, nil)`; `(3, &exec.ExitError{})` → `(3, nil)`;
    `(-1, &exec.ExitError{})` → `(1, nil)`; `(0, fmt.Errorf("wait: %w", exec.ErrWaitDelay))` →
    `(1, err)` with `errors.Is(err, exec.ErrWaitDelay)`; `(-1, errors.New("copy"))` → `(1, err)`.
* AC:
  1. RED evidence (compile PASS, assertion failures) is captured under
     `logs/diagnostics/174078-*`.
  2. GREEN: `go test ./cmd/test-budget -run '^(TestResultSink_Write|TestResultSink_Footer|TestMapWaitResult|TestResultSinkSourceShape)$' -count=1 -v`
     shows exactly 4 top-level PASS.
  3. `go vet ./cmd/test-budget` exits 0.
  4. `golangci-lint run ./cmd/test-budget/...` reports nothing.

**U19f `174.079-T` — Implement test-budget run, exec, and header**

* Tests first. Non-exempt. Wave 3. Depends on `174.075-T`, `174.076-T`, `174.078-T`.
* Files: `cmd/test-budget/main_test.go` (new), `cmd/test-budget/main.go`. `result.go` is not
  touched (174.078-T already removed its anchors).
* Functions given bodies: 3 (`run`, `buildGoTestArgs`, `execGo`). `main` is unchanged.
* `run` contract:
  * Subcommands are `print` and `run`; anything else (including no subcommand) returns 2 with
    usage.
  * It rejects `-timeout`, `--timeout`, `-test.timeout`, and `--test.timeout`, in `=` and
    separate-value forms: exit 2 and `TEST_BUDGET_ERROR: -timeout is governed by 073-DL`.
  * It requires `go.mod` in the working directory and reads its `module` path; otherwise exit 2.
  * It calls `testbudget.Largest(".")`. An error or `Tests == 0` returns 2.
  * It calls `testbudget.Compute`. `ErrCeilingExceeded` clamps the budget to 45m and marks the
    run for exit 3.
  * It prints the header plus the R3, R4 (`-race` && `hostGOOS == "windows"`), and CEILING
    lines.
  * `print` returns 0, or 3 at the ceiling.
  * `run` builds `w := io.MultiWriter(stdout, sink)` and passes **the same `w` as both stdout
    and stderr**. `os/exec` then uses one pipe and one copy goroutine, because it compares the
    two writers with `interfaceEqual`. There is no concurrent `Write` and no race.
  * The terminal receives the child's raw bytes; only the sink normalizes lines.
  * After the child exits, `run` prints `footer(...)` lines to stdout.
* Exit mapping (`run` return values):

| Runner returns | `run` returns |
|---|---|
| `(0, nil)` | 0, or 3 at the ceiling |
| `(c, nil)` with c ≠ 0 | c |
| any `(c, err)` | c, and prints `TEST_BUDGET_ERROR: <err>` |

* `execGo` contract:
  * It uses `exec.CommandContext(ctx, "go", args...)` with `Stdout`/`Stderr` as given and
    `cmd.WaitDelay = 10 * time.Second`.
  * A `Start` failure returns `(2, fmt.Errorf("start go: %w", err))` inline. After `Wait`, it
    returns `mapWaitResult(cmd.ProcessState.ExitCode(), err)` (or code -1 when
    `ProcessState` is nil), so every row of the table below is unit-tested in 174.078-T.
  * It registers `signal.Notify(ch, os.Interrupt)` on a buffered channel of size 1, with
    `defer signal.Stop(ch)`, and starts no goroutine. This only stops the wrapper from dying
    before the child. The terminal or console delivers the interrupt to the child itself, and
    nothing is forwarded or killed.
  * Returns:

| Outcome | `execGo` returns |
|---|---|
| `Start` failure | `(2, err)` |
| `*exec.ExitError` | `(ExitCode(), nil)`; a negative code (signal) becomes 1 |
| `errors.Is(err, exec.ErrWaitDelay)` (a descendant kept the output open after the child exited) | `(1, err)`, fail closed |
| success | `(0, nil)` |

* Harness:
  * `TestRun_DerivationAndPrint` (table; fixtures are generated in `t.TempDir()`; the working
    directory is set with `t.Chdir`, available since Go 1.24; no `t.Parallel`), rows:
    * an unknown subcommand (`frobnicate`) and an empty argument list each return 2 with usage;
    * `print` on a module with 2 tests writes exactly
      `TEST_BUDGET: timeout=10m0s largest_package=. top_level_tests=2 policy=073-DL` and
      returns 0;
    * the four timeout-flag spellings each return 2;
    * no `go.mod` returns 2;
    * a zero-test module returns 2;
    * a generated 1200-test package prints `revisit=R3` and `timeout=40m0s`;
    * a generated 1501-test package prints `TEST_BUDGET_CEILING_EXCEEDED` and
      `timeout=45m0s`, and returns 3.
  * `TestRun_ExecutionAndExitMapping`, with a fake `goRunner`:
    * the recorded vector equals `["test", "-timeout=10m0s", "./..."]`;
    * the stdout and stderr the fake receives are the same writer;
    * header, then child output, then `TEST_BUDGET_SUMMARY`, in that order;
    * the fake returning `(1, nil)` gives 1;
    * `(2, errors.New("start"))` gives 2 and `TEST_BUDGET_ERROR`;
    * the 1501-test fixture with a `(0, nil)` fake gives 3;
    * `hostGOOS = "windows"` with `-race` prints `revisit=R4`; `hostGOOS` is restored with
      `t.Cleanup`;
    * the real `execGo` under `t.Setenv("PATH", t.TempDir())` (no `go` resolvable) returns code
      2 and a non-nil error wrapping the start failure.
  * `TestGoTestTimeout_BoundedHangIsDetected` (real toolchain, bounded):
    * `t.Chdir` into a `t.TempDir()` module whose `go.mod` pins `go 1.24` and whose single test
      blocks on `select {}`;
    * `t.Setenv("GOWORK","off")`, `t.Setenv("GOFLAGS","")`, and
      `t.Setenv("GOTOOLCHAIN","local")`, which the child inherits. Production `execGo` inherits
      the caller environment unchanged;
    * a 2-minute context;
    * builds `args, err := buildGoTestArgs(3*time.Second, []string{"-count=1", "./..."})`,
      fails the test on `err`, then runs `execGo(ctx, args, &buf, &buf)`;
    * asserts a non-zero code, `panic: test timed out after 3s` in the output, and a return
      before the context deadline;
    * no `t.Parallel`.
* AC:
  1. RED evidence is captured under `logs/diagnostics/174079-*`.
  2. GREEN: `go test ./cmd/test-budget -count=1 -timeout=5m -v` shows exactly 8 top-level PASS
     (the 2 shape tests, the 3 result tests, and the 3 run tests).
  3. `go run ./cmd/test-budget print` from the repo root prints
     `TEST_BUDGET: timeout=30m0s largest_package=internal/core top_level_tests=<885|886> policy=073-DL`
     and exits 0. It is captured as evidence; this is derivation only and no suite runs.
  4. `go vet ./cmd/test-budget ./internal/testbudget` exits 0.
  5. `golangci-lint run ./cmd/test-budget/... ./internal/testbudget/...` reports nothing.
  6. `go build ./cmd/backlogit` is unaffected.

**U19g `174.080-T` — RED contract test coupling governed test-budget surfaces**

* Tests domain. Non-exempt **red deliverable**. Wave 4. Depends on `174.079-T`.
* File: `tests/integration/test_budget_coupling_test.go` (new, package `integration_test`).
  * It reuses the existing `findRepoRoot`, `readFileString`, `readCIWorkflow`, and `findStep`
    helpers from `ci_compliance_test.go`.
  * It adds 2 local helpers: `bareFullSuiteForms(text) []string` and
    `governedSpans(text) []string`.
  * `canonicalFull` and `canonicalCI` are interpreted-string constants.
* **Bare-form detector** (tokenizing, not a single regex). All text is CRLF-normalized first.
  * **Occurrence.** `go test` counts only at the start of a line or after a non-word character
    (so `cargo test` is ignored).
  * **Continuation.** A line whose trimmed text ends in ` \` is joined with the next line before
    spans are cut.
  * **Span.** For each occurrence of `go test`: if the character immediately before it is a
    backtick, `"`, or `'`, the span ends at the next matching character. Otherwise the span
    runs to the end of the (joined) line.
  * **Tokens.** The span is split on whitespace. Surrounding quotes are stripped from each
    token, and trailing `,;:)` is trimmed.
  * **Selector.** The `-run` value is read from any of `-run=V`, `-run V`, `-test.run=V`, or
    `-test.run V`, with quotes stripped from V.
  * **Full-suite form.** A token starts with `./...`, or equals `./internal/core` or
    `./internal/core/...`.
  * **Exempt (compile-only).** The selector value is exactly `^$`.
  * **Bare.** A full-suite form with no selector, or with selector `.`, `.*`, or empty.
  * **Targeted and allowed.** Any other selector value.
  * **Governed span.** `go run ./cmd/test-budget run` through the end of its span. It fails if
    any token equals `-timeout`, `--timeout`, `-test.timeout`, or `--test.timeout`, or starts
    with one of them followed by `=`.
  * **Allowlisted lines:**
    * lines carrying the literal marker `test-budget:bare-mention` (at most 1 per file; a second
      marker fails);
    * workflow-policies amendment-log table rows;
    * `harness-manifest.yaml` lines whose trimmed text starts with `note:`, `drift_reason:`, or
      `checksum:`.
* **`TestGovernedTestBudgetSurfaces`** is table-driven (one scenario): each row carries the
  subtest name, the files, and a predicate function, and `t.Run` executes one subtest per row:

| Subtest | Unit | Requirements |
|---|---|---|
| `ci-workflows` | 174.081-T | ci `Test` step `run` == canonicalCI; release `Test` step `run` == canonicalFull |
| `pre-push` | 174.082-T | the Test gate's third argument == canonicalFull in both scripts; the Build gate stays compile-only |
| `make` | 174.083-T | the line after `test:` in `Makefile` == `"\t"+canonicalCI`; every `Step "test"` block in `make.ps1` wraps canonicalCI (≥ 1 found) |
| `ship-gate` | 174.084-T | `_ship.agent.md` and build-feature each contain canonicalFull and no bare form |
| `policy` | 174.085-T | workflow-policies contains canonicalFull, no bare form, and an amendment-log row containing `073-DL` |
| `fix-ci` | 174.086-T | fix-ci SKILL contains a governed span; neither it nor github-pr-automation has a bare form |
| `constitution` | 174.087-T | AGENTS.md `Constitution version: X` == constitution `**Version**: X`; the constitution has an amendment record containing `073-DL`; both files contain canonicalFull or canonicalCI and no bare form |
| `instructions` | 174.088-T | copilot-instructions and go.instructions each contain a governed span and no bare form |
| `engineer-readme` | 174.089-T | go-engineer and README each contain canonicalFull and no bare form |
| `review-instructions` | 174.090-T | both review-instruction files contain a governed span and no bare form; `copilot-code-review.instructions.md` contains the literal `summaryLine` |
| `harness-inputs` | 174.091-T | the `variables_used:` block of `harness-manifest.yaml` is found and non-empty; its `TEST_COMMAND` value == canonicalFull; the block has no bare form outside the allowlist; workspace-profile has no bare form and contains canonicalFull; the block contains the literal `summaryLine` (below) |

* `summaryLine` is the interpreted-string constant
  ``"Test-first red-green gate mechanics (P-002, P-004) and the `go run ./cmd/test-budget run ./...` suite run"``.
  It is the text `HARNESS_ENFORCED_SUMMARY` (manifest line 596) renders into
  `copilot-code-review.instructions.md` line 53, so 174.090-T and 174.091-T write identical text.
* No subtest pins a budget value, a version number, or an exact occurrence count.
* **`TestGovernedTestBudgetExecutableSweep`**:
  * Subtest `detector`: a table of about 19 literal rows. It is GREEN from landing:

| Row | Expected |
|---|---|
| `go test ./...` | bare |
| a backticked `go test ./...` | bare |
| `Invoke-Gate "Test" "go" "go test ./..."` | bare |
| `go test -race -coverprofile=coverage.out ./...` | bare |
| `go test -timeout=30m ./...` | bare |
| `go test -run . ./...` | bare |
| `go test -run=".*" ./...` | bare |
| `go test ./internal/core` | bare |
| `go test ./internal/core/...` | bare |
| `go test -run=^$ -count=1 ./...` | exempt |
| `go test -run '^$' ./...` | exempt |
| `go test ./internal/core/... ./internal/events/... -run "ItemLog\|ShipmentReconcile" -count=1 -v` (the literal `ci.yml:165` line) | allowed |
| `go test ./tests/integration -run '^TestX$'` | allowed |
| canonicalFull | governed |
| `go run ./cmd/test-budget run -timeout=1m ./...` | governed-with-timeout, fails |
| `go vet ./...` | ignored |
| `cargo test ./...` | ignored |
| `go test -count=1 \` followed by a line `  ./...` | bare (continuation joined) |
| `go run ./cmd/test-budget run --test.timeout=1m ./...` | governed-with-timeout, fails |

  * Subtest `executables`: every file under `.github/workflows/`, every `*.ps1` and `*.sh`
    under `scripts/`, `Makefile`, and `make.ps1` has zero bare forms. Each of the 6 known
    executable files (`ci.yml`, `release.yml`, the 2 pre-push scripts, `Makefile`, `make.ps1`)
    has ≥ 1 governed span.
  * (Grep evidence at planning time: under `scripts/`, only the two pre-push Test lines are
    full-suite forms. The two Build lines are compile-only.)
* Red-deliverable contract:

```text
red_deliverable: true
red_deliverable_reason: Deliverable IS the persistent RED coupling contract for the 073-DL governed test budget; it lands red by design (every surface still runs the bare default-timeout form) and is driven green only by the 11 Wave 19 surface migrations.
red_selector_command: go test ./tests/integration -run '^(TestGovernedTestBudgetSurfaces|TestGovernedTestBudgetExecutableSweep)$' -count=1 -timeout=5m -v
green_maker_tasks: 174.081-T, 174.082-T, 174.083-T, 174.084-T, 174.085-T, 174.086-T, 174.087-T, 174.088-T, 174.089-T, 174.090-T, 174.091-T
green_maker_closes_wave: 5
```

* AC:
  1. `go vet ./tests/integration` exits 0.
  2. The red selector compiles and fails with exactly the 11 surface subtests plus
     `executables` failing and `detector` passing. This is captured to
     `logs/diagnostics/174080-red-harness.txt` plus `.metadata.json` with `Compilation: PASS`
     and `Red Phase: CONFIRMED`.
  3. `go test ./tests/integration -run '^TestGovernedTestBudgetExecutableSweep$/^detector$' -count=1 -v`
     PASSes.
  4. `golangci-lint run ./tests/integration/...` reports nothing new.
  5. No surface file is edited.

**Surface units.** All are Wave 5 and depend on `174.080-T`. Each edits only the listed files,
only the lines named in 19.2, and the version and amendment lines named in its own row, replacing
the bare form with a governed span. Prose around the command may be adjusted in the same
sentence. Task bodies carry each contract block below verbatim, wrapped in
`<!-- BEGIN:harness-exemption-contract -->` / `<!-- END:harness-exemption-contract -->` (and
174.080-T in `<!-- BEGIN:red-deliverable-contract -->` / `<!-- END:red-deliverable-contract -->`).

| Unit | ID | Title | Files | Class |
|---|---|---|---|---|
| U19h | 174.081-T | Route CI and release Test steps through test-budget | `ci.yml` (canonicalCI), `release.yml` (canonicalFull). The `Test` step `if:` stays unchanged (pinned by `TestHeavyStepsAreFailSafeGated`); `TestReleaseWorkflowDropsRaceMatrix` stays green | covered-by `174.080-T` |
| U19i | 174.082-T | Route pre-push Test gates through test-budget | `scripts/pre-push-quality-gates.ps1`, `.sh`: the Test gate becomes canonicalFull; the probe argument stays `go`; the Build gate is unchanged | covered-by `174.080-T` |
| U19j | 174.083-T | Route Makefile and make.ps1 test through test-budget | `Makefile`, `make.ps1` (canonicalCI; keep `-race`; the R4 WARN is expected on Windows `make.ps1 test`) | covered-by `174.080-T` |
| U19k | 174.084-T | Govern Ship final-gate full-suite command | `.github/agents/_ship.agent.md` (950/989), `.github/skills/build-feature/SKILL.md` (355/365) | docs-only |
| U19l | 174.085-T | Govern workflow-policies full-suite rule (1.30.0) | `.github/policies/workflow-policies.md` (916/925/1149; version 1.29.0 → 1.30.0; amendment row citing 073-DL and 11BE840F) | docs-only |
| U19m | 174.086-T | Govern fix-ci and PR automation test commands | `.github/skills/fix-ci/SKILL.md` (13/72/126/189), `.github/instructions/github-pr-automation.instructions.md` (126) | docs-only |
| U19n | 174.087-T | Govern constitution quality gate and AGENTS (1.1.0) | `.github/instructions/constitution.instructions.md` (22/209/221; version 1.0.0 → 1.1.0), `AGENTS.md` (16/43/239/252) | docs-only |
| U19o | 174.088-T | Govern copilot and Go instruction test commands | `.github/copilot-instructions.md` (13/47), `.github/instructions/go.instructions.md` (35 becomes canonicalFull; 36 becomes `go run ./cmd/test-budget run -coverprofile=coverage.out ./...`) | docs-only |
| U19p | 174.089-T | Govern go-engineer loop and README test command | `.github/agents/subagents/go-engineer.agent.md` (76 becomes canonicalFull, 1:1; 74 stays compile-only), `README.md` (283) | docs-only |
| U19q | 174.090-T | Govern review-instruction test commands | `.github/copilot-review-instructions.md` (195 becomes canonicalFull), `.github/instructions/copilot-code-review.instructions.md` (53 becomes `* ` + `summaryLine`) | docs-only |
| U19r | 174.091-T | Govern autoharness test-command render inputs | `.autoharness/harness-manifest.yaml` (556/596/633 only, 596 becoming `    * ` + `summaryLine`; 461 is historical), `.autoharness/workspace-profile.yaml` (27/47) | covered-by `174.080-T` |

**174.087-T constitution amendment AC** (constitution Governance: version bump, rationale, and
sync impact report):

* The `**Version**` line reads `1.1.0`, with an amended date.
* It adds an amendment record containing `073-DL`, with:
  * **rationale:** provenance of the unowned 10m default, and the count-scaled governed
    budget;
  * **sync impact report:** AGENTS.md; workflow-policies 1.30.0; the Ship agent and
    build-feature; fix-ci and PR automation; copilot, go, and review instructions; the
    go-engineer agent and README; CI, release, pre-push, and make; the `.autoharness` render
    inputs. Pending: the upstream template and plugin parity (P-021 (c)).
* The Technical Constraints `Test` row and the Quality Gates block use canonicalFull.
* Principle II line 22 reads "pass via `go run ./cmd/test-budget run ./...`".
* AGENTS.md line 16 reads `Constitution version: 1.1.0`.

**Closed exempt set (exactly these 11 IDs; rendered contract blocks).** Every block follows the
P-002.1 grammar: the five canonical keys in order, plus `harness_owner_command` for
`covered-by`.

* docs-only tasks probe content (their own subtest `--- PASS:` line), then run the doc lint gate
  on each edited Markdown file (P-002.3).
* covered-by tasks probe the owner subtest `--- PASS:` line, then their own deliverable literal.
* Every task is labelled `harness-exempt`.
* The commands use a single-quoted outer `pwsh -NoProfile -Command '...'` with doubled inner
  quotes. All 15 commands were syntax-checked with the PowerShell parser during planning, and
  none was executed.
* **must-fail-before-deliverable.** Before `174.080-T` lands, the subtest does not exist, so no
  PASS line is printed and `no tests to run` appears. After it lands and before the migration,
  the subtest fails.

**174.081-T** (`ci-workflows`)

```text
harness_exemption_class: covered-by
harness_exemption_reason: Edits only the CI and release Test-step run lines; the behavior is pinned by owner 174.080-T subtest ci-workflows, so a second harness would duplicate the owner contract.
harness_owner: 174.080-T
exempt_verification_command: pwsh -NoProfile -Command '$o = go test ./tests/integration -run ''^TestGovernedTestBudgetSurfaces$/^ci-workflows$'' -count=1 -timeout=5m -v 2>&1 | Out-String; $rc = $LASTEXITCODE; Write-Output $o; if ($rc -ne 0) { exit 1 }; if (-not $o.Contains(''--- PASS: TestGovernedTestBudgetSurfaces/ci-workflows'')) { exit 1 }; if (-not (Get-Content -Raw ''.github/workflows/ci.yml'').Contains(''go run ./cmd/test-budget run -race -coverprofile=coverage.out ./...'')) { exit 1 }; if (-not (Get-Content -Raw ''.github/workflows/release.yml'').Contains(''go run ./cmd/test-budget run ./...'')) { exit 1 }; Write-Output ''EXEMPT_VERIFY_OK:174.081-T''; exit 0'
exempt_precondition: must-fail-before-deliverable
harness_owner_command: pwsh -NoProfile -Command '$o = go test ./tests/integration -run ''^TestGovernedTestBudgetSurfaces$/^ci-workflows$'' -count=1 -timeout=5m -v 2>&1 | Out-String; $rc = $LASTEXITCODE; Write-Output $o; if ($rc -ne 0 -or $o.Contains(''no tests to run'') -or -not $o.Contains(''--- PASS: TestGovernedTestBudgetSurfaces/ci-workflows'')) { exit 1 }; exit 0'
```

**174.082-T** (`pre-push`)

```text
harness_exemption_class: covered-by
harness_exemption_reason: Edits only the pre-push Test gate command strings; the behavior is pinned by owner 174.080-T subtest pre-push, so a second harness would duplicate the owner contract.
harness_owner: 174.080-T
exempt_verification_command: pwsh -NoProfile -Command '$o = go test ./tests/integration -run ''^TestGovernedTestBudgetSurfaces$/^pre-push$'' -count=1 -timeout=5m -v 2>&1 | Out-String; $rc = $LASTEXITCODE; Write-Output $o; if ($rc -ne 0) { exit 1 }; if (-not $o.Contains(''--- PASS: TestGovernedTestBudgetSurfaces/pre-push'')) { exit 1 }; if (-not (Get-Content -Raw ''scripts/pre-push-quality-gates.ps1'').Contains(''go run ./cmd/test-budget run ./...'')) { exit 1 }; if (-not (Get-Content -Raw ''scripts/pre-push-quality-gates.sh'').Contains(''go run ./cmd/test-budget run ./...'')) { exit 1 }; Write-Output ''EXEMPT_VERIFY_OK:174.082-T''; exit 0'
exempt_precondition: must-fail-before-deliverable
harness_owner_command: pwsh -NoProfile -Command '$o = go test ./tests/integration -run ''^TestGovernedTestBudgetSurfaces$/^pre-push$'' -count=1 -timeout=5m -v 2>&1 | Out-String; $rc = $LASTEXITCODE; Write-Output $o; if ($rc -ne 0 -or $o.Contains(''no tests to run'') -or -not $o.Contains(''--- PASS: TestGovernedTestBudgetSurfaces/pre-push'')) { exit 1 }; exit 0'
```

**174.083-T** (`make`)

```text
harness_exemption_class: covered-by
harness_exemption_reason: Edits only the Makefile test recipe and make.ps1 test steps; the behavior is pinned by owner 174.080-T subtest make, so a second harness would duplicate the owner contract.
harness_owner: 174.080-T
exempt_verification_command: pwsh -NoProfile -Command '$o = go test ./tests/integration -run ''^TestGovernedTestBudgetSurfaces$/^make$'' -count=1 -timeout=5m -v 2>&1 | Out-String; $rc = $LASTEXITCODE; Write-Output $o; if ($rc -ne 0) { exit 1 }; if (-not $o.Contains(''--- PASS: TestGovernedTestBudgetSurfaces/make'')) { exit 1 }; if (-not (Get-Content -Raw ''Makefile'').Contains(''go run ./cmd/test-budget run -race -coverprofile=coverage.out ./...'')) { exit 1 }; if (-not (Get-Content -Raw ''make.ps1'').Contains(''go run ./cmd/test-budget run -race -coverprofile=coverage.out ./...'')) { exit 1 }; Write-Output ''EXEMPT_VERIFY_OK:174.083-T''; exit 0'
exempt_precondition: must-fail-before-deliverable
harness_owner_command: pwsh -NoProfile -Command '$o = go test ./tests/integration -run ''^TestGovernedTestBudgetSurfaces$/^make$'' -count=1 -timeout=5m -v 2>&1 | Out-String; $rc = $LASTEXITCODE; Write-Output $o; if ($rc -ne 0 -or $o.Contains(''no tests to run'') -or -not $o.Contains(''--- PASS: TestGovernedTestBudgetSurfaces/make'')) { exit 1 }; exit 0'
```

**174.084-T** (`ship-gate`)

```text
harness_exemption_class: docs-only
harness_exemption_reason: Replaces the bare full-suite command in Ship agent and build-feature instruction text only; no production or test code changes.
harness_owner: none
exempt_verification_command: pwsh -NoProfile -Command '$o = go test ./tests/integration -run ''^TestGovernedTestBudgetSurfaces$/^ship-gate$'' -count=1 -timeout=5m -v 2>&1 | Out-String; $rc = $LASTEXITCODE; Write-Output $o; if ($rc -ne 0) { exit 1 }; if (-not $o.Contains(''--- PASS: TestGovernedTestBudgetSurfaces/ship-gate'')) { exit 1 }; foreach ($f in @(''.github/agents/_ship.agent.md'',''.github/skills/build-feature/SKILL.md'')) { go run ./cmd/backlogit docs lint --path $f --no-update-check; if ($LASTEXITCODE -ne 0) { exit 1 } }; Write-Output ''EXEMPT_VERIFY_OK:174.084-T''; exit 0'
exempt_precondition: must-fail-before-deliverable
```

**174.085-T** (`policy`)

```text
harness_exemption_class: docs-only
harness_exemption_reason: Amends workflow-policies full-suite rule text, version, and amendment log only; no production or test code changes.
harness_owner: none
exempt_verification_command: pwsh -NoProfile -Command '$o = go test ./tests/integration -run ''^TestGovernedTestBudgetSurfaces$/^policy$'' -count=1 -timeout=5m -v 2>&1 | Out-String; $rc = $LASTEXITCODE; Write-Output $o; if ($rc -ne 0) { exit 1 }; if (-not $o.Contains(''--- PASS: TestGovernedTestBudgetSurfaces/policy'')) { exit 1 }; foreach ($f in @(''.github/policies/workflow-policies.md'')) { go run ./cmd/backlogit docs lint --path $f --no-update-check; if ($LASTEXITCODE -ne 0) { exit 1 } }; Write-Output ''EXEMPT_VERIFY_OK:174.085-T''; exit 0'
exempt_precondition: must-fail-before-deliverable
```

**174.086-T** (`fix-ci`)

```text
harness_exemption_class: docs-only
harness_exemption_reason: Replaces the bare full-suite command in fix-ci and PR automation instruction text only; no production or test code changes.
harness_owner: none
exempt_verification_command: pwsh -NoProfile -Command '$o = go test ./tests/integration -run ''^TestGovernedTestBudgetSurfaces$/^fix-ci$'' -count=1 -timeout=5m -v 2>&1 | Out-String; $rc = $LASTEXITCODE; Write-Output $o; if ($rc -ne 0) { exit 1 }; if (-not $o.Contains(''--- PASS: TestGovernedTestBudgetSurfaces/fix-ci'')) { exit 1 }; foreach ($f in @(''.github/skills/fix-ci/SKILL.md'',''.github/instructions/github-pr-automation.instructions.md'')) { go run ./cmd/backlogit docs lint --path $f --no-update-check; if ($LASTEXITCODE -ne 0) { exit 1 } }; Write-Output ''EXEMPT_VERIFY_OK:174.086-T''; exit 0'
exempt_precondition: must-fail-before-deliverable
```

**174.087-T** (`constitution`)

```text
harness_exemption_class: docs-only
harness_exemption_reason: Amends constitution quality-gate text and version with its sync impact report and the coupled AGENTS.md lines only; no production or test code changes.
harness_owner: none
exempt_verification_command: pwsh -NoProfile -Command '$o = go test ./tests/integration -run ''^TestGovernedTestBudgetSurfaces$/^constitution$'' -count=1 -timeout=5m -v 2>&1 | Out-String; $rc = $LASTEXITCODE; Write-Output $o; if ($rc -ne 0) { exit 1 }; if (-not $o.Contains(''--- PASS: TestGovernedTestBudgetSurfaces/constitution'')) { exit 1 }; foreach ($f in @(''.github/instructions/constitution.instructions.md'',''AGENTS.md'')) { go run ./cmd/backlogit docs lint --path $f --no-update-check; if ($LASTEXITCODE -ne 0) { exit 1 } }; Write-Output ''EXEMPT_VERIFY_OK:174.087-T''; exit 0'
exempt_precondition: must-fail-before-deliverable
```

**174.088-T** (`instructions`)

```text
harness_exemption_class: docs-only
harness_exemption_reason: Replaces the bare full-suite command in copilot and Go instruction text only; no production or test code changes.
harness_owner: none
exempt_verification_command: pwsh -NoProfile -Command '$o = go test ./tests/integration -run ''^TestGovernedTestBudgetSurfaces$/^instructions$'' -count=1 -timeout=5m -v 2>&1 | Out-String; $rc = $LASTEXITCODE; Write-Output $o; if ($rc -ne 0) { exit 1 }; if (-not $o.Contains(''--- PASS: TestGovernedTestBudgetSurfaces/instructions'')) { exit 1 }; foreach ($f in @(''.github/copilot-instructions.md'',''.github/instructions/go.instructions.md'')) { go run ./cmd/backlogit docs lint --path $f --no-update-check; if ($LASTEXITCODE -ne 0) { exit 1 } }; Write-Output ''EXEMPT_VERIFY_OK:174.088-T''; exit 0'
exempt_precondition: must-fail-before-deliverable
```

**174.089-T** (`engineer-readme`)

```text
harness_exemption_class: docs-only
harness_exemption_reason: Replaces the bare full-suite command in the go-engineer validation loop and README contributor text only; no production or test code changes.
harness_owner: none
exempt_verification_command: pwsh -NoProfile -Command '$o = go test ./tests/integration -run ''^TestGovernedTestBudgetSurfaces$/^engineer-readme$'' -count=1 -timeout=5m -v 2>&1 | Out-String; $rc = $LASTEXITCODE; Write-Output $o; if ($rc -ne 0) { exit 1 }; if (-not $o.Contains(''--- PASS: TestGovernedTestBudgetSurfaces/engineer-readme'')) { exit 1 }; foreach ($f in @(''.github/agents/subagents/go-engineer.agent.md'',''README.md'')) { go run ./cmd/backlogit docs lint --path $f --no-update-check; if ($LASTEXITCODE -ne 0) { exit 1 } }; Write-Output ''EXEMPT_VERIFY_OK:174.089-T''; exit 0'
exempt_precondition: must-fail-before-deliverable
```

**174.090-T** (`review-instructions`)

```text
harness_exemption_class: docs-only
harness_exemption_reason: Replaces the bare full-suite command in the two review-instruction files only; no production or test code changes.
harness_owner: none
exempt_verification_command: pwsh -NoProfile -Command '$o = go test ./tests/integration -run ''^TestGovernedTestBudgetSurfaces$/^review-instructions$'' -count=1 -timeout=5m -v 2>&1 | Out-String; $rc = $LASTEXITCODE; Write-Output $o; if ($rc -ne 0) { exit 1 }; if (-not $o.Contains(''--- PASS: TestGovernedTestBudgetSurfaces/review-instructions'')) { exit 1 }; foreach ($f in @(''.github/copilot-review-instructions.md'',''.github/instructions/copilot-code-review.instructions.md'')) { go run ./cmd/backlogit docs lint --path $f --no-update-check; if ($LASTEXITCODE -ne 0) { exit 1 } }; Write-Output ''EXEMPT_VERIFY_OK:174.090-T''; exit 0'
exempt_precondition: must-fail-before-deliverable
```

**174.091-T** (`harness-inputs`)

```text
harness_exemption_class: covered-by
harness_exemption_reason: Edits only the autoharness render-input values for the test command; the behavior is pinned by owner 174.080-T subtest harness-inputs, so a second harness would duplicate the owner contract.
harness_owner: 174.080-T
exempt_verification_command: pwsh -NoProfile -Command '$o = go test ./tests/integration -run ''^TestGovernedTestBudgetSurfaces$/^harness-inputs$'' -count=1 -timeout=5m -v 2>&1 | Out-String; $rc = $LASTEXITCODE; Write-Output $o; if ($rc -ne 0) { exit 1 }; if (-not $o.Contains(''--- PASS: TestGovernedTestBudgetSurfaces/harness-inputs'')) { exit 1 }; if (-not (Get-Content -Raw ''.autoharness/harness-manifest.yaml'').Contains(''go run ./cmd/test-budget run ./...'')) { exit 1 }; if (-not (Get-Content -Raw ''.autoharness/workspace-profile.yaml'').Contains(''go run ./cmd/test-budget run ./...'')) { exit 1 }; Write-Output ''EXEMPT_VERIFY_OK:174.091-T''; exit 0'
exempt_precondition: must-fail-before-deliverable
harness_owner_command: pwsh -NoProfile -Command '$o = go test ./tests/integration -run ''^TestGovernedTestBudgetSurfaces$/^harness-inputs$'' -count=1 -timeout=5m -v 2>&1 | Out-String; $rc = $LASTEXITCODE; Write-Output $o; if ($rc -ne 0 -or $o.Contains(''no tests to run'') -or -not $o.Contains(''--- PASS: TestGovernedTestBudgetSurfaces/harness-inputs'')) { exit 1 }; exit 0'
```

**Ship-time coupling note.** Between the W4 red landing and the W5 close, CI on any interim push
is red on `TestGovernedTestBudget*` by design. Ship closes W5 before pushing, or treats that
interim red as the declared red deliverable.

### 19.8 Closed exempt set, schedule, and 155-S membership snapshot

**155-S membership snapshot** (2026-09-24, `backlogit shipment get 155-S`): 36 items, status
`active`.

* `174-F`: active.
* 30 tasks `done`.
* 4 archived: `174.069-T` through `174.072-T`.
* The only non-terminal task is `174.073-T` (queued, no dependencies).

The waves are therefore derived over {`174.073-T`, `174.074-T` … `174.091-T`}:

| Wave | Tasks | Dependencies |
|---|---|---|
| W1 | `174.073-T`, `174.074-T`, `174.076-T`, `174.077-T` | none; recommended order is `174.073-T` first (already reviewed), then 074, 076, 077 |
| W2 | `174.075-T`, `174.078-T` | 075 ← 074; 078 ← 074, 077 |
| W3 | `174.079-T` | ← 075, 076, 078 |
| W4 | `174.080-T` | ← 079 (red deliverable) |
| W5 | `174.081-T` … `174.091-T` | each ← 080 (the closed exempt set; green makers; closes the red deliverable) |

* **No edge from `174.074-T` to `174.073-T`.** They touch disjoint files and domains.
  `174.073-T` only lowers `internal/core` cost and does not change the static count rule. Its
  one new test takes N from 885 to 886, which is the same 30m budget.
* **`174.073-T` is preserved unchanged.** Its AC7 rule governs the final gate below.
* **Session limit.** The 19 open tasks stay under the 20-tasks-per-session breaker. Ship
  checkpoints if a session ends before W5.
* `155-S` stays a supervised bootstrap shipment. These members are added by governed
  `backlogit shipment add` in dependency order.

### 19.9 Named deviations (constitutional conflicts, each with justification and rejected alternative)

* **D1 — Quality Gates "Do not skip any gate" (`go test ./...` first) and P-002.6 convergence
  item 1 / build-feature Step 0.5 repo-wide compile check, during W1-W5.**
  * Mapping: `go vet ./...`, which type-checks all test files without running them, plus the
    scoped `go test <pkg>` selectors named in each task.
  * Justification: the operator stop rule and `174.073-T` AC7 forbid any `go test ./...`
    (including `-run '^$' ./...`) over the **repository module** until a separately authorized
    run. `go vet ./...` is not a `go test` form. The hang probe's child `go test ./...` runs in a
    hermetic `t.TempDir()` module containing one test, so it is outside the stop rule.
  * Rejected alternative: an unfiltered compile or run, which is unauthorized.
* **D2 — P-002.6 unfiltered-suite convergence for W1-W5, and Principle II "`go test ./...`
  before merge".**
  * Ship records `FULL_SUITE_OPERATOR_DEFERRED` (citing the stop rule and `174.073-T` AC7) in
    place of each wave's unfiltered convergence run, **including W5**.
  * W5, and therefore 155-S, stays **unconverged** until the one authorized governed run
    (19.10) passes. The deviation defers the gate; it never waives it.
  * Rejected alternative: running canonicalFull per wave, which is an unauthorized full-suite
    execution.
* **D3 — Quality Gates `golangci-lint run` and `gofmt -l .` (repo-wide).**
  * Narrowed per task to `gofmt -l <package dirs>` and `golangci-lint run <package>/...`.
  * Trust basis: the installed golangci-lint v1.64.8 is the version CI pins.
  * Justification: repo-wide lint carries unrelated debt (`4DB1DFF1`).
  * Rejected alternative: fixing repo-wide lint here, which is out of scope.
* **D4 — P-004 non-zero red clause.**
  * Satisfied by the scoped red selectors (assertion failures with a compile PASS), not by a
    full-suite non-zero exit.
  * Rejected alternative: a full-suite red, which is unauthorized and would conflate
    unrelated failures.

### 19.10 Application to the imminent 155-S final gate and future growth

* **Readiness.** After W5 closes, all 155-S correctives are landed. Ship STOPS and requests a
  new explicit operator authorization for exactly one full-suite execution:
  `go run ./cmd/test-budget run ./...` from the repo root on the branch HEAD. The full output is
  captured under `logs/diagnostics/155-s-go-test-governed-<date>.txt` plus metadata.
* **The request cites the recorded operator disposition of 11BE840F:** `073-DL` O11, as
  `174.073-T` AC7 requires. AC7's recommended form, `go test -json ./...`, is superseded by
  canonicalFull (19.4.6). This plan records the supersession; `174.073-T` itself is not
  edited.
* **Expected result.** The header is `timeout=30m0s largest_package=internal/core
  top_level_tests=886 policy=073-DL`. The expected `internal/core` time is about 760-800s, about
  45% budget use.
* **Pass criteria:**
  * exit 0;
  * no `panic: test timed out`;
  * `TEST_BUDGET_SUMMARY` present;
  * `TEST_BUDGET_CEILING_EXCEEDED` absent.
  * Then the 155-S zero-P0/P1 stop-gate and the final standard + adversarial review apply.
* **A `revisit=R1`/`R2` WARN** does not fail the gate. Ship captures it as a Stage input (P-021
  capture-only) for re-calibration.
* **If it times out anyway:**
  * If the running test is ≥ 60s old: R5, a genuine hang or slow-test defect. Ship captures a
    P-021 expansion and does not raise the budget.
  * If it is < 60s old: the calibration is wrong. Stage re-deliberates the constants with the
    captured summary. No ad hoc flag is ever added.
* **Future growth.**
  * Each new test in the largest package raises the budget automatically at the 5m steps in
    19.4.2, reproducibly on every machine.
  * N_max ≥ 1200 triggers R3 early, 300 tests before the ceiling.
  * A ceiling breach turns the governed run red, which forces a Stage decision.
  * Constants change only through a new deliberation citing ≥ 2 governed-run summaries (R1-R4).
* **Authorization-request text** (for Ship's stop):
  "155-S Wave 19 closed. Per 174.073-T AC7, 11BE840F is dispositioned by 073-DL (O11). Request
  authorization for ONE governed full-suite run: `go run ./cmd/test-budget run ./...` (budget
  printed by the wrapper). `-json` is not used for the gate; a per-test `-json` diagnostic
  would be a separate request."

### 19.11 Out of scope for Wave 19

* Performance work on `internal/core`, partitioning, and fixture or template caching (O6).
* `D8EF5443`, the slog capture leak. The policy does not mechanically require it, and it stays
  active.
* `plugin/**` parity.
* Windows `-race` calibration.
* Re-rendering the harness. The contract test is the tripwire: a re-render that reintroduces the
  bare form turns `harness-inputs` and `executables` red.
* No drift-ignore edits.

### 19.12 P-021 follow-up captures (Stage-authored 2026-09-24: (a) `5F1A1873`, (b) `5A1C4D3F`, (c) `95DF7CE9`)

* **(a) core runtime reduction** (medium; trigger R2/R3): partition `internal/core`, or reduce
  fixture I/O and construction cost.
* **(b) Windows `-race` calibration** for `make.ps1 test` (low; R4).
* **(c) Upstream autoharness template and `plugin/**` parity** for the governed test command
  (low). Consumer workspaces lack `cmd/test-budget`, so this needs a template-level
  capability-conditional token.

### Plan Hardening Signals — Wave 19

* Cross-cutting change to governed CI, release, and pre-push entry points: **yes** (rollback is
  the literal revert of one line per surface).
* New executable in the repository: **yes** (`cmd/test-budget`; not in the release artifact).
* Policy and constitution text amendments: **yes** (MINOR bumps with amendment records).
* Irreversible data or schema changes: **no**.
* **Requires plan hardening: yes.**

### Constitution Check — Wave 19

| Principle / section | Mapping |
|---|---|
| I. Safety-First Go | The new Go code wraps errors with `%w`, has no `panic`, and guards overflow before multiplying. `go vet` and scoped golangci-lint run per task. The new binary is not in the release artifact |
| II. Test-First (NON-NEGOTIABLE) | Each Go surface has a declaration task with a source-shape harness (074/076/077), then a behavior task with an assertion-red harness (075/078/079). The surface migrations are gated by the red-deliverable contract test 080. The full-suite clause is deferred, not waived (D2) |
| III. Workspace Isolation | Every path the wrapper and counter use is relative to the module root (cwd). The tests use `t.TempDir()` |
| IV. CLI Containment (NON-NEGOTIABLE) | The wrapper requires `go.mod` in cwd and never walks above it |
| V. Structured Observability | The header, summary, WARN, CEILING, and ERROR markers are stable machine-readable lines that name `073-DL` and the revisit criterion |
| VI. Single Responsibility | `internal/testbudget` holds the policy and counter; `cmd/test-budget` holds the process wrapper. No new dependency |
| VII. Destructive Approval (NON-NEGOTIABLE) | No destructive operation. The wrapper never kills a process |
| VIII. Safety Modes | Not triggered (no elevated-risk operation) |
| IX. Git-Friendly Persistence | Only Markdown/YAML and Go source text changes. Evidence goes under `logs/diagnostics/` |
| X. Context Efficiency | One source of truth plus one contract test replaces about 20 scattered literals |
| XI. Merge Commit History | Unaffected (Ship's merge discipline is unchanged) |
| Quality Gates | Mapped by D1 (go test), D3 (lint, gofmt). `go vet` runs as specified. Conflicts are documented in 19.9 with justification and rejected alternative |
| Task Granularity (NON-NEGOTIABLE) | 18 tasks. Each is ≤ 2 files, ≤ 4 functions, ≤ 3 test scenarios, and a single domain (Go code+tests, a single test file, docs, or config). 077/078 hold 3 functions each, 079 holds 3 |
| Stop Conditions | The 19 open tasks are under the 20-task session breaker. Plan-review attempts follow the Stage cycle limit |
| Versioning / Governance | workflow-policies 1.30.0 (MINOR: new governed rule). Constitution 1.1.0 (MINOR: a material quality-gate change) with rationale and sync impact report (174.087-T). AGENTS.md is coupled by the contract test |
| P-010 | Stage authored no code |

Constitution Check: documented-deviations

## Plan Hardening — Wave 19

| # | Risk | Mitigation (bound to a unit and a check) |
|---|---|---|
| H1 | The counter undercounts, the budget stays too small, and the timeout recurs | `TestLargest/repository` requires ≥ 500 for `internal/core` (174.075-T). The build-tag-agnostic count is conservative. `print` evidence shows 885/886 (174.079-T AC3) |
| H2 | The wrapper hides the hang detector | The budget is always finite (≤ 45m). `TestGoTestTimeout_BoundedHangIsDetected` proves a real alarm fires through `execGo` (174.079-T). The wrapper never kills or signals the child, and `cmd/go`'s +1m backstop remains |
| H3 | A caller supplies its own timeout and bypasses the policy | `run` rejects all four spellings (174.079-T table rows). The coupling test fails on a timeout flag in any governed span (174.080-T) |
| H4 | Surfaces drift back to bare forms (manual edit or harness re-render) | The persistent coupling contract, including the `harness-inputs` subtest and the executable sweep (174.080-T) |
| H5 | Unexplained constant creep | `PolicyID` is printed in every header. The constants live only in `internal/testbudget`. R1-R5 plus the ≥ 2-summary rule |
| H6 | Output parsing breaks on Windows CRLF, partial writes, over-long lines, or concurrent writes | A single shared writer means a single copy goroutine. `Write` always returns `len(p), nil`. Covered by the `TestResultSink_Write` rows (174.078-T) and the same-writer assertion (174.079-T) |
| H7 | `go run` exit-code collapse hides the class | Gates key on markers and zero/non-zero status only (19.4.4) |
| H8 | Interim CI red between W4 and W5 | Declared red deliverable. Ship closes W5 before pushing (19.7 note) |
| H9 | Machine-specific flakiness | The budget depends only on committed source. There is no platform branch in the formula. R4 WARN is informational |
| H10 | Ceiling breach silently grows the budget | It clamps at 45m, prints `TEST_BUDGET_CEILING_EXCEEDED`, and returns non-zero (exit 3). Pinned by the 1501-test rows (174.079-T) |
| H11 | Lint `unused` fails the declaration tasks | The 076/077 stubs and anchors reference every unexported declaration (076/077 AC4). 078 removes its anchors only when its own tests call the symbols. 074 declares only exported symbols |
| H12 | A descendant holds the output open, and the wrapper hangs or passes | `WaitDelay` 10s, then `ErrWaitDelay` maps to exit 1, fail closed |

<!-- plan-review-attempt: rev21-attempt-1-FAIL -->

## Plan Review — Wave 19 attempt 1

```text
dispatch_mode: multi-agent-dispatch
reviewers: Go Reviewer, Scope Boundary Auditor, Constitution Reviewer, Architecture Strategist, Schema-CLI-Docs Coupling Reviewer
decision: FAIL
```

**Verdicts on rev21:**

| Reviewer | Verdict | Blocking findings |
|---|---|---|
| Go | FAIL | 2 P1 |
| Scope | FAIL | 2 P1 |
| Constitution | FAIL | 2 P0, 1 P1 |
| Architecture | ADVISORY | 1 P1 |
| Coupling | ADVISORY | P2/P3 only |

**Findings and their rev21.1 dispositions:**

* **Constitution P0-1 (Principle II).** New Go packages were created without a declaration +
  source-shape harness split. *Fixed:* 074/076 are declarations and 075/077 are behavior tasks.
* **Constitution P0-2.** The contract test landed last, so the surface migrations had no red
  gate. *Fixed:* 174.078-T is a W4 red deliverable, with the surfaces as green makers.
* **Constitution P1.** The constitution bump was PATCH. *Fixed:* MINOR 1.1.0, with an AGENTS.md
  coupling check.
* **Go P1-1.** Scanner/pipe goroutine risks (deadlock, lost partial lines, CRLF). *Fixed:*
  `io.MultiWriter` plus `resultSink` inline parsing.
* **Go P1-2.** The hang probe inherited the host `GOFLAGS`/`GOWORK`/toolchain, and there was no
  `WaitDelay`. *Fixed:* 19.7 U19d.
* **Go P2s** (all fixed):
  * counter rules for `helper.go`, TestMain, and param type;
  * overflow guard;
  * negative exit codes;
  * signal forwarding;
  * raw-elapsed precision.
* **Scope P1-1.** Drift-ignore edits widened scope. *Fixed:* dropped; P-021 (c) captured.
* **Scope P1-2.** Missing surfaces: the review instructions and 3 manifest variables. *Fixed:*
  added 174.088-T and extended 174.089-T.
* **Architecture P1.** Ceiling fail-closed before running destroyed the R3 evidence. *Fixed:*
  clamp, run, non-zero exit.
* **Coupling P2/P3** (all fixed):
  * the growth-table row error (334-555 → corrected 334-500/501-666);
  * the "targeted selectors keep explicit small timeouts" wording;
  * the `-json` supersession note;
  * header field order pinned.

<!-- plan-review-attempt: rev21.1-attempt-2-FAIL -->

## Plan Review — Wave 19 attempt 2

```text
dispatch_mode: multi-agent-dispatch
reviewers: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor
decision: FAIL
```

**Verdicts on rev21.1** (all attempt-1 findings confirmed fixed):

| Reviewer | Verdict | Blocking findings |
|---|---|---|
| Constitution | FAIL | 1 P1 |
| Go | FAIL | 2 P1 |
| Scope | ADVISORY | P2/P3 only |

**Cycle accounting.** Per the Stage cycle limit (maximum 2 re-entry cycles) and plan precedent
rev15 ("PASS after 2 FAIL"), attempt 3 is the last allowed re-entry. A FAIL in attempt 3 halts
to the operator.

**Findings and their rev21.2 dispositions:**

* **Constitution P1.** The 174.077-T harness left the ceiling, R2/R3/R4, negative-exit, `n/a`,
  and 64 KiB behaviors with no failing test first. *Fixed:* the wrapper is split into 076
  (run declaration), 077 (sink declaration), 078 (sink behavior), and 079 (run behavior), and
  every 19.4.4 behavior has a named table row.
* **Constitution P2** (all fixed):
  * the exemption contracts are now rendered in full (11 blocks; `covered-by`
    `harness_owner_command` wrapped with the PASS and no-tests checks);
  * D2 is extended through W5, which stays unconverged until the authorized run;
  * 174.087-T AC now requires the rationale and sync impact report;
  * the Constitution Check maps I-XI and documents each deviation as a constitutional conflict.
* **Constitution P3.** The shape-harness imports are listed exactly, with a justification.
  *Fixed.*
* **Go P1-1.** Two `MultiWriter`s sharing one sink caused a concurrent-write race. *Fixed:* one
  writer is passed as both `Stdout` and `Stderr`, so there is one copy goroutine, and `Write`
  always returns `len(p), nil`.
* **Go P1-2.** golangci-lint `unused` would fail on the declaration stubs. *Fixed:* the stubs
  and anchors reference every unexported declaration; lint is part of the AC.
* **Go P2/P3** (all fixed):
  * the signal lifecycle (`Notify` + `defer Stop`, no goroutine, no forwarding);
  * `ErrWaitDelay` maps to 1 and a start failure to 2;
  * the largest package is matched by exact module path;
  * the output wording (raw bytes to the terminal);
  * `--timeout` and `--test.timeout` rejection;
  * the two-names-per-field counter row;
  * the fixture `go.mod` pins `go 1.24`;
  * the cached-run `n/a` fallback.
* **Scope P2** (all fixed):
  * the detector parses `-run=V`, `-run V`, and `-test.run`;
  * the span rule is quote-aware only when a quote opens the span;
  * rows added for `ci.yml:165` and the `-run=^$` compile-only lines;
  * the `variables_used:` block is named and asserted non-empty.
* **Scope P3** (all fixed):
  * `go.instructions.md:36` becomes a governed coverage span;
  * signal forwarding is dropped (YAGNI);
  * `-json` mode and exit code 3 are kept (the ceiling test and the diagnostic pass-through
    need them).

<!-- plan-review-attempt: rev21.2-attempt-3-FAIL -->

## Plan Review — Wave 19 attempt 3

```text
dispatch_mode: multi-agent-dispatch
reviewers: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor
decision: FAIL
```

**Verdicts on rev21.2** (all attempt-2 findings confirmed fixed):

| Reviewer | Verdict | Blocking findings |
|---|---|---|
| Constitution | FAIL | 1 P1 |
| Go | ADVISORY | P2/P3 only |
| Scope | ADVISORY | P3 only |

**Circuit breaker: OPEN.** This is the third consecutive FAIL. The attempt-2 record declared
attempt 3 the last allowed re-entry, so Stage halts here. Stage did **not** re-invoke
plan-review and did **not** harvest. rev21.3 carries every disposition below, but it is
**unreviewed**. `decision: FAIL` stays the authoritative final record until an
operator-authorized review appends a later `## Plan Review` section.

**Findings and their rev21.3 dispositions (applied, not yet re-reviewed):**

* **Constitution P1.** 174.079-T left the `execGo` exit mapping (negative code, `ErrWaitDelay`,
  start failure) and the unknown-subcommand path with no failing test first. *Remediated:*
  * the pure helper `mapWaitResult(code, err)` is declared in 174.077-T and gets a 5-row
    `TestMapWaitResult` in 174.078-T;
  * 174.079-T adds rows for an unknown subcommand, an empty argument list, and the real
    `execGo` start failure (with `PATH` set to an empty temp dir).
* **Constitution P2.** 174.079-T's anchor removal made it a 3-file task. *Remediated:* 174.078-T
  removes the anchors, since its own tests use the symbols. 174.079-T touches only `main.go`
  and `main_test.go`. The sink files are renamed `result.go`, `result_test.go`, and
  `result_shape_test.go`.
* **Constitution P3** (all remediated):
  * contract blocks are wrapped in BEGIN/END markers in the task bodies;
  * the surfaces test is table-driven;
  * D1 scopes the stop rule to the repository module (the hang probe is hermetic);
  * the `Constitution Check: documented-deviations` verdict line is added.
* **Go P2** (all remediated):
  * the errcheck-safe 076 `run` stub;
  * `type resultSink struct{}` with no fields until 174.078-T;
  * `Largest` skip rules apply only when `path != root`, with a `dot-root` row under `t.Chdir`.
* **Go P3** (all remediated):
  * the hang probe assigns `buildGoTestArgs`'s two results;
  * the detector requires a line start or a non-word character before `go`, and joins ` \`
    continuations;
  * timeout flags match an exact token or a `flag=` prefix;
  * the detector table gains 3 rows.
* **Scope P3** (all remediated):
  * go-engineer line 76 becomes canonicalFull 1:1;
  * surface tasks may edit the version and amendment lines named in their row;
  * 174.090-T and 174.091-T share the pinned `summaryLine` (manifest line 596 renders into
    `copilot-code-review.instructions.md` line 53);
  * the heading is retitled.

**Escalation (P-013.6).**

```text
threshold_kind: plan-review consecutive FAIL
count: 3 (rev21 attempt 1, rev21.1 attempt 2, rev21.2 attempt 3)
failure_summary: each attempt converged (P0/P1 counts 5 -> 3 -> 1); the final blocker was a single missing-test P1 with a mechanical fix
last_actions: attempt-1/2/3 records above; rev21.3 remediation applied
artifact_refs: docs/exec-plans/2026-09-14-resumable-shipment-blocked-lifecycle-plan.md (Wave 19 rev21.3), .backlogit/queue/073-DL.md, stash 11BE840F
telemetry_evidence: logs/diagnostics/155-s-go-test-post-wave17-20260924.txt (+ metadata)
resumption_checkpoint: docs/memory/2026-09-24/stage-11be840f-governed-test-budget.md
resolved_escalation_route: gpt-6-sol / openai / xhigh (config.model_routing.escalation, reloaded 2026-09-24; differs from the Stage route claude-opus-5.5 / anthropic / high)
escalation_status: ESCALATION_DEGRADED (the Engram CLI exposes no escalation-handoff command; MCP is not used) -> operator halt
```

**Operator decision required.** Choose one:

* **(a)** Authorize one further plan-review cycle on rev21.3. On PASS, Stage harvests
  174.074-T … 174.091-T into 155-S.
* **(b)** Accept rev21.3 as ADVISORY-equivalent without re-review. Stage appends
  `operator_authorization: approved` in a new final `## Plan Review` section and harvests.
* **(c)** Reject Wave 19 and re-deliberate 073-DL.

<!-- plan-review-attempt: rev21.3-attempt-4-FAIL (operator-authorized single additional pass) -->

## Plan Review — Wave 19 attempt 4 (operator-authorized single additional pass, rev21.3) (2026-09-24)

```text
dispatch_mode: multi-agent-dispatch
reviewers: Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Learnings Researcher, Architecture Strategist, Schema-CLI-Docs Coupling Reviewer, Correctness Reviewer
decision: FAIL
plan_hardening_required: yes
plan_hardening_satisfied: yes (## Plan Hardening — Wave 19, H1-H12)
constitution_check_verdict: documented-deviations (recognized)
```

**Authorization.** Operator 2026-09-24: "Authorize one additional Stage plan-review pass for
Wave 19 rev21.3. If it passes, harvest the reviewed tasks into 155-S; do not implement them or
run the full suite." This record is that single pass. It does **not** authorize any further
remediation or re-review cycle. `TOOL_OK: reviewer-subagent-dispatch` (all 7 personas
dispatched and returned). Security Lens Reviewer: not triggered (no auth/authz, secrets, or
trust-boundary surface). Agent-Native Parity Reviewer: not triggered (no MCP tool or
agent-action surface; the Ship/skill edits are command-text swaps pinned by 174.080-T).

**Verdicts on rev21.3:**

| Reviewer | Verdict | Blocking findings |
|---|---|---|
| Constitution | ADVISORY | none (3 P2, 7 P3) |
| Go | FAIL | 1 P1 (B1) |
| Scope | ADVISORY | none (1 P2, 3 P3) |
| Learnings Researcher | FAIL | 1 P0 (B1, contradicts `docs/compound/best-practices/source-shape-harnesses-must-allow-lifecycle-successors-2026-09-11.md`) |
| Architecture | ADVISORY | none (2 P2, 4 P3) |
| Schema-CLI-Docs Coupling | ADVISORY | none (2 P2, 7 P3) |
| Correctness | ADVISORY | none (3 P2, 7 P3) |

**Attempt-3 findings: all confirmed fixed** (Constitution P1 exit-mapping/unknown-subcommand
rows present at 3082-3088, 3116-3118, 3180, 3199-3200; 174.079-T is 2 files; BEGIN/END
markers; table-driven surfaces test; D1 scoping; verdict line; every Go P2/P3 and Scope P3
item). Independently confirmed correct: `mapWaitResult` 5-row table against GOROOT `os/exec`
(`ExitError` and `ErrWaitDelay` never co-occur), the single-writer claim, the counter rules
against `cmd/go` `isTestFunc`, the growth-table arithmetic and the 1500 overflow guard, the
bounded hang-test timing, the ceiling clamp/run/exit-3 precedence, release isolation of
`cmd/test-budget`, the 19.2 line inventory, P-021 scope (`5F1A1873`, `5A1C4D3F`, `95DF7CE9`,
`D8EF5443` stay separate), and the dependency order 074, 076, 077 → 075, 078 → 079 → 080 →
081..091.

**Blocking finding (merged; final severity P0, the more conservative of P0/P1):**

* **B1 — 174.077-T/174.078-T lifecycle-frozen source-shape harness (Go P1 + Learnings P0).**
  `TestResultSinkSourceShape` (174.077-T, 3051-3058) pins transition-only scaffold state as a
  permanent test: `type resultSink struct{}` with no fields, plus the removable anchors
  `var _ = (*resultSink).footer` and `var _ = mapWaitResult`. 174.078-T is required to add the
  `resultSink` fields and delete those two anchors (3076-3078), may touch only `result.go` and
  `result_test.go` (3074), and its AC2 (3122-3123) — and 174.079-T AC2 — require
  `TestResultSinkSourceShape` to PASS. H11 repeats the conflict. As written, 174.078-T either
  cannot go green or must edit `result_shape_test.go` as a third file (2-hour/2-file rule
  breach). This contradicts the resolved compound lesson that permanent source-shape harnesses
  assert only permanent shape. **Recommended remediation (NOT applied; no remediation cycle is
  authorized):** the 174.077-T harness asserts only that `resultSink` is a struct type (fields
  unchecked), the three signatures, and the permanent `var _ io.Writer = (*resultSink)(nil)`;
  the two removable anchors remain required by 174.077-T AC4 (lint) but are not asserted by the
  harness; H11 is reworded to match.

**Non-blocking P2 findings (recorded for the remediation revision; not dispositioned here):**

* **P2-1 (Constitution + Go + Correctness).** Exit-2 rows in `TestRun_DerivationAndPrint`
  (four timeout spellings 3184, no `go.mod` 3185, zero tests 3186, unknown/empty 3180), the
  real start-failure row (3199-3200), and the 174.075-T error rows pass against the stubs, so
  those behaviors never fail first. Pin a per-path `TEST_BUDGET_ERROR: <class>` stderr marker,
  a runner-not-called check, both `=` and separate-value timeout forms inside a valid fixture,
  and `errors.Is(err, exec.ErrNotFound)` plus the `start go:` prefix.
* **P2-2 (Constitution + Correctness).** `TestResultSink_Write` (3097) must observe parsing only
  through `Write` + `footer(...)`; otherwise the RED run is a compile failure against the
  field-less struct, contradicting 174.078-T AC1.
* **P2-3 (Constitution).** Constitution Check Principle VIII row (3639) should declare
  Freeze-scope for W5 and Careful mode for 081/082/083/085/087.
* **P2-4 (Go; Correctness P3).** Hang probe: add `t.Setenv("GOENV","off")`; an empty `GOFLAGS`
  does not neutralize a `go env -w GOFLAGS=...` setting.
* **P2-5 (Scope).** `.github/copilot-instructions.md:86` (`go test ./internal/...`) is neither
  migrated, exempted, nor matched by the 174.080-T full-suite rule.
* **P2-6 (Coupling).** `.github/copilot-instructions.md:13` table row: the unquoted `go test`
  Technology cell plus the `./...` Notes token is flagged bare by the detector even after the
  planned 174.088-T edit, and `workspace-profile.yaml:26` `runner: "go test"` re-renders it.
* **P2-7 (Coupling).** `.github/skills/build-feature/SKILL.md:266` 45m stall kill equals the
  Ceiling, pre-empting the Go alarm/R5 evidence and the footer markers near the ceiling.
* **P2-8 (Architecture).** R2 (1.25) and R3 (1200) thresholds live in `cmd/test-budget`, outside
  the `internal/testbudget` single source of truth claimed by 19.4.1 and H5.
* **P2-9 (Architecture; Scope P3).** The ship-time note (3498-3500) permits pushing into the
  declared interim red, contradicting H8; after 174.082-T the pre-push Test gate and PR CI both
  run the full suite, which conflicts with D1/D2 and the single authorized run in 19.10.
* **P2-10 (Learnings).** Hand-edited autoharness-managed rendered surfaces
  (constitution, go, github-pr-automation, copilot-code-review instructions, go-engineer) have
  no stated checksum/drift disposition against `autoharness gate check` at task completion.
* **P2-11 (Learnings).** The plugin-parity coverage boundary is unstated (no confirmation that
  existing `.github`↔`plugin` parity tests exclude the edited files).

**P3 (advisory):** `run` `(0, err)` should map to 1; `buildGoTestArgs` error condition unpinned;
`print` exit-code wording (2819 vs 3144); `\r` stripping and 64 KiB prefix-parse wording; pin
`run(context.Background(), os.Args[1:], ...)` in the 076 shape harness; stdlib `module`-directive
parse (no `golang.org/x/mod` promotion); allow one unexported `run` helper; gofmt AC on
075/078/079; merged-stream note; package doc in `budget.go`; testify import allowance or a D5
deviation; soft edges 077←076 and 087←085; explicit AC per W5 surface task; mechanical check of
the 174.087-T rationale/sync-impact headings; unused `bare-mention`/`drift_reason` allowlists;
footer exact-line row; shape harness pins constant values; package-wide no-`t.Parallel`;
rationale for rejecting smaller caller timeouts; `Largest` tie-break and base-name prefix rule;
missing growth-table boundary pairs; TESTING_RULES consumer, fix-ci:72 canonicalCI, `-json` mixed
output; constitution amendment-log heading anchor; surface units may fix red-capture-named lines;
wrapper static existence check in the coupling test; release-build-target guard; 174.079-T and
174.080-T are circuit-breaker-sensitive in size.

**Gate outcome.** FAIL (one P0). Stage did **not** harvest, did **not** apply any remediation,
and did **not** re-invoke plan-review. `11BE840F` stays ACTIVE. No task `174.074-T` …
`174.091-T` exists; shipment `155-S` membership is unchanged.

**Escalation (P-013.6).**

```text
threshold_kind: plan-review consecutive FAIL (operator-extended cycle)
count: 4 (rev21 attempt 1, rev21.1 attempt 2, rev21.2 attempt 3, rev21.3 attempt 4)
failure_summary: blocking count converged 5 -> 3 -> 1 -> 1; attempt-4 blocker B1 is a single-sentence harness-scope fix (lifecycle-frozen shape assertion), plus 11 non-blocking P2s
last_actions: attempt-4 multi-agent dispatch (7 personas) on rev21.3; no remediation applied
artifact_refs: docs/exec-plans/2026-09-14-resumable-shipment-blocked-lifecycle-plan.md (Wave 19 rev21.3), .backlogit/queue/073-DL.md, stash 11BE840F
telemetry_evidence: logs/diagnostics/155-s-go-test-post-wave17-20260924.txt (+ metadata)
resumption_checkpoint: docs/memory/2026-09-24/stage-11be840f-attempt4-review-fail.md
resolved_escalation_route: gpt-6-sol / openai / xhigh (config.model_routing.escalation, freshly reloaded 2026-09-24; differs from the Stage route claude-opus-5.5 / anthropic / high)
escalation_status: ESCALATION_DEGRADED (no Engram escalation-handoff operation is exposed) -> operator halt
```

**Operator decision required.** Choose one:

* **(a)** Authorize a remediation revision (rev21.4) applying B1 (and optionally the P2s), plus
  one further plan-review pass; on PASS, Stage harvests 174.074-T … 174.091-T into 155-S.
* **(b)** Accept rev21.3 with B1 as explicitly accepted residual risk under
  `operator_authorization: approved` (not recommended: 174.078-T is not satisfiable as written).
* **(c)** Reject Wave 19 and re-deliberate 073-DL.
