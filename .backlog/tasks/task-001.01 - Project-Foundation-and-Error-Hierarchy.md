---
id: TASK-001.01
title: Project Foundation and Error Hierarchy
status: Done
assignee: []
created_date: '2026-03-30 01:36'
labels:
  - epic
dependencies: []
parent_task_id: TASK-001
priority: high
ordinal: 1000
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Scaffold the Go project structure, initialize `go.mod`, create the Makefile, and establish the error system that all subsequent packages depend on.

Creates `cmd/backlogit/main.go` as the minimal CLI entrypoint, `internal/errors/errors.go` with sentinel errors (`ErrConfig`, `ErrValidation`, `ErrQuery`, `ErrRehydration`, `ErrMigration`, `ErrMCP`) and typed error structs that support `errors.Is` and `errors.As`, and a Makefile with build/test/lint/vet/format targets.

Module: `github.com/backlogit/backlogit`, Go 1.22+

Dependencies: `github.com/spf13/cobra`, `github.com/mark3labs/mcp-go`, `gopkg.in/yaml.v3`, `modernc.org/sqlite`, `github.com/go-playground/validator/v10`, `golang.org/x/sync`, `github.com/stretchr/testify`
<!-- SECTION:DESCRIPTION:END -->
