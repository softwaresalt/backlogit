---
chunk_strategy: h1-h2-h3
description: "autoharness pipeline-topology pre_claim gate derives predecessors from numeric shipment-ID adjacency instead of the blocks DAG, contradicting dag-readiness and blocking DAG-independent roots such as 148-S/149-S"
doc_type: guide
schema_version: "1.0"
source: docs/scratch/2026-09-11-autoharness-pipeline-topology-numeric-predecessor-bug.md
title: "autoharness bug — pipeline-topology pre_claim uses numeric shipment-ID predecessor instead of the blocks DAG"
date: 2026-09-11
author: Stage
status: draft-for-transfer
---

# autoharness bug — `pipeline-topology pre_claim` uses numeric shipment-ID predecessor instead of the blocks DAG

> **Transfer note.** This document was authored in the `backlogit` workspace but
> describes a defect that belongs to the **separate `autoharness` workspace**
> (the `autoharness` gate binary). It is written to be self-contained so it can
> be copied verbatim into the `autoharness` workspace as a bug report / issue.
> No fix is applied in `backlogit`; `backlogit` carries only this documentation
> and a tracking stash entry that references this path.

## 1. Summary

The `autoharness` `pipeline-topology` gate, when run with `--phase pre_claim`,
derives a shipment's "predecessor" from **numeric shipment-ID adjacency**
(`ID − 1`) rather than from the explicit shipment **`blocks` dependency DAG**.
As a result it blocks shipments that are genuine dependency roots — reporting a
`PREDECESSOR_NOT_SHIPPED` on a shipment that is *not* an actual predecessor.

This directly contradicts the sibling gate `dag-readiness`, which correctly reads
the `blocks` DAG and reports the same shipments as ready. Two gates in the same
binary therefore disagree about claim-readiness for the same shipment.

## 2. Environment

- Component: `autoharness` gate binary (`pipeline-topology`, `dag-readiness`).
- Surface: `autoharness gate ...` CLI, `--mode agent`.
- Backlog source: a `backlogit` workspace whose shipment artifacts carry explicit
  `blocks` dependency edges.
- Reproduction below uses a placeholder `<WORKSPACE>` for the backlog workspace
  path; substitute the local checkout path. No machine-specific or secret values
  are required to reproduce.

## 3. Reproduction

Assume a backlog whose shipment `blocks` DAG has two independent numeric-adjacent
roots (neither blocks the other), for example `148-S` and `149-S`, plus a linear
track ending at `147-S`:

```text
137-S (SHIPPED) -> 138-S -> 139-S -> ... -> 147-S      (linear track)
148-S            (independent ROOT — no predecessor)
149-S            (independent ROOT — no predecessor)
   (148-S, 149-S) -> 150-S
   (148-S, 149-S) -> 151-S
```

### 3.1 Confirm the true dependency edges (backlogit)

```sh
# 148-S has no incoming blocks edges; only 150-S and 151-S depend on it.
backlogit dep list 148-S
```

Observed: no incoming dependencies for `148-S`; reverse edges show only `150-S`
and `151-S` depend on `148-S`.

### 3.2 Confirm the true ready-set (autoharness dag-readiness)

```sh
autoharness gate dag-readiness --workspace <WORKSPACE> --json
```

Observed: `ready_set` includes `148-S` (e.g. `[141-S, 148-S, 149-S, 152-S]`);
`148-S` is reported ready.

### 3.3 Run the failing gate (autoharness pipeline-topology pre_claim)

```sh
autoharness gate pipeline-topology --mode agent --shipment 148-S --phase pre_claim --json
```

## 4. Observed vs. expected

### Observed

- `pre_claim 148-S` → `blocked: true` with `PREDECESSOR_NOT_SHIPPED`, selecting
  predecessor **`147-S`**. `147-S` is *not* a `blocks` predecessor of `148-S`
  (which has zero predecessors); it is merely the numerically-adjacent ID.
- `pre_claim 149-S` → `blocked: true`, selecting predecessor **`148-S`** — again
  a numeric neighbor, not a real edge.
- `pre_claim 138-S` → `blocked: false`, because its numeric neighbor `137-S`
  happens to be shipped (the numeric guess coincidentally matches reality here).

### Expected

- `pre_claim 148-S` → `blocked: false`. `148-S` has no `blocks` predecessors, so
  it is claimable per the dependency DAG, consistent with `dag-readiness`.
- `pre_claim 149-S` → `blocked: false` for the same reason.
- When a shipment *does* have `blocks` predecessors, `pre_claim` should block iff
  at least one of those **DAG** predecessors is not yet shipped — never on a
  numeric-ID neighbor that is not an edge.

The two gates must agree: any shipment in the `dag-readiness` `ready_set` must not
be `pre_claim`-blocked on a fabricated numeric predecessor.

## 5. Impact

- **Correct pipeline order is unreachable through the gate.** The numeric rule
  imposes an effective total order (`138 → 139 → … → 151`) that contradicts the
  real DAG. DAG-independent roots (`148-S`, `149-S`) cannot be claimed in their
  correct, dependency-valid position.
- **Critical work is stranded behind unrelated tracks.** A critical, fully
  plan-ready, DAG-independent shipment is held behind a numerically-lower track
  that may itself be stalled — maximizing delivery latency and rework risk, and
  blocking any downstream program (e.g. `150-S`/`151-S`) that legitimately
  converges on those roots.
- **Gate disagreement erodes trust.** `dag-readiness` says ready; `pre_claim`
  says blocked. Operators cannot rely on the gate suite for claim decisions and
  are pushed toward manual overrides.

## 6. Likely contract boundary (where the fix belongs)

- The defect is inside the **`autoharness` `pipeline-topology` gate core**, in the
  `pre_claim` predecessor-derivation step. It currently computes the predecessor
  from the shipment **ID** (numeric adjacency) rather than querying the shipment
  **`blocks` dependency graph**.
- The correct source of truth is the same `blocks` DAG that `dag-readiness`
  already consumes. The fix is to make `pre_claim` resolve predecessors from that
  graph (ideally sharing the exact predecessor-resolution code path with
  `dag-readiness`) and drop the numeric-ID heuristic entirely.
- **No `backlogit` change is required or appropriate.** The dependency data is
  already correct in `backlogit` (`dep list` and `dag-readiness` agree); only the
  `autoharness` gate misreads it. Note also that no `backlogit`-side lever
  (dependency edges, priority, or `queue_position`) can influence a predecessor
  that is derived from the shipment ID itself — this must be fixed in
  `autoharness`.

## 7. Acceptance criteria

1. `pipeline-topology --phase pre_claim` derives predecessors **exclusively** from
   the explicit shipment `blocks` DAG — never from numeric shipment-ID adjacency.
2. For a shipment with **no** `blocks` predecessors (a DAG root), `pre_claim`
   returns `blocked: false` regardless of the numeric value of its ID or the
   shipped-state of any numerically-adjacent shipment.
3. For a shipment **with** `blocks` predecessors, `pre_claim` returns
   `blocked: true` iff at least one DAG predecessor is not yet shipped, and the
   reported `predecessor_id` is an actual `blocks` predecessor.
4. `pre_claim` and `dag-readiness` agree: every shipment in the `dag-readiness`
   `ready_set` is **not** `pre_claim`-blocked on a fabricated numeric predecessor.
5. **Regression tests** cover independent numeric-adjacent roots — specifically a
   fixture equivalent to `148-S`/`149-S` (numerically adjacent, but neither is a
   `blocks` predecessor of the other, and both are DAG roots) — asserting both are
   `pre_claim`-claimable. Include a convergence case (`150-S`/`151-S` depending on
   both roots) asserting they block until both real predecessors ship, and a
   linear-track case asserting numeric coincidence does not mask a missing edge.
6. The predecessor-resolution logic is shared (or provably equivalent) between
   `pre_claim` and `dag-readiness` so the two gates cannot diverge again.

## 8. Safe interim workaround (until the gate is fixed)

- The only interim path to claim a DAG-independent root that the numeric gate
  wrongly blocks is an **operator-authorized, audited** `pre_claim` override
  (e.g. `--force`).
- **This override must be explicitly authorized by a human operator for a
  specific, named shipment, and recorded in the audit trail. It must NEVER be
  applied automatically, blanket-enabled, or issued by an agent on its own
  authority.**
- The override is a stopgap for a single known-good claim, not a substitute for
  the fix. Each use should reference this defect and the `dag-readiness` evidence
  showing the shipment is genuinely ready.

## 9. Supporting evidence (backlogit-side, read-only)

- `backlogit dep list 148-S`: no incoming dependencies; reverse edges show only
  `150-S` and `151-S` depend on `148-S`.
- `autoharness gate dag-readiness --json`: `ready_set` contains `148-S` (ready).
- `autoharness gate pipeline-topology --phase pre_claim --shipment 148-S --json`:
  `blocked: true`, `predecessor_id: 147-S` (not a real `blocks` edge).
- Authoritative prior analysis in the `backlogit` workspace:
  `docs/decisions/2026-09-06-queued-shipment-ordered-scope-decision.md`
  (see section 4 "Topology gate: numeric-predecessor behavior" and follow-up 7a).
