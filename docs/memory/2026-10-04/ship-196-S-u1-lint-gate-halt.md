# Ship 196-S U1 Lint-Gate Halt

## Halt

- Gate result: `LINT_GATE_FAILED` (Step 0.5d does not define a more specific canonical halt token).
- Shipment: `196-S`; task: `196.001-T`.
- Branch: `feat/196-s-195-s-follow-up-orchestrator-served-root-handoff-and-explicit-feature-reconcile-contract`.
- HEAD at gate failure: `ba559ce975fe1b4d7acca8a285dde870a29997a4`.
- The selected recovery checkpoint `checkpoint-20261004-204623.json` was successfully resumed and resolved only after the corrected no-follow reader verified exactly one `WORK_STARTED: 196-S` in the latest U1 claim epoch. The R14 per-task `pragma_database_list` check passed before the read. The clean pre-dispatch baseline was recaptured as `ba559ce975fe1b4d7acca8a285dde870a29997a4`.

## Build-feature Step 0.5 evidence

- Repo compile command `go test -run=^$ -count=1 ./...`: passed, exit 0.
- Declared U1 selector `go test -count=1 -v -run '^TestUSR1_' ./tests/integration`: assertion-RED, exit 1. `ProcedureLiterals` and `CallSites` failed as intended; `CrossReferenceInvariant` passed.
- All three zero-delta passes against the supplied baseline were empty: unstaged tracked, staged tracked, and untracked.
- The first Step 0.5d gate, `golangci-lint run`, failed with 57 diagnostics: 50 `errcheck`, 7 `staticcheck`. The listed diagnostics were in existing CLI, core, DB, hooks, telemetry, canonical, gate-evidence, and contract-test files; none were in the new U1 harness. No code or task files were changed. The command returned exit 1.
- Per build-feature Step 0.5d, a red-deliverable dispatch does not iterate on quality-gate failure. The skill stopped at lint; format and later Step 0.5d checks were not run.
- P-005 telemetry records the gate failure. No task completion, status move, commit, PR, or additional task dispatch occurred. `196.001-T` remains active and the shipment remains active.

## Why Ship stopped

The existing lint diagnostics are outside U1's authorized contract surface. Fixing them would expand scope; P-021 and the shipment boundary do not authorize that work. The red-deliverable branch explicitly forbids a fix iteration, while the required lint gate did not pass. Ship therefore cannot mark U1 complete or advance the wave. No deferred-scope stash was created: this is a repository-wide pre-existing quality-gate failure, not a new change finding in a review or CI-fix loop.

## Resume requirements

This checkpoint records a completed U1 red-deliverable validation attempt that is blocked at Step 0.5d. Do not mark U1 done, advance the wave, rerun the red-deliverable branch as a fix iteration, or fix unrelated lint findings in this shipment. Resume only after an authorized Stage/operator decision provides a compliant path for the required lint gate without widening the 196-S scope or weakening the gate. The historical red baseline above predates this halt record; if a later approved path permits a fresh dispatch, verify a clean tree and use the correct current baseline prescribed by that path.
