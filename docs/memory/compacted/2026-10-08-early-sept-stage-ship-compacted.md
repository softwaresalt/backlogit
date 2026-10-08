---
chunk_strategy: h1-h2-h3
schema_version: "1.0"
title: "Early-September 2026 Stage/Ship session records (compacted)"
description: "Dense summary of eight aged memory notes from 2026-09-03 to 2026-09-08 covering the 135-S to 147-S dark-factory grouping, the PR #404/#405 remediation, the S4 138-S replan, the 171-F checkpoint schema decision, the 138-S pre-PR checkpoint, 134-S gate registration, and the continuity-file carry-forward learning. Originals are archived, not deleted."
doc_type: memory
source: docs/memory/compacted/2026-10-08-early-sept-stage-ship-compacted.md
docline:
    date: 2026-10-08T00:00:00Z
    tags:
        - compacted
        - p-020
        - 153-S
---

# Early-September 2026 Stage/Ship session records

This summary was produced by the P-020 compaction run in the 153-S post-merge
closure. The verbose originals are archived under
`docs/archive/memory/2026-10-08/`; nothing was deleted.

## Dark-factory grouping and session (2026-09-03)

* Stage grouped the dark-factory intake into thirteen release units, S1 to
  S13, and staged them as shipments `135-S` through `147-S`.
* PR #404 merged as `d0d0790d` and carried the staged backlog.
* Originals: `2026-09-03-dark-factory-grouping-checkpoint.md` and
  `2026-09-03-dark-factory-session-final.md`.

## PR #404/#405 remediation closure (2026-09-03)

* The remediation PR #410 merged as `191cf64b`.
* Copilot raised 23 threads: 15 were fixed and 8 were recorded as unresolved
  residuals with their blockers.
* Original: `2026-09-03-stage-pr404-405-remediation-closure.md`.

## 134-S gate registration (2026-09-03)

* PR #406 merged as `826864df` and registered the 134-S closure gate.
* An earlier P-020 pass in that session archived 19 memory files.
* Original: `2026-09-03-134-s-gate-registration-session-memory.md`.

## S4 / 138-S replan (2026-09-05)

* PR #422 merged as `63167559` after 17 Copilot review rounds.
* It settled the S4 contract design that 138-S later shipped against.
* Original: `2026-09-05-stage-s4-138s-replan-closure.md`.

## 171-F checkpoint schema decision (2026-09-06)

* Stage staged `152-S` / `171-F` through PR #428.
* Decision: a legacy checkpoint import is upgrade-or-reject. There is no
  silent pass-through of a non-conforming document.
* Original: `2026-09-06-stage-6fdc4a49-171f-checkpoint-schema-reject.md`.

## 138-S pre-PR checkpoint (2026-09-08)

* All `156-F` tasks were done and local review returned
  `READY_WITH_FOLLOWUPS`.
* Follow-ups were stashed as `4DB1DFF1`, `C549C922`, and `FE4FA974`.
* Original: `2026-09-08-ship-138s-pre-pr-checkpoint.md`.

## Continuity-file carry-forward learning (2026-09-08)

* Continuity files that a later session depends on must be carried forward
  explicitly at closure rather than left in a dated session folder.
* Original: `2026-09-08-carry-forward-continuity-files-compound-memory.md`.

## Excluded from this pass

Two equally aged notes stay in place because live closure records link to
them:

* `docs/memory/2026-09-08-ship-138s-compact-context.md`, linked from
  `docs/closure/138-S-156-F-post-merge-closure.md`
* `docs/memory/2026-08-20/azure-devops-sync-design-memory.md`, linked from
  `docs/closure/2026-09-11-140-s-p020-compact-context-report.md`