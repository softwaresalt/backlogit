---
chunk_strategy: h1-h2-h3
description: Superseded implementation plan for early Copilot CLI plugin distribution.
doc_type: plan
docline:
    date: 2026-04-27T00:00:00Z
    origin: .backlogit/queue/042-DL.md
    status: superseded
ingested_at: "2026-06-26T02:33:21Z"
schema_version: "1.0"
source: docs/exec-plans/2026-04-27-copilot-cli-plugin-distribution-plan.md
title: Superseded Copilot CLI Plugin Distribution Plan
---

## Superseded status

This plan captured an early plugin distribution design that depended on a
retired JavaScript package wrapper. Feature `095-F` replaces that design with a
PATH-resolved native binary launch.

The current design is intentionally smaller:

* install `backlogit` through the repository install scripts, `go install`, or a
  SHA256-verified GitHub Releases binary
* keep `backlogit` on PATH for the environment where Copilot CLI starts
* install the plugin with `copilot plugin install softwaresalt/backlogit`
* let the plugin manifest launch the MCP server as `backlogit mcp`

The historical wrapper details are obsolete and should not guide future plugin
work.
