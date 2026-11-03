---
title: "175-F wave closure summary (159-S to 169-S, compacted)"
chunk_strategy: h1-h2-h3
compaction_status: done
description: "Compacted closure summary for eight 175-F baseline-convergence shipments (159-S, 160-S, 162-S, 165-S, 166-S, 167-S, 168-S, 169-S). Originals archived under docs/archive/closure/ by P-020 compact-context on 2026-11-03."
doc_type: closure
feature_id: 175-F
schema_version: "1.0"
source: docs/closure/2026-11-03-175-f-wave-closure-summary.md
---

## Scope and result

This summary was written by the P-020 `compact-context` run (`target: all`) on 2026-11-03. It consolidates the post-merge closure records for eight shipments under feature 175-F (repository baseline convergence). Feature 175-F is archived with status done.

None of the eight source records is cited by a backlog item, checkpoint, stash entry, plan, decision, or compound entry. Each original moved to `docs/archive/closure/` with `git mv`, under the same file name. Nothing was deleted, and no original was edited.

Every record reports `closure_status: READY` and `compaction_status: done` in its own front matter.

## Shipment map

| Shipment | Scope | PR | Merge commit |
|---|---|---|---|
| 159-S | RS-W02 errcheck remediation and U14 line-endings workflow (175.011, 175.013 to 175.021) | #536 | 28060960737a26ae653be79f5e3ba2019d2b1799 |
| 160-S | RS-W03 lint remediation (175.022 to 175.030) | #568 | c866d072f6454c6c37b2093c2f570071fd33c87d |
| 162-S | RS-W05 lint remediation (175.041, 175.042, 175.045 to 175.051) | #573 | caa245db4920c4ea7dfbdb652a261a18a34a2bd6 |
| 165-S | RS-W08 lint remediation (175.068 to 175.076) | #580 | c826980140ae29c11112df398cf459f213f648d0 |
| 166-S | RS-W09 lint remediation (175.077 to 175.085) | #582 | 749ee24f5cdc728ad86a5a6fca1231370a79272d |
| 167-S | RS-W10 lint remediation (175.086 to 175.094) | #584 | 90504476919940135feb228d853f02aaff46c155 |
| 168-S | RS-W11 Linux-surface lint remediation (175.095 to 175.098) | #587 | 5717a20d5289ec8fd345ef67f666247691f164e4 |
| 169-S | RS-W12 U12 terminal convergence verification (175.012) | #589 | 1832f1945efe30cb44769c290c78bab5e0d39f0d |

## Per-shipment record

### 159-S

* Changes: checked `Close` and `fmt.Fprint*` error returns in `internal/cli` and `internal/core`. Added the always-run `line-endings.yml` workflow (U14) and task-scoped harnesses.
* Review: three-reviewer adversarial review of `e2b75e44` returned READY_WITH_FOLLOWUPS (P2=2, P3=5). Copilot raised three threads, all fixed and resolved, plus one review-body finding fixed. P-018 gate satisfied at `905383c6`.
* Partial scope: `migrate.go` slice 1 of 2 retired 10 of 19 flagged calls. Nine remain for a later slice.
* Transient: the backlogit MCP claim, five task moves, and ship timed out client-side (about 60 seconds) and converged server-side. Verified by polling; no CLI retry.
* Monitoring: the `Line endings guard` workflow on every PR and push to main. Rollback: revert PR #536.
* Follow-up: stash `3C2B15C4` (deferred scope expansion, P-021 C2).

### 160-S

* Changes: checked `Close` error returns and one tagged-switch refactor across `internal/core`, `internal/db`, `internal/events`, `internal/gateevidence`, and `internal/hooks`. Nine AST harnesses.
* Review: standard review found zero issues. Adversarial review of `7c21bbc4` returned READY_WITH_FOLLOWUPS (P2=2, P3=2). Copilot raised one thread, fixed by resolving the superseded active checkpoints. P-018 gate satisfied at `c64040f7`.
* Lint: zero findings in owned files. Nineteen pre-existing findings in untouched files are tracked by stash `558861F9`. `task_lint_cmd` is a recorded non-gating `TOOL_DEGRADED` (stash `C007D571`).
* Transient: the `backlogit_ship_shipment` call timed out client-side and completed server-side. Completion was verified from the item log (`shipment_status_changed`, `commit_tracked`, server `archived` event), the hook queue, and clean `doctor` output. No retry and no operator reconciliation were needed.
* Follow-ups: `558861F9`, `C007D571`, `3C2B15C4`, `8E8CD750`, `69AF7B15`. The last two are duplicate captures of the stale implementation-notes predicate, held for Stage triage.

### 162-S

* Changes: checked `fmt.Fprint*` and `Close` error returns in `internal/cli`. Fourteen QF1012 rewrites in `internal/telemetry/reporter.go`. Nine AST harnesses. A review fix keeps `migrate` rehydrating after a failed summary write, with a regression test.
* Behavior: a failed write of a success message after a committed side effect now returns an error.
* Scope decision: a scope over-fix touched nine `telemetry.go` occurrences owned by slices 175.043 and 175.044. It was reverted in `5a8e6769` under P-021.
* Review: a P1 harness false-pass was fixed in `9ac957bf` and re-reviewed clean. A Copilot finding was fixed in `8cc487b0`. P-018 gate satisfied at the last mile.
* Transient: `backlogit_claim_shipment`, nine `backlogit_move_item` calls, and `backlogit_ship_shipment` timed out client-side under load. Each completed server-side and was verified by quiescence polling. Ship was not retried.
* Test note: `internal/core` hit the 30-minute timeout in the concurrent full run under machine load. It passed in isolation (2095 s).
* Follow-ups: the stash entries carried from earlier waves (`558861F9`, `C007D571`, `3C2B15C4`, `8E8CD750`, `69AF7B15`, `D8391636`, `3FF72EB8`). `autoharness gate closure-evidence` fails only at `close_path` (known posture, stash `3FF72EB8`), and no `close_path` was fabricated. A residual P2 asks for the AST walkers to be consolidated later.

### 165-S

* Changes: checked `rows.Close`, `database.Close`, `ws.Close`, `sqliteDB.Close`, and `fmt.Fprint` returns in `internal/core`, `internal/db`, and their tests. Nine harnesses. No function signature changed.
* Behavior: a failed row close in the touched query helpers now surfaces as an error.
* Runtime verification: built `./cmd/backlogit` from synchronized `main` (`c8269801`). `backlogit queue view --group-by status` exited 0 with populated output (the `QueryQueue` path). `backlogit_get_queue` with `limit: 3` returned items.
* Review: adversarial and standard review of `3b296d63` returned READY_WITH_FOLLOWUPS (P2=3, P3=4). Copilot had no open threads at the final HEAD.
* Transient: `backlogit_claim_shipment`, several `backlogit_move_item` calls, and `backlogit_ship_shipment` timed out client-side. Each completed server-side and was verified by quiescence polling. None was retried. Ship archival finished about 7 minutes after the call.
* Test note: six lint-harness packages flaked under concurrent lint runs and pass when run with `-p 1`. Tracked by stash `FE7D57F1`.
* Follow-ups: `FE7D57F1` (lint-harness cost and flake), `C007D571` (task-lint verifier limitation).

### 166-S

* Changes: checked `rows.Close`, `database.Close`, and telemetry file-close returns in `internal/db` and `internal/events/telemetry.go`. Nine harnesses. No signature changed, with named error returns joined by `errors.Join`.
* Behavior: a successful scan followed by a close failure returns the result plus a non-nil error.
* Runtime verification: CLI `queue view --group-by status` and `stash list` exited 0 with populated output. MCP `backlogit_get_queue` returned items.
* Review: single-reviewer adversarial review of `a9a7369b` returned READY_WITH_FOLLOWUPS (P2=1, P3=2). The P2 is a swallowed `file.Close` error in the pre-existing item-log lock release in `internal/events/stream.go`.
* Transient: `backlogit_claim_shipment`, six `backlogit_move_item` calls, and `backlogit_ship_shipment` timed out client-side. Each completed server-side and was verified by quiescence polling. None was retried.
* Follow-ups: stash `913890C2` (deferred scope expansion; the release-func signature needs deliberation), `FE7D57F1`, and `C007D571`.

### 167-S

* Changes: checked `Close` returns in `internal/mcp`, `internal/telemetry`, and the contract and integration tests. Temp-file cleanup in `harvest.go` now joins close and remove errors. Read-only close failures are logged in `harvest.go`, `correlator.go`, and `session_events.go`. Nine harnesses.
* Runtime verification: CLI `queue view` and `stash list` exited 0. MCP `backlogit_get_queue` returned items.
* Review: code review of `dad88b70` found P0=0 and P1=0. An adversarial pass without a shell returned READY_WITH_FOLLOWUPS (P2=1 pre-existing, P3=3). Copilot had no open threads.
* Transient: `backlogit_claim_shipment`, one `backlogit_move_item`, and `backlogit_ship_shipment` timed out client-side. Each completed server-side and was verified by quiescence polling. None was retried.
* Traceability gap: the nine member archive records and the shipment record carry no `commit` field this wave. The merge SHA is recorded here and in the hook queue.
* Learning: run the full suite with `-count=1`. A cached run stalled the `go` command for over 30 minutes.
* Follow-ups: `B1C101D2` (deferred scope expansion; `writeTelemetryJSONL` removes the destination before the Windows rename, so a rename failure loses both copies, and needs deliberation) and `FE7D57F1`.

### 168-S

* Changes: checked `Close` returns in the Linux-only files `internal/core/shipment_reconcile_{append,evidence,fs,lock,snapshot}_unix.go` and `internal/events/item_log_lock_unix.go`, joined with `errors.Join`. Four harnesses run `golangci-lint` with `GOOS=linux` and carry no build tag.
* Behavior: a failed directory or file close now surfaces as an error. An acquired lock is released when the directory close fails.
* Limit: these Unix-only paths cannot run on the Windows host. They are covered by cross-compile (`GOOS=linux go vet` and `go build`), Linux errcheck, and CI.
* Review: single-reviewer adversarial review of `7f643580` returned READY_WITH_FOLLOWUPS (P2=3, P3=4). The in-scope P2 (double sentinel on close-after-failure) was fixed in `06b25f4c`. Ship ran vet, build, gofmt, and the full suite because the re-review agent had no shell.
* Transient: the `backlogit_ship_shipment` call timed out client-side and completed server-side. Its archive record carries `archived_status: shipped` and the merge commit.
* Test note: lint-harness packages run with `-p 1` because concurrent `golangci-lint` runs exit with status 3 (stash `FE7D57F1`).
* Follow-ups: `4503EF93` (deferred scope expansion: close-error behavioral tests, harness positive control, pre-existing `_ =` discards). Operator stash `760670A3` (single backlogit daemon per workspace) was carried forward untouched. Twenty-one backlogit processes were observed during closure.

### 169-S

* Changes: the evidence artifact `docs/closure/175-baseline-convergence-convergence-evidence.md` (six gate markers and the 43-row NOLINT inventory). Stage contract amendment `469ae8a7`. No Go source changed.
* Verification: the exempt verification command exited 0 (`EXEMPT_VERIFY_OK:175.012-T`). Task lint passed (`TASK-LINT-OK:175.012-T:terminal-convergence`). No runtime surface changed, so runtime verification is not applicable.
* Review: single-reviewer adversarial review of `a0837abf` returned READY_WITH_FOLLOWUPS (P0=0, P1=0, P2=1, P3=1). Ship re-verified the P2 items. Copilot P-018 gate satisfied at the last mile.
* Transient: `backlogit_ship_shipment` timed out client-side and completed server-side. A CLI retry then returned `shipment status conflict`, which confirmed completion. The archive record carries `archived_status: shipped` and the merge commit. This is the only shipment in the wave whose record shows a retry after a ship timeout.
* Feature completion: all 101 child tasks of 175-F are archived with `archived_status: done`. The feature was moved to done and archived as a separate explicit step.
* Monitoring: CI `test` on the next PR to main. Rollback: revert PR #589.
* Follow-up: stash `03211AD7` (load-sensitive flake in `internal/evidencedag`, heartbeat test). The full suite takes about 50 minutes serially. Shipment 149-S is unblocked.

## Archive map

| Original path | Archived path |
|---|---|
| `docs/closure/159-S-175-F-post-merge-closure.md` | `docs/archive/closure/159-S-175-F-post-merge-closure.md` |
| `docs/closure/160-S-175-F-post-merge-closure.md` | `docs/archive/closure/160-S-175-F-post-merge-closure.md` |
| `docs/closure/162-S-175-F-post-merge-closure.md` | `docs/archive/closure/162-S-175-F-post-merge-closure.md` |
| `docs/closure/165-S-175-F-post-merge-closure.md` | `docs/archive/closure/165-S-175-F-post-merge-closure.md` |
| `docs/closure/166-S-175-F-post-merge-closure.md` | `docs/archive/closure/166-S-175-F-post-merge-closure.md` |
| `docs/closure/167-S-175-F-post-merge-closure.md` | `docs/archive/closure/167-S-175-F-post-merge-closure.md` |
| `docs/closure/168-S-175-F-post-merge-closure.md` | `docs/archive/closure/168-S-175-F-post-merge-closure.md` |
| `docs/closure/169-S-175-F-post-merge-closure.md` | `docs/archive/closure/169-S-175-F-post-merge-closure.md` |

The `source:` front matter in each archived record still names its original path. Use this table to resolve it.
