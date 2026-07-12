---
chunk_strategy: h1-h2-h3
description: Launch the backlogit Copilot CLI plugin through a PATH-resolved native binary instead of a package-manager wrapper.
doc_type: learning
docline:
    category: best_practice
    component: cli
    date: 2026-07-11T00:00:00Z
    file_path: .github/plugin/plugin.json
    message: Copilot CLI plugin MCP servers should direct-execute the native backlogit binary from PATH with args ["mcp"].
    problem_type: best_practice
    resolution_type: simplification
    resolved: true
    root_cause: retired_distribution_path
    severity: high
    tags:
        - copilot-plugin
        - mcp
        - binary-distribution
        - path
        - go
schema_version: "1.0"
source: docs/compound/best-practices/plugin-path-resolved-binary-launch-2026-07-11.md
title: PATH-resolved binary launch for the Copilot CLI plugin
---

## Problem

The Copilot CLI plugin needs to start the backlogit MCP server reliably after
the CLI moved to a native binary distribution model. The plugin schema does not
provide install-time binary provisioning for MCP servers, and the MCP server
configuration direct-executes the configured command through PATH lookup.

## Resolution

Declare the plugin MCP server as:

```json
{
  "type": "stdio",
  "command": "backlogit",
  "args": ["mcp"]
}
```

Document the native `backlogit` binary as a prerequisite. Users can install it
with the repository install scripts, `go install`, or a SHA256-verified GitHub
Releases binary, then run `copilot plugin install softwaresalt/backlogit`.

## Evidence

* Feature `095-F` updated the plugin MCP launch contract to direct-execute
  `backlogit mcp`
* Feature `097-F` moved the canonical install manifest to
  `.github/plugin/plugin.json` and removed legacy drift copies
* `tests/integration/plugin_manifest_test.go` guards the canonical manifest,
  referenced agent and skill asset paths, and active plugin/MCP docs from
  drifting back to the retired wrapper path
* The superseded historical wrapper learning remains at
  `docs/compound/best-practices/npm-hybrid-go-binary-resolver-2026-04-28.md`
