---
title: "Ship 154-S resumed: wave 1 red-deliverable progress"
date: "2026-09-30"
agent: ship
shipment_id: 154-S
feature_id: 173-F
branch: feat/154-s-shipment-claim-scheduler-baseline-marker
status: active
---

## Resume and intake

- Resumed the operator-selected checkpoint
  `.backlogit/checkpoints/checkpoint-20260930-035203.json`, confirmed it was
  valid, conforming, Ship-owned, and matched shipment `154-S`, feature `173-F`,
  task set, and branch. Resolved only that selected checkpoint after restoring
  the session. The earlier checkpoint's HEAD hint predated Stage's
  `f0b891ab` contract amendment; current execution uses the branch's live state.
- The active shipment is `154-S` on
  `feat/154-s-shipment-claim-scheduler-baseline-marker`. The only active
  top-level feature is its covering feature `173-F`; there are no active chores
  or other active shipments. The worktree was clean at resume.
- Backlog tool availability passed; index synchronization returned
  `INDEX_SYNC_OK`. Engram was reachable, bound to this repository, and reported
  a fresh source graph. Graphtor-docs was reachable and reported 3 sources /
  197,739 chunks. Agent-intercom is not installed.
- `go test -run=^$ -count=1 ./...` passed at the resumed branch state.
  `scripts/wave-scheduler-sim.ps1 -VerifyAgainstQueue` returned
  `WAVE_SIM_OK: 186/186`.

## Frozen schedule

- Shipment manifest `S` has eight explicit members: seven task members plus
  covering feature `173-F`. Frozen task set `M` has seven members:
  `173.006-T`, `173.007-T`, `173.008-T`, `173.009-T`, `173.001-T`,
  `173.003-T`, and `173.005-T`. Excluded member: `173-F` (`feature`).
- Live workspace status catalog contains ten values:
  `queued`, `active`, `blocked`, `review`, `done`, `accepted`, `rejected`,
  `archived`, `shipped`, and `abandoned`. Registry mapping contains
  `queued`, `active`, `done`, and `blocked`; executable statuses are
  `queued`/`active`/`blocked`, terminal-success statuses are `done`/`archived`,
  and all remaining values are unsupported.
- The seven-member DAG is acyclic and orders into five waves:
  1. `173.006-T`, `173.007-T`, `173.008-T`
  2. `173.009-T`
  3. `173.001-T`
  4. `173.003-T`
  5. `173.005-T`
- All seven tasks remain `active` from the approved 154-S bootstrap snapshot.
  Apply the shipment's recorded conditional bootstrap exception narrowly: it
  bypasses only the active-residual halt. It does not waive dependency order,
  harness gates, convergence, review, CI, Copilot review, or merge approval.
- Red-deliverable contracts remain: `173.006-T` and `173.007-T` close in wave 3
  via green-maker `173.001-T`; `173.008-T` closes in wave 2 via
  `173.009-T`. `173.003-T` now has the canonical wave-4 command
  `go test -count=1 -race ./internal/core/... ./internal/cli/... ./internal/mcp/... ./internal/db/...`.
  No declared red deliverable remains open at wave 4 if its green-makers finish.

## Wave 1 task 173.006-T

- Reviewed the wave-one harnesses. A false hierarchy-path delta in U0a's
  shipment equality assertion and vacuous custom-fields checks in U0b's CLI/MCP
  characterizations were classified as same-contract-surface P-021 C1 findings.
  Harness-architect corrected only the three test files; no production code
  changed. The characterizations pass, while all declared marker selectors
  remain assertion-RED.
- Harness correction commit:
  `269a7fbd6a9b1794dda44adad96b984b97831fea`
  (`test: correct claim marker harness assertions (173.006-T)`).
- For `173.006-T`, corrected baseline was
  `269a7fbd6a9b1794dda44adad96b984b97831fea`. The exact selector
  `go test -count=1 -run '^TestClaimMarkerRed_' ./internal/core` exited 1 on
  named assertions in `TestClaimMarkerRed_ClaimMarksQueuedMembers` and
  `TestClaimMarkerRed_CascadeOrderingMarksParent`; the prior hierarchy-path
  assertion no longer fails. Compile-only check passed and the tracked,
  staged, and untracked zero-delta set was empty.
- `173.006-T` was moved to `done` as a red-deliverable task. Its task comment
  records the lint correction and red evidence; its original harness commit
  remains tracked and the harness correction commit is associated with it.
- The failed PATH lint result was caused by local golangci-lint v2.13.2 using a
  different default set than CI's pinned v1.64.8. The corrected, authoritative
  command
  `go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8 run --timeout=10m`
  passed with zero findings. This is a gate-command correction, not a waiver or
  lint cleanup; Stage re-characterized existing stash `AD5AECAF` as tooling
  work. LF-normalized gofmt checked 867 tracked Go files with zero unformatted
  paths.
- Report-only review findings about the expected missing marker/recovery
  behavior are the scheduled REDs, not implementation defects to fix in wave 1.
  Harness assertion corrections were in scope; no new deferred-scope stash was
  captured. Older checkpoint and memory files remain timestamped historical
  snapshots; this note and the live `173.003-T` contract reflect the amended
  schedule.

## Current state / next steps

- At this checkpoint, wave 1 task `173.006-T` is done; `173.007-T` and
  `173.008-T` remain. No wave convergence, implementation, PR, CI, Copilot
  review, or merge has occurred.
- Continue only with wave-one members on the same branch. Then run the
  convergence gate: repo-wide compile, vet, pinned lint, LF-normalized format,
  all three scoped red selectors, and explicit `FULL_SUITE_DEFERRED` because
  the open-red set is non-empty. Keep circuit breakers across all waves.
- Continue waves 2–5 in the frozen order, run the 173.003-T `-race` regression
  command in wave 4, and discharge the unfiltered full suite at the first
  convergence gate with no open red.
- Do not create a branch/worktree per wave. Before PR preparation, run
  `go build ./cmd/backlogit`, the full final quality gates, current-HEAD local
  review, CI, and the Copilot review gate. Stop before merge; explicit operator
  approval is required.
