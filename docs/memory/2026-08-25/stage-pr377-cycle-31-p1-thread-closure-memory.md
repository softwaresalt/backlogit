---
chunk_strategy: h2
description: "Stage PR #377 cycle-31 P1 thread closure — six unresolved review threads reduced to three roots and remediated: declaration-stub-before-harness withdrawn plan-wide, blocked-member accounting made total and fail-closed under an immutable manifest set, and wave green semantics scoped per task with a new Ship Step 4.6 convergence gate; workflow-policies 1.19.0; gate set to FAIL pending fresh local review"
doc_type: memory
schema_version: "1.0"
source: cycle-31-p1-thread-closure-session
title: "PR #377 Cycle 31 P1 Thread Closure Memory"
---

# PR #377 Cycle 31 P1 Thread Closure Memory

**Date**: 2026-08-25/26
**Agent**: Stage (prompt-builder scope)
**Branch**: `chore/stage-130-s`
**PR**: #377
**Cycle**: 31 (bounded P1 remediation of the six threads cycles 29/30 acknowledged and HELD)
**Worktree**: `.copilot/session-state/337f2436-0fad-4797-be93-b72985d25d56/files/stage-130s-worktree`
**HEAD at session start**: `5212ee45c0a36c9255fcf89b5fe61d4804057c45`
**Shipment**: `130-S` (queued — **not** claimed, **not** shipped)
**Scope honoured**: no subagents, no push, no merge, no production Go

## Mandate and outcome

The six remaining unresolved PR #377 review threads were all P1, all previously acknowledged as
**valid** and deliberately **HELD** because prior sessions were bounded to P2/P3 work. This cycle
closed all six. They reduce to **three roots**, and they were fixed at the root rather than at the
quoted line.

| Thread anchor | Root | Fix |
|---|---|---|
| `147.001-T:26` | R1 | Declaration doctrine |
| `147.030-T:44` | R1 | Declaration doctrine |
| `147.031-T:37` | R1 | Declaration doctrine |
| `147.034-T:44` | R1 | Declaration doctrine + declaration/behaviour split |
| `workflow-policies.md:534` | R2 | Blocked accounting |
| `.ship.agent.md:410` | R3 | Wave green semantics |

## R1 — declaration stub before harness, withdrawn plan-wide

Cycle 29 withdrew the `declaration-only` exemption class but never propagated the corrected
lifecycle into the staged tasks. Four of them still said *"land the declaration stub so the package
compiles, then the harness, then implement"*. That is the same NON-NEGOTIABLE Principle II
carve-out by a different route — the stub is observable production surface admitted with **no**
observed failing test — and it was additionally *unschedulable*, because `harness-architect` no
longer fabricates the missing declaration, so no actor was willing to produce the stub the tasks
assumed would exist.

**Canonical sequence now normative in P-002.1**: the compile-capable **source-shape** (`go/parser`
+ `go/ast`) failing harness lands **first**; the declaration implementation lands second and makes
it green. No agent may scaffold, request, or accept a production stub ahead of the harness that
gates it. The plan's three-step lifecycle became a **two-step** one.

**No stub loophole in either direction.** A companion rule states that behaviour beyond the declared
shape needs its own prior red, and that a seam or declaration whose body would absorb real
behaviour MUST be split into *declaration → behaviour-harness → implementation*. `147.034-T` / U11
was the severe case — its stub was specified to perform real read/parse/mutate/marshal/atomic-
replace behaviour — so U11 is now gated by its own `TestU11_` source-shape red, and its behaviour
red is relocated to U12's contract harness (the U11 → U12 → U13 shape), which is exactly the
"split declaration/behavior behind a pre-declaration harness" remedy the reviewer named.

**The sweep went beyond the four named tasks**, per the "all class matches" mandate: `147.005-T` /
U2d (empty-map stub) and `147.002-T` / U2 (`return nil` stub) carried the same defect unquoted;
`147.035-T` / U12 referenced "U11's declaration stub". In the plan, the Implementation Units head,
the Green-step-guards paragraph, U1/U1b/U1c/U2/U2d/U11/U12/U15, invariant I4, the U8b harness
posture, and Constitution Check II were all corrected. Historical amendment-log rows (lines 3179+)
were **preserved unmodified as history**; only live normative statements were rewritten, with a
supersession marker added to the one live-adjacent historical bullet in `147-F.md`.

## R2 — blocked accounting made total and fail-closed

`queued` was overloaded between a derived residual (`M \ done`) and a literal status. Neither
reading is safe: the derived one silently re-admits a member an operator explicitly returned; the
literal one leaves a `blocked` member in **no examined set at all**, so `WAVE_NO_PROGRESS` never
fires and the loop terminates reporting completion with part of the set unfinished — a fail-open
outcome in a gate whose whole purpose is to fail closed.

The fix rests on an **immutable shipment manifest member set `M`**, frozen at Ship Step 0.5 and
never re-derived from the shipment's live `items` list. That last clause is load-bearing:
`core.ReturnBlockedItem` (`internal/core/shipment.go:626-665`) *removes* a returned member from
that list, so a scheduler that re-read it each wave would watch the member vanish and then report
completion over a shrunken set. The simulation's non-frozen-`M` control reproduces exactly that.

* Statuses are the five recognized tokens; `M = queued ⊎ active ⊎ blocked ⊎ terminal` is **total**.
* `terminal = {done, archived}`; **completion is `terminal = M` and nothing else**. An empty ready
  frontier is never completion.
* Any blocked member halts with new code **`WAVE_MEMBER_BLOCKED`**, reporting the full census, each
  `blocked_reason`, the **transitive dependency impact**, and whether the existing `return_blocked`
  process has been invoked/recorded (invoking it when applicable).
* An `active` residual halts with `WAVE_NO_PROGRESS` (detail: `active residual`).
* The snapshot recipe must return `blocked` and `active` rows, and its `status != 'archived'`
  predicate was **removed** — `archived` is terminal and satisfies dependencies.
* A **blocked-injection validation** is now mandatory before a wave schedule may be relied on.

## R3 — wave green semantics

Step 4.0 scaffolded the whole wave red at once (item 5) and then built its members one at a time
(item 6), while `build-feature`'s post-loop gate required `go test ./...`. The first task of any
multi-member wave therefore ran the full suite against its siblings' still-red harnesses and could
never reach green regardless of its own correctness. **11 of this release unit's 18 waves** carry
more than one non-exempt member, so this was not hypothetical.

* `harness-architect` **may** still batch-scaffold every current-wave harness — that is the point
  of a wave, and simultaneous red is the designed state.
* The per-task modify→test loop and post-loop gate use **only the task's scoped command**.
* A repo-wide or full-package suite is **forbidden inside a per-task loop** when it would include a
  sibling RED. The `-run=^$` compile check stays repo-wide: it executes no test.
* The tolerated-red set is exactly `sibling_red_selectors` — explicit, closed, supplied by Ship,
  and **non-widenable**. Any failure outside it halts. It is not a blanket "ignore failing tests"
  mode; a negative control in the simulation confirms the gate halts.
* The full suite is **relocated, not skipped**, to a new mandatory **Ship Step 4.6 wave convergence
  gate**, which runs once per wave after every member is individually green, with an **empty**
  tolerated-red set. On failure, remediate **inside the current wave**; never advance.
* **Six task-scoped command requirements** were defined so the scoping cannot become a weakening.
* Every branch-wide gate (Step 4.3 lint/format/exempt, Step 4.4 review, Step 5 PR lifecycle) is
  preserved unchanged.

## Simulation evidence (17/17 assertions PASS)

* **Baseline**: `COMPLETE`, **18 waves**, 42/42 tasks, **0** stalls, **0** compile-order violations.
* **Blocked injection** (`147.030-T` → `blocked`): `WAVE_MEMBER_BLOCKED` at wave 1, **no** false
  completion, member retained in `M`, dependency impact **26** members. Mid-run, active-residual,
  and unrecognized-status variants all fail closed.
* **Non-frozen-`M` control**: `WAVE_NO_PROGRESS` at wave 10 — the failure the frozen-`M` contract
  prevents.
* **Sibling-red wave** (wave 2): OLD repo-wide gate progressed **0/2** (the original P1 deadlock
  reproduced); NEW scoped gate progressed **2/2**; **1** convergence full-suite run; **0**
  full-suite runs inside per-task loops.

## Validation evidence

| Gate | Result |
|---|---|
| Markdown P-008 repo-wide | 0 issues in 2291 files |
| `backlogit docs lint` | `valid: true`, 0 violations |
| `backlogit sync` | 1209 artifacts, 0 parse failures |
| Topology | 42 tasks / 104 executable edges / 43 shipment members — unchanged |
| `go build ./...`, `go vet ./...` | exit 0, exit 0 |
| `go test ./...` | **exit 0**, 29 `ok`, 0 `FAIL` |
| Production Go modified | **none** |

`gofmt -l .` lists every file in this Windows checkout — a pre-existing CRLF artifact, not a
finding; `git status` shows zero `*.go` modifications.

## Gate state

`cycle: 31`, `decision: FAIL`, `pending: fresh-local-review-required`, `push_allowed: no`,
severity P0=0 / P1=6 (all remediated in-pass) / P2=0 / P3=0, topology SOUND and unchanged.
`workflow-policies.md` **1.18.0 → 1.19.0**.

**Why FAIL rather than ADVISORY.** A cycle that rewrites the harness lifecycle plan-wide, the wave
scheduler's completion semantics, and the per-task green gate is not self-certifying. The corrected
contract has been reviewed only by the session that authored it, so the honest gate is FAIL with a
fresh local review required before push. §1.9 Check 3 remains blocking.

## Lessons

* **A withdrawal must be swept, not just declared.** Cycle 29 withdrew `declaration-only` in the
  policy and fixed the two units that motivated it, but four sibling tasks kept the old lifecycle.
  The class was gone; the *ordering* it had licensed survived. Withdrawing a carve-out means
  sweeping every artifact that encoded it, including the ones the reviewer did not quote.
* **Fail-closed gates compose badly with each other.** This is now the fourth defect of the same
  shape in the P-002.6 design (vacuous delta gate, missing `blocked` disposition, unsatisfiable
  wave order). Each new gate was individually reasonable and broke an existing one. New gates need
  a composition check against the gates already in the path, not just a soundness check.
* **Derived sets hide states.** `queued := T \ done` looked like a safe definition and silently
  created a state (`blocked`) that no branch examined. A **total** partition over an explicit,
  closed status vocabulary makes the missing branch impossible to omit.
* **Immutability is the fix when an operation mutates the thing you are counting.**
  `return_blocked` mutating the live items list is correct behaviour; the defect was counting
  against a mutable set. Freezing `M` separates the accounting universe from the working list.
* **Scoping a gate is safe only when the scope is explicit, closed, and paired with a converged
  gate.** "Tolerate sibling red" is an escape hatch unless the tolerated set is enumerated,
  non-widenable, and followed by a full-suite gate with an empty tolerated set.

## Next actions

1. **Fresh local plan review of the cycle-31 contract** — required before push; this is what the
   FAIL blocks on.
2. Push `chore/stage-130-s` (blocked on 1).
3. Reply to and resolve the six PR #377 threads against the new HEAD (blocked on 2).
4. Operator merge approval (P-014) — not requested.
