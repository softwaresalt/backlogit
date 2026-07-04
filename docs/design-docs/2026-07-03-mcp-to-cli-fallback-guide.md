---
chunk_strategy: h1-h2-h3
description: 'CLI fallback guide for agents operating when the backlogit MCP server is unavailable — directs to .autoharness/backlog-registry.yaml as the single source of truth for the MCP→CLI operation pairing (guarded by the U2 drift test), explains the export-command-map / metadata catalog discoverability aids, names the MCP-only tools that have no CLI fallback, and documents the durable hook-queue posture.'
doc_type: design
docline:
    date: 2026-07-03T00:00:00Z
    status: accepted
    tags:
        - cli-fallback
        - mcp
        - registry
        - discoverability
        - session-continuity
        - parity
schema_version: "1.0"
source: docs/design-docs/2026-07-03-mcp-to-cli-fallback-guide.md
title: 'MCP-to-CLI Fallback Guide (078-F, E16F4664 phase-1)'
---

# MCP-to-CLI Fallback Guide

## Overview

When an agent normally drives backlogit through the MCP server (`backlogit mcp`)
but the server is unavailable — a crashed daemon, a stdio transport failure, or an
environment that never launched it — the same operations remain reachable through
the `backlogit` command-line binary. This guide tells an agent **where to find the
authoritative MCP→CLI mapping**, how to discover the live command and tool
surfaces, and which operations have **no** CLI fallback so it never invents a
phantom command path.

This guide is deliberately **narrative plus a pointer to the single source of
truth**. It does not restate a second, hand-maintained mapping table. A parallel
table would reproduce the exact drift anti-pattern that feature 078-F set out to
fix (see `docs/reviews/2026-07-03-cli-mcp-parity-matrix.md`): two copies of the
same mapping inevitably diverge.

## The single source of truth: `.autoharness/backlog-registry.yaml`

The authoritative MCP→CLI pairing lives in the operation map at
`.autoharness/backlog-registry.yaml`. Each entry under `operations:` records:

* `mcp_tool` — the `backlogit_`-prefixed MCP tool name (e.g. `backlogit_create_item`).
* `cli_command` — the equivalent CLI invocation template, with `{{param}}`
  placeholders (e.g. `backlogit add --type {{artifact_type}} --title {{title}}`),
  **when a CLI fallback exists**.
* `mcp_only: true` — a machine-checkable marker on every registered MCP tool that
  has **no** CLI counterpart. When this is present there is no `cli_command`, and
  an agent must not assume one exists.
* `params` — the mapping from the operation's logical parameter names to the
  MCP argument / CLI flag names.

To resolve a fallback: look up the operation whose `mcp_tool` matches the tool you
would have called, then run its `cli_command`, substituting the `{{param}}`
placeholders from the `params` map. If the operation carries `mcp_only: true`,
there is no CLI path — see [Operations with no CLI fallback](#operations-with-no-cli-fallback).

### Why this file, and how it stays honest

This map is **guarded by an automated drift-detection test**
(`internal/cli/registry_parity_test.go`, unit U2 of 078-F). The test drives its
enumeration from the live MCP tool set (`mcp.Server.ListTools()`) and the real
cobra command tree (`core.DescribeCLICommands`), and fails CI when:

1. a registered MCP tool has neither a resolvable `cli_command` nor `mcp_only: true`
   (a missing or under-specified row);
2. a registry `cli_command` points at a cobra command that does not exist
   (an over-claim);
3. a registry `mcp_tool` names a tool absent from the live registration set
   (an orphan); or
4. the exported discovery surfaces drift from the live CLI and MCP surfaces.

Because the registry is enforced against the running binary, an agent can trust it
rather than re-deriving the mapping from source. When a new tool is added without a
fallback decision, the drift test fails until the registry records either a real
`cli_command` or an explicit `mcp_only: true` — so the file cannot silently rot.

## Command and tool discoverability

Two artifacts help an agent enumerate **what exists** on each surface. Neither of
them encodes the MCP→CLI pairing — that pairing lives only in the registry above.
They render two **disjoint** lists:

* `backlogit metadata catalog` — a unified workspace metadata catalog that
  includes the flattened CLI command tree and the registered MCP tool list, among
  other workspace facts.
* `backlogit metadata export-command-map <path> [--format markdown|json]` — writes
  a command-map artifact rendered from `core.DescribeCLICommands` (the CLI list)
  and the MCP server's `DescribeTools()` (the tool list).

Use these to answer "does a CLI command / MCP tool named X exist?" — **not** "what
is the CLI equivalent of MCP tool X?". The binary that renders them does not read
`.autoharness/backlog-registry.yaml`, so the exported command-map cannot tell you
the pairing. For the pairing, always return to the registry.

> Phase-2 enhancement (out of scope here): extend `ToolInfo` +
> `RenderCommandMapMarkdown` to emit a per-tool `cli_command` / `mcp_only` field
> sourced from the operation map, so the exported command-map artifact itself
> carries the pairing. Until then, the registry is the only source.

## Grouped, consistent help conventions

The CLI groups related operations under noun subcommand trees, so an agent that
knows the MCP tool's domain can usually find the CLI verb by exploring the matching
group's `--help`:

* `backlogit shipment …` — `create`, `get`, `list`, `claim`, `ship`, `add`,
  `return-blocked` (mirrors the `*_shipment` / `add_to_shipment` tools).
* `backlogit checkpoint …` — `create`, `get`, `list`, `resolve`, `cleanup`
  (mirrors the `*_checkpoint` / `cleanup_checkpoints` tools). This lifecycle is
  now complete on the CLI, so an agent can persist and recover session state in
  CLI-fallback mode during an MCP outage.
* `backlogit stash …` — `add`, `get`, `edit`, `list`, `archive` (alias `remove`),
  `harvest` (mirrors the `stash*` / `fetch_stash` / `harvest_stash` tools).
* `backlogit dep …` — `add`, `remove`, `list` (mirrors the `*_dependency` tools).
* `backlogit docs …` — `lint`, `migrate`, `scope`, `classify` (mirrors the
  `docs_*` tools).
* `backlogit metadata …` — `catalog`, `export-command-map` (mirrors
  `get_metadata_catalog` / `export_command_map`).
* `backlogit telemetry …` — `harvest` and reporting verbs (`harvest` mirrors
  `telemetry_harvest`; the reporting verbs are CLI-only).

Positional arguments follow the sibling convention within a group (for example,
`backlogit shipment add <shipment-id> <item-id>` matches `backlogit shipment get
<id>`), so an agent can infer argument order from a neighboring command.

## Operations with no CLI fallback

The following registered MCP tools are **intentionally MCP-only** — they carry
`mcp_only: true` in the registry and have no CLI command. An agent operating in
CLI-fallback mode must treat these as unavailable rather than guessing a command:

* `backlogit_append_comment` — comment append is an MCP-side convenience.
* `backlogit_save_memory` — agent memory persistence is MCP-side; the CLI equivalent
  for durable session state is `backlogit checkpoint create`.
* `backlogit_log_telemetry` — event logging is MCP-side (the CLI exposes telemetry
  **reporting** and `harvest`, not single-event logging).
* `backlogit_poll_hook_events` / `backlogit_ack_hook_events` — hook-queue consumption
  is MCP-only (see the hook-queue posture below).
* `backlogit_get_wit_metadata`, `backlogit_list_types`, `backlogit_list_templates` —
  discovery helpers; use `backlogit metadata catalog` for a broad workspace view.
* `backlogit_merge_sync` — merge-time index reconciliation is MCP-only.
* `backlogit_add_link`, `backlogit_remove_link`, `backlogit_get_links` — there is no
  `backlogit link` command; semantic-link management is MCP-only.

The registry's `mcp_only: true` markers are the machine-checkable version of this
list; this prose enumeration is a convenience and the drift test keeps the two in
agreement.

## Hook-queue posture during an MCP outage

Hook events are **persisted durably** to `.backlogit/hooks_queue.jsonl`. If the MCP
server is down, events are not lost — they accumulate in that file and remain
available for consumption once the server is back. The consumption verbs
(`poll_hook_events` / `ack_hook_events`) are MCP-only, so an agent in CLI-fallback
mode cannot acknowledge events, but it also does not need to: the queue survives the
outage and the next MCP-connected session drains it. An agent should therefore
neither hand-edit nor truncate `.backlogit/hooks_queue.jsonl` during an outage.

## Quick procedure

1. Identify the MCP tool you intended to call (the `backlogit_…` name).
2. Open `.autoharness/backlog-registry.yaml` and find the operation whose
   `mcp_tool` equals that name.
3. If it has a `cli_command`, run it, substituting `{{param}}` placeholders from the
   operation's `params` map.
4. If it has `mcp_only: true`, there is no CLI fallback — defer that operation until
   MCP is restored.
5. To confirm a command or tool exists, consult `backlogit metadata catalog` or
   `backlogit metadata export-command-map` — but return to the registry for the
   pairing itself.
