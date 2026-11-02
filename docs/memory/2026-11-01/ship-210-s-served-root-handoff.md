---
title: "Ship 210-S served-root handoff"
doc_type: memory
schema_version: "1.0"
source: docs/memory/2026-11-01/ship-210-s-served-root-handoff.md
chunk_strategy: h1-h2-h3
---

# Served-root handoff

Shipment `210-S` is active, with update timestamp
`2026-10-31T06:59:28.7708084Z` and ordered members `209-F`, `209.001-T`,
`209.002-T`, `209.003-T`. The workspace root is `.`, storage root is
`.backlogit`, index is `.backlogit/backlogit.db`, and manifest is
`.backlogit/queue/210-S.md`. Live metadata and the database-list query matched
the supplied roots and the shipment response matched the supplied membership.

The Orchestrator's scoped P-012 raw-read declaration reports Windows
`CreateFileW` with `FILE_FLAG_OPEN_REPARSE_POINT` and
`GetFinalPathNameByHandleW` verification of the manifest, zero reparse flags
across root, storage, queue and manifest, and frontmatter matching the shipment
response. Ship did not independently repeat that raw read.

Checkpoint `checkpoint-20261101-234526.json` was selected and confirmed for
restore. The restored cursor, checkpoint pointer and prior gate results were
retained; the engram workspace was reachable and bound. Unscoped documentation
lint reported zero violations. The lifecycle topology gate and literal
`gofmt -l .` passed in this session. Merge remains unauthorized.
