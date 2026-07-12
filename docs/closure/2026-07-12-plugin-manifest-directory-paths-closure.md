---
chunk_strategy: h1-h2-h3
description: Closure for 099-F — plugin manifest directory-path fix that unblocked Windows plugin install.
doc_type: closure
docline:
    ms.date: 2026-07-12T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/closure/2026-07-12-plugin-manifest-directory-paths-closure.md
title: "099-F plugin manifest directory-path fix closure"
date: "2026-07-12"
item: "099-F"
status: "verified-live"
---

# 099-F — Plugin manifest directory paths (Windows install fix)

## Problem

`copilot plugin install softwaresalt/backlogit` failed on Windows with
`os error 267` (ERROR_DIRECTORY), even after the manifest-location fix
(#218) and the plain-form docs (#213). Manifest discovery succeeded; the
failure moved to the install load phase.

## Root cause

The Copilot CLI resolves the `agents` and `skills` fields in `plugin.json`
by calling `read_dir()` on each entry — they must be **directory** paths.
The manifest listed individual **file** paths
(`plugin/agents/stage.agent.md`, `plugin/skills/build-feature/SKILL.md`,
...), so `read_dir()` on a file returned `ERROR_DIRECTORY` (267). This was
wrong in the original `plugin/plugin.json` too; the earlier fixes only
advanced the failure to the next stage. The drift-guard test compounded it
by asserting the paths were files.

## Fix (PR #222, merge 6b13854)

- `.github/plugin/plugin.json`: `agents` → `plugin/agents`, `skills` →
  `plugin/skills` (parent directories, repo-root-relative), matching the
  proven `cockroachdb/copilot-plugin` manifest.
- `tests/integration/plugin_manifest_test.go`: the drift-guard now requires
  `agents`/`skills` to be **directories** (`os.Stat` + `IsDir`) and asserts
  each declared agent file and each of the 19 skill `SKILL.md` files exist.

## Live verification (096-F residual closed)

Because `COPILOT_HOME` resolves to the in-workspace `.copilot/`, the live
install was run inside the workspace boundary:

```text
> copilot plugin install softwaresalt/backlogit
Plugin "backlogit" installed successfully. Installed 19 skills.

> copilot plugin list
Installed plugins:
  • backlogit (v1.1.0)
```

`config.json` records the plugin as `enabled: true`, source `github`
`softwaresalt/backlogit`. The installed manifest resolves
`agents: plugin/agents`, `skills: plugin/skills`, and the `backlogit mcp`
stdio server. No `os error 267`. This closes the 096-F live-install
residual with real evidence.

## Follow-up

The CLI emitted a forward-looking deprecation warning:

> Direct plugin installs (repos, URLs, local paths) are deprecated. Only
> `plugin@marketplace` installs will be supported in a future release.

Publishing backlogit to a Copilot plugin marketplace is a future follow-up
(captured in the stash) so direct-repo install deprecation does not
eventually break distribution.
