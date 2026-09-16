---
chunk_strategy: h1-h2-h3
description: "Deliberation for stash 808E4323 — introduce a resumable, non-terminal shipment `blocked` lifecycle status; reconcile with the stalled S12 `parked` state (164-F); capture 7AA35A39 as informing defect only."
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-09-14-resumable-shipment-blocked-lifecycle-status-deliberation.md
title: "Deliberation: Resumable shipment `blocked` lifecycle status (canonical, supersedes S12 parked)"
docline:
    stash_id: 808E4323
    linked_stash_kind: feature
    informing_defect_stash_id: 7AA35A39
    status: decided
    created_at: 2026-09-14T13:41:59Z
---

# Deliberation: Resumable shipment `blocked` lifecycle status

**Depth:** standard. **Route:** `deliberate` (feature-shaped intake `808E4323`,
`requires deliberation: true` for the informing defect `7AA35A39`).
**Subject:** the shipment-lifecycle `blocked` capability (feature-shaped), NOT the
individual tasks and NOT the informing workflow defect.

Companion product spec: `docs/product-specs/2026-09-14-resumable-shipment-blocked-lifecycle-status.md`
(requirement IDs SBLK-R1…R27).

---

## 0. CURRENT AUTHORITATIVE REVISION — rev3 (2026-09-15)

> §0 is the **authoritative current decision**; rev1/rev2 §0 and the detailed sections below are
> **retained as superseded audit trail**. The core decision is unchanged (canonical, non-terminal,
> resumable shipment `blocked`, supersedes S12 `parked`). **rev3 removes the rollout circularity,
> the generic-move bootstrap, and the `155-S` topology `--force` entirely** via a branch-scoped
> source-of-truth (authoritative §6 below).

* **Decision (unchanged):** Option A — one canonical shipment status `blocked`; non-terminal;
  excluded from the single active slot; transitions `active → blocked`, `blocked → {queued,
  active}`; correlated **intent→commit** with a durable **intent + complete preimage** persisted
  before any mutation; workspace-global lock held across block/unblock/claim; target-aware unblock
  (to-queued preserves snapshot; to-active exact restore); only Claim/unblock-to-active create an
  active shipment; never active members under a queued shipment.
* **Decomposition (rev3):** rev1/rev2 `174.001-T…174.038-T` superseded and **archived** (terminal,
  `archived_status: blocked`, history/events preserved) so they sit OUTSIDE the live 174-F release
  scope; replaced by **16 RED-before-GREEN ≤2h tasks** — `174.039-T…174.051-T` (13) plus the core
  stateful-seam split tasks `174.052-T` (declaration-only compile-green) and `174.053-T`
  (core-lifecycle behavior RED) and the R4g pre-edge guard `174.054-T` (generic `MoveShipmentStatus`
  block/unblock refusal landed before the R5/R6 transition-table edges) (spec §0.2) — so `155-S`
  now carries **17 members including `174-F`**. The core `BlockShipment`/`UnblockShipment` seam
  follows **source-shape RED → declaration-only compile-green → behavior RED → implementation** (R1
  split into R1s/Rd/R1b), and its EXACT [local] Go API shape (`BlockShipment(ctx, ws, id, BlockOptions)`
  / `UnblockShipment(ctx, ws, id, UnblockOptions) (*models.Artifact, error)` + `blerrors.ErrNotImplemented`
  sentinel) is pinned by the R1s source-shape harness, kept separate from the [shared] contract.
  `164.002-T` (S12 forward-repair) is **retired** — set `blocked` (history preserved) with **no
  dependency on `174.050-T`** (the obsolete cross-shipment edge is removed); `146-S` no longer
  couples to `155-S`.
* **Authoritative rollout (no circularity; §6):** (1) after staging merges, `155-S` ships
  **normally on `main`** — no pre-block, no `155` topology force, no `blocked` token during its
  execution; (2) then a **backlog-only `chore/block-154`** branch off synchronized `main` imports
  authoritative `154-S`/`173-F` provenance from Ship branch
  `feat/shipment-claim-scheduler-baseline-marker-enabling-precondition@dd9f01a1` with content
  hashes + allowlist (no code); (3) the **shipped governed `BlockShipment`** performs a legal
  `active → blocked` there, then the blocked backlog state is PR-merged to `main`; (4) the
  corrective `7AA35A39` shipment runs on `main` while `154` is blocked, then `main` is merged into
  the `154` feature branch and governed **unblock-to-active** + checkpoint resume follow.
* **Topology (spec §0.4):** `155-S` never exercises the gate on `blocked`; the corrective shipment
  needs the one-line upstream allowlist add if available, else an audited per-phase `--force`
  ONLY after the normal gate proves the sole failure is the unsupported `blocked` status.
## 1. Problem frame

Shipment `154-S` (covering feature `173-F`, the scheduler-baseline marker) is `active`
but cannot progress: the build-feature Step 0.5 RED-deliverable pre-claim zero-delta
baseline union conflicts with the mandatory Ship task-claim mutation (Ship checkpoint
`checkpoint-20260914-070735.json`: `blocker_token: RED_DELIVERABLE_DELTA_OUT_OF_SURFACE`,
`deferred_stash_id: 7AA35A39`, branch
`feat/shipment-claim-scheduler-baseline-marker-enabling-precondition`, active task
`173.006-T`). We must NOT abandon `154-S` — its branch, Wave 1 harnesses, commits,
checkpoints, and intent must remain resumable — but leaving it in the single `active`
slot (P-001) blocks all other delivery.

The shipment lifecycle today is `queued → active → shipped | abandoned`
(`internal/core/shipment.go:24-36`) with guard `isValidShipmentTransition`
(`shipment.go:778-786`). There is no non-terminal parking state. This is exactly the gap
the operator identifies for a `blocked` status.

---

## 2. Precedent reconciliation (the decisive question)

Two existing artifacts overlap and MUST be reconciled before planning:

**(a) Existing intake — stash `808E4323` (feature):** *"Add support for a `blocked`
state for shipments, including a `blocked_reason` attribute … such as unmet dependencies
on other shipments."* This is the canonical feature-shaped intake for this capability.
This deliberation adopts it as the subject rather than synthesizing a new feature.

**(b) Overlapping precedent — `164-F` / `146-S` (S12 "parked state"):** the S12 plan
(`docs/exec-plans/2026-09-03-s12-shipment-lifecycle-features-plan.md`) proposes a
`parked` state: *"add a `parked` state; parked shipments remain in the queue directory
and are not archived, but queue queries and ready-work selection filter them out;
unparking restores eligibility."* Its plan review is **FAIL (attempt 2)** — a controlling
P1 finding was precisely *"the new `parked` state is not integrated into the closed
shipment status taxonomy consumed by already-planned gates."* The
`2026-09-06-queued-shipment-ordered-scope-decision.md` marks S12 as `RE-PLAN` and driven
by stash `FF6D467A`. `146-S` is still `queued`; parked was never implemented.

`parked` (voluntary pause to let higher-priority work proceed) and `blocked`
(blocker-driven, resumable, with `blocked_reason`) are the **same underlying mechanism**:
a non-terminal state that stays in the queue directory, is excluded from ready-work
selection, and is reversible. Shipping both as distinct lifecycle states would reproduce
the exact taxonomy-bloat P1 that failed the S12 review.

---

## 3. Options

**Option A — `blocked` is the canonical non-terminal resumable state; supersede S12
`parked` (CHOSEN).** Implement one non-terminal, queue-excluded, resumable lifecycle
state named `blocked`, carrying `blocked_reason` (and richer audit metadata). The S12
`parked` unit (`164.001-T`) is superseded; a voluntary pause is expressed as a
`blocked_reason` on the same state. The S12 forward-repair unit (`164.002-T`, `queued →
active` break-glass) is unaffected and remains 146-S's residual scope.

* Pros: single closed taxonomy (fixes the S12 P1 directly); matches the existing intake
  `808E4323` and the operator's detailed semantics; reuses the 124-F precedent for a
  governed `blocked` transition set; no duplicate mechanism.
* Cons: requires an explicit supersession decision on `164-F`/`146-S` (operator-facing
  backlog reconciliation).

**Option B — ship both `parked` and `blocked` as distinct lifecycle states.** Two
non-terminal states sharing the queue-exclusion machinery.

* Pros: preserves a literal "voluntary vs. involuntary" label distinction.
* Cons: reintroduces the taxonomy-integration P1 that failed S12; two near-identical
  states multiply gate/doctor/queue branches for no functional gain. Rejected.

**Option C — extend the S12 `parked` plan instead of authoring `blocked`.** Re-plan
S12 to add reason/audit/single-active semantics onto `parked`.

* Pros: reuses the existing 146-S shell.
* Cons: `parked` naming does not match the intake (`808E4323`) or operator semantics;
  the S12 plan is already at attempt 2 FAIL and entangled with `FF6D467A`'s broader
  re-plan scope. Cleaner to make `blocked` canonical and reduce `146-S` to forward-repair.
  Rejected as the primary path (but see §6 recommendation to the operator).

---

## 4. Decision

Adopt **Option A**. Introduce a single canonical non-terminal shipment lifecycle status
`blocked` per the product spec (SBLK-R1…R27), with the governed transition set
`active → blocked`, `blocked → queued`, `blocked → active` (SBLK-R2), enforced
unconditionally at the core seam (SBLK-R3), excluded from the single active slot
(SBLK-R4), preserving all resumption evidence (SBLK-R10), and carrying
`blocked_reason`/`blocked_at`/`blocked_by`/`resume_checkpoint_ref` audit metadata
(SBLK-R7) cleared at every re-entry choke point on unblock (SBLK-R8).

**Supersession:** the S12 `parked` unit `164.001-T` is superseded by this feature. A
`supersedes` semantic link is recorded from the new covering feature to `164.001-T`, and
a `related_to` link to `164-F`. The forward-repair unit `164.002-T` is untouched and
remains `146-S`'s scope. Stage does not delete or rewrite `164-F`/`146-S`; re-scoping
`146-S` to forward-repair-only is recommended to the operator (§6).

**Distinctions preserved (SBLK-R9/R12, spec §3):** shipment-lifecycle `blocked` is kept
separate from member/task `blocked` (`return-blocked`) and from dependency gating.
Blocking a shipment does NOT cascade member `blocked`.

**Transition-set minimality:** no direct `blocked → abandoned` edge; terminal
abandonment is `blocked → active → abandoned` (SBLK-R17), keeping the guard matrix
small and operator-faithful.

**Cross-repository contract boundary (operator clarification, 2026-09-14).** This capability
is already stashed in the upstream **`autoharness` backlog**; the backlogit implementation is a
**local first-mover expected to be eventually superseded by the autoharness implementation**.
The decision therefore commits to a portability boundary (spec §2.5, SBLK-R20…R23): the
**shared contract** — the `blocked` token, the three-edge transition set, the
non-terminal/resumable/single-active-exclusion behavior, the audit field names, and the
`shipment_status_changed` event — must remain **stable and portable**; the **backlogit-local
implementation** (choke-point seams, `.locks/` global lock, SQLite projection, doctor, CLI/MCP
surface shapes) may be replaced upstream. The local implementation MUST NOT introduce a
conflicting alternate token or local-only semantics (SBLK-R21), and migration to the autoharness
implementation MUST be non-destructive, additive-only, and 1:1 mappable (SBLK-R22). This is a
decisive reason Option A (single canonical `blocked`, no new `parked`/`paused` synonym) is the
correct choice: introducing a backlogit-specific synonym would immediately diverge from the
shared contract.

**External informing reference.** No autoharness stash/backlog ID is identifiable from local
evidence (searched `.autoharness/` config, registry, manifests, backups). Per operator
instruction it is recorded as an **external informing backlog item (upstream autoharness
backlog) without inventing an ID** (SBLK-R23). Local intake was backlogit stash `808E4323`
(consumed → `174-F`). If a concrete reference surfaces later, link via
`link add 174-F <ref> informs`.

---

## 5. Informing-defect handling — stash `7AA35A39` (P-021 C5/C6)

`7AA35A39` is a `DEFERRED SCOPE EXPANSION` bug carrying source refs `task=173.006-T;
feature=173-F; shipment=154-S; PR=N/A; review-thread=N/A`. It is captured here as the
**informing** motivation only and is **NOT harvested, absorbed, or promoted to planning**
this session. It remains active in the stash for a future dedicated session.

**Deferred-scope-expansion triage obligations, recorded:**

* **(A) Duplicate detection — UNCONDITIONAL — result: CLEAN.** A stash scan over
  `RED | baseline | zero-delta | claim bookkeeping | Step 0.5 | scheduler-baseline`
  found only `7AA35A39` describing the RED-baseline-vs-claim-mutation defect. The related
  deliberation `CC0EBB59` (`2026-09-13-shipment-claim-scheduler-reconciliation`) is the
  covering-feature deliberation for `173-F`/`154-S` (the scheduler-baseline marker), i.e.
  the parent work — NOT a duplicate of the workflow defect. No duplicate exists; nothing
  merged or archived.
* **(B) Late-identifier reconciliation — TRIGGERED (`PR=N/A`, `review-thread=N/A`) —
  result: NO LATE IDENTIFIER FOUND.** The Ship-owned residual-risk records citing
  `7AA35A39` are `checkpoint-20260914-070735.json` and `.backlogit/logs/173.006-T.jsonl`.
  Neither carries a PR number or review-thread ID: this is a pre-PR build/RED-gate finding
  that never reached a PR surface. The recorded `N/A` STANDS as a truthful terminal
  record. This is non-blocking and is NOT a C3/C6 shortfall.

Reconciliation over `7AA35A39` is therefore a no-op beyond this record. The
`resume_checkpoint_ref` for `154-S` (`checkpoint-20260914-070735.json`) is carried into
this deliberation as the concrete evidence the `blocked` status must preserve (SBLK-R7/R10).

---

## 6. Recommendations to the operator (backlog reconciliation — NOT executed as edits to 154-S)

> **rev3 supersedes the rev2 pre-governance generic-move bootstrap.** The rev2 items 3–5 (a
> one-time pre-governance generic-move runbook + `155-S` topology `--force` + single-worktree
> handoff off `origin/main`) are **retired**; they solved a circularity that **rev3 removes at the
> source** by ordering the shipment normally on `main` first. The retired text remains in git
> history as superseded audit. The authoritative rollout is below.

1. **`146-S` / `164-F` reconciliation (APPLIED, see §6b).** `164.001-T` removed from `146-S`
   membership; `164.002-T` re-pointed off the superseded `parked`/`174.001-T` chain. Under rev3 its
   dependency is anchored to the **final live replacement task `174.050-T`** (canonical `blocked`
   docs/verification), keeping the S12 forward-repair coherent with the single canonical taxonomy.
   Traceability preserved; no history deleted.

2. **Authoritative rollout sequence (removes rollout circularity — NO pre-block, NO generic move,
   NO `155` topology force).** Operator-owned; Stage does not execute it and does not edit `154-S`
   or the Ship branch.
   1. **`155-S` ships normally on `main` first.** `main`/staging hold `154-S` and `173.006-T` as
      `queued`, so after the staging PR merges the single active slot is free and `155-S` is
      claimed/shipped normally. **No `blocked` token exists during `155-S` execution**, so the
      external topology gate is never exercised on `blocked` and **no `--force` is used for
      `155-S`.**
   2. **Backlog-only `chore/block-154` branch off synchronized `main`.** After `155-S` ships,
      `git switch main && git pull`, then `git switch -c chore/block-154`. Import ONLY authoritative
      backlog/checkpoint provenance for `154-S`/`173-F` members from the preserved Ship branch
      `feat/shipment-claim-scheduler-baseline-marker-enabling-precondition@dd9f01a1` **with content
      hashes and an explicit allowlist (no Go/source/harness code)**. This hydration reconstructs
      the shipment-active + member-active state on the bootstrap branch. The Ship branch is left
      exactly at `dd9f01a1` (never touched/rewritten). No parallel worktrees.
   3. **Governed `BlockShipment` performs a legal `active → blocked` there.** Invoke the
      newly-shipped governed seam (NOT a generic move): it acquires the workspace-global lock,
      persists durable intent + complete preimage, writes the **machine-readable snapshot**
      (`.backlogit/bootstrap/154-S.snapshot.json` — sole source of truth, consumed by `174.047-T`),
      dispositions active members (`173.006-T → queued`) and records governed
      `blocked_reason`/`blocked_at`/`blocked_by`/`resume_checkpoint_ref` + the
      `shipment_status_changed` event. Then commit the COMPLETE governed output as one atomic backlog
      change onto `chore/block-154` — the `154-S` shipment record, every changed member artifact
      (`173.006-T` requeued), the durable intent + preimage + machine-readable snapshot/recovery state,
      and the authoritative per-item event logs for the shipment and each dispositioned member — and
      PR-merge it to `main`. **Committing only `154-S` + snapshot is insufficient** (it would drop the
      changed-member, intent/recovery, and per-item event provenance R9/`174.047-T` and the doctor
      checks consume). **No generic move, no pre-governance rollback,
      no migration debt** (metadata is governed from the first write because `BlockShipment` already
      exists on `main`).
   4. **Corrective `7AA35A39` shipment on `main` while `154` is blocked.** `154` being `blocked` is
      non-active, so the corrective shipment occupies the single active slot legally. Later, merge
      current `main` into the preserved `154` feature branch, resolve backlog state to the blocked
      provenance, invoke governed **unblock-to-active** after the RED-baseline/claim-bookkeeping
      contract is reconciled, and resume from checkpoint `checkpoint-20260914-070735.json`.

3. **External topology compatibility (VERIFIED 2026-09-15).** The current external gate
   (`autoharness/gates/topology.py:553`) rejects a `blocked` token because
   `_VALID_LIVE_SHIPMENT_STATUSES = {queued, active, shipped, abandoned}` excludes it, while
   `_active_shipments` / `_detect_before_consistency` already treat `blocked` as non-active and flag
   a `blocked` shipment that still holds an `active` member (external corroboration of the
   member-disposition-to-`queued` requirement). This **does not affect `155-S`** (§6.2 step 1). For
   the **later corrective shipment** (run while `154` is `blocked`): request the **smallest upstream
   change** — add `"blocked"` to `_VALID_LIVE_SHIPMENT_STATUSES` (one line, external autoharness
   Python, recorded as an external ratification item) — and, only if upstream is unavailable, use an
   **audited per-phase `--force` scoped to that shipment ONLY after the normal gate proves the sole
   failure is the unsupported `blocked` status** (no standing/blanket override; removed on upstream
   ratification).

---
## 6a. External informing item (no invented ID)

The **autoharness stash** already tracks the shared `blocked`-shipment capability (the
upstream owner of contract §0.1(a)); its exact stash ID is **not identifiable from local
evidence** and is therefore recorded as an *external informing backlog item* without inventing
an ID. The **smallest upstream compatibility change** (add `"blocked"` to
`_VALID_LIVE_SHIPMENT_STATUSES` in `autoharness/gates/topology.py`) is likewise an external
ratification item, not a backlogit deliverable.

---

## 6b. Precedent reconciliation applied (146-S / 164-F)

Executed via backlog-native operations in the current staging session: `164.001-T` removed from
`146-S` membership and the obsolete `164.002-T → 164.001-T` dependency removed, retaining
`164.002-T → 174.001-T` and traceability. `164.002-T` re-points to the canonical `blocked`
semantics (not the superseded `parked`). History preserved (no deletion).

> **SUPERSEDED 2026-09-15 (current-HEAD remediation §6c).** `164.002-T` is now **retired**, not
> merely re-pointed. Under rev3's exclusive-activation invariant a shipment-record-only
> `queued → active` forward-repair is illegal (only `Claim`/unblock-to-active create an active
> shipment, both disposing members), and the R9 recovery+normalizer (`174.047-T`) plus R11 doctor
> (`174.049-T`) subsume its reconciliation role. `164.002-T` is set `blocked` (history preserved)
> and its obsolete cross-shipment dependency (`174.050-T`) is removed, so `146-S` no longer retains
> a live member depending on `155-S` and needs no shipment-level `blocks` edge. See §6c.

---

## 7. Scope boundary

In-repo backlogit shipment lifecycle + CLI/MCP/doctor/queue/index + tests + operator
docs. No autoharness edit. No modification of `154-S` by Stage (both the §6 governed block and
the one-time operator-approved pre-governance bootstrap runbook are operator-owned, not executed
by Stage). No shipment claim, no Ship
invocation, no application source/test writes by Stage. The change is additive and
backward-compatible (SBLK-R18). The steady-state sanctioned path to `blocked` for a shipment is
the governed `BlockShipment` seam (SBLK-R24); the generic-move bootstrap is permitted ONLY as an
exceptional, one-time, operator-approved pre-governance migration (manual member disposition +
bounded pre-claim rollback), removed after migration.

---

## 6c. Current-HEAD remediation cycle (2026-09-15, Stage-owned; no source/Ship work)

Applied five P1 remediations to `155-S`/`174-F` (and `146-S`/`164-F`) while `155-S` is `queued`,
editing only Stage-owned planning/backlog artifacts:

1. **RED deliverable contracts (F1).** Added canonical `red-deliverable-contract` blocks to the
   three harness-only RED tasks. Validated with the ACTUAL scheduler parser
   (`scripts/wave-scheduler-sim.ps1` `Read-RedDeliverableContract` + `Test-TaskScopedCommandShape`):
   `174.039-T`/R1 → selector `go test -count=1 -run '^TestUR1_' ./internal/core`, green-makers
   `174.043-T,174.044-T`, closes wave 4; `174.040-T`/R2 → `^TestUR2_`, green-makers
   `174.042-T,174.045-T,174.051-T`, closes wave 5; `174.041-T`/R3 → `^TestUR3_`, green-makers
   `174.047-T,174.048-T`, closes wave 6. All parse clean (0 errors).
2. **Governed BlockShipment commit contents (F2).** §6.2 step 3 (and plan §0.2, spec §0.3) now
   require the governed block commit to carry the COMPLETE governed output — `154-S` record, every
   changed member artifact, durable intent/preimage/snapshot recovery state, AND authoritative
   per-item event logs — not `154-S` + snapshot alone.
3. **164.002-T retired (F3).** Parked-era shipment-record-only `queued → active` forward-repair is
   incompatible with rev3 exclusive activation and subsumed by R9/`174.047-T` + R11/`174.049-T`;
   set `blocked` (history preserved), obsolete `174.050-T` dependency removed.
4. **Release-unit ordering (F4).** Because `164.002-T`'s cross-shipment dependency was removed,
   `146-S` no longer retains a live member depending on `155-S`; no shipment-level `blocks` edge is
   required (consistent with the F3 retire choice).
5. **174.045-T split (F5).** R7 split into `174.045-T`/R7a (route bypass WRITE paths through the
   governed writer) and new `174.051-T`/R7b (guard create-as-active + generic activation refusal);
   both ≤5 functions, both R2 green-makers, both wave 5, RED-before-GREEN preserved. `174.051-T`
   added to the `155-S` manifest after `174.045-T`; dependencies `[174.040-T,174.042-T,174.044-T]`.

Validation: `backlogit sync` (parse_failures=0), `doctor --check-orphans --check-duplicates` (no
new findings on touched artifacts), `wave-scheduler-sim -VerifyAgainstQueue` (186/186 PASS), RED
contract parser (3/3 clean). No source/tests edited, no `154-S` edit, no shipment claim, no PR.

---

## 6d. Current-HEAD remediation cycle 2 (2026-09-15, branch `chore/stage-155`; Stage-owned)

Completes the two P1 gaps left open after §6c. §6c removed the obsolete `164.002-T → 174.050-T`
dependency but left the retired task **inside the live queued `146-S` manifest**, and the Rev3 §0
headings still declared 12 tasks.

1. **Manifest reconciliation (smallest valid mutation).** Governed
   `backlogit shipment return-blocked --shipment 146-S --item 164.002-T` removed the retired member
   from the `146-S` executable manifest while keeping it `blocked` (return-blocked journal +
   `shipment_item_returned_blocked`/`item_blocked` events; `related_to 174-F` retained). `146-S`
   manifest is now `[164-F]` — a covering feature whose only children (`164.001-T`, `164.002-T`)
   are `blocked`, i.e. no executable task work (features are excluded from the wave member set).
   No live queued shipment now contains a blocked/superseded member.
2. **Downstream decouple.** Because `146-S` has no executable work, `backlogit dep remove 147-S
   146-S` removed the `147-S → 146-S` `blocks` edge so no downstream shipment is permanently
   blocked by the retired empty release unit. `147-S` now sits in the `queued` frontier. No
   unsupported shipment status invented; `146-S`/`147-S`/`155-S` stay `queued`.
3. **Rev3 §0 topology corrected.** Spec §0.2, decision §0, and plan §0/§0.1 now state 13 live tasks
   (`174.039-T…174.050-T` plus `174.051-T`) / 14 `155-S` members incl. `174-F`, and that
   `164.002-T` is retired with no dependency on `174.050-T`. Old topology remains only in the
   superseded `## Plan Review — Revision 3` audit record.

Validation (current HEAD): `backlogit sync` (1525 artifacts, parse_failures=0); `docs lint` on all
three docs (`valid: true`, 0 violations each); `doctor` — 23 pre-existing findings unchanged, none
on touched artifacts; scheduler verification — `164.002-T` is a member of no shipment manifest, no
shipment depends on `146-S`, `147-S` unblocked; `wave-scheduler-sim -VerifyAgainstQueue`
WAVE_SIM_OK 186/186. No source/tests edited, no `154-S` edit, no shipment claim, no PR.

## 6e. Current-HEAD remediation cycle 3 (2026-09-15, branch `chore/stage-155`; Stage-owned)

Two same-contract P1 findings on PR #444 (HEAD `380fa1e9`), backlog/docs only — no source/tests, no
`154-S`, no shipment claim, no PR.

1. **Rd declaration-only purity.** `174.052-T` (Rd) no longer populates the
   `isValidShipmentTransition` blocked-transition table — adding those entries is functional
   transition-enablement that would let generic `MoveShipmentStatus` persist a governed blocked
   transition in the declaration wave. Rd now lands ONLY the `ShipmentBlocked` enum + not-implemented
   `BlockShipment`/`UnblockShipment` stub signatures; `isValidShipmentTransition` stays fail-closed
   after Rd. The R1s source-shape harness (`174.039-T`) drops the transition-table assertion. The
   `active->blocked` / `blocked->{queued,active}` transition enablement moves to the governed
   implementation tasks `174.043-T` (R5) / `174.044-T` (R6), AFTER behavior RED (`174.053-T`), gated
   so generic `MoveShipmentStatus` remains unable to persist a blocked transition. No dependency,
   wave, or manifest change.
2. **Authoritative §0 count.** The plan §0 intro paragraph is corrected from a stale 13-task/14-member
   statement to **15 live tasks / 16 `155-S` members**, explicitly naming `174.052-T` and `174.053-T`.
   Remaining 13/14 references survive only inside clearly-labeled superseded audit records (§6c/§6d,
   `## Plan Review` history).

Validation (current HEAD): `backlogit sync` (1527 artifacts, parse_failures=0); `docs lint` all
three `valid: true`; `wave-scheduler-sim -VerifyAgainstQueue` WAVE_SIM_OK 186/186; RED-contract
parser 4/4 clean with Rd carrying no red block; dependency graph acyclic (9 waves) with 043/044
transition enablement strictly after 053 behavior RED; manifest 16 members == 174-F + 15 live queued
descendants; doctor 23 pre-existing findings, none on 174.*. No source/tests, no `154-S`, no claim,
no PR.

## §6f — Copilot PR #444 remediation cycle 4 (2026-09-15, branch `chore/stage-155`)

Bounded remediation of the two OPEN Copilot review threads on PR #444 (HEAD `409f3c54`),
Stage-owned backlog/docs only; caller replies/resolves the threads. Append-only — prior counts in
earlier records are retained as audit context.

1. **Exact API shape (thread `PRRT_kwDORzozKM6iuWEe`, `174.039-T`).** `BlockShipment`/`UnblockShipment`
   were spelled only as `(...)`. Now pinned to exact [local] signatures grounded in core conventions:
   `BlockShipment(ctx, ws, shipmentID string, opts BlockOptions) (*models.Artifact, error)` and
   `UnblockShipment(ctx, ws, shipmentID string, opts UnblockOptions) (*models.Artifact, error)` with
   `BlockOptions{Reason(required), BlockedBy, ResumeCheckpointRef}` / `UnblockOptions{Target, Confirm, UnblockedBy}`;
   declaration-only sentinel `blerrors.ErrNotImplemented`. Updated 174.039/052/053/043/044/046 + spec
   §0.2/§2.5 + plan Portability boundary. The [local] Go shape (R1s go/ast) stays SEPARATE from the
   [shared] token/edge/field-name/event conformance (U17).

2. **Guard-before-edge (thread `PRRT_kwDORzozKM6iunWq`, `174.045-T`).** R5/R6 enabled the
   `isValidShipmentTransition` edges before R7a's generic `MoveShipmentStatus` refusal (wave 7),
   exposing ungoverned generic block/unblock at waves 5–6. New smallest task **R4g `174.054-T`**@**W4**
   lands a top-level fail-closed `MoveShipmentStatus` block/unblock refusal (sentinel
   `blerrors.ErrShipmentBlockedRequiresEnvelope`, placed ABOVE the transition-table check) strictly
   before any edge; R5/R6 now depend on it; R7a keeps the remaining bypass routing. Governed seam
   exempt by construction (writes beneath the choke point). Scope: **16 tasks / 17 members incl. 174-F**.

Validation (current HEAD): `backlogit sync` parse_failures=0; `docs lint` all three `valid: true`;
`wave-scheduler-sim -VerifyAgainstQueue` WAVE_SIM_OK; RED-contract parser clean (174.054 carries no
red block); dependency graph acyclic (9 waves) with 174.054@W4 strictly before 043@W5/044@W6; manifest
17 members == 174-F + 16 live queued descendants; doctor pre-existing findings only, none on 174.*.
No source/tests, no `154-S`, no claim.

## §6g — Copilot PR #444 remediation cycle 5 (2026-09-15, branch `chore/stage-155`, HEAD `8fb4e8b5`)

Bounded remediation of two OPEN Copilot review threads on PR #444, Stage-owned backlog/docs only;
caller replies/resolves the threads. Append-only — §6a…§6f above are retained verbatim as audit
context (their now-superseded ownership wording for the refusal sentinel is preserved intentionally).

1. **Refusal-sentinel declare-after-use cycle (thread `PRRT_kwDORzozKM6ivJeO`, `174.040-T`/`174.054-T`).**
   The R2 behavior harness `174.040-T` (depends only on Rd `174.052-T`) references
   `blerrors.ErrShipmentBlockedRequiresEnvelope`, but that sentinel was INTRODUCED by R4g `174.054-T`,
   which depends on `174.040-T` — a declare-after-use cycle causing a compile failure. Fix: the
   refusal sentinel is now a **declaration-only symbol landed by Rd `174.052-T`** alongside
   `blerrors.ErrNotImplemented` (a bare `errors.New(...)` var, NO behavior — Rd stays
   declaration-only), and its EXACT name is **source-shape-gated by R1s `174.039-T`** (go/ast asserts
   `internal/errors` declares it). `174.040-T` now compiles against the already-declared sentinel and
   fails behaviorally only; R4g `174.054-T` now WIRES/uses the sentinel in the top-level guard rather
   than introducing it. No dependency-graph edge changed; graph stays acyclic (9 waves); compile-green
   wave semantics hold (R1s@W1 red → Rd@W2 lands both sentinels + stubs → green; R2 behavior-red
   compiling against Rd; R4g@W4 turns the refusal portion green). Updated 174.039/052/054/040 + spec
   §0/§2.5 tables + plan task tables/P1-A/P1-B narrative + new plan P1-C.
2. **`non-repudiation` overclaim on the mutable JSONL event (thread `PRRT_kwDORzozKM6ivJej`).** The
   plan (U3 SBLK-R7 + risk table) and spec (SBLK-R7) called the fsynced-but-mutable, unsigned
   `shipment_status_changed` JSONL record the "non-repudiation record" while `blocked_by` is advisory
   and no signing/tamper-evidence exists. Replaced with accurate wording — **"authoritative durable
   correlated audit record" / "durable audit evidence"** — in all authoritative current docs, with an
   explicit note that it is NOT a non-repudiation/tamper-evident guarantee. No auth/signing scope
   added; no active acceptance criterion claims non-repudiation. Superseded audit history retained.

Validation (HEAD after commit): `backlogit sync` parse_failures=0; `docs lint` all three
`valid: true`; `wave-scheduler-sim -VerifyAgainstQueue` WAVE_SIM_OK; RED-contract parser clean
(`174.054-T` carries no red block); dependency graph acyclic (9 waves) with `174.054-T`@W4 strictly
before 043@W5/044@W6; manifest 17 members == 174-F + 16 live queued descendants; targeted contract
check — both sentinels declared by Rd `174.052-T`, asserted by R1s `174.039-T`, referenced (not
introduced) by R2 `174.040-T` and R4g `174.054-T`; no `non-repudiation` claim remains in any
authoritative current doc/live task; doctor pre-existing findings only, none on 174.*. No
source/tests, no `154-S`, no claim.

## §6h — Flat-manifest shipment scope determination (rev4, 2026-09-15, branch `chore/stage-155-flat-shipment-scope`; Stage-owned)

Operator clarified the authoritative product rule: **a shipment is a FLAT manifest of explicitly
listed deliverables (`custom_fields.items`); it is not expanded or encumbered by feature hierarchy.
Including a feature does not implicitly include its descendants. Dependencies govern execution
ordering; feature hierarchy does not govern shipment membership.** Backlog/docs + planning artifacts
only — no source/tests written by Stage, no `154-S`, no shipment claim, no PR.

**Problem frame.** Direct code inspection (engram daemon unavailable → ENGRAM_DEGRADED, bounded local
read-only inspection) confirmed the live defect: the shipment lifecycle derives release scope by
expanding a listed feature into all descendants — `releaseScopeItemIDs`
(`internal/core/shipment_lifecycle.go:1142`) → `descendantItems` (`:1219`, BFS on `ParentID`,
`IncludeArchived:true`) — and the member/size projection `compositionMemberIDs`
(`internal/core/size_composition.go:291`) expands feature→children. `155-S`'s 17-entry explicit
manifest projected to **54** `size_composition.members`, pulling in the archived `174.001-T…174.038-T`
band that was never an explicit member. Expanded `releaseScope` is consumed by member-evidence
validation (`validateMemberGateEvidence`), `completeReleaseScope` (closure), `collectArchiveCandidateIDs`
(archival cascade), rollback locking, and `returnUnreleasedFeatureItems`. No exported API expands;
`NormalizeShipmentItems` (exported accessor) only parses the flat manifest.

**Options considered.**
* **Option A — Descope by archival only (status quo mental model).** Keep hierarchy expansion; rely on
  archiving superseded descendants so they fall out of the expanded set. *Rejected:* archival becomes a
  load-bearing descoping mechanism; a non-archived-but-unlisted descendant is still silently pulled in;
  the projection still misrepresents membership (54 vs 17); it contradicts the operator's flat rule and
  couples membership to lifecycle status.
* **Option B — Flat explicit membership (CHOSEN).** Make release scope equal the flat explicit
  `custom_fields.items` manifest across lifecycle, gate, projection, and archival; descendants are members
  only when explicitly listed; parent-first is ordering only; feature-only manifests are valid and must
  free the active slot. Delivered test-first as +4 tasks on `174-F` layered on the finalized
  `blocked`-lifecycle seam. *Chosen:* matches the operator's authoritative rule, is the smallest correct
  internal change (no public API), and makes the projection truthful.
* **Option C — New explicit `release_scope` field separate from `items`.** *Rejected:* redundant with the
  existing flat manifest, adds a public schema seam and migration for no behavioral gain (YAGNI); the
  operator's rule is precisely that `items` IS the scope.

**Decision (Option B) — SBLK-R28 [shared].** A shipment's release scope is its flat explicit
`custom_fields.items` manifest. No lifecycle/gate/completion/archival/projection surface expands a listed
parent into unexpressed descendants; descendants are in scope only when explicitly listed; parent-first
is manifest ordering only; feature-only/zero-executable-task manifests are valid and their ship/closure
frees the single active slot (never strands it). Recorded authoritatively in spec §0.5/§0.6 + plan §0.4.

**Rev3 reconciliation.** Spec §0.2 and plan §0.1 previously said the archived `174.001-T…174.038-T` band
"sit[s] OUTSIDE the live 174-F release scope" — phrasing that assumed feature-root expansion. Reconciled:
those IDs are outside `155-S` scope **because they were never explicit members of the `155-S` manifest**,
not because they were archived; archival preserved history but is **not required** to descope them, and
**no archived/superseded task is restored merely for shipment scope**.

**Task delta (+4, all ≤2h, ≤5 functions, <4 scenarios), RED-before-GREEN:** `174.055-T` (SCOPE-RED-A,
tests) + `174.056-T` (SCOPE-RED-B regression, tests) precede `174.057-T` (SCOPE-IMPL-1, code, flatten
the `releaseScopeItemIDs` derivation; evidence/completion flatten transitively — **rollback is an
INDEPENDENT expansion, re-scoped in §6i (rev5)**) + `174.058-T`
(SCOPE-IMPL-2, code, flatten the `compositionMemberIDs` projection AND the independent ship/closure
descendant re-expansions in `collectArchiveCandidateIDs`/`returnUnreleasedFeatureItems`, plus
feature-only active-slot safety and coupled-test updates). The two ship/closure functions re-expand
descendants directly (not via the `releaseScope` parameter), so they are flattened by `174.058-T`, not
`174.057-T` — a distinction confirmed by the cycle's plan review. Deps (blocks): `055→044`, `056→044`,
`057→{055,045,051}`, `058→{056,057}`. Integrated waves stay **9** (`055`,`056`@W7; `057`@W8; `058`@W9).
`155-S` → **20 tasks / 21 members incl. `174-F`** (flat `size_composition.members` = 20 listed tasks,
feature excluded as non-sizable; currently 58 under expansion), appended in dependency order. Existing
`blocked`-lifecycle tasks and their RED contracts are unchanged.

**Topology / force posture.** The external numeric-predecessor wave/topology gate
(`autoharness/gates/topology.py`) reads neither `releaseScopeItemIDs` nor the member projection, so it is
**independent** of this correction (verified by inspection; the P-002.6 wave-scheduler simulation is
bound to the 130-S/147-F fixture, not 155-S). **No `--force` override is authorized or applied.**

Validation evidence and the review verdict for this cycle are recorded in the companion plan's
**"Plan Review — Revision 4 (flat-manifest shipment scope)"** section.

## §6i — Flat-manifest scope hardening (rev5, 2026-09-16, branch `chore/stage-155-flat-shipment-scope`; Stage-owned)

Bounded remediation of **four P1 findings** raised against the rev4 flat-manifest addendum. All fixes
are Stage-owned backlog/docs only — no source/tests written by Stage, `155-S` stays `queued`, no PR.
The findings and their dispositions:

* **P1-1 — rollback/snapshot path independently expands (mis-scoped as "flatten transitively").**
  `rollbackIDs`/`snapshotShipArtifacts` are built independently at `shipment_lifecycle.go:603-612`,
  appending covering-feature ancestors (`featureIDs`) **and every** `descendantItems` on top of
  `{shipmentID} ∪ releaseScope` — so flattening the derivation alone does NOT flatten the artifact
  lock/snapshot/restore set. **Disposition:** this expansion is re-scoped to `174.057-T` (neutralize
  `:603-612`), with new RED harness `174.059-T` (SCOPE-RED-C, `^TestURollbackScopeFlat_`) proving the
  lock/snapshot/restore set set-equals `{shipmentID} ∪ flat manifest` and that unlisted ancestors and
  descendants are absent and non-restorable. The covering-feature **status-rollup** revert via
  `nonMemberFeatureSnapshots`/`restoreRolledUpNonMemberFeatures` is a SEPARATE mechanism and is
  preserved unchanged.
* **P1-2 — RED/GREEN contract inconsistent for `releaseScopeItemIDs`.** RED (`174.055-T`) pins the
  `releaseScopeItemIDs` seam, but the rev4 `174.057-T` instruction bypassed it with a direct
  `explicitScope` assignment. **Disposition:** `174.057-T` now flattens `releaseScopeItemIDs`
  **in place** (`return uniqueNonEmptyStrings(itemIDs)`, seam + signature retained; line 549 still
  calls it), removing the bypass instruction, so RED and GREEN target the same function and all callers
  consume its flat result. Pins to 174.055/spec §0.5/plan §0.4 as already written.
* **P1-3 — `collectArchiveCandidateIDs` also appends unlisted linked deliberations.** The covering-feature
  loop appends `linkedDeliberationIDs(feature)` with no membership guard. **Disposition:** included in
  `174.058-T` ownership; new RED harness `174.060-T` (SCOPE-RED-D, `^TestUArchiveCandidateFlat_`)
  asserts an unlisted linked deliberation is absent from `ArchivedIDs` and untouched.
* **P1-4 — archive RED used already-archived descendants (a no-op the collector skips).** The collector
  skips `archived` descendants, so that assertion never exercised the expansion. **Disposition:**
  `174.060-T` uses an unlisted **terminal-but-not-archived** descendant (status `done`/`accepted`),
  which the collector DOES append today, asserting it stays untouched and absent from `ArchivedIDs`;
  archived-descendant projection coverage is retained separately (in `174.056-T` and as a
  `174.060-T` negative-control that no archived artifact is restored for scope).

**Task delta (rev5): +2 (total scope-correction delta now +6).** New RED tasks `174.059-T`
(SCOPE-RED-C → green-maker `174.057-T`@close-wave 8) and `174.060-T` (SCOPE-RED-D → green-maker
`174.058-T`@close-wave 9), each `dep 174.044-T`, each ≤2h / <4 scenarios. Impl edges added:
`057→059`, `058→060`. Waves stay **9** (`055`,`059`,`056`,`060`@W7; `057`@W8; `058`@W9). `155-S`
manifest → **22 tasks / 23 members incl. `174-F`** (flat `size_composition.members` target updated
`20 → 22`), appended in dependency order `… 174.055 → 174.059 → 174.056 → 174.060 → 174.057 →
174.058`. **No public/exported API introduced**; no archived/superseded task restored; **no `--force`
override authorized or applied.** The external topology/wave gate is still independent (it reads
neither `releaseScopeItemIDs`, the projection, nor the rollback set). Validation evidence and verdict
recorded in the companion plan's **"Plan Review — Revision 5 (flat-manifest scope hardening)"**
section.

## §6j — Non-member ancestor rollup elimination (rev6, 2026-09-16, branch `chore/stage-155-flat-shipment-scope`; Stage-owned)

Bounded remediation of **one P1 concurrency defect** in the rev5 flat-scope plan. Stage-owned
backlog/docs only — no source/tests written by Stage, `155-S` stays `queued`, no PR/Ship work.
**Concurrency fix-cycle-3 (2026-09-16)** resolved two further same-contract P1 findings against this
addendum without a redesign: (1) the member-completion cascade boundary must be carried on a separate
in-closure context and must not leak onto the ship's escaping outer `ctx` reaching the post-ship
`FirePost` hook (else an unrelated post-ship-hook update loses its global cascade) — pinned by the new
RED harness `174.063-T`; and (2) authoritative terminology now distinguishes the RELEASE/MEMBER scope
(flat `custom_fields.items`) from the transactional lock/snapshot/rollback set
(`{shipment control record ID} ∪ explicit manifest IDs`, the shipment record being the sole exemption),
removing "manifest only" wording. Both remain Stage-owned docs/backlog only.

**Problem frame.** rev5 (§6i P1-1) explicitly PRESERVED the non-member covering-feature status-rollup
revert (`nonMemberFeatureSnapshots` snapshot at `shipment_lifecycle.go:598` +
`restoreRolledUpNonMemberFeatures` at the in-line `:684`, the deferred fallback at `:496`, and the
`classifyShippedEventAppendFailure` indeterminate branch at `:927`), treating it as a mechanism
separate from the artifact lock/snapshot set that `174.057-T` flattens. But `174.057-T` **removes
unlisted ancestors from the outer artifact lock** (`:603-612` → `lockArtifactMutations`). With the
ancestor no longer locked, the preserved path snapshots its status, the `completeReleaseScope` cascade
(`cascadePersistedParentStatuses`) rolls it to `done`/`archived`, and the restore later writes the
snapshot back — so any mutation to the ancestor **between snapshot and restore** is silently
overwritten. This is a **lost-update (TOCTOU) P1**: the compensation designed to protect a non-member
ancestor now itself corrupts a concurrent write to it.

**Options considered.**
* **Option A — Add a separate protected lock/CAS around the non-member snapshot/restore.** Re-lock (or
  compare-and-swap) each non-member ancestor across the snapshot→restore window so a concurrent write
  is serialized or detected. *Rejected:* it re-introduces exactly the hierarchy-derived ancestor
  locking that `174.057-T` removed to satisfy SBLK-R28 ("shipment processing must not lock artifacts
  absent from its manifest"), and adds a second lock lifecycle and drift-detection path — extra
  complexity for a side effect the flat-manifest rule says should not exist at all.
* **Option B (CHOSEN) — Eliminate the non-member ancestor rollup side effects entirely.** Bound the
  member-completion cascade to explicit members so a non-member ancestor is never rolled up, and delete
  the now-dead snapshot/restore compensation. Nothing is snapshotted, restored, mutated, or locked
  outside the manifest, so there is no window to race. Explicitly-listed **feature members** keep their
  own governed status handling (the `:648` member-guarded `setArtifactStatus(done)`), satisfying
  "explicitly listed feature artifacts may still receive governed shipment status handling, but
  hierarchy-derived ancestors cannot."

**Decision (Option B) — SBLK-R28 §0.5 point 8 [shared].** Shipment ship/rollback/closure MUST NOT
mutate, snapshot, restore, or lock any non-member artifact absent from `custom_fields.items`; a
hierarchy-derived non-member ancestor receives no status-rollup handling. The shipment control record
is the **sole** exemption — it is not a `custom_fields.items` member but must be locked, snapshotted,
and transitioned because ship/rollback changes its own status; the transactional lock/snapshot/rollback
set is therefore `{shipment control record ID} ∪ explicit manifest IDs`, never "manifest only". The
membership boundary that bounds the completion cascade MUST be carried on a **separate in-closure
context** confined to the governed member mutations, and MUST NOT be set on the ship's escaping outer
`ctx` (which flows into `collectArchiveCandidateIDs`/`VerifyPostShipConsistency` and the post-ship
`FirePost` hook at `:704`) — otherwise an unrelated post-ship-hook artifact update would inherit the
boundary and have its legitimate global parent-status cascade suppressed. Delivered as **+2 `155-S`
tasks (concurrency fix-cycle-3 adds a third, `174.063-T`)**, RED-first:
`174.061-T` (SCOPE-RED-E, `^TestUNonMemberRollupSafe_`, dep `174.044-T`) pins the contract using the
existing production seams `persistArtifactPreLockHook`/`persistArtifactWriteFn` (so it compiles and is
RED against current code) across three scenarios — no ship-originated write to an unlisted ancestor
on success (with a listed-member-feature governed-`done` control), concurrent-mutation survives ship
success, concurrent-mutation survives forced rollback; `174.063-T` (SCOPE-RED-F,
`^TestUPostShipHookCascadeGlobal_`, dep `174.044-T`) pins that a post-ship `FirePost` hook's unrelated
artifact update still triggers the normal GLOBAL parent-status cascade (the boundary does not leak onto
the escaping outer `ctx`). `174.062-T` (SCOPE-IMPL-3, dep
`174.061-T,174.063-T,174.057-T,174.058-T`) bounds `cascadePersistedParentStatuses` to the
explicit-membership set via a separate in-closure boundary context (never the escaping outer `ctx`) and
deletes `snapshotNonMemberFeatureStatuses` /
`restoreRolledUpNonMemberFeatures` and their `ShipShipment`/`classifyShippedEventAppendFailure`
call-sites, updating the coupled tests. `174.057-T`/`174.059-T` were amended to stop describing the
status-rollup revert as "preserved" — they leave it in place pending `174.062-T`, and their
transactional-set terminology now reads `{shipment control record ID} ∪ explicit manifest IDs`.

**Task delta (rev6): +2 (total scope-correction delta now +8); concurrency fix-cycle-3 adds
`174.063-T` → delta now +9.** Impl edge `062 → {061,063,057,058}`,
RED edges `061 → 044`, `063 → 044`. **Waves 9 → 10** (`174.061`@W7, `174.063`@W7; `174.062`@W10;
SCOPE-RED-E `174.061-T` and SCOPE-RED-F `174.063-T` closed by
`174.062-T`@close-wave 10). `155-S` manifest → **25 tasks / 26 members incl. `174-F`**, appended
`… 174.058 → 174.061 → 174.063 → 174.062`. **No public/exported API introduced** (unexported context boundary
key only); no archived/superseded task restored; **no re-lock/CAS added**; **no `--force` authorized
or applied**; the external topology/wave gate remains independent (it reads neither the cascade, the
projection, nor the rollback set). Validation evidence and verdict recorded in the companion plan's
**"Plan Review — Revision 6 (non-member ancestor rollup elimination; concurrency)"** and
**"Plan Review — Revision 7 (post-ship-hook cascade isolation; terminology)"** sections.
