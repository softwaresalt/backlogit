---
id: TASK-009.02
title: 'Epic B: WIT Type System & Self-Description'
status: Done
assignee: []
created_date: '2026-03-31 06:03'
updated_date: '2026-04-01 05:20'
labels:
  - sub-epic
  - phase-1
  - phase-2
dependencies: []
references:
  - .backlog/exec-plans/2026-03-31-queue-features-v2-plan.md
parent_task_id: TASK-009
priority: high
ordinal: 3000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Extend the WIT type system with self-describing templates, agent-queryable metadata, dynamic SQLite schema generation from YAML field definitions, and required/optional field enforcement. Provides the foundation for agents to discover WIT structures, relationships, fields, and enums without hardcoded knowledge.

Phase 1-2 (Foundation + Type System). Units 3-5 from the implementation plan.

Review finding F1 (P1): Dynamic DDL generation in Unit 3 needs column name validation regex (`^[a-z][a-z0-9_]{0,62}$`) to prevent SQL injection through crafted field names in config.yaml.
<!-- SECTION:DESCRIPTION:END -->
