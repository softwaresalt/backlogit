---
chunk_strategy: h1-h2-h3
description: Closure record for shipment 090-S post-drain follow-ups.
doc_type: closure
docline:
    ms.date: 2026-07-11T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/closure/2026-07-11-090-S-post-drain-followups-closure.md
title: "090-S post-drain follow-ups closure"
shipment: "090-S"
items:
  - "094-F"
  - "096-F"
status: "closed-with-operator-residual"
---

## Summary

Shipment `090-S` closed two post-drain follow-ups:

* `094-F` evaluated the Windows self-update crash-window recovery path
* `096-F` verified the in-workspace Copilot plugin launch prerequisites and
  recorded the operator-only live install residual

## 094-F outcome

No code change was made. The existing Windows self-update implementation already
removes stale `.old` backups before the next replacement attempt, removes stale
`.old` during a later already-current update run, and rolls back `.old` if the
second Windows rename fails synchronously.

The accepted residual is a bounded two-syscall crash window where `.old` remains
recoverable. A deeper reduction would require a helper process or deferred
installer flow, which is not justified for this low-priority follow-up.

## 096-F outcome

In-workspace verification passed:

* `plugin/.mcp.json` and `plugin/plugin.json` declare the backlogit MCP server
  as `type: "stdio"`, `command: "backlogit"`, and `args: ["mcp"]`
* manifest drift guard passed for the PATH-resolved launch contract
* `.\backlogit-dev.exe mcp` returned a well-formed JSON-RPC `initialize` result

The true live command, `copilot plugin install softwaresalt/backlogit`, was not
run by the agent because it writes under `~/.copilot/installed-plugins/`, outside
`C:\Source\GitHub\backlogit`. The runtime verification record gives the operator
the exact manual command sequence and expected result. Per the 096-F execution
constraint, the agent-owned deliverable is the in-workspace evidence plus this
operator handoff, not a fabricated installed-plugin result.

## Validation

Local verification:

* `go build .\cmd\backlogit`
* `go test .\internal\cli -run "TestRunSelfUpdateAlreadyCurrentCleansWindowsOldBinary|TestRunSelfUpdateRestoresOriginalWhenWindowsMoveNewFails|TestRunSelfUpdateKeepsOriginalWhenWindowsFirstRenameFails" -count=1`
* `go test .\tests\integration -run "TestPluginManifestsLaunchBacklogitFromPath|TestActivePluginDocsDoNotReferenceRetiredNPMWrapper" -count=1`
* `go test .\...`
* `go vet .\...`
* `golangci-lint run`
* `backlogit docs lint`
* `git --no-pager diff --cached --check`

`gofmt -l .` on this Windows checkout listed existing Go files because of
repository line-ending normalization. No Go files were changed for this
shipment.

## Review and CI

Local review:

* Review skill invoked in report-only mode
* P0/P1 findings: none after scoped staged-diff rereview
* One P2 suggestion to add an H1 was declined because repository markdown
  instructions allow frontmatter `title:` to replace the H1, matching existing
  closure records

Hosted review and CI:

* Main PR: #216
* Main PR head: `600638dba1a621a3f471f9c325e13ec1a094eed2`
* Copilot review state: `COMMENTED`
* Copilot review covered the PR head with zero unresolved Copilot threads
* CI checks passed: Detect code changes, test, Docline frontmatter gate, and
  CLI Reference Drift
* Merge strategy: normal merge commit
* Merge commit: `b84bf96e3fefd93290242f1cb9a3c3ad5cc55671`

## Backlog closure

Post-merge backlog operations:

* `094-F` moved to `done` and archived
* `096-F` moved to `done` and archived
* `090-S` shipped and archived

## Follow-ups

Operator-only residual:

```powershell
where.exe backlogit
copilot plugin install softwaresalt/backlogit
copilot
```

Expected result: Copilot launches the plugin MCP server with `backlogit mcp` and
a harmless backlogit MCP request succeeds.

This residual is outside CLI agent authority under Constitution IV. If the
operator wants durable tracking after the manual run, create a new targeted
follow-up with the captured install output.
