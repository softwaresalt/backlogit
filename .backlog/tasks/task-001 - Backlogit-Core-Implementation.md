---
id: TASK-001
title: Backlogit Core Implementation
status: To Do
assignee: []
created_date: '2026-03-30 01:35'
labels:
  - epic
dependencies: []
references:
  - .backlog/research/Backlogit-Architecture-Design.md
  - .backlog/plans/2026-03-30-backlogit-core-plan.md
  - .backlog/reviews/2026-03-30-backlogit-core-plan-review.md
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Complete Go implementation of the backlogit file-backed task management system with MCP server, SQLite CQRS cache, and developer CLI.

## Problem Statement

backlogit requires a full Go 1.22+ implementation of a hybrid data architecture (CQRS) combining Markdown files as the source of truth, an ephemeral SQLite cache for token-efficient agent queries, and JSONL streams for event history. The system exposes capabilities through a Cobra CLI for developers and an MCP stdio server for AI agents.

## Approach

Build bottom-up from foundational packages (errors, config, models) through the data layer (parser, database, events) to the interface layer (MCP server, CLI). Each unit is self-contained with its own tests. A prototype exists in `.context/prototype.go` as reference but will be properly decomposed into packages.

## Key Decisions

1. CGo-free SQLite via `modernc.org/sqlite` for single-binary distribution
2. Concurrent rehydration with `errgroup` and `context.Context`
3. Read-only SQL gate via regex patterns for `backlogit_query_sql`
4. Atomic file writes (temp-file-then-rename) for corruption prevention
5. Dependency injection via Server struct (no global mutable state)
6. Workspace orchestration struct coordinates cross-store writes (Markdown + SQLite + JSONL)

## Deferred Scope

- TUI (Bubble Tea) — future plan
- Slash command integrations — future plan
- External sync hooks (Jira, Azure DevOps) — future plan
<!-- SECTION:DESCRIPTION:END -->
