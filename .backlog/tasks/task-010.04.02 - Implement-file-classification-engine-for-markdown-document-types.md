---
id: TASK-010.04.02
title: Implement file classification engine for markdown document types
status: To Do
assignee: []
created_date: '2026-04-01 22:38'
labels:
  - go
dependencies:
  - TASK-010.04.01
parent_task_id: TASK-010.04
priority: medium
ordinal: 2000
implementation_notes: |
  Harness command: go test ./internal/parser/... -run "TestClassifier|TestNewClassifier|TestDocumentClassConstants" -v
  Test file: internal/parser/adapter_test.go
  Stub file: internal/parser/adapter.go (Classifier, NewClassifier, Classify, ClassifyDir)
  Execution note: test-first
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Implement a file classification engine that analyzes markdown documents to detect their type (spec, plan, work item, decision) for migration routing.

Content to implement:
- `Classifier` struct with methods: `Classify(path) (DocumentClass, float64, error)` returning class and confidence score
- Document class enum: `Spec`, `Plan`, `WorkItem`, `Decision`, `Note`, `Unknown`
- Heuristic detection rules:
  - YAML frontmatter presence and field analysis (type, status, priority fields suggest work items)
  - Heading structure analysis (ADR-style numbered headings suggest decisions)
  - Checklist density (high checklist ratio suggests work items or plans)
  - Keyword frequency (requirement, acceptance criteria, user story suggest specs)
  - Directory path hints (plans/, decisions/, specs/ suggest corresponding types)
- Batch classification: scan a directory and classify all markdown files
- Integration with migration adapter framework

Files to create:
- `internal/parser/classifier.go` (new: classification engine)
- `internal/parser/classifier_test.go` (new: tests with sample documents)

Verification: `go test ./internal/parser/...` passes; classifier correctly categorizes sample documents.
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 Classifier detects document types (specs, plans, work items, decisions) from heading patterns and frontmatter
- [ ] #2 Heuristic scoring ranks classification confidence for ambiguous documents
- [ ] #3 Classifier integrates with migration adapter framework for source detection
<!-- AC:END -->
