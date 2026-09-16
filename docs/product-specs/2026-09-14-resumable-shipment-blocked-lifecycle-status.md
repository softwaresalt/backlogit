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

## 0. CURRENT AUTHORITATIVE REVISION — rev3 (2026-09-15)

> §0 is the **authoritative current revision** and supersedes rev1/rev2 §0 and the detailed
> sections below, which are **retained as superseded audit trail** (including all prior
> `## Plan Review` records in the companion plan). `SBLK-R1…R27` remain the shared contract
> vocabulary. **rev3 removes the rollout circularity, the generic-move bootstrap, and the
> topology `--force` for `155-S` entirely** by using a branch-scoped source-of-truth.

### 0.1 Scope split — shared contract vs. backlogit-local implementation

**(a) Shared, portable contract (STABLE — must not diverge from autoharness).** Shipment
lifecycle status **`blocked`**: non-terminal; preserves shipment/member/reconciliation/
resumption evidence; **excluded from the single *active* execution slot**; cannot be
claimed/executed while blocked; governed transitions `active → blocked` and
`blocked → {queued, active}`. At most one `active` shipment; multiple `blocked` may coexist.
Governed metadata `blocked_reason`, `blocked_at`, `blocked_by`, `resume_checkpoint_ref`. A
**durable INTENT + complete PREIMAGE snapshot persist before any member/shipment mutation**; on
block, active/in-flight members are captured to a **machine-readable snapshot** and returned to
`queued`; **never leave active members under a queued shipment**. Target-aware unblock: to-queued
leaves members queued and preserves the snapshot; to-active restores the exact snapshot under the
workspace-global lock. **Only ClaimShipment and unblock-to-active may create an active shipment**,
both under that global lock. Every transition uses a **correlated intent→commit** record.

**(b) backlogit-local implementation (REPLACEABLE).** The central governed writer/envelope, the
governed public `WriteArtifactFile` boundary + private lower writer, `.locks/` workspace-global
lock, SQLite projection, doctor checks, and CLI/MCP surface shapes are local and may be
re-implemented upstream; they MUST NOT leak backlogit-specific semantics into (a).

### 0.2 Concise decomposition (feature 174-F / shipment 155-S) — 16 tasks (17 shipment members incl. 174-F), RED-before-GREEN

The rev2 9-task set (`174.030-T…174.038-T`) is **superseded**; together with the rev1 remnants
`174.001-T…174.029-T` the full `174.001-T…174.038-T` band is now **archived** (terminal,
`archived_status: blocked`, history/events preserved). Under the flat-manifest scope rule (§0.5,
SBLK-R28) those superseded descendants are OUTSIDE `155-S` scope **because their IDs were never
explicit members of the `155-S` manifest**, not because they were archived — archival preserved
history but was **not required** to descope them from `155-S`. Replacement is **16 live tasks** — `174.039-T…174.051-T` (13) plus the
core stateful-seam split tasks `174.052-T` (declaration-only compile-green) and `174.053-T`
(core-lifecycle behavior RED) and the R4g pre-edge guard `174.054-T` (generic `MoveShipmentStatus`
block/unblock refusal, landed before the R5/R6 transition-table edges) — all ≤2h, RED harnesses
precede their implementations. Shipment `155-S` therefore carries **17 members** (the 16 tasks plus
covering feature `174-F`).

**Stateful-seam RED discipline (rev3 remediation).** The core `BlockShipment`/`UnblockShipment`
seam follows the mandated chain **source-shape RED → declaration-only compile-green → behavior RED →
implementation** (P-002.1 source-shape harness; no declaration-only *exemption*). R1 is split so the
harness no longer jumps from a source-shape RED directly into declaring/implementing the seam:
`174.039-T` (R1s) is a source-shape harness pinning the `ShipmentBlocked` enum + the EXACT
`BlockShipment(ctx, ws, shipmentID string, opts BlockOptions) (*models.Artifact, error)` /
`UnblockShipment(ctx, ws, shipmentID string, opts UnblockOptions) (*models.Artifact, error)`
signatures, the `BlockOptions`/`UnblockOptions` structs, and BOTH the `blerrors.ErrNotImplemented`
and `blerrors.ErrShipmentBlockedRequiresEnvelope` sentinels (declaration shape only; NOT the
`isValidShipmentTransition` blocked-transition table,
which is functional transition-enablement deferred to R5/R6); `174.052-T` (Rd) lands ONLY those
declarations (stubs return `ErrNotImplemented`; BOTH sentinels declared declaration-only —
`ErrShipmentBlockedRequiresEnvelope` is WIRED, not introduced, by R4g `174.054-T`), turning R1s green; `174.053-T` (R1b) is the behavior
RED harness that compiles against the stubs and stays red until R5/R6 implement. The writer/crash
harnesses R2 (`174.040-T`) and R3 (`174.041-T`) depend on Rd (`174.052-T`) so they compile-green
before asserting behavior. These exact Go signatures/structs/sentinel are the **[local] backlogit
API shape** (§2.5); the **[shared] portable contract** (token/edge-set/field-names/event) is
asserted separately (U17) and does not overlap the R1s go/ast shape assertions.

**Guard-before-edge ordering (PR #444 remediation).** The generic exported `MoveShipmentStatus`
block/unblock refusal is landed by `174.054-T` (R4g) at **wave 4**, STRICTLY BEFORE R5 (`174.043-T`)
enables the `active→blocked` edge and R6 (`174.044-T`) enables `blocked→{queued,active}` in
`isValidShipmentTransition`. R4g is a top-level fail-closed guard placed ABOVE the transition-table
check (returning the Rd-declared `blerrors.ErrShipmentBlockedRequiresEnvelope`), so no intermediate wave exposes an
ungoverned generic block/unblock; the governed seam is exempt by construction (it writes beneath the
choke point). R5/R6 therefore depend on R4g. R7a (`174.045-T`) retains the remaining bypass routing.

| Task | Req | Scope | Domain |
|---|---|---|---|
| `174.039-T` | R1s | Core-lifecycle SOURCE-SHAPE RED harness (go/ast: `ShipmentBlocked` enum + EXACT `BlockShipment`/`UnblockShipment` signatures + `BlockOptions`/`UnblockOptions` + `blerrors.ErrNotImplemented` + `blerrors.ErrShipmentBlockedRequiresEnvelope` sentinels; no transition-table) | tests |
| `174.052-T` | Rd | Core-lifecycle declaration-only stubs (compile-green): `ShipmentBlocked` const + `BlockOptions`/`UnblockOptions` + block/unblock stub signatures returning `blerrors.ErrNotImplemented`; declares BOTH the `ErrNotImplemented` and `ErrShipmentBlockedRequiresEnvelope` sentinels (no `isValidShipmentTransition` blocked entries) | code |
| `174.053-T` | R1b | Core-lifecycle BEHAVIOR RED harness (transitions, metadata, intent+preimage, disposition, target-aware unblock) | tests |
| `174.040-T` | R2 | Writer/bypass BEHAVIOR RED harness (writer boundary, generic move/update, MoveShipmentStatus, bulk/cascade, create-as-active) | tests |
| `174.041-T` | R3 | Crash/reopen BEHAVIOR RED harness (durable intent/preimage recovery; subprocess kill+reopen) | tests |
| `174.042-T` | R4 | Governed writer core + envelope + public `WriteArtifactFile` boundary | code |
| `174.054-T` | R4g | Pre-edge generic `MoveShipmentStatus` block/unblock top-level refusal (`blerrors.ErrShipmentBlockedRequiresEnvelope`, above the transition-table check; lands @W4 before any R5/R6 edge) | code |
| `174.043-T` | R5 | Core `BlockShipment` (global lock held across intent→preimage→disposition→persist→commit/compensation; member-mutation guard while blocked; enable governed `active→blocked` transition, generic move still refused via R4g) | code |
| `174.044-T` | R6 | `UnblockShipment` + `Claim` under shared global lock (target-aware restore, CAS/drift refusal, create-active restriction; enable governed `blocked→{queued,active}` transitions, generic move still refused via R4g) | code |
| `174.045-T` | R7a | Route REMAINING bypass WRITE call-sites (generic move/update, bulk/cascade; MoveShipmentStatus writer delegation) through the governed writer — MoveShipmentStatus block/unblock refusal now owned by R4g | code |
| `174.051-T` | R7b | Guard create-as-active + all generic activation paths (refuse activation outside claim/unblock) | code |
| `174.046-T` | R8 | CLI + MCP parity for block/unblock + read/list status (surface inputs map 1:1 to `BlockOptions`/`UnblockOptions` over the same core) | code |
| `174.047-T` | R9 | Recovery + normalizer (durable-intent recovery under locks; machine-readable snapshot schema as input; MCP normalizer parity) | code |
| `174.048-T` | R10 | Subprocess crash/reopen GREEN tests | tests |
| `174.049-T` | R11 | Doctor production checks (active-count, malformed-blocked, torn-intent; severity/exit; MCP; isolated fixture) | code |
| `174.050-T` | R12 | Operator docs + branch-scoped bootstrap runbook + topology note | docs |

Topological order (parent-first): `174-F → 174.039 → 174.052 → 174.053 → 174.040 → 174.041 →
174.042 → 174.054 → 174.043 → 174.044 → 174.045 → 174.051 → 174.046 → 174.047 → 174.048 → 174.049 →
174.050` (16 tasks + `174-F` = 17 `155-S` members). Waves (dependency layers): W1 `039`; W2 `052`;
W3 `053`, `040`, `041`; W4 `042`, `054`; W5 `043`; W6 `044`; W7 `045`, `051`, `046`, `047`; W8
`048`, `049`; W9 `050` (9 waves). RED-deliverable closing waves: R1s@2, R1b@6, R2@7, R3@8. `164.002-T` (S12 forward-repair) is **retired/superseded by rev3** — set
`blocked` (history preserved) with **no dependency on `174.050-T`** (the obsolete cross-shipment
edge is removed), because its shipment-record-only `queued → active` contract is incompatible with
rev3 exclusive activation and its reconciliation role is subsumed by R9 (`174.047-T`) / R11
(`174.049-T`); `146-S` therefore no longer couples to `155-S`.

### 0.3 Authoritative rollout sequence (removes circularity — NO pre-block, NO 155 topology force)

1. `main`/staging currently hold `154-S` and `173.006-T` as `queued`; therefore after the staging
   PR merges, **`155-S` is claimed and shipped normally on `main`** — no pre-block of `154`, no
   topology `--force` for `155`. **During `155-S` execution no `blocked` token exists**, so the
   external topology gate is never exercised on `blocked`.
2. After `155-S` ships and the governed `BlockShipment` exists on `main`, create a dedicated
   **backlog-only `chore/block-154` branch** from synchronized `main`. Import ONLY authoritative
   backlog/checkpoint provenance for `154-S`/`173-F` members from the preserved Ship branch
   `feat/shipment-claim-scheduler-baseline-marker-enabling-precondition@dd9f01a1` **with content
   hashes and an explicit allowlist (no Go/source/harness code)**. This hydration reconstructs the
   shipment-active + member-active state on the bootstrap branch.
3. Invoke the newly shipped **governed `BlockShipment`** there (a legal `active → blocked`), which
   writes the machine-readable snapshot, queues active members, and records metadata/events; then
   commit the complete governed output as one atomic backlog change — the `154-S` shipment record,
   every changed member artifact (`173.006-T` requeued), the durable intent + preimage +
   snapshot/recovery state, and the authoritative per-item event logs for the shipment and each
   dispositioned member — and PR-merge it to `main` (committing only `154-S` + snapshot is
   insufficient). **No generic move, no
   pre-governance rollback.**
4. Run the corrective `7AA35A39` shipment on `main` while `154` is `blocked`. Later merge current
   `main` into the preserved `154` feature branch, resolve backlog state to the blocked provenance,
   invoke governed **unblock-to-active** after the fix, and resume from checkpoint.

### 0.4 External topology compatibility (VERIFIED 2026-09-15)

Direct inspection of `autoharness/gates/topology.py`: the current gate would reject a `blocked`
token because `_VALID_LIVE_SHIPMENT_STATUSES = {queued, active, shipped, abandoned}` excludes it
(`topology.py:553`), while `_active_shipments` and `_detect_before_consistency` already treat
`blocked` as non-active (external corroboration of member-disposition-to-`queued`). **This never
affects `155-S`** (no `blocked` token exists during its execution, §0.3 step 1). For the **later
corrective shipment** (run while `154` is `blocked`), require the **smallest upstream change** —
add `"blocked"` to `_VALID_LIVE_SHIPMENT_STATUSES` (one line, external autoharness Python, NOT
backlogit Go) — recorded as an external ratification item. If upstream is unavailable, use an
**audited per-phase `--force` ONLY after the normal gate proves the sole failure is the
unsupported `blocked` status** (no blanket/standing override), removed once upstream ratifies.

### 0.5 Flat-manifest shipment scope rule (rev4 clarification — 2026-09-15, authoritative)

> rev4 does **not** change §0.1–§0.4. It adds one authoritative product rule (below, **SBLK-R28**)
> and the scope-correction task addendum (§0.6). It was raised after direct inspection of live
> shipment `155-S` showed the shipment lifecycle deriving its release scope by **expanding a listed
> feature into all of its descendants** (`releaseScopeItemIDs` → `descendantItems`,
> `internal/core/shipment_lifecycle.go:1142/1219`) and the size/member projection doing the same
> (`compositionMemberIDs`, `internal/core/size_composition.go:291`): `155-S`'s **17-entry** explicit
> manifest (`custom_fields.items` = `174-F` + 16 tasks) projected to **54** `size_composition.members`,
> pulling in the archived `174.001-T…174.038-T` band that is **not** in the manifest. Further
> independent expansions survive even a flat `releaseScope`: (a) the ship/closure cleanup re-expands a
> member feature's descendants directly via `descendantItems` in `collectArchiveCandidateIDs`
> (`shipment_lifecycle.go:800`, which **also** appends the feature's **linked deliberations** via
> `linkedDeliberationIDs` with no membership guard) and `returnUnreleasedFeatureItems` (`:735`, which
> returns-to-backlog and `clearParentID`-orphans unlisted non-terminal descendants); and (b) the
> rollback/snapshot path (`:603-612`) **independently** re-expands the artifact lock/snapshot/restore
> set (`rollbackIDs`/`snapshotShipArtifacts`) with covering-feature ancestors (`featureIDs`) **and
> every** `descendantItems` of each. None of these are neutralized by the derivation change, so all
> must be flattened explicitly (the covering-feature status-rollup revert via
> `nonMemberFeatureSnapshots` is a separate mechanism and is preserved).

**A shipment's scope is EXACTLY its explicit, flat `custom_fields.items` manifest.** The rule:

1. **Membership is explicit and flat.** The manifest is an ordered list of deliverable artifact IDs.
   An artifact is a member **iff its own ID appears in `custom_fields.items`**. Including a feature
   (or any parent) artifact includes **only that artifact**, never its descendants by implication.
2. **Descendants are members only when explicitly listed.** A descendant of a listed feature is in
   scope **iff that descendant's own ID is itself in the manifest**. Feature hierarchy does **not**
   govern shipment membership.
3. **Release scope == explicit manifest.** The set that evidence validation, ship/closure gating,
   member completion, archival cascade, rollback locking, and **any machine-readable release-scope
   or member projection** operate on **equals the explicit manifest** — no hierarchy expansion.
4. **Dependencies govern ordering, not membership.** Dependency edges (`blocks`) determine execution
   ORDER and wave layering among manifest members. They never add or remove members.
5. **Parent-first is ordering only.** A covering feature is listed before its listed children as a
   deterministic manifest ORDERING convention; this is **never** an expansion mechanism. Preserve
   parent-first as ordering, not as a descendant-inclusion rule.
6. **Feature-only / zero-executable-task manifests are valid and must not strand the active slot.**
   A manifest whose only executable content is a covering feature (no task members listed) is
   well-formed. Shipping/closing it completes the shipment and its **listed** members only, completes
   or archives **no** unlisted descendant, and MUST release the single active slot (P-001) rather
   than leaving it occupied by a shipment with zero remaining executable members.
7. **Archival is not a descoping mechanism.** The archived `174.001-T…174.038-T` band is OUTSIDE
   `155-S` scope **because those IDs were never explicit members of the `155-S` manifest**, not
   because they were archived. Archiving them preserved history but was **not required** to descope
   them from `155-S`, and superseded archived tasks are **never** restored merely to satisfy shipment
   scope.
8. **Shipment processing must not touch non-member artifacts — including hierarchy-derived
   ancestors (rev6, 2026-09-16, authoritative).** Ship, rollback, and closure MUST NOT mutate,
   snapshot, restore, or lock any artifact whose ID is absent from `custom_fields.items`. In
   particular, a **hierarchy-derived covering-feature ancestor** that is not itself a manifest member
   receives **no** status-rollup handling: the member-completion cascade must **stop at the manifest
   boundary** and must not roll a non-member ancestor to `done`/`archived`, and no non-member
   snapshot/restore compensation may run. (Retaining such compensation after the ancestor is dropped
   from the ship's artifact lock is a lost-update concurrency defect: the ancestor is snapshotted then
   restored **without a lock**, so a concurrent mutation to it between snapshot and restore is silently
   overwritten.) Explicitly-listed **feature members** still receive their own governed shipment status
   handling (marked `done` on release); only hierarchy-derived non-members are excluded. Re-locking a
   non-member ancestor to make the snapshot/restore safe is **rejected** — it would re-introduce the
   hierarchy expansion this rule forbids; the compensation is **eliminated**, not protected.

**SBLK-R28 (M) [shared] — Flat explicit shipment membership.** A shipment's release scope is its
flat explicit `custom_fields.items` manifest. No lifecycle, gate, completion, archival, or projection
surface may expand a listed parent into unexpressed descendants; descendants are in scope only when
their IDs are explicitly listed. No ship/rollback/closure surface may mutate, snapshot, restore, or
lock a **non-member** artifact (including a hierarchy-derived ancestor): non-member ancestors receive
**no** status-rollup handling, while explicitly-listed feature members do. Parent-first order is
manifest ordering only. Feature-only manifests are valid and their ship/closure must free the active
slot. This is a portable contract requirement (§2.5 [shared] layer); the specific backlogit code-path
corrections are [local] (§0.6).

### 0.6 Scope-correction task addendum (feature `174-F` / shipment `155-S`) — +4 tasks, RED-before-GREEN, ≤2h

The flat-manifest rule (§0.5) is delivered as part of `155-S` (Ship implements it **test-first**),
layered on top of the finalized `blocked`-lifecycle seam so it lands on the stabilized shipment
lifecycle/gate code. Six ≤2h tasks are appended to `174-F`; each RED harness precedes its GREEN
maker. The pre-existing 16-task `blocked`-lifecycle set (§0.2) and its RED contracts are **unchanged**.

| Task | Role | Scope | Domain | Depends on |
|---|---|---|---|---|
| `174.055-T` | SCOPE-RED-A | Behavior RED harness: shipment release scope == explicit flat manifest — a task-only manifest scopes to exactly its items; a listed feature does **not** expand to unlisted descendants; an explicitly listed descendant **is** in scope (asserts over the retained `releaseScopeItemIDs`/evidence-set derivation seam). <4 scenarios | tests | `174.044-T` |
| `174.059-T` | SCOPE-RED-C | Behavior RED harness: the ship/closure **rollback lock/snapshot/restore** set == `{shipment} ∪ flat manifest` — an unlisted ancestor feature + unlisted descendant are absent from the snapshot/lock set and non-restorable (pins the independent `:603-612` expansion). <4 scenarios | tests | `174.044-T` |
| `174.056-T` | SCOPE-RED-B | Behavior RED harness (regression): an unlisted **blocked** descendant is excluded from the projection; unlisted (incl. **archived**) descendants are excluded; a **feature-only** manifest scopes to `{feature}` and its ship/closure frees the active slot (zero-executable-task lifecycle). <4 scenarios | tests | `174.044-T` |
| `174.060-T` | SCOPE-RED-D | Behavior RED harness: `collectArchiveCandidateIDs` excludes an unlisted **terminal-but-not-archived** (`done`/`accepted`) descendant **and** an unlisted **linked deliberation** of a member feature from `ArchivedIDs`; no archived artifact restored to satisfy scope. <4 scenarios | tests | `174.044-T` |
| `174.057-T` | SCOPE-IMPL-1 | Flatten release-scope **derivation**: `releaseScopeItemIDs` (`shipment_lifecycle.go:1142`) flattened **in place** (seam retained, not bypassed) to return the flat explicit manifest; evidence set/`validateMemberGateEvidence` + `completeReleaseScope` flatten transitively; **AND** neutralize the independent rollback/snapshot expansion at `:603-612` so the lock/snapshot/restore set == `{shipment} ∪ flat manifest` (separate non-member feature status-rollup revert preserved). Preserve parent-first as ordering. Makes `174.055-T` + `174.059-T` green | code | `174.055-T`, `174.059-T`, `174.045-T`, `174.051-T` |
| `174.058-T` | SCOPE-IMPL-2 | Flatten the machine-readable **projection** + **ship/closure feature-hierarchy cleanup**: `compositionMemberIDs` (listed task members, feature excluded → `155-S` **22**) and the independent re-expansions in `collectArchiveCandidateIDs` (`:800`, unlisted **terminal-but-not-archived** descendants **and linked deliberations** left untouched) + `returnUnreleasedFeatureItems` (`:735`, no archival/`clearParentID`-orphan of unlisted descendants); feature-only/zero-executable-task ship/closure completes listed members only and frees the active slot; update coupled legacy tests. Makes `174.056-T` + `174.060-T` green | code | `174.056-T`, `174.060-T`, `174.057-T` |

**Dependency sub-graph (appended; existing W1–W9 unchanged):**

```
174.044(R6) ─┬─► 174.055(SCOPE-RED-A) ─┬─► 174.057(SCOPE-IMPL-1) ─► 174.058(SCOPE-IMPL-2)
             ├─► 174.059(SCOPE-RED-C) ─┘
             ├─► 174.056(SCOPE-RED-B) ─┬─────────────────────────────► 174.058
             └─► 174.060(SCOPE-RED-D) ─┘
174.045(R7a)+174.051(R7b) ─► 174.057
```

**Integrated waves (still 9):** `174.055`,`174.059`,`174.056`,`174.060` join **W7** (deps ≤ W6);
`174.057` joins **W8** (deps in W7); `174.058` joins **W9** (deps in W7/W8). RED-deliverable closing
waves add: SCOPE-RED-A (`174.055-T`) and SCOPE-RED-C (`174.059-T`) closed by `174.057-T`@**W8**;
SCOPE-RED-B (`174.056-T`) and SCOPE-RED-D (`174.060-T`) closed by `174.058-T`@**W9**.

**Updated `155-S` manifest — 22 tasks (23 members incl. `174-F`).** Appended in dependency order:
`… → 174.050 → 174.055 → 174.059 → 174.056 → 174.060 → 174.057 → 174.058` (each appended ID's
predecessors already precede it, so the flat manifest order remains a valid parent-first topological
order).

**No public API seam is introduced** — the correction changes existing internal behavior
(`releaseScopeItemIDs`, `compositionMemberIDs`, `validateMemberGateEvidence`) only; `NormalizeShipmentItems`
(the exported flat-manifest accessor) is unchanged. The external numeric-predecessor topology/wave
gate (`autoharness/gates/topology.py`) reads neither `releaseScopeItemIDs` nor this projection, so it
is **independent** of this correction; **no `--force` override is authorized or applied.**

> **rev6 addendum (2026-09-16, branch `chore/stage-155-flat-shipment-scope`) — non-member ancestor
> rollup elimination (§0.5 point 8).** The rev4/rev5 tasks preserved the non-member covering-feature
> status-rollup revert (`nonMemberFeatureSnapshots`/`restoreRolledUpNonMemberFeatures`). Once
> `174.057-T` drops unlisted ancestors from the outer artifact lock, that preserved path snapshots and
> restores an **unlocked** ancestor — a **P1 lost-update** when the ancestor is mutated between
> snapshot and restore. Per §0.5 point 8 the fix **eliminates** the non-member rollup (no re-lock, no
> CAS). **+2 tasks (delta now +8):**
>
> | Task | Role | Scope | Domain | Depends on |
> |---|---|---|---|---|
> | `174.061-T` | SCOPE-RED-E | Behavior RED harness: a hierarchy-derived non-member covering-feature ancestor receives no ship-originated status write (no rollup, no restore) and a concurrent mutation to it is never overwritten on ship success **or** forced rollback; an explicitly-listed member feature still gets governed `done` (control). Uses the existing `persistArtifactPreLockHook`/`persistArtifactWriteFn` seams. `^TestUNonMemberRollupSafe_`. <4 scenarios | tests | `174.044-T` |
> | `174.062-T` | SCOPE-IMPL-3 | Bound `completeReleaseScope`→`cascadePersistedParentStatuses` rollup to explicit members (ctx boundary) and delete `snapshotNonMemberFeatureStatuses`/`restoreRolledUpNonMemberFeatures` + their `ShipShipment`/`classifyShippedEventAppendFailure` call-sites; explicitly-listed feature members keep governed `done` handling; update coupled tests. Makes `174.061-T` green | code | `174.061-T`, `174.057-T`, `174.058-T` |
>
> **Waves 9 → 10:** `174.061`@**W7**; `174.062`@**W10**. RED-deliverable closing wave adds SCOPE-RED-E
> (`174.061-T`) closed by `174.062-T`@**W10**. **Updated `155-S` manifest — 24 tasks (25 members incl.
> `174-F`)**, appended `… → 174.058 → 174.061 → 174.062`. No public/exported API introduced (unexported
> context boundary key only); topology/wave gate still independent; no `--force`.

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
| **[shared]** — portable contract, must stay stable | SBLK-R1 (token value), R2 (transition set), R4 (single-active exclusion invariant), R5 (unblock-readiness *semantics*: blocker-resolution assertion + free slot), R6 (non-claimable/non-executable while blocked), R7 (audit **field names** + authoritative event), R9 (member-disposition semantics: no active release-unit execution while blocked), R10 (evidence/branch/checkpoint/member-snapshot preserved), R11 (ready-work exclusion + kept-in-queue *behavior*), R12 (dependency stays execution-blocking), R17 (no direct `blocked→terminal`), R20, R22, R27 (partial-failure durability semantics), **R28 (flat explicit shipment membership — §0.5)** |
| **[local]** — backlogit implementation detail, replaceable | SBLK-R3 (choke-point seams/wiring; the exact Go API shape `BlockShipment(ctx, ws, shipmentID string, opts BlockOptions) (*models.Artifact, error)` / `UnblockShipment(ctx, ws, shipmentID string, opts UnblockOptions) (*models.Artifact, error)`, the `BlockOptions`/`UnblockOptions` structs, and the `blerrors.ErrNotImplemented` + `blerrors.ErrShipmentBlockedRequiresEnvelope` sentinels), R5 (`--confirm` flag + `.locks/` free-slot *mechanism*), R8 (clear-helper mechanism), R13 (CLI verb shape), R14 (MCP tool shape), R15 (SQLite projection), R16 (`doctor` checks), R18 (backlogit enum/index additive handling), R24/R25 (bootstrap seam + normalization *mechanism*) |
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
  the **authoritative durable correlated audit record** (durable audit evidence) and carries
  actor + reason + `resume_checkpoint_ref`. It is NOT a non-repudiation/tamper-evident record:
  the JSONL is a mutable fsynced append with no signing, and `blocked_by` is advisory.
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
