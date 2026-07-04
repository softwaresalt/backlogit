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
title: 'CLI/MCP Command Parity Audit Matrix (078-F, E16F4664 phase-1)'
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
`.autoharness/backlog-registry.yaml`. That registry is guarded by the U2 drift
test `internal/cli/registry_parity_test.go`, which checks command **existence**:
it verifies that each registry `cli_command` resolves to a real cobra command (or
is intentionally `mcp_only`). This matrix goes one level deeper. It verifies
per-row **flag-level** parity — that the parameters each mapped MCP tool accepts
line up with real flags or positional arguments on the target command — which the
drift test does not assert.

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
| `backlogit_add_link` | `— (mcp_only)` | `over-claim-fixed` | Registry pointed at non-existent `backlogit link add`; corrected to mcp_only; real CLI deferred to phase-2 |
| `backlogit_remove_link` | `— (mcp_only)` | `over-claim-fixed` | Registry pointed at non-existent `backlogit link remove`; corrected to mcp_only; real CLI deferred to phase-2 |
| `backlogit_get_links` | `— (mcp_only)` | `over-claim-fixed` | Registry pointed at non-existent `backlogit link list`; corrected to mcp_only; real CLI deferred to phase-2 |
| `backlogit_log_telemetry` | `— (mcp_only)` | `intentional-mcp-only-permanent` | Telemetry is written by the MCP server's own instrumentation; no operator-facing CLI verb is meaningful |
| `backlogit_ack_hook_events` | `— (mcp_only)` | `deferred-phase-2` | True gap; mcp_only for now; tracked in follow-up stash 6C6ACE00 |
| `backlogit_poll_hook_events` | `— (mcp_only)` | `deferred-phase-2` | True gap; mcp_only for now; tracked in follow-up stash 6C6ACE00 |
| `backlogit_append_comment` | `— (mcp_only)` | `deferred-phase-2` | True gap; mcp_only for now; tracked in follow-up stash 6C6ACE00 |
| `backlogit_save_memory` | `— (mcp_only)` | `deferred-phase-2` | True gap; mcp_only for now; tracked in follow-up stash 6C6ACE00 |
| `backlogit_get_wit_metadata` | `— (mcp_only)` | `deferred-phase-2` | True gap; mcp_only for now; tracked in follow-up stash 6C6ACE00 |
| `backlogit_list_types` | `— (mcp_only)` | `deferred-phase-2` | True gap; mcp_only for now; tracked in follow-up stash 6C6ACE00 |
| `backlogit_list_templates` | `— (mcp_only)` | `deferred-phase-2` | True gap; mcp_only for now; tracked in follow-up stash 6C6ACE00 |
| `backlogit_merge_sync` | `— (mcp_only)` | `deferred-phase-2` | True gap; mcp_only for now; tracked in follow-up stash 6C6ACE00 |

Row totals by classification: 30 parity + 9 stale-fixed + 3 missing-fixed +
2 gap-filled + 3 over-claim-fixed + 1 intentional-mcp-only-permanent +
8 deferred-phase-2 = 56.

## True Gap Disposition

The 14 tools below had genuinely no CLI command before this feature. Each is
dispositioned explicitly so none reads as a silent omission.

* `backlogit_add_to_shipment` — gap-filled (U3, phase-1). New `backlogit shipment add` verb attaches an item to a shipment
* `backlogit_create_checkpoint` — gap-filled (U6, phase-1). New `backlogit checkpoint create` verb writes a continuity checkpoint
* `backlogit_add_link` — deferred to phase-2 stash 6C6ACE00. Needs a real `backlogit link` command family before a CLI verb can exist
* `backlogit_remove_link` — deferred to phase-2 stash 6C6ACE00. Belongs to the same unbuilt `backlogit link` family
* `backlogit_get_links` — deferred to phase-2 stash 6C6ACE00. Belongs to the same unbuilt `backlogit link` family
* `backlogit_log_telemetry` — intentional MCP-only (permanent). Telemetry is emitted by the MCP server's own instrumentation, so an operator CLI verb carries no meaning
* `backlogit_ack_hook_events` — deferred to phase-2 stash 6C6ACE00. Hook-event lifecycle is not yet surfaced to the CLI
* `backlogit_poll_hook_events` — deferred to phase-2 stash 6C6ACE00. Paired with hook-event acknowledgement; deferred together
* `backlogit_append_comment` — deferred to phase-2 stash 6C6ACE00. Comment traceability CLI verb not yet designed
* `backlogit_save_memory` — deferred to phase-2 stash 6C6ACE00. Agent-continuity memory CLI verb not yet designed
* `backlogit_get_wit_metadata` — deferred to phase-2 stash 6C6ACE00. Work-item-type introspection CLI verb not yet designed
* `backlogit_list_types` — deferred to phase-2 stash 6C6ACE00. Type-catalog CLI verb not yet designed
* `backlogit_list_templates` — deferred to phase-2 stash 6C6ACE00. Template-catalog CLI verb not yet designed
* `backlogit_merge_sync` — deferred to phase-2 stash 6C6ACE00. Merge-aware sync CLI verb not yet designed

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

The U2 drift test `internal/cli/registry_parity_test.go` guards command
**existence** only — it confirms each registry `cli_command` resolves to a real
cobra command or is intentionally `mcp_only`. It does not assert that the flags
match. This matrix documents that flag-level parity boundary explicitly, which is
Risk R-1 in the plan: an operator or agent could rely on a mapped command whose
flag surface has drifted from the MCP tool's parameters without the drift test
catching it. Until a flag-level assertion is added, this document is the standing
record of that boundary.
