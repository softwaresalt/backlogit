---
title: "TASK-002 Harness Manifest"
date: 2026-03-30
branch: 002-queue-features-cli-header-templates-tools
epic: TASK-002
---

## Harness Manifest

Maps each TASK-002 subtask to its test harness file(s), stub file(s), and the `go test` command to run.

### Sub-Epic A: Model Expansion

| Task ID | Title | Test File | Stub File | Harness Command |
|---------|-------|-----------|-----------|-----------------|
| TASK-002.01.01 | Expand Artifact Model | `internal/models/artifact_expansion_test.go` | `internal/models/artifact.go` | `go test -v -run TestNewArtifact_NewFields ./internal/models/...` |
| TASK-002.01.02 | Update DB Schema & Queries | `internal/db/queries_expansion_test.go` | `internal/db/queries.go` | `go test -v -run "TestUpsertItem_NewFields\|TestQueryItems_Filter\|TestSearchItems_MatchesLabels" ./internal/db/...` |
| TASK-002.01.03 | Update Frontmatter Parser | `internal/models/frontmatter_expansion_test.go` | `internal/models/frontmatter.go` | `go test -v -run "TestArtifactFromFrontmatter_NewFields\|TestSerializeFrontmatter_NewFieldsRoundTrip" ./internal/models/...` |
| TASK-002.01.04 | Update Rehydration | `internal/db/rehydration_expansion_test.go` | `internal/db/rehydration.go` | `go test -v -run TestRehydrate_NewFieldsFlowThrough ./internal/db/...` |
| TASK-002.01.05 | Update Core CRUD | `internal/core/artifacts_expansion_test.go` | `internal/core/artifacts.go` | `go test -v -run "TestCreateArtifact_WithNewOptions\|TestUpdateArtifact_NewFields\|TestUpdateArtifact_IDImmutability" ./internal/core/...` |

### Sub-Epic B: Header Definition

| Task ID | Title | Test File | Stub File | Harness Command |
|---------|-------|-----------|-----------|-----------------|
| TASK-002.02.01 | Header Definition Schema | `internal/config/headerdef_test.go` | `internal/config/headerdef.go` | `go test -v -run "TestLoadHeaderDef\|TestResolveFieldSchema\|TestIsImmutable" ./internal/config/...` |
| TASK-002.02.02 | Header Definition Defaults | `internal/config/defaults_headerdef_test.go` | `internal/config/defaults.go` | `go test -v -run "TestWriteDefaults_CreatesHeaderDef\|TestWriteDefaults_HeaderDefLoadable" ./internal/config/...` |

### Sub-Epic C: Template System

| Task ID | Title | Test File | Stub File | Harness Command |
|---------|-------|-----------|-----------|-----------------|
| TASK-002.03.01 | Template Schema & Loader | `internal/config/templates_test.go` | `internal/config/templates.go` | `go test -v -run "TestLoadTemplates\|TestGetTemplateForType" ./internal/config/...` |
| TASK-002.03.02 | Default Templates | `internal/config/defaults_templates_test.go` | `internal/config/defaults.go` | `go test -v -run "TestWriteDefaults_CreatesTemplates\|TestWriteDefaults_TemplatesLoadable" ./internal/config/...` |
| TASK-002.03.03 | Section Parser | `internal/parser/sections_test.go` | `internal/parser/sections.go` | `go test -v -run "TestParseSections\|TestWriteSections\|TestWriteSection" ./internal/parser/...` |

### Sub-Epic D: CLI Commands

| Task ID | Title | Test File | Stub File | Harness Command |
|---------|-------|-----------|-----------|-----------------|
| TASK-002.04.01 | CLI add Command | `internal/cli/add_test.go` | `internal/cli/add.go` | `go test -v -run TestAddCommand ./internal/cli/...` |
| TASK-002.04.02 | CLI list Command | `internal/cli/list_test.go` | `internal/cli/list.go` | `go test -v -run TestListCommand ./internal/cli/...` |
| TASK-002.04.03 | CLI get Command | `internal/cli/get_test.go` | `internal/cli/get.go` | `go test -v -run TestGetCommand ./internal/cli/...` |
| TASK-002.04.04 | CLI update Command | `internal/cli/update_test.go` | `internal/cli/update.go` | `go test -v -run TestUpdateCommand ./internal/cli/...` |
| TASK-002.04.05 | CLI move Command | `internal/cli/move_test.go` | `internal/cli/move.go` | `go test -v -run TestMoveCommand ./internal/cli/...` |
| TASK-002.04.06 | CLI delete & search | `internal/cli/delete_search_test.go` | `internal/cli/delete.go`, `internal/cli/search.go` | `go test -v -run "TestDeleteCommand\|TestSearchCommand" ./internal/cli/...` |
| TASK-002.04.07 | CLI query & status | `internal/cli/query_status_test.go` | `internal/cli/query.go`, `internal/cli/status_cmd.go` | `go test -v -run "TestQueryCommand\|TestStatusCommand" ./internal/cli/...` |
| TASK-002.04.08 | CLI Command Registration | `internal/cli/root_expansion_test.go` | `internal/cli/root.go` | `go test -v -run "TestRootCommand_RegistersAllCommands\|TestRootCommand_NoFlagCollisions" ./internal/cli/...` |

### Sub-Epic E: MCP Dynamic Tools

| Task ID | Title | Test File | Stub File | Harness Command |
|---------|-------|-----------|-----------|-----------------|
| TASK-002.05.01 | MCP Tools Expansion | `tests/contract/tools_expansion_test.go` | `internal/mcp/tools.go` | `go test -v -run "TestToolSchema\|TestToolExists" ./tests/contract/...` |
| TASK-002.05.02 | Dynamic Tool Generation | `internal/mcp/dynamic_test.go` | `internal/mcp/dynamic.go` | `go test -v -run "TestRegisterDynamicTools" ./internal/mcp/...` |

### Sub-Epic F: Integration

| Task ID | Title | Test File | Stub File | Harness Command |
|---------|-------|-----------|-----------|-----------------|
| TASK-002.06.01 | End-to-End Workflow | `tests/integration/workflow_test.go` | (all stubs) | `go test -v ./tests/integration/...` |

## Harness Summary

| Metric | Count |
|--------|-------|
| Total subtasks | 21 |
| Test files created | 22 |
| Stub files created/modified | 18 |
| Total test functions | ~60 |
| Packages touched | 9 |

## Running All Harnesses

```bash
# Compile check (should pass)
go build ./...

# Run all harness tests (should all FAIL with "not implemented" panics)
go test -v ./internal/models/... ./internal/db/... ./internal/core/... ./internal/config/... ./internal/cli/... ./internal/mcp/... ./internal/parser/... ./tests/contract/... ./tests/integration/...

# Run a specific subtask harness
go test -v -run <TestPattern> ./<package>/...
```

## Red Phase Status

All harness tests fail with `panic("not implemented: Worker: ...")` messages containing implementation instructions for each subtask. The build-feature skill uses these panics as the acceptance criteria: a subtask is complete when its harness tests pass without panics.
