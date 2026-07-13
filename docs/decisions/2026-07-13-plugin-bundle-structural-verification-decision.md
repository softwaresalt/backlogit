---
chunk_strategy: h1-h2-h3
description: "Decision to replace plugin byte-identity drift checks with structural verification against the plugin manifest."
doc_type: decision
docline:
  conclusion: structural-verification-replaces-byte-identity
  confidence: high
  date: 2026-07-13T00:00:00Z
  linked_parent_work_item: 101-F
  tags:
    - plugin
    - verification
    - ci
    - manifest
schema_version: "1.0"
source: docs/decisions/2026-07-13-plugin-bundle-structural-verification-decision.md
title: "Plugin bundle structural verification"
---

## Context

`plugin/` is the product bundle installed into user workspaces. It is a
distributable surface for the backlogit product. `.github/` is backlogit's
self-hosting harness. The two trees are related operationally, but they are not
byte-identical sources of truth.

The previous `verify-plugin` target assumed byte identity between those trees.
That invariant was false. The target also referenced
`.github/agents/stage.agent.md` and `.github/agents/ship.agent.md`, while the
actual self-hosting agent sources are dot-prefixed files under `.github/agents/`.
The check therefore failed before it could validate meaningful plugin drift.

## Decision

Retire byte-identity comparison between `plugin/` and `.github/`. Validate the
plugin bundle structurally against `.github/plugin/plugin.json` instead.

The canonical bundle list lives once in
`tests/integration/plugin_manifest_test.go`. The structural test verifies:

* the manifest declares `plugin/agents` and `plugin/skills`
* the actual agent file set is exactly `stage.agent.md` and `ship.agent.md`
* the actual skill directory set is exactly the 19 shipped plugin skills
* every shipped agent and skill has YAML frontmatter with `name` metadata and a
  non-empty body

## CI enforcement

CI runs `go test -race -coverprofile=coverage.out ./...` in
`.github/workflows/ci.yml`. The new structural test is in `tests/integration`,
so that existing `./...` test command enforces the plugin bundle contract.

The CI path classifier treats `plugin/**`, `.github/plugin/**`, the structural
test file, and the local verify wrappers as plugin-bundle inputs. Those paths
run the existing Go gate even when the bundle edit is Markdown-only. No
redundant `make verify-plugin` CI step is required.

## Consequences

`make verify-plugin` and `make.ps1 verify-plugin` now delegate to the Go
structural test:

```text
go test ./tests/integration/ -run 'TestPluginBundleStructurallyValid' -count=1
```

This keeps the bundle list in one place and avoids reintroducing the false
`.github/` byte-identity invariant.
