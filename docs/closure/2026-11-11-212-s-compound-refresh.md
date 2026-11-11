---
title: "212-S evidence-backed compound refresh"
doc_type: reference
source: docs/closure/2026-11-11-212-s-compound-refresh.md
schema_version: "1.0"
chunk_strategy: h1-h2-h3
date: "2026-11-11"
shipment_id: 212-S
feature_id: 211-F
mode: propose
---

## Scope and evidence

The six scoped entries below were read in full. Evidence is the shipped
PR645 source/CI handoff, the existing detailed runtime/closure records,
and the certified post-archive report. No fresh code audit or test run is
claimed. No compound entry is rewritten, consolidated, replaced or deleted.

| Entry under docs/compound | Classification | Evidence and rationale |
| --- | --- | --- |
| test-failures/private-stdio-for-reexecuted-go-coverage-2026-11-10.md | keep | CI cycle2 preserves genuine instrumentation-only failures and private descriptor ownership; final Linux race/coverage CI passed without suppressing assertions |
| concurrency-issues/overlapped-pipe-buffer-lifetime-2026-11-08.md | keep | U4 accepted memory preserves completion/drain lifetime analysis; race PASS alone was never a garbage-collector liveness proof |
| concurrency-issues/unregister-completed-request-session-cancellation-2026-11-09.md | keep | U5 option5 evidence separates bridge-registration cleanup from observable handler cancellation and request-ID reuse |
| runtime-errors/derive-daemon-version-from-pinned-image-2026-11-09.md | keep | U6 option5 evidence distinguishes library-host version from independently built pinned image; live daemon adoption is still not established |
| test-failures/separate-compile-budget-and-own-subprocess-tree-2026-11-08.md | keep | U2j C1 manifest preserves actual assertion RED, separate compile budgets, native errors and owned descendant cleanup |
| 2026-10-06-ship-shipment-mcp-timeout-result-unknown.md | keep | Rules 1-7 remain authoritative; 212-S supplies another certified state reconstruction, not proof that all timed-out calls succeed |

## Retained operational distinctions

The timeout rule requires quiescence, archive/log/hook checks, no returned
members, doctor and GI/GR/P-007. The original early lock release remains a
deviation; the later locked audit does not erase it. No retry, restoration,
synthetic shipped event or fabricated RPC reply is justified.

References preserved at original paths include U4 accepted, U5/U6 option5,
U2j C1 manifest and detailed CI cycle2 memory. Only three unrelated historical
launch narratives were archived with redirects by
[actual compaction](2026-11-11-212-s-compaction-report.md).

## Follow-up boundaries

Monitoring activation/owner acceptance and compatible rollback rehearsal
belong to future approved adoption, not this merge-only closure.
The Windows private-wrapper P2 remains open, alongside all twelve existing
C2 residual IDs in the canonical closure. Parent owns any new follow-up
capture. No source-artifact cleanup or discretionary backlog change occurs.
