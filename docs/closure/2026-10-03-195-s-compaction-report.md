---
chunk_strategy: h1-h2-h3
compaction_status: degraded
description: "P-020 target-all assessment preserved 195-S closure context but did not complete the wider candidate review."
doc_type: closure
schema_version: "1.0"
source: docs/closure/195-S-claim-start-proof-post-merge-closure.md
target: all
shipment_id: 195-S
title: "P-020 context compaction — 195-S"
docline:
  date: 2026-10-03T05:49:35Z
  status: degraded
  tags:
    - context-compaction
    - p-020
    - 195-S
---

## Result

`compact-context` was invoked for `target: all` again after governed shipment
closure. The inventory completed, but candidate-by-candidate lifecycle,
reference, and supersession checks across the full target set did not. No
memory, plan, or closure source file was moved or deleted. Compaction status
is `degraded`, not `done`.

| Directory | Files | Bytes | Older than 14 days by file timestamp |
|---|---:|---:|---:|
| `docs/memory/` | 117 | 548,493 | 29 |
| `docs/exec-plans/` | 130 | 4,414,603 | 121 |
| `docs/closure/` | 169 | 1,231,868 | 155 |

The current backlog query returned no `active` items and three `blocked` tasks:
`164.001-T`, `164.002-T`, and `165.002-T`. The 195-S records remain necessary
to document this closure and its follow-ups and were preserved. The older
inventories contain 29 memory files, 121 plans, and 155 closure records past
the age threshold. The dated plan scan also found numerous review-bearing
plans; each candidate still needs completion, active/blocked-reference, and
supersession checks before any archival move.

## Preservation and follow-up

No files were compacted or archived. The verbose originals remain in their
current locations. Re-run `compact-context` with `target: all` after the
candidate lifecycle, blocked-work, and reference checks are complete.
Follow-up stash: `359D8F32`. The 195-S closure artifact records
`compaction_status: degraded`.
