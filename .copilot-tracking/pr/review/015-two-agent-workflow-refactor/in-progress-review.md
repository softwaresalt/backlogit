<!-- markdownlint-disable-file -->
# PR Review Status: 015-two-agent-workflow-refactor

## Review Status

* Phase: 4 — remediation complete
* Last Updated: 2026-04-06T11:46:24.1031140-07:00
* Summary: All merged review findings from Phase 3 were fixed on-branch. Shipment schema, MCP pre-init startup, stash rehydration precedence and provenance, shipment lifecycle and persistence safety, and the stale memory path were updated, and the validation gates are clean.

## Branch and Metadata

* Normalized Branch: `015-two-agent-workflow-refactor`
* Source Branch: `015-two-agent-workflow-refactor`
* Base Branch: `origin/main`
* Head SHA: `95f6a04`
* Author: `williamsderek <software.salt@gmail.com>`
* Total Commits on Branch: 13
* Linked Work Items: F015 (branch work also includes broader supporting changes under `.backlogit`, tests, and review-prep artifacts)
* PR-Ref Generated: Manual (`scripts/dev-tools/pr-ref-gen.sh` not present in repo)
* Working Tree Note: Untracked local artifacts are present and not included in the git diff (`.backlogit/checkpoints/`, `docs/memory/[20260406-195954]-015-shipment-validation-review-fixes-memory.md`)

## Phase 1 Log

| Step | Status | Notes |
|------|--------|-------|
| Normalize branch name | ✅ Done | `015-two-agent-workflow-refactor` |
| Create tracking directory | ✅ Done | `.copilot-tracking/pr/review/015-two-agent-workflow-refactor/` |
| Generate pr-reference.xml | ✅ Done | Manual generation from `git diff --find-renames origin/main`; expected script absent |
| Seed tracking document | ✅ Done | This file |
| Parse PR reference | ✅ Done | 278 changed files mapped with hunk ranges |
| Draft PR overview | ✅ Done | See Overview section below |

## PR Overview

This review covers the full current branch and tracked working-tree delta against `origin/main`. The dominant surfaces are Go production code and tests, `.backlogit` artifact/template state, and PR/review support artifacts. The most recent local fixes tightened shipment validation, rollback safety, MCP error mapping, template correctness, placeholder rendering, and error propagation. Earlier branch work still contributes a broad diff footprint, so Phase 3 should treat this as a cross-cutting review rather than a narrow patch inspection.

## Diff Summary

* Total changed files in tracked diff: 278
* Total changed lines (adds + deletes): 13256
* Go files: 42
* Markdown files: 225
* YAML files: 7
* Tracking artifacts already in diff: 2

## Diff Mapping

| File | Type | New Line Range | Old Line Range | Notes |
|------|------|----------------|----------------|-------|
| `.backlog/completed/task-008.04 - Fix-CLI-update-command-UpsertItem-and-bump-updated_at-after-section-writes.md` | deleted | N/A | 1-28 | Markdown content and docs |
| `.backlog/completed/task-001.07 - Event-Telemetry-and-Memory-Streams.md` | deleted | N/A | 1-24 | Markdown content and docs |
| `.github/agents/backlog-harvester.agent.md` | modified | 9-10 | N/A | Markdown content and docs |
| `.backlog/completed/task-009.02.03 - Required-Optional-Field-Enforcement.md` | deleted | N/A | 1-54 | Markdown content and docs |
| `.github/agents/build-orchestrator.agent.md` | modified | 8-9 | N/A | Markdown content and docs |
| `.backlog/completed/task-001.08.06 - Write-MCP-contract-tests.md` | deleted | N/A | 1-42 | Markdown content and docs |
| `.backlog/completed/task-001.10 - Legacy-Migration-Pipeline.md` | deleted | N/A | 1-25 | Markdown content and docs |
| `.backlogit/queue/F015.T011.md` | added | 1-12 | N/A | Backlog artifact state |
| `.github/skills/harness-architect/SKILL.md` | added | 1-96 | N/A | Markdown content and docs |
| `.backlog/tasks/task-010.04.02 - Implement-file-classification-engine-for-markdown-document-types.md` | deleted | N/A | 1-46 | Markdown content and docs |
| `.backlogit/archive/F013.F002.md` | renamed | 2; 14 | N/A; 13 | Markdown content and docs |
| `tests/contract/shipment_tools_test.go` | added | 1-294 | N/A | Contract coverage |
| `.backlog/tasks/task-002.04 - CLI-Command-Suite.md` | deleted | N/A | 1-25 | Markdown content and docs |
| `.context/prototype.go` | deleted | N/A | 1-376 | Supporting changes |
| `internal/models/artifact.go` | modified | 15-24; 29-47 | 15-22; 27-45 | Supporting changes |
| `.github/instructions/backlog-integration.instructions.md` | added | 1-90 | N/A | Markdown content and docs |
| `internal/db/stash.go` | modified | 105-108 | 105-108 | SQLite and rehydration |
| `.backlogit/archive/F013.R001-branch-review.md` | renamed | 2; N/A; N/A; 6-22 | 2-4; 6; 8-10; 12-21 | Markdown content and docs |
| `.backlog/completed/task-009.06 - Epic-F-Workflow-Policy-CLI-Enhancements.md` | deleted | N/A | 1-26 | Markdown content and docs |
| `.backlog/completed/task-002.01.03 - Update-frontmatter-parser-and-serializer-for-new-fields.md` | deleted | N/A | 1-24 | Markdown content and docs |
| `.backlog/tasks/task-010.03 - Positioning-Migration-Documentation.md` | deleted | N/A | 1-22 | Markdown content and docs |
| `.backlog/completed/task-009.04 - Epic-D-Archive-Lifecycle-Management.md` | deleted | N/A | 1-27 | Markdown content and docs |
| `.backlogit/queue/F015.T002.ST004.md` | added | 1-12 | N/A | Backlog artifact state |
| `.backlog/completed/task-001.08.05 - Implement-MCP-resource-handlers.md` | deleted | N/A | 1-37 | Markdown content and docs |
| `.backlog/tasks/task-002.04.05 - Implement-CLI-move-command.md` | deleted | N/A | 1-26 | Markdown content and docs |
| `.backlog/completed/task-001.06 - Rehydration-Engine.md` | deleted | N/A | 1-27 | Markdown content and docs |
| `.copilot-tracking/plan-review/2026-04-05-two-agent-workflow-plan-review.md` | added | 1-185 | N/A | PR tracking artifact |
| `.backlog/completed/task-009.05.02 - Queue-CLI-MCP-Tools.md` | deleted | N/A | 1-42 | Markdown content and docs |
| `.backlogit/queue/F015.T005.md` | added | 1-15 | N/A | Backlog artifact state |
| `.backlog/completed/task-001.10.01 - Implement-legacy-backlog.md-AST-parser.md` | deleted | N/A | 1-41 | Markdown content and docs |
| `.github/skills/spike/SKILL.md` | added | 1-124 | N/A | Markdown content and docs |
| `.backlogit/archive/F013.T002.md` | renamed | 2; 11 | N/A; 10 | Markdown content and docs |
| `.backlogit/queue/F015.T002.ST005.md` | added | 1-12 | N/A | Backlog artifact state |
| `.backlogit/archive/F013.T001.ST001.md` | renamed | 2; 7 | N/A; 6 | Markdown content and docs |
| `.backlog/completed/task-008 - MCP-CLI-Section-Template-Bug-Fixes.md` | deleted | N/A | 1-27 | Markdown content and docs |
| `.backlog/completed/task-002.02.02 - Generate-default-header-def.yaml-in-WriteDefaults.md` | deleted | N/A | 1-30 | Markdown content and docs |
| `.backlog/completed/task-001.05.04 - Implement-read-only-SQL-query-gate.md` | deleted | N/A | 1-38 | Markdown content and docs |
| `.backlog/completed/task-009.02 - Epic-B-WIT-Type-System-Self-Description.md` | deleted | N/A | 1-28 | Markdown content and docs |
| `.github/agents/deliberator.agent.md` | added | 1-114 | N/A | Markdown content and docs |
| `.backlogit/queue/F015.T004.ST002.md` | added | 1-12 | N/A | Backlog artifact state |
| `.backlog/archive/tasks/task-006 - Fix-CLI-move-command-route-relocate-artifact-file-per-registry.yaml-on-status-change.md` | deleted | N/A | 1-33 | Markdown content and docs |
| `.backlog/tasks/task-002.04.06 - Implement-CLI-delete-and-search-commands.md` | deleted | N/A | 1-30 | Markdown content and docs |
| `.backlog/tasks/task-010.04.03 - Create-general-migration-scripts-and-configuration-templates.md` | deleted | N/A | 1-47 | Markdown content and docs |
| `.backlogit/templates/shipment.md` | added | 1-31 | N/A | Template validity and rendering |
| `.backlogit/archive/F013.T003.md` | renamed | 2; 11 | N/A; 10 | Markdown content and docs |
| `.backlog/completed/task-009.06.01 - Harness-Status-Attribute.md` | deleted | N/A | 1-44 | Markdown content and docs |
| `.backlogit/queue/F015.T009.md` | added | 1-12 | N/A | Backlog artifact state |
| `.backlog/completed/task-001.03.01 - Define-Artifact-model-with-status-constants.md` | deleted | N/A | 1-37 | Markdown content and docs |
| `.github/agents/harness-architect.agent.md` | modified | 8-9 | N/A | Markdown content and docs |
| `.backlog/completed/task-002.01.02 - Update-DB-schema-and-queries-for-new-artifact-fields.md` | deleted | N/A | 1-26 | Markdown content and docs |
| `.backlog/tasks/task-002 - Queue-Features-CLI-Commands-Header-Definitions-Templates-and-Dynamic-Tools.md` | deleted | N/A | 1-44 | Markdown content and docs |
| `internal/mcp/errors.go` | modified | 34-43; 53-58; 60-61; N/A | N/A; 43; N/A; 46 | MCP protocol surface |
| `.backlogit/queue/F015.T003.ST002.md` | added | 1-12 | N/A | Backlog artifact state |
| `internal/core/templates/service.go` | modified | 7; 68; 119; 205-208 | N/A; 67; 118; N/A | Core backlog domain |
| `.backlog/completed/task-009.05 - Epic-E-Work-Queue.md` | deleted | N/A | 1-24 | Markdown content and docs |
| `internal/core/shipment_test.go` | added | 1-316 | N/A | Core backlog domain |
| `internal/cli/search.go` | modified | 26 | 26 | Supporting changes |
| `.backlog/completed/task-001.01 - Project-Foundation-and-Error-Hierarchy.md` | deleted | N/A | 1-25 | Markdown content and docs |
| `internal/config/shipment_defaults_test.go` | added | 1-104 | N/A | Workspace config and defaults |
| `docs/compound/config-issues/queue-view-empty-filter-values-2026-04-05.md` | added | 1-136 | N/A | Markdown content and docs |
| `.backlog/tasks/task-002.04.03 - Implement-CLI-get-command.md` | deleted | N/A | 1-26 | Markdown content and docs |
| `.backlog/queue.md` | deleted | N/A | 1-10 | Markdown content and docs |
| `.backlogit/stash.jsonl` | added | 1-12 | N/A | Supporting changes |
| `internal/cli/dep.go` | modified | 38; 65-66; 68; 99 | 38; 65-66; 68; 99 | Supporting changes |
| `.backlog/tasks/task-002.01 - Artifact-Model-Expansion.md` | deleted | N/A | 1-20 | Markdown content and docs |
| `internal/core/artifacts.go` | modified | 11; 441; 529 | N/A; 440; 528 | Core backlog domain |
| `.backlog/tasks/task-010.04 - General-Purpose-Migration-Tooling.md` | deleted | N/A | 1-22 | Markdown content and docs |
| `internal/core/harness_status.go` | modified | 19-20 | 19-20 | Core backlog domain |
| `.backlog/tasks/task-010.03.03 - Write-Backlog.md-to-backlogit-migration-guide.md` | deleted | N/A | 1-43 | Markdown content and docs |
| `.backlog/completed/task-001.11.01 - Implement-environment-detection-and-config-injection.md` | deleted | N/A | 1-45 | Markdown content and docs |
| `.backlog/.claude/instructions.md` | deleted | N/A | 1-25 | Markdown content and docs |
| `.backlog/archive/tasks/task-003 - Implement-section-extraction-in-handleGetItem.md` | deleted | N/A | 1-25 | Markdown content and docs |
| `.backlog/completed/task-009.02.01 - Dynamic-Schema-Extension-Engine.md` | deleted | N/A | 1-47 | Markdown content and docs |
| `.backlog/completed/task-001.08.04 - Implement-system-and-agent-operation-tools.md` | deleted | N/A | 1-38 | Markdown content and docs |
| `docs/memory/2026-04-05/harness-merge-install-memory.md` | added | 1-136 | N/A | Session memory artifact |
| `.backlogit/queue/F015.T008.ST002.md` | added | 1-12 | N/A | Backlog artifact state |
| `.backlog/completed/task-001.08.01 - Implement-MCP-Server-struct-and-stdio-lifecycle.md` | deleted | N/A | 1-41 | Markdown content and docs |
| `.backlog/completed/task-001.01.02 - Implement-error-hierarchy-with-sentinel-and-typed-errors.md` | deleted | N/A | 1-43 | Markdown content and docs |
| `.backlog/completed/task-009.02.02 - Template-Self-Description-WIT-Metadata-API.md` | deleted | N/A | 1-46 | Markdown content and docs |
| `.github/instructions/architecture-doc.instructions.md` | added | 1-54 | N/A | Markdown content and docs |
| `.backlog/tasks/task-010.02.03 - Write-process-and-workflow-documentation.md` | deleted | N/A | 1-42 | Markdown content and docs |
| `.backlog/completed/task-001.09.03 - Implement-query-command-with-formatted-output.md` | deleted | N/A | 1-39 | Markdown content and docs |
| `internal/stash/jsonl.go` | added | 1-146 | N/A | Supporting changes |
| `.backlog/tasks/task-002.04.07 - Implement-CLI-query-and-status-commands.md` | deleted | N/A | 1-30 | Markdown content and docs |
| `internal/core/shipment.go` | added | 1-486 | N/A | Core backlog domain |
| `.backlog/tasks/task-002.03 - Template-System.md` | deleted | N/A | 1-22 | Markdown content and docs |
| `.backlog/tasks/task-002.05.01 - Add-new-fields-and-static-tools-to-MCP-server.md` | deleted | N/A | 1-30 | Markdown content and docs |
| `.backlogit/queue/F015.T005.ST002.md` | added | 1-12 | N/A | Backlog artifact state |
| `.backlog/tasks/task-010.04.04 - Write-general-migration-integration-tests.md` | deleted | N/A | 1-46 | Markdown content and docs |
| `.backlog/completed/task-001 - Backlogit-Core-Implementation.md` | deleted | N/A | 1-44 | Markdown content and docs |
| `internal/cli/move.go` | modified | 24 | 24 | Supporting changes |
| `.backlogit/queue/F015.T005.ST001.md` | added | 1-12 | N/A | Backlog artifact state |
| `internal/cli/delete.go` | modified | 25 | 25 | Supporting changes |
| `.backlog/completed/task-001.11 - MCP-Environment-Registration.md` | deleted | N/A | 1-23 | Markdown content and docs |
| `.github/skills/operational-closure/SKILL.md` | added | 1-32 | N/A | Markdown content and docs |
| `".backlog/completed/task-009.01.02 - Migration-Pipeline-Flat-\342\206\222-Hierarchical.md"` | deleted | N/A | N/A | Supporting changes |
| `.backlogit/queue/F015.T004.ST001.md` | added | 1-12 | N/A | Backlog artifact state |
| `.backlogit/queue/F015.T002.ST003.md` | added | 1-12 | N/A | Backlog artifact state |
| `.backlog/completed/task-001.04.05 - Implement-artifact-creation-with-hierarchy-enforcement.md` | deleted | N/A | 1-40 | Markdown content and docs |
| `.backlog/archive/tasks/task-004 - Fix-CLI-update-command-UpsertItem-and-bump-updated_at-after-section-writes.md` | deleted | N/A | 1-28 | Markdown content and docs |
| `.backlogit/archive/F013.md` | renamed | 2; 16 | N/A; 15 | Markdown content and docs |
| `.backlog/tasks/task-002.06.01 - Full-workflow-integration-tests.md` | deleted | N/A | 1-34 | Markdown content and docs |
| `docs/memory/2026-04-05/two-agent-workflow-continuation-memory.md` | added | 1-59 | N/A | Session memory artifact |
| `.backlog/completed/task-001.04 - Core-Business-Logic.md` | deleted | N/A | 1-26 | Markdown content and docs |
| `internal/core/workspace_internal_test.go` | added | 1-50 | N/A | Core backlog domain |
| `.backlog/.github/copilot-instructions.md` | deleted | N/A | 1-25 | Markdown content and docs |
| `.backlogit/queue/F015.T002.md` | added | 1-18 | N/A | Backlog artifact state |
| `.backlog/completed/task-001.04.01 - Implement-SafeResolve-workspace-containment.md` | deleted | N/A | 1-35 | Markdown content and docs |
| `.backlog/completed/task-002.03.03 - Implement-section-parser-and-writer.md` | deleted | N/A | 1-30 | Markdown content and docs |
| `.backlog/completed/task-001.03 - Models-Frontmatter-and-Markdown-Parser.md` | deleted | N/A | 1-25 | Markdown content and docs |
| `.backlog/completed/task-009.01.01 - Hierarchical-File-Organization-.backlogit-queue.md` | deleted | N/A; N/A | 1-53; 1-47 | Markdown content and docs |
| `.backlog/completed/task-009.01 - Epic-A-Hierarchical-File-Organization-Migration.md` | deleted | N/A | 1-25 | Markdown content and docs |
| `internal/core/templates/service_test.go` | modified | 260-261 | N/A | Core backlog domain |
| `.backlog/tasks/task-010.01.02 - Add-migration-CLI-enhancements-dry-run-progress-validation.md` | deleted | N/A | 1-44 | Markdown content and docs |
| `.backlog/tasks/task-002.04.08 - Register-all-CLI-commands-in-root.md` | deleted | N/A | 1-30 | Markdown content and docs |
| `.backlogit/queue/F015.T001.md` | added | 1-16 | N/A | Backlog artifact state |
| `.github/agents/shipper.agent.md` | added | 1-174 | N/A | Markdown content and docs |
| `.backlog/completed/task-008.05 - Fix-CLI-move-command-route-relocate-artifact-file-per-registry.yaml-on-status-change.md` | deleted | N/A | 1-31 | Markdown content and docs |
| `.backlog/archive/tasks/task-007 - Strengthen-contract-tests-exercise-MCP-handlers-with-real-assertions-not-key-existence-checks.md` | deleted | N/A | 1-33 | Markdown content and docs |
| `.backlog/completed/task-002.03.01 - Implement-template-schema-and-loader.md` | deleted | N/A | 1-30 | Markdown content and docs |
| `.backlogit/archive/F013.R002-followup-review.md` | renamed | 2; N/A; N/A; 6-22 | 2-4; 6; 8-10; 12-21 | Markdown content and docs |
| `.backlog/completed/task-001.09.01 - Implement-Cobra-root-command-and-init.md` | deleted | N/A | 1-43 | Markdown content and docs |
| `.backlog/completed/task-001.05.03 - Implement-parameterized-query-functions.md` | deleted | N/A | 1-42 | Markdown content and docs |
| `.backlogit/queue/F015.T003.ST001.md` | added | 1-12 | N/A | Backlog artifact state |
| `internal/cli/shipment_test.go` | added | 1-92 | N/A | Supporting changes |
| `.backlogit/queue/F015.T006.ST002.md` | added | 1-12 | N/A | Backlog artifact state |
| `.backlog/completed/task-001.01.03 - Create-Makefile-with-build-targets.md` | deleted | N/A | 1-42 | Markdown content and docs |
| `.backlog/archive/tasks/task-008.01 - Fix-template-service-Update-set-updated_at-and-call-UpsertItem-after-section-writes.md` | deleted | N/A | 1-29 | Markdown content and docs |
| `.github/instructions/backlogit.instructions.md` | added | 1-64 | N/A | Markdown content and docs |
| `.backlog/completed/task-001.08 - MCP-Server-and-Tool-Handlers.md` | deleted | N/A | 1-32 | Markdown content and docs |
| `.github/skills/harvest/SKILL.md` | added | 1-128 | N/A | Markdown content and docs |
| `.backlog/completed/task-001.07.01 - Implement-JSONL-event-stream-writer.md` | deleted | N/A | 1-39 | Markdown content and docs |
| `.backlog/memory/2026-03-29/feature-001-checkpoint.md` | deleted | N/A | 1-60 | Markdown content and docs |
| `.backlogit/queue/F015.T001.ST005.md` | added | 1-12 | N/A | Backlog artifact state |
| `.backlogit/queue/F015.T004.ST003.md` | added | 1-12 | N/A | Backlog artifact state |
| `.backlog/completed/task-009 - Queue-Features-V2-WIT-Metadata-Archive-Lifecycle-Dependency-Queue-Hierarchical-File-Organization.md` | deleted | N/A | 1-32 | Markdown content and docs |
| `internal/cli/archive.go` | modified | 23 | 23 | Supporting changes |
| `.copilot-tracking/harness/F015-harness.md` | added | 1-103 | N/A | PR tracking artifact |
| `.backlog/completed/task-001.04.03 - Implement-custom-field-validation.md` | deleted | N/A | 1-36 | Markdown content and docs |
| `internal/core/artifacts_expansion_test.go` | modified | 156-214 | N/A | Core backlog domain |
| `.backlogit/queue/F015.T001.ST004.md` | added | 1-12 | N/A | Backlog artifact state |
| `.gitignore` | modified | 24-25 | N/A | Supporting changes |
| `.github/skills/safety-modes/SKILL.md` | added | 1-58 | N/A | Markdown content and docs |
| `docs/compound/workflow-issues/stable-contract-before-two-agent-adoption-2026-04-05.md` | added | 1-138 | N/A | Markdown content and docs |
| `docs/compound/go-patterns/f015-shipment-stash-patterns.md` | added | 1-298 | N/A | Markdown content and docs |
| `.backlog/completed/task-001.08.02 - Implement-item-write-tools-create-and-update.md` | deleted | N/A | 1-37 | Markdown content and docs |
| `internal/db/rehydration.go` | modified | 92-94; 96; 226-227; 233; 237-269; 271 | 92-93; N/A; 224; 230; 234; 236 | SQLite and rehydration |
| `.backlogit/queue/F015.T001.ST003.md` | added | 1-12 | N/A | Backlog artifact state |
| `.backlog/completed/task-002.01.04 - Update-rehydration-engine-for-new-fields.md` | deleted | N/A | 1-25 | Markdown content and docs |
| `internal/cli/root.go` | modified | 25-26; 69 | 25-26; N/A | Supporting changes |
| `.backlog/tasks/task-010.04.01 - Design-pluggable-migration-adapter-interface.md` | deleted | N/A | 1-42 | Markdown content and docs |
| `.backlogit/header-def.yaml` | modified | 37-48; 95-112 | N/A; N/A | YAML configuration |
| `internal/cli/shipment.go` | added | 1-240 | N/A | Supporting changes |
| `.github/skills/pr-lifecycle/SKILL.md` | added | 1-92 | N/A | Markdown content and docs |
| `.backlog/completed/task-008.06 - Strengthen-contract-tests-exercise-MCP-handlers-with-real-assertions.md` | deleted | N/A | 1-29 | Markdown content and docs |
| `.backlog/tasks/task-010.02.02 - Write-installation-guide.md` | deleted | N/A | 1-41 | Markdown content and docs |
| `.backlog/completed/task-001.01.01 - Initialize-Go-module-and-project-structure.md` | deleted | N/A | 1-39 | Markdown content and docs |
| `.github/agents/pr-review.agent.md` | modified | 6-7 | N/A | Markdown content and docs |
| `.backlog/completed/task-001.02.03 - Create-default-config-and-registry-templates.md` | deleted | N/A | 1-34 | Markdown content and docs |
| `.autoharness/harness-manifest.yaml` | added | 1-131 | N/A | YAML configuration |
| `.backlog/tasks/task-002.04.04 - Implement-CLI-update-command.md` | deleted | N/A | 1-27 | Markdown content and docs |
| `.backlogit/queue/F015.T002.ST001.md` | added | 1-12 | N/A | Backlog artifact state |
| `.backlog/completed/task-002.01.05 - Update-core-CRUD-with-new-fields-and-ID-immutability.md` | deleted | N/A | 1-26 | Markdown content and docs |
| `internal/core/queue.go` | modified | 48-50; 55-57; 63-65; 144-162 | N/A; 52-54; 60-62; N/A | Core backlog domain |
| `.backlogit/queue/F015.T001.ST001.md` | added | 1-12 | N/A | Backlog artifact state |
| `.backlog/tasks/task-010.01 - Backlog.md-Migration-Tooling.md` | deleted | N/A | 1-26 | Markdown content and docs |
| `.backlog/completed/task-001.07.03 - Implement-event-tail-reader.md` | deleted | N/A | 1-36 | Markdown content and docs |
| `.backlog/tasks/task-010.03.01 - Write-tool-rationale-and-agent-harness-value-proposition.md` | deleted | N/A | 1-40 | Markdown content and docs |
| `.backlog/completed/task-001.03.03 - Implement-Markdown-file-parser.md` | deleted | N/A | 1-37 | Markdown content and docs |
| `internal/cli/get.go` | modified | 35 | 35 | Supporting changes |
| `.backlogit/queue/F015.T008.ST001.md` | added | 1-12 | N/A | Backlog artifact state |
| `.backlog/tasks/task-002.04.01 - Implement-CLI-add-command.md` | deleted | N/A | 1-38 | Markdown content and docs |
| `.backlogit/queue/DL001.md` | added | 1-63 | N/A | Backlog artifact state |
| `.backlogit/queue/F015.md` | added | 1-13 | N/A | Backlog artifact state |
| `tests/contract/queue_tools_test.go` | modified | 23-32; 38-50; 58-62; 66-70; 80-81; 108-114; 118-124; 128-141; 149-155; 163-167; 171-189; 197-201; 205-210; 218-222; 230-236; 240-246; 250-259 | 23-32; 38-50; 58-62; 66-70; 80-81; 108-114; 118-124; 128-141; 149-155; 163-167; 171-189; 197-201; 205-210; 218-222; 230-236; 240-246; 250-259 | Contract coverage |
| `internal/cli/migrate_import_test.go` | modified | 209 | 209 | Supporting changes |
| `.backlogit/archive/F013.T003.ST001.md` | renamed | 2; 7 | N/A; 6 | Markdown content and docs |
| `.backlog/completed/task-008.01 - Implement-section-extraction-in-handleGetItem.md` | deleted | N/A | 1-27 | Markdown content and docs |
| `.backlogit/queue/F015.T008.md` | added | 1-21 | N/A | Backlog artifact state |
| `internal/config/defaults.go` | modified | 81-99; 274-305; 338-341; 354 | N/A; N/A; N/A; 299 | Workspace config and defaults |
| `.autoharness/backlog-registry.yaml` | added | 1-186 | N/A | YAML configuration |
| `.backlog/tasks/task-002.02 - Header-Definition-System.md` | deleted | N/A | 1-24 | Markdown content and docs |
| `.backlogit/archive/F013.T002.ST001.md` | renamed | 2; 7 | N/A; 6 | Markdown content and docs |
| `.github/copilot-instructions.md` | modified | 8; 10-14; 16-34; 38-95; 102; N/A; N/A; N/A; N/A; N/A; N/A; N/A; N/A; 113-115; 122; N/A; 126; 129; 134; 136; 138-141; 143; 145-147; 149; 151-152; 154; 156-159; 161; 163; 165-167; 169; 171-174; 176; 178-181; 183; 185; 187; 189; 191-195; 197; 199-216; 218-221; 225; 229-230 | 8; 10; 12; 16-28; 35; 38-43; 45-47; 49-52; 54-58; 60; 62-64; 66-68; 70-72; 74-79; 86-87; 90; 92; N/A; 99; 101; 103; 105; 107; 109; 111; 113; 115; 117; 119-123; 125; 127-132; 134; 136-141; 143; 145-147; 149; 151-155; 157; 159; 161-165; 167; 169-209; 213; 217 | Markdown content and docs |
| `.backlog/tasks/task-002.05.02 - Implement-dynamic-MCP-tool-generation-from-templates.md` | deleted | N/A | 1-34 | Markdown content and docs |
| `internal/cli/query.go` | modified | 25 | 25 | Supporting changes |
| `.backlog/completed/task-001.02 - Configuration-System.md` | deleted | N/A | 1-25 | Markdown content and docs |
| `.backlog/archive/tasks/task-005 - Fix-CLI-update-command-UpsertItem-and-bump-updated_at-after-section-writes.md` | deleted | N/A | 1-28 | Markdown content and docs |
| `.backlogit/queue/F015.T003.ST003.md` | added | 1-12 | N/A | Backlog artifact state |
| `.backlog/completed/task-002.02.01 - Implement-header-def.yaml-schema-and-loader.md` | deleted | N/A | 1-35 | Markdown content and docs |
| `.backlog/completed/task-002.03.02 - Create-default-templates-for-8-artifact-types.md` | deleted | N/A | 1-26 | Markdown content and docs |
| `.backlog/tasks/task-010.02.01 - Write-comprehensive-README.md.md` | deleted | N/A | 1-41 | Markdown content and docs |
| `.backlogit/queue/F015.T010.md` | added | 1-12 | N/A | Backlog artifact state |
| `.backlogit/queue/F015.T007.ST001.md` | added | 1-12 | N/A | Backlog artifact state |
| `internal/stash/stash.go` | modified | 21-22 | N/A | Supporting changes |
| `internal/errors/shipment_errors_test.go` | added | 1-55 | N/A | Supporting changes |
| `internal/cli/queue_cmd.go` | modified | N/A; 52-57; 79-80; 82; 107-108 | 49-50; N/A; 75-76; 78; 103-104 | Supporting changes |
| `.backlogit/archive/F013.T003.ST003.md` | renamed | 2; 7 | N/A; 6 | Markdown content and docs |
| `.backlogit/config.yaml` | modified | 8-10; 29-32; 45-46; 57-58 | N/A; N/A; N/A; N/A | YAML configuration |
| `.backlog/tasks/task-010.01.01 - Enhance-legacy-parser-for-broader-Backlog.md-format-coverage.md` | deleted | N/A | 1-42 | Markdown content and docs |
| `.backlogit/archive/F013.F001.md` | renamed | 2; 14 | N/A; 13 | Markdown content and docs |
| `.backlogit/queue/F015.T007.md` | added | 1-15 | N/A | Backlog artifact state |
| `.backlog/completed/task-001.05 - Database-Foundation.md` | deleted | N/A | 1-26 | Markdown content and docs |
| `.backlog/config.yml` | deleted | N/A | 1-17 | YAML configuration |
| `internal/cli/update.go` | modified | 41 | 41 | Supporting changes |
| `.backlog/completed/task-009.04.02 - Commit-Tracking.md` | deleted | N/A | 1-49 | Markdown content and docs |
| `.backlog/completed/task-001.07.04 - Implement-memory-and-checkpoint-persistence.md` | deleted | N/A | 1-36 | Markdown content and docs |
| `tests/integration/shipment_workflow_test.go` | added | 1-209 | N/A | Integration coverage |
| `.backlog/completed/task-008.02 - Wire-sections-param-through-handleCreateItem-to-template-service.md` | deleted | N/A | 1-21 | Markdown content and docs |
| `.backlogit/queue/F015.T002.ST002.md` | added | 1-12 | N/A | Backlog artifact state |
| `docs/memory/2026-04-05/T008-checkpoint.md` | added | 1-58 | N/A | Session memory artifact |
| `.autoharness/workspace-profile.yaml` | added | 1-144 | N/A | YAML configuration |
| `.backlog/archive/tasks/task-002.01 - MCP-Tool-Expansion-and-Dynamic-Generation.md` | deleted | N/A | 1-22 | Markdown content and docs |
| `.backlog/completed/task-009.06.02 - CLI-Enhancements-Tabular-Listing.md` | deleted | N/A | 1-44 | Markdown content and docs |
| `.backlogit/queue/F015.T006.md` | added | 1-14 | N/A | Backlog artifact state |
| `.backlog/completed/task-009.04.01 - Archive-Command-Lifecycle-Management.md` | deleted | N/A | 1-44 | Markdown content and docs |
| `.backlog/tasks/task-002.04.02 - Implement-CLI-list-command.md` | deleted | N/A | 1-26 | Markdown content and docs |
| `.backlog/completed/task-001.02.02 - Implement-YAML-config-loader-with-env-var-overrides.md` | deleted | N/A | 1-39 | Markdown content and docs |
| `.backlog/memory/2024-01-16/task-010-epic-complete-memory.md` | deleted | N/A | N/A | Markdown content and docs |
| `docs/workflow.md` | modified | 5; 28-71; 247-252 | 5; N/A; N/A | Markdown content and docs |
| `.backlog/completed/task-001.04.02 - Implement-naming-template-resolution.md` | deleted | N/A | 1-37 | Markdown content and docs |
| `.backlog/completed/task-009.03.02 - Dependency-CLI-MCP-Wiring.md` | deleted | N/A | 1-43 | Markdown content and docs |
| `internal/core/wit_metadata.go` | modified | 13-22; 95-101 | 13-22; 95-101 | Core backlog domain |
| `.backlog/completed/task-001.04.06 - Implement-Workspace-orchestration-struct.md` | deleted | N/A | 1-39 | Markdown content and docs |
| `.backlog/completed/task-001.09.04 - Implement-mcp-and-migrate-commands.md` | deleted | N/A | 1-41 | Markdown content and docs |
| `.backlog/completed/task-001.04.04 - Implement-file-routing-from-registry-config.md` | deleted | N/A | 1-37 | Markdown content and docs |
| `docs/rationale.md` | modified | 5; 84-93 | 5; N/A | Markdown content and docs |
| `.backlog/tasks/task-010.03.02 - Write-backlogit-vs-Backlog.md-comparison-document.md` | deleted | N/A | 1-40 | Markdown content and docs |
| `.backlog/completed/task-009.03 - Epic-C-Dependency-Graph.md` | deleted | N/A | 1-27 | Markdown content and docs |
| `.backlogit/queue/F015.T001.ST002.md` | added | 1-12 | N/A | Backlog artifact state |
| `.backlog/.cursor/mcp.json` | deleted | N/A | 1-7 | Supporting changes |
| `.backlogit/queue/F015.T003.md` | added | 1-18 | N/A | Backlog artifact state |
| `.github/instructions/constitution.instructions.md` | modified | 114; 118-121; 124-125; 129-137; 142-146; 150-159; 164-167; 246 | 114; 118-120; 123; N/A; 131-132; 136-142; 147-148; 227 | Markdown content and docs |
| `.backlogit/queue/F015.T007.ST002.md` | added | 1-12 | N/A | Backlog artifact state |
| `.backlog/completed/task-001.02.01 - Define-configuration-schema-structs.md` | deleted | N/A | 1-40 | Markdown content and docs |
| `.backlog/completed/task-002.01.01 - Expand-Artifact-struct-with-queue-specified-fields.md` | deleted | N/A | 1-30 | Markdown content and docs |
| `.backlog/completed/task-001.09.02 - Implement-create-and-sync-commands.md` | deleted | N/A | 1-43 | Markdown content and docs |
| `docs/memory/2026-04-05/two-agent-workflow-design-session.md` | added | 1-214 | N/A | Session memory artifact |
| `internal/mcp/tools.go` | modified | 269-314; 1072-1240 | N/A; N/A | MCP protocol surface |
| `internal/config/defaults_templates_test.go` | modified | 37; 39; 56-57 | 37; 39; 56-57 | Workspace config and defaults |
| `.backlogit/queue/F015.T004.md` | added | 1-19 | N/A | Backlog artifact state |
| `internal/core/queue_test.go` | modified | 77-93 | N/A | Core backlog domain |
| `.github/policies/workflow-policies.md` | added | 1-106 | N/A | Markdown content and docs |
| `.backlog/archive/tasks/task-004 - Fix-template-service-Update-set-updated_at-and-call-UpsertItem-after-section-writes.md` | deleted | N/A | 1-30 | Markdown content and docs |
| `.autoharness/config.yaml` | added | 1-40 | N/A | YAML configuration |
| `.backlog/tasks/task-010.01.03 - Create-migration-configuration-for-document-class-mapping.md` | deleted | N/A | 1-42 | Markdown content and docs |
| `.backlog/completed/task-009.03.01 - Dependency-Table-Junction-Model.md` | deleted | N/A | 1-45 | Markdown content and docs |
| `.backlog/archive/tasks/task-003 - Wire-sections-param-through-handleCreateItem-to-template-service.md` | deleted | N/A | 1-26 | Markdown content and docs |
| `.backlog/tasks/task-010.02 - Core-Documentation.md` | deleted | N/A | 1-27 | Markdown content and docs |
| `.backlog/completed/task-001.07.02 - Implement-telemetry-stream-writer.md` | deleted | N/A | 1-37 | Markdown content and docs |
| `.backlogit/queue/.stash.md` | modified | 4; 9-24 | 4; 9 | Backlog artifact state |
| `.backlog/completed/task-001.09 - CLI-Commands.md` | deleted | N/A | 1-25 | Markdown content and docs |
| `.backlog/completed/task-001.10.02 - Implement-transformation-and-migration-pipeline.md` | deleted | N/A | 1-43 | Markdown content and docs |
| `.github/agents/groomer.agent.md` | added | 1-148 | N/A | Markdown content and docs |
| `.backlog/tasks/task-010.01.04 - Write-Backlog.md-migration-integration-tests.md` | deleted | N/A | 1-50 | Markdown content and docs |
| `.backlog/completed/task-001.06.02 - Write-rehydration-integration-tests.md` | deleted | N/A | 1-40 | Markdown content and docs |
| `internal/errors/errors.go` | modified | 14-19 | N/A | Supporting changes |
| `internal/stash/jsonl_test.go` | added | 1-184 | N/A | Supporting changes |
| `.backlog/tasks/task-002.06 - Integration-Testing.md` | deleted | N/A | 1-21 | Markdown content and docs |
| `.backlog/completed/task-001.03.02 - Implement-frontmatter-parser-and-serializer.md` | deleted | N/A | 1-38 | Markdown content and docs |
| `AGENTS.md` | modified | 1-4; 36-228 | N/A; N/A | Markdown content and docs |
| `.backlog/completed/task-001.06.01 - Implement-concurrent-rehydration-engine.md` | deleted | N/A | 1-39 | Markdown content and docs |
| `.backlog/completed/task-001.03.04 - Define-Sprint-container-model.md` | deleted | N/A | 1-37 | Markdown content and docs |
| `.backlogit/queue/F015.T008.ST003.md` | added | 1-12 | N/A | Backlog artifact state |
| `.backlog/completed/task-001.05.01 - Implement-SQLite-connection-management.md` | deleted | N/A | 1-36 | Markdown content and docs |
| `.backlog/tasks/task-010 - Backlogit-Documentation-Migration-Suite.md` | deleted | N/A | 1-35 | Markdown content and docs |
| `.backlog/completed/task-009.05.01 - Work-Queue-Engine.md` | deleted | N/A | 1-46 | Markdown content and docs |
| `.backlog/completed/task-001.08.03 - Implement-item-read-and-query-tools.md` | deleted | N/A | 1-36 | Markdown content and docs |
| `.github/skills/runtime-verification/SKILL.md` | added | 1-34 | N/A | Markdown content and docs |
| `.backlog/tasks/task-002.05 - MCP-Tool-Expansion-and-Dynamic-Generation.md` | deleted | N/A | 1-23 | Markdown content and docs |
| `.backlogit/queue/F015.T006.ST001.md` | added | 1-12 | N/A | Backlog artifact state |
| `.backlog/completed/task-008.03 - Fix-template-service-Update-set-updated_at-and-call-UpsertItem-after-section-writes.md` | deleted | N/A | 1-30 | Markdown content and docs |
| `.backlogit/archive/F013.T003.ST002.md` | renamed | 2; 7 | N/A; 6 | Markdown content and docs |
| `.backlogit/archive/F013.T001.md` | renamed | 2; 11 | N/A; 10 | Markdown content and docs |
| `.backlog/completed/task-001.05.02 - Implement-schema-bootstrapping-with-FTS5.md` | deleted | N/A | 1-38 | Markdown content and docs |
| `internal/core/workspace.go` | modified | 37-42; 84; 92-133 | 37; 79; N/A | Core backlog domain |
| `.backlogit/templates/deliberation.md` | added | 1-47 | N/A | Template validity and rendering |
| `docs/exec-plans/2026-04-05-two-agent-workflow-plan.md` | added | 1-425 | N/A | Markdown content and docs |

## Instruction Files Reviewed

| Instruction File | ApplyTo Pattern | Applicability |
|-----------------|-----------------|---------------|
| `.github/instructions/pull-request.instructions.md` | `**/.copilot-tracking/pr/**` | Applies to the tracking artifacts generated in this review workflow |
| `.github/instructions/markdown.instructions.md` | `**/*.md` | Applies to changed Markdown artifacts, templates, queue items, docs memory, and tracking files |
| `.github/instructions/writing-style.instructions.md` | `**/*.md` | Applies to review-tracking prose plus changed Markdown content |
| `.github/instructions/go.instructions.md` | `**/*.go` | Applies to 42 changed Go source/test files across core, MCP, DB, and tests |
| `.github/instructions/go-mcp-server.instructions.md` | `**/*.go` | Applies especially to MCP-surface Go changes such as `internal/mcp/errors.go` |
| `.github/instructions/workflows.instructions.md` | `**/.github/workflows/*.yml` | Available for workflow changes if any are in scope during follow-up review; none are primary in the latest local fix set |
| `.github/instructions/architecture-doc.instructions.md` | `**/docs/**` | Applies to `docs/memory/...` artifacts pending commit |
| `.github/instructions/constitution.instructions.md` | `**` | Global constraints for type safety, test-first development, workspace containment, and CQRS data architecture |
| `.github/instructions/backlogit.instructions.md` | `**` | Global backlogit workflow and source-of-truth constraints |
| `.github/instructions/backlog-integration.instructions.md` | `**` | Global backlog integration expectations for queue and traceability surfaces |

## Phase 2 Analysis Log

| Step | Status | Notes |
|------|--------|-------|
| Extract changed files from pr-reference.xml | ✅ Done | 278 files extracted with per-hunk line ranges |
| Match instruction files | ✅ Done | Applied global, Go, Markdown, docs, and PR tracking instructions based on changed paths |
| Build review plan | ✅ Done | Coverage checklist seeded for every changed file |
| Summarize findings | ✅ Done | Initial risk queue prepared for Phase 3 delegation |

### File Coverage Plan

- [ ] `.backlog/completed/task-008.04 - Fix-CLI-update-command-UpsertItem-and-bump-updated_at-after-section-writes.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.07 - Event-Telemetry-and-Memory-Streams.md` — Markdown content and docs
- [ ] `.github/agents/backlog-harvester.agent.md` — Markdown content and docs
- [ ] `.backlog/completed/task-009.02.03 - Required-Optional-Field-Enforcement.md` — Markdown content and docs
- [ ] `.github/agents/build-orchestrator.agent.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.08.06 - Write-MCP-contract-tests.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.10 - Legacy-Migration-Pipeline.md` — Markdown content and docs
- [ ] `.backlogit/queue/F015.T011.md` — Backlog artifact state
- [ ] `.github/skills/harness-architect/SKILL.md` — Markdown content and docs
- [ ] `.backlog/tasks/task-010.04.02 - Implement-file-classification-engine-for-markdown-document-types.md` — Markdown content and docs
- [ ] `.backlogit/archive/F013.F002.md` — Markdown content and docs
- [ ] `tests/contract/shipment_tools_test.go` — Contract coverage
- [ ] `.backlog/tasks/task-002.04 - CLI-Command-Suite.md` — Markdown content and docs
- [ ] `.context/prototype.go` — Supporting changes
- [ ] `internal/models/artifact.go` — Supporting changes
- [ ] `.github/instructions/backlog-integration.instructions.md` — Markdown content and docs
- [ ] `internal/db/stash.go` — SQLite and rehydration
- [ ] `.backlogit/archive/F013.R001-branch-review.md` — Markdown content and docs
- [ ] `.backlog/completed/task-009.06 - Epic-F-Workflow-Policy-CLI-Enhancements.md` — Markdown content and docs
- [ ] `.backlog/completed/task-002.01.03 - Update-frontmatter-parser-and-serializer-for-new-fields.md` — Markdown content and docs
- [ ] `.backlog/tasks/task-010.03 - Positioning-Migration-Documentation.md` — Markdown content and docs
- [ ] `.backlog/completed/task-009.04 - Epic-D-Archive-Lifecycle-Management.md` — Markdown content and docs
- [ ] `.backlogit/queue/F015.T002.ST004.md` — Backlog artifact state
- [ ] `.backlog/completed/task-001.08.05 - Implement-MCP-resource-handlers.md` — Markdown content and docs
- [ ] `.backlog/tasks/task-002.04.05 - Implement-CLI-move-command.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.06 - Rehydration-Engine.md` — Markdown content and docs
- [ ] `.copilot-tracking/plan-review/2026-04-05-two-agent-workflow-plan-review.md` — PR tracking artifact
- [ ] `.backlog/completed/task-009.05.02 - Queue-CLI-MCP-Tools.md` — Markdown content and docs
- [ ] `.backlogit/queue/F015.T005.md` — Backlog artifact state
- [ ] `.backlog/completed/task-001.10.01 - Implement-legacy-backlog.md-AST-parser.md` — Markdown content and docs
- [ ] `.github/skills/spike/SKILL.md` — Markdown content and docs
- [ ] `.backlogit/archive/F013.T002.md` — Markdown content and docs
- [ ] `.backlogit/queue/F015.T002.ST005.md` — Backlog artifact state
- [ ] `.backlogit/archive/F013.T001.ST001.md` — Markdown content and docs
- [ ] `.backlog/completed/task-008 - MCP-CLI-Section-Template-Bug-Fixes.md` — Markdown content and docs
- [ ] `.backlog/completed/task-002.02.02 - Generate-default-header-def.yaml-in-WriteDefaults.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.05.04 - Implement-read-only-SQL-query-gate.md` — Markdown content and docs
- [ ] `.backlog/completed/task-009.02 - Epic-B-WIT-Type-System-Self-Description.md` — Markdown content and docs
- [ ] `.github/agents/deliberator.agent.md` — Markdown content and docs
- [ ] `.backlogit/queue/F015.T004.ST002.md` — Backlog artifact state
- [ ] `.backlog/archive/tasks/task-006 - Fix-CLI-move-command-route-relocate-artifact-file-per-registry.yaml-on-status-change.md` — Markdown content and docs
- [ ] `.backlog/tasks/task-002.04.06 - Implement-CLI-delete-and-search-commands.md` — Markdown content and docs
- [ ] `.backlog/tasks/task-010.04.03 - Create-general-migration-scripts-and-configuration-templates.md` — Markdown content and docs
- [ ] `.backlogit/templates/shipment.md` — Template validity and rendering
- [ ] `.backlogit/archive/F013.T003.md` — Markdown content and docs
- [ ] `.backlog/completed/task-009.06.01 - Harness-Status-Attribute.md` — Markdown content and docs
- [ ] `.backlogit/queue/F015.T009.md` — Backlog artifact state
- [ ] `.backlog/completed/task-001.03.01 - Define-Artifact-model-with-status-constants.md` — Markdown content and docs
- [ ] `.github/agents/harness-architect.agent.md` — Markdown content and docs
- [ ] `.backlog/completed/task-002.01.02 - Update-DB-schema-and-queries-for-new-artifact-fields.md` — Markdown content and docs
- [ ] `.backlog/tasks/task-002 - Queue-Features-CLI-Commands-Header-Definitions-Templates-and-Dynamic-Tools.md` — Markdown content and docs
- [ ] `internal/mcp/errors.go` — MCP protocol surface
- [ ] `.backlogit/queue/F015.T003.ST002.md` — Backlog artifact state
- [ ] `internal/core/templates/service.go` — Core backlog domain
- [ ] `.backlog/completed/task-009.05 - Epic-E-Work-Queue.md` — Markdown content and docs
- [ ] `internal/core/shipment_test.go` — Core backlog domain
- [ ] `internal/cli/search.go` — Supporting changes
- [ ] `.backlog/completed/task-001.01 - Project-Foundation-and-Error-Hierarchy.md` — Markdown content and docs
- [ ] `internal/config/shipment_defaults_test.go` — Workspace config and defaults
- [ ] `docs/compound/config-issues/queue-view-empty-filter-values-2026-04-05.md` — Markdown content and docs
- [ ] `.backlog/tasks/task-002.04.03 - Implement-CLI-get-command.md` — Markdown content and docs
- [ ] `.backlog/queue.md` — Markdown content and docs
- [ ] `.backlogit/stash.jsonl` — Supporting changes
- [ ] `internal/cli/dep.go` — Supporting changes
- [ ] `.backlog/tasks/task-002.01 - Artifact-Model-Expansion.md` — Markdown content and docs
- [ ] `internal/core/artifacts.go` — Core backlog domain
- [ ] `.backlog/tasks/task-010.04 - General-Purpose-Migration-Tooling.md` — Markdown content and docs
- [ ] `internal/core/harness_status.go` — Core backlog domain
- [ ] `.backlog/tasks/task-010.03.03 - Write-Backlog.md-to-backlogit-migration-guide.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.11.01 - Implement-environment-detection-and-config-injection.md` — Markdown content and docs
- [ ] `.backlog/.claude/instructions.md` — Markdown content and docs
- [ ] `.backlog/archive/tasks/task-003 - Implement-section-extraction-in-handleGetItem.md` — Markdown content and docs
- [ ] `.backlog/completed/task-009.02.01 - Dynamic-Schema-Extension-Engine.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.08.04 - Implement-system-and-agent-operation-tools.md` — Markdown content and docs
- [ ] `docs/memory/2026-04-05/harness-merge-install-memory.md` — Session memory artifact
- [ ] `.backlogit/queue/F015.T008.ST002.md` — Backlog artifact state
- [ ] `.backlog/completed/task-001.08.01 - Implement-MCP-Server-struct-and-stdio-lifecycle.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.01.02 - Implement-error-hierarchy-with-sentinel-and-typed-errors.md` — Markdown content and docs
- [ ] `.backlog/completed/task-009.02.02 - Template-Self-Description-WIT-Metadata-API.md` — Markdown content and docs
- [ ] `.github/instructions/architecture-doc.instructions.md` — Markdown content and docs
- [ ] `.backlog/tasks/task-010.02.03 - Write-process-and-workflow-documentation.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.09.03 - Implement-query-command-with-formatted-output.md` — Markdown content and docs
- [ ] `internal/stash/jsonl.go` — Supporting changes
- [ ] `.backlog/tasks/task-002.04.07 - Implement-CLI-query-and-status-commands.md` — Markdown content and docs
- [ ] `internal/core/shipment.go` — Core backlog domain
- [ ] `.backlog/tasks/task-002.03 - Template-System.md` — Markdown content and docs
- [ ] `.backlog/tasks/task-002.05.01 - Add-new-fields-and-static-tools-to-MCP-server.md` — Markdown content and docs
- [ ] `.backlogit/queue/F015.T005.ST002.md` — Backlog artifact state
- [ ] `.backlog/tasks/task-010.04.04 - Write-general-migration-integration-tests.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001 - Backlogit-Core-Implementation.md` — Markdown content and docs
- [ ] `internal/cli/move.go` — Supporting changes
- [ ] `.backlogit/queue/F015.T005.ST001.md` — Backlog artifact state
- [ ] `internal/cli/delete.go` — Supporting changes
- [ ] `.backlog/completed/task-001.11 - MCP-Environment-Registration.md` — Markdown content and docs
- [ ] `.github/skills/operational-closure/SKILL.md` — Markdown content and docs
- [ ] `".backlog/completed/task-009.01.02 - Migration-Pipeline-Flat-\342\206\222-Hierarchical.md"` — Supporting changes
- [ ] `.backlogit/queue/F015.T004.ST001.md` — Backlog artifact state
- [ ] `.backlogit/queue/F015.T002.ST003.md` — Backlog artifact state
- [ ] `.backlog/completed/task-001.04.05 - Implement-artifact-creation-with-hierarchy-enforcement.md` — Markdown content and docs
- [ ] `.backlog/archive/tasks/task-004 - Fix-CLI-update-command-UpsertItem-and-bump-updated_at-after-section-writes.md` — Markdown content and docs
- [ ] `.backlogit/archive/F013.md` — Markdown content and docs
- [ ] `.backlog/tasks/task-002.06.01 - Full-workflow-integration-tests.md` — Markdown content and docs
- [ ] `docs/memory/2026-04-05/two-agent-workflow-continuation-memory.md` — Session memory artifact
- [ ] `.backlog/completed/task-001.04 - Core-Business-Logic.md` — Markdown content and docs
- [ ] `internal/core/workspace_internal_test.go` — Core backlog domain
- [ ] `.backlog/.github/copilot-instructions.md` — Markdown content and docs
- [ ] `.backlogit/queue/F015.T002.md` — Backlog artifact state
- [ ] `.backlog/completed/task-001.04.01 - Implement-SafeResolve-workspace-containment.md` — Markdown content and docs
- [ ] `.backlog/completed/task-002.03.03 - Implement-section-parser-and-writer.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.03 - Models-Frontmatter-and-Markdown-Parser.md` — Markdown content and docs
- [ ] `.backlog/completed/task-009.01.01 - Hierarchical-File-Organization-.backlogit-queue.md` — Markdown content and docs
- [ ] `.backlog/completed/task-009.01 - Epic-A-Hierarchical-File-Organization-Migration.md` — Markdown content and docs
- [ ] `internal/core/templates/service_test.go` — Core backlog domain
- [ ] `.backlog/tasks/task-010.01.02 - Add-migration-CLI-enhancements-dry-run-progress-validation.md` — Markdown content and docs
- [ ] `.backlog/tasks/task-002.04.08 - Register-all-CLI-commands-in-root.md` — Markdown content and docs
- [ ] `.backlogit/queue/F015.T001.md` — Backlog artifact state
- [ ] `.github/agents/shipper.agent.md` — Markdown content and docs
- [ ] `.backlog/completed/task-008.05 - Fix-CLI-move-command-route-relocate-artifact-file-per-registry.yaml-on-status-change.md` — Markdown content and docs
- [ ] `.backlog/archive/tasks/task-007 - Strengthen-contract-tests-exercise-MCP-handlers-with-real-assertions-not-key-existence-checks.md` — Markdown content and docs
- [ ] `.backlog/completed/task-002.03.01 - Implement-template-schema-and-loader.md` — Markdown content and docs
- [ ] `.backlogit/archive/F013.R002-followup-review.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.09.01 - Implement-Cobra-root-command-and-init.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.05.03 - Implement-parameterized-query-functions.md` — Markdown content and docs
- [ ] `.backlogit/queue/F015.T003.ST001.md` — Backlog artifact state
- [ ] `internal/cli/shipment_test.go` — Supporting changes
- [ ] `.backlogit/queue/F015.T006.ST002.md` — Backlog artifact state
- [ ] `.backlog/completed/task-001.01.03 - Create-Makefile-with-build-targets.md` — Markdown content and docs
- [ ] `.backlog/archive/tasks/task-008.01 - Fix-template-service-Update-set-updated_at-and-call-UpsertItem-after-section-writes.md` — Markdown content and docs
- [ ] `.github/instructions/backlogit.instructions.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.08 - MCP-Server-and-Tool-Handlers.md` — Markdown content and docs
- [ ] `.github/skills/harvest/SKILL.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.07.01 - Implement-JSONL-event-stream-writer.md` — Markdown content and docs
- [ ] `.backlog/memory/2026-03-29/feature-001-checkpoint.md` — Markdown content and docs
- [ ] `.backlogit/queue/F015.T001.ST005.md` — Backlog artifact state
- [ ] `.backlogit/queue/F015.T004.ST003.md` — Backlog artifact state
- [ ] `.backlog/completed/task-009 - Queue-Features-V2-WIT-Metadata-Archive-Lifecycle-Dependency-Queue-Hierarchical-File-Organization.md` — Markdown content and docs
- [ ] `internal/cli/archive.go` — Supporting changes
- [ ] `.copilot-tracking/harness/F015-harness.md` — PR tracking artifact
- [ ] `.backlog/completed/task-001.04.03 - Implement-custom-field-validation.md` — Markdown content and docs
- [ ] `internal/core/artifacts_expansion_test.go` — Core backlog domain
- [ ] `.backlogit/queue/F015.T001.ST004.md` — Backlog artifact state
- [ ] `.gitignore` — Supporting changes
- [ ] `.github/skills/safety-modes/SKILL.md` — Markdown content and docs
- [ ] `docs/compound/workflow-issues/stable-contract-before-two-agent-adoption-2026-04-05.md` — Markdown content and docs
- [ ] `docs/compound/go-patterns/f015-shipment-stash-patterns.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.08.02 - Implement-item-write-tools-create-and-update.md` — Markdown content and docs
- [ ] `internal/db/rehydration.go` — SQLite and rehydration
- [ ] `.backlogit/queue/F015.T001.ST003.md` — Backlog artifact state
- [ ] `.backlog/completed/task-002.01.04 - Update-rehydration-engine-for-new-fields.md` — Markdown content and docs
- [ ] `internal/cli/root.go` — Supporting changes
- [ ] `.backlog/tasks/task-010.04.01 - Design-pluggable-migration-adapter-interface.md` — Markdown content and docs
- [ ] `.backlogit/header-def.yaml` — YAML configuration
- [ ] `internal/cli/shipment.go` — Supporting changes
- [ ] `.github/skills/pr-lifecycle/SKILL.md` — Markdown content and docs
- [ ] `.backlog/completed/task-008.06 - Strengthen-contract-tests-exercise-MCP-handlers-with-real-assertions.md` — Markdown content and docs
- [ ] `.backlog/tasks/task-010.02.02 - Write-installation-guide.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.01.01 - Initialize-Go-module-and-project-structure.md` — Markdown content and docs
- [ ] `.github/agents/pr-review.agent.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.02.03 - Create-default-config-and-registry-templates.md` — Markdown content and docs
- [ ] `.autoharness/harness-manifest.yaml` — YAML configuration
- [ ] `.backlog/tasks/task-002.04.04 - Implement-CLI-update-command.md` — Markdown content and docs
- [ ] `.backlogit/queue/F015.T002.ST001.md` — Backlog artifact state
- [ ] `.backlog/completed/task-002.01.05 - Update-core-CRUD-with-new-fields-and-ID-immutability.md` — Markdown content and docs
- [ ] `internal/core/queue.go` — Core backlog domain
- [ ] `.backlogit/queue/F015.T001.ST001.md` — Backlog artifact state
- [ ] `.backlog/tasks/task-010.01 - Backlog.md-Migration-Tooling.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.07.03 - Implement-event-tail-reader.md` — Markdown content and docs
- [ ] `.backlog/tasks/task-010.03.01 - Write-tool-rationale-and-agent-harness-value-proposition.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.03.03 - Implement-Markdown-file-parser.md` — Markdown content and docs
- [ ] `internal/cli/get.go` — Supporting changes
- [ ] `.backlogit/queue/F015.T008.ST001.md` — Backlog artifact state
- [ ] `.backlog/tasks/task-002.04.01 - Implement-CLI-add-command.md` — Markdown content and docs
- [ ] `.backlogit/queue/DL001.md` — Backlog artifact state
- [ ] `.backlogit/queue/F015.md` — Backlog artifact state
- [ ] `tests/contract/queue_tools_test.go` — Contract coverage
- [ ] `internal/cli/migrate_import_test.go` — Supporting changes
- [ ] `.backlogit/archive/F013.T003.ST001.md` — Markdown content and docs
- [ ] `.backlog/completed/task-008.01 - Implement-section-extraction-in-handleGetItem.md` — Markdown content and docs
- [ ] `.backlogit/queue/F015.T008.md` — Backlog artifact state
- [ ] `internal/config/defaults.go` — Workspace config and defaults
- [ ] `.autoharness/backlog-registry.yaml` — YAML configuration
- [ ] `.backlog/tasks/task-002.02 - Header-Definition-System.md` — Markdown content and docs
- [ ] `.backlogit/archive/F013.T002.ST001.md` — Markdown content and docs
- [ ] `.github/copilot-instructions.md` — Markdown content and docs
- [ ] `.backlog/tasks/task-002.05.02 - Implement-dynamic-MCP-tool-generation-from-templates.md` — Markdown content and docs
- [ ] `internal/cli/query.go` — Supporting changes
- [ ] `.backlog/completed/task-001.02 - Configuration-System.md` — Markdown content and docs
- [ ] `.backlog/archive/tasks/task-005 - Fix-CLI-update-command-UpsertItem-and-bump-updated_at-after-section-writes.md` — Markdown content and docs
- [ ] `.backlogit/queue/F015.T003.ST003.md` — Backlog artifact state
- [ ] `.backlog/completed/task-002.02.01 - Implement-header-def.yaml-schema-and-loader.md` — Markdown content and docs
- [ ] `.backlog/completed/task-002.03.02 - Create-default-templates-for-8-artifact-types.md` — Markdown content and docs
- [ ] `.backlog/tasks/task-010.02.01 - Write-comprehensive-README.md.md` — Markdown content and docs
- [ ] `.backlogit/queue/F015.T010.md` — Backlog artifact state
- [ ] `.backlogit/queue/F015.T007.ST001.md` — Backlog artifact state
- [ ] `internal/stash/stash.go` — Supporting changes
- [ ] `internal/errors/shipment_errors_test.go` — Supporting changes
- [ ] `internal/cli/queue_cmd.go` — Supporting changes
- [ ] `.backlogit/archive/F013.T003.ST003.md` — Markdown content and docs
- [ ] `.backlogit/config.yaml` — YAML configuration
- [ ] `.backlog/tasks/task-010.01.01 - Enhance-legacy-parser-for-broader-Backlog.md-format-coverage.md` — Markdown content and docs
- [ ] `.backlogit/archive/F013.F001.md` — Markdown content and docs
- [ ] `.backlogit/queue/F015.T007.md` — Backlog artifact state
- [ ] `.backlog/completed/task-001.05 - Database-Foundation.md` — Markdown content and docs
- [ ] `.backlog/config.yml` — YAML configuration
- [ ] `internal/cli/update.go` — Supporting changes
- [ ] `.backlog/completed/task-009.04.02 - Commit-Tracking.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.07.04 - Implement-memory-and-checkpoint-persistence.md` — Markdown content and docs
- [ ] `tests/integration/shipment_workflow_test.go` — Integration coverage
- [ ] `.backlog/completed/task-008.02 - Wire-sections-param-through-handleCreateItem-to-template-service.md` — Markdown content and docs
- [ ] `.backlogit/queue/F015.T002.ST002.md` — Backlog artifact state
- [ ] `docs/memory/2026-04-05/T008-checkpoint.md` — Session memory artifact
- [ ] `.autoharness/workspace-profile.yaml` — YAML configuration
- [ ] `.backlog/archive/tasks/task-002.01 - MCP-Tool-Expansion-and-Dynamic-Generation.md` — Markdown content and docs
- [ ] `.backlog/completed/task-009.06.02 - CLI-Enhancements-Tabular-Listing.md` — Markdown content and docs
- [ ] `.backlogit/queue/F015.T006.md` — Backlog artifact state
- [ ] `.backlog/completed/task-009.04.01 - Archive-Command-Lifecycle-Management.md` — Markdown content and docs
- [ ] `.backlog/tasks/task-002.04.02 - Implement-CLI-list-command.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.02.02 - Implement-YAML-config-loader-with-env-var-overrides.md` — Markdown content and docs
- [ ] `.backlog/memory/2024-01-16/task-010-epic-complete-memory.md` — Markdown content and docs
- [ ] `docs/workflow.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.04.02 - Implement-naming-template-resolution.md` — Markdown content and docs
- [ ] `.backlog/completed/task-009.03.02 - Dependency-CLI-MCP-Wiring.md` — Markdown content and docs
- [ ] `internal/core/wit_metadata.go` — Core backlog domain
- [ ] `.backlog/completed/task-001.04.06 - Implement-Workspace-orchestration-struct.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.09.04 - Implement-mcp-and-migrate-commands.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.04.04 - Implement-file-routing-from-registry-config.md` — Markdown content and docs
- [ ] `docs/rationale.md` — Markdown content and docs
- [ ] `.backlog/tasks/task-010.03.02 - Write-backlogit-vs-Backlog.md-comparison-document.md` — Markdown content and docs
- [ ] `.backlog/completed/task-009.03 - Epic-C-Dependency-Graph.md` — Markdown content and docs
- [ ] `.backlogit/queue/F015.T001.ST002.md` — Backlog artifact state
- [ ] `.backlog/.cursor/mcp.json` — Supporting changes
- [ ] `.backlogit/queue/F015.T003.md` — Backlog artifact state
- [ ] `.github/instructions/constitution.instructions.md` — Markdown content and docs
- [ ] `.backlogit/queue/F015.T007.ST002.md` — Backlog artifact state
- [ ] `.backlog/completed/task-001.02.01 - Define-configuration-schema-structs.md` — Markdown content and docs
- [ ] `.backlog/completed/task-002.01.01 - Expand-Artifact-struct-with-queue-specified-fields.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.09.02 - Implement-create-and-sync-commands.md` — Markdown content and docs
- [ ] `docs/memory/2026-04-05/two-agent-workflow-design-session.md` — Session memory artifact
- [ ] `internal/mcp/tools.go` — MCP protocol surface
- [ ] `internal/config/defaults_templates_test.go` — Workspace config and defaults
- [ ] `.backlogit/queue/F015.T004.md` — Backlog artifact state
- [ ] `internal/core/queue_test.go` — Core backlog domain
- [ ] `.github/policies/workflow-policies.md` — Markdown content and docs
- [ ] `.backlog/archive/tasks/task-004 - Fix-template-service-Update-set-updated_at-and-call-UpsertItem-after-section-writes.md` — Markdown content and docs
- [ ] `.autoharness/config.yaml` — YAML configuration
- [ ] `.backlog/tasks/task-010.01.03 - Create-migration-configuration-for-document-class-mapping.md` — Markdown content and docs
- [ ] `.backlog/completed/task-009.03.01 - Dependency-Table-Junction-Model.md` — Markdown content and docs
- [ ] `.backlog/archive/tasks/task-003 - Wire-sections-param-through-handleCreateItem-to-template-service.md` — Markdown content and docs
- [ ] `.backlog/tasks/task-010.02 - Core-Documentation.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.07.02 - Implement-telemetry-stream-writer.md` — Markdown content and docs
- [ ] `.backlogit/queue/.stash.md` — Backlog artifact state
- [ ] `.backlog/completed/task-001.09 - CLI-Commands.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.10.02 - Implement-transformation-and-migration-pipeline.md` — Markdown content and docs
- [ ] `.github/agents/groomer.agent.md` — Markdown content and docs
- [ ] `.backlog/tasks/task-010.01.04 - Write-Backlog.md-migration-integration-tests.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.06.02 - Write-rehydration-integration-tests.md` — Markdown content and docs
- [ ] `internal/errors/errors.go` — Supporting changes
- [ ] `internal/stash/jsonl_test.go` — Supporting changes
- [ ] `.backlog/tasks/task-002.06 - Integration-Testing.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.03.02 - Implement-frontmatter-parser-and-serializer.md` — Markdown content and docs
- [ ] `AGENTS.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.06.01 - Implement-concurrent-rehydration-engine.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.03.04 - Define-Sprint-container-model.md` — Markdown content and docs
- [ ] `.backlogit/queue/F015.T008.ST003.md` — Backlog artifact state
- [ ] `.backlog/completed/task-001.05.01 - Implement-SQLite-connection-management.md` — Markdown content and docs
- [ ] `.backlog/tasks/task-010 - Backlogit-Documentation-Migration-Suite.md` — Markdown content and docs
- [ ] `.backlog/completed/task-009.05.01 - Work-Queue-Engine.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.08.03 - Implement-item-read-and-query-tools.md` — Markdown content and docs
- [ ] `.github/skills/runtime-verification/SKILL.md` — Markdown content and docs
- [ ] `.backlog/tasks/task-002.05 - MCP-Tool-Expansion-and-Dynamic-Generation.md` — Markdown content and docs
- [ ] `.backlogit/queue/F015.T006.ST001.md` — Backlog artifact state
- [ ] `.backlog/completed/task-008.03 - Fix-template-service-Update-set-updated_at-and-call-UpsertItem-after-section-writes.md` — Markdown content and docs
- [ ] `.backlogit/archive/F013.T003.ST002.md` — Markdown content and docs
- [ ] `.backlogit/archive/F013.T001.md` — Markdown content and docs
- [ ] `.backlog/completed/task-001.05.02 - Implement-schema-bootstrapping-with-FTS5.md` — Markdown content and docs
- [ ] `internal/core/workspace.go` — Core backlog domain
- [ ] `.backlogit/templates/deliberation.md` — Template validity and rendering
- [ ] `docs/exec-plans/2026-04-05-two-agent-workflow-plan.md` — Markdown content and docs

### High-Risk Areas Identified

1. 🔍 **Shipment lifecycle and rollback safety** — `internal/core/shipment.go`, `internal/mcp/errors.go`, `internal/core/shipment_test.go`, and `tests/contract/shipment_tools_test.go` changed together and need cross-layer contract review.
2. ⚠️ **Error classification and masking** — `internal/core/workspace.go`, `internal/core/artifacts.go`, and `internal/core/shipment.go` now preserve underlying failures; review should verify no caller assumptions broke.
3. ✅ **Template correctness and rendering** — `.backlogit/templates/shipment.md`, `.backlogit/queue/DL001.md`, and `internal/core/templates/service.go` changed together and need markdown/template contract validation.
4. 🔒 **Persisted backlog state in repo** — `.backlogit` artifacts and template files remain part of the diff and should be checked against constitution/backlog workflow rules.
5. 💡 **Pending untracked artifacts** — `.backlogit/checkpoints/` and `docs/memory/[20260406-195954]-015-shipment-validation-review-fixes-memory.md` are present locally but not represented in the git diff yet; Phase 4 should decide whether they belong in the PR.

## Review Items

### ✅ Resolved on Branch

* Fixed the P1 shipment schema mismatch by updating `.backlogit/header-def.yaml` so `shipment.items` is declared as a list and by tightening the defaults assertion in `internal/config/shipment_defaults_test.go`.
* Fixed MCP pre-init startup by allowing `backlogit mcp` to boot without an initialized workspace, preserving unconditional tool visibility and returning `workspace_not_initialized` at call time instead (`internal/cli/root.go`, `internal/cli/root_mcp_test.go`, `internal/mcp/server.go`, `internal/mcp/tools.go`, `tests/contract/tools_real_test.go`).
* Fixed stash rehydration precedence and provenance so `stash.jsonl` overrides duplicate legacy entries and preserves per-entry `source_path` in the index (`internal/db/rehydration.go`, `internal/db/stash.go`, `internal/db/rehydration_expansion_test.go`).
* Fixed shipment lifecycle and persistence findings by blocking mutation of shipped, abandoned, and archived shipments, allowing reassignment after archival, restoring Markdown files on DB upsert failure, and journaling blocked-item returns for recovery on reopen (`internal/core/shipment.go`, `internal/core/workspace.go`, `internal/core/shipment_test.go`, `tests/contract/shipment_tools_test.go`).
* Fixed the P3 memory reference so the stash JSONL path now points to `.backlogit/stash.jsonl` (`docs/memory/2026-04-05/two-agent-workflow-continuation-memory.md`).
* Published the durable branch-review closure record to `docs/closure/2026-04-06/015-two-agent-workflow-refactor-review-closure.md`.
* Remaining manual decision: keep local-only checkpoint and memory artifacts out of the PR unless we explicitly decide they belong in the branch diff.

### ✅ Approved for PR Comment

* All previously merged P1, P2, and P3 findings listed in this tracker were resolved on the branch.
* Validation gates passed after remediation: `go test ./...`, `go vet ./...`, `golangci-lint run`, and `gofmt -l .`.

### ❌ Rejected / No Action

* Rejected the Learnings Researcher claim that `.backlogit/queue/DL001.md` contains control characters after direct byte-level verification showed no unexpected control bytes.

## Next Steps

* [x] Delegate the current diff scope to the `review` skill in interactive mode for persona analysis.
* [x] Merge the remaining persona findings into this tracker and complete Phase 3 triage.
* [x] Fix the current P1/P2/P3 findings on this branch.
* [ ] Decide whether local-only checkpoint and memory artifacts should stay out of the PR or be promoted into the tracked diff.
