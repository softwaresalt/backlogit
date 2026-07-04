---
chunk_strategy: h1-h2-h3
description: 'Grounded audit matrix mapping all 56 registered backlogit MCP tools to their cobra CLI commands with drift classification and true-gap disposition.'
doc_type: review
docline:
    author: ship-skill
    ms.date: 2026-07-03T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/reviews/2026-07-03-cli-mcp-parity-matrix.md
title: 'CLI/MCP Command Parity Audit Matrix (phase-1 078-F; phase-2 079-S closes 10 gaps + flag-parity gate)'
---

# CLI/MCP Command Parity Audit Matrix

## Overview

This matrix audits all 56 registered MCP tools in the backlogit manifest against
the cobra CLI command tree and the `.autoharness/backlog-registry.yaml` fallback
map. Every row is grounded against the running binary rather than inferred from
source alone, so a tool that the manifest advertises but the CLI never exposes is
surfaced as a real gap instead of an assumed one.

Each tool is assigned a drift classification:

* `stale` — the registry `cli_command` was empty even though a CLI command existed
* `missing` — the tool and its CLI both exist, but no registry row mapped them
* `over-claim` — the registry pointed at a CLI command that does not exist
* `true-gap` — the tool has no CLI command at all
* `CLI-only` — a CLI command exists with no corresponding MCP tool (Class E)

The authoritative MCP-to-CLI fallback mapping lives in
`.autoharness/backlog-registry.yaml`. That registry is guarded by the drift test
`internal/cli/registry_parity_test.go`. As of phase-2 (079-S U6) the drift test
checks parity at **two** levels: command **existence** (each registry
`cli_command` resolves to a real cobra command or is intentionally `mcp_only`)
**and** command **flag/positional parity** (each literal `--flag` resolves to a
real flag on the target command and the `{{positional}}` count satisfies the
command's `Args` validator). This matrix records the resulting parity
disposition per row; the flag-level boundary it formerly documented as unguarded
is now enforced in code.

## The Matrix

The table below lists all 56 registered MCP tools. `CLI Command` shows the mapped
cobra command, or `— (mcp_only)` when the tool is intentionally or provisionally
MCP-only. `Classification` records the parity disposition after the phase-1
corrections landed.

| MCP Tool | CLI Command | Classification | Notes |
| --- | --- | --- | --- |
| `backlogit_add_dependency` | `backlogit dep add` | `parity` | Registry cli_command correct; flag-level parity verified |
| `backlogit_adopt_item` | `backlogit adopt` | `parity` | Registry cli_command correct; flag-level parity verified |
| `backlogit_archive_item` | `backlogit archive` | `parity` | Registry cli_command correct; flag-level parity verified |
| `backlogit_claim_shipment` | `backlogit shipment claim` | `parity` | Registry cli_command correct; flag-level parity verified |
| `backlogit_cleanup_checkpoints` | `backlogit checkpoint cleanup` | `parity` | Registry cli_command correct; flag-level parity verified |
| `backlogit_create_item` | `backlogit add` | `parity` | Registry cli_command correct; flag-level parity verified |
| `backlogit_create_shipment` | `backlogit shipment create` | `parity` | Registry cli_command correct; flag-level parity verified |
| `backlogit_delete_item` | `backlogit delete` | `parity` | Registry cli_command correct; flag-level parity verified |
| `backlogit_doctor` | `backlogit doctor` | `parity` | Registry cli_command correct; flag-level parity verified |
| `backlogit_fetch_stash` | `backlogit stash list` | `parity` | Registry cli_command correct; flag-level parity verified |
| `backlogit_get_checkpoint` | `backlogit checkpoint get` | `parity` | Registry cli_command correct; flag-level parity verified |
| `backlogit_get_dependencies` | `backlogit dep list` | `parity` | Registry cli_command correct; flag-level parity verified |
| `backlogit_get_item` | `backlogit get` | `parity` | Registry cli_command correct; flag-level parity verified |
| `backlogit_get_queue` | `backlogit queue view` | `parity` | Registry cli_command correct; flag-level parity verified |
| `backlogit_get_shipment` | `backlogit shipment get` | `parity` | Registry cli_command correct; flag-level parity verified |
| `backlogit_harvest_stash` | `backlogit stash harvest` | `parity` | Registry cli_command correct; flag-level parity verified |
| `backlogit_list_checkpoints` | `backlogit checkpoint list` | `parity` | Registry cli_command correct; flag-level parity verified |
| `backlogit_list_items` | `backlogit list` | `parity` | Registry cli_command correct; flag-level parity verified |
| `backlogit_list_shipments` | `backlogit shipment list` | `parity` | Registry cli_command correct; flag-level parity verified |
| `backlogit_move_item` | `backlogit move` | `parity` | Registry cli_command correct; flag-level parity verified |
| `backlogit_query_sql` | `backlogit query` | `parity` | Registry cli_command correct; flag-level parity verified |
| `backlogit_remove_dependency` | `backlogit dep remove` | `parity` | Registry cli_command correct; flag-level parity verified |
| `backlogit_resolve_checkpoint` | `backlogit checkpoint resolve` | `parity` | Registry cli_command correct; flag-level parity verified |
| `backlogit_return_blocked` | `backlogit shipment return-blocked` | `parity` | Registry cli_command correct; flag-level parity verified |
| `backlogit_search_items` | `backlogit search` | `parity` | Registry cli_command correct; flag-level parity verified |
| `backlogit_ship_shipment` | `backlogit shipment ship` | `parity` | Registry cli_command correct; flag-level parity verified |
| `backlogit_stash` | `backlogit stash add` | `parity` | Registry cli_command correct; flag-level parity verified |
| `backlogit_sync_index` | `backlogit sync` | `parity` | Registry cli_command correct; flag-level parity verified |
| `backlogit_track_commit` | `backlogit update --commit` | `parity` | Commit association maps to the `--commit` flag on `update` |
| `backlogit_update_item` | `backlogit update` | `parity` | Registry cli_command correct; flag-level parity verified |
| `backlogit_deliberate` | `backlogit deliberate` | `stale-fixed` | Registry cli_command was empty; CLI existed; wired in U2 |
| `backlogit_get_metadata_catalog` | `backlogit metadata catalog` | `stale-fixed` | Registry cli_command was empty; CLI existed; wired in U2 |
| `backlogit_export_command_map` | `backlogit metadata export-command-map` | `stale-fixed` | Registry cli_command was empty; CLI existed; wired in U2 |
| `backlogit_get_version` | `backlogit version` | `stale-fixed` | Registry cli_command was empty; CLI existed; wired in U2 |
| `backlogit_telemetry_harvest` | `backlogit telemetry harvest` | `stale-fixed` | Registry cli_command was empty; CLI existed; wired in U2 |
| `backlogit_stash_get` | `backlogit stash get` | `stale-fixed` | Registry cli_command was empty; CLI existed; wired in U2 |
| `backlogit_stash_edit` | `backlogit stash edit` | `stale-fixed` | Registry cli_command was empty; CLI existed; wired in U2 |
| `backlogit_stash_archive` | `backlogit stash archive` | `stale-fixed` | Registry cli_command was empty; CLI existed; wired in U2 |
| `backlogit_stash_remove` | `backlogit stash archive` | `stale-fixed` | Deprecated tool; CLI `remove` is an alias of `stash archive`; wired in U2 |
| `backlogit_docs_lint` | `backlogit docs lint` | `missing-fixed` | MCP and CLI both existed; registry row added in U2 |
| `backlogit_docs_migrate` | `backlogit docs migrate` | `missing-fixed` | MCP and CLI both existed; registry row added in U2 |
| `backlogit_docs_scope` | `backlogit docs scope` | `missing-fixed` | MCP and CLI both existed; registry row added in U2 |
| `backlogit_add_to_shipment` | `backlogit shipment add` | `gap-filled` | True gap; new CLI verb added this phase (U3) |
| `backlogit_create_checkpoint` | `backlogit checkpoint create` | `gap-filled` | True gap; new CLI verb added this phase (U6) |
| `backlogit_add_link` | `backlogit link add` | `gap-filled-phase-2` | Built in 079-S U1 over `core.AddArtifactLink`; flag-parity verified by the U6 drift assertion |
| `backlogit_remove_link` | `backlogit link remove` | `gap-filled-phase-2` | Built in 079-S U1 over `core.RemoveArtifactLink`; flag-parity verified by the U6 drift assertion |
| `backlogit_get_links` | `backlogit link list` | `gap-filled-phase-2` | Built in 079-S U1 over extracted `core.GetLinks` (nil→[]); flag-parity verified by the U6 drift assertion |
| `backlogit_log_telemetry` | `— (mcp_only)` | `intentional-mcp-only-permanent` | Telemetry is written by the MCP server's own instrumentation; the CLI telemetry surface is read/report-only, so no operator-facing write verb is meaningful |
| `backlogit_ack_hook_events` | `backlogit hooks ack` | `gap-filled-phase-2` | Built in 079-S U2 over `events.AckHookEvents`; flag-parity verified by the U6 drift assertion |
| `backlogit_poll_hook_events` | `backlogit hooks poll` | `gap-filled-phase-2` | Built in 079-S U2 over `events.PollHookEvents`; flag-parity verified by the U6 drift assertion |
| `backlogit_append_comment` | `backlogit comment add` | `gap-filled-phase-2` | Built in 079-S U4 over extracted `core.AppendComment`; flag-parity verified by the U6 drift assertion |
| `backlogit_save_memory` | `backlogit memory save` | `gap-filled-phase-2` | Built in 079-S U3 over `events.SaveMemory`; flag-parity verified by the U6 drift assertion |
| `backlogit_get_wit_metadata` | `backlogit metadata wit` | `gap-filled-phase-2` | Built in 079-S U5 over `core.DescribeType`; flag-parity verified by the U6 drift assertion |
| `backlogit_list_types` | `backlogit metadata types` | `gap-filled-phase-2` | Built in 079-S U5 over `core.ListTypes`; flag-parity verified by the U6 drift assertion |
| `backlogit_list_templates` | `backlogit metadata templates` | `gap-filled-phase-2` | Built in 079-S U5 over `templates.ListTemplates`; flag-parity verified by the U6 drift assertion |
| `backlogit_merge_sync` | `— (mcp_only)` | `deferred-phase-3` | Write-by-default merge-aware sync; a CLI verb needs Rule-4 safety design before it can be flipped; still `mcp_only` |

Row totals by classification: 30 parity + 9 stale-fixed + 3 missing-fixed +
2 gap-filled (phase-1) + 10 gap-filled-phase-2 + 1 intentional-mcp-only-permanent +
1 deferred-phase-3 = 56.

## True Gap Disposition

The 14 tools below had genuinely no CLI command before this feature. Each is
dispositioned explicitly so none reads as a silent omission.

* `backlogit_add_to_shipment` — gap-filled (U3, phase-1). New `backlogit shipment add` verb attaches an item to a shipment
* `backlogit_create_checkpoint` — gap-filled (U6, phase-1). New `backlogit checkpoint create` verb writes a continuity checkpoint
* `backlogit_add_link` — gap-filled (079-S U1, phase-2). New `backlogit link add` verb over `core.AddArtifactLink`
* `backlogit_remove_link` — gap-filled (079-S U1, phase-2). New `backlogit link remove` verb over `core.RemoveArtifactLink`
* `backlogit_get_links` — gap-filled (079-S U1, phase-2). New `backlogit link list` verb over extracted `core.GetLinks`
* `backlogit_log_telemetry` — intentional MCP-only (permanent). Telemetry is emitted by the MCP server's own instrumentation and the CLI telemetry surface is read/report-only, so an operator CLI write verb carries no meaning
* `backlogit_ack_hook_events` — gap-filled (079-S U2, phase-2). New `backlogit hooks ack` verb over `events.AckHookEvents`
* `backlogit_poll_hook_events` — gap-filled (079-S U2, phase-2). New `backlogit hooks poll` verb over `events.PollHookEvents`
* `backlogit_append_comment` — gap-filled (079-S U4, phase-2). New `backlogit comment add` verb over extracted `core.AppendComment`
* `backlogit_save_memory` — gap-filled (079-S U3, phase-2). New `backlogit memory save` verb over `events.SaveMemory`
* `backlogit_get_wit_metadata` — gap-filled (079-S U5, phase-2). New `backlogit metadata wit` verb over `core.DescribeType`
* `backlogit_list_types` — gap-filled (079-S U5, phase-2). New `backlogit metadata types` verb over `core.ListTypes`
* `backlogit_list_templates` — gap-filled (079-S U5, phase-2). New `backlogit metadata templates` verb over `templates.ListTemplates`
* `backlogit_merge_sync` — deferred to phase-3. Merge-aware sync is write-by-default and needs a Rule-4 safety design before a CLI verb can be exposed

## CLI-only Commands (Class E, Intentional)

Several cobra commands have no MCP tool equivalent by design. They are operator
and bootstrap commands that do not belong on the agent-facing MCP surface:

* `backlogit init` — scaffolds a workspace; a bootstrap-only operation
* `backlogit mcp` — starts the MCP server itself; cannot be an MCP tool
* `backlogit manifest` — prints the tool manifest for operator inspection
* `backlogit completion` — generates shell completion scripts
* `backlogit help` — cobra's built-in help command
* `backlogit migrate` — operator-run config/data migration
* Parent group commands (`stash`, `shipment`, `checkpoint`, `dep`, `metadata`, `telemetry`, `docs`, `queue`) — routing parents that dispatch to leaf verbs

These are excluded from parity accounting because the MCP surface intentionally
does not expose them.

## Flag-level Parity Note

For every corrected or existing row above, the registry `params:` entries and the
templated `{{param}}` placeholders map to a real flag or positional argument on
the target command. For example, `backlogit_track_commit` maps its commit SHA
parameter to the `--commit` flag on `backlogit update`, and `backlogit_move_item`
maps its status parameter to the `--status` flag on `backlogit move`. This is
flag-level parity: the mapped command not only exists but accepts the parameters
the MCP tool advertises.

As of phase-2 (079-S U6), the drift test
`internal/cli/registry_parity_test.go` guards command parity at **two** levels:

1. **Existence** — each registry `cli_command` resolves to a real cobra command,
   or the row is intentionally `mcp_only` (mutually exclusive with `cli_command`).
2. **Flag/positional parity** (`TestRegistryParity_FlagAndPositionalParity`) —
   each literal `--flag` in a `cli_command` template resolves to a real
   local/persistent/inherited flag on the target command, and the number of
   `{{positional}}` placeholders satisfies that command's `Args` validator
   (bool-flag and literal-flag-value aware).

This closes Risk R-1 from the plan: a fallback row whose flag surface drifts from
the target command (a typo'd flag or wrong positional count) now fails CI instead
of silently breaking at runtime. When first introduced, the assertion immediately
caught a real latent drift — the `stash` row declared `stash add --text {{text}}`
while `stash add` takes `text` positionally (`--text` exists only on `stash edit`)
— which was corrected to `stash add {{text}}`. This document is no longer the sole
record of the flag-level boundary; it is now enforced in code, and this matrix is
the human-readable companion to that gate.
