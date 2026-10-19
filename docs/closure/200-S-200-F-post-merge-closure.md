---
chunk_strategy: h1-h2-h3
closure_status: READY
compaction_status: done
description: "Post-merge closure for 200-S (CY-B: harden lock ordering, events, and declarations)."
doc_type: closure
docline:
  date: 2026-10-19T08:00:00Z
  status: final
  tags:
    - closure
    - 200-S
schema_version: "1.0"
shipment_id: 200-S
source: docs/closure/200-S-200-F-post-merge-closure.md
title: "200-S closure"
---

# 200-S closure

Context: feature 200-F (CY-B), PR #540, merge `daac36a27cde0944970d5fe24cf4aca77fbce94d`. Members: 200-F plus 200.001-T through 200.013-T (14). Changes: canonical lock order global, A, C, B for ShipShipment and standalone/cascade ArchiveItem; archive events appended under C; deferred event sink retired for those paths; fail-closed C-under-B check; CAS-guarded post-archive stash side effect run by the outermost owner after lock release; conditional `RepresentationSet` marker with EventsJSONL conditional for CreateItem and ArchiveItem. B3 needed no code (reconcile already holds global and A).

## Releasability

`closure_status: READY`. No runtime surface, so runtime-verification is not applicable. Monitoring: watch for `item log lock busy` and `archive item-log lock set incomplete` errors during ship and archive. Rollback: revert PR #540. Owner: operator. Validation window: next shipment closure (the ship path is exercised).

## Compaction status (P-020)

`compaction_status: done`. compact-context ran with `target: all`. docs/memory held 176 files / 870 KB, so it archived 13 files that had no incoming references, wrote one summary and rewrote five citations. 164 files remain because the rest are referenced from `.backlogit/` or other docs (BDDB9F9C check). Commit `c049de82`.

## Evidence

* Local review: three persona reviewers (Concurrency, Correctness, Go) on the full diff, then re-reviews of each remediation. The adversarial-review agent could not run (its session had no shell). Outcome READY_WITH_FOLLOWUPS, P0=0, P1=0.
* Copilot: three iterations. Flat C set fixed in `3ebe5251`. Nested stash-effect deadlock (P1) fixed in `335066b8`. Commit-writer lock order deferred as stash `703835DC`. All threads replied to and resolved. P-018 gate SATISFIED at `335066b8`.
* CI: all required checks green at `335066b8`, including Windows handle/lock tests. Merge strategy: merge commit.
* Local: the full `internal/core` suite passed at the earlier commits. After `335066b8` two runs hit load-sensitive timing tests that pass alone (`TestShipmentLifecycleBarrier_SingularTypedMemberWriterWaits`, `TestUR10_SubprocessCrashReopenRecovery`), and one run timed out at 30 minutes; CI passed on the same commit.
* Transient: backlogit MCP claim, task moves and ship timed out client-side (about 60s) and converged server-side; verified by polling, no CLI retry. Claim took about 7 minutes and ship about 30.

## Source artifact provenance (untouched)

Source stash IDs 010F437C, 5247D4BC, AB31C9C5, BACD94FC, DBF89C9C and E4908F30 were archived by Stage at harvest. No source stash or deliberation was a shipment member, and none was mutated here.

## Compound refresh

New entry `docs/compound/2026-10-19-lock-held-side-effect-ordering.md`. No existing entry is superseded; `2026-10-06-ship-shipment-mcp-timeout-result-unknown.md` is kept (reproduced again).

## Follow-ups

* Stash `11547BEF` (DEFERRED SCOPE EXPANSION): generic status-event paths lack the fail-closed guard, C hold duration during the ship tail, a stale `returned_to_backlog` comment, an A-under-B invariant test.
* Stash `703835DC` (DEFERRED SCOPE EXPANSION): commit writers take C then global, the inverse of archive and ship.
* Advisory (not stashed): the negative-window archive tests rely on 1s and 250ms timing windows.
