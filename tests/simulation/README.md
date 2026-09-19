# Wave scheduler contract simulation

`wave-scheduler-contract.json` is a tracked, read-only fixture that pins the behaviour of the
**P-002.6 dependency-aware wave scheduler** (`.github/policies/workflow-policies.md`). The primary
scheduler corpus is shipment `130-S`: **44** explicit members and exactly **43 task-type IDs** in
`M`. Queue-backed verification also projects baseline-convergence shipment `156-S` dynamically:
the release-unit feature and executable task set come from the live shipment. The simulator
derives the exact role partition, dependency edge set/count, unique source/sink, reachability,
acyclic Kahn wave decomposition, and terminal coverage, then exact-compares every value with the
canonical feature contract.

It exists because the scheduler is a *contract executed by agents*, not compiled code: nothing in
the Go test suite can fail when the contract regresses. The fixture plus its runner make the
contract reproducibly checkable by a human, by CI, or by an agent at a gate point.

## Running it

```pwsh
pwsh -NoProfile -File scripts/wave-scheduler-sim.ps1
pwsh -NoProfile -File tests/simulation/lint-runtime-tests.ps1
```

Add `-VerifyAgainstQueue` to parse `.backlogit/queue/130-S.md` and
`.backlogit/queue/156-S.md`,
`.backlogit/config.yaml` status values, and `.autoharness/backlog-registry.yaml` status
mapping/features; resolve every listed artifact's type; filter task IDs into `M`; exact-compare
manifest `M` with the explicit fallback set; report excluded non-task IDs; and fail on drift:

```pwsh
pwsh -NoProfile -File scripts/wave-scheduler-sim.ps1 -VerifyAgainstQueue
```

When a live `baseline-lint-convergence-contract` exists, also pass its operator-supplied opaque
record identity as `-OperatorAuthorizationRecord <record>`. The repository block cannot
authenticate itself. Until Stage authors the current block and machine inventory on the shipment's
release-unit feature, `-VerifyAgainstQueue` intentionally fails its Stage-contract-presence or
contract-validity assertion; that preclaim failure is the executable readiness blocker, not a
harness simulation failure to waive.

Verification compares the exact filtered and fallback task-ID sets, configured status sources,
statuses, dependencies, exemption metadata, the optional green-regression projection, and **all
five** canonical red-deliverable keys:
`red_deliverable`, `red_deliverable_reason`, `red_selector_command`, `green_maker_tasks`, and
`green_maker_closes_wave`. It then runs 13 fixture-declared mutation checks in memory, proving
that task/exclusion drift, status-catalog drift, registry mapping/feature drift, archived sibling
inclusion, every red-contract-key mutation, and a non-empty green-regression mutation are detected.
Three parser controls also require a green-regression payload to be a JSON object whose
`green_regression_cmds` value is an array, and 18 red-deliverable branch controls pin
`build-feature` Step 0.5 routing and result classification (see *Red-deliverable branch controls*
below). Additional controls parse task-lint contracts, reject missing/vacuous/native-failure-masked
commands, accept one exact owned file or a small explicitly enumerated set of owned files and
ordinal finding identities, exercise strict and explicit baseline-convergence lint modes,
exact-compare intermediate residual finding identities, require zero-warning global lint at the
terminal boundary, and validate the exact governed claim-path union before and after a
verification-only commit.

The runner prints one line per scenario and a final `WAVE_SIM_OK: {pass}/{total} assertions PASS`
line, exiting non-zero on any failed assertion. Assertion totals are derived from the fixture and
optional live projection rather than pinned here. It is **read-only**: it reads the fixture and
backlog Markdown, computes and mutates copies in memory, and writes nothing. It runs no `go`
command; when a live
baseline-convergence block exists, it uses only read-only Git/file hashing to verify that the
machine inventory is committed and digest-bound.

## Baseline lint and claim controls

`lint_mode` defaults to `strict`, where unmodified `golangci-lint run` must be green at each wave.
The simulator accepts `baseline-convergence` only through the canonical release-unit JSON block.
Its `member_scope` must exactly equal every executable shipment task. Exactly one
`baseline-control` task is the unique source, one or more `finding-remediation` tasks each own at
least one exact inventory finding, zero or more `support` tasks own no inventory findings, and
exactly one `terminal-convergence` task is the unique sink. Control and terminal tasks also own no
findings. Support tasks remain exact digest/status/task-lint members of the release unit, may have
dependencies, and may gate terminal convergence. The graph must be acyclic, every edge must stay
inside the member set, and every member—including support—must be reachable from control and able
to reach terminal. Generic controls cover a valid connected support chain plus support ownership,
missing support task lint, hidden/off-catalog support, disconnected support, support without a path
to terminal, missing members, duplicate roles, cycles, multiple sinks, and incomplete terminal
coverage.
At intermediate waves the observed normalized repository-wide finding set must equal exactly the
committed baseline rows owned by unfinished `finding-remediation` tasks. Support status cannot
remove or retain a finding; an explicit negative control rejects residual allowance derived from
support completion. New, missing, moved, changed-linter, malformed, or unowned findings fail. At
the declared terminal task, the residual must be empty and the frozen supported-platform terminal
command must pass.

The block's intermediate command is a byte-exact invocation of
`scripts/verify-baseline-lint.ps1`, binding the inventory path and SHA-256 plus shipment, feature,
and terminal task. The verifier resolves tasks across queue/archive by lifecycle, rejects
duplicates and status/location mismatches, binds immutable canonical task-lint JSON digests,
requires golangci-lint v2.13.2, validates the machine-readable finding schema, preserves native and
parser failures, and emits a success marker only after exact Windows and Linux residual equality.
The primary inventory is Windows-scoped; Linux exclusions and Linux-only rows are explicit, so a
Windows host is never described as covering Unix-tagged files. Terminal convergence uses
`scripts/verify-terminal-lint.ps1` and requires zero findings for both GOOS values; GitHub CI runs
the pinned Linux surface natively.

Every `task_lint_cmd` is the short canonical
`pwsh -NoProfile -File scripts/verify-task-lint.ps1 -TaskId <task> -FeatureId <feature>` form;
inline PowerShell programs are rejected. A no-Go task requires explicit `-Phase Schema`,
`-Phase PreCommit`, or `-Phase PostCommit` at the corresponding workflow gate; the unsuffixed
stored command cannot pass its completion gate. The package set must equal the owned-file directory
package set, and native `go list` under each supported GOOS determines file participation. Across
those packages, the task runner permits only exact inventory identities owned by unfinished other
tasks to remain, preserving later slices without admitting new, moved, linter-changed, message-changed,
or terminal-owner findings. For a harness-exempt task, `governed_claim_delta` is not an allowlist. Its task/event paths must be
registry-derived, its before/after digests and status/event transitions must validate, and the
pre-commit working set must equal the task-owned deliverable union with that governed claim delta.
Build-feature invokes the runner with `-Phase PreCommit`. Ship invokes it with
`-Phase PostCommit`; the commit must equal the task-owned set, the working set must equal the
governed claim set, and their combined union must remain exact. The unsuffixed command performs
schema screening only. Omitted claim paths, uncommitted deliverables, and extra admitted paths are
negative controls.
Queue-backed topology controls mutate a member count, role partition, edge, edge count, source,
sink, wave, and terminal coverage; each mutation must be rejected.

## Canonical Stage adoption schema

Each member keeps its existing structured `lint_scope` for
`bounded-finding-set-go-lint` or `harness-file-scoped-go-lint`, but replaces copied program text
with:

```json
{
  "task_lint_cmd": "pwsh -NoProfile -File scripts/verify-task-lint.ps1 -TaskId 900.001-T -FeatureId 900-F",
  "lint_scope": {},
  "non_vacuity_evidence": {
    "success_marker": "TASK-LINT-OK:900.001-T:finding-remediation"
  }
}
```

The shown empty `lint_scope` is a placeholder for the task's real closed structured scope, not a
valid value. A terminal verification-only member uses exactly:

```json
{
  "kind": "no-go-lint-surface",
  "owned_paths": ["docs/closure/example.md"],
  "governed_claim_paths": [
    ".backlogit/queue/900.005-T.md",
    ".backlogit/hooks_queue.jsonl"
  ]
}
```

Each `member_scope` row binds `task_contract_sha256`, computed over the compact canonical JSON of
that task's complete `task-lint-contract`. The baseline inventory has this exact top-level shape:

```json
{
  "primary_goos": "windows",
  "supported_goos": ["windows", "linux"],
  "findings": [],
  "surface_exclusions": {
    "windows": [],
    "linux": ["path|line|column|linter|message"]
  },
  "additional_findings": [
    {
      "goos": "linux",
      "path": "path",
      "line": 1,
      "column": 1,
      "linter": "errcheck",
      "message": "message",
      "owner_task_id": "900.002-T"
    }
  ]
}
```

`findings` is the primary Windows set. `surface_exclusions.linux` lists only primary identities
absent on Linux; `additional_findings` lists only identities absent from the primary set. The
feature's `baseline_inventory.finding_count` is the union count. The feature also declares
`supported_goos`, exact `topology`, the canonical intermediate command including `-FeatureId`, and
the canonical terminal command described above.

## What it checks

| Scenario | Contract obligation |
|---|---|
| `baseline` | The 43-task `M` / 106-edge DAG partitions into **18** waves with 0 stalls and 0 compile-order violations, on one logical exact-`M` snapshot per wave |
| `persistent_red_mapping` | `open_red_deliverables` tracks completed red-harness tasks whose green-maker is not terminal; wave 4 advances while U8b and U3c are legitimately red; every entry still open after a wave's completions — including carried-in entries — is re-run and re-confirmed RED at that wave's gate, and every entry closed at that gate is re-run and confirmed GREEN; the unfiltered full suite runs only when the set is empty, and immediately at the wave that empties it |
| `blocked_injection` | A `blocked` member halts with `WAVE_MEMBER_BLOCKED`, reports transitive dependency impact, stays in `M`, and never permits a completion claim |
| `blocked_mid_run` | The same halt fires at the next admission when a member is blocked mid-run |
| `active_residual` | An `active` leftover claim halts with `WAVE_NO_PROGRESS` (detail `active residual`) |
| `unsupported_status_review` | A configured-catalog token that is neither executable nor terminal-success halts with `WAVE_STATUS_UNSUPPORTED` |
| `unsupported_status_abandoned` | A non-completion terminal (`abandoned`) never false-completes |
| `unsupported_status_off_catalog` | A token outside the catalog entirely halts rather than being read as success |
| `status_catalog_unavailable` | An unreadable or empty configured catalog halts with `WAVE_STATUS_CATALOG_UNAVAILABLE` before any admission |
| `status_catalog_disagrees` | A configured token absent from the workspace catalog is a disagreement, not a widening |
| `cycle_injection` | A dependency cycle halts at schedule construction with `WAVE_CYCLE_DETECTED` and a cycle path |
| `sibling_red_wave4` | The withdrawn repo-wide per-task gate progresses 0 of wave 4's 5 non-exempt members; the task-scoped gate progresses 5, with 0 full-suite runs inside a per-task loop |
| `non_frozen_m_control` | Negative control: re-deriving `M` each wave lets `return_blocked` shrink the accounting universe and strands the loop; the positive assertion exact-compares manifest `M` with the explicit fallback set |
| `missing_green_maker` | A red deliverable with no declared green-maker fails closed with `WAVE_RED_MAPPING_UNRESOLVED` |
| `ambiguous_green_maker` | A green-maker outside `M` fails closed; it is never resolved by nearest match |
| `missing_red_selector` | An empty selector fails closed; scope is never inferred |
| `wrong_green_maker_close_wave` | A close-wave value that differs from the actual last green-maker wave fails closed |
| `green_maker_descoped` | A green-maker `archived` rather than `done` satisfies dependencies but does **not** close its open-red obligation; the deferral budget halts with `WAVE_OPEN_RED_UNCLOSED` |
| `open_red_early_green_carried_in` | Negative control for the open-red RED re-confirmation: an entry carried in from an earlier wave is injected green three waves before its declared green-maker, and the gate halts with `WAVE_RED_DELIVERABLE_EARLY_GREEN` at the wave that observes it rather than advancing |
| `open_red_closed_entry_not_reconfirmed` | Complement of the control above: an entry the wave **closed** leaves the still-open set and is re-confirmed **GREEN** rather than RED, so the same injection is the expected outcome and the schedule completes — proving RED re-confirmation covers exactly the still-open set and no more |
| `green_maker_lands_but_selector_stays_red` | Negative control for the newly-closed verification: a green-maker completes but its entry's selector keeps failing. Because another open red defers the full suite for six more waves, the gate must catch it itself and halts with `WAVE_GREEN_MAKER_UNVERIFIED` |

## Red-deliverable branch controls

The scenarios above pin the **scheduler**. The `red_deliverable_branch_controls` block pins the
**per-task execution** of a red deliverable — `build-feature` Step 0.5 — which the scheduler
scenarios cannot reach, because a wave schedule says nothing about how one dispatch is classified.

Step 0.5 **consumes** a harness `harness-architect` already scaffolded with the wave at Ship
Step 4.0 item 10; it never lands one and never writes. Each control declares an observation record
(dispatch inputs, the Ship-supplied post-scaffolding `red_baseline_sha`, compile state, selector
signal, changed files, evidence completeness) and the branch outcome Step 0.5 requires; the runner
classifies it in the same order the skill specifies. The delta rule is **zero files**, measured
against `red_baseline_sha` — captured after the scaffolding commit, so a sibling's harness cannot
enter this task's delta — and the set is the union of three passes: tracked-unstaged, tracked-staged,
and **untracked**. The third matters most: `git diff` reports only files Git already tracks, so a
newly created file would otherwise satisfy an empty-delta claim.

| Control | Contract obligation |
|---|---|
| `accepted-assertion-red` | The deliverable: the scaffolded harness compiles, the anchored selector fails on named assertions, the delta is empty, evidence complete |
| `not-red-deliverable-uses-generic-loop` | Routing: an ordinary dispatch still enters the generic loop, so Step 0.5 does not capture tasks it does not own |
| `red-deliverable-never-enters-generic-loop` | **Load-bearing control.** The exact observation the generic loop reads as SUCCESS must halt here with `WAVE_RED_DELIVERABLE_EARLY_GREEN` and never route to the loop. Removing the branch fails this control on both assertions |
| `early-green-at-dispatch` | Step 0.5a: a scaffolded selector already passing before any green-maker closed halts with `WAVE_RED_DELIVERABLE_EARLY_GREEN` |
| `harness-not-scaffolded-under-selector` | Step 0.5a: a no-tests-to-run signal means no test matches the declared selector — `WAVE_RED_DELIVERABLE_VACUOUS`, the mirror of the P-002.3 false-green rule |
| `uncompilable-scaffolded-harness` | Step 0.5a item 1: a harness that does not compile contradicts the P-002 compiling-but-failing contract and returns to `harness-architect` |
| `panic-is-not-assertion-red` | Step 0.5a: a panic aborts the package with no matching `--- FAIL:` line, so it is rejected rather than accepted |
| `timeout-is-not-assertion-red` | Step 0.5a: a timeout is likewise a non-zero exit that proves no assertion failed |
| `production-delta-refused` | Step 0.5b: the branch writes nothing, and a non-test `*.go` file is the production change a red deliverable may never make |
| `extra-test-file-refused` | Step 0.5b: a `*_test.go` file is still a write — the permitted delta is empty, so a test extension is not an exemption |
| `extra-non-go-file-refused` | Step 0.5b: a configuration or documentation file is a write too |
| `untracked-test-file-refused` | Step 0.5b: a **newly created** `*_test.go` file is invisible to `git diff`, so the untracked pass is what makes the empty-delta claim true |
| `untracked-production-file-refused` | Step 0.5b: a newly created non-test `*.go` file is caught by the same pass and keeps its distinct production label |
| `missing-baseline-sha-refused` | Precondition 3 fails closed: without the Ship-supplied post-scaffolding baseline the zero-delta gate would measure a self-chosen range |
| `exempt-pairing-refused` | Precondition 1: `red_deliverable` and `harness-exempt` are mutually exclusive — `WAVE_RED_MAPPING_UNRESOLVED` |
| `selector-mismatch-refused` | Precondition 2: `harness_cmd` must be the declared `red_selector_command` verbatim |
| `weakened-selector-refused` | Precondition 2: a bare `./...` with no `-count=1`, no anchored selector, and a `\|\| true` suffix is a contract defect, never a substitute command |
| `incomplete-evidence-report-refused` | Step 0.5c: without the report Ship cannot build the `open_red_deliverables` entry that convergence items 4 and 5 re-confirm |

## Keeping it honest

* The fixture mirrors the primary scheduler shipment. For baseline convergence,
  `-VerifyAgainstQueue` derives authority from the canonical feature block and live shipment
  instead of mirroring its topology. Run it whenever shipment/fallback membership, a status source,
  a member type, a dependency edge, an exemption label, a `red-deliverable-contract`, a
  `green-regression-contract`, a `task-lint-contract`, or a release-unit
  `baseline-lint-convergence-contract` changes.
* Every expectation lives in the fixture, not in the runner. A contract change is a fixture change,
  and it shows up as a diff.
* The runner implements the contract; it does not implement the repository. It proves the wave
  state machine's ordering, accounting, and gate-scope decisions, and deliberately proves nothing
  about Go test outcomes.
