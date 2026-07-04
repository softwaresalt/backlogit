---
chunk_strategy: h1-h2-h3
description: 'CLI fallback guide for agents operating when the backlogit MCP server is unavailable — directs to .autoharness/backlog-registry.yaml as the single source of truth for the MCP→CLI operation pairing (guarded by the registry drift test, extended with flag/positional parity in 079-S U6), explains the export-command-map / metadata catalog discoverability aids, names the two remaining MCP-only tools (log_telemetry, merge_sync), and documents the durable hook-queue posture now that hooks poll/ack have CLI fallbacks.'
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
title: 'MCP-to-CLI Fallback Guide (phase-1 078-F; phase-2 079-S closes 10 gaps)'
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
(`internal/cli/registry_parity_test.go`, introduced as unit U2 of 078-F and
extended with a flag/positional-parity assertion in unit U6 of 079-S). The test
drives its enumeration from the live MCP tool set (`mcp.Server.ListTools()`) and
the real cobra command tree (`core.DescribeCLICommands`), and fails CI when:

1. a registered MCP tool has neither a resolvable `cli_command` nor `mcp_only: true`
   (a missing or under-specified row);
2. a registry `cli_command` points at a cobra command that does not exist
   (an over-claim);
3. a registry `mcp_tool` names a tool absent from the live registration set
   (an orphan);
4. the exported discovery surfaces drift from the live CLI and MCP surfaces; or
5. a registry `cli_command`'s literal `--flag` does not resolve to a real flag on
   the target command, or its `{{positional}}` count violates the command's `Args`
   validator (flag/positional parity, added in 079-S U6).

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

> Future enhancement (still out of scope): extend `ToolInfo` +
> `RenderCommandMapMarkdown` to emit a per-tool `cli_command` / `mcp_only` field
> sourced from the operation map, so the exported command-map artifact itself
> carries the pairing. 079-S closed the CLI-command gaps and added the flag-parity
> gate but did not change the export-command-map surface; until that lands, the
> registry remains the only source of the pairing.

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
* `backlogit metadata …` — `catalog`, `export-command-map`, `types`, `wit`,
  `templates` (mirrors `get_metadata_catalog` / `export_command_map` /
  `list_types` / `get_wit_metadata` / `list_templates`).
* `backlogit link …` — `add`, `remove`, `list` (mirrors the `add_link` /
  `remove_link` / `get_links` tools).
* `backlogit hooks …` — `poll`, `ack` (mirrors the `poll_hook_events` /
  `ack_hook_events` tools).
* `backlogit memory save` — persists keyed session memory (mirrors `save_memory`).
* `backlogit comment add` — appends a comment event (mirrors `append_comment`).
* `backlogit telemetry …` — `harvest` and reporting verbs (`harvest` mirrors
  `telemetry_harvest`; the reporting verbs are CLI-only).

Positional arguments follow the sibling convention within a group (for example,
`backlogit shipment add <shipment-id> <item-id>` matches `backlogit shipment get
<id>`), so an agent can infer argument order from a neighboring command.

## Operations with no CLI fallback

The following registered MCP tools are **intentionally MCP-only** — they carry
`mcp_only: true` in the registry and have no CLI command. An agent operating in
CLI-fallback mode must treat these as unavailable rather than guessing a command:

* `backlogit_log_telemetry` — intentional-permanent MCP-only. Single-event logging
  is MCP-side; the CLI telemetry surface is read/report-only (`telemetry harvest`
  and the reporting verbs), so there is no operator-facing write verb to map to.
* `backlogit_merge_sync` — deferred to phase-3. Merge-time index reconciliation is
  write-by-default and needs a Rule-4 safety design before a CLI verb is exposed.

> Closed in phase-2 (079-S): `add_link` / `remove_link` / `get_links` (now
> `backlogit link add|remove|list`), `poll_hook_events` / `ack_hook_events` (now
> `backlogit hooks poll|ack`), `save_memory` (now `backlogit memory save`),
> `append_comment` (now `backlogit comment add`), and `get_wit_metadata` /
> `list_types` / `list_templates` (now `backlogit metadata wit|types|templates`)
> all gained CLI fallbacks and are no longer MCP-only.

The registry's `mcp_only: true` markers are the machine-checkable version of this
list; this prose enumeration is a convenience and the drift test keeps the two in
agreement.

## Hook-queue posture during an MCP outage

Hook events are **persisted durably** to `.backlogit/hooks_queue.jsonl`. If the MCP
server is down, events are not lost — they accumulate in that file and remain
available for consumption once the server is back. As of phase-2 (079-S U2), the
consumption verbs also have CLI fallbacks — `backlogit hooks poll --consumer-id
{{consumer_id}}` and `backlogit hooks ack --consumer-id {{consumer_id}} --seq
{{seq}}` — so an agent in CLI-fallback mode **can** now drain and acknowledge the
queue over the same durable file the MCP tools use. Whether it consumes during the
outage or lets the next MCP-connected session drain the queue, the events survive
either way. An agent should still neither hand-edit nor truncate
`.backlogit/hooks_queue.jsonl`; use the `hooks` verbs (or the MCP tools) to advance
the consumer checkpoint so sequence tracking stays consistent.

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
