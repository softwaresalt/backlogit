---
title: "Deliberation: Enforcing condition (b) for multi-member shipments before the 154-S marker"
doc_type: "decision"
source: "docs/decisions/2026-09-28-513e62ab-condition-b-enforcement-deliberation.md"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
description: "Stage decision on deferred stash 513E62AB. It covers which queued multi-member shipments get a blocks edge onto 154-S and where the shipped-provenance and marker-consumption pre-claim check lives."
topic: "Condition (b) pre-marker scheduling discipline: DAG edges and the pre-claim check"
depth: "standard"
decision_status: "decided (operator, 2026-09-28)"
promoted_to: "none (no harvest; edges recorded; check placement routed to existing items AF1E5075 and 6434A4D7)"
linked_artifacts:
  - "docs/decisions/2026-09-20-shipment-claim-wave-scheduler-convergence-deliberation.md"
  - "docs/memory/2026-09-27/orchestrator-pr452-merge-and-153-s-halt-memory.md"
  - "docs/compound/2026-07-20-ship-gate-descoped-archived-member-exemption.md"
source_stash_id: "513E62AB"
source_refs:
  pr: "#453"
  review_threads:
    - "PRRT_kwDORzozKM6mYTlg"
    - "PRRT_kwDORzozKM6mYTlo"
    - "PRRT_kwDORzozKM6mYVKX"
    - "PRRT_kwDORzozKM6mYVKv"
    - "PRRT_kwDORzozKM6mYXaA"
    - "PRRT_kwDORzozKM6mYXaI"
  shipment: "154-S"
  feature: "N/A"
---

## Problem Frame

Decision `2026-09-20-shipment-claim-wave-scheduler-convergence-deliberation`
(lines 318-334) states condition (b) of the bootstrap resolution. No unrelated
multi-member shipment may be claimed in the pre-marker window. The window stays
open until two things are true: `154-S` has shipped the scheduler-baseline
marker, and the external autoharness P-002.6 scheduler consumes that marker.
`core.ClaimShipment` activates every manifest member, and Ship wave admission
halts on any active member. A claim in the window therefore ends in
`WAVE_NO_PROGRESS`.

Condition (b) was written only as prose. `153-S` got a `blocks` edge onto
`154-S` in PR #453. Six other queued multi-member shipments have no
unfinished predecessor, so the queue dependency filter treats them as
claimable.

The deliberation answers two questions:

1. Should each of `141-S`, `152-S`, `147-S`, `177-S`, `178-S`, and `179-S` get
   a `blocks` edge onto `154-S`, or a documented concrete exemption?
2. Where should the consumption-aware, shipped-provenance pre-claim check live?
   The candidates are Orchestrator routing, `backlogit` claim, and the
   autoharness `pipeline-topology` gate.

## Evidence (main `131577c1`, index synced 2026-09-28)

* **Complete target set.** An index query of every queued, active, or blocked
  shipment shows that exactly six multi-member shipments have no unfinished
  blocking predecessor:

  | Shipment | Members | Current blocking predecessor | Title |
  |---|---|---|---|
  | `141-S` | 7 | `140-S` (archived) | S7 — API-backed evidence and documentation validator |
  | `147-S` | 4 | none | S13 — Harness and documentation hygiene |
  | `152-S` | 7 | none | Checkpoint create schema-reject + legacy-import opt-in |
  | `177-S` | 4 | `155-S` (archived, shipped) | Governed test budget E1 |
  | `178-S` | 5 | `155-S` (archived, shipped) | Governed test budget E2 |
  | `179-S` | 5 | `155-S` (archived, shipped) | Governed test budget E5 |

  Every other queued shipment is already held, directly or through a chain:
  `153-S` and `176-S` depend on `154-S`; `142-S` through `145-S` hang off
  `141-S`; `180-S` and `181-S` hang off `179-S`; `149-S` through `151-S`,
  `156-S`, and `157-S` through `169-S` hang off `176-S`. Adding edges on the
  six roots therefore puts every queued multi-member shipment behind `154-S`.
* **Cycle safety.** `154-S` depends only on `155-S`, which is archived with
  `archived_status: shipped`. None of the six is upstream of `154-S`, so the
  new edges cannot form a cycle.
* **Engine semantics.** A `blocks` edge only holds a shipment back. The queue
  filter releases the edge when `154-S` reaches any of six terminal statuses:
  done, accepted, archived, shipped, abandoned, or rejected. `ClaimShipment`
  does not check dependencies at claim time (`6434A4D7`). The edges alone
  therefore cannot distinguish a shipped `154-S` from an abandoned one, and
  they cannot see marker consumption.
* **Prior art (compound library).**
  `2026-07-20-ship-gate-descoped-archived-member-exemption.md` shows that the
  SQLite index does not project `archived_status`, and that loading is
  index-first. A provenance check that reads the index, including
  `backlogit_query_sql` or an index-backed `get_item`, cannot see
  `archived_status: shipped`. The check has to read the Markdown source
  (`.backlogit/archive/154-S.md` once archived), and it must fail closed when
  provenance is missing. `2026-07-17-backlogit-update-drops-archive-provenance.md`
  adds that `backlogit update` on an archived record drops
  `archived_status`. Nothing may run `update` on the archived `154-S` record
  before the check reads it.

## Q1 — Edges or exemptions

### Options

* **E1: add `blocks` edges from all six onto `154-S` (no exemptions).**
* **E2: add edges for some and record exemptions for the rest.** An exemption
  would need a concrete reason that the shipment can execute in the pre-marker
  window without `WAVE_NO_PROGRESS`.
* **E3: add no edges and rely on prose plus Orchestrator discipline.** This is
  the current state.

### Exemption analysis

An exemption must name a concrete mechanism that avoids the halt. None applies
to these six:

* Each shipment has three or more task members. `ClaimShipment` activates all
  of them, and Ship wave admission halts on any active member. The shipment's
  internal wave shape does not matter.
* The only technique that can run a claim-activated shipment without the
  marker is Model M2, the operator-supervised direct bootstrap. The 2026-09-20
  decision limits it to `{155-S, 154-S}` and says it "does not extend" to any
  other shipment. For `154-S` it was UNAPPROVED at triage
  (`bootstrap-bypass-unapproved`). The operator later approved it with
  conditions, for `154-S` only (see OQ-4), so it still covers none of the six.
* No shipment in the set is a marker predecessor. The marker producer's
  predecessor closure is `{155-S, 154-S}` and ends there.

### Decision (Q1): E1

Stage adds `blocks` edges from `141-S`, `147-S`, `152-S`, `177-S`, `178-S`,
and `179-S` onto `154-S`, and grants no exemptions. The change is within Stage
authority:

* Each edge only holds a shipment back. No edge releases anything.
* No existing edge is removed.
* The edge change leaves `154-S` itself unchanged: its status and dependencies
  stay the same. (The later waiver record changed only its labels and banner;
  see OQ-4.)
* No shipment is claimed, routed, unblocked, or bootstrapped.

The edges also mean the Orchestrator's existing Step 2 check, "no unshipped
blocking predecessor", now catches all six. Before this change it could
not.

**Limit of the edges.** This hold is temporary. It works only while `154-S` is
unshipped. The queue filter releases the edges as soon as `154-S` reaches any
terminal status, including `abandoned`. Once `154-S` ships, the edges release
while (C), marker consumption, is still unchecked. Nothing in the current
Orchestrator or Ship contract picks up the check at that point (see Q2).

An operator can later exempt a shipment by removing its edge. That requires a
recorded concrete mechanism that avoids the pre-marker halt. Stage does not
remove edges on its own.

## Q2 — Where the pre-claim check lives

The check has two parts:

* **(P) Shipped provenance.** `154-S` is `status: shipped`, or it is
  `status: archived` with `archived_status: shipped`, read from Markdown
  source. Any other terminal status does not count.
* **(C) Consumption.** The external autoharness scheduler is consuming the
  marker.

### Options

| Option | Can enforce (P) | Can enforce (C) | Enforcement strength | Owner / surface | Blockers |
|---|---|---|---|---|---|
| **L1 Orchestrator routing** (Step 2 pre-claim re-check) | Yes, if it reads source provenance | Yes, if it has a machine-checkable signal | Governance only; a direct Ship claim can bypass it. Not implemented today: current Step 2 checks only for an unshipped blocking predecessor and optionally runs `pipeline-topology` | Harness agent contract (`_orchestrator.agent.md`, `_ship.agent.md` pre-claim), rendered by autoharness | Same surface as `AF1E5075`; Stage cannot edit agent contracts |
| **L2 backlogit claim** (`ClaimShipment` claim-time guard) | Yes, in code, for all callers | No: backlogit has no knowledge of the external scheduler | Code-enforced and cannot be bypassed | `internal/core` (Go) | Already scoped by active stash `6434A4D7` items (1) and (2); touches `ClaimShipment`, as the `154-S` / `173-F` marker work does (`C29EBEE5` re-validation) |
| **L3 autoharness `pipeline-topology` gate** (`--phase pre_claim`) | Yes | Yes: it belongs to the scheduler that consumes the marker | Deterministic gate; the current Orchestrator Step 2 runs it only optionally before claim | External autoharness workspace (P-017) | Predecessor derivation is currently wrong (`A592FC1C`, numeric adjacency); out-of-workspace |

### Decision (Q2): layered target design; no backlogit code harvest this session

**Actual enforcement state today.** No agent contract implements a check for
(P) or (C). The current Orchestrator Step 2 only checks for an unshipped
blocking predecessor and optionally runs `pipeline-topology`. It has no
`154-S` source-provenance read, no marker-consumption check, and no
`CONDITION_B_UNSATISFIED` path. So condition (b) is enforced today only by:

* the `blocks` edges onto `154-S`, which hold only while `154-S` is unshipped
  (see "Limit of the edges" under Q1); and
* a **manual** operator/Orchestrator policy: do not route anything behind
  `154-S` unless (P) and (C) both hold, as the Orchestrator did in the
  2026-09-27 halt.

The manual policy stays in force until `AF1E5075` delivers the contract change
below. It is not an automated or fail-closed guard.

The target design has three layers:

1. **L1, Orchestrator routing (target; not implemented).** Only this surface
   can combine (P) and (C), and it runs before every claim. Once implemented,
   for any shipment whose `blocks`-edge closure includes `154-S`, the
   Orchestrator and Ship pre-claim must:
   * read `154-S` provenance from Markdown source, not the index, and fail
     closed when it is missing;
   * require (P);
   * require (C) through the consumption signal decided in OQ-2;
   * halt with `CONDITION_B_UNSATISFIED` when either part fails.

   Writing this into the Orchestrator and Ship contracts is a harness-contract
   change. Stage does not make it. Per OQ-3 it is folded into the `AF1E5075`
   release unit, which changes the same Orchestrator Step 2 and Ship pre-claim
   surface. A note recording this was appended to stash `AF1E5075`.
2. **Code enforcement of (P): L2 through `6434A4D7`.** The shipped-only,
   claim-time dependency guard is already captured and prioritized in
   `6434A4D7`. A new Stage harvest here would duplicate it. It must be
   sequenced after `154-S`, or be re-validated with it, because both change
   `ClaimShipment` (see `C29EBEE5`). The guard must read `archived_status`
   from Markdown source, following the compound learning above. It is not
   bounded enough to harvest in this session. A cross-reference note was
   appended to stash `6434A4D7`.
3. **Long-term home of (C): L3.** Once `A592FC1C` is fixed, the
   `pipeline-topology` pre_claim gate is the right deterministic owner of the
   consumption half. At that point L1 reduces to "run the gate and honor it".
   This is external autoharness work (P-017), and Stage records it as a
   follow-up only.

### Why not a single location

* L2 alone cannot see (C).
* L3 alone is external and currently mis-derives predecessors.
* L1 alone is governance-only and can be bypassed.

The layers do not duplicate each other: L2 will own (P) in code, L3 will own
(C) deterministically, and L1 will connect the two. Until `AF1E5075` lands,
the stopgap is the manual policy above, not L1.

## Operator decisions (recorded 2026-09-28)

The operator accepted the remaining recommended decisions on 2026-09-28
("And with that, I think we should move forward with the remaining
recommended decisions.", 19:42 -07:00).

* **OQ-1 (exemptions): DECIDED — no exemptions.** All six keep their `blocks`
  edge onto `154-S`. Stage found no concrete mechanism that avoids the halt.
* **OQ-2 (consumption signal): DECIDED.**
  * Interim: (c) an explicit operator attestation, recorded as a comment on
    `154-S` after it ships, for example the operator confirming that the
    external autoharness scheduler is consuming the marker.
  * Long-term: (a) the `pipeline-topology` pre_claim gate, once `A592FC1C` is
    fixed.
  * Rejected: (b) a declared scheduler capability in
    `.autoharness/harness-manifest.yaml`. It asserts capability, not
    consumption.

  Until the attestation exists, nothing behind `154-S` is routed.
* **OQ-3 (fold vs separate): DECIDED — fold into `AF1E5075`.** The L1 contract
  text is not a separate release unit.
* **OQ-4 (`154-S` waiver outcome): APPROVED WITH CONDITIONS.** The operator
  approved the `154-S` bootstrap waiver:
  * it covers `154-S` only;
  * stash `C29EBEE5` must pass first (re-check the `173-F` tasks against the
    current claim code);
  * the bootstrap is supervised, with nothing else active;
  * it skips only the active-residual halt; local review, CI, Copilot review,
    and merge-commit rules all still apply, with no admin bypass.

  Stage recorded the approval as an operator comment on `154-S` and in its
  banner, and replaced the label `bootstrap-bypass-unapproved` with
  `bootstrap-bypass-approved-conditional`. The hold label
  `do-not-claim-until-convergence` stays until a later Stage session records
  `C29EBEE5` PASS. The waiver does not extend to any of the six shipments
  above.

## Next Steps

1. Done: add the six hold-only `blocks` edges onto `154-S`.
2. Orchestrator (manual policy): do not route `141-S`, `147-S`, `152-S`,
   `153-S`, `176-S`, `177-S`, `178-S`, or `179-S`, or anything downstream of
   them, until `154-S` has shipped provenance (read from Markdown source) and
   the operator attestation comment on `154-S` exists. This must be applied by
   hand even after the edges release.
3. Stage, in a later session: run the `C29EBEE5` re-validation. On PASS,
   remove `do-not-claim-until-convergence` from `154-S` and route it to Ship
   under the conditional waiver.
4. Stage, in a later session when the operator selects it: deliberate
   `AF1E5075` with the L1 contract text from this decision folded in.
5. Stage, in a later session: deliberate `6434A4D7`, sequenced after `154-S`
   and `C29EBEE5`.
6. External: fix `A592FC1C`, then move (C) into the `pipeline-topology`
   pre_claim gate.
