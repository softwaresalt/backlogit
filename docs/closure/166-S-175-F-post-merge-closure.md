---
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
description: "Post-merge closure for 166-S (baseline convergence RS-W09 lint remediation wave 9)."
doc_type: closure
docline:
  date: 2026-10-26T00:50:00Z
  status: final
  tags:
    - closure
    - 166-S
schema_version: "1.0"
shipment_id: 166-S
source: docs/closure/166-S-175-F-post-merge-closure.md
title: "166-S closure"
---

# 166-S closure

Context: feature 175-F (baseline convergence), PR #582, merge `749ee24f5cdc728ad86a5a6fca1231370a79272d`. Members: 175.077 to 175.085 (9 tasks, one wave). Changes: checked `rows.Close`, `database.Close` and telemetry file-close error returns in `internal/db` (`links.go`, `logs.go`, `stash.go`, `duplicates.go`, `gate.go`, `gate_evidence.go`, `schema_gen.go`, `upsert_projection.go`), `internal/events/telemetry.go`, and the matching `internal/db` tests; nine golangci-lint-backed harnesses. No function signature changed (named error returns with `errors.Join` only).

## Releasability

`closure_status: READY`. There is no deployed service, but the changed `internal/db` query helpers back the CLI and MCP read paths, so runtime verification was performed (evidence below). Behaviour change: a failed row close in the touched query helpers now surfaces as an error (a successful scan with a close failure returns the result plus a non-nil error). No unmet condition exists, so no `conditions:` block is required. Monitoring: CI `test` on the next PR to main. Rollback: revert PR #582. Owner: operator. Validation window: next PR to main.

## Runtime verification (validator evidence)

* CLI probe: built `./cmd/backlogit` from synchronized `main` (`749ee24f`); `backlogit queue view --group-by status` and `backlogit stash list` against this workspace both exited 0 with populated output (`QueryQueue`, `ListStashEntries` paths).
* MCP probe: `backlogit_get_queue` with `limit: 3` returned a populated item list.
* Manual checkpoints and blocked prerequisites: none. No automation was faked.

## Compaction status (P-020)

`compaction_status: done`. compact-context ran with `target: all`; top-level `docs/memory/` holds about 36 files and about 108 KB, under the 40-file / 500 KB thresholds, so there were no candidates.

## Evidence

* Local review: single-reviewer adversarial review of `a9a7369b` found P0=0, P1=0, P2=1 (swallowed `file.Close` error in the pre-existing `internal/events/stream.go` lock release), P3=2 (harness skip/vacuity risk; partial result plus close error); outcome `READY_WITH_FOLLOWUPS`. PR HEAD `3f60bc29c5c8f547c831bb8f80fa22f30f565c06` adds only `.backlogit/` state and `docs/memory/` on top of the reviewed code.
* Copilot: review of the final HEAD reported no open issues and no inline threads; P-018 gate SATISFIED before merge. CI all green (test, Windows handle/lock, CLI drift, docline, markdown lint, line endings, topology). Merge used the merge-commit strategy; no admin fallback.
* Tests: build, vet and gofmt clean; unfiltered `go test -p 1 -timeout=30m ./...` green; nine scoped harnesses green.
* Transient: `backlogit_claim_shipment`, six `backlogit_move_item` calls and `backlogit_ship_shipment` timed out client-side. Each completed server-side and was verified by quiescence polling; none was retried.
* Reconcile reports: `.backlogit/reconcile/166-S-pre-20261026T003900.md`, `166-S-safe-close-20261026T004700.md`, `166-S-post-20261026T004800.md`.
* Commit traceability: shipment and all nine member archive records carry `commit: 749ee24f`.

## Source artifact provenance (untouched)

No source stash or deliberation was a shipment member; none mutated.

## Compound refresh

`docs/compound/2026-10-16-errcheck-signature-change-test-callers.md` still applies (no signature changed this wave). No entry is superseded or invalidated.

## Follow-ups

* Stash `913890C2` (P-021 C2 deferred scope expansion): the item-log lock release in `internal/events/stream.go` swallows a `file.Close` error; fixing it needs deliberation (release-func signature).
* The lint-harness cost and skip/vacuity concerns are already tracked by stash `FE7D57F1`; the `scripts/verify-task-lint.ps1` limitation by `C007D571`.
* Remaining errcheck findings in other files belong to later tasks of 175-F.
