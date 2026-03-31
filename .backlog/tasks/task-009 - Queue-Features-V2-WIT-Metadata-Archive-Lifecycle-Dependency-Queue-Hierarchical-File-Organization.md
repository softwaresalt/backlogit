---
id: TASK-009
title: >-
  Queue Features V2: WIT Metadata, Archive Lifecycle, Dependency Queue,
  Hierarchical File Organization
status: To Do
assignee: []
created_date: '2026-03-31 06:03'
labels:
  - epic
dependencies: []
references:
  - .backlog/queue.md
  - .backlog/plans/2026-03-31-queue-features-v2-plan.md
  - .backlog/reviews/2026-03-31-queue-features-v2-plan-review.md
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Second evolution of the backlogit feature set. Builds on TASK-002 (CLI/headers/templates) to add: self-describing WIT type system with agent-queryable metadata, archive lifecycle management with commit tracking, cross-level dependency-aware work queues, workflow policy integration, and hierarchical `.backlogit/queue/` file organization with configurable WIT-to-level mappings.

Six implementation epics across three delivery phases:
- Phase 1 (Foundation): Hierarchical file org, migration, dynamic schema, template descriptions, dependency table
- Phase 2 (Type System + Dependencies): Required/optional enforcement, WIT metadata API, dependency wiring, harness status, CLI enhancements
- Phase 3 (Lifecycle + Queue): Archive command, commit tracking, work queue

Review findings (ADVISORY): 1 P1 (DDL injection protection needed in dynamic schema), 5 P2, 2 P3.
<!-- SECTION:DESCRIPTION:END -->
