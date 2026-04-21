---
title: "038-F: CLI Type Safety — newRenderer and validateFormat Signature Hardening"
description: Implementation plan for accepting format.Format instead of raw string in CLI rendering helpers
ms.date: 2026-04-21
---

## Problem Statement

`newRenderer` and `validateFormat` in `internal/cli/list.go` accept `string`
parameters, internally cast to `format.Format`, and silently default to table
output for unknown values. This weakens the typed format API and allows
unvalidated strings to flow through the rendering pipeline without compiler
enforcement. Finding F-003 (P3) from the 035-S review gate.

## Proposed Approach

Change both function signatures to accept `format.Format` directly. Update all
call sites across four CLI command files to perform the `format.Format()` type
conversion at the Cobra flag boundary (where `string` enters the system). This
pushes the `string → format.Format` conversion to the earliest possible point,
making the type contract explicit at the API surface.

## Scope

### In scope

- Change `newRenderer(f string, w io.Writer)` to `newRenderer(f format.Format, w io.Writer)`
- Change `validateFormat(f string, allowed ...format.Format)` to `validateFormat(f format.Format, allowed ...format.Format)`
- Update call sites in `list.go`, `queue_cmd.go`, `shipment.go`, `stash.go`
- Add test coverage for invalid format rejection via CLI
- Remove redundant `string() ↔ format.Format()` round-trip casts

### Out of scope

- Changing `newRenderer` to return `(format.Renderer, error)` — callers already validate via `validateFormat` before calling `newRenderer`
- Adding new format types
- Refactoring the format package itself

## Affected Files

| File | Change |
|---|---|
| `internal/cli/list.go` | Signature change for `newRenderer` and `validateFormat`; update call sites on lines 128, 154 |
| `internal/cli/queue_cmd.go` | Convert `formatOutput` to `format.Format` before calling `validateFormat` and `newRenderer` (lines 64, 67, 69) |
| `internal/cli/shipment.go` | Same pattern as queue_cmd.go (lines 127, 130, 132) |
| `internal/cli/stash.go` | Same pattern (lines 94, 119) |
| `internal/cli/list_test.go` | Add `TestListCommand_RejectsInvalidFormat` |

## Implementation Steps

### Step 1: Add failing test

Add `TestListCommand_RejectsInvalidFormat` to `list_test.go` that passes
`--format banana` and asserts an error containing `"banana"` is returned.

### Step 2: Change function signatures

In `list.go`:
- `newRenderer(f format.Format, w io.Writer) format.Renderer` — update internal references to remove cast
- `validateFormat(f format.Format, allowed ...format.Format) error` — update comparison to use direct equality

### Step 3: Update call sites

For each calling file, introduce an `effFmt := format.Format(formatOutput)`
local variable at the validation boundary, then thread it through
`validateFormat`, `switch`, and `newRenderer` calls, eliminating redundant
casts.

For `list.go` specifically, `effectiveFormat` is already `format.Format` — just
remove the `string()` cast on line 154.

**Special case — stash.go**: Lines 90 and 98 have additional string-typed
format comparisons (`formatOutput != string(format.FormatJSON)` and
`format.Format(formatOutput) == format.FormatJSON`). Introduce `effFmt` before
the `groupByPriority` check and use it consistently through all format
branching.

### Step 4: Additional tests

Add invalid-format rejection tests for `queue view` and `stash list` in
addition to `list`. This ensures the 3 most distinct call paths all properly
reject unknown formats through the typed API.

### Step 5: Quality gates

Run: `go test ./...`, `go vet ./...`, `golangci-lint run`, `gofmt -l .`

## Constitution Check

| Principle | Status | Notes |
|---|---|---|
| I. Type-Safe Go | Satisfies | This change enforces type safety at function boundaries |
| II. MCP Protocol Fidelity | N/A | No MCP tools affected |
| III. Test-First Development | Satisfied | Failing test written before signature change |
| IV. Workspace Containment | N/A | No filesystem path changes |
| V. Structured Observability | N/A | No logging changes |
| VI. Single-Binary Simplicity | N/A | No new dependencies |
| VII. CQRS Data Architecture | N/A | No data layer changes |
| VIII. Git-Friendly Persistence | N/A | No file format changes |
| IX. Agent Context Efficiency | N/A | No MCP tool response changes |

## Risk Assessment

**ActionRisk: low** — Pure refactor of unexported function signatures within a
single package. No behavioral change. All existing callers already validate
formats before reaching `newRenderer`. Covered by existing CLI integration tests
plus the new invalid-format test.
