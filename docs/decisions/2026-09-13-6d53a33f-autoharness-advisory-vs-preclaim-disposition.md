---
chunk_strategy: h1-h2-h3
description: "Stage disposition for stash 6D53A33F — autoharness advisory DAG-readiness vs authoritative pre_claim semantics; cross-workspace boundary and non-blocking determination for the backlogit dark run"
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-09-13-6d53a33f-autoharness-advisory-vs-preclaim-disposition.md
title: "Disposition: 6D53A33F autoharness advisory-vs-pre_claim semantics (cross-workspace)"
docline:
    stash_id: 6D53A33F
    status: dispositioned
    created_at: 2026-09-13T17:31:00Z
---

## Disposition Record: 6D53A33F

**Kind**: bug (as stashed) → reclassified for routing as a cross-workspace
product/contract decision. **Priority**: high. **Route**: `deliberate`
(requires deliberation: true). **Outcome**: recorded disposition, **no in-repo
backlog item, no shipment**, non-blocking for this backlogit dark run.

### Problem Frame

`dag-readiness` (an autoharness advisory/visibility reporter) surfaces
`148-S`/`149-S` in its `ready_set` from the explicit `blocks` DAG. That output
is **advisory only** — it does not authorize a shipment claim. The **sole**
block/claim authority is `pipeline-topology --phase pre_claim`, which
intentionally blocks `148-S→147-S` and `149-S→148-S` via an **implicit
numeric-adjacency predecessor**. Per the stash provenance, that implicit
predecessor is *intentional* current autoharness safety behavior, preserved on
purpose by upstream commit `14c32ef879fd7d67a8ac6b0dfd55dad056ba34d2` and
covered by `tests/test_gates_topology.py::ImplicitNumericPredecessorTests`.

This is a **misleading-readiness-presentation** concern, NOT a competing-claim-
authority bug: `pre_claim` is authoritative; `dag-readiness` answers a different
(advisory) question. The two are not required to return identical claim answers.

### Product-Decision Input (recorded, not actioned here)

The operator has now explicitly stated that **strict numeric sequencing is
legacy and no longer valid**. This is recorded as an explicit product-decision
input to the **autoharness** workspace. It does NOT authorize removing the
implicit numeric predecessor from within this backlogit run, because:

* Removal is explicitly a **future contract change** requiring updates to
  `ImplicitNumericPredecessorTests` + affected tests/docs and a documented
  migration / back-compat path.
* The implementing code, tests, and gate presentation all live in the
  **separate autoharness workspace**, not in backlogit.

### Workspace-Containment Determination (NON-NEGOTIABLE)

The requested remedy — (a) label `dag-readiness` advisory output with its model
+ non-authorizing status [REQUIRED], (b) surface the authoritative `pre_claim`
outcome alongside the report [REQUIRED], (c) model implicit predecessors as a
separate field/view [OPTIONAL], (d) ensure `next_eligible` cannot be mistaken
for claim authorization [REQUIRED] — is entirely **gate-presentation work in
the autoharness workspace**. No backlogit data change is needed: the backlogit
dependency list and `dag-readiness` already agree; the mismatch is between an
autoharness advisory reporter and an autoharness authoritative gate.

Stage MUST NOT edit outside this repository (P-010 role boundary / P-017
workspace containment). Therefore Stage does **not** create a backlogit backlog
item or shipment for this work, and does **not** pretend the autoharness gate
was fixed here.

### Blocking Analysis for the backlogit dark run

**Does 6D53A33F block this backlogit run? NO — with one Ship-time claim caveat
noted below.**

* Stage-owned deliverables (triage, deliberation, planning, hardening, review,
  harvest, dependency wiring, shipment creation/update) require **no** change to
  autoharness gate presentation. They proceed unblocked.
* Caveat (Ship-time, not a Stage blocker): because the numeric-adjacency
  `pre_claim` predecessor remains **active and unpatched** in autoharness (the
  legacy-sequencing product decision is not yet implemented there, and
  implementing it is out of this workspace), any Ship-time attempt to claim a
  shipment **ahead of** its implicit numeric predecessor would be blocked at
  `pre_claim` unless the numeric predecessor first reaches the shipped terminal
  state. `pipeline-topology --force` is **NOT authorized** for this run and is
  operator-only/audited regardless. This is the direct cause of the 152-S
  claimability caveat recorded in the session summary; it is an autoharness
  gate reality, not a backlogit defect.

### Handoff

* Copy the durable provenance (this disposition + PR history) into the
  **autoharness** workspace backlog as the home for remedies (a)/(b)/(d)
  [required] and (c) [optional]. The temporary transfer artifact
  `docs/scratch/2026-09-11-autoharness-pipeline-topology-numeric-predecessor-bug.md`
  is NOT durable and must be manually carried over before scratch compaction.
* The superseded backlogit analysis
  `docs/decisions/2026-09-06-queued-shipment-ordered-scope-decision.md`
  (section 4 + follow-up 7a) is HISTORICAL and NOT authoritative for claim
  decisions.

### Stash Disposition

`6D53A33F` is archived from the active stash with a forward reference to this
disposition doc. No in-repo backlog artifact is created. Cross-workspace remedy
is handed to autoharness.

### Traceability

`6D53A33F`, autoharness `pipeline-topology --phase pre_claim`,
`ImplicitNumericPredecessorTests`, upstream commit
`14c32ef879fd7d67a8ac6b0dfd55dad056ba34d2`, PR #438 (softwaresalt/backlogit).
