---
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
description: "Post-merge closure for 167-S (baseline convergence RS-W10 lint remediation wave 10)."
doc_type: closure
docline:
  date: 2026-10-26T04:30:00Z
  status: final
  tags:
    - closure
    - 167-S
schema_version: "1.0"
shipment_id: 167-S
source: docs/closure/167-S-175-F-post-merge-closure.md
title: "167-S closure"
---

# 167-S closure

Context: feature 175-F (baseline convergence), PR #584, merge `90504476919940135feb228d853f02aaff46c155`. Members: 175.086 to 175.094 (9 tasks, one wave). Changes: checked `Close` returns in `internal/mcp`, `internal/telemetry`, `tests/contract` and `tests/integration` tests; checked temp-file cleanup in `internal/telemetry/harvest.go` (new helpers join close and remove errors); logged close failures for read-only files in `harvest.go`, `correlator.go` and `session_events.go`; nine golangci-lint-backed harnesses. No function signature changed.

## Releasability

`closure_status: READY`. There is no deployed service, but the touched telemetry code backs the CLI and MCP telemetry harvest path, so runtime verification was performed. Behaviour change: a failed close or remove of the telemetry JSONL temp file now surfaces as a joined error. No unmet condition exists. Monitoring: CI `test` on the next PR to main. Rollback: revert PR #584. Owner: operator. Validation window: next PR to main.

## Runtime verification (validator evidence)

* CLI probe: built `./cmd/backlogit` from synchronized `main` (`90504476`); `backlogit queue view --group-by status` and `backlogit stash list` both exited 0 with populated output.
* MCP probe: `backlogit_get_queue` with `limit: 3` returned a populated item list.
* Manual checkpoints and blocked prerequisites: none. No automation was faked.

## Compaction status (P-020)

`compaction_status: done`. compact-context ran with `target: all`; top-level `docs/memory/` holds 36 files, under the 40-file / 500 KB thresholds, so there were no candidates.

## Evidence

* Local review: code-review of `dad88b70` found P0=0, P1=0 (outcome READY); an adversarial-review pass without shell access gave a provisional READY_WITH_FOLLOWUPS over the production files (P2=1 pre-existing, P3=3). Overall outcome `READY_WITH_FOLLOWUPS`. PR HEAD `5c843f80f382cc044ebcaf58b277d833627e1be3` adds only `.backlogit/` state and `docs/memory/` on top of the reviewed code.
* Copilot: review of the final HEAD reported no open issues and no inline threads, so there was nothing to reply to or resolve; P-018 gate SATISFIED before merge and again at the last mile. CI all green (test, Windows handle/lock, CLI drift, docline, markdown lint, line endings, topology). Merge used the merge-commit strategy; no admin fallback.
* Tests: build, vet and gofmt clean; unfiltered `go test -p 1 -count=1 -timeout=30m ./...` green; nine scoped harnesses red before the fix and green after.
* Transient: `backlogit_claim_shipment`, one `backlogit_move_item` and `backlogit_ship_shipment` timed out client-side. Each completed server-side and was verified by quiescence polling; none was retried.
* Commit traceability: the nine member archive records and the shipment record carry no `commit` field this wave (the move calls were passed `dad88b70` but it was not persisted); the merge SHA is recorded here and in the hook queue.
* Reconcile reports: `.backlogit/reconcile/167-S-pre-20261026T041200.md`, `167-S-safe-close-20261026T042200.md`, `167-S-post-20261026T042400.md`.

## Source artifact provenance (untouched)

No source stash or deliberation was a shipment member; none mutated.

## Compound refresh

`docs/compound/2026-10-16-errcheck-signature-change-test-callers.md` still applies (no signature changed this wave). No entry is superseded or invalidated. New learning recorded in `docs/memory/2026-10-26-167-S-rs-w10-execution.md`: run the full suite with `-count=1`, because a cached run stalled the `go` command for over 30 minutes.

## Follow-ups

* Stash `B1C101D2` (P-021 C2 deferred scope expansion): `writeTelemetryJSONL` removes the destination JSONL before the Windows rename, so a rename failure loses both copies; needs deliberation.
* The lint-harness cost and skip/vacuity concerns are already tracked by stash `FE7D57F1`.
* Remaining errcheck findings in other files (for example `internal/mcp/tools.go`) belong to later tasks of 175-F.
