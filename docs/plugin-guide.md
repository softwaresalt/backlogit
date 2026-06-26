---
chunk_strategy: h1-h2-h3
description: Installing backlogit as a Copilot CLI plugin with hybrid binary distribution
doc_type: guide
docline:
    ms.date: 2026-04-27T00:00:00Z
    ms.topic: guide
ingested_at: "2026-06-26T02:34:29Z"
schema_version: "1.0"
source: docs/plugin-guide.md
title: backlogit Plugin Guide
---

## Overview

backlogit distributes as a Copilot CLI plugin. When installed, the plugin contributes the Stage and Ship agents, 19 universal workflow skills, and an MCP server backed by the native backlogit binary.

## Prerequisites

- [GitHub Copilot CLI](https://docs.github.com/en/copilot/github-copilot-in-the-cli) installed and authenticated
- Node.js 18+ on your PATH (required for the npm wrapper)

---

## Installation

### Option 1 — Copilot plugin (recommended)

```bash
copilot plugin install softwaresalt/backlogit
```

This is the primary installation path. The plugin includes the Stage and Ship agents, all 19 skills, and configures the backlogit MCP server automatically.

**Note:** On first invocation, the MCP server downloads the backlogit binary (~10–30 s depending on connection speed). Subsequent starts are instant. If you want to skip this first-run delay, use Option 2 to pre-install the binary.

### Option 2 — npm global install (faster first run)

Pre-install the npm wrapper and binary cache before installing the plugin:

```bash
npm install -g @backlogit/backlogit-mcp
copilot plugin install softwaresalt/backlogit
```

With a globally installed wrapper, `npx` finds the binary immediately — no download on first use.

### Option 3 — native binary (maximum performance)

Install the backlogit binary directly:

```bash
go install github.com/softwaresalt/backlogit/cmd/backlogit@latest
copilot plugin install softwaresalt/backlogit
```

When `backlogit` is on your PATH, the npm wrapper uses it directly (Tier 1 resolution) with zero overhead.

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

---

## How Binary Resolution Works

The npm wrapper (`@backlogit/backlogit-mcp`) resolves the backlogit binary using a three-tier fallback:

| Tier | Source | Latency |
|------|--------|---------|
| 1 | `backlogit` already on PATH | ~0 ms |
| 2 | `@backlogit/{platform}-{arch}` npm optional dep | ~0 ms |
| 3 | GitHub Releases download + SHA256 verification | ~10–30 s (first time only) |

The downloaded binary is cached at `~/.cache/backlogit/bin/backlogit-{version}[.exe]` and reused on subsequent starts.

### Supported platforms

| Platform | npm key |
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
3. **Install natively**: Use Option 3 to put `backlogit` on PATH and bypass all npm resolution.
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

If the binary download failed during postinstall, the server will attempt a download on first use. You can force a fresh download by removing the cache:

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
