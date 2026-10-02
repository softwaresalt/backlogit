---
chunk_strategy: h1-h2-h3
description: "Upstream hand-off for stash A592FC1C: the autoharness pipeline-topology pre_claim gate must derive shipment predecessors from the authoritative backlogit shipment blocks DAG instead of numeric ID adjacency; includes evidence, bounded upstream tests and acceptance, the 074-DL layer L3 role, and the partial supersession of the 6D53A33F disposition"
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-09-30-a592fc1c-pipeline-topology-dag-predecessor-upstream-handoff.md
title: "Upstream hand-off: pipeline-topology gate must derive predecessors from the shipment blocks DAG (A592FC1C)"
docline:
    stash_id: A592FC1C
    status: proposed
    created_at: 2026-09-30T17:20:00Z
---

## Status

* Owner of the fix: the external autoharness workspace (P-017 boundary).
* Local status: hand-off authored. Nothing is fixed. This document does not
  claim, and must not be read as claiming, that the upstream defect is fixed.
* Upstream delivery state: `pending-operator-delivery`. Delivery is an
  operator act through a channel the operator chooses. No agent writes to
  any external system: no agent opens an upstream issue, sends a message, or
  writes to the external autoharness filesystem. The backlogit task that
  records delivery only transcribes what the operator reports: the channel,
  the upstream reference, and who approved the delivery and when, or a dated
  reason when delivery has not happened.
* Local closure rule: backlogit treats the defect as corrected only after the
  installed autoharness version includes the upstream fix and a re-run of the
  T1 fixture shape below returns the expected result.
* Prior disposition superseded in part:
  `docs/decisions/2026-09-13-6d53a33f-autoharness-advisory-vs-preclaim-disposition.md`.
  That record classified numeric-adjacency predecessors as an advisory
  presentation concern. The evidence below shows a real scheduling deadlock.
  The rest of that record, including its workspace-containment determination,
  still stands.

## Defect

The `autoharness gate pipeline-topology --phase pre_claim` gate infers a
shipment's predecessor from numeric ID adjacency (`N-S` follows `(N-1)-S`). It
does not read the backlogit shipment `blocks` DAG. When the authoritative DAG
disagrees with numeric order, the gate blocks the wrong shipment.

## Evidence

* backlogit commit `69e900be` recorded `154-S` `blocks`-depends on `155-S`.
  `155-S` has no shipment predecessor, so it is a DAG root.
* The gate still raised `PREDECESSOR_NOT_SHIPPED` for `155-S`, naming `154-S`
  as its predecessor because 155 minus 1 is 154.
* The advisory `dag-readiness` reporter agreed with the DAG at the same time:
  `155-S` was in `ready_set` with `downstream=[154-S]`, and `154-S` was
  suppressed.
* The gate result was therefore inverted against the DAG. The authoritative
  root could not be claimed, and its dependent could not be claimed either.
* Live DAG today (backlogit index, 2026-09-30): `154-S` `blocks`-depends on
  `155-S` and `182-S`, and both are archived as shipped. Several queued
  shipments are not numerically adjacent to their real predecessors, for
  example `149-S` depends on `169-S`, and `177-S` depends on `154-S` and
  `155-S`.

## Requested Upstream Fix

Derive pre_claim predecessors only from backlogit `blocks` edges where both
endpoints are shipments (the `item_deps` rows with `dep_type = blocks`). This
is the same DAG that `dag-readiness` already consumes.

* No implicit numeric predecessor in the default path.
* When the DAG source cannot be read, return `invalid` (exit 2). Do not fall
  back to numeric adjacency, and do not fail open.
* Keep the existing meaning of "predecessor shipped" unchanged in this fix.
  Closure awareness (stash `F05661B1`) is a separate upstream item and is not
  part of this hand-off.
* Recommendation, not a requirement: replace or retire the upstream tests
  named in the prior disposition,
  `tests/test_gates_topology.py::ImplicitNumericPredecessorTests`, with the
  tests below.
* Recommendation, not a requirement: any back-compatibility switch for
  numeric adjacency is an upstream design choice. If one exists, backlogit
  recommends that it default to off.

## Bounded Upstream Tests and Acceptance

| # | Fixture | Expected pre_claim result |
|---|---|---|
| T1 | `155-S` root, `154-S` `blocks`-depends on `155-S`, both `queued` | `155-S`: no predecessor finding. `154-S`: `PREDECESSOR_NOT_SHIPPED` naming `155-S` |
| T2 | Adjacent IDs `10-S` and `11-S` with no `blocks` edge | `11-S`: no predecessor finding |
| T3 | `20-S` `blocks`-depends on `7-S` (not adjacent), `7-S` not shipped | `20-S`: `PREDECESSOR_NOT_SHIPPED` naming `7-S` |
| T4 | DAG source unreadable or missing | `invalid` (exit 2), never `pass` |

Acceptance: T1 to T4 pass upstream, the gate's other tokens are unchanged, and
a backlogit operator can re-run the gate against `155-S`/`154-S`-shaped
fixtures and observe T1.

## Role in 074-DL (Layer L3)

074-DL (`docs/decisions/2026-09-28-513e62ab-condition-b-enforcement-deliberation.md`)
splits condition (b) enforcement into three layers:

* L1, agent contracts (`AF1E5075`): governance-only (P)+(C) check.
* L2, backlogit core (`6434A4D7`): code guard for shipped provenance (P).
* L3, the `pipeline-topology` pre_claim gate: the long-term deterministic owner
  of scheduler consumption (C).

This fix is the precondition for L3. A gate that derives the wrong
predecessors cannot own (C). The fix alone does not transfer (C): a correct
predecessor derivation still does not check scheduler consumption. Adding a
consumption check to the gate is a later upstream decision and is not
requested here. The interim (C) signal, the operator attestation comment on
`154-S` decided in OQ-2, stays required until a recorded operator or Stage
decision confirms that the gate checks consumption.

## Not in This Hand-off

* No copy of autoharness sources or tests in backlogit.
* No change to backlogit Go code; backlogit already exposes the DAG.
* No closure-awareness change (`F05661B1`) and no other gate defect.
* No supersession notice for other numeric-gate records, such as the one
  tracked by stash `9A8E1879`.
