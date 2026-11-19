---
title: "PR #658 GSD 1.6.0 Migration Compacted Memory"
description: "Dense P-020 compacted memory summary for PR #658 (chore/gsd-harness-migration)."
doc_type: memory
source: docs/memory/compacted/2026-11-19-pr658-gsd-harness-migration-compacted.md
ingested_at: "2026-11-19T06:37:12Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  date: "2026-11-19"
  pr_number: 658
  compacted_from:
    - docs/memory/2026-11-18/tune-harness-memory.md
    - docs/memory/2026-11-19/pr658-gsd-harness-migration-memory.md
---

## Summary

PR `#658` (`chore/gsd-harness-migration`) migrated the backlogit agent harness
from `.autoharness/` to **GSD 1.6.0** (`.gsd/` + workspace-local `.bin/`),
renamed `graphtor-docs` to read-only `kbdb`, deployed missing GSD 1.6.0 skills
and prompt shims, applied `/tune-harness` repairs (`TUNE-001`..`TUNE-007`), and
migrated affected Go test consumers and `wave-scheduler-sim` artifacts
(`be0efc09`). Merged to `main` at `b71f8f6a900a531165a373926c815c73502d117f`
(`2026-11-19T06:30:12Z`).

## Key outcomes

* **State & launcher migration**: Tracked `.autoharness/` config, registry,
  manifest, grants, continuous-learning state, cascade-close journals, and
  tuning reports moved to `.gsd/`; `start.ps1` / `start.sh` aligned with `.bin/`
  and read-only `kbdb`.
* **Tuning & verification**: `gsd verify-workspace` reports `0` blockers, `0`
  strict schema blockers, `0` unresolved findings, and `97/97` verified
  checksums; `wave-scheduler-sim.ps1` passes `279/279` assertions across `38`
  scenarios; `go build`, `docs lint`, and migrated CLI/contract/integration
  tests pass on `main`.
* **Tracked follow-ups**: P-021 C2 stash entries `2963DA86`, `D9FF2A9B`, and
  `85442104` (plus Stage disposition of `294A05DE`) and policy items in
  `.gsd/tuning-reports/2026-11-18-tuning-report.md` remain queued for Stage and
  operator follow-up.
