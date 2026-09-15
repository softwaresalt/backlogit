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

## 0. CURRENT AUTHORITATIVE REVISION — rev2 (2026-09-15)

> §0 is the **authoritative current decision**; the detailed sections below are **retained as
> historical deliberation and audit trail**. The core decision is unchanged (introduce a
> canonical, non-terminal, resumable shipment `blocked` status; it supersedes S12 `parked`);
> rev2 records the concise delivery shape, the **verified** external-topology result, and the
> **bootstrap source-of-truth handoff**.

* **Decision (unchanged):** Option A — a single canonical shipment lifecycle status `blocked`,
  non-terminal, excluded from the single *active* slot, governed transitions
  `active → blocked` and `blocked → {queued, active}`, correlated intent→commit events, member
  snapshot/disposition on block and exact restore on unblock. Shared portable contract vs.
  backlogit-local implementation are separated in spec §0.1.
* **Decomposition (revised):** the prior 29-task set (`174.001-T…174.029-T`) is **superseded**
  and parked at `blocked` (preserved, not deleted); replaced by **9 test-first ≤2h tasks**
  `174.030-T…174.038-T` (spec §0.2) under feature `174-F` / shipment `155-S`.
* **External topology (VERIFIED, §6 item 4):** the current `autoharness gate
  pipeline-topology` **rejects** a `blocked` token (`_VALID_LIVE_SHIPMENT_STATUSES` excludes
  `blocked`, `topology.py:553`), while its active-slot and consistency logic already treat
  `blocked` as non-active. Smallest fix is a one-line upstream allowlist change (external, not
  backlogit Go); interim admission via an audited operator-only `--force` override for `155-S`.
* **Bootstrap source-of-truth (§6 item 5):** snapshot `154-S` from the **intact Ship branch
  `…precondition` @ `dd9f01a1`** (authoritative active provenance), not the staging projection;
  single-worktree branch-switch handoff, dedicated `chore/bootstrap-154` branch off post-merge
  main, no parallel worktrees, no loss of Ship commits. Operator-owned; not executed by Stage.

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

3. **One-time, operator-approved, PRE-GOVERNANCE bootstrap for `154-S` (resolves the rollout
   circularity; SBLK-R24/R25/R26).** There is a genuine **circularity**: the governed
   `BlockShipment` seam (U2c) with member disposition (U6) is the correct way to block `154-S`,
   but it does not exist until `155-S` ships — and `155-S` cannot run until the single active
   slot is free, which requires `154-S` to be blocked first. Without a controlled exception,
   rollout is impossible. The earlier blanket prohibition on the generic-move bootstrap was
   correct about its two defects but left the circularity unresolved. This deliberation resolves
   it with an **explicitly exceptional, one-time, operator-approved pre-governance bootstrap
   runbook** — NOT the final API, and NOT the naive single generic move. It manually reproduces
   the two things the governed seam would guarantee (member disposition + a bounded rollback),
   and is **removed after migration**. Stage does NOT execute it (operator-owned; `154-S` must
   not be edited by Stage).

   Local code already supports the primitive (verified 2026-09-14: `models.StatusBlocked` at
   `internal/models/artifact.go:17`; generic `move` → `core.UpdateArtifactWithGate` at
   `internal/cli/move.go:62`; `list --type shipment --status blocked` works). The runbook the
   operator may run, in order:

   **Pre-verification / snapshot (capture baseline):**
   * `backlogit shipment get 154-S` → confirm `status: active`; record covering feature `173-F`,
     the **exact status of every member** (esp. active `173.006-T`), branch
     `feat/shipment-claim-scheduler-baseline-marker-enabling-precondition`, and checkpoint
     `checkpoint-20260914-070735.json`. Persist this **member snapshot** durably (memory/audit).
   * `backlogit list --type shipment --status active` → confirm `154-S` is the sole active.

   **Bootstrap (manual member disposition FIRST, then status move):**
   * Move the active member to `queued`: `backlogit move 173.006-T --status queued` (removes the
     in-flight release-unit execution so P-001 is no longer contended — this is the manual
     equivalent of the governed member disposition).
   * Move the shipment token to blocked: `backlogit move 154-S --status blocked` (existing
     generic move; members already dispositioned, branch/checkpoint untouched).
   * Append a **durable comment/memory audit** naming the **migration debt**: the record now
     lacks governed `blocked_reason`/`blocked_at`/`blocked_by`/`resume_checkpoint_ref` and the
     governed `shipment_status_changed` event, to be backfilled by the U18a/U18b normalizer.

   **Post-verification:**
   * No member of `154-S` remains `active` (esp. `173.006-T` now `queued`); `154-S` `status:
     blocked`; branch + checkpoint intact vs. the snapshot; `list --status active` → `154-S`
     ABSENT and the slot genuinely free.
   * Run the **autoharness topology pre-claim check for `155-S`** (item 4 / SBLK-R26) before
     relying on the freed slot.

   **Bounded rollback (permitted ONLY before any other claim):**
   * Rollback is valid **only if no other shipment has become active in the interim**. First
     verify `list --type shipment --status active` is empty; then `backlogit move 154-S --status
     active` and **restore every member to its exact snapshot status** (e.g. `173.006-T` back to
     its recorded pre-block status). If any other shipment is already active, rollback would
     create a second active and MUST **fail closed** (rollback refused; forward-fix only).

   **Debt discharge:** after `155-S` ships U2c/U3 and the U18a/U18b normalizer, run the governed
   normalizer to backfill the canonical metadata and emit the governed event; the unblock
   readiness gate (U12) refuses any unblock until this is done (SBLK-R25).

   This bootstrap is **exceptional, operator-approved, one-time, and removed after migration**;
   it is never presented as the shared contract or the steady-state API. The governed
   `BlockShipment`/`UnblockShipment` seam remains the only sanctioned path once the capability
   ships; the generic-move primitive survives only as the normalizer's internal remediation
   target for out-of-band `blocked` records.

4. **Autoharness topology-gate assessment (SBLK-R26) — VERIFIED 2026-09-15.** Direct
   inspection of the installed external gate (`autoharness/gates/topology.py`) now RESOLVES
   the previously-open question:
   * **The current external gate REJECTS a `blocked` shipment token.** Queue-folder shipment
     parsing at `topology.py:553` raises `BacklogUnavailableError("missing or unsupported
     status")` for any status outside `_VALID_LIVE_SHIPMENT_STATUSES = {queued, active,
     shipped, abandoned}`; `blocked` is absent, so a `blocked` `154-S` fails the gate closed
     **before** any active-slot classification runs.
   * **Design intent already agrees `blocked` is non-active** once admitted: `_active_shipments`
     counts only `live_status == "active"`, and `_detect_before_consistency`
     (`_NOT_YET_CLAIMED_STATUSES = {queued, blocked}`, `topology.py:33/1643`) raises
     `SHIPMENT_STATE_INCONSISTENT` when a `blocked` shipment still has an `active`/`done` member.
     The latter is **external corroboration** that the bootstrap MUST disposition `173.006-T`
     to `queued` before/with blocking `154-S`.
   * **Smallest compatibility change:** add `"blocked"` to `_VALID_LIVE_SHIPMENT_STATUSES`
     (one line) **upstream in external autoharness** `topology.py`. This is out-of-repo Python,
     NOT backlogit Go source, so backlogit does not implement it; it is recorded as an
     **external informing/ratification item** for the autoharness maintainers.
   * **Until upstream ratifies:** admit `155-S` using an **operator-only, audited temporary
     override** — `autoharness gate pipeline-topology --force` scoped to phases
     `pre_claim`/`post_claim`/`lifecycle` for the `155-S` bootstrap window only, recorded in the
     bootstrap audit. No standing override; removed after upstream adds `blocked` to the
     allowlist. This preserves the fail-closed default for every other shipment.

5. **Bootstrap source-of-truth & single-worktree handoff (154-S) — authoritative,
   operator-owned, NOT executed by Stage.** The staging branch `chore/stage-155` (based on
   `origin/main`) projects `154-S` and member `173.006-T` as `queued`, but the **intact Ship
   branch `feat/shipment-claim-scheduler-baseline-marker-enabling-precondition` @ `dd9f01a1`**
   carries the authoritative *active* provenance and checkpoint
   (`checkpoint-20260914-070735.json`, `blocker_token: RED_DELIVERABLE_DELTA_OUT_OF_SURFACE`).
   The bootstrap snapshot MUST be taken from the **Ship-branch truth**, not the staging
   projection. The handoff uses ONE worktree, creates NO parallel worktrees, and loses NO Ship
   commits:

   1. **Land staging first.** Merge `chore/stage-155` → `main` via the normal PR (Orchestrator
      owns Step 1.5). Do not proceed until merged.
   2. **Capture the machine-readable snapshot from Ship-branch truth.** In the single worktree,
      `git switch feat/shipment-claim-scheduler-baseline-marker-enabling-precondition` (a plain
      branch switch — non-destructive, no rebase/reset, Ship commits untouched). Read the
      authoritative state and write a **machine-readable snapshot file** (e.g.
      `.backlogit/bootstrap/154-S.snapshot.json`) recording: `154-S` status/manifest, every
      member ID and its exact status (esp. `173.006-T = active`), the Ship branch name + tip SHA
      `dd9f01a1`, and the checkpoint ref. No free-form memory evidence — the snapshot file is the
      sole source of truth consumed by `174.036-T` (R6).
   3. **Return to a dedicated bootstrap branch based on post-merge main.**
      `git switch main && git pull` then `git switch -c chore/bootstrap-154` (based on the merged
      main that now contains the shipped `blocked` capability). The Ship branch is left exactly
      as-is at `dd9f01a1`.
   4. **Commit the blocked state on the bootstrap branch.** Using the SHIPPED governed
      `BlockShipment` (post-`155-S`), or — during the pre-governance window only — the audited
      one-time runbook in item 3, apply member disposition (`173.006-T → queued`) FIRST, set
      `154-S → blocked`, backfill governed metadata/event (or record migration debt), and commit
      **only** the `154-S`/snapshot state onto `chore/bootstrap-154`. Never commit onto or
      rewrite the Ship branch.
   5. **Verify + bounded rollback.** As in item 3: confirm no member remains `active`, `154-S`
      is `blocked`, the Ship branch + checkpoint are intact vs. the snapshot file, and the active
      slot is free; run the topology pre-claim check (with the audited `--force` override until
      upstream ratifies). Bounded rollback is valid ONLY before any other claim; else fail closed.

   Stage authors this procedure but does **not** execute it, does not switch branches for the
   bootstrap, and does not edit `154-S` or the Ship branch.

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
