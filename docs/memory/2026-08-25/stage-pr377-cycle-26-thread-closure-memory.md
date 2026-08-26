---
chunk_strategy: h1-h2
description: "Stage cycle-26 PR #377 lifecycle: pushed the cycle-20..25 remediation, replied to and resolved all 20 Copilot threads, and fixed three fresh review findings in prompt artifacts"
doc_type: memory
schema_version: "1.0"
title: Stage PR #377 cycle-26 thread closure and review-fix memory
---

## Session frame

* Agent: Stage acting as PR lifecycle owner for PR #377 under explicit operator authorization
  (push and PR interaction granted; merge, production Go changes, and shipment claim withheld)
* Worktree: `.copilot/session-state/337f2436-0fad-4797-be93-b72985d25d56/files/stage-130s-worktree`
* Branch: `chore/stage-130-s`; local HEAD at session start `18d41b1f`, remote at `3bcff086`
* Local gate carried in: cycle-24 Plan Review `ADVISORY`, `P0=0 / P1=0`,
  `operator_authorization: approved`, topology 42/104/43, ready roots `{147.001-T, 147.032-T}`

## What happened

1. **Push.** Verified the worktree clean at the exact expected HEAD and pushed
   `3bcff086..18d41b1f` normally (no force). Six commits, zero `*.go` files in the range.
2. **Thread closure.** Paginated all PR #377 review threads over the GraphQL API (98 threads,
   20 unresolved, all `copilot-pull-request-reviewer`-authored, zero unresolved human threads).
   Each of the 20 already carried a cycle-19 "accepted, deferred to cycle 20" reply. Posted a
   **new** reply on every one naming the exact fixing commit(s), the concrete behavior change, and
   the validation evidence, then resolved each thread. 20 replies posted, 20 threads resolved.
   `mergeStateStatus` moved `BLOCKED -> CLEAN` on resolution, confirming that the `PR-Required`
   ruleset's `required_review_thread_resolution` was the blocking condition.
3. **Fresh review.** The `PR-Required` ruleset carries `copilot_code_review` with
   `review_on_push: true`, so the review for `18d41b1f` was auto-requested by the push rather than
   needing an explicit re-request. It completed as `COMMENTED` on `18d41b1f` and opened three new
   threads.
4. **Review-fix cycle 1 of 3.** All three findings were valid and confined to prompt/planning
   artifacts. Fixed, validated, and pushed.

## Thread-to-commit mapping used in the replies

| Root cause | Threads | Fixing commits |
|---|---|---|
| A — green regression guards inside the red harness | 14 | `f5f745fa` (three-step lifecycle, `TestU<unit>Guard_` naming contract, invariant I4); `e8b974e8` reinforces `147.009-T` |
| B — units with no genuine failing RED | `147.007-T`, `147.032-T`, `147.038-T` | `f5f745fa` + `e8b974e8` + `35aac6c0` + `89aebab5` (harness-exempt closed set, P-002.1-P-002.5, `EXEMPT_VERIFY_OK`) |
| C — false published read-surface contract | `147.014-T` line 32 | `f5f745fa` (new `147.043-T` / U6e makes the intent contract total) |
| D — stale halt path / stale archive note | `147.021-T`, `.backlogit/archive/147.010-T.md` | `f5f745fa`; ordering hardened in `89aebab5` |

## Cycle-26 findings and fixes

1. **Declaration-first DAG deadlock** (`.github/agents/.ship.agent.md` Step 2). The up-front
   harness gate required every queued task harness-satisfied before any implementation, but
   `147.032-T` / U1d and `147.038-T` / U15 are `declaration-only` exempts whose declarations are
   what `147.012-T` / `147.016-T` harnesses compile against. Fixed by inserting an explicit
   **declaration-prerequisite ordering rule** as Step 2 item 3 (shifting the old items 3 and 4 to
   4 and 5): execute and `EXEMPT_VERIFY_OK`-verify declaration-only prerequisites before
   scaffolding their dependents. Ordering only — no task with a harness obligation is implemented
   before its harness is red, and a non-`declaration-only` blocker halts.
   Mirrored in `.github/skills/harness-architect/SKILL.md` Step 1 item 6: halt and report a missing
   prerequisite rather than fabricating or stubbing the declaration.
2. **Guard-file collision** (`.github/skills/build-feature/SKILL.md` Step 5). `147.042-T` / U3c
   creates `internal/events/checkpoint_lifecycle_conformance_test.go` at Kahn position 12;
   `147.007-T` / U3b named the same file for its `TestU3bGuard_*` guards but runs after
   `147.037-T` at position 25, so the file is pre-existing and the narrow new-test-file exception
   would reject the append. Fixed by giving U3b its own
   `internal/events/checkpoint_lifecycle_conformance_guard_test.go` rather than by re-widening the
   exception (the narrowing in `35aac6c0` was deliberate). Updated `147.007-T`'s Files line,
   `exempt_verification_command` `Test-Path`, baseline-probe note, and P-002.4 class delta surface,
   plus the plan's U3b Files bullet. Added a **no-collision rule** to the exception: "new" is
   evaluated against the execution order, and a collision raises
   `EXEMPT_DELTA_EXCEEDS_CLASS` instead of being admitted.
3. **Stale PR description.** The PR body still described pre-cycle-20 scope
   (planning/backlog/memory-only, 40/98/41). Corrected to the actual HEAD: 42/104/43, the
   agent/skill/policy/config surfaces in the diff, the cycle-24 ADVISORY gate, and the P-016
   topology warnings. No commit was made solely for GitHub state.

## Files modified this cycle

* `.github/agents/.ship.agent.md` — Step 2 declaration-prerequisite ordering rule
* `.github/skills/harness-architect/SKILL.md` — Step 1 item 6 prerequisite assumption and halt
* `.github/skills/build-feature/SKILL.md` — Step 5 no-collision rule
* `.backlogit/queue/147.007-T.md` — distinct guard file across four references
* `docs/exec-plans/2026-08-24-checkpoint-toplevel-key-disposition-plan.md` — U3b Files bullet
* `.autoharness/drift-ignore` — cycle-26 re-apply obligation note

## Validation

| Gate | Result |
|---|---|
| `backlogit --cwd . sync` | 1209 artifacts, 0 parse failures |
| `backlogit --cwd . docs lint` | `valid: true, violation_count: 0` |
| `markdownlint-cli2` (5 changed files) | 0 issues |
| CI on `18d41b1f` | 5/5 green — `test`, `Detect code changes`, `Docline frontmatter gate`, `Markdown lint (P-008)`, `CLI Reference Drift` |
| P-009 merge settings | `allow_merge_commit: true`, `allow_squash_merge: false`, `allow_rebase_merge: false`; ruleset `allowed_merge_methods: ["merge"]` |

## P-016 topology

Four worktrees are attached. `stage-130s-worktree` is the single implementation worktree for this
release unit and is the only one that produced commits on `chore/stage-130-s`. Two extras are
recorded as **warnings**, not deletions:

* `.copilot/worktrees/cycle24-remediation` @ `cd2ad50b` [`chore/cycle-24-remediation`] — auxiliary
* `.copilot/session-state/ecebe820-.../files/dark-factory-worktree` @ `5803cbd0`
  [`chore/121-s-closure`] — unrelated release unit

Neither is an explicit Stage spike/research worktree, so both are `prohibited/ambiguous` under
P-016's classification step. They did not contribute to this branch, so they do not invalidate the
PR's own review evidence — but P-016's Violation Action names PR/closure work explicitly, so they
must be resolved by the operator before Ship claims `130-S` or executes closure.

## Open questions and next steps

1. Await the fresh Copilot review on the final head and re-run the §1.9 readiness gate.
2. Merge remains withheld — this session holds no merge authority (P-014 operator approval
   required, and Stage does not merge).
3. The operator must resolve or explicitly exempt the two extra worktrees before Ship handoff.

## Cycle 27 — second review-fix cycle (`a83680e7`)

The review on `fcd57a9c` raised two findings against the cycle-26 fixes themselves. Both valid.

1. **The ordering rule defined a partial gate.** Step 2 item 3 said to verify each declaration
   prerequisite "with its `exempt_verification_command`, requiring the `EXEMPT_VERIFY_OK` marker" —
   a shortcut probe that skipped the pre-work must-fail observation, the P-002.5 screen, the claim,
   the `build-feature` dispatch, the quality and delta gates, and the status transition. Rewritten
   so the rule **selects which tasks run early and in what order** and nothing else: each
   prerequisite goes through the full Step 4 loop (4.1a → 4.1b → 4.2 → 4.3) unchanged, and any
   Step 4 halt code halts the early pass exactly as it would the main loop.
2. **`build-feature` Step 0 let a malformed gate command through.** It listed "exit 0 without the
   marker" among must-fail signals and then called it "still a failed attempt, not a green light to
   skip Step 1" — permitting the loop to start after a command that never printed its marker, which
   contradicts Ship Step 4.1a and the P-002.2 taxonomy. Items 3 and 4 split so both exit-0 branches
   halt: with marker → `EXEMPT_FALSE_GREEN`, without marker → `EXEMPT_MARKER_MISSING`. Ship Step
   4.1a's parallel wording was aligned so neither surface reads as more permissive than the other.

## Cycle 28 — third review-fix cycle (`76bf8493`)

The review on `a83680e7` raised one finding: the P-002.4 completion gate could not enforce the
delta classification it advertised.

* `<pre-task-commit>` was a placeholder in `.ship.agent.md` Step 4.3 and `workflow-policies.md`
  P-002.4 that **no step ever captured or passed**, and the check was filename-only — so a
  `declaration-only` task could alter behavior inside a file its own contract names and pass,
  despite P-002.4's class table already stating content restrictions.
* Fixed by defining `exempt_baseline_sha`: captured at Ship Step 4.1a item 5 (`git rev-parse HEAD`,
  after the probe, before claim, halting on a dirty tree), passed through Step 4.2, and consumed by
  `build-feature` as a required input that must not be re-derived from `HEAD`. An absent or
  disputed baseline halts with `EXEMPT_DELTA_EXCEEDS_CLASS`.
* The Evidence rule became **two required passes**: a path pass, plus a per-class **content pass**
  over the hunks (`declaration-only` may not touch a pre-existing function body absent the
  behavior-preserving re-expression and its contract pin; `verification-only` may not weaken,
  delete, rename, or narrow a pre-existing assertion; `docs-only` no `*.go` hunk; `covered-by` no
  `*_test.go` hunk). Full AST validation was declined as out of reach for this workflow; the rule
  prefers `list_symbols` / `impact_analysis` when `agent-engram` is installed and falls back to the
  raw diff.

Three review-fix cycles are now spent, which is the §1.8 limit. Any further Copilot finding is a
backlog follow-up, not a fix in this session — and per §1.8 it remains merge-blocking rather than
waived.

## Cumulative validation

| Gate | Result |
|---|---|
| CI on `18d41b1f`, `fcd57a9c`, `a83680e7` | 5/5 green each — `test`, `Detect code changes`, `Docline frontmatter gate`, `Markdown lint (P-008)`, `CLI Reference Drift` |
| `backlogit docs lint` after every cycle | `valid: true, violation_count: 0` |
| `markdownlint-cli2` over each changed set | 0 issues |
| Go source touched | none, across all four pushes |
| `<pre-task-commit>` occurrences remaining | 0 |
| Threads | 26 replied and resolved (20 cycle-20 + 3 cycle-26 + 2 cycle-27 + 1 cycle-28) |
