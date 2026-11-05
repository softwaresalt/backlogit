---
chunk_strategy: h1-h2-h3
description: 'Handoff for the Stage hierarchy prompt correction and validation limits.'
doc_type: memory
schema_version: "1.0"
source: docs/memory/2026-11-05/prompt-builder-stage-hierarchy-handoff.md
title: 'Stage Hierarchy Prompt Correction Handoff'
docline:
  date: '2026-11-05'
  status: handoff
---

## Contract

The approved correction maps implementation units to atomic tasks directly
under a configured covering feature or chore. Subtasks are optional when
allowed and needed. Conceptual plan headings do not require an epic type.
Parent-first ordering, sizing, dependencies, and existing gates remain.

The installed and plugin Stage and harvest artifacts are aligned. The Stage
manifest entry records the local override and its normalized checksum. The
separate Stage-owned `083-DL` decision and backlog changes are not part of
this correction's commit.

## Validation and Remaining Gates

Focused hierarchy, frontmatter, Markdown, plugin, and manifest checks passed.
Go 1.24 vet and `golangci-lint run --timeout=10m` passed. No Go source or test
files were changed.

The Go 1.27 full suite failed, including a core timeout at 30 minutes and
other reported test failures. Go 1.24 focused no-repository tests also failed:
Git subprocesses filter the workspace-contained discovery ceiling, and the
tried contained barriers do not represent the tested no-repository contract.
Do not treat these results as a full-suite pass. Future merge remains subject
to the full CI and review gates; the known PR P1 and P2 findings remain open.
