---
description: "Backlogit workflow rules for query-first lookup, dependency-aware execution, checkpoints, and traceability"
applyTo: '**'
---

# Backlogit Instructions

Use these rules when backlogit is the active workflow system in the workspace.

## Required tool surface

When the workspace exposes these operations, use them instead of inventing a parallel tracking system:

* query or SQL lookup for targeted backlog state
* queue views for ready work selection
* dependency operations for explicit ordering
* memory or checkpoint operations for continuity
* comment operations for notable execution outcomes
* sync or rehydrate operations after out-of-band edits
* commit tracking for traceability
* hook event polling for priority signals at session start (`backlogit_poll_hook_events`, `backlogit_ack_hook_events`)

## Query-first protocol

When inspecting backlog state:

1. Prefer targeted backlogit queries over reading many `.backlogit/` markdown files directly.
2. Use direct item retrieval for current-state lookups.
3. Fall back to file reads only when the query surface cannot answer the question.

The goal is token-efficient lookup, not ceremony.

## Queue and dependency protocol

When selecting work or establishing execution order:

1. Prefer queue-aware operations for ready-work selection.
2. Encode real sequencing with explicit dependencies when the graph can represent it.
3. Re-check unfinished dependencies before claiming work that appears ready.
4. Do not hide critical sequencing only in prose when backlogit can store it directly.

## Continuity protocol

At meaningful boundaries such as task completion, review handoff, or session end:

1. Write the markdown memory artifact required by the harness.
2. When checkpoint or memory operations are available, also persist a concise structured summary through backlogit.
3. Summaries should capture outcome, changed files or surfaces, decisions, blockers, and next steps.
4. Do not dump raw transcript logs into backlogit memory fields.

## Traceability protocol

When work changes backlog state materially:

1. Append concise comments for notable outcomes, blocked conditions, or handoff notes when supported.
2. Associate commits with task IDs when commit tracking is supported.
3. Keep comments focused on operational facts.

## Write-only discipline

Agents MUST NOT write directly to `.backlogit/` files. All mutations must go through backlogit CLI commands or MCP tools. Direct file edits bypass validation, event logging, index maintenance, and naming conventions enforced by the tool surface.

## Hierarchy ordering rule

When creating tasks, you MUST provide a `parent_id` referencing an existing feature. Create the parent feature first if one does not exist. When adding items to a shipment, always add the parent feature before its child tasks so dependency and hierarchy constraints resolve correctly.

## Index freshness rule

If `.backlogit/` content was edited outside the usual backlogit mutation flow, refresh the index before relying on query or queue output.

## Data ownership rule

Treat backlogit's markdown files as the current-state source of truth, its query index as a disposable cache, and its event or telemetry streams as tool-managed history. Do not edit generated cache artifacts directly.
