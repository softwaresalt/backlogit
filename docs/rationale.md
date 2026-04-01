---
title: Why backlogit
description: Design rationale and agent harness value proposition
author: backlogit contributors
ms.date: 2026-04-01
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

## Why Existing Tools Fall Short for AI Agents

**JIRA and Azure DevOps** have rich APIs, but those APIs impose authentication overhead, rate limits, network latency, and complex OAuth flows. An agent running in a local coding session cannot reasonably provision API credentials, and every call adds latency and token cost for the API response JSON.

**GitHub Issues** are accessible through the GitHub API, but the API's rate limits become a constraint for agentic workflows that query dozens of issues per session. The response format includes significant metadata overhead, and there is no SQL query interface for filtering across hundreds of open issues efficiently.

**Plain text files** like `TODO.md` or `BACKLOG.md` contain all the information, but agents must load the entire file into context to find anything. A 500-line backlog file costs thousands of tokens. Filtering for active bugs requires the agent to read, parse, and reason about every line rather than executing a targeted query.

**Backlog.md** improved on plain text with a structured checklist format and Backlog.md's companion tooling. But it still lacks a query layer, a typed event stream, dependency tracking, and MCP protocol support.

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

- A structured task board that agents can read and write without consuming the entire workspace in context
- A persistent memory system through `backlogit_save_memory` and `backlogit_create_checkpoint`, so agents resume sessions with relevant context rather than starting cold
- A commit tracking mechanism through `backlogit_track_commit` that connects code changes to the tasks that motivated them
- A telemetry stream through `backlogit_log_telemetry` that captures agent execution metrics for debugging and improvement
- A dependency graph that lets agents reason about task sequencing without manual coordination

An agent running inside Claude Code, GitHub Copilot CLI, or Cursor can query the backlogit workspace to understand what work is queued, what is blocked, and what dependencies exist before choosing its next action. This is qualitatively different from an agent that must infer project state from file contents alone.

## MCP Protocol Bridge

The Model Context Protocol provides a standard JSON-RPC 2.0 interface for connecting AI tools to external services. backlogit implements MCP over stdio: the agent starts `backlogit mcp`, and the server exposes 21 tools that the agent discovers through the protocol's `initialize` handshake.

MCP was chosen because it is client-agnostic. Any MCP-compatible client, including Claude Code, GitHub Copilot CLI, Cursor, and VS Code extensions, connects to the same server without backlogit needing to implement client-specific plugins for each. The protocol also handles capability negotiation, so clients know which tools are available before calling them.

## Design Philosophy

Several decisions shaped backlogit's architecture:

**Git-friendly persistence.** Every artifact is a separate Markdown file with stable YAML frontmatter field ordering. Files merge cleanly, history is transparent, and the workspace travels with the codebase. There are no binary blobs and no database files in the Git history.

**Single-binary simplicity.** Installation is one `go install` command. There are no runtime dependencies, no container images, no background services to manage. The SQLite driver uses a pure-Go implementation with no CGo, so cross-compilation works without a C toolchain.

**Workspace containment.** All file operations resolve within `.backlogit/`. Path traversal attempts are rejected at the API layer. The `backlogit_query_sql` MCP tool enforces a read-only gate so agents cannot execute destructive SQL.

**Ephemeral cache.** `index.db` is gitignored and disposable. Deleting it loses nothing. The rehydration engine rebuilds it from the Markdown files in seconds. This means the source of truth is always the files, never the database.

## Three Storage Layers

backlogit separates state across three storage mechanisms, each with a distinct purpose:

The Markdown layer holds current artifact state: title, status, type, description, and custom fields. These files are committed to Git and are the authoritative record. History, comments, and agent traces do not belong here; they live in the event stream.

The SQLite cache enables fast relational queries. It is rebuilt automatically whenever it is missing or stale. Agents and CLI commands read from it exclusively; they never scan Markdown files directly for query operations.

The JSONL event stream records state transitions and agent activity in append-only files. `events.jsonl` captures comments and status changes. `telemetry.jsonl` captures agent execution metrics. These files accumulate over time and support audit and replay workflows.
