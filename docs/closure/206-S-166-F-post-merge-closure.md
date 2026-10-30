---
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
description: "Post-merge closure for shipment 206-S and covering feature 166-F: CLI/MCP gate-error payload parity drift remediation."
doc_type: closure
schema_version: "1.0"
shipment_id: 206-S
feature_id: 166-F
source: docs/closure/206-S-166-F-post-merge-closure.md
title: "206-S / 166-F Post-Merge Closure"
---

# 206-S / 166-F Post-Merge Closure

## Release record

| Field | Value |
|---|---|
| Shipment | `206-S` |
| Covering feature | `166-F` |
| Tasks | `166.001-T` through `166.003-T` (3 tasks, 3 waves, all test-first) |
| Implementation PR | [#625](https://github.com/softwaresalt/backlogit/pull/625) (merge commit) |
| Merge commit | `1e01f341d55b803614789d5f4370ff0703cd368f` |
| Reviewed implementation HEAD | `43828e6b0fc5a3f55b732a2aac1007fff81777ea` |
| Closure branch | `post-merge/206-s-gate-payload-parity` |
| Closure status | `READY` |
| Context compaction | `done` (P-020 `compact-context` invoked) |

## Delivered

* CLI `--json` gate-error payload now carries `remediation` (setup, config) and `retry_after_ms` (timeout, in_progress when greater than 0), matching the MCP `gateClassResult` payload
* `cli.RenderGateErrorJSON`, `cli.GateExitError` and `mcp.GateErrorResult` exported for cross-package parity tests
* Parity comparator `knownDrifts` emptied; the gate-payload drift scenario now fails closed instead of reporting only
* `exitCodeClasses` maps MCP `gate_*` error types to CLI exit classes 6, 7 and 8, with a real cross-surface parity test over every gate-error class

## Gates

| Gate | Result |
|---|---|
| Local review (P-014) | `READY` (0 findings) at `43828e6b` |
| Copilot review (P-018) | `SATISFIED` for the merged HEAD, no threads |
| CI | All required checks passed, including the full `test` job |
| Merge strategy (P-009) | Merge commit |
| Governed closure | `backlogit_ship_shipment` returned a client timeout; completion was confirmed by waiting for quiescence and reading the `shipment_status_changed` (`shipped`), `commit_tracked` and `archived` events for `206-S`, `166-F` and the three tasks. The post-ship `ship_shipment` hook event (`seq` 4279, `active` to `shipped`) was emitted at 2026-10-30T08:44:28Z in the local `.backlogit/hooks_queue.jsonl`, whose working-copy change is carried forward and not part of this PR. `backlogit doctor --check-shipped-event-completeness` reports no finding for `206-S`, `166-F` or `166.001-T` through `166.003-T` (the 41 findings it reports are older shipments). Nothing was retried or synthesized |

## Runtime verification and releasability

Releasability: `READY`. The change is confined to CLI JSON output and parity test tooling, covered by unit and parity tests that ran in CI.

* Monitoring: parity corpus tests run in the standard CI suite
* Rollback: revert the merge commit
* Owner: backlogit maintainers
* Validation window: none required

## Compaction (P-020)

`compact-context` ran with `target: all`. No memory files were moved in this closure; the only new memory file is the cited served-root handoff record.

## Environment notes

* The local `golangci-lint` binary fails to load export data (toolchain mismatch), which fails the `TestU175_*_LintFix` tests locally. CI is unaffected.
* A full local `go test ./...` hung until the 30 minute timeout in `internal/core`, which this release changed only in comments. CI passed.

## Follow-ups

* None
