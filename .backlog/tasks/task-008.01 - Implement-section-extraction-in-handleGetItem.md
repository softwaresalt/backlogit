---
id: TASK-008.01
title: Implement section extraction in handleGetItem
status: Done
assignee: []
created_date: '2026-03-31 05:40'
updated_date: '2026-03-31 22:13'
labels:
  - mcp
  - sections
dependencies: []
parent_task_id: TASK-008
priority: medium
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
The section param on backlogit_get_item is declared in the tool schema but the handler only reads from the DB cache and never parses the Markdown body. When section is provided, the handler should: (1) resolve the artifact file path via FindArtifactPath, (2) read the file from disk, (3) call ParseSections on the body, (4) return the named section content only.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 backlogit_get_item with section param returns only the content of the named section
- [ ] #2 backlogit_get_item with no section param continues to return full artifact from DB cache
- [ ] #3 missing section returns a descriptive error not empty output
<!-- AC:END -->
