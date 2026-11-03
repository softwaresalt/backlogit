---
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
description: "Post-merge closure for 168-S (baseline convergence RS-W11 lint remediation wave 11, Linux surfaces)."
doc_type: closure
docline:
  date: 2026-10-26T05:58:00Z
  status: final
  tags:
    - closure
    - 168-S
schema_version: "1.0"
shipment_id: 168-S
source: docs/closure/168-S-175-F-post-merge-closure.md
title: "168-S closure"
---

# 168-S closure

Context: feature 175-F (baseline convergence), PR #587, merge `5717a20d5289ec8fd345ef67f666247691f164e4`. Members: 175.095 to 175.098 (4 tasks, one wave). Changes: checked `Close` returns in the Linux-only files `internal/core/shipment_reconcile_{append,evidence,fs,lock,snapshot}_unix.go` and `internal/events/item_log_lock_unix.go`, using named-return deferred close handlers joined with `errors.Join`; four golangci-lint-backed harnesses (`lintfix_175_095..098_unix_test.go`) that run the linter with `GOOS=linux` in the child environment and carry no build tag, so they execute on the Windows host. No function signature changed.

## Releasability

`closure_status: READY`. There is no deployed service. The touched code is Linux-only locking and reconcile I/O that backs the CLI and MCP shipment close and item-log paths. Behaviour change: a failed directory or file close now surfaces as an error (an `ErrWriteIndeterminate`-wrapped error after a successful write, with results cleared; a joined error without a second sentinel after an earlier failure; an acquired lock is released when the directory close fails). These paths run only on Unix hosts and were validated by `GOOS=linux go vet`, `GOOS=linux go build ./...` and Linux errcheck, and by CI. No unmet condition exists. Monitoring: CI `test` on the next PR to main. Rollback: revert PR #587. Owner: operator. Validation window: next PR to main.

## Runtime verification (validator evidence)

* CLI probe: built `./cmd/backlogit` from synchronized `main` (`5717a20d`); `backlogit queue view --group-by status` and `backlogit stash list` both exited 0 with populated output.
* MCP probe: `backlogit_get_queue` with `limit: 3` returned a populated item list.
* Limit: the Unix-only code paths cannot execute on this Windows host; they are covered by cross-compile, vet and lint evidence only. No automation was faked.

## Compaction status (P-020)

`compaction_status: done`. compact-context ran with `target: all`; top-level `docs/memory/` holds 37 files (about 110 KB), under the 40-file / 500 KB thresholds, so there were no candidates.

## Evidence

* Local review: adversarial review of `7f643580` ran as a single reviewer without a shell (provisional READY_WITH_FOLLOWUPS; P0=0, P1=0, P2=3, P3=4). The one in-scope P2 (double sentinel on close-after-failure) was fixed in `06b25f4c`; a Go Reviewer re-review of `06b25f4c` could not run because that agent has no shell, so Ship ran vet, build, gofmt and the full suite itself. Overall outcome `READY_WITH_FOLLOWUPS`. PR HEAD `5f0dd69e6309e7b8561afdda249e258fb14a35e1` adds only `.backlogit/` state and `docs/memory/` on top of the reviewed code.
* Copilot: review of the final HEAD was a `COMMENTED` overview with approval recommended, no open issues and no inline threads, so there was nothing to reply to or resolve; P-018 gate SATISFIED after CI and again at the last mile. CI all green. Merge used the merge-commit strategy; no admin fallback.
* Tests: build, vet and gofmt clean; unfiltered `go test -count=1 ./...` passes, with the lint-harness packages run via `-p 1` because concurrent golangci-lint runs fail with exit status 3 (stash `FE7D57F1`); four harnesses red before the fix and green after.
* Transient: `backlogit_ship_shipment` timed out client-side and completed server-side; it was verified by quiescence polling and not retried. The archive record carries `status: archived`, `archived_status: shipped` and the merge commit.
* Reconcile reports: `.backlogit/reconcile/168-S-pre-20261026T0550Z.md`, `168-S-safe-close-20261026T0555Z.md`, `168-S-post-20261026T0556Z.md`.

## Source artifact provenance (untouched)

No source stash or deliberation was a shipment member; none mutated.

## Compound refresh

No entry in `docs/compound/` is superseded or invalidated by this wave. New learning recorded in `docs/memory/2026-10-26-ship-168-s-rs-w11.md`: run lint-harness packages serially.

## Follow-ups

* Stash `4503EF93` (P-021 C2 deferred scope expansion): behavioural tests for the close-error paths, harness positive control, pre-existing `_ =` discards; needs deliberation.
* Operator stash `760670A3` (single backlogit daemon per workspace) was carried forward untouched. Note: 21 backlogit processes were running during closure.
* Remaining errcheck findings in other files (for example `internal/core/shipment_ops_unix.go`) belong to later tasks of 175-F.
