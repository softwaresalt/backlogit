---
title: backlogit vs Backlog.md
description: Feature comparison and architectural differences
author: backlogit contributors
ms.date: 2026-04-01
ms.topic: concept
keywords:
  - backlogit
  - backlog.md
  - comparison
  - migration
---

## Introduction

Backlog.md and backlogit are both markdown-native, Git-friendly, and AI-aware. Both expose MCP-based workflows for agents. Both aim to keep project state local to the repository rather than forcing every operation through a hosted SaaS tracker.

The real difference is not "AI support versus no AI support" or "MCP versus no MCP." The real difference is architectural emphasis. Backlog.md is a polished markdown task manager with strong CLI, browser, and agent workflows. backlogit is a configurable local work-item platform with typed schemas, queryable caches, richer metadata, and explicit portability to upstream Agile systems.

## Feature Comparison

| Area                        | Backlog.md                                                                | backlogit                                                                                                         |
| --------------------------- | ------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------- |
| Storage architecture        | Project-local backlog folders with markdown task, doc, and decision files | `.backlogit/` Markdown artifacts plus SQLite cache and JSONL streams                                              |
| AI integration              | MCP and CLI workflows for agents are part of the upstream design          | MCP plus CLI workflows and type-aware agent surfaces                                                              |
| Primary emphasis            | Task-manager and board workflow with strong browser and CLI UX            | Configurable local work-item system of record for humans and agents                                               |
| Query surface               | Search, list, board, browser, and task-focused workflows                  | Read-only SQL, FTS search, filtered lists, dependency-aware queue, WIT metadata, template discovery               |
| Workflow modeling           | Opinionated task/docs/decisions flow documented upstream                  | User-defined artifact types, field schemas, named templates, routing rules, and queue hierarchy                   |
| Metadata model              | Rich task metadata oriented around Backlog.md's workflow                  | Rich artifact metadata plus custom fields, `external_map` translation, commit links, hierarchy metadata           |
| Hierarchy and dependencies  | Supports subtasks and dependencies                                        | Supports dependencies plus configurable parent-child structure, queue levels, `level`, and `hierarchy_path`       |
| Type introspection          | Public docs focus on commands and workflows                               | Agents can inspect configured work item types, fields, defaults, sections, and hierarchy through MCP              |
| History and telemetry       | Public docs emphasize markdown files and tool workflows                   | Per-item JSONL logs plus telemetry separate current state from history                                            |
| External-system portability | Not a primary design theme in public docs                                 | Designed for mapping and integration through field schemas, `external_map`, and portable metadata                 |

## Architectural Differences

Both tools keep markdown files in the repository. backlogit adds two extra layers around that file model.

The first extra layer is schema. backlogit separates workspace behavior from work-item semantics. `config.yaml` defines types, naming, and hierarchy rules. `header-def.yaml` defines per-type fields, defaults, required and optional values, and immutable system fields. Templates define named sections for each type. This gives backlogit a configurable work-item model rather than one built-in task model.

The second extra layer is query and history infrastructure. backlogit maintains an ephemeral SQLite index for token-efficient reads and append-only JSONL streams for events and telemetry. That lets the Markdown files stay concise while the system still supports search, SQL queries, queue operations, dependency lookups, and traceability.

This is the real architectural distinction: Backlog.md is a task-management workflow built on markdown files, while backlogit is a configurable work-item platform built on markdown files.

## Token Efficiency

For AI agents working inside context-limited LLMs, token cost matters. Backlog.md already offers search, list, and MCP workflows, so it would be inaccurate to describe it as "just load the whole file." The advantage backlogit claims here is narrower and more specific: it gives agents a relational query surface over an indexed cache.

backlogit allows the agent to ask precisely:

```sql
SELECT id, title FROM items WHERE type='bug' AND status='blocked' ORDER BY updated_at DESC
```

This query returns the relevant rows in a small, predictable structure. Context window capacity goes toward reasoning and code generation rather than translating tool output back into an internal table the model has to reconstruct on the fly.

The difference is not that Backlog.md has no query surface. The difference is that backlogit makes queryability part of the storage architecture itself.

## Where backlogit Genuinely Differs

backlogit earns its place when a team needs capabilities that go beyond a polished task manager.

It lets users compose workflow semantics in layers. A team can define artifact types and naming in `config.yaml`, define required and optional fields in `header-def.yaml`, define per-type document sections in templates, and route items by status or type through `registry.yaml`.

It treats metadata as a first-class portability boundary. Fields such as sprint, owner, assignee, labels, dependencies, references, commit links, custom fields, and hierarchy metadata are stored explicitly, not inferred from prose. `external_map` exists so local values can translate cleanly into upstream tracker-specific representations such as Azure DevOps, Jira, or GitHub Issues, even though automatic outbound sync is still future work.

It gives agents introspection APIs instead of forcing hardcoded assumptions. `backlogit_get_wit_metadata`, `backlogit_list_types`, and template discovery let an agent inspect the configured workflow before it starts mutating work items.

It separates current state from history. Markdown files hold the current artifact state, SQLite supports queries, and JSONL captures events and telemetry.

## When to Use Each Tool

| Scenario                                                                      | Recommended Tool                                                                                       |
| ----------------------------------------------------------------------------- | ------------------------------------------------------------------------------------------------------ |
| You want a mature markdown task manager with strong browser and CLI UX        | Backlog.md                                                                                             |
| You want local markdown files plus a configurable work-item schema            | backlogit                                                                                              |
| You need custom work item types, custom fields, and template-defined sections | backlogit                                                                                              |
| You want a simple task/docs/decisions workflow with minimal configuration     | Backlog.md                                                                                             |
| You need dependency-aware queueing and SQL-based agent queries                | backlogit                                                                                              |
| You want MCP-connected agent workflows out of the box                         | Either, depending on whether you prefer opinionated task management or configurable work-item modeling |
| You need portable metadata that can map cleanly into upstream Agile systems   | backlogit                                                                                              |
| You need explicit event history, telemetry, and commit traceability           | backlogit                                                                                              |

## Where Backlog.md Is Strong

Backlog.md deserves credit for several things this comparison should not minimize. The current upstream project is markdown-native, AI-aware, MCP-capable, supports dependencies, and provides a polished browser experience. If your team wants a ready-made task-management workflow with less configuration work, Backlog.md may be the better fit.

The case for backlogit begins when the workflow itself needs to be modeled, queried, and translated, not merely managed.

## Compatibility Note

backlogit's Backlog.md interoperability adapter supports two source shapes:

- modern structured Backlog.md workspaces rooted at `backlog/` or `.backlog/`, with imports focused on task-like directories such as `tasks/`, `drafts/`, `completed/`, `archive/`, and `milestones/`
- older checklist-style backlog files

That still does not mean full-fidelity import of every current Backlog.md document class. Work-item migration is the primary path today. Documentation and decision directories are not and will not be imported because documentation and decisions are a separate concern from workflow management and should be treated separately.  Our position is that such documentation should exist in a root "docs" folder as distinct, first-class artifacts of the project workspace.

