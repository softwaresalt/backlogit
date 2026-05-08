---
title: Why backlogit
description: Design rationale and agent harness value proposition
author: backlogit contributors
ms.date: 2026-04-06
ms.topic: concept
keywords:
  - backlogit
  - rationale
  - design
  - ai agents
  - cqrs
---

## The Fundamental Tension

Every agile tool faces a choice: optimize for humans, or optimize for machines. Humans need files they can read, edit, and merge in Git. AI agents need structured, queryable data that fits inside a context window without wasting tokens on prose they have already seen. These requirements pull in opposite directions, and most tools pick one at the expense of the other.

backlogit was built to resolve that tension. It stores task state in human-readable Markdown with YAML frontmatter, the same format developers use for documentation, ADRs, and changelogs. At the same time, it maintains an ephemeral SQLite cache that agents can query with SQL, returning structured results in a fraction of the tokens that raw file content would require.

## backlogit Is Not Just Another Markdown Task Manager

Current markdown-native tools already solve part of the problem well. Backlog.md, for example, is Git-friendly, AI-aware, and MCP-capable. backlogit does not exist because other local-first tools forgot to add agent support. It exists because some teams need a deeper level of control over their workflow model, metadata model, and integration boundary than a task-manager-first design usually provides.

backlogit treats the backlog as a configurable local work-item system of record. That difference shows up in four places: workflow composition, metadata richness, queryability, and portability to upstream systems.

## Workflow Composition in Three Layers

backlogit is designed so teams can shape their workflow in layers instead of accepting a single built-in process.

1. `config.yaml` defines artifact identity and hierarchy. It controls artifact types, prefixes, naming formats, allowed child relationships, and queue layout. The default queue spans three levels, and the type-to-level mapping is configurable.
2. `header-def.yaml` defines per-type field schemas. It specifies required and optional fields, enum values, defaults, and immutable system-managed fields such as IDs and timestamps.
3. `templates/` and `registry.yaml` define structure and lifecycle. Templates declare named sections for each work item type, while routing rules map item types or statuses to directories such as `queue`, `review`, or `archive`.

That combination lets a team model Scrum, Kanban, a bug triage flow, a feature-harness workflow, or a custom delivery process without rewriting the application.

## Rich Metadata, Not Just Better Titles

backlogit tracks far more than title and status. The artifact model includes fields such as `parent_id`, `sprint`, `priority`, `assigned_to`, `owner`, `labels`, `dependencies`, `references`, `commit`, `custom_fields`, `level`, and `hierarchy_path`.

That richer metadata matters for two reasons. First, it gives agents more context for planning, sequencing, and filtering work. Second, it makes the local file model portable. Custom fields can define `external_map` translation rules so a local value can map cleanly into an upstream system's representation instead of forcing the local Markdown file to adopt vendor-specific field names.

This is the bridge backlogit is designed for: a local Git-friendly source of truth that can still map to systems such as Azure DevOps, Jira, or GitHub Issues when a team needs that integration boundary.

## Queryability Is Part of the Data Model

The SQLite cache is not an accessory. It is part of the architecture. backlogit uses the file layer for durable state, the database layer for query efficiency, and JSONL streams for history and telemetry.

Because work item fields are schema-driven, backlogit can extend the SQLite schema from `header-def.yaml` and make custom fields queryable. An agent can ask for exactly the rows it needs, including status, type, priority, sprint, parentage, and custom fields, instead of scraping prose out of Markdown files.

That is a meaningful difference from tools that expose search and listing commands but do not center their architecture around an explicit, queryable work-item index.

## The CQRS Solution

backlogit applies Command Query Responsibility Segregation at the storage layer. Writes update Markdown files (the permanent source of truth). Reads query the SQLite cache (an ephemeral index rebuilt on demand from the Markdown files).

The token efficiency difference is substantial. Consider a project with 50 active bug reports:

Reading 50 Markdown files directly costs approximately 50,000 tokens of context. An agent would need to load, parse, and reason about all of them to answer a simple question like "which bugs are currently blocked?"

The equivalent SQL query costs roughly 20 tokens:

```sql
SELECT id, title, status FROM items WHERE type='bug' AND status='blocked'
```

The result returns only the rows the agent asked for, in a predictable structure, without context pollution from description prose or frontmatter fields the agent does not need for this task.

This is not a theoretical optimization. It is the reason backlogit's architecture exists.

## The Agent Harness Value Proposition

backlogit functions as an operating system for AI coding agents. It provides:

- A structured work-item store that agents can read and write without consuming the entire workspace in context
- A persistent memory system through `backlogit_save_memory` and `backlogit_create_checkpoint`, so agents resume sessions with relevant context rather than starting cold
- A commit tracking mechanism through `backlogit_track_commit` that connects code changes to the work items that motivated them
- A telemetry stream through `backlogit_log_telemetry` that captures agent execution metrics for debugging and improvement
- A dependency graph and dependency-aware queue that let agents reason about sequencing without manual coordination
- A type-metadata surface through `backlogit_get_wit_metadata`, `backlogit_list_types`, and template discovery so agents can inspect the configured workflow instead of guessing it

An agent running inside Claude Code, GitHub Copilot CLI, or Cursor can query the backlogit workspace to understand what work is queued, what is blocked, and what dependencies exist before choosing its next action. This is qualitatively different from an agent that must infer project state from file contents alone.

## Current repository orchestration model

F015 simplified the repository harness to a two-agent path so future sessions do
not have to reconstruct a larger top-level topology from scattered legacy docs.
`Stage` owns `STASH -> BACKLOG`, and `Ship` owns `SHIPMENT -> SHIPPED`.

The older `backlog-harvester`, `build-orchestrator`, and `pr-review` surfaces
still exist as reusable migration or support tooling, but they are no longer the
primary durable workflow description for this repository.

## MCP Protocol Bridge

The Model Context Protocol provides a standard JSON-RPC 2.0 interface for connecting AI tools to external services. backlogit implements MCP over stdio: the agent starts `backlogit mcp`, and the server exposes its configured tool surface through the protocol's `initialize` handshake.

MCP was chosen because it is client-agnostic. Any MCP-compatible client, including Claude Code, GitHub Copilot CLI, Cursor, and VS Code extensions, connects to the same server without backlogit needing to implement client-specific plugins for each. The protocol also handles capability negotiation, so clients know which tools are available before calling them.

## Design Philosophy

Several decisions shaped backlogit's architecture:

**Git-friendly persistence.** Every artifact is a separate Markdown file with stable YAML frontmatter field ordering. Files merge cleanly, history is transparent, and the workspace travels with the codebase. There are no binary blobs and no database files in the Git history.

**Configurable workflow semantics.** The workflow is not hardcoded into one task model. Teams can define artifact types, field schemas, default values, named sections, hierarchy levels, and routing rules without changing the binary.

**Portable metadata model.** backlogit is designed to keep local files readable while preserving enough structure to map into upstream Agile systems. `external_map` translations, type metadata, and commit links keep that bridge explicit. Full external sync remains deferred.

**Single-binary simplicity.** Installation is one `go install` command. There are no runtime dependencies, no container images, no background services to manage. The SQLite driver uses a pure-Go implementation with no CGo, so cross-compilation works without a C toolchain.

**Workspace containment.** All file operations resolve within `.backlogit/`. Path traversal attempts are rejected at the API layer. The `backlogit_query_sql` MCP tool enforces a read-only gate so agents cannot execute destructive SQL.

**Ephemeral cache.** `backlogit.db` is gitignored and disposable. Deleting it loses nothing. The rehydration engine rebuilds it from the Markdown files in seconds. This means the source of truth is always the files, never the database.

## Three Storage Layers

backlogit separates state across three storage mechanisms, each with a distinct purpose:

The Markdown layer holds current artifact state: title, status, type, description, and custom fields. These files are committed to Git and are the authoritative record. History, comments, and agent traces do not belong here; they live in the event stream.

The SQLite cache enables fast relational queries. It is rebuilt automatically whenever it is missing or stale. Agents and CLI commands read from it for query operations, including full-text search, filtered lists, queue views, dependency lookups, and type-aware metadata access.

The JSONL event model records state transitions and telemetry in append-only files. Per-item logs capture comments and status changes under `.backlogit/logs/{item-id}.jsonl`. `telemetry.jsonl` captures agent execution metrics, while `telemetry-sessions.jsonl` captures harvested Copilot CLI session summaries and server-attributed token usage. These files accumulate over time and support audit and replay workflows.

## Where backlogit Fits Best

backlogit is a strong fit when a team wants all of the following at the same time:

- Git-friendly local files as the source of truth.
- Agent-native querying and orchestration.
- Configurable workflow semantics rather than a fixed task model.
- Rich metadata that can be filtered locally and translated for external systems.
- A durable event trail and agent telemetry without polluting the Markdown artifacts.

That is the niche backlogit is designed to occupy.
