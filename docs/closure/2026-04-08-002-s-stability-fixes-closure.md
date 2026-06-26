---
chunk_strategy: h1-h2-h3
description: 'Post-merge closure record for shipment 002-S and PR #11.'
doc_type: closure
docline:
    author: Copilot
    estimated_reading_time: 4
    keywords:
        - closure
        - shipment
        - 002-s
        - stability
        - stash-rehydration
        - upsert-tx
        - completion-scope
    ms.date: 2026-04-08T00:00:00Z
    ms.topic: reference
ingested_at: "2026-06-26T02:32:32Z"
schema_version: "1.0"
source: docs/closure/2026-04-08-002-s-stability-fixes-closure.md
title: 002-S stability fixes closure
---

## Post-merge closure

Post-merge closure for shipment `002-S` (Shipment Stability Fixes) and PR `#11`.

| Field | Value |
|---|---|
| Feature | `017-F` |
| Shipment | `002-S` |
| Branch | `ship/002-s-stability-fixes` |
| PR | `#11` |
| Merge commit | `311c82fd39d00678fd6e3784b36888fff479739f` |
| Result | `READY` |

## Change summary

Four correctness issues surfaced during post-F015 stabilization were addressed:

| Task | Area | Change |
|---|---|---|
| `015.009-T` | `internal/db` | Added `UpsertItemsTx` — variadic atomic multi-artifact write within a caller-provided `*sql.Tx`; nil guard prevents panic on nil pointer |
| `015.010-T` | `internal/db` | Fixed `rehydrateStash` to treat `stash.jsonl` as authoritative only when non-empty; empty JSONL (created by `backlogit init`) falls back to `.stash.md` so legacy entries are not silently dropped |
| `015.011-T` | `internal/mcp` | Regression harness for `domainError` error classification — no production change needed; existing sentinel routing was already correct |
| `017.012-T` | `internal/core` | Introduced `ReconcileCompletionScope` — cascades done status to all queued/active descendants of a done parent, with max-depth guard (10) and `slog.Debug` observability |

## CI status

| Check | Outcome |
|---|---|
| `test (1.23)` | passed |
| `test (1.24)` | passed |
| `golangci-lint` | clean (local gate) |
| `go vet` | clean (local gate) |

All 14 packages passed `go test ./...`. 26 new tests added across `internal/db`, `internal/core`, and `internal/mcp`.

## Unresolved review items

Copilot review generated 6 inline comments; 5 were applied and 1 was intentionally declined.

| # | File | Resolution |
|---|---|---|
| C1 | `internal/db/rehydration.go` | Applied — `f.Close()` error now captured and returned |
| C2 | `internal/db/upsert_tx_test.go` | Applied — stale "does not exist" header removed |
| C3 | `internal/core/completion_scope_test.go` | Applied — stale harness-only header removed |
| C4 | `internal/core/completion_scope_test.go` | Declined — `bldb.UpsertItem` bypass is intentional; `setArtifactStatus` would be overridden by `cascadePersistedParentStatuses`, making the inconsistent precondition impossible to construct |
| C5 | `internal/db/upsert_tx_test.go` | Applied — `init()` compile guard replaced with package-level `var _` |
| C6 | `internal/core/shipment_atomic_test.go` | Applied — comment corrected to reflect compensating-rollback implementation |

No P0 or P1 findings remain open.

## Healthy signals

* `go test ./...` passes on both Go 1.23 and 1.24 after merge.
* `backlogit sync` rebuilds the SQLite index cleanly from Markdown files.
* Stash rehydration correctly prefers `stash.jsonl` when non-empty and falls back to `.stash.md` on empty or absent JSONL.
* `UpsertItemsTx` commits atomically and rolls back cleanly on transaction abort.
* `ReconcileCompletionScope` promotes all queued/active children to done without exceeding the depth guard.

## Failure signals

* Stash entries disappear after `backlogit sync` on a workspace that has both `.stash.md` and a non-empty `stash.jsonl` — indicates a rehydration regression; verify `rehydrateStash` JSONL-first logic.
* Items remain queued after parent is explicitly set to done and `ReconcileCompletionScope` is called — indicates depth guard or parent-status cascade override; check `cascadePersistedParentStatuses` interaction.
* DB row count diverges from Markdown file count after a multi-artifact write — indicates `UpsertItemsTx` is not being used for multi-artifact paths; audit callers.

## Monitoring plan

* Run `go test ./internal/db/... ./internal/core/...` after any change to rehydration, upsert, or status reconciliation code paths.
* After workspace init on a legacy repo, verify stash entry count matches `.stash.md` before and after first `backlogit sync`.
* Review `slog.Debug` output from `ReconcileCompletionScope` when completion cascade produces unexpected parent status transitions.

## Rollback trigger

Roll back to `main@132c7bf` (pre-shipment) if any of the following occur within the validation window:

* Stash entries are silently lost during rehydration on a workspace with both `.stash.md` and `stash.jsonl`.
* A multi-artifact `UpsertItemsTx` write partially commits (item visible without matching shipment update or vice versa).
* `ReconcileCompletionScope` recurses beyond expected depth or produces incorrect final status on a valid hierarchy.

Rollback method: `git revert 311c82fd` on `main`. If additional follow-up commits must also be unwound, use coordinated `git revert` commits for the affected range rather than rewriting branch history.

## Validation window

72 hours from merge (`2026-04-08` to `2026-04-11`). No special deploy steps required — this is a library and CLI change with no persistent service component.

## Owner

softwaresalt

## Follow-up work

The following items are noted for future consideration but are not blockers:

* `UpsertItemsTx` is not yet wired into `ReturnBlockedItem` production path; `ReturnBlockedItem` still uses compensating rollback via two independent `UpsertItem` calls. Wiring `UpsertItemsTx` here would eliminate the compensation logic entirely.
* `ReconcileCompletionScope` triggers `cascadePersistedParentStatuses` per child, which can produce transient spurious JSONL events on the parent during reconciliation. Final state is always correct but the intermediate events are noise. Addressed properly by refactoring the status update pipeline to batch cascades.
* On-disk child status assertions after `ReconcileCompletionScope` (noted by Copilot review C4 as future improvement).
