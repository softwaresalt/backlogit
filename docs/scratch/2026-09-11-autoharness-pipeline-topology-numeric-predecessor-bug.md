---
chunk_strategy: h1-h2-h3
description: "autoharness cross-gate contract inconsistency: dag-readiness reports 148-S/149-S ready while pipeline-topology pre_claim blocks them via the intentional implicit numeric-adjacency predecessor fallback; the two gates give contradictory readiness answers with no reconciled source of truth"
doc_type: guide
docline:
  author: Stage
  date: 2026-09-11
  status: draft-for-transfer
ingested_at: "2026-09-11T00:00:00Z"
schema_version: "1.0"
source: docs/scratch/2026-09-11-autoharness-pipeline-topology-numeric-predecessor-bug.md
title: "autoharness cross-gate inconsistency — dag-readiness and pipeline-topology pre_claim disagree on implicit vs. explicit predecessor sequencing"
---

# autoharness cross-gate inconsistency — `dag-readiness` and `pipeline-topology pre_claim` disagree on implicit vs. explicit predecessor sequencing

> **Transfer note & lifecycle.** This document was authored in the `backlogit`
> workspace but describes a defect that belongs to the **separate `autoharness`
> workspace** (the `autoharness` gate binary). It is written to be self-contained
> so it can be copied verbatim into the `autoharness` workspace as a bug report /
> issue. No fix is applied in `backlogit`; `backlogit` carries only this
> documentation and a tracking stash entry (`6D53A33F`) that references this path.
>
> **This file lives under `docs/scratch/`, which is intentionally ephemeral.**
> Per `.github/instructions/context-efficiency.instructions.md`, scratch files
> may be archived/compacted once they age past the retention window, so this path
> is **not** durable and may stop resolving. That placement is deliberate: this is
> a **temporary transfer artifact** whose only job is to be **manually copied into
> the `autoharness` workspace** before the next scratch compaction/archive cycle.
> **Do not treat this path as durable provenance, and do not rely on it surviving.**
> Durable provenance is preserved independently of this file:
>
> - **PR #438** in `softwaresalt/backlogit` carries this document and the stash
>   entry in version history (recoverable even after scratch compaction).
> - The authoritative **source-workspace** analysis lives at
>   `docs/decisions/2026-09-06-queued-shipment-ordered-scope-decision.md`
>   (section 4, "Topology gate: numeric-predecessor behavior", and follow-up 7a),
>   which remains the durable `backlogit`-side record.
>
> If you are reading this after transfer, copy it into a durable `autoharness`
> location; do not link back to this `docs/scratch/` path as a stable reference.

## 1. Summary

Two sibling gates in the `autoharness` binary return **contradictory
claim-readiness answers** for the same shipment, with **no exposed or reconciled
source of truth** for which sequencing model governs:

- `dag-readiness` reads the explicit shipment **`blocks` DAG** and reports
  `148-S` (and `149-S`) in its `ready_set`.
- `pipeline-topology --phase pre_claim` reports the *same* shipments as
  **`blocked` with `PREDECESSOR_NOT_SHIPPED`**, because it applies an **implicit
  numeric-adjacency predecessor fallback** (nearest lower-numbered shipment) when
  no explicit `blocks` predecessor is declared.

**The implicit numeric-adjacency fallback is an intentional current `autoharness`
safety contract, not an accidental bug.** It is preserved on purpose — see
upstream commit `14c32ef879fd7d67a8ac6b0dfd55dad056ba34d2`, which *retains* the
numeric-adjacency implicit-predecessor fallback (while fixing a separate
fail-open suppression bug), and `tests/test_gates_topology.py::ImplicitNumericPredecessorTests`,
which documents the heuristic as intentional: it catches an
**unstated-but-intended sequential predecessor** when no shipment declares
dependencies. This report therefore does **not** assert that numeric-predecessor
logic is itself a defect, and does **not** mandate DAG-only resolution as settled
contract.

**The actionable defect is the cross-gate contract inconsistency / ambiguous
source of truth.** For a shipment that is a DAG root but has a lower-numbered
neighbor, `dag-readiness` (explicit-DAG model) and `pre_claim` (implicit-fallback
model) disagree, and neither gate exposes *which* model is authoritative or
reconciles the two. Operators and agents cannot get one coherent claim-readiness
answer from the gate suite.

## 2. Environment

- Component: `autoharness` gate binary (`pipeline-topology`, `dag-readiness`).
- Surface: `autoharness gate ...` CLI, `--mode agent`.
- Backlog source: a `backlogit` workspace whose shipment artifacts carry explicit
  `blocks` dependency edges.
- **Upstream evidence that the numeric fallback is intentional (verified):**
  - Commit `14c32ef879fd7d67a8ac6b0dfd55dad056ba34d2` intentionally **preserves**
    the numeric-adjacency implicit-predecessor fallback and fixes a fail-open
    suppression bug (it hardens, rather than removes, the heuristic).
  - `tests/test_gates_topology.py::ImplicitNumericPredecessorTests` asserts the
    heuristic is intentional — it exists to catch an unstated-but-intended
    sequential predecessor when **no** shipment declares dependencies.
- Reproduction below uses a placeholder `<WORKSPACE>` for the backlog workspace
  path; substitute the local checkout path. No machine-specific or secret values
  are required to reproduce.

## 3. Reproduction

Assume a backlog whose shipment `blocks` DAG has two independent numeric-adjacent
roots (neither blocks the other), for example `148-S` and `149-S`, plus a linear
track ending at `147-S`:

```text
137-S (SHIPPED) -> 138-S -> 139-S -> ... -> 147-S      (linear track)
148-S            (independent ROOT — no explicit predecessor)
149-S            (independent ROOT — no explicit predecessor)
   (148-S, 149-S) -> 150-S
   (148-S, 149-S) -> 151-S
```

> **Workspace binding.** `backlogit dep list` and `pipeline-topology` are
> **cwd-relative** — they inspect the workspace of the current directory. Only
> `dag-readiness` accepts an explicit `--workspace`. So the portable `sh` blocks
> below bind the workspace explicitly with `cd <WORKSPACE>` before every
> cwd-relative command, and pass `--workspace <WORKSPACE>` to `dag-readiness`.
> Without this, the same commands copied into another checkout would silently
> inspect the wrong workspace.

### 3.1 Confirm the true dependency edges (backlogit)

```sh
cd <WORKSPACE>

# Forward edges — what 148-S depends on. Expect NONE (148-S is a DAG root).
backlogit dep list 148-S

# Reverse edges — what depends on 148-S. Expect only 150-S and 151-S.
backlogit dep list 148-S --reverse
```

Observed:

- `backlogit dep list 148-S` (forward) → **no incoming/forward dependencies**
  for `148-S` (empty result): it has zero explicit `blocks` predecessors.
- `backlogit dep list 148-S --reverse` →

  ```text
  150-S → 148-S (blocks)
  151-S → 148-S (blocks)
  ```

  only `150-S` and `151-S` depend on `148-S`.

### 3.2 Confirm the explicit-DAG ready-set (autoharness dag-readiness)

```sh
autoharness gate dag-readiness --workspace <WORKSPACE> --json
```

Observed: `ready_set` includes `148-S` (e.g. `[141-S, 148-S, 149-S, 152-S]`);
`148-S` is reported ready under the explicit-DAG model.

### 3.3 Run the contradicting gate (autoharness pipeline-topology pre_claim)

```sh
cd <WORKSPACE>

autoharness gate pipeline-topology --mode agent --shipment 148-S --phase pre_claim --json
```

## 4. Observed vs. expected

### Observed

- `pre_claim 148-S` → `blocked: true` with `PREDECESSOR_NOT_SHIPPED`, selecting
  predecessor **`147-S`** via the implicit numeric-adjacency fallback. `147-S` is
  *not* an explicit `blocks` predecessor of `148-S` (which has zero explicit
  predecessors); it is the numerically-adjacent ID that the intentional heuristic
  substitutes when no dependency is declared.
- `pre_claim 149-S` → `blocked: true`, selecting predecessor **`148-S`** — again
  the numeric-adjacency fallback, not an explicit edge.
- `pre_claim 138-S` → `blocked: false`, because its numeric neighbor `137-S`
  happens to be shipped (the implicit fallback coincidentally matches reality).
- `dag-readiness` reports `148-S`/`149-S` **ready** at the same time.

The observed *behavior of each gate individually* is consistent with its own
model. The problem is that the **two gates model sequencing differently and
disagree**, and nothing reconciles them.

### Expected (one coherent contract — direction is an upstream decision, see §7)

- The gate suite must return **one coherent claim-readiness answer** per shipment.
  A shipment reported `ready` by `dag-readiness` must not simultaneously be
  `pre_claim`-blocked with no exposed rationale linking the two models.
- Whichever sequencing model is authoritative (explicit-DAG-only, or
  explicit-DAG-plus-intentional-implicit-fallback) must be **applied consistently
  by both gates** and **surfaced in the gate output**, so an operator can tell
  *why* a root with a lower-numbered neighbor is or is not claimable.

This report deliberately does **not** assert the "expected" result is
`blocked: false`. If the implicit fallback is retained as the authoritative
contract (§7 Option A), then `148-S` being held behind `147-S` is *intended*, and
`dag-readiness` is the gate that must be reconciled to report the same implicit
predecessor. Which gate changes is the upstream decision in §7.

## 5. Impact

- **Ambiguous source of truth for claim decisions.** `dag-readiness` says ready;
  `pre_claim` says blocked. Neither gate declares which sequencing model wins, so
  operators and agents cannot derive a single trustworthy claim-readiness answer
  and are pushed toward manual overrides or guesswork.
- **Sequencing intent is invisible.** If the implicit numeric fallback is the
  intended safety contract (catching an unstated-but-intended predecessor), that
  intent is not exposed by `dag-readiness`, so a DAG-independent root looks freely
  claimable in one gate and serialized in the other with no explanation.
- **Delivery latency risk from an unreconciled contract.** Whichever direction is
  correct, today a critical, plan-ready, DAG-independent root such as `148-S` can
  be held behind a numerically-lower track while a *different* gate simultaneously
  reports it ready — and downstream convergent work (`150-S`/`151-S`) inherits the
  ambiguity. This is a program-latency and trust risk, not evidence that either
  gate's internal logic is broken.

## 6. Likely contract boundary (where the reconciliation belongs)

- The inconsistency is between two gates in the **`autoharness` gate core**: the
  `pre_claim` predecessor-derivation step (explicit `blocks` edges **plus** the
  intentional implicit numeric-adjacency fallback) and the `dag-readiness`
  ready-set computation (explicit `blocks` edges **only**).
- The reconciliation therefore belongs in `autoharness`: define a **single
  predecessor/sequencing model** and have **both** gates consume it (ideally a
  shared code path), plus expose in the gate output which model produced the
  answer. The specific direction is the §7 decision.
- **No `backlogit` change is required or appropriate.** The dependency data is
  already correct and consistent in `backlogit` (`dep list` and the explicit-DAG
  `dag-readiness` agree). No `backlogit`-side lever (dependency edges, priority,
  or `queue_position`) can change a predecessor that the intentional heuristic
  derives from the shipment ID itself — this must be reconciled inside
  `autoharness`.

## 7. Required upstream decision & acceptance criteria

**This is a follow-up requiring an upstream decision, not a settled bug fix.**
Upstream must choose **one** authoritative sequencing model and make both gates
agree. The intentional numeric fallback (commit `14c32ef` +
`ImplicitNumericPredecessorTests`) must **not** be deleted as though it were a
proven accidental bug.

### Decision required (choose exactly one)

- **Option A — Retain implicit sequencing; reconcile `dag-readiness` to it.**
  Keep the intentional numeric-adjacency implicit predecessor in `pre_claim` and
  make `dag-readiness` **model and report the same implicit predecessors**, so a
  root with an unshipped lower-numbered neighbor is reported *not* ready by both
  gates (with the implicit predecessor named). Preserves the existing safety
  guarantee; changes `dag-readiness`.
- **Option B — Adopt explicit-DAG-only readiness; deliberately migrate off the
  fallback.** Make `pre_claim` derive predecessors **exclusively** from the
  explicit `blocks` DAG (matching `dag-readiness`), and **deliberately remove or
  migrate** the numeric-adjacency fallback. This is a **contract change**: it must
  update `ImplicitNumericPredecessorTests` (and any peer fixtures) to reflect the
  new contract, document the removed safety behavior, and provide a migration path
  for backlogs that relied on implicit sequencing (e.g. require explicit `blocks`
  edges where implicit adjacency previously served).

### Acceptance criteria (apply to whichever option is chosen)

1. **One sequencing model, both gates.** `pipeline-topology --phase pre_claim` and
   `dag-readiness` resolve predecessors from the **same** model (ideally a shared,
   provably-equivalent code path) so they cannot diverge again.
2. **Agreement invariant.** For every shipment, `pre_claim` blocked-state and
   `dag-readiness` ready-set membership are **mutually consistent**: a shipment in
   the `ready_set` is not `pre_claim`-blocked, and vice-versa, under the chosen
   model.
3. **Model is exposed.** Gate output states which predecessor(s) drove the
   decision and whether each is an **explicit `blocks` edge** or an **implicit
   numeric-adjacency** predecessor, so the answer is auditable.
4. **Regression coverage for the reconciled contract.** Cover the independent
   numeric-adjacent roots fixture (`148-S`/`149-S`: numerically adjacent, both DAG
   roots, neither an explicit predecessor of the other), a convergence case
   (`150-S`/`151-S` depending on both roots), and a linear-track case, asserting
   **both gates return the same claim-readiness** for each.
5. **If Option B: contract-change hygiene.** `ImplicitNumericPredecessorTests` and
   any dependent fixtures/docs are updated to the new explicit-DAG-only contract,
   the removed safety behavior is documented, and backward-compatibility/migration
   implications for implicit-sequencing backlogs are stated.
6. **If Option A: reconciliation hygiene.** `dag-readiness` regression tests are
   added asserting it reports the same implicit predecessors as `pre_claim`, and
   its output documents the implicit-predecessor semantics.

### Preferred recommendation (proposal only — not existing contract)

**Proposed (not authoritative):** prefer **Option A** *unless* the program has
already committed to explicit `blocks` edges everywhere. Rationale: the numeric
fallback is a deliberately-retained safety net for backlogs that omit explicit
edges (per `14c32ef` and `ImplicitNumericPredecessorTests`), and reconciling
`dag-readiness` to it is lower-blast-radius than removing a guarantee other
consumers may depend on. If upstream instead standardizes on explicit `blocks`
edges as the sole sequencing source, Option B is preferable — but only as a
deliberate, tested, documented contract change. **This preference is a proposal
for the upstream owner to decide; it is not the current `autoharness` contract.**

## 8. Safe interim workaround (until the contract is reconciled)

- The only interim path to claim a DAG-root that the implicit fallback holds
  behind a numeric neighbor is an **operator-authorized, audited** `pre_claim`
  override (e.g. `--force`).
- **This override must be explicitly authorized by a human operator for a
  specific, named shipment, and recorded in the audit trail. It must NEVER be
  applied automatically, blanket-enabled, or issued by an agent on its own
  authority.**
- The override is a stopgap for a single known-good claim, not a substitute for
  reconciling the gates. Each use should reference this inconsistency and the
  `dag-readiness` evidence showing the shipment is ready under the explicit-DAG
  model.

## 9. Supporting evidence

- **Upstream (autoharness), verified — fallback is intentional:**
  - Commit `14c32ef879fd7d67a8ac6b0dfd55dad056ba34d2` preserves the
    numeric-adjacency implicit-predecessor fallback (and fixes a separate
    fail-open suppression bug).
  - `tests/test_gates_topology.py::ImplicitNumericPredecessorTests` documents the
    heuristic as intentional (catches an unstated-but-intended sequential
    predecessor when no shipment declares dependencies).
- **Backlogit-side, read-only:**
  - `backlogit dep list 148-S` (forward): no incoming dependencies;
    `backlogit dep list 148-S --reverse`: only `150-S` and `151-S` depend on
    `148-S`.
  - `autoharness gate dag-readiness --workspace <WORKSPACE> --json`: `ready_set`
    contains `148-S` (ready under explicit-DAG model).
  - `autoharness gate pipeline-topology --mode agent --shipment 148-S --phase pre_claim --json`:
    `blocked: true`, `predecessor_id: 147-S` (implicit numeric-adjacency fallback,
    not an explicit `blocks` edge).
- **Non-portable source-workspace reference (does not resolve after transfer).**
  A prior analysis exists in the *source* `backlogit` workspace at
  `docs/decisions/2026-09-06-queued-shipment-ordered-scope-decision.md`
  (section 4 "Topology gate: numeric-predecessor behavior" and follow-up 7a).
  This path is internal to the `backlogit` workspace and will **not** resolve in
  the `autoharness` workspace; it is cited for provenance only. This report is
  self-contained — all evidence needed to understand and reconcile the
  inconsistency is in sections 1–8 above and does not depend on that document.
