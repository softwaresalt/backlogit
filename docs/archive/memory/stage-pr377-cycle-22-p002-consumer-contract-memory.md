---
chunk_strategy: h1-h2
description: "Stage cycle-22 bounded prompt/policy remediation: the harness-exempt ready-selection rule generalized from a shipment-local adapter into the authoritative fail-closed P-002 consumer contract, plus the cycle-21 plan-side findings"
doc_type: memory
schema_version: "1.0"
title: Stage PR #377 cycle-22 P-002 harness-exempt consumer contract memory
---

## Session frame

* Agent: Stage (prompt/policy and planning artifacts only; no Go source, no Go test, no build
  loop, no push, no merge, no shipment claim, no subagent delegation)
* Worktree: `.copilot/session-state/337f2436-0fad-4797-be93-b72985d25d56/files/stage-130s-worktree`
* Branch: `chore/stage-130-s`; HEAD at session start `ee01e986895daa3e209582bddf1d08e2d94a69e3`
* Tooling: worktree-bound `go run ./cmd/backlogit --cwd .` CLI, `markdownlint-cli2@0.23.1`, and
  read-only `go test ./tests/integration/` for the existing structural guards (MCP not used)

## Baseline observed before any edit

Against the live index under `147-F`:

| Measure | Value |
|---|---|
| Queued tasks | 42 |
| Carrying `harness-ready` | 0 |
| Carrying `harness-exempt` | 10 |

The P-002 consumer as written admitted only `harness-ready`. Ship's Step 2 would therefore have
partitioned all forty-two tasks into "needs harness", scaffolded red tests for the ten
declaration-only / docs-only / verification-only / `covered-by` units, and then halted at its own
Step 2 gate ("confirm every queued task now carries the `harness-ready` label"). Cycle 21's
shipment-local adapter paragraph could not prevent that, because the global consumer halts first.

## What this session did

Generalized the rule into an authoritative, fail-closed consumer contract — not a PR-specific
carve-out — and fixed the cycle-21 plan-side findings that were in prompt/docs scope.

### Authoritative policy — `.github/policies/workflow-policies.md`

* **P-002** retitled *TDD Gate (Harness-Satisfied Precondition)*. Statement, precondition,
  postcondition, enforcement, and violation action now admit a task that carries `harness-ready`
  **or** carries `harness-exempt` and is P-002.1-valid.
* **P-002.1 — Harness-Exempt Alternative Satisfaction (Fail-Closed)** added. Closed class
  vocabulary (`declaration-only`, `docs-only`, `verification-only`, `covered-by <owner-id>`);
  required metadata (label, class, one-line reason, membership in a declared closed exempt set,
  and the owner ID for `covered-by`); owner conditions for `covered-by` (exists in the release
  unit, is a declared dependency, is itself `harness-ready` rather than exempt, and has confirmed
  red evidence with its harness commit landed before build); a six-step evaluation order; the
  producer no-scaffold obligation; and the preserved test-first semantics for all non-exempt
  behavior.
* **P-002.2 — Harness-Exempt Halt Taxonomy and Reporting** added: five codes
  (`EXEMPT_CLASS_UNRECOGNIZED`, `EXEMPT_NOT_IN_CONTRACT`, `EXEMPT_BEHAVIOR_NO_OWNER`,
  `EXEMPT_OWNER_INVALID`, `EXEMPT_OWNER_NOT_RED`) with report strings, recorded through P-005.
* **P-004** gains the vacuous-satisfaction relationship: its precondition quantifies over
  scaffolded harness test functions, an exempt task scaffolds none, and fabricating a test to
  manufacture a red assertion is a producer-obligation violation rather than compliance.
* Amendment log entry `1.15.0`.

### Consumers

* `.github/agents/.ship.agent.md` — Step 2 becomes a three-way partition (already harnessed /
  exempt / needs harness); new **Step 2a** carries the fail-closed evaluation with explicit halt
  codes; Step 3 selects on the harness-satisfied predicate and names the `backlogit` equivalent
  SQL, valid only after Step 2a has run; Step 4.2 defines `harness_cmd` per exemption class; the
  Role bullet notes the skill receives only not-yet-satisfied tasks.
* `.github/skills/harness-architect/SKILL.md` — Step 1 excludes already-satisfied tasks; new
  **Step 1a** carries the same fail-closed evaluation; an exempt-skip intercom row; Step 6 scopes
  the `harness-ready` label to tasks this run scaffolded; completion criteria and guardrails
  forbid scaffolding or fabricating a RED for a valid exempt task.
* `.github/skills/build-feature/SKILL.md` — "When to Use" accepts harness-satisfied dispatch,
  `harness_cmd` is defined for exempt tasks, and a constraint forbids weakening any pre-existing
  harness assertion to reach green.

### Generation-drift bookkeeping

`.autoharness/drift-ignore` gains `.github/skills/build-feature/SKILL.md` and a dated note naming
the three generated surfaces that now carry the local P-002.1 / P-002.2 contract, so a future
autoharness template adoption re-applies it. `workflow-policies.md` and `.ship.agent.md` were
already listed. `harness-architect/SKILL.md` is repo-authored (no generation footer, absent from
`.autoharness/harness-manifest.yaml`) and needs no entry.

## Plan-side findings fixed

| ID | Finding | Fix |
|---|---|---|
| C1 | Global P-002 consumer accepted only `harness-ready`, so Ship halts before shipment `130-S` and would scaffold red harnesses for all ten exempt units | The policy/agent/skill contract above; the plan's adapter paragraph renamed "Ship ready-selection contract" and rewritten as a conforming consumer of the general rule |
| C2 | "No exception for behaviour" asserted that a behaviour-changing task *always* requires `harness-ready`, contradicting U13 in the same table | Reworded plan-wide: behaviour requires red **evidence**, carried on the unit itself except for exactly one explicitly allowed edge-backed carve-out, **U13 / `147.036-T`**, whose harness is owned by prerequisite **U12 / `147.035-T`**. Corrected in the Documented deviations row, the Constitution Check Principle II row, the Ship ready-selection contract bullets, the U13 unit, `147-F.md`, and `147.036-T.md` |
| C3 | The `147.018-T` / U9b hard merge gate was discoverable only from its owner | `147.007-T`, `147.008-T`, `147.009-T` each gain a `merge-gate-dependent` label and a backreference paragraph; `147-F.md` and `130-S.md` state the gate; the plan's U9b bullet records the discoverability delta. Labels and prose only — no dependency edge, executable ordering unchanged |
| C4 | Two current-gate pointers still named cycle 20 as current, and the cycle-20 remediation appendix asserted itself as current | All three normalized to `cycle: 21`. Exactly one current-gate-state claim remains |
| C5 | `147-F.md` claimed in the present tense that "32 tasks carry a red harness" when none has been scaffolded | Reworded to planned ownership: 32 tasks are **planned to own** a red harness, each naming the `TestU<unit>_` functions its harness step must land |

## Gate status — unchanged

Cycle 22 is **remediation evidence, not a gate**. No `## Plan Review` record was appended, no
`PASS` is claimed, and the current gate state remains `cycle: 21` `FAIL` with
`restage_recommendation: confirmatory-review-of-cycle-21-fixes`. The plan carries a
"PR #377 plan remediation, cycle 22" appendix under the cycle-21 record that says so explicitly.

## Topology (unchanged, re-verified)

* 42 queued tasks under `147-F`, 104 queued-to-queued executable edges, 43 shipment members
* Ready roots unchanged: `{147.001-T, 147.032-T}`
* Independent Kahn topological sort re-run against the live index: 42 nodes, 104 edges, 42/42
  ordered, **acyclic**
* No dependency edge, task count, shipment membership, or unit definition changed — only policy
  and prompt text, prose, and labels

## Validation

| Gate | Result |
|---|---|
| `sync` | 1208 artifacts indexed, 0 parse failures |
| `doctor` | 23 issues, all pre-existing orphans (`106.0xx-T`, `016.001-R`) outside `147-F`; 0 new |
| `docs lint` | `valid: true, violation_count: 0` |
| `scripts/md-lint.ps1` (markdownlint-cli2 0.23.1, repo-wide) | 2284 files, 0 issues, exit 0 |
| `go test ./tests/integration/ -count=1` | `ok` — existing structural prompt/plugin/CI guards pass |
| YAML frontmatter parse (changed prompt artifacts + all `147.*` and `130-S`) | 0 failures |
| Cross-reference existence check on every path named in the new text | all present |
| Harness-manifest checksum probe | only `workflow-policies.md` is checksum-tracked, and it already drifted at HEAD (documented local customization) |
| `query`: queued task count / executable edges / exempt / gate dependents under `147-F` | 42 / 104 / 10 / 3 |
| `query`: root tasks (no queued dependency) | exactly `147.001-T`, `147.032-T` |
| Independent Kahn topological sort | 42/42 ordered, acyclic |

## Open questions and next steps

1. **The independent confirmation review is still outstanding.** Cycle 21's P1 remains
   remediated-but-unconfirmed, and cycle 22's changes are themselves unreviewed. The gate stays
   `FAIL` until that pass runs.
2. Only after that review should the branch be pushed and PR #377 reconciled.
3. No push, merge approval, or shipment claim was performed, and no subagent was invoked.
4. **Deferred by scope**: the repository's prompt/template validation tests are Go
   (`tests/integration/github_skill_parity_test.go`, `plugin_manifest_test.go`). This session was
   scoped to exclude Go source, so no test was added to assert the P-002.1 contract structurally.
   A follow-up may add a structural guard that the policy declares P-002.1/P-002.2, that the Ship
   agent carries Step 2a, and that the closed class vocabulary matches across policy, agent, and
   skill.
5. `plugin/agents/ship.agent.md` and `plugin/skills/` were deliberately not modified. That tree is
   the frozen standalone product bundle with a different owner (see
   `docs/plugin-guide.md` § "Skill locations and drift"), and it carries no `harness-ready`,
   `harness-exempt`, or P-002 vocabulary at all, so it is not a coupled surface for this contract.
