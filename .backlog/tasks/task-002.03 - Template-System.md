---
id: TASK-002.03
title: Template System
status: To Do
assignee: []
created_date: '2026-03-30 06:55'
labels:
  - epic
dependencies: []
parent_task_id: TASK-002
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Build the template engine for standardized artifact body content. Templates live in `.backlogit/templates/` as Markdown files with YAML frontmatter defining section names and CLI flags. Section delimiters use `<!-- BEGIN:{name} -->` / `<!-- END:{name} -->` HTML comments.

Includes: template schema and loader, default templates for 8 artifact types, section parser and writer for reading/writing individual sections programmatically.

Covers plan Units 8-10.
<!-- SECTION:DESCRIPTION:END -->
