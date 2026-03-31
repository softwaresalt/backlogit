---
id: TASK-003
title: Wire sections param through handleCreateItem to template service
status: To Do
assignee: []
created_date: '2026-03-31 00:34'
labels:
  - mcp
  - sections
  - templates
dependencies: []
priority: high
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
handleCreateItemSections in internal/mcp/dynamic.go always returns ("", nil) — it is a permanent stub. ParseSectionsParam correctly handles string/object wire format but is never called from handleCreateItem. NewServer passes nil as the templateSvc to RegisterSectionAwareTools so handleListTemplates always returns "[]".\n\nFix requires: (1) construct a live *templates.Service in NewServer from workspace config, (2) pass it to RegisterSectionAwareTools, (3) call ParseSectionsParam in handleCreateItem and apply section content via WriteArtifactFile.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 backlogit_list_templates returns actual template definitions from workspace config
- [ ] #2 backlogit_create_item sections param writes section content into the created artifact file
- [ ] #3 handleCreateItemSections stub is replaced with real implementation
<!-- AC:END -->
