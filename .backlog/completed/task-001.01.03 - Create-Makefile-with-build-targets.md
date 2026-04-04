---
id: TASK-001.01.03
title: Create Makefile with build targets
status: Done
assignee: []
created_date: '2026-03-30 01:38'
labels: []
dependencies: []
parent_task_id: TASK-001.01
priority: high
ordinal: 1300
---

## Description

<!-- SECTION:DESCRIPTION:BEGIN -->
Create `Makefile` with targets:

- `build`: `go build -o bin/backlogit ./cmd/backlogit`
- `test`: `go test -race -coverprofile=coverage.out ./...`
- `lint`: `golangci-lint run`
- `vet`: `go vet ./...`
- `fmt`: `gofmt -l .`
- `cover`: `go tool cover -func=coverage.out`
- `clean`: Remove build artifacts
- `install`: `go install ./cmd/backlogit`
- `all`: `fmt vet lint test build` (default target)
<!-- SECTION:DESCRIPTION:END -->

## Acceptance Criteria
<!-- AC:BEGIN -->
- [ ] #1 make build compiles the binary
- [ ] #2 make test runs go test ./...
- [ ] #3 make lint runs golangci-lint run
- [ ] #4 make vet runs go vet ./...
- [ ] #5 make fmt checks gofmt formatting
<!-- AC:END -->


## Implementation Notes

Completed in commit de8d31c. Gates passed: `go test ./...`, `go vet ./...`, `golangci-lint run`.