---
chunk_strategy: h1-h2-h3
description: Closure record for fixing Copilot CLI owner/repo plugin install manifest discovery.
doc_type: closure
docline:
    ms.date: 2026-07-12T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/closure/2026-07-12-plugin-install-path-fix-closure.md
title: Plugin install path fix closure
---

## Summary

Feature `097-F` fixed the Copilot CLI direct-install failure for
`copilot plugin install softwaresalt/backlogit`. The canonical plugin manifest
now lives at `.github/plugin/plugin.json`, which is in Copilot CLI's direct
install search set. The manifest keeps the plugin assets under `plugin/` and
references them with repo-root-relative `plugin/...` paths.

## Shipped changes

* Added `.github/plugin/plugin.json` as the single canonical install manifest
* Removed legacy drift copies at `plugin/plugin.json` and `plugin/.mcp.json`
* Updated the integration drift guard to assert the canonical manifest, the
  `backlogit mcp` stdio contract, and every referenced agent and skill path
* Updated README, plugin guide, installation docs, rationale, and compound
  learning references
* Preserved `plugin/agents/*` and `plugin/skills/*/SKILL.md` asset contents

## Verification

* TDD red: `go test ./tests/integration -run TestPluginManifest` failed before
  implementation because `.github/plugin/plugin.json` was missing and legacy
  manifests still existed
* TDD green: `go test ./tests/integration -run TestPluginManifest` passed after
  implementation
* `go build ./cmd/backlogit`
* `go test ./...`
* `go vet ./...`
* `golangci-lint run`
* `gofmt -l tests/integration/plugin_manifest_test.go`
* CI for PR #218 passed: Detect code changes, Docline frontmatter gate, CLI
  Reference Drift, and test

## Review and PR

* Local review readiness: no in-scope P0/P1 findings
* Residual review follow-up: stash `84B73A39` tracks a separate audit of
  standalone plugin agent and skill copy drift plus the `verify-plugin` target
* Copilot review on PR #218 completed for head
  `527a0bb5bca831ee308aff68aa9f63cf8a883895`
* Copilot raised three local-dev `.github/plugin` documentation comments; all
  were declined with rationale based on the operator-supplied primary-source
  research and resolved through GraphQL
* PR #218 merged normally with a merge commit:
  `2673e6d85ca05fae9c613e736c73a1a889f6faa1`

## Runtime and operator notes

Live `copilot plugin install` was not executed by the agent because it writes
outside the repository workspace. Operators can perform that manual validation
after the merge by running:

```bash
copilot plugin install softwaresalt/backlogit
```

For older revisions before this fix, the immediate workaround was the
subdirectory form:

```bash
copilot plugin install softwaresalt/backlogit:plugin
```

After this fix, use the plain owner/repo form.

## Rollback

Revert merge commit `2673e6d85ca05fae9c613e736c73a1a889f6faa1` if the canonical
manifest causes an unexpected install regression. The old workaround remains
available only on revisions where `plugin/plugin.json` still exists.
