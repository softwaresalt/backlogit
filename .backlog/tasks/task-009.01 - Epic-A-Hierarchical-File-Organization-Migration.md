---
id: TASK-009.01
title: 'Epic A: Hierarchical File Organization & Migration'
status: To Do
assignee: []
created_date: '2026-03-31 06:03'
labels:
  - sub-epic
  - phase-1
dependencies: []
references:
  - .backlog/plans/2026-03-31-queue-features-v2-plan.md
parent_task_id: TASK-009
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Restructure artifact storage from flat per-type directories (tasks/, bugs/, epics/) to a single `.backlogit/queue/` folder with hierarchical numeric naming (001, 001.001, 001.001.001). Add configurable WIT-to-level mappings in config.yaml and a migration pipeline to convert existing flat layouts.

Phase 1 (Foundation). Units 1-2 from the implementation plan.
<!-- SECTION:DESCRIPTION:END -->
