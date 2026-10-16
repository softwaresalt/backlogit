---
title: "154-S wave 1 converged with declared open-red deferral"
date: "2026-09-30"
agent: ship
shipment_id: 154-S
feature_id: 173-F
branch: feat/154-s-shipment-claim-scheduler-baseline-marker
wave: 1
status: active
---

## Wave convergence

- Re-read `173.006-T`: `done`. `173.007-T` and `173.008-T` also reached
  `done` through their governed move operations. All three wave-1 members are
  terminal-success; none is active, blocked, or unsupported.
- Always-on gates passed:
  - `go test -run=^$ -count=1 ./...` — repo-wide compile.
  - `go vet ./...`.
  - `go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8 run --timeout=10m`
    — CI-pinned lint, zero findings.
  - LF-normalized gofmt — 867 tracked Go files checked, zero format differences;
    `.copilot/session-state` excluded.
- The closed wave command set was executed. The three declared red-deliverable
  selectors remain assertion-RED on their named functions:
  1. Owner `173.006-T`: `go test -count=1 -run '^TestClaimMarkerRed_' ./internal/core`.
     `TestClaimMarkerRed_ClaimMarksQueuedMembers` and
     `TestClaimMarkerRed_CascadeOrderingMarksParent` fail on the absent or
     incorrect scheduler marker. The earlier `HierarchyPath` false assertion
     has been corrected and no longer fails.
  2. Owner `173.007-T`:
     `go test -count=1 -run '^TestClaimMarkerReadSurface_(CLI|MCP)$/^RED_' ./internal/cli ./internal/mcp`.
     Both `RED_cli_recipe_reads_marker` and `RED_mcp_recipe_reads_marker` fail
     on the absent marker.
  3. Owner `173.008-T`:
     `go test -count=1 -run '^TestClaimMarkerRecoveryRed_' ./internal/core`.
     `TestClaimMarkerRecoveryRed_CrashAfterMarkingRollsBack` and
     `TestClaimMarkerRecoveryRed_DoubleFaultPartialCompensationConverges` fail
     on the expected marker/recovery conflict assertions.
- Each wave member's `green_regression_cmds` is exactly `[]`; no green
  regression command was inferred or added.
- The open-red set is these three declared selectors. No selector has closed at
  this gate, so there are no newly closed selectors to verify GREEN. All open
  entries remain within their declared closure budget:
  - `.008` closes via `173.009-T`, scheduled wave 2.
  - `.006` and `.007` close via `173.001-T`, scheduled wave 3.
- **FULL_SUITE_DEFERRED: wave 1.** Stable open-red inventory:
  - `^TestClaimMarkerRed_` — owner `173.006-T`; green-maker `173.001-T`
    (wave 3).
  - `^TestClaimMarkerReadSurface_(CLI|MCP)$/^RED_` — owner `173.007-T`;
    green-maker `173.001-T` (wave 3).
  - `^TestClaimMarkerRecoveryRed_` — owner `173.008-T`; green-maker
    `173.009-T` (wave 2).

Compile, vet, pinned lint, format, and every declared scoped command were run
and passed according to its contract: normal commands passed and all three
still-open red-deliverable selectors were run and re-confirmed RED. The only
failing selectors in the repository are the declared open-red deliverables,
whose green-makers are scheduled at waves 2 and 3. A full run here would be
classified rather than verified; classification can silently absorb a real
failure when a package containing an open red aborts on a build error, panic,
or timeout. The unfiltered full suite is deferred to the first convergence gate
where the open-red set is empty.

The pinned lint command is authoritative because local PATH golangci-lint
v2.13.2 has a different default linter set from CI's pinned v1.64.8. The exact
correction and rationale citing `AD5AECAF` are recorded in the `.006`, `.007`,
and `.008` task comments and their task memory records; this is not a waiver.

## Next step

Admit wave 2 only after a fresh exact-`M` snapshot confirms the expected state
and dependency frontier. The only expected wave-2 task is `173.009-T`; its
declared scoped command and its implementation contract drive `.008` RED to
GREEN. Keep `.006` and `.007` open until wave 3. Continue on the same branch
and worktree; do not run the full suite until the open-red set is empty.
