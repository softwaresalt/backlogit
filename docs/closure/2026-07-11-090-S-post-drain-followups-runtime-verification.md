---
chunk_strategy: h1-h2-h3
description: Runtime verification for shipment 090-S post-drain follow-ups.
doc_type: closure
docline:
    ms.date: 2026-07-11T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/closure/2026-07-11-090-S-post-drain-followups-runtime-verification.md
title: "090-S post-drain follow-ups runtime verification"
date: "2026-07-11"
shipment: "090-S"
items:
  - "094-F"
  - "096-F"
status: "ready-with-operator-residual"
---

## 094-F evaluation finding

The current Windows self-update implementation already contains the small
crash-window hardening that this follow-up asked us to evaluate.

Evidence:

* `replaceSelfUpdateBinaryWindows` removes a stale `.old` backup before the
  next Windows replacement attempt
* the same path renames `current -> .old`, renames `new -> current`, and rolls
  back `.old -> current` if the second rename fails synchronously
* `cleanupSelfUpdateBackupOnCurrent` removes a stale `.old` on a later
  already-current `backlogit update` invocation
* tests cover already-current stale backup cleanup and rollback after the
  second Windows rename fails

Judgment: no additional code change is warranted for this low-priority
follow-up. The remaining risk is a bounded two-syscall crash window that leaves
the old binary recoverable as `.old`. Reducing that further would require a
separate helper process or deferred installer flow, which is disproportionate
for this item.

## 096-F in-workspace verification

Manifest verification:

* `plugin/.mcp.json` declares `mcpServers.backlogit.type` as `stdio`
* `plugin/.mcp.json` declares `mcpServers.backlogit.command` as `backlogit`
* `plugin/.mcp.json` declares `mcpServers.backlogit.args` as `["mcp"]`
* `plugin/plugin.json` declares the same stdio server launch contract
* both manifests contain no `npx` or `@backlogit/` references

Drift guard:

```text
go test .\tests\integration -run TestPluginManifestsLaunchBacklogitFromPath -count=1
ok  	github.com/softwaresalt/backlogit/tests/integration	0.523s
```

MCP stdio smoke test:

Command:

```powershell
$request = '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"ship-smoke","version":"0.0.0"}}}'
$request | .\backlogit-dev.exe mcp
```

Captured initialize result:

```json
{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2024-11-05","capabilities":{"resources":{"listChanged":true},"tools":{"listChanged":true}},"serverInfo":{"name":"backlogit","version":"1.5.1-0.20260711080112-20a3a857c621"}}}
```

Result: the in-workspace binary started the backlogit MCP stdio server and
returned a well-formed JSON-RPC `initialize` result.

## Operator-only live install residual

The true end-to-end Copilot plugin install step writes to
`~/.copilot/installed-plugins/`, which is outside
`C:\Source\GitHub\backlogit`. Constitution IV prohibits this agent from running
that command or working around the boundary.

Operator command sequence:

```powershell
where.exe backlogit
copilot plugin install softwaresalt/backlogit
copilot
```

Inside the Copilot CLI session, request a harmless backlogit MCP operation, for
example:

```text
Use the backlogit plugin to report the backlogit version.
```

Expected result:

* `where.exe backlogit` resolves the intended backlogit executable on `PATH`
* `copilot plugin install softwaresalt/backlogit` completes successfully
* Copilot launches the plugin MCP server with `backlogit mcp`
* a harmless backlogit MCP request returns successfully, confirming the live
  installed plugin can start and communicate with the PATH-resolved server

Record result:

* On success, add the captured operator evidence to the final 090-S closure
  record before marking the residual live-install check complete
* On failure, leave the live-install residual open and record the command output
  so `096-F` can be reopened or followed by a new targeted backlog item
