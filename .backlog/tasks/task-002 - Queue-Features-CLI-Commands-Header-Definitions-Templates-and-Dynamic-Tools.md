---
id: TASK-002
title: 'Queue Features: CLI Commands, Header Definitions, Templates, and Dynamic Tools'
status: done
assignee: []
created_date: '2026-03-30 06:53'
labels:
  - epic
dependencies: []
references:
  - .backlog/queue.md
  - .backlog/plans/2026-03-30-queue-features-plan.md
  - .backlog/reviews/2026-03-30-queue-features-plan-review.md
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Evolve backlogit from its foundational core (TASK-001) into a fully functional workspace management tool. This epic covers four feature areas from `.backlog/queue.md`:

1. **Full CLI Command Suite** — add, list, get, update, move, delete, search, query, status commands
2. **Header Definition System** — `header-def.yaml` with per-type immutable field defaults and OP-prefixed IDs
3. **Template System** — `.backlogit/templates/` with section-tagged markdown bodies and registry integration
4. **Dynamic MCP Tool Generation** — Runtime MCP tool creation from registered template definitions

**Problem**: The current CLI has only `init`, `sync`, and `mcp` commands. Artifacts lack per-type field schemas, templates, and section management. MCP tools are statically defined.

**Approach**: Expand the artifact model with queue-specified fields, introduce header-def.yaml for per-type schemas, build a template engine with BEGIN/END section tags, implement all CLI commands, and generate MCP tools dynamically from templates.

**Key Decisions**:
- D1: HTML comment section delimiters (`<!-- BEGIN:{name} -->` / `<!-- END:{name} -->`)
- D2: Separate `header-def.yaml` file (with clear boundary from `config.yaml`)
- D3: JSON arrays in SQLite for slice fields (labels, dependencies, references)
- D4: Dynamic MCP tools generated at startup, not hot-reloaded
- D5: Multi-line stdin input triggered by `-` flag value (Unix convention)
- D6: Configurable ID prefix (default OP) in header-def.yaml

**Review Advisory Findings (P2)**:
- F1: Clarify config source-of-truth boundary between config.yaml and header-def.yaml
- F2: Unit 20 may need splitting for dynamic tool generation
- F3: Backward compatibility with existing TASK-001 naming format
- F4: Ensure slog instrumentation in all CLI commands
<!-- SECTION:DESCRIPTION:END -->
