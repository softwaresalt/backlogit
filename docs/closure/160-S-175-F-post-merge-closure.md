---
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
description: "Post-merge closure and compound refresh for 160-S (baseline convergence RS-W03 lint remediation wave 3)."
doc_type: closure
docline:
  date: 2026-10-23T04:40:00Z
  status: final
  tags:
    - closure
    - compound-refresh
    - 160-S
schema_version: "1.0"
shipment_id: 160-S
source: docs/closure/160-S-175-F-post-merge-closure.md
title: "160-S closure and compound refresh"
---

# 160-S closure and compound refresh

Context: feature 175-F (baseline convergence), PR #568, merge `c866d072f6454c6c37b2093c2f570071fd33c87d`. Members: 175.022-T through 175.030-T (9 tasks, task-only manifest by decomposition decision H6). Changes: handled previously-unchecked `Close` error returns and one tagged-switch refactor across `internal/core`, `internal/db`, `internal/events`, `internal/gateevidence`, and `internal/hooks`, with nine task-scoped AST harnesses.

## Releasability

`closure_status: READY`. No runtime surface, so runtime-verification is not applicable. Monitoring: CI `test` and lint-related checks on the next PR. Rollback: revert PR #568. Owner: operator. Validation window: next PR to main.

## Compaction status (P-020)

`compaction_status: done`. compact-context ran with `target: all`; `docs/memory/` holds 26 files / 99 KB, under the 40-file / 500 KB thresholds, so there were no candidates and nothing was archived.

## Evidence

* Local review: standard review zero findings; 3-reviewer adversarial review of `7c21bbc4` READY_WITH_FOLLOWUPS (P0=0, P1=0, P2=2, P3=2). The later head differed only by backlog-only commits.
* Lint: golangci-lint v2.13.2 shows zero findings in owned files; remaining findings are owned by 161-S..169-S per `docs/decisions/baseline-lint-inventory.json`, except 19 pre-existing findings in untouched files (stash `558861F9`). `task_lint_cmd` is a recorded non-gating TOOL_DEGRADED (stash `C007D571`).
* Copilot: one review iteration with one thread (stale active checkpoint); fixed by resolving the superseded checkpoints, replied, resolved; P-018 gate SATISFIED at `c64040f7`.
* CI: all checks green at `c64040f7`.
* Full suite: `go build`, `go vet`, gofmt clean; `internal/core` 740s ok and all other packages ok with `-count=1`. Note: an unfiltered cached `go test ./...` stalled in the go tool after test binaries finished (no child processes); running uncached per package avoided it.
* Transient: `backlogit_ship_shipment` timed out client-side and was not retried. Completion was verified by quiescence polling and independent evidence, not by the shipped event alone: the item log records `shipment_status_changed` (shipped) 21:27:06, `commit_tracked` (c866d072) 21:27:15 and the server's own shipment `archived` event 21:32:46; the hook queue carries the `ship_shipment` event (seq 3991); all nine member archive records carry commit `c866d072`; and `backlogit doctor --check-shipped-event-completeness` reports no `160-S` finding (the earlier `shipped_unarchived_residue` was a transient in-flight observation made before the server archived). A redundant `backlogit_archive_item` call at 21:33:07 produced a second, harmless `archived` event and `archive_item` hook seq 3992. No operator reconciliation was required.

## Source artifact provenance (untouched)

No source stash or deliberation was a shipment member; none mutated.

## Compound refresh

No entry in `docs/compound/` is superseded or invalidated. `2026-10-06-ship-shipment-mcp-timeout-result-unknown.md` is kept (reproduced again).

## Follow-ups

* Stash `558861F9`: 19 non-inventoried pre-existing lint findings (inventory gap).
* Stash `C007D571`: task-lint verifier FixtureRoot contract defect.
* Stash `3C2B15C4`: lintfix AST harness blind spots (method-value, unused-error forms).
* Stash `8E8CD750` and `69AF7B15`: duplicate captures of the stale implementation-notes predicate, for Stage disposition at next triage.
