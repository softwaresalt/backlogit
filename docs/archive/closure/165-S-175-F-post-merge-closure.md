---
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
description: "Post-merge closure for 165-S (baseline convergence RS-W08 lint remediation wave 8)."
doc_type: closure
docline:
  date: 2026-10-25T22:40:00Z
  status: final
  tags:
    - closure
    - 165-S
schema_version: "1.0"
shipment_id: 165-S
source: docs/closure/165-S-175-F-post-merge-closure.md
title: "165-S closure"
---

# 165-S closure

Context: feature 175-F (baseline convergence), PR #580, merge `c826980140ae29c11112df398cf459f213f648d0`. Members: 175.068 to 175.076 (9 tasks, one wave). Changes: checked `rows.Close`, `database.Close`, `ws.Close`, `sqliteDB.Close` and `fmt.Fprint` error returns in `internal/core` (`migrate_links.go`, `naming.go`, `queue.go`, `shipment_reconcile_snapshot.go`, `size_composition.go`), `internal/db/schema.go`, and the matching tests in `internal/core`, `internal/core/gate`, `internal/core/templates`, `internal/db`; nine golangci-lint-backed harnesses. No function signature changed (named error returns only).

## Releasability

`closure_status: READY`. There is no deployed service, but the changed `QueryQueue` and `NextID` helpers back the CLI queue view and the MCP queue handler, so runtime verification was performed on those surfaces (validator evidence below). Behaviour change: a failed row close in the touched query helpers now surfaces as an error. No unmet condition exists, so no `conditions:` block is required. Monitoring: CI `test` on the next PR to main. Rollback: revert PR #580. Owner: operator. Validation window: next PR to main.

## Runtime verification (validator evidence)

* CLI probe: built `./cmd/backlogit` from synchronized `main` (`c8269801`) and ran `backlogit queue view --group-by status` against this workspace; exit 0 and a populated grouped table (`QueryQueue` path).
* MCP probe: `backlogit_get_queue` with `limit: 3` returned a prioritized item list (the handler shares `QueryQueue`).
* Manual checkpoints and blocked prerequisites: none. No automation was faked.

## Compaction status (P-020)

`compaction_status: done`. compact-context ran with `target: all`; top-level `docs/memory/` holds 33 files and about 106 KB, under the 40-file / 500 KB thresholds, so there were no candidates.

## Evidence

* Local review: adversarial review and standard code review of `3b296d63` found P0=0, P1=0, P2=3 (lint-harness cost, vacuity risk, parse fragility), P3=4; outcome `READY_WITH_FOLLOWUPS`. PR HEAD `4d7eb4819059f6875c60a90a7092babe57a430a7` adds only `.backlogit/` state on top of the reviewed code.
* Copilot: review of the final HEAD reported no open issues and no inline threads; P-018 gate SATISFIED before and at merge. CI all green (test, Windows handle/lock, CLI drift, docline, markdown lint, line endings, topology).
* Tests: build, vet and gofmt clean; unfiltered `go test -timeout=30m ./...` green except golangci-lint harnesses (044, 060, 061, 073, 075, 076) that flaked under concurrent lint runs; all pass when run sequentially (`-p 1`).
* Transient: `backlogit_claim_shipment`, several `backlogit_move_item` calls and `backlogit_ship_shipment` timed out client-side. Each completed server-side and was verified by quiescence polling; none was retried (ship archival finished about 7 minutes after the call).
* Reconcile reports: `.backlogit/reconcile/165-S-pre-20261025T222900.md`, `165-S-safe-close-20261025T223300.md`, `165-S-post-20261025T223400.md`.
* Commit traceability: shipment and all nine member archive records carry `commit: c826980`.

## Source artifact provenance (untouched)

No source stash or deliberation was a shipment member; none mutated.

## Compound refresh

`docs/compound/2026-10-16-errcheck-signature-change-test-callers.md` still applies (no signature changed this wave). No entry is superseded or invalidated.

## Follow-ups

* No new stash entries. The lint-harness cost and concurrent-run flake (also raised by this wave's reviews) are already tracked by stash `FE7D57F1`; the `scripts/verify-task-lint.ps1` limitation by `C007D571`.
* Remaining errcheck findings in other files belong to later tasks of 175-F.
