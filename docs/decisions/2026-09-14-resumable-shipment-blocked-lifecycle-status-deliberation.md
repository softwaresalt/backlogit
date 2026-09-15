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
* **Decomposition (rev3):** rev2 `174.030-T…174.038-T` superseded (parked `blocked`, preserved);
  replaced by **12 RED-before-GREEN ≤2h tasks** `174.039-T…174.050-T` (spec §0.2). `164.002-T`
  re-pointed from superseded `174.001-T` to the final live task `174.050-T`.
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
