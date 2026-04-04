---
id: TASK-002.05.02
title: Implement dynamic MCP tool generation from templates
status: done
assignee: []
created_date: '2026-03-30 07:02'
labels: []
dependencies:
  - TASK-002.03.01
  - TASK-002.03.03
  - TASK-002.05.01
parent_task_id: TASK-002.05
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Implement `RegisterDynamicTools(s *MCPServer, templates []*TemplateConfig)` in `internal/mcp/dynamic.go`. For each registered template, generate MCP tools:
- `backlogit_create_{type}` — pre-fills artifact type, exposes section-specific string parameters from template section definitions
- `backlogit_update_{type}_section` — accepts `id`, `section_name`, and `content` to update a specific section

Each dynamic tool handler delegates to the core create/update paths with template-aware section writing. Tool descriptions include section names and their required/optional status.

Detect and reject naming collisions with static tools. Call `RegisterDynamicTools` from `NewServer` after loading templates.

**From review F2**: This unit is the most complex. If it exceeds sizing, split schema generation from handler registration.

**Files:** `internal/mcp/dynamic.go` (new)
**Test files:** `internal/mcp/dynamic_test.go` (new)
**Patterns:** Follow `RegisterTools` in `internal/mcp/tools.go`, `LoadTemplates` from TASK-002.03.01
**Verification:** Dynamic tools appear in MCP tool list based on registered templates. Creating an artifact via a dynamic tool produces a file with correct template structure.
<!-- SECTION:DESCRIPTION:END -->

