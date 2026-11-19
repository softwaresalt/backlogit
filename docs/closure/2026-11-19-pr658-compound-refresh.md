---
title: "PR #658 compound refresh and knowledge evaluation"
description: "Post-merge compound-refresh classifications and documentation graduation evaluation for PR #658 (chore/gsd-harness-migration)."
doc_type: closure
source: docs/closure/2026-11-19-pr658-compound-refresh.md
ingested_at: "2026-11-19T06:37:12Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  date: "2026-11-19"
  pr_number: 658
  merge_commit: b71f8f6a900a531165a373926c815c73502d117f
---

## Invocation and evidence

Evaluated `docs/compound/` entries and durable architecture documentation under
Ship Step 6 items 3-5 (`compound-refresh`, `scope: recent`, `mode: propose`).
Evidence sources: merged PR `#658` (`b71f8f6a900a531165a373926c815c73502d117f`),
`.gsd/config.yaml`, `.gsd/backlog-registry.yaml`, `.gsd/harness-manifest.yaml`,
`.gsd/tuning-reports/2026-11-18-gsd-migration-report.md`, and
`.gsd/tuning-reports/2026-11-18-tuning-report.md`.

## Compound classifications

| Entry | Classification | Rationale |
| --- | --- | --- |
| [Autoharness gate-broker integration contract](../compound/2026-07-06-autoharness-gate-broker-integration-contract.md) | `update` (proposed) | Historical 082-S Go `internal/core/gate/*` contract remains accurate for the binary seam, but live harness invocation now targets `.bin/gsd.exe` and `.gsd/` rather than `autoharness` and `.autoharness/`; propose adding a non-destructive migration header note without rewriting historical evidence |
| [Closure gate requires a completed compaction status and a conditions block](../compound/2026-10-23-closure-gate-requires-compaction-and-conditions.md) | `update` (proposed) | Core P-020 / `pre_claim` gate rule remains authoritative; CLI example cites `autoharness gate pipeline-topology`, which is now `gsd gate pipeline-topology` |
| [READY_WITH_CONDITIONS closures need a conditions list or the pre_claim gate blocks successors](../compound/2026-10-21-closure-conditions-list-predicate.md) | `update` (proposed) | Predicate rule remains authoritative; CLI citations reference `autoharness gate ...` and stash `3FF72EB8`, now migrated to `gsd gate ...` |
| [Docline frontmatter contract](../compound/2026-06-26-docline-frontmatter-contract.md) | `keep` | Authoring and ingestion contract remains unchanged; `graphtor-docs` historical origin is superseded by `kbdb` in live config while historical provenance remains accurate |
| [CLI/MCP honest fallback map and registry drift test](../compound/2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md) | `keep` | Registry parity invariant remains accurate; `be0efc09` updated `internal/cli/registry_parity_test.go` to load `.gsd/backlog-registry.yaml` |

Reviewed in detail: 5 primary entries (`keep: 2`, `update` proposed: `3`, `consolidate: 0`, `replace: 0`, `delete: 0`). Ten additional historical entries under `docs/compound/` (`2026-07-06-*`, `2026-07-07-*`, `2026-07-26-*`, `2026-08-31-*`, `2026-04-05-*`) retain historical `autoharness` references as past-session context and are classified `keep`.

## Documentation graduation assessment

* `docs/ARCHITECTURE.md` — Updated line 183 from `graphtor-docs` pipeline to `kbdb` pipeline as direct post-merge doc-gardening aligned with PR `#658`.
* `AGENTS.md` and `.github/copilot-instructions.md` — Already migrated to `.gsd/`, `.bin/`, and `kbdb` in PR `#658`; no additional edits required.
* `docs/cli-reference/` — Confirmed zero drift in CI (`CLI Reference Drift` passed on `be0efc09`).
