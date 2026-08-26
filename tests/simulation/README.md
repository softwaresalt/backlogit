# Wave scheduler contract simulation

`wave-scheduler-contract.json` is a tracked, read-only fixture that pins the behaviour of the
**P-002.6 dependency-aware wave scheduler** (`.github/policies/workflow-policies.md`) against the
live backlog manifest of release unit `147-F` / shipment `130-S`.

It exists because the scheduler is a *contract executed by agents*, not compiled code: nothing in
the Go test suite can fail when the contract regresses. The fixture plus its runner make the
contract reproducibly checkable by a human, by CI, or by an agent at a gate point.

## Running it

```pwsh
pwsh -NoProfile -File scripts/wave-scheduler-sim.ps1
```

Add `-VerifyAgainstQueue` to additionally re-derive the manifest from `.backlogit/queue/*.md`
and fail on any drift between the fixture and the live backlog:

```pwsh
pwsh -NoProfile -File scripts/wave-scheduler-sim.ps1 -VerifyAgainstQueue
```

The runner prints one line per scenario and a final `WAVE_SIM_OK: {pass}/{total} assertions PASS`
line, exiting non-zero on any failed assertion. It is **pure**: it reads the fixture (and the queue
Markdown under `-VerifyAgainstQueue`), computes in memory, and writes nothing. It starts no
process, runs no `go` command, and mutates no repository state, so it clears the P-002.5 read-only
command screen and is safe to run at any gate point.

## What it checks

| Scenario | Contract obligation |
|---|---|
| `baseline` | The 42-member / 104-edge DAG partitions into **18** waves with 0 stalls and 0 compile-order violations, on one snapshot per wave |
| `persistent_red_mapping` | `open_red_deliverables` tracks completed red-harness tasks whose green-maker is not terminal; wave 4 advances while U8b and U3c are legitimately red; the unfiltered full suite runs only when the set is empty, and immediately at the wave that empties it |
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
| `non_frozen_m_control` | Negative control: re-deriving `M` each wave lets `return_blocked` shrink the accounting universe and strands the loop |
| `missing_green_maker` | A red deliverable with no declared green-maker fails closed with `WAVE_RED_MAPPING_UNRESOLVED` |
| `ambiguous_green_maker` | A green-maker outside `M` fails closed; it is never resolved by nearest match |
| `green_maker_descoped` | A green-maker `archived` rather than `done` satisfies dependencies but does **not** close its open-red obligation; the deferral budget halts with `WAVE_OPEN_RED_UNCLOSED` |

## Keeping it honest

* The fixture mirrors the live queue. `-VerifyAgainstQueue` is the drift gate: run it whenever the
  manifest, a dependency edge, an exemption label, or a `red-deliverable-contract` block changes.
* Every expectation lives in the fixture, not in the runner. A contract change is a fixture change,
  and it shows up as a diff.
* The runner implements the contract; it does not implement the repository. It proves the wave
  state machine's ordering, accounting, and gate-scope decisions, and deliberately proves nothing
  about Go test outcomes.
