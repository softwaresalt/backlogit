# Wave scheduler contract simulation

`wave-scheduler-contract.json` is a tracked, read-only fixture that pins the behaviour of the
**P-002.6 dependency-aware wave scheduler** (`.github/policies/workflow-policies.md`) against
shipment `130-S` and its live backlog artifacts. The shipment contains **44** explicit members;
the scheduler's `M` is exactly its **43 task-type IDs**. The excluded non-task report is
`147-F=feature`. The explicit non-shipment fallback is the same closed 43-ID task set; retired
archived sibling `147.010-T` is in neither source and can never enter a snapshot.

It exists because the scheduler is a *contract executed by agents*, not compiled code: nothing in
the Go test suite can fail when the contract regresses. The fixture plus its runner make the
contract reproducibly checkable by a human, by CI, or by an agent at a gate point.

## Running it

```pwsh
pwsh -NoProfile -File scripts/wave-scheduler-sim.ps1
```

Add `-VerifyAgainstQueue` to parse `.backlogit/queue/130-S.md`,
`.backlogit/config.yaml` status values, and `.autoharness/backlog-registry.yaml` status
mapping/features; resolve every listed artifact's type; filter task IDs into `M`; exact-compare
manifest `M` with the explicit fallback set; report excluded non-task IDs; and fail on drift:

```pwsh
pwsh -NoProfile -File scripts/wave-scheduler-sim.ps1 -VerifyAgainstQueue
```

Verification compares the exact filtered and fallback task-ID sets, configured status sources,
statuses, dependencies, exemption metadata, the optional green-regression projection, and **all
five** canonical red-deliverable keys:
`red_deliverable`, `red_deliverable_reason`, `red_selector_command`, `green_maker_tasks`, and
`green_maker_closes_wave`. It then runs 13 fixture-declared mutation checks in memory, proving
that task/exclusion drift, status-catalog drift, registry mapping/feature drift, archived sibling
inclusion, every red-contract-key mutation, and a non-empty green-regression mutation are detected.
Three parser controls also require a green-regression payload to be a JSON object whose
`green_regression_cmds` value is an array.

The runner prints one line per scenario and a final `WAVE_SIM_OK: {pass}/{total} assertions PASS`
line, exiting non-zero on any failed assertion. It is **pure**: it reads the fixture and backlog
Markdown, computes and mutates copies in memory, and writes nothing. It starts no process, runs no
`go` command, and mutates no repository state, so it clears the P-002.5 read-only command screen
and is safe to run at any gate point.

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

## Keeping it honest

* The fixture mirrors the real shipment projection. `-VerifyAgainstQueue` is the drift gate: run
  it whenever shipment/fallback membership, a status source, a member type, a dependency edge, an
  exemption label, a `red-deliverable-contract`, or a `green-regression-contract` changes.
* Every expectation lives in the fixture, not in the runner. A contract change is a fixture change,
  and it shows up as a diff.
* The runner implements the contract; it does not implement the repository. It proves the wave
  state machine's ordering, accounting, and gate-scope decisions, and deliberately proves nothing
  about Go test outcomes.
