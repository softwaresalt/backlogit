---
title: "Ship 196-S wave 1 U1 red-deliverable complete"
description: "Resume and corrected lint/format gate evidence for 196.001-T."
doc_type: memory
schema_version: "1.0"
---

# Ship 196-S wave 1 U1 red-deliverable complete

## Resume

- Shipment scope remains only `196-S`.
- Resumed the operator-selected Ship checkpoint
  `checkpoint-20261004-205741.json` from branch
  `feat/196-s-195-s-follow-up-orchestrator-served-root-handoff-and-explicit-feature-reconcile-contract`
  at `60b01b1081d87540b18d5e74489e628704f29363`.
- Re-ran the R14 served-root attestation. The metadata catalog reported the
  supplied workspace and storage roots, and the read-only
  `pragma_database_list` query returned exactly one `main` row at the served
  storage root's direct-child `backlogit.db`.
- A no-follow, reparse-rejecting read verified exactly one
  `WORK_STARTED: 196-S` comment in U1's latest claim epoch. The event schema
  uses `event_type` (not `type`); after correcting the local reader's event
  field lookup, the check passed.
- The selected checkpoint was resolved after the successful resume and its
  resolution was committed as `5f7222bb`
  (`chore(backlog): resolve 196-S lint recovery checkpoint`).

## U1 evidence

- Clean red-deliverable baseline after the checkpoint-resolution commit:
  `5f7222bb0682d2427868d942879fa34278f1e5f7`.
- `go test -run=^$ -count=1 ./...`: PASS.
- `go test -count=1 -v -run '^TestUSR1_' ./tests/integration`: assertion-RED
  as declared. `ProcedureLiterals` and `CallSites` failed; `CrossReferenceInvariant`
  passed. The selector was re-confirmed RED at Step 0.5d.
- Three-pass zero-delta check against the recorded baseline was empty:
  unstaged tracked, staged tracked, and untracked.
- CI-pinned lint:
  `GOTOOLCHAIN=local go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8 run --timeout=5m`
  exited 0 with zero findings.
- Local golangci-lint v2.13.2 no-new-debt check against merge-base
  `2741626afdd92e16262c365c689a54545e298f6e` exited 0 with zero new findings
  in the three changed Go files.
- LF-canonical `git archive` extraction under ignored `logs/` followed by
  `gofmt -l .` returned no files.
- `green_regression_cmds` is the frozen empty array `[]`.
- Step 0.5c evidence declares green-maker `196.002-T`, scheduled to close the
  U1 red selector in wave 2.
- The tracked harness commit is `413dff42`; the red-deliverable task required
  no task-local source commit. `196.001-T` is now `done`, associated with
  `413dff42`.

## Next

Continue wave 1 on the same branch with the remaining ready members:
`196.003-T`, `196.004-T`, `196.007-T`, and `196.008-T`. Preserve each task's
Amendment 2 command and exemption/red-deliverable contract; do not advance the
wave until the wave-convergence gate passes.
