---
chunk_strategy: h1-h2-h3
description: Installing backlogit as a Copilot CLI plugin with hybrid binary distribution
doc_type: guide
docline:
    ms.date: 2026-07-10T00:00:00Z
    ms.topic: guide
ingested_at: "2026-06-26T02:34:29Z"
schema_version: "1.0"
source: docs/plugin-guide.md
title: backlogit Plugin Guide
---

## Overview

backlogit distributes as a standalone Copilot CLI plugin. When installed, the
plugin contributes the Stage and Ship agents, 19 universal workflow skills, and
an MCP server backed by the native backlogit binary. Choose this path when you
want to use backlogit as a self-contained Copilot CLI product.

## Choose plugin or Autoharness

| Path | Choose when | Result |
|---|---|---|
| Path A - Standalone backlogit | You want backlogit agents, skills, and MCP tools without adopting a generated harness | `copilot plugin install softwaresalt/backlogit` installs the bundled product from `plugin/` |
| Path B - Autoharness-composed harness | Backlogit is one capability inside an Autoharness-generated repo harness | Autoharness install or tune composes templates and writes repo-specific files into `.github/` |

If Path B applies, do not install the standalone backlogit plugin into the same
repo harness. Use Autoharness to install or tune the harness so the
constitution, policies, instructions, agents, and skills match the workspace.
Autoharness replaces the plugin-provided agents and skills, not the
`backlogit` runtime; generated registries can still invoke `backlogit` and
`backlogit mcp`. For binary/runtime installs, see
[Installation](installation.md#which-installation-path).

## Prerequisites

* [GitHub Copilot CLI](https://docs.github.com/en/copilot/github-copilot-in-the-cli) installed and authenticated
* Node.js 18+ on your PATH. The current plugin manifest still launches the MCP
  bootstrap through `npx`; direct GitHub Releases bootstrap is tracked by
  backlog stash `60B8564F`.

---

## Installation

### Option 1 — Copilot plugin (recommended)

```bash
copilot plugin install softwaresalt/backlogit
```

This is the primary Path A installation. The plugin includes the Stage and Ship
agents, all 19 skills listed in [`../plugin/plugin.json`](../plugin/plugin.json),
and configures the backlogit MCP server automatically.

**Note:** On first invocation, the MCP server resolves the backlogit binary
(~10–30 s depending on connection speed). Subsequent starts are instant. If you
want to skip this first-run delay, use Option 2 to pre-install the native
runtime.

### Option 2 — one-line native runtime install

Install the native `backlogit` binary before installing the plugin. This is the
recommended way to avoid first-run binary resolution.

#### Windows PowerShell

```powershell
irm https://raw.githubusercontent.com/softwaresalt/backlogit/main/scripts/install/install.ps1 | iex
copilot plugin install softwaresalt/backlogit
```

#### Linux and macOS

```bash
curl -fsSL https://raw.githubusercontent.com/softwaresalt/backlogit/main/scripts/install/install.sh | sh
copilot plugin install softwaresalt/backlogit
```

When `backlogit` is on your PATH, the current bootstrap uses it directly with
zero binary download delay.

If your organization requires review before executing remote scripts, download
the installer script first or use a SHA256-verified binary from GitHub
Releases.

### Option 3 — install from source

Use this path when you want the Go toolchain workflow for the latest tagged
release:

```bash
go install github.com/softwaresalt/backlogit/cmd/backlogit@latest
copilot plugin install softwaresalt/backlogit
```

### Option 4 — direct GitHub Releases binary

Download the platform asset and `SHA256SUMS` from
[GitHub Releases](https://github.com/softwaresalt/backlogit/releases), verify
the checksum, place the executable on your PATH, and then install the plugin:

```bash
copilot plugin install softwaresalt/backlogit
```

See [Installation](installation.md#method-2-download-the-binary-directly) for
platform-specific commands.

> [!IMPORTANT]
> npm publishing for `@backlogit/*` was retired in v1.5.0. Do not use the
> retired npm packages as a user install path.

---

## What's Included

| Component | Details |
|-----------|---------|
| Agent: Stage | Stash-to-backlog pipeline: triage, deliberate, plan, review, harvest |
| Agent: Ship | Backlog-to-shipped pipeline: harness, build, review, CI, PR |
| Skills | 19 universal workflow skills (see list below) |
| MCP Server | backlogit MCP over stdio — all backlog tools available to agents |

### Bundled skills

`build-feature`, `compact-context`, `compound`, `compound-refresh`, `deliberate`,
`file-lock`, `fix-ci`, `harness-architect`, `harvest`, `impl-plan`,
`operational-closure`, `plan-harden`, `plan-review`, `pr-lifecycle`, `review`,
`runtime-verification`, `safety-modes`, `skill-search`, `spike`

## Skill locations and drift

The standalone plugin and an Autoharness-generated harness are related, but
their skill directories have different owners:

| Location | Owner | Meaning |
|---|---|---|
| [`plugin/skills/`](../plugin/skills/) | Standalone backlogit plugin bundle | Frozen product skills shipped by `copilot plugin install softwaresalt/backlogit` |
| [`.github/skills/`](../.github/skills/) | Autoharness-generated repo harness | Workspace-tailored skills tracked by [`.autoharness/harness-manifest.yaml`](../.autoharness/harness-manifest.yaml); many generated files also carry `Generated by autoharness` footers |

Do not treat the two trees as interchangeable. A plugin release can carry 19
bundled skills while a repo-local Autoharness harness can add, remove, or tune
skills for that workspace. Updating `plugin/skills/` changes the standalone
product bundle; updating `.github/skills/` changes this repository's generated
harness material.

---

## How Binary Resolution Works

The current plugin bootstrap still invokes the
`@backlogit/backlogit-mcp` wrapper through `npx`. That wrapper resolves the
native backlogit binary with this fallback behavior:

| Tier | Source | Latency |
|------|--------|---------|
| 1 | `backlogit` already on PATH | ~0 ms |
| 2 | Legacy `@backlogit/{platform}-{arch}` npm optional dep (retired in v1.5.0) | N/A |
| 3 | GitHub Releases download + SHA256 verification | ~10–30 s (first time only) |

The downloaded binary is cached at `~/.cache/backlogit/bin/backlogit-{version}[.exe]` and reused on subsequent starts.

The wrapper exists only as the current plugin bootstrap detail. User-facing
installs should use `copilot plugin install`, the one-line runtime installers,
`go install`, or a SHA256-verified GitHub Releases binary.

### Legacy wrapper platform keys

| Platform | Key |
|----------|---------|
| Linux x64 | `linux-x64` |
| Linux arm64 | `linux-arm64` |
| macOS x64 | `darwin-x64` |
| macOS arm64 (Apple Silicon) | `darwin-arm64` |
| Windows x64 | `win32-x64` |

---

## Troubleshooting

### Binary resolution fails

1. **Check Node.js version**: Node 18+ is required.
2. **Check network access**: Tier 3 downloads from `github.com`. If you're behind a proxy, set `HTTPS_PROXY`.
3. **Install natively**: Use Option 2 or Option 3 to put `backlogit` on PATH
   and bypass binary download.
4. **Clear cache**: Delete `~/.cache/backlogit/bin/` and retry.

### `copilot plugin install` shows no agents or skills

Verify the plugin installed correctly:

```bash
copilot plugin list
# Should show: backlogit

copilot /skills list
# Should include: build-feature, deliberate, harvest, impl-plan, ...
```

If skills are missing, reinstall:

```bash
copilot plugin uninstall backlogit
copilot plugin install softwaresalt/backlogit
```

### MCP server fails to start

Check that Node.js is on PATH:

```bash
node --version   # should be >= 18
npx --version
```

If binary resolution fails, you can force a fresh download by removing the
cache:

```bash
rm -rf ~/.cache/backlogit/bin/
```

---

## Initializing a workspace

After plugin installation, initialize backlogit in any project:

```bash
cd your-project
backlogit init
```

Then start Copilot CLI. The Stage agent can begin triaging ideas immediately.

---

## Uninstalling

```bash
copilot plugin uninstall backlogit
```

To remove the binary cache:

```bash
rm -rf ~/.cache/backlogit/
```
