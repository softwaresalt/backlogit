---
id: TASK-009.02.02
title: Template Self-Description & WIT Metadata API
status: In Progress
assignee: []
created_date: '2026-03-31 06:05'
updated_date: '2026-04-01 04:05'
labels:
  - task
  - phase-1
  - phase-2
dependencies:
  - TASK-002
  - TASK-009.02.01
parent_task_id: TASK-009.02
priority: high
ordinal: 1500
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
**Unit 4 — Template Self-Description & WIT Metadata API**

Make templates self-describing and expose WIT metadata through an MCP API so agents can discover WIT structures without hardcoded knowledge.

Key deliverables:
- Extend `TemplateConfig` in `internal/config/templates.go` with `Description string`, `WITType string`, `AttributeDescriptions map[string]string`
- Extend `SectionDef` with `Description string`, `Required bool`
- New MCP tool: `backlogit_get_wit_metadata` — returns full WIT schema including type hierarchy, fields, allowed values, required/optional status
- Update `backlogit_list_templates` to return template descriptions and field metadata
- New struct `WITMetadata` in `internal/models/`: type name, description, parent types, field definitions with constraints
- CLI: `backlogit templates list --verbose` shows descriptions and field metadata
- Template YAML format extended with `description:`, `wit_type:`, `attributes:` sections

This is the foundation for the agent-queryable WIT system (queue.md requirement #5).
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Templates include description field and attribute descriptions for each section/field
- [ ] #2 backlogit_list_templates MCP tool returns template descriptions and field metadata
- [ ] #3 backlogit_get_wit_metadata MCP tool returns full WIT schema including required/optional fields and enums
- [ ] #4 Agent can discover all WIT types, their fields, allowed values, and relationships via MCP tools alone
- [ ] #5 Template descriptions visible via CLI: backlogit templates list --verbose
<!-- AC:END -->
