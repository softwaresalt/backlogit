---
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
description: "Post-merge closure for shipment 205-S and covering feature 204-F: single backlogit MCP daemon per workspace with mcp status, mcp stop and --replace."
doc_type: closure
schema_version: "1.0"
shipment_id: 205-S
feature_id: 204-F
source: docs/closure/205-S-204-F-post-merge-closure.md
title: "205-S / 204-F Post-Merge Closure"
---

# 205-S / 204-F Post-Merge Closure

## Release record

| Field | Value |
|---|---|
| Shipment | `205-S` |
| Covering feature | `204-F` |
| Tasks | `204.001-T` through `204.005-T` (5 tasks, 5 waves; `204.005-T` is `harness-exempt` docs-only) |
| Implementation PR | [#620](https://github.com/softwaresalt/backlogit/pull/620) (merge commit) |
| Merge commit | `a76cf37b57a5e8eb410ccf7561257a7f60fb15d7` |
| Reviewed implementation HEAD | `de8665b06137fc158599d5da339351276a0c4d5c` |
| Closure branch | `post-merge/205-s-single-daemon-per-workspace` |
| Closure status | `READY` |
| Context compaction | `done` (P-020 `compact-context` invoked) |

## Delivered

* Exclusive per-workspace runtime lock with a metadata sidecar for `backlogit mcp`, so a second daemon for the same workspace is refused
* New `backlogit mcp status` and `backlogit mcp stop` commands, and a `--replace` flag on `backlogit mcp`
* Verified termination of the lock holder on Windows and Linux (process identity checks before signalling); other platforms refuse to terminate
* Sidecar hardening from review: reject non-regular sidecar entries, require recorded workspace, storage and lock paths to match this workspace by filesystem identity, and do not fall back to `.backlog` when `BACKLOGIT_WORKSPACE_DIR` is set explicitly
* Regenerated CLI reference pages for `mcp`, `mcp status` and `mcp stop`

## Gates

| Gate | Result |
|---|---|
| Exempt gate (`204.005-T`) | Probe failed before generation; `EXEMPT_VERIFY_OK:204.005-T` afterwards |
| Local review (P-014) | `READY_WITH_FOLLOWUPS` at `e8f72a2c` (P3 1, stashed as `A045DBA4`); `READY` (0 findings) on both fix deltas, final HEAD `de8665b0` |
| Copilot review (P-018) | `SATISFIED`; six threads across two rounds fixed, replied to and resolved |
| CI | All required checks passed for the merged HEAD |
| Merge strategy (P-009) | Merge commit |
| Governed closure | `backlogit_ship_shipment` returned a client timeout; completion was confirmed by waiting for quiescence and reading the `shipment_status_changed` (`shipped`), `commit_tracked` and `archived` log events. The shipped-event completeness check does not flag `205-S` |

## Runtime verification and releasability

Releasability: `READY`. The change adds CLI behaviour for the MCP daemon; it is covered by unit, subprocess and CLI tests that ran in CI.

* Monitoring: the daemon lock and CLI tests run in the standard CI suite, including the Windows handle and lock job
* Rollback: revert the merge commit
* Owner: backlogit maintainers
* Validation window: none required

## Compaction (P-020)

`compact-context` ran with `target: all`. Of 38 age-eligible memory files, 26 are cited from durable docs. The other 12 are already-compacted summaries or two unrelated `199-S` session files, so nothing was moved in this closure.

## Environment notes

* The local `golangci-lint` binary fails to load export data (toolchain mismatch), which fails the `TestU175_*_LintFix` tests locally. CI is unaffected.
* A full local `go test ./...` timed out at 30 minutes in `internal/core` (`TestMigrateCustomFieldLinks_*`), a package this release did not change. CI passed.

## Follow-ups

* Stash `A045DBA4` (P3): `mcp status` and `mcp stop` probe by taking the exclusive daemon lock, so a concurrent `backlogit mcp` start can fail spuriously
* `.backlogit/logs/204.004-T.jsonl` carries a duplicate `WORK_STARTED: 205-S` from an MCP retry; accepted by the operator and left untouched
