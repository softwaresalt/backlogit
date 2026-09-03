---
chunk_strategy: h1-h2
description: "Stage cycle-29 PR #377 correction: withdrew the declaration-only exemption class, introduced dependency-aware harness waves (P-002.6), and closed the three bounded threads"
doc_type: memory
schema_version: "1.0"
title: Stage PR #377 cycle-29 declaration-only withdrawal and harness-wave remediation memory
---

## Session frame

* Agent: Stage acting as prompt/plan artifact owner for PR #377 under explicit operator scope —
  prompt, policy, plan, and task artifacts only; **no subagents, no push, no merge, no production
  Go implementation**
* Worktree: `.copilot/session-state/337f2436-0fad-4797-be93-b72985d25d56/files/stage-130s-worktree`
* Branch: `chore/stage-130-s`; local and remote HEAD at session start `6a822ceb`, worktree clean
* Carried-in gate: §1.9 readiness **FAIL on Check 3** (six unresolved merge-blocking threads);
  cycle 26-28 fix budget spent per `github-pr-automation` §1.8
* Carried-in topology: 42 tasks / 104 executable edges / 43 shipment members; exempt set **ten**

## The six threads and how they map

All six were queried live over the GraphQL API (110 threads total, 6 unresolved, all
`copilot-pull-request-reviewer`-authored, zero unresolved human threads). Each had already been
accepted as valid and deferred with a written classification; this cycle executes those remedies.

| # | Thread ID | Anchor | Carried classification | Cycle-29 disposition |
|---|---|---|---|---|
| T1 | `PRRT_kwDORzozKM6cTh8V` | `workflow-policies.md:261` | bounded | P-002.3 must-fail list no longer admits an unmarked exit 0 |
| T2 | `PRRT_kwDORzozKM6cTh8s` | `.ship.agent.md:254` | requires decomposition | new **P-002.6** dependency-aware waves |
| T3 | `PRRT_kwDORzozKM6cTh9B` | `build-feature/SKILL.md:213` | bounded | completion gate diffs baseline → working tree |
| T4 | `PRRT_kwDORzozKM6cTo9A` | `workflow-policies.md:75` | foundational | `declaration-only` **withdrawn**; source-shape harnesses |
| T5 | `PRRT_kwDORzozKM6cTo9V` | `147.019-T.md:83` | requires decomposition | workspace relocated to an already-ignored path |
| T6 | `PRRT_kwDORzozKM6cTo9l` | `.autoharness/drift-ignore:70` | bounded | marker note corrected to gate-command-only |

## The coupled decision (T4 + T2)

The deferral notes recorded that T4 and T2 **had to be decided together**, because the wave design
depends on whether declarations stay exempt. Both were decided in favour of the reviewer.

**T4 — `declaration-only` withdrawn.** The class let observable production surface land with no
observed failing test: U1d adds a serialized `remediation_intent` field to `CheckpointSummary`,
U15 adds an exported `GetCheckpointResult` wrapper. Constitution Principle II is NON-NEGOTIABLE and
admits no carve-out, so the class is withdrawn rather than covered by a signed deviation
(option **A**, not option **B**).

**The third option cycle 20 missed.** Cycle 20 saw only two shapes for a declaration's test — one
that references the missing symbol (a build error, which P-004 rejects) or one that passes the
instant the shape lands (never red) — and resolved the tension by exempting. A **source-shape**
harness is neither: it parses the production file with `go/parser` and asserts the declared shape
through `go/ast`, naming no undeclared identifier. It compiles before the declaration exists and
fails on an assertion. **Proven empirically at HEAD `6a822ceb`**, not merely asserted:

* `go vet ./internal/events` → exit **0** with the harness present
* `go test -run='^$' -count=1 ./internal/events` → `ok … [no tests to run]` (P-002 postcondition)
* `go test -count=1 -v -run '^TestU1d_'` → exit 1, `--- FAIL` ×2 with assertion messages
* `go test -count=1 -v -run '^TestU15_'` → exit 1, `--- FAIL` ×2 with assertion messages
* `RemediationIntent`, `remediation_intent`, `CheckpointReadResult`, `GetCheckpointResult`:
  **0** occurrences under `internal/events/` at that HEAD

The probe file was deleted after measurement; the worktree was re-verified clean before any edit.

**T2 — waves.** The reviewer's counter-example was exact: U8b (`147.016-T`) cannot compile until
U15 (`147.038-T`) lands, but U15 depends on U1b (`147.030-T`), which is **not** exempt and cannot
be implemented before its own harness exists. The cycle-26 "early declaration pass" was still only
**one wave** and cannot satisfy a chain of depth > 1. New policy **P-002.6** interleaves harness
generation and implementation per topological wave.

**The two fixes compose.** Removing the exemption *adds* two harness obligations to the DAG, and it
is the wave scheduler that makes them satisfiable: U1d (wave 1) → U1b (wave 2) → U15 (wave 3) →
U8b (wave 4). Each harness is scaffolded only after every declaration it compiles against is
`done`. No waiver, no early pass, no exemption.

## Wave verification

Simulated against the **live index** after re-sync (not against the prose):

* 42 nodes, 104 edges, acyclic — Kahn orders 42/42
* **18 waves, 42/42 tasks scheduled, 0 stalls, 0 compile-order violations**
* every dependency lands in a strictly earlier wave than its dependent
* `covered-by` owner U12 (wave 6) precedes U13 (wave 7), so `EXEMPT_OWNER_NOT_RED` is satisfiable
* negative control: injecting the cycle `147.032-T → 147.038-T` halts at `WAVE_NO_PROGRESS` after
  9 waves with 26 blocked — the detector fires rather than looping

## The three bounded fixes

* **T1**: the pre-work probe now classifies three ways — required failure (proceed),
  `EXEMPT_FALSE_GREEN` (exit 0 **with** marker), `EXEMPT_MARKER_MISSING` (exit 0 **without**
  marker). The unmarked-exit-0 row is deliberately **removed** from the false-green table with a
  note saying why, so it cannot drift back. The marker's scope is stated positively:
  `exempt_verification_command` **only**, never `harness_owner_command`.
* **T3**: the two consumers sit on opposite sides of the commit, so they need different diff forms.
  `build-feature` runs **before** its own `### Commit`, so it uses `git diff {baseline}` (two-dot,
  no right-hand side) plus `--cached`; Ship Step 4.3 runs **after** and keeps `{baseline}..HEAD`.
  A P-002.4 table now states both forms and why they differ, and an **empty** changed-file set is
  a halt rather than a trivial pass — the previous `..HEAD`-before-commit form compared the
  baseline against itself and made the fail-closed gate a no-op.
* **T6**: the cycle-24 drift-ignore note claimed the marker applied to `harness_owner_command`
  "distinctly". Corrected — a template adoption would otherwise reconstruct a requirement the
  policy does not have and that U13 would fail. The cycle-26 notes are marked SUPERSEDED so the
  withdrawn early-pass rule cannot be re-applied either.

## T5 — U10 resolved without decomposition

The squeeze was real: `verification-only` admits only `*_test.go` and `docs/closure/`, but U10 also
committed a `.gitignore` rule for a scratch workspace at `docs/scratch/checkpoint-verification/`,
which `.gitignore` does not cover. With the rule → `EXEMPT_DELTA_EXCEEDS_CLASS`; without it → dirty
tree → U10b's claim-time baseline halts. Both branches halted.

Neither remedy the reviewer named was needed. The workspace moves to
`.copilot/scratch/checkpoint-verification/`, which is **already** ignored — verified with
`git check-ignore -v` → `.gitignore:5:.copilot/`, and `git ls-files .copilot` → 0 tracked entries;
`*.exe` (`.gitignore:25`) additionally covers the built binary. U10 therefore commits **no** ignore
rule, topology is unchanged at 42/104/43, and no task was added. The class contract is **narrowed**
rather than widened: P-002.4 now explicitly rejects `.gitignore` and other configuration files
under `verification-only`, which protects the `EXEMPT_BEHAVIOR_NO_OWNER` boundary cycles 22-24
spent four passes tightening. The deferral note's stated preference ("prefer the split") was
overtaken by a better option that costs nothing.

## Contract changes

* `workflow-policies.md` **1.16.0 → 1.17.0** — P-002 postcondition per wave; P-002.1 vocabulary
  4 → 3 tokens plus the source-shape rule; P-002.2 gains `WAVE_NO_PROGRESS`,
  `WAVE_CYCLE_DETECTED`, `WAVE_BUDGET_EXCEEDED`; P-002.3 three-way probe + marker scoping;
  P-002.4 per-consumer diff form, empty-delta halt, hygiene prohibition; P-004 wave scoping and
  declaration applicability; **new P-002.6**
* `.ship.agent.md` — Step 2 wave-scoped (cycle-26 early pass removed), Step 3 → wave schedule,
  **new Step 4.0** wave admission, Step 4.3 diff rationale, two new circuit breakers, one
  branch/worktree/PR for all waves (P-016)
* `build-feature/SKILL.md`, `harness-architect/SKILL.md`, `.autoharness/drift-ignore` — aligned
* `147.032-T`, `147.038-T` — de-exempted, source-shape harness specs, red selectors, wave placement
* `147.019-T`, `147.041-T` — workspace relocated; `147-F`, `130-S`, plan — counts and wave schedule

## Validation performed

| Check | Result |
|---|---|
| markdownlint (P-008) repo-wide | 2289 files, **0 issues** |
| `backlogit sync` | 1209 artifacts indexed, `parse_failures=0` |
| `backlogit doctor` | 23 pre-existing `orphaned_artifact` findings in `016.*`/`106.*`; **zero** under `147.*` |
| `backlogit docs lint` | `valid: true`, `violation_count: 0` |
| `go test ./tests/...` | contract + integration **PASS** |
| Exempt set from live index | exactly **8**, matching the plan table |
| Contract block ↔ label parity | 8/8 both present; **0** `declaration-only` class values |
| Topology from live index | 42 tasks / 104 edges (unchanged); 34 harness-required / 8 exempt |
| Wave simulation (live index) | 18 waves, 0 stalls, 0 compile-order violations |
| U1d/U15 baseline probes | compile PASS, RED confirmed on assertions (see above) |
| Task widths | U1d and U15 each 2 files / 3 scenarios (limits: `<3` files, `<4` scenarios); U10 reduced 1 → 0 files |
| Step-pointer resolution | all `Ship Step …` references resolve to real headings |

## Gate state and what is NOT done

* **No push, no merge, no production Go.** The AST probe used to prove the RED was deleted and the
  worktree re-verified clean before editing.
* §1.9 readiness stays **FAIL on Check 3**. The six threads are *addressed* but not self-resolved:
  they need a fresh local review of these contract changes and a re-review of PR #377 on the new
  HEAD. This session does not clear its own gate.
* Plugin bundle (`plugin/`) remains explicitly **out of scope** — unchanged from the cycle-24 B2
  disposition and follow-up stash `633818E1`.

## Observation for a later session (not fixed here, out of scope)

`workflow-policies.md` P-015 carries a pointer to `.ship.agent.md` "Step 6.1.b", which is not a
heading in Ship (`Step 6` and `Step 6.0` exist). It is **pre-existing** at `origin/main`, belongs
to shipment closure rather than the harness/claim-time domain, and is not one of the six threads —
so it was left alone rather than guessed at. Worth a bounded fix in a future pass.
