---
schema_version: "1.0"
doc_type: memory
title: Ship 197-S final-gate halt: unfiltered full suite not clean at HEAD 86928a6f
description: Ship completed all 197-S execution waves and the branch review. The required unfiltered full repository suite at the final HEAD did not exit 0 on two runs: the internal/core package failed intermittently under the full parallel run and passed when run alone. No PR has been opened. Ship halted before PR creation pending an operator decision.
timestamp: "2026-10-10T22:35:00Z"
---

# Ship 197-S final-gate halt

## Status

**HALTED (DARK_MODE_HALTED: failed required local gate; no PR, no merge).** Scope `[197-S]`.
Branch `feat/cx-ship-closure-protocol-and-gate-correctness`, HEAD `86928a6f` (pushed).
`origin/main` `d9d0237b` untouched. No PR exists. Nothing outside 197-S was claimed.

## What the gate found

| Run | HEAD | Result |
|---|---|---|
| Wave 3 unfiltered `go test -timeout=30m ./...` | `0fa2f112` | exit 0, 40 packages ok, zero failing tests. |
| Branch-review run (concurrent with review subagents) | `becfca4e` | exit 1. Failing: `TestU172_LockOrder_ArchiveItemDoesNotHoldArtifactLockWhileWaitingForItemLog`, `TestArchiveItemGovernance_WaitsForGlobalLifecycleLock`. Both PASS in isolation (exit 0 each). |
| Quiet unfiltered `go test -timeout=30m ./...` | `df4a2df9` | exit 1. One package failed (`internal/core`), one panic line, no `--- FAIL` name captured. |
| `go test -count=1 -timeout=30m ./internal/core` alone | `df4a2df9` | exit 0, `ok ... internal/core 907.6s`. |

## Diagnosis (evidence, not a conclusion)

* No Go source under `internal/`, `cmd/`, `go.mod`, or `go.sum` changed between the passing wave-3
  gate (`0fa2f112`) and HEAD (`git diff --name-only 0fa2f112..HEAD -- internal cmd go.mod go.sum` is empty).
  The changes since are documentation, the drift manifest, and one harness test file.
* The `internal/core` package passes alone. The failures occur only in the full parallel run, and
  the two named tests are lock-timing tests. This pattern points to load sensitivity, not to a
  deterministic regression from 197-S.
* The panic was not captured with its test name in the quiet run. Its origin is therefore not proven.
  An unexplained panic cannot be waived by Ship.

## Why Ship stopped

Step 5 item 1 requires an unfiltered full suite at the final HEAD that exits 0 with no open red, and
dark mode halts on failed required local gates. Ship will not waive, retry until green, or treat the
two tests as known flakes: the known flake named in the Ship definition is a different test, and the
panic has no test name.

## Operator decision needed (choose one)

1. Authorise a focused diagnosis of the `internal/core` panic and the two lock-timing tests under
   load (outside 197-S's authorised surfaces, so this needs a new shipment or an explicit scope
   amendment). Then re-run the unfiltered suite until it exits 0 on a quiet workspace.
2. Accept a documented exception for these three tests, recorded as a named limitation in the PR
   readiness record and the closure artifact, after the operator confirms the isolated results above.
3. Hold 197-S. Ship records a checkpoint and returns control to Stage.

## Recorded for resume

* Shipment: 197-S, 18 tasks `done`, M = 18, no open red. All waves converged. Branch-review fix
  cycles are exhausted (3 of 3); residual risks and out-of-scope captures are recorded in
  `docs/memory/2026-10-10/ship-197s-branch-review-complete-memory.md`.
* Next steps after the operator decision: quiet unfiltered full suite at the final HEAD; Local Review
  Readiness record for that HEAD; feature PR (BOM-less body file under `logs/`); Copilot review loop;
  CI; merge only on explicit operator approval (admin fallback not authorised); post-merge closure.
* AC3 (U17) is satisfied at `df4a2df9` for the three manifest-tracked files changed late in the run.
  Re-run it if any of the eight manifest files changes again before merge.
