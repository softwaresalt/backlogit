---
chunk_strategy: h1-h2-h3
description: Session memory for the Copilot plugin install path fix.
doc_type: memory
docline:
    ms.date: 2026-07-12T00:00:00Z
    ms.topic: memory
schema_version: "1.0"
source: docs/memory/2026-07-12/plugin-install-path-fix-memory.md
title: Plugin install path fix memory
---

## Outcome

Feature `097-F` fixed the Copilot CLI direct-install manifest location bug.
PR #218 merged with merge commit `2673e6d85ca05fae9c613e736c73a1a889f6faa1`.

## Files changed

* Added `.github/plugin/plugin.json`
* Removed `plugin/plugin.json` and `plugin/.mcp.json`
* Updated `tests/integration/plugin_manifest_test.go`
* Updated plugin install docs and compound learning references
* Added closure record
  `docs/closure/2026-07-12-plugin-install-path-fix-closure.md`

## Decisions

* `.github/plugin/plugin.json` is the single canonical manifest
* `plugin/` remains the asset bundle only
* The new manifest references assets with repo-root-relative `plugin/...` paths
* Live `copilot plugin install` remains operator-manual because it writes outside
  the repository workspace

## Follow-ups

* Stash `84B73A39`: audit standalone plugin agent and skill copy drift plus the
  `verify-plugin` target

## Compact-context result

Compaction was assessed at batch completion. `docs/memory/` contained 23 files
and 82.53 KB, below the 40-file and 500 KB thresholds, with no old memory
candidates found. No files were archived.
