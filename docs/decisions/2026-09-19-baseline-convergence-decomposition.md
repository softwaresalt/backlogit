---
title: Baseline-Convergence Decomposition into Wave-Aligned Replacement Shipments
date: 2026-09-19
status: accepted
supersedes: Single-shipment packaging (156-S) of feature 175-F
provenance:
    - PR #448 (CLOSED WITHOUT MERGE) — historical
    - branch stage/baseline-convergence-main @ d4d394e1 — read-only evidence
    - docs/exec-plans/2026-09-17-baseline-convergence-plan.md — task DAG remains authoritative
references:
    - docs/decisions/2026-09-17-baseline-convergence-deliberation.md
    - docs/decisions/2026-09-17-baseline-convergence-authorization.md
    - docs/decisions/2026-09-17-baseline-lint-inventory.md
    - docs/exec-plans/2026-09-17-baseline-convergence-plan.md
---

# Baseline-Convergence Decomposition into Wave-Aligned Replacement Shipments

## Status

**Accepted.** This decision is the SOLE current execution recommendation for
delivering feature `175-F` (baseline lint convergence). It supersedes the
single-shipment packaging in `156-S`. PR #448 and `156-S` remain **historical
provenance only** and are preserved, not deleted.

## Context

PR #448 packaged the entire baseline-convergence scope — feature `175-F`, its 98
executable tasks, and their 184 dependency edges — into a single queued shipment
`156-S`. That shipment additionally listed `175-F` itself as a member, which under
backlogit shipment semantics expands scope to all descendants. The PR grew too
large for a bounded review/build/PR lifecycle and was **closed without merge**
(closing comment: PR #448 issuecomment-5746031335).

The operator decision is to **abandon the single PR, restage on a clean branch
based on `origin/main`, and decompose the same 98 tasks into multiple smaller,
dependency-ordered shipments** — without re-landing any Ship-authored
implementation as already-committed code, and without altering the underlying
task DAG.

The task DAG (98 members, 184 edges, 13 bounded waves) authored in
`docs/exec-plans/2026-09-17-baseline-convergence-plan.md` is unchanged and remains
authoritative. Only the **packaging** (shipment membership and shipment-level
sequencing) changes.

## Decision

Decompose `175-F` into **13 wave-aligned replacement shipments**, one per bounded
wave of the DAG, executed as a deterministic linear chain via `blocks`
dependencies. The DAG's longest-path layering yields exactly 13 waves — 1 source
boundary wave (U1), 11 remediation waves (U40 and U14 fold in), and 1 terminal
boundary wave (U12) — matching the plan's declared structure.

Each replacement shipment:

- lists **task IDs only** and MUST NOT include `175-F` as a member (avoiding the
  descendant-expansion anti-pattern that oversized `156-S`);
- contains one full wave (all `<= 10` executable tasks);
- `blocks`-depends on the immediately preceding wave's shipment.

The mechanical layering is provably clean: **every one of the 184 edges points
from a lower wave to a strictly higher wave** (no intra-wave and no backward
edges). Therefore the shipment-level chain `RS-W00 -> RS-W01 -> ... -> RS-W12`
fully and exactly captures all task-level ordering; no intra-shipment ordering
subtlety exists.

## Membership Map (task IDs only; `175-F` excluded from every manifest)

| Wave | Role | Tasks | Count |
|---|---|---|---|
| RS-W00 | Source boundary (U1 strict predecessor) | 175.001-T | 1 |
| RS-W01 | Remediation 1 (+U40) | 175.002-T, 175.003-T, 175.004-T, 175.005-T, 175.006-T, 175.007-T, 175.008-T, 175.009-T, 175.010-T, 175.040-T | 10 |
| RS-W02 | Remediation 2 (+U14) | 175.011-T, 175.013-T, 175.014-T, 175.015-T, 175.016-T, 175.017-T, 175.018-T, 175.019-T, 175.020-T, 175.021-T | 10 |
| RS-W03 | Remediation 3 | 175.022-T, 175.023-T, 175.024-T, 175.025-T, 175.026-T, 175.027-T, 175.028-T, 175.029-T, 175.030-T | 9 |
| RS-W04 | Remediation 4 | 175.031-T, 175.032-T, 175.033-T, 175.034-T, 175.035-T, 175.036-T, 175.037-T, 175.038-T, 175.039-T | 9 |
| RS-W05 | Remediation 5 | 175.041-T, 175.042-T, 175.045-T, 175.046-T, 175.047-T, 175.048-T, 175.049-T, 175.050-T, 175.051-T | 9 |
| RS-W06 | Remediation 6 | 175.043-T, 175.052-T, 175.053-T, 175.054-T, 175.055-T, 175.056-T, 175.057-T, 175.058-T, 175.059-T | 9 |
| RS-W07 | Remediation 7 | 175.044-T, 175.060-T, 175.061-T, 175.062-T, 175.063-T, 175.064-T, 175.065-T, 175.066-T, 175.067-T | 9 |
| RS-W08 | Remediation 8 | 175.068-T, 175.069-T, 175.070-T, 175.071-T, 175.072-T, 175.073-T, 175.074-T, 175.075-T, 175.076-T | 9 |
| RS-W09 | Remediation 9 | 175.077-T, 175.078-T, 175.079-T, 175.080-T, 175.081-T, 175.082-T, 175.083-T, 175.084-T, 175.085-T | 9 |
| RS-W10 | Remediation 10 | 175.086-T, 175.087-T, 175.088-T, 175.089-T, 175.090-T, 175.091-T, 175.092-T, 175.093-T, 175.094-T | 9 |
| RS-W11 | Remediation 11 (Linux surfaces) | 175.095-T, 175.096-T, 175.097-T, 175.098-T | 4 |
| RS-W12 | Terminal boundary (U12; unblocks 149-S) | 175.012-T | 1 |

**Total: 98 tasks, each in exactly one replacement shipment.**

Actual assigned shipment IDs (shipment ID counter is per-type, continued from 156-S):

| Wave | Assigned Shipment ID |
|---|---|
| RS-W00 | 157-S |
| RS-W01 | 158-S |
| RS-W02 | 159-S |
| RS-W03 | 160-S |
| RS-W04 | 161-S |
| RS-W05 | 162-S |
| RS-W06 | 163-S |
| RS-W07 | 164-S |
| RS-W08 | 165-S |
| RS-W09 | 166-S |
| RS-W10 | 167-S |
| RS-W11 | 168-S |
| RS-W12 | 169-S |

## Dependency / Ordering Map

- **Chain (deterministic sequence):** `RS-W(k+1)` `blocks`-depends on `RS-Wk` for
  k = 0..11. This yields the strict order RS-W00 → RS-W01 → ... → RS-W12.
- **149-S rewire:** the existing edge `149-S depends_on 156-S` is removed and
  replaced with `149-S depends_on RS-W12` (the terminal shipment carrying U12).
  U12 (`175.012-T`) is titled "terminal sink; unblocks 149-S", so gating 149-S on
  the terminal replacement shipment preserves the original semantics exactly:
  149-S cannot become eligible until the entire replacement sequence closes.
- **150-S / 151-S:** already `blocks`-depend on `149-S`; unchanged and therefore
  transitively gated behind the full replacement sequence.
- **168.001-T:** remains archived / `done`; it is not re-listed in any replacement
  shipment and is not re-executed.

## Supersession of 156-S (non-destructive)

The backlogit shipment lifecycle exposes no `abandon`/supersede verb in Stage
scope, and `move` does not route a shipment `abandoned` status safely. Per the
operator directive, `156-S` is therefore **left queued with an explicit
supersession record plus a hard dependency guard**, never deleted:

1. A supersession comment is appended to `156-S` naming the 13 replacement
   shipment IDs and marking it DO-NOT-CLAIM.
2. A hard guard edge `156-S depends_on RS-W12` is added so `156-S` can never
   become claim-eligible ahead of the replacement sequence; by the time RS-W12
   ships, all 98 member tasks are already `done` via the replacement shipments.
3. This decision record documents the supersession for the Orchestrator's claim
   routing.

`156-S` retains its historical 98-task membership as provenance; the exclusive
execution ownership of those tasks moves to the replacement shipments.

## What Is NOT Carried Forward

Ship-authored implementation from PR #448 is intentionally excluded from this
staging branch (it was never merged and is not treated as landed code):

- `.github/workflows/ci.yml` and other `.github` harness edits;
- `scripts/verify-*.ps1`, `scripts/lint-runtime.ps1`, and simulation tests;
- `AGENTS.md` / `.autoharness` harness manifest edits.

The still-required implementation those artifacts represent is owned by the
existing tasks (U1 line-ending root cause, U40/U14 guards, per-file remediation,
U12 terminal verification) and will be produced by Ship during execution of the
relevant boundary tasks. The exec-plan's references to the verifier commands are
retained as **design specification**, not as committed tooling.

## Invariants Preserved

- 98 unique executable tasks, covered exactly once across the 13 replacement
  shipments (no omission, no duplicate).
- No replacement shipment lists `175-F`.
- Every replacement shipment is `queued`.
- Dependency graph is acyclic and yields the declared ordered wave sequence.
- 149-S remains gated until the terminal replacement shipment (RS-W12) ships.
- 168.001-T remains archived / `done`.
- No source or workflow implementation files are modified on this branch.

## Plan Hardening

| # | Risk | Severity | Mitigation |
|---|---|---|---|
| H1 | Task dropped or duplicated across shipments | High | Mechanical validator (`logs/validate_decomp.py`) asserts exactly-once coverage of all 98 tasks; re-run post-assembly against live shipment manifests. |
| H2 | A dependency points backward/within a wave, breaking the shipment chain guarantee | High | Validator asserts all 184 edges are strictly forward-across-waves; confirmed PASS (0 intra-wave, 0 backward). |
| H3 | Backlog tool enforces single-shipment membership, blocking creation while 156-S holds the tasks | Medium | Contingency: if creation/add is rejected due to 156-S membership, clear 156-S membership via Stage file-authority edit + `sync` (non-destructive; 156-S retained), then retry. Empirically probed at assembly. |
| H4 | 149-S becomes eligible early | High | Rewire removes 149-S→156-S and adds 149-S→RS-W12; verify 149-S has no path to readiness until RS-W12 ships. |
| H5 | 156-S accidentally claimed after supersession | Medium | Supersession comment (DO-NOT-CLAIM) + guard edge 156-S→RS-W12 + this decision record. |
| H6 | Accidental inclusion of 175-F in a subset shipment (descendant expansion) | High | Manifests use task IDs only; validator/readback asserts no shipment lists 175-F. |
| H7 | Cycle introduced by chain + guard + rewire edges | High | Post-assembly acyclicity check over shipment-level edges. |
| H8 | Ship implementation leaks onto staging branch | Medium | `git diff origin/main` restricted to `.backlogit/` + `docs/`; assert zero source/workflow/test changes before commit. |

Requires plan hardening: no (hardening completed inline above).

## Plan Review

_dispatch_mode:_ multi-agent-dispatch
_reviewer:_ Correctness Reviewer (single focused cycle, per operator cost-control directive; no multi-model/adversarial escalation absent a new P0/P1)
_decision:_ PASS
_notes:_

Review confirmed A–E explicitly (independent trace against `_deps_new.json` +
`_taskids.json` + the wave map):
- **A** — membership map matches validator WAVES; all 98 tasks covered exactly once.
- **B** — all 184 edges strictly forward across waves; shipment chain fully captures ordering.
- **C** — U1 (175.001-T) unique source (indegree 0); U12 (175.012-T) unique sink (outdegree 0).
- **D** — 149-S rewire keeps 149-S gated to end-of-sequence; no early-eligibility path (given the old 149-S→156-S edge is deleted at assembly).
- **E** — chain + 149-S rewire + 156-S guard introduce no cycle (156-S becomes a pure leaf-dependent after rewire).

Non-blocking P3 advisories (accepted, dispositioned):
1. Validator covered only task-level guarantees → **addressed**: a shipment-level
   session validator was added and run in Step 9, asserting the chain edges, the
   149-S rewire, the 156-S guard, and shipment-level acyclicity from the live
   backlog (results recorded under "Validation Evidence").
2. WAVES dict hard-coded independently of the doc table → accepted; they match
   exactly and both are committed together for auditability.
3. 156-S retains historical membership including 175-F → accepted as
   non-destructive provenance; fully neutralized by the DO-NOT-CLAIM comment and
   the `156-S→RS-W12` guard edge (156-S can never become claim-eligible, and all
   members are `done` before RS-W12 ships).

No P0/P1 defects; single focused cycle sufficient.

<!-- plan-review-attempt: 1 -->
_operator_authorization: not required (decision == PASS)_

## Validation Evidence

Mechanically validated 2026-09-19 by two session validators
(`validate_decomp.py` task-level, `validate_shipments.py` shipment-level) run
against the live backlog index after assembly. The validators are session tooling
(not committed); their results are recorded durably here. Both **PASS**:

Task-level (`validate_decomp.py`):
- 98 backlog tasks, 98 assigned, 184 edges.
- Wave sizes: W0=1, W1=10, W2=10, W3..W10=9, W11=4, W12=1 (all ≤ 10).
- All 184 edges strictly forward-across-waves (0 intra-wave, 0 backward) → the
  shipment chain fully captures task ordering.
- Task DAG acyclic; U1 (`175.001-T`) unique source, U12 (`175.012-T`) unique sink.

Shipment-level (`validate_shipments.py`), invariants 1–6:
- **INV1** — 98/98 tasks covered, each in exactly one replacement shipment, 0 duplicates.
- **INV2** — NONE of 157-S..169-S lists `175-F`.
- **INV3** — all 13 replacement shipments `queued`.
- **INV4** — shipment DAG acyclic (28/28 nodes toposorted), all 12 chain edges
  present, RS-W00..RS-W12 order honored.
- **INV5** — `149-S depends_on 169-S` = true; `149-S depends_on 156-S` = false;
  `169-S` transitively behind all 12 predecessors (149-S cannot be eligible until
  the whole sequence closes).
- **INV6** — `168.001-T` status = `done`.

Invariant 7 (no source/workflow implementation changed) verified via
`git diff origin/main`: staged changes are confined to `.backlogit/` and `docs/`;
zero changes under `internal/`, `cmd/`, `scripts/`, `tests/`, `.github/`,
`AGENTS.md`, `.autoharness/`, `Makefile`, `go.mod`, `go.sum`.
