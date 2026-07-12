---
chunk_strategy: h1-h2-h3
description: Superseded learning for the retired JavaScript package wrapper.
doc_type: learning
docline:
    category: best_practice
    component: cli
    date: 2026-04-28T00:00:00Z
    file_path: legacy package wrapper
    message: Superseded by the PATH-resolved backlogit plugin launch.
    problem_type: best_practice
    resolution_type: superseded
    resolved: false
    root_cause: retired_distribution_path
    severity: medium
    tags:
        - legacy
        - go
        - binary-distribution
        - copilot-plugin
        - sha256
ingested_at: "2026-06-26T02:32:58Z"
schema_version: "1.0"
source: docs/compound/best-practices/npm-hybrid-go-binary-resolver-2026-04-28.md
title: Superseded legacy wrapper for Go binary distribution
---

## Superseded status

This learning described a retired JavaScript package wrapper for launching the
backlogit MCP server. Feature `095-F` superseded that approach with a direct
PATH-based launch in the Copilot CLI plugin manifest.

Current plugin manifests declare the stdio server as:

```json
{
  "type": "stdio",
  "command": "backlogit",
  "args": ["mcp"]
}
```

The supported installation guidance is now:

* install the native `backlogit` binary with the repository install scripts,
  `go install`, or a SHA256-verified GitHub Releases binary
* ensure `backlogit` is on PATH in the environment where Copilot CLI starts
* install the plugin with `copilot plugin install softwaresalt/backlogit`

Do not revive the retired JavaScript wrapper unless the plugin schema gains a
first-class, documented binary provisioning mechanism.
