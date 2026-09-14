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
(requirement IDs SBLK-R1…R19).

---

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
`blocked` per the product spec (SBLK-R1…R18), with the governed transition set
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

1. **Re-scope `146-S` / `164-F`** to the forward-repair unit (`164.002-T`) only, since
   `164.001-T` (parked) is superseded by this feature. This also removes the parked-state
   item from `FF6D467A`'s re-plan burden.
   * **Forward-repair re-point (from Architecture review):** `164.002-T` currently expresses
     its dependency and acceptance in terms of the superseded `parked` state. Once this
     feature's canonical `blocked` state lands, `164.002-T` must be re-pointed so its
     dependency/acceptance reference the canonical `blocked` state (not `parked`), keeping the
     S12 forward-repair coherent with the single canonical taxonomy. Recorded as an operator
     recommendation; Stage does not rewrite `164.002-T` here (it is `146-S`'s scope).
2. **Operational recommendation for `154-S`:** once this capability SHIPS, move
   `154-S` `active → blocked` with `--reason` referencing `7AA35A39`/`RED_DELIVERABLE_DELTA_OUT_OF_SURFACE`
   and `--resume-checkpoint checkpoint-20260914-070735.json`. That frees the active slot so
   a corrective shipment (the `7AA35A39` fix) can run; `154-S` later unblocks to `active`
   once the RED-baseline/claim-bookkeeping contract is reconciled. Until the capability
   ships, `154-S` remains `active` and parked only in intent.

3. **One-time bootstrap migration for `154-S` (temporary compatibility seam — SBLK-R24/R25/R26).**
   Local code already supports enough to bootstrap `154-S` into `blocked` *before* the governed
   seam exists (verified 2026-09-14: `models.StatusBlocked` at `internal/models/artifact.go:17`;
   generic `move` → `core.UpdateArtifactWithGate` at `internal/cli/move.go:62`;
   `list --type shipment --status blocked` works). This is a **temporary compatibility seam
   only**, never the final contract. **Stage does NOT execute it** (operator-owned; `154-S` must
   not be edited by Stage). Procedure the operator/Ship may run:

   **Pre-verification (capture baseline):**
   * `backlogit shipment get 154-S` → confirm `status: active`, record covering feature `173-F`,
     the member task statuses (esp. active `173.006-T`), branch
     `feat/shipment-claim-scheduler-baseline-marker-enabling-precondition`, and checkpoint
     `checkpoint-20260914-070735.json`.
   * `git status` on `.backlogit/` → confirm no unexpected pending deletions.
   * `backlogit list --type shipment --status active` → confirm `154-S` is the sole active.

   **Bootstrap (single generic status move — the only mutation):**
   * `backlogit move 154-S --status blocked` (generic artifact status path; sets ONLY the status
     token; no cascade, so member statuses/branch/checkpoint are preserved by construction).

   **Post-verification:**
   * `backlogit shipment get 154-S` → `status: blocked`; member statuses, branch, and checkpoint
     UNCHANGED versus the pre-verification snapshot.
   * `backlogit list --type shipment --status active` → `154-S` ABSENT (active slot freed for
     `155-S` at the backlogit layer).
   * Record the **migration debt** explicitly: `blocked_reason`/`blocked_at`/`blocked_by`/
     `resume_checkpoint_ref` ABSENT and no governed `shipment_status_changed` event — to be
     backfilled by `155-S`/U18 before any unblock (SBLK-R25).

   **Rollback (if anything is wrong or the external gate rejects it):**
   * `backlogit move 154-S --status active` restores the prior status token (again status-only,
     non-destructive). Because the bootstrap never touched members/branch/checkpoint, rollback
     is a clean single-field revert. No data loss on either edge.

   **Migration-debt discharge:** `154-S` MUST NOT be unblocked while the governed metadata is
   absent. Once `155-S` ships U2c/U3/U4 and the U18 normalizer, run the normalizer to backfill
   `blocked_reason` (= `RED_DELIVERABLE_DELTA_OUT_OF_SURFACE` / `7AA35A39`), `blocked_at`,
   `blocked_by`, `resume_checkpoint_ref` (= `checkpoint-20260914-070735.json`) and emit the
   governed event; only then is `154-S` eligible for governed `unblock --to active` (U12 refuses
   unblock while debt is outstanding — SBLK-R25).

4. **Autoharness topology-gate assessment (SBLK-R26).** The bootstrap frees *backlogit's own*
   status-keyed active-slot scan, but the **external autoharness pipeline-topology gate**
   (out-of-repo, numeric-predecessor ordering) is a separate authority. It is **not established**
   that it interprets the new `blocked` token as non-active:
   * **If** the external gate keys off backlogit `status == active`, a `blocked` `154-S` is
     excluded and `155-S` can proceed.
   * **If** it keys off queue position / predecessor completion, a `blocked` `154-S` may still
     occupy a predecessor slot and continue to gate `155-S`, or may not recognize `blocked` at
     all and fail closed.
   * **Recommendation:** before relying on the bootstrap to admit `155-S`, independently verify
     the autoharness gate's treatment of `blocked`. If it does not recognize `blocked` as
     non-active, a **separate external compatibility gate / configuration** is required. Until
     verified, treat admission as an **unconfirmed assumption** and fail closed. This is an
     external, out-of-repo dependency and is explicitly NOT implemented in backlogit
     (consistent with the §2.5 non-goal on the autoharness gate).

---

## 7. Scope boundary

In-repo backlogit shipment lifecycle + CLI/MCP/doctor/queue/index + tests + operator
docs. No autoharness edit. No modification of `154-S` by Stage (the bootstrap in §6 is an
operator-owned recommendation, not executed here). No shipment claim, no Ship
invocation, no application source/test writes by Stage. The change is additive and
backward-compatible (SBLK-R18); the bootstrap seam (SBLK-R24) is temporary and
non-destructive (status-only, reversible).
