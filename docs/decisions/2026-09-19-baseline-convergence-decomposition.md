---
title: Baseline-Convergence Decomposition into Wave-Aligned Replacement Shipments
date: 2026-09-19
status: accepted
doc_type: decision
source: docs/decisions/2026-09-19-baseline-convergence-decomposition.md
schema_version: "1.0"
chunk_strategy: h1-h2-h3
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
large for a bounded review/build/PR lifecycle and was **closed without merge**.

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

**Total: 98 remediation sub-DAG tasks, each in exactly one replacement shipment
(`157-S`..`169-S`).** Three dependency-ordered runner-bootstrap prerequisite tasks
`175.099-T -> 175.100-T -> 175.101-T` — the members of the prerequisite shipment
`176-S` (RS-W(-1)) — sit ahead of
this sequence, bringing the feature's executable-task total to **101** (98
remediation sub-DAG + 3 prerequisite). None of the three appears in any other shipment.
See "## Runner-bootstrap prerequisite" below.

Actual assigned shipment IDs (shipment ID counter is per-type, continued from 156-S):

| Wave | Assigned Shipment ID |
|---|---|
| RS-W(-1) | 176-S |
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

- **Runner-bootstrap prerequisite gate:** the entire
  replacement baseline sequence is gated behind the prerequisite shipment `176-S`
  (RS-W(-1), carrying the three tasks `175.099-T`, `175.100-T`, `175.101-T`) via
  `157-S depends_on 176-S`, with a
  reinforcing task-level edge `175.001-T depends_on 175.101-T` (the bootstrap sink).
  `RS-W00` (`157-S`)
  therefore cannot begin until `176-S` has shipped. The bootstrap tasks are
  dependency ordered `175.099-T -> 175.100-T -> 175.101-T`; `175.099-T` is the
  in-degree-zero source of the full-feature graph and `175.101-T` its sink; U1 remains
  the source of the
  98-member remediation sub-DAG.
- **Chain (deterministic sequence):** `RS-W(k+1)` `blocks`-depends on `RS-Wk` for
  k = 0..11. This yields the strict order RS-W00 → RS-W01 → ... → RS-W12.
- **149-S rewire:** the existing edge `149-S depends_on 156-S` is removed and
  replaced with `149-S depends_on RS-W12` (the terminal shipment carrying U12).
  U12 (`175.012-T`) is titled "terminal sink; unblocks 149-S", so ordering 149-S
  behind the terminal replacement shipment preserves the intended sequence. This
  edge is an ADVISORY ordering signal, not a hard shipped-only guarantee —
  backlogit queue filtering treats any no-longer-blocking terminal status of
  `169-S` (the 6-status cascade done/accepted/archived/shipped/abandoned/rejected)
  as satisfying it, and a direct claim bypasses dependencies entirely.
  Shipped-only readiness is governed by the Orchestrator claim-routing policy
  until tool-level enforcement lands (captured in stash `6434A4D7`); see
  `.backlogit/queue/149-S.md`.
- **150-S / 151-S:** already `blocks`-depend on `149-S`; unchanged and therefore
  transitively gated behind the full replacement sequence.
- **168.001-T:** remains archived / `done`; it is not re-listed in any replacement
  shipment and is not re-executed.

## Claim/Scheduler Execution Readiness (BLOCKED)

**This topology is structurally complete but not yet executable.** The
`runner-bootstrap` prerequisite gate above resolves the *lint-runner* bootstrap;
it does **not** resolve a second, independent blocker: the shipment-claim /
wave-scheduler contract mismatch. `core.ClaimShipment`
(`internal/core/shipment_lifecycle.go:46-98`) transitions **every** queued
manifest member to `active` on claim, while Ship wave admission
(`.github/agents/_ship.agent.md:526-535`) halts on **any** active member and
computes `ready_k` only from `queued` tasks. Claiming `176-S` makes all three
bootstrap tasks `active` and immediately yields `WAVE_NO_PROGRESS`; the same
applies to `157-S`..`169-S`. (Identical to the mismatch that blocked `140-S`.)

Execution readiness of the entire replacement sequence (`176-S` and
`157-S`..`169-S`) is therefore **BLOCKED** until the shipment-claim /
wave-scheduler convergence prerequisite lands. The authoritative claim model and
its rationale are recorded in
`docs/decisions/2026-09-20-shipment-claim-wave-scheduler-convergence-deliberation.md`
(deliberation over stash `6434A4D7`): `core.ClaimShipment` keeps its
all-members-active semantics; dependency-gated wave admission is realized
additively via the scheduler-baseline marker plus its scheduler consumption.

**Exact readiness dependencies:**

1. **`154-S` / `173-F`** — the in-repo scheduler-baseline **marker** enabling
   precondition (reviewed PASS). Encoded as the advisory edge `154-S blocks 176-S`
   (the front of the replacement chain waits for the marker). Advisory, not a hard
   shipped-only guard, until `6434A4D7` lands.
2. **External autoharness P-002.6 scheduler marker-consumption** (out-of-workspace,
   P-017) — must treat marked-active members as the wave-0 admissible baseline and
   reserve the active-residual halt for unmarked residuals. Cross-workspace
   follow-up; cannot be a backlog edge.
3. **`6434A4D7`** dependency-axis hardening (deferred Go-core release unit) —
   claim-time dependency guard + shipped-only readiness gate + governed
   non-claimable disposition. Governs, but does not gate, the baseline sequence.

**Bootstrap of the marker prerequisite itself.** `154-S`/`173-F` cannot be
executed through the marked-aware scheduler (the marker its own tasks produce
does not exist at its own claim time; `173-F` is internally multi-wave). It is
executed via an **operator-authorized single-shipment bootstrap**: Ship claims
`154-S`, then drives the claim-activated members green in their declared
dependency order **without** the strict active-residual halt, because the
just-claimed members are the intended working set. The exception is scoped to
`154-S` only; all later shipments use the normal marked-aware scheduler.

**Consequence for `#449`:** this PR is a planning-only decomposition; it makes no
shipment executable on its own. Until readiness dependencies (1) and (2) land, no
member shipment may be claimed for wave execution. The Orchestrator claim-routing
policy holds the sequence non-claimable in the interim.

## Runner-bootstrap prerequisite

All 98 baseline-remediation task contracts invoke `scripts/verify-task-lint.ps1`,
and the feature also requires `scripts/verify-baseline-lint.ps1` and
`scripts/verify-terminal-lint.ps1`, but none of those three runner scripts exists
on this branch and no `175.*` remediation task owned creating them. The first
replacement shipment `157-S` therefore had no executable task-lint command, and
folding the runners into `157-S` would exceed its single-task ownership.

**Resolution.** Three dependency-ordered runner-bootstrap prerequisite tasks and one
shipment are added under feature `175-F` (split from a prior oversized single bootstrap
task per PR #449 cycle 9 so each task owns at most two files and a bounded
at-most-three-scenario behavioral matrix):

| Item | ID | Role | Contents / ownership |
|---|---|---|---|
| Prerequisite task | `175.099-T` | `runner-bootstrap` | OWNS creation of `scripts/verify-task-lint.ps1` + owned harness `tests/runner_bootstrap_task_lint_175_099_test.go` |
| Prerequisite task | `175.100-T` | `runner-bootstrap` | OWNS creation of `scripts/verify-baseline-lint.ps1` + owned harness `tests/runner_bootstrap_baseline_lint_175_100_test.go`; depends on `175.099-T` |
| Prerequisite task | `175.101-T` | `runner-bootstrap` | OWNS creation of `scripts/verify-terminal-lint.ps1` + owned harness `tests/runner_bootstrap_terminal_lint_175_101_test.go`; depends on `175.100-T`; bootstrap sink |
| Prerequisite shipment | `176-S` (RS-W(-1)) | prerequisite | task-only manifest `[175.099-T, 175.100-T, 175.101-T]`; gates `157-S` |

**Explicit dependency edges added:**

- `154-S blocks 176-S` — the replacement chain's front (`176-S`) waits for the
  in-repo scheduler-baseline marker enabling precondition (`154-S`/`173-F`) so the
  wave scheduler can execute claim-activated members without `WAVE_NO_PROGRESS`.
  Advisory (not a hard shipped-only guard) until `6434A4D7` lands; see
  "## Claim/Scheduler Execution Readiness (BLOCKED)".
- `157-S depends_on 176-S` — the replacement baseline sequence cannot begin until
  the prerequisite shipment ships.
- `175.100-T depends_on 175.099-T` and `175.101-T depends_on 175.100-T` — the
  runner-bootstrap chain is dependency ordered (baseline runner after task runner,
  terminal runner after baseline runner).
- `175.001-T depends_on 175.101-T` — task-level reinforcement; U1 waits for the
  bootstrap sink (terminal-lint runner), transitively the whole runner bootstrap.

**Non-vacuous bootstrap contract (honest resolution).** Because each of `175.099-T`,
`175.100-T`, `175.101-T`
CREATES the runner it verifies, its task-lint GATE MUST NOT be a direct invocation of
that runner (using the created runner as its own gate would be a
circular self-dependency) and MUST be buildable before its runner exists. Each
therefore carries a self-contained task-lint contract of the `lint_scope` kind
`runner-bootstrap-self-contained`: an owned Go harness
(`tests/runner_bootstrap_task_lint_175_099_test.go` /
`tests/runner_bootstrap_baseline_lint_175_100_test.go` /
`tests/runner_bootstrap_terminal_lint_175_101_test.go`, functions
`TestU175_099_TaskLintRunnerBootstrap` / `TestU175_100_BaselineLintRunnerBootstrap` /
`TestU175_101_TerminalLintRunnerBootstrap`) that
(static layer) proves its runner script exists, is tracked, is non-empty,
parses cleanly under the built-in PowerShell AST parser, and declares its required
`param(...)` surface, and (behavioral layer, non-vacuous) — after the runner exists
— EXERCISES that runner against harness-authored deterministic fixture
workspaces/inputs to assert its success, native/nonzero failure-propagation, and
malformed/missing-input fail-closed contracts with divergent exit codes, so a no-op
runner that merely parses is rejected — RED before its runner exists, GREEN after —
using ONLY existing main-branch tooling (`go test` + `pwsh`), constructing all
fixtures itself and invoking runners via fixed argument vectors (no untrusted shell
input). No GATE makes its created runner its own lint gate, so each
prerequisite is independently buildable under existing tooling before its runner
exists. No Go source, workflow, or script is implemented in this planning PR; the
three tasks declare the ownership that Ship executes later. Full contracts:
`.backlogit/queue/175.099-T.md`, `.backlogit/queue/175.100-T.md`,
`.backlogit/queue/175.101-T.md`; feature contract: the "## Runner-bootstrap
prerequisite" section and `packaging.prerequisite_*` fields in
`.backlogit/queue/175-F.md`.

The 98-member remediation sub-DAG (184 edges, U1 source, U12 sink, 13 waves
RS-W00..RS-W12) is UNCHANGED internally; the bootstrap chain
`175.099-T -> 175.100-T -> 175.101-T` sits outside it as a
pre-DAG bootstrap. Feature executable-member total: **101** (98 remediation + 3
prerequisite).

## Supersession of 156-S (non-destructive)

The backlogit shipment lifecycle exposes no `abandon`/supersede verb in Stage
scope (valid shipment transitions are `queued -> active` and
`active -> shipped|abandoned`; there is no governed `queued -> abandoned/archived`
path without activation). Per the operator directive, `156-S` is therefore **left
queued with an explicit supersession record plus a best-effort ordering edge**,
never deleted:

1. A supersession comment is appended to `156-S` naming the 13 replacement
   shipment IDs and marking it DO-NOT-CLAIM.
2. A best-effort ordering edge `156-S depends_on RS-W12` is retained. This edge is
   NOT a hard claim guard — direct `core.ClaimShipment` bypasses dependency checks
   and queue filtering clears the edge on any no-longer-blocking terminal status of
   `169-S`. `156-S` is held non-claimable by GOVERNANCE (this DO-NOT-CLAIM record +
   Orchestrator claim routing) and by its empty manifest (a claim would activate no
   work). Tool-level non-claimable enforcement is captured as prerequisite work in
   stash `6434A4D7`. By the time RS-W12 ships, all 98 member tasks are already
   `done` via the replacement shipments.
3. This decision record documents the supersession for the Orchestrator's claim
   routing.

`156-S`'s committed manifest is now empty (`items: []`); its historical membership
(feature `175-F` plus the 98 remediation tasks) is preserved ONLY in Git history
and on branch `stage/baseline-convergence-main`, not in the current manifest. The
exclusive execution ownership of those tasks now lives in the replacement
shipments.

The operator authorization for retaining governed lint findings until terminal
convergence targets the active replacement sequence `157-S`..`169-S` — a derived
application of existing authority over the operator-directed repackaging, not
`156-S`. The prerequisite shipment `176-S` (tasks `175.099-T`, `175.100-T`, `175.101-T`) owns no baseline
findings and falls outside that authorization scope. The verifier bindings
(intermediate-wave and terminal) likewise target the active replacement sequence,
never `156-S`. See `docs/decisions/2026-09-17-baseline-convergence-authorization.md`.

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

- 98 unique remediation tasks, covered exactly once across the 13 replacement
  shipments `157-S`..`169-S` (no omission, no duplicate).
- 3 dependency-ordered runner-bootstrap prerequisite tasks `175.099-T`,
  `175.100-T`, `175.101-T`, each owned exactly once by the
  prerequisite shipment `176-S` and present in no other shipment. Feature
  executable-task total: 101.
- No replacement shipment (nor `176-S`) lists `175-F`.
- The replacement baseline sequence is gated behind `176-S` via
  `157-S depends_on 176-S`; the runner-bootstrap prerequisite is buildable under
  existing tooling without invoking the runner it creates.
- **Execution readiness of the replacement sequence (`176-S` and
  `157-S`..`169-S`) is BLOCKED** until the shipment-claim / wave-scheduler
  convergence prerequisite lands: the in-repo scheduler-baseline marker
  (`154-S`/`173-F`, edge `154-S blocks 176-S`) plus its external autoharness
  scheduler consumption (P-017). See "## Claim/Scheduler Execution Readiness
  (BLOCKED)" and
  `docs/decisions/2026-09-20-shipment-claim-wave-scheduler-convergence-deliberation.md`.
- Every replacement shipment and the prerequisite shipment `176-S` are `queued`.
- Dependency graph (including the prerequisite edges `154-S blocks 176-S`,
  `157-S depends_on 176-S`, and `175.001-T depends_on 175.101-T`) is acyclic and
  yields the declared ordered wave sequence RS-W(-1) → RS-W00 → … → RS-W12.
- 149-S is ordered behind the terminal replacement shipment (RS-W12 / `169-S`) by
  an advisory `blocks` edge; shipped-only readiness is governed by claim-routing
  policy until tool-level enforcement lands (stash `6434A4D7`). The advisory edge
  is not a hard shipped-only guarantee (see the Dependency / Ordering Map).
- 168.001-T remains archived / `done`.
- No source or workflow implementation files are modified on this branch.

## Plan Hardening

| # | Risk | Severity | Mitigation |
|---|---|---|---|
| H1 | Task dropped or duplicated across shipments | High | Mechanical validator (`logs/validate_decomp.py`) asserts exactly-once coverage of all 98 tasks; re-run post-assembly against live shipment manifests. |
| H2 | A dependency points backward/within a wave, breaking the shipment chain guarantee | High | Validator asserts all 184 edges are strictly forward-across-waves; confirmed PASS (0 intra-wave, 0 backward). |
| H3 | Backlog tool enforces single-shipment membership, blocking creation while 156-S holds the tasks | Medium | Contingency: if creation/add is rejected due to 156-S membership, clear 156-S membership via Stage file-authority edit + `sync` (non-destructive; 156-S retained), then retry. Empirically probed at assembly. |
| H4 | 149-S becomes eligible early | High | Rewire removes 149-S→156-S and adds advisory 149-S→RS-W12. This edge is advisory, not a hard shipped-only guarantee (queue filtering clears on any terminal cascade status; direct claim bypasses deps). Shipped-only readiness is governed by Orchestrator claim-routing policy; tool-level enforcement tracked in stash `6434A4D7`. |
| H5 | 156-S accidentally claimed after supersession | Medium | DO-NOT-CLAIM supersession record + empty manifest + advisory 156-S→RS-W12 edge + Orchestrator claim-routing. The edge is not a hard claim guard (direct claim bypasses deps); non-claimability is governed, with tool-level enforcement tracked in stash `6434A4D7`. |
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
- **D** — 149-S rewire orders 149-S behind the terminal replacement shipment
  (advisory ordering; the old 149-S→156-S edge is deleted at assembly).
  Shipped-only readiness is governed by claim-routing policy, not a hard tool
  guard (queue filtering clears on any terminal cascade status; direct claim
  bypasses deps).
- **E** — chain + 149-S rewire + 156-S guard introduce no cycle (156-S is a pure
  leaf-dependent after rewire).

Non-blocking P3 advisories (accepted, dispositioned):
1. Validator covered only task-level guarantees → **addressed**: a shipment-level
   session validator was added and run in Step 9, asserting the chain edges, the
   149-S rewire, the 156-S guard, and shipment-level acyclicity from the live
   backlog (results recorded under "Validation Evidence").
2. WAVES dict hard-coded independently of the doc table → accepted; they match
   exactly and both are committed together for auditability.
3. 156-S manifest cleared to empty (`items: []`); historical membership (including
   175-F) preserved only in Git history and on branch
   `stage/baseline-convergence-main`. Non-claimability is governed
   (DO-NOT-CLAIM record + empty manifest + Orchestrator claim-routing), not a hard
   tool guard; all members are `done` before RS-W12 ships.

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
  `169-S` transitively behind all 12 predecessors in the shipment DAG. This DAG
  placement orders 149-S last as an ADVISORY ordering signal; it does not by itself
  guarantee 149-S waits for a *successful ship* of 169-S (queue filtering clears the
  edge on any no-longer-blocking terminal status, and a direct claim bypasses
  dependencies). Shipped-only readiness is governed by claim-routing policy;
  tool-level enforcement is tracked in stash `6434A4D7`.
- **INV6** — `168.001-T` status = `done`.

Invariant 7 (no source/workflow implementation changed) verified via
`git diff origin/main`: staged changes are confined to `.backlogit/` and `docs/`;
zero changes under `internal/`, `cmd/`, `scripts/`, `tests/`, `.github/`,
`AGENTS.md`, `.autoharness/`, `Makefile`, `go.mod`, `go.sum`.
