---
id: TASK-001.01.02
title: Implement error hierarchy with sentinel and typed errors
status: Done
assignee: []
created_date: '2026-03-30 01:38'
labels: []
dependencies: []
parent_task_id: TASK-001.01
priority: high
ordinal: 1200
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `internal/errors/errors.go` with:

1. Sentinel errors: `ErrConfig`, `ErrValidation`, `ErrQuery`, `ErrRehydration`, `ErrMigration`, `ErrMCP`
2. Typed error structs: `ConfigError` (Field, Message, Err), `ValidationError` (Field, Value, Constraint, Err), `QueryError` (SQL, Err)
3. Each typed error implements `Error()`, `Unwrap()`, and `Is()` methods
4. Constructor functions: `NewConfigError(field, message, err)`, etc.

Create `internal/errors/errors_test.go` with table-driven tests for:
- Error message formatting
- `errors.Is` matching through wrapped chains
- `errors.As` extraction of typed error fields
- Wrapping with `fmt.Errorf("context: %w", err)`
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 ErrConfig, ErrValidation, ErrQuery, ErrRehydration, ErrMigration, ErrMCP sentinel errors defined
- [ ] #2 ConfigError, ValidationError, QueryError typed structs implement error interface
- [ ] #3 errors.Is(wrappedErr, ErrConfig) returns true for ConfigError
- [ ] #4 errors.As(wrappedErr, &configErr) populates ConfigError fields
- [ ] #5 go test ./internal/errors/... passes with 100% coverage
<!-- AC:END -->


## Implementation Notes

Completed in commit de8d31c. Gates passed: `go test ./...`, `go vet ./...`, `golangci-lint run`.