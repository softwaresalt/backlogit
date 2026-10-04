---
title: "Ship 196-S wave 1 U3 red-deliverable complete"
description: "Verified U3 assertion-RED evidence and corrected lint/format gates."
doc_type: memory
schema_version: "1.0"
---

# Ship 196-S wave 1 U3 red-deliverable complete

## U3 evidence

- Task: `196.003-T`; shipment scope remains only `196-S`.
- R14 `pragma_database_list` was repeated immediately before reading U3's
  item log. The no-follow read found no start comment before append; the MCP
  append succeeded; the no-follow re-read verified exactly one
  `WORK_STARTED: 196-S` in the latest claim epoch.
- Clean red-deliverable baseline: `7d3f06bdaa833970aacf2e94c83ffa0dc6fadcdd`.
- `go test -run=^$ -count=1 ./...`: PASS.
- `go test -count=1 -v -run '^TestUSR3_' ./tests/integration`: assertion-RED
  as declared. `SkillLiterals`, `ProceedAndShipStep6`, and
  `SupersededAndPreserved` failed as specified.
- Three-pass zero-delta check against the recorded baseline was empty:
  unstaged tracked, staged tracked, and untracked.
- CI-pinned lint:
  `GOTOOLCHAIN=local go run github.com/golangci/golangci-lint/cmd/golangci-lint@v1.64.8 run --timeout=5m`
  exited 0 with zero findings.
- Local golangci-lint v2.13.2 no-new-debt check against merge-base
  `2741626afdd92e16262c365c689a54545e298f6e` exited 0 with zero new findings.
- LF-canonical `git archive` extraction under ignored `logs/` followed by
  `gofmt -l .` returned no files.
- `green_regression_cmds` is the frozen empty array `[]`.
- Step 0.5c evidence declares green-maker `196.005-T`, scheduled to close the
  U3 red selector in wave 2.
- The tracked harness commit is `413dff42`; the red-deliverable task required
  no task-local source commit. `196.003-T` is now `done`, associated with
  `413dff42`.

## Next

Continue wave 1 on the same branch with `196.004-T` and `196.007-T`
(`verification-only` exemptions) and `196.008-T` (red deliverable). Preserve
the contracts, claim-time gates, and remaining open-red entries. Do not
advance the wave until every wave-1 member converges.
