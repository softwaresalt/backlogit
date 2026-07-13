---
chunk_strategy: h1-h2-h3
description: Closure for 100-F — self-hosted Copilot plugin marketplace, future-proofing plugin distribution.
doc_type: closure
docline:
    ms.date: 2026-07-12T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/closure/2026-07-12-plugin-marketplace-closure.md
title: "100-F self-hosted Copilot plugin marketplace closure"
---

## Problem

The Copilot CLI warns that direct plugin installs
(`copilot plugin install owner/repo`, URLs, local paths) are deprecated and
only `plugin@marketplace` installs will be supported in a future release.
backlogit's plugin worked only via the direct-install path, so distribution
would eventually break.

## Fix (PR #224, merge 11922e4)

- **`.claude-plugin/marketplace.json`** — a self-hosted marketplace indexing
  the `backlogit` plugin via `source: {source: github, repo: softwaresalt/backlogit}`
  (no subpath; the CLI discovers the manifest at `.github/plugin/plugin.json`).
  Schema mirrors the built-in `github/copilot-plugins` marketplace.
- **Drift-guard test** (`tests/integration/plugin_manifest_test.go`) — asserts
  the marketplace indexes the backlogit plugin from the correct GitHub source
  and that the marketplace version exactly matches the canonical plugin
  manifest version (no version drift).
- **Docs** (README, plugin-guide, installation) — document the future-proof
  marketplace install.

## Live verification

Verified inside the workspace boundary (`COPILOT_HOME` is the in-repo
`.copilot/`):

```text
> copilot plugin marketplace add softwaresalt/backlogit
Marketplace "softwaresalt" added successfully.
> copilot plugin install backlogit@softwaresalt
Plugin "backlogit" installed successfully.
> copilot plugin list
  • backlogit@softwaresalt (v1.1.0)
```

No deprecation warning (this is the marketplace path). Users register the
marketplace once, then install the plugin from it:

```bash
copilot plugin marketplace add softwaresalt/backlogit
copilot plugin install backlogit@softwaresalt
```

## Follow-ups

- **License discrepancy (stash B55FF3DC):** the root `LICENSE` file is
  Apache-2.0, but `plugin.json`, the README badge, and the README license
  section declare MIT. The marketplace entry omits `license` to avoid
  advertising a divergent value. The authoritative license and the alignment
  of all metadata is an operator decision.
- Copilot review also drove a harvest-provenance fix: `100-F` was created with
  `add` + `stash archive` instead of `stash harvest`, so the stash-to-feature
  link was reconstructed (`source_stash_*` on `100-F`,
  `reason: harvested` + `harvested_artifact_id` on stash `A17D7DC3`).
