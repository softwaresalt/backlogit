---
chunk_strategy: h1-h2-h3
description: Closure record for shipment 087-S docs installation path clarification.
doc_type: closure
docline:
    ms.date: 2026-07-09T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/closure/2026-07-09-087-S-docs-install-paths-closure.md
title: 087-S Docs Installation Paths Closure
---

## Summary

Shipment `087-S` clarified the two backlogit installation paths:

* Path A - standalone backlogit through `copilot plugin install`
* Path B - backlogit inside an Autoharness-composed repo harness

The shipped docs distinguish the standalone plugin's agent and skill bundle
from the backlogit binary/runtime that Autoharness-generated registries can
still call through `backlogit` and `backlogit mcp`.

## Changed surfaces

| Surface | Closure note |
|---|---|
| `README.md` | Adds the up-front installation path decision point and Autoharness guidance |
| `docs/installation.md` | Explains binary/runtime install applicability for standalone and Autoharness paths |
| `docs/plugin-guide.md` | Documents the standalone plugin bundle and skill-location drift |
| `docs/rationale.md` | Records how standalone plugin and Autoharness adoption relate |
| `.backlogit/` | Shipment `087-S` shipped; `088-F` and tasks archived |

## Validation

* `C:\Tools\backlogit.exe docs lint` passed with zero findings
* `git --no-pager diff --check` passed for edited docs
* `LOCAL_REVIEW_READY`: self-review found no unresolved P0/P1 findings
* PR #195 CI passed; code-only checks were skipped as expected for docs-only work
* Copilot review raised 13 comments; 12 were fixed, 1 line-ending cleanup was
  declined with rationale, and all threads were replied to and resolved

## Merge and release record

* Feature branch: `feat/087-S-docs-install-paths`
* PR: #195
* Reviewed HEAD: `320bbf9c5d76b52e71234a576a6778e1df4a9dde`
* Merge commit: `9eb2f087548b96d7d2c7834c48324f7ca4433b86`
* Merge path: normal `--merge` was blocked by branch policy; dark-mode admin
  fallback used `gh pr merge 195 --merge --admin`
* P-009 preserved: no squash or rebase merge was used

## Operational notes

No runtime, schema, migration, or Go code changed. Monitoring is limited to
documentation feedback and future docs-lint results. Rollback is a normal
merge-commit revert if the installation guidance proves confusing.
