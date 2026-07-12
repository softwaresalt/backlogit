---
chunk_strategy: h1-h2-h3
description: Closure record for plugin PATH-resolved binary launch feature 095-F.
doc_type: closure
docline:
    ms.date: 2026-07-11T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/closure/2026-07-11-plugin-path-binary-launch-closure.md
title: Plugin PATH Binary Launch Closure
---

## Summary

Feature `095-F` migrated the Copilot CLI plugin away from the retired npm
wrapper launch path. The plugin manifests now launch the MCP server with the
PATH-resolved `backlogit` binary:

```json
{ "type": "stdio", "command": "backlogit", "args": ["mcp"] }
```

The design matches the Copilot CLI plugin schema: `mcpServers.command` is a
direct executable lookup, and MCP server entries do not support per-OS
`bash` or `powershell` dispatch keys.

## Changed surfaces

| Surface | Closure note |
|---|---|
| `plugin/.mcp.json` | Replaced `npx -y @backlogit/backlogit-mcp mcp` with `backlogit mcp` |
| `plugin/plugin.json` | Replaced the plugin MCP server launch while preserving agents and skills |
| `npm/` | Removed the obsolete npm wrapper and platform package tree |
| `scripts/package-npm*.sh` | Removed obsolete npm packaging scripts |
| `.gitignore` | Removed the stale npm binary negation |
| `README.md`, `docs/plugin-guide.md`, `docs/installation.md`, `docs/workflow.md` | Documented the PATH binary prerequisite and `backlogit mcp` launch |
| `tests/integration/plugin_manifest_test.go` | Added a drift guard for both plugin manifests and active plugin install docs |
| `docs/compound/` | Added a current PATH-launch learning and marked the old npm-wrapper learning as superseded |
| `.backlogit/` | Archived feature `095-F` and tasks `095.001-T` through `095.004-T` |

## Validation

* TDD red phase: `TestPluginManifestsUsePathResolvedBacklogitBinary` failed
  against the prior `npx` manifest launch
* Green phase: the guard passed after both manifests used `backlogit mcp`
* Local gates passed before PR:
  * `go build ./cmd/backlogit`
  * `go test ./...`
  * `go vet ./...`
  * `golangci-lint run`
  * changed Go test file was `gofmt` clean
* Documentation validation passed with `backlogit docs lint`
* Local review reached `LOCAL_REVIEW_READY` after fixing two P1 findings:
  * expanded the drift guard beyond the two manifests
  * corrected backlog text that still described the superseded resolver/cache scope
* CI passed on PR #213 for head
  `2faa68cd3a3f01ed5a893b41c8c798745f52dab2`
* Copilot review covered the final head SHA with zero unresolved Copilot threads

## Copilot review log

| Iteration | Result | Resolution |
|---|---|---|
| 1 | Three valid comments identified historical evidence corruption from over-broad npm reference rewrites | Restored historical evidence in commit `13dcbc4`; replied to and resolved all threads |
| 2 | Eight valid comments identified fabricated or incorrect historical path replacements | Restored historical paths and commands in commit `2faa68c`; replied to and resolved all threads |
| 3 | Review covered current head `2faa68cd3a3f01ed5a893b41c8c798745f52dab2` | No unresolved Copilot threads remained |

## Release readiness

* Runtime contract is narrowed to the documented plugin prerequisite: the
  `backlogit` binary must be on `PATH`
* Plugin users can satisfy the prerequisite with the repository install scripts,
  GitHub Releases, or:

```bash
go install github.com/softwaresalt/backlogit/cmd/backlogit@latest
```

* The MCP launch path is observable by running `backlogit mcp`
* The drift guard fails if an active plugin manifest or active plugin install
  guide reintroduces the retired npm wrapper launch

## Rollback and limitations

Rollback is a manifest and documentation revert: restore the previous plugin
MCP server command and reintroduce the npm wrapper artifacts. That rollback is
not recommended because native Copilot CLI installs do not guarantee Node or
`npx`.

The live `copilot plugin install softwaresalt/backlogit` flow was not exercised
in CI. CI validates the checked-in manifests, documentation, and Go test guard;
the live plugin install remains a manual runtime verification step.

## Merge and closure

* Stash: `60B8564F`
* Feature: `095-F`
* Tasks: `095.001-T`, `095.002-T`, `095.003-T`, `095.004-T`
* Main PR: #213
* Merge strategy: normal merge commit
* Merge commit: `7f7d82c3f6237e5c94d6aa5f0eba0b15bfd01f8c`
* Admin fallback: not used
* Closure branch: `chore/plugin-path-binary-closure`
* Dark-mode markers: `LOCAL_REVIEW_READY`, `DARK_MODE_MERGE_AUTHORIZED`,
  `DARK_MODE_COMPLETE`
