
<!-- BACKLOGIT MCP GUIDELINES START -->

<CRITICAL_INSTRUCTION>

## BACKLOGIT WORKFLOW INSTRUCTIONS

This project uses backlogit MCP for all task and project management activities.

**CRITICAL GUIDANCE**

- Call `backlogit_get_metadata_catalog` to load the current backlogit workspace model, supported artifact types, status values, queue and stash conventions, and MCP tool inventory.
- Call `backlogit_export_command_map` when you need a cached command reference in the workspace, for example `.github/instructions/backlogit-command-map.md`.
- Use `backlogit_list_types`, `backlogit_list_templates`, and `backlogit_get_wit_metadata` when you need type-specific field, section, or hierarchy details before creating or updating items.

- **First time working here?** Call `backlogit_get_metadata_catalog` IMMEDIATELY to learn the active workflow surface.
- **Already familiar?** Refresh the catalog before creating items if you are unsure whether config or templates changed.
- **When to read it**: BEFORE creating work items, harvesting stash entries, or when you are unsure how to track work.

These tools cover:
- Search-first workflow support through queue, item, and SQL discovery
- The configured feature, task, and subtask hierarchy
- Template sections and type-specific metadata
- The current backlogit CLI and MCP command surface

You MUST read the metadata catalog or the exported command map before relying on stale workflow assumptions.

</CRITICAL_INSTRUCTION>

<!-- BACKLOGIT MCP GUIDELINES END -->

