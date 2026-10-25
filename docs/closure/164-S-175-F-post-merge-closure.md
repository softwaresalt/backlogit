---
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
description: "Post-merge closure for 164-S (baseline convergence RS-W07 lint remediation wave 7)."
doc_type: closure
docline:
  date: 2026-10-25T16:45:00Z
  status: final
  tags:
    - closure
    - 164-S
schema_version: "1.0"
shipment_id: 164-S
source: docs/closure/164-S-175-F-post-merge-closure.md
title: "164-S closure"
---

# 164-S closure

Context: feature 175-F (baseline convergence), PR #578, merge `f6b8e87fa017c545cfae1af43e98ed7ba0b7f33d`. Members: 175.044, 175.060, 175.061, 175.062, 175.063, 175.064, 175.065, 175.066, 175.067 (9 tasks, one wave). Changes: checked `Close` and `fmt.Fprint*` error returns in `internal/cli/telemetry.go`, `stash_correct.go`, and `internal/core` (`hierarchy.go`, `stash.go`, `commits.go`, `blocking_cascade.go`, `harness_status.go`) plus their tests; nine golangci-lint-backed harnesses. `renderBranchTable` now returns an error and its caller propagates it.

## Releasability

`closure_status: READY`. There is no deployed runtime surface (library and CLI only), so runtime-verification is not applicable. Behaviour change: a failed workspace, row, or file close, or an output write failure, now surfaces as an error. No unmet condition exists, so no `conditions:` block is required. Monitoring: CI `test` on the next PR to main. Rollback: revert PR #578. Owner: operator. Validation window: next PR to main.

## Compaction status (P-020)

`compaction_status: done`. compact-context ran with `target: all`; top-level `docs/memory/` holds 32 files and about 107 KB, under the 40-file / 500 KB thresholds, so there were no candidates.

## Evidence

* Local review: P1 on harnesses hard-failing without golangci-lint v2, fixed in `2f132787` (t.Skip when absent); re-review clean. Copilot thread on capped linter output fixed in `21bd1f15`, replied and resolved. Final PR HEAD `21bd1f15` reviewed `READY` (P0=0, P1=0, P2=0, P3=0).
* Copilot: P-018 gate SATISFIED at the final HEAD. CI all green.
* Tests: unfiltered `go test -timeout=30m ./...` green; build, vet and gofmt clean; golangci-lint v2.13.2 reports no finding in the owned files.
* Transient: several `backlogit_move_item` calls and `backlogit_ship_shipment` timed out client-side. Each completed server-side and was verified by quiescence polling; ship was not retried (archival of the control record finished about 8 minutes after the call).
* Reconcile reports: `.backlogit/reconcile/164-S-pre-20261025T163000.md`, `164-S-safe-close-20261025T163500.md`, `164-S-post-20261025T163600.md`.
* Commit traceability: shipment and all nine member archive records carry `commit: f6b8e87f`.

## Source artifact provenance (untouched)

No source stash or deliberation was a shipment member; none mutated.

## Compound refresh

`docs/compound/2026-10-16-errcheck-signature-change-test-callers.md` still applies (the `renderBranchTable` signature change was handled the same way). No entry is superseded or invalidated.

## Follow-ups

* New stash `FE7D57F1`: lint harnesses are slow (4 to 70 seconds per package) and flaked once when cli and core ran golangci-lint concurrently; share one run per package or retire them.
* `scripts/verify-task-lint.ps1` fails closed without `-FixtureRoot`, so harnesses assert residual counts through golangci-lint v2 (tracked by earlier stash `C007D571`).
* Remaining errcheck findings in other files belong to later tasks of 175-F.
