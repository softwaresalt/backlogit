# 122-S Ship Session Memory

## Summary

Executed the full Ship lifecycle for shipment 122-S (Administrative checkpoint
disposition, feature 136-F) in dark-factory mode. Feature PR #342 merged
(5b963353), post-merge runtime verification caught a genuine relative-`--cwd`
regression, fixed via same-session PR #343 (04d14fbe), then completed
post-merge closure (archival, runtime re-verification, closure docs, compound
learning).

## Task IDs Completed

- 136-F, 136.001-T through 136.013-T (archived, status=done, commit=04d14fbe — the final main tip including both PR #342 and the PR #343 fix-forward, not the intermediate pre-fix merge commit)
- 122-S shipment (archived, status=done)

## New Backlog Follow-Ups (queued, not part of this shipment)

- 136.014-T — TOCTOU classify-then-move race in QuarantineCheckpoint
- 136.015-T — moveNoReplace discards unwind error on double-failure branch
- 136.016-T — quarantine move doesn't respect durable_writes config flag

## Files Modified (feature PR #342)

- internal/core/checkpoint_disposition.go, checkpoint_target.go, checkpoint_audit.go (new)
- internal/errors/checkpoint_errors.go (new)
- internal/events/checkpoint_schema.go, checkpoint_lifecycle.go, checkpoint_disposition_types.go (new)
- internal/cli/checkpoint.go (abandon/quarantine subcommands)
- internal/mcp/tools.go, errors.go (abandon/quarantine tools + error mapping)
- .autoharness/backlog-registry.yaml (governed operation rows + parity fixtures)
- docs/design-docs/checkpoint-administrative-disposition.md (new)
- .github/instructions/backlogit.instructions.md (updated)
- Extensive test coverage across all packages

## Files Modified (bugfix PR #343)

- internal/core/checkpoint_target.go (filepath.Abs fix)
- internal/core/checkpoint_target_test.go (regression test)

## Decisions and Rationale

- **Review-fix cycle limit exceeded (5 rounds vs 3 documented)**: each round's
  findings were genuine (not manufactured), decreasing in severity. Rounds
  4-5 findings (TOCTOU race, unwind error discard, durable_writes gap) were
  accepted as backlog per protocol once the limit was clearly exceeded,
  with explicit decline-with-rationale replies on each review thread before
  resolving.
- **Fix-forward for the relative-cwd regression was NOT deferred to backlog**:
  unlike the review-fix-cycle nits, this broke the feature's primary
  intended usage (CLI invoked with a relative --cwd) with zero unit-test
  coverage of the failure mode. Treated as a release-blocking correctness
  bug, fixed immediately via a dedicated PR with full TDD (confirmed RED
  before fix, GREEN after) rather than deferred.
- Chose `os.Link` + `os.Remove` (atomic no-replace move) over `os.Stat` +
  `os.Rename` for the quarantine clobber-refuse guard, closing a TOCTOU race
  a reviewer identified.
- Used `atomicfile.WriteFileAtomic` (existing repo primitive) instead of a
  hand-rolled temp-then-rename helper for the abandon rewrite and quarantine
  sidecar write, fixing a Windows-specific rename-cannot-replace-existing bug.

## Failed Approaches / Blind Alleys

- Initial `ResolveDispositionTarget` implementation joined a relative
  candidate path directly (not via `filepath.Abs`) before passing it to
  `confineToStorageRoot`, corrupting containment for relative workspace
  roots — caught only by post-merge runtime verification with a real
  compiled binary and relative `--cwd`, not by any unit test (all unit tests
  used `t.TempDir()`, which is always absolute). See compound learning
  `2026-08-10-path-confinement-helper-reuse-relative-workspace-root-double-join.md`.

## Open Questions / Next Steps

- 136.014-T, 136.015-T, 136.016-T remain queued for future pickup.
- Post-merge closure PR (this branch) still needs: commit, push, PR, Copilot
  review, CI, merge.
- `C:\Tools\backlogit.exe` (this repo's own dogfooded pinned binary) does not
  yet include these commands — documented as residual exposure, consistent
  with prior compound learning on self-hosted CLI version skew.

