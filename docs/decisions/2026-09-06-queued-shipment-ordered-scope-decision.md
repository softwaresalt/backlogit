---
chunk_strategy: h1-h2-h3
description: "Stage ordered-scope decision over queued shipments 138-S…151-S: DAG, plan-readiness, FF6D467A resolution, and topology numeric-predecessor gate treatment"
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-09-06-queued-shipment-ordered-scope-decision.md
title: "Ordered-scope decision — queued shipments 138-S … 151-S (fault-line + reconciliation + trust-boundary)"
date: 2026-09-06
author: Stage
status: authoritative
mode: staging-scoping (NOT dark factory; NOT Ship execution)
---

# Ordered-scope decision — queued shipment set 138-S … 151-S

Stage-owned ordered scoping exercise over the complete queued shipment set. This
artifact determines the correct execution sequence, resolves plan-readiness
shipment-by-shipment, resolves the applicability of stash `FF6D467A`, and states
the explicit treatment of the installed topology gate's numeric-predecessor
behavior.

**Boundary note.** This is a planning/decomposition decision only. No production
code was written, no shipment was claimed, and no PR was merged. Routing any
shipment to Ship requires fresh explicit operator approval.

---

## 1. Evidence base (all read-only, this session)

* Index synced (`backlogit sync`, 1377 artifacts). Workspace `.backlogit`.
* Explicit dependency (`blocks`) edges read via `backlogit dep list` for every
  queued shipment.
* True DAG readiness read via `autoharness gate dag-readiness --json`.
* Numeric-predecessor behavior observed via
  `autoharness gate pipeline-topology --shipment <id> --phase pre_claim --json`.
* Final `## Plan Review` verdict read from each covering plan
  (`docs/exec-plans/2026-09-03-s*-plan.md`, plus the 148-S and 866FDC8C plans).
* Prior Stage memory: `docs/memory/2026-09-06-stage-866fdc8c-148s-restage-reassess.md`.
* Stash `FF6D467A` (re-plan driver) read via `backlogit stash get`.

---

## 2. Explicit dependency DAG (from backlogit `blocks` edges)

```
137-S (SHIPPED)
  └─▶ 138-S ─▶ 139-S ─▶ 140-S ─▶ 141-S ─▶ 142-S ─▶ 143-S ─▶ 144-S ─▶ 145-S ─▶ 146-S ─▶ 147-S      (Track A: fault-line S4→S13, strict linear)

148-S  (Track B: reconciliation #423 — ROOT, no predecessor)
149-S  (Track C: trust-boundary — ROOT, no predecessor)
   └─(148-S,149-S) ─▶ 150-S
   └─(148-S,149-S) ─▶ 151-S
```

`autoharness gate dag-readiness` confirms the true ready-set:

```
ready_set      = [138-S, 148-S, 149-S]
critical_path  = 137-S → 138-S → … → 147-S   (Track A)
```

**Finding D-1 — the dependency graph is already architecturally correct.**
There are no missing, incorrect, or redundant edges. 148-S and 149-S are genuine
independent roots; 150-S/151-S correctly converge on both. No dependency-edge
mutation is warranted or made.

**Finding D-2 — hard vs. historical serialization inside Track A.** Track A is a
strict linear chain. The evidence-framework core (S4→…→S10 / 138→…→144) is a
genuine architectural layering: S10 (144-S, "fault-line evidence DAG, seq 7/7")
aggregates the evidence produced by S4–S9, so those edges are hard. The tail
S11→S12→S13 (145→146→147: policy engine → lifecycle parked-state → docs/harness
hygiene) is **more loosely coupled** — S13 hygiene is docs-only and does not hard-
depend on S12 implementation. That tail is partly historical/numeric sequencing.
This is noted but **not re-wired**, because (a) under P-001 single-active-shipment
these serialize regardless, and (b) the numeric gate (§4) forces the order anyway,
so re-wiring the tail yields no achievable-order benefit and adds churn/risk.

---

## 3. Plan-readiness — shipment by shipment

Final `## Plan Review` section (last in document order) governs. Gate satisfied
only when `decision: PASS`, or `decision: ADVISORY` **with**
`operator_authorization: approved`.

| Ship | Feat | Seq | Plan (final review) | Hardening | Attempt | Plan-gate | Ready? |
|------|------|-----|---------------------|-----------|---------|-----------|--------|
| 138-S | 156-F | S4  | **PASS**     | yes | 3 | satisfied | ✅ READY (re-planned, PR #422) |
| 139-S | 157-F | S5  | **PASS**     | no  | 1 | satisfied | ✅ READY |
| 140-S | 158-F | S6  | **FAIL**     | no  | 2 | not satisfied | ❌ NEEDS RE-PLAN |
| 141-S | 159-F | S7  | **PASS**     | yes | 1 | satisfied | ✅ READY (plan) — blocked upstream by 140-S |
| 142-S | 160-F | S8  | **FAIL**     | yes | 2 | not satisfied | ❌ NEEDS RE-PLAN |
| 143-S | 161-F | S9  | **FAIL**     | yes | 2 | not satisfied | ❌ NEEDS RE-PLAN |
| 144-S | 162-F | S10 | **FAIL**     | yes | 3 | not satisfied | ⛔ NEEDS RE-PLAN + **operator intervention** (max re-entry cycles reached) |
| 145-S | 163-F | S11 | **PASS**     | yes | 1 | satisfied | ✅ READY (plan) — blocked upstream |
| 146-S | 164-F | S12 | **FAIL**     | yes | 2 | not satisfied | ❌ NEEDS RE-PLAN |
| 147-S | 165-F | S13 | **ADVISORY** | yes | 2 | **not satisfied** (no `operator_authorization`) | ⚠ NEEDS OPERATOR AUTHORIZATION (or re-plan) |
| 148-S | 167-F | #423 | **PASS**    | yes | 3 | satisfied | ✅ READY (sized, PR #426) |
| 149-S | 168-F | 866-A | **PASS**   | yes | 1 | satisfied | ✅ READY (PR #425) |
| 150-S | 169-F | 866-B | **PASS**   | yes | 1 | satisfied | ✅ READY (plan) — blocked by 148-S+149-S |
| 151-S | 170-F | 866-C | **PASS**   | yes | 1 | satisfied | ✅ READY (plan) — blocked by 148-S+149-S |

**Finding P-1 — the fault-line FAIL wall.** Track A is plan-ready only through
139-S. The next hop, 140-S (S6), FAILs plan-review. Therefore **nothing beyond
139-S on Track A can execute** — 141-S/145-S are plan-PASS but sit behind the
FAIL wall, and 147-S (the numeric unblocker of 148-S, see §4) can never ship until
140/142/143/144/146 are re-planned and 147's ADVISORY is authorized.

**Finding P-2 — 144-S/S10 is at attempt 3.** It has exhausted the 2 re-entry
cycles (`plan-review-attempt: 3`, FAIL). Per the plan-review cycle rule this
requires **operator intervention**, not another automatic re-plan.

---

## 4. Topology gate: numeric-predecessor behavior (the core constraint)

Two gates in the installed `autoharness` binary disagree:

| Gate | Basis | 148-S | 149-S |
|------|-------|-------|-------|
| `dag-readiness` | real `blocks` DAG | **ready** (in ready_set) | **ready** (in ready_set) |
| `pipeline-topology --phase pre_claim` | **numeric ID − 1** | **BLOCKED** — `PREDECESSOR_NOT_SHIPPED: predecessor 147-S` | **BLOCKED** — `PREDECESSOR_NOT_SHIPPED: predecessor 148-S` |

Observed verbatim:

* `pre_claim 148-S` → `blocked: true`, `predecessor_id: "147-S"` (147-S is **not**
  a `blocks` predecessor of 148-S — 148-S has zero predecessors).
* `pre_claim 149-S` → `blocked: true`, `predecessor_id: "148-S"` (likewise not a
  real edge).
* `pre_claim 138-S` → `blocked: false` (its numeric predecessor 137-S is shipped).

**Finding G-1.** The `pre_claim` gate derives "predecessor" from **numeric
shipment-ID adjacency**, not from the dependency graph. It therefore imposes an
effective total order `138 → 139 → … → 151` that contradicts the true DAG and
contradicts `dag-readiness`.

**Finding G-2 — not fixable via backlog operations.** Shipments carry no
`queue_position` field, and the predecessor is computed from the shipment ID
itself. No dependency edge, priority change, or reorder that Stage can perform
will change which shipment the gate treats as the numeric predecessor. The only
backlog-level lever would be renumbering shipment IDs — destructive, unsupported,
and out of scope. The numeric-predecessor logic lives inside the external
`autoharness` binary (`autoharness.exe`), i.e. production code outside this repo.

**Decision G — requirement #7 path.** Because the gate itself prevents the correct
order and changing it would require production-code work on `autoharness`, Stage
**does not work around it** (no `--force`, no ID renumbering, no edge fudging).
Stage records a narrowly-scoped follow-up (§7) and prepares only the
planning/backlog changes valid under the current gate.

---

## 5. Recommended execution order

### 5a. Architecturally-optimal order (what SHOULD happen if the gate honored the DAG)

Under P-001 (at most one active shipment) the three tracks must still serialize
into one sequence; optimality is about **interleaving**. Optimizing for
criticality, risk reduction, unblocking, and minimal rework:

| Pos | Ship | Why here |
|-----|------|----------|
| 1 | 138-S | Track A head, READY, pre_claim already passes |
| 2 | 139-S | READY; only remaining plan-ready Track A hop before the FAIL wall |
| 3 | **148-S** | **critical**, READY, DAG-independent, unblocks the security program (150/151) |
| 4 | 149-S | READY, DAG-independent, unblocks the security program (150/151) |
| 5 | 150-S | READY plan; deps 148-S+149-S now satisfied |
| 6 | 151-S | READY plan; deps 148-S+149-S now satisfied |
| 7+ | 140→147 | after S6/S8/S9/S10/S12 re-plan to PASS and S13 authorized; then 141,145 unblock |

Rationale for the key promotions: 148-S is the only **critical**-priority queued
shipment, is fully sized and plan-PASS, has **no** architectural dependency on the
fault-line program, and is a hard predecessor of the entire trust-boundary
security program (150-S/151-S). Holding it — and 149-S — behind a fault-line chain
that is itself stalled at a FAIL wall maximizes both delay and rework risk.

### 5b. Gate-constrained achievable order (what CAN happen under the installed gate today)

The numeric `pre_claim` gate forces strict numeric order and the FAIL wall stops
Track A at 140-S:

```
138-S  ✅ claimable now (plan PASS, pre_claim PASS)
139-S  ✅ claimable after 138-S ships
140-S  ⛔ STALL — plan FAIL (S6). Numeric gate also blocks everything ≥141 behind it.
```

Under the installed gate, **critical 148-S is unreachable** until 140/142/143/144/146
are re-planned to PASS, 147's ADVISORY is authorized, and 141→147 all ship. This
is the central operator decision (§8).

---

## 6. Readiness / blocker table (consolidated)

| Ship | Plan | pre_claim (numeric gate) | DAG | Net status | Blocker |
|------|------|--------------------------|-----|------------|---------|
| 138-S | PASS | PASS | ready | **NEXT ELIGIBLE** | none — awaiting operator route-to-Ship |
| 139-S | PASS | needs 138 shipped | ready after 138 | eligible after 138 | 138-S |
| 140-S | FAIL | needs 139 shipped | blocked | **re-plan (S6)** | plan FAIL + 139 |
| 141-S | PASS | needs 140 shipped | blocked | plan-ready, gated | 140-S wall |
| 142-S | FAIL | needs 141 shipped | blocked | **re-plan (S8)** | plan FAIL |
| 143-S | FAIL | needs 142 shipped | blocked | **re-plan (S9)** | plan FAIL |
| 144-S | FAIL(a3) | needs 143 shipped | blocked | **re-plan + operator (S10)** | plan FAIL, cycles exhausted |
| 145-S | PASS | needs 144 shipped | blocked | plan-ready, gated | wall |
| 146-S | FAIL | needs 145 shipped | blocked | **re-plan (S12)** | plan FAIL |
| 147-S | ADVISORY | needs 146 shipped | blocked | **authorize or re-plan (S13)** | no operator_authorization + wall |
| 148-S | PASS | **BLOCKED by numeric 147** | **ready** | **critical, DAG-ready, gate-blocked** | topology gate G-1 |
| 149-S | PASS | **BLOCKED by numeric 148** | **ready** | DAG-ready, gate-blocked | topology gate G-1 |
| 150-S | PASS | needs 149 shipped | needs 148+149 | plan-ready, gated | 148-S,149-S |
| 151-S | PASS | needs 150 shipped* | needs 148+149 | plan-ready, gated | 148-S,149-S |

\* numeric gate makes 151's predecessor 150-S even though the true DAG only
requires 148-S+149-S — another G-1 numeric artifact.

**Restart cursor / next eligible shipment (under the installed gate): `138-S`.**
It is plan-PASS and `pre_claim` PASS. Routing it to Ship still requires fresh
operator approval (this session is not authorized to claim).

---

## 7. FF6D467A resolution (re-plan driver), and required changes

Stash `FF6D467A` (high, feature): re-author dark-factory plans
**S2/S3/S4/S6/S8/S9/S10/S12** for genuine plan-review P1s, then re-run plan-review.

Resolved against current state:

| Plan | Ship | FF6D467A-listed? | Current | Action |
|------|------|------------------|---------|--------|
| S2 | (136-S) | yes | **SHIPPED/done** | resolved — remove from scope |
| S3 | (137-S) | yes | **SHIPPED/done** | resolved — remove from scope |
| S4 | 138-S | yes | **PASS** (PR #422) | resolved — remove from scope |
| S6 | 140-S | yes | FAIL (a2) | **RE-PLAN** (decomposition, review-record integrity) |
| S8 | 142-S | yes | FAIL (a2) | **RE-PLAN** (soundness linter; spoofable waiver/TTY auth) |
| S9 | 143-S | yes | FAIL (a2) | **RE-PLAN** (baseline-overlay seam + workspace containment) |
| S10 | 144-S | yes | FAIL (a3) | **RE-PLAN + operator** (evidence forgery; max cycles reached) |
| S12 | 146-S | yes | FAIL (a2) | **RE-PLAN** (lifecycle parked-state) |

Additional gap **not** in FF6D467A: **S13 / 147-S** is ADVISORY without
`operator_authorization` → needs operator authorization or a re-plan pass. Because
147-S is the numeric unblocker of critical 148-S, closing this gap is on the
critical numeric path.

**Recommended stash actions (deferred to operator — NOT executed this session).**
The stash file `.backlogit/stash.jsonl` carries a pre-existing, preservation-flagged
line-ending-only modification. To avoid entangling that artifact, Stage did **not**
mutate the stash. On operator approval, apply:

1. Edit `FF6D467A` to the residual, precise scope:
   *"Re-author fault-line plans S6/140-S, S8/142-S, S9/143-S, S10/144-S, S12/146-S
   to PASS (S10 requires operator intervention — attempt 3), and resolve S13/147-S
   ADVISORY (authorize or re-plan). S2/S3/S4 resolved."*
2. Add a narrowly-scoped follow-up stash entry (see §7a).

### 7a. Narrowly-scoped follow-up (autoharness gate defect)

> **Title:** `autoharness pre_claim topology gate uses numeric shipment-ID
> predecessor instead of the blocks DAG — contradicts dag-readiness`
>
> **Body:** `pipeline-topology --phase pre_claim` blocks 148-S on "predecessor
> 147-S" and 149-S on "predecessor 148-S", but neither is a `blocks` predecessor;
> both are in `dag-readiness` ready_set. The pre_claim readiness check should
> derive predecessors from the shipment `blocks` DAG (as `dag-readiness` does),
> not numeric ID adjacency. Scope: `autoharness` gate core only; no backlogit
> change. Until fixed, DAG-independent shipments (148-S/149-S) cannot be claimed
> in optimal order without operator `--force`.` **kind:** bug · **priority:** high

This is an `autoharness` production-code change → outside this repo and outside
Stage's role boundary. Captured as a follow-up, not actioned.

---

## 8. Operator decision required

The installed numeric gate makes the architecturally-optimal order (§5a)
unreachable and leaves **critical 148-S + the trust-boundary security program
(149/150/151) blocked behind a stalled fault-line FAIL wall**. Choose one:

* **Option A — Re-plan the FAIL wall first (gate-compliant, no bypass).** Route
  138-S then 139-S to Ship; concurrently run the Stage re-plan pipeline for
  S6/S8/S9/S12 and operator-intervene on S10, and authorize/​re-plan S13. Only
  then does the numeric path reach 148-S. Highest latency to the critical item;
  zero gate risk. *(Stage-executable once approved.)*
* **Option B — Operator `--force` claim of 148-S ahead of the fault-line.**
  Operator-only, audited `pre_claim --force` to ship critical 148-S (then 149-S)
  before the fault-line completes. Delivers the critical item and unblocks the
  security program fastest. **Stage cannot perform this** (operator-only bypass).
* **Option C — Fix the autoharness pre_claim gate (§7a).** Make `pre_claim`
  DAG-based; then 148-S/149-S become claimable now per `dag-readiness`, and the
  optimal order (§5a) becomes gate-compliant. Correct long-term fix; requires
  `autoharness` production-code work outside this repo. **Out of Stage scope.**

**Stage recommendation:** pursue **C** as the durable fix (follow-up filed) and,
in the interim, **A** for gate-compliant forward progress — with **B** available
to the operator if 148-S criticality must be honored before the fault-line
re-plan completes. Re-planning S6/S8/S9/S10/S12 + resolving S13 is the real
critical-path work regardless of which option is chosen, because Track A cannot
advance past 139-S until that wall clears.

---

## 9. Changes made vs. proposed

**Made this session (additive, docs-only):**
* This decision artifact.
* Session memory (`docs/memory/2026-09-06-stage-queued-shipment-ordered-scope.md`).
* Structured checkpoint via `backlogit checkpoint create`.

**Explicitly NOT made (by design):**
* No dependency-edge changes — the DAG is already correct (D-1).
* No `queue_position`/priority reorder — cannot influence the numeric gate (G-2).
* No stash mutation — preserves the flagged `.backlogit/stash.jsonl` line-ending
  state; FF6D467A edit + follow-up entry deferred to operator (§7).
* No shipment claim, no Ship invocation, no PR merge, no production code.
* No `--force` gate bypass.

**Proposed (operator-gated):** §7 stash actions, §8 option selection, §7a
autoharness follow-up.
