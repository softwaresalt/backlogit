---
title: "210-S PR 638 Linux verification halt"
doc_type: memory
schema_version: "1.0"
source: docs/memory/2026-11-01/ship-210-s-pr638-linux-verification-halt.md
description: "Resume state after Linux core verification and Windows full-suite stall."
chunk_strategy: h1-h2-h3
docline:
  shipment_id: 210-S
  feature_id: 209-F
  status: blocked_verification
---

# PR 638 verification halt

Shipment `210-S` remains active on
`feat/210-s-declarative-multi-shipment-manifest-reconstitution-209-f`.
PR #638 remains open. Local HEAD at inspection was
`8ff9f2f3b1a38cb85a2709da2492fedf95908204`, and remote PR HEAD was
`f43d967f7affffde05daf533fd6e590f56041793`. No commit, push, thread reply,
thread resolution, or merge was performed. Preserve all existing dirty and
untracked files, including `.backlogit/stash.jsonl`.

The confirmed Ship checkpoint `checkpoint-20261101-170817.json` was loaded.
Its review-blocked state and the Stage decision for `7E14915F` / `078-DL`
were retained. The bounded rollback guard fix was delegated to Go Engineer;
the requested targeted `TestReview`, `TestRound3`, `TestReconstitute`, and
`TestLegacyReconstitution` suite passed.

## Served-root handoff

Workspace-relative roots: workspace `.`, storage `.backlogit`, index
`.backlogit/backlogit.db`, manifest `.backlogit/queue/210-S.md`. Live metadata
and the index database path matched the supplied roots. The shipment was
`active`, updated at `2026-10-31T06:59:28.7708084Z`, with ordered members
`209-F`, `209.001-T`, `209.002-T`, `209.003-T`. The Orchestrator reported
Windows no-follow handle verification of the manifest with no reparse
components; this session did not independently repeat that raw read.

## Gate evidence

* Linux `internal/db` cross-compiled test binary: PASS under Ubuntu-26.04 with
  `TMPDIR=/tmp`, `TEMP=/tmp`, `TMP=/tmp`, and `GOTMPDIR=/tmp`.
* Linux `internal/core` cross-compiled test binary: FAIL. Fourteen Git-gate
  tests encountered Git's mount-boundary diagnostic because `/tmp` is a
  separate mount and the bounded probe's minimal environment removes
  `GIT_DISCOVERY_ACROSS_FILESYSTEM`; the no-repository marker did not match.
  `TestWorkspaceLiteralGuard_NoHardcodedLegacyStorageRoot` also failed because
  the cross-compiled binary recorded a Windows source path. A compiler
  source-path remapping fixed the latter in a targeted smoke test, but the
  Git-gate failure persisted. Neither run is a full core PASS.
* Windows `go test -timeout=30m ./...`: INCOMPLETE. The serialized Go driver
  consumed CPU for over 35 minutes without a test child or package-level
  progress; the session was stopped. Earlier cached results and stopped runs
  are not final test evidence.
* Windows `go vet ./...`: PASS.
* Windows `golangci-lint run --timeout=5m`: PASS.
* Literal `gofmt -l .`: PASS, zero output lines.
* Windows `go build -o <temporary path> ./cmd/backlogit`: PASS. The temporary
  build and Linux test binaries created for this session were removed.

## Next steps

Return the Linux gate issue and stalled Windows suite to the operator for
disposition. The Git-gate issue concerns a separate pre-existing contract
surface; do not broaden shipment remediation or mark it fixed. No deferred
stash capture was made because the operator explicitly required the stash to
remain untouched; the finding remains open. Re-run the required unfiltered
tests, report-only review, runtime/closure verification, current-HEAD CI and
P-014/P-018 gates before representing PR #638 as ready. Explicit operator
approval is required before any merge.
