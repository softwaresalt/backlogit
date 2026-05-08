---
title: "BuildAttributor pattern: compile prefix registry once per harvest run"
tags: [telemetry, attribution, performance, pattern, prefix-registry]
date: 2026-05-07
severity: medium
---

# BuildAttributor Pattern

## Problem

`AttributeToolWithConfig(toolName, customPrefixes)` merged and sorted the
combined prefix registry on every single call. In a telemetry harvest over
thousands of tool call records, this repeated allocation is unnecessary and
expensive. Original code called it per event:

```go
for _, event := range events {
    server := AttributeToolWithConfig(event.ToolName, customPrefixes)
    ...
}
```

A Copilot review finding (PRRT_kwDORzozKM6AeVil) correctly flagged this.

## Solution

Add a `BuildAttributor(customPrefixes map[string]string)` factory function that
compiles the merged prefix registry once and returns a closure:

```go
// BuildAttributor compiles the merged prefix registry once and returns
// a closure that attributes a single tool name with zero allocations.
func BuildAttributor(customPrefixes map[string]string) func(string) string {
    merged := buildMergedPrefixes(customPrefixes) // sort+merge happens here
    return func(toolName string) string {
        return attributeWithPrefixes(toolName, merged)
    }
}
```

Call it once at the top of the harvest/correlate loop:

```go
attr := BuildAttributor(opts.AttributionPrefixes)
for _, event := range events {
    server := attr(event.ToolName)
    ...
}
```

## Benefits

- One allocation per harvest run instead of one per tool call
- The returned closure captures the compiled slice; subsequent calls are pure lookups
- Nil/empty `customPrefixes` fast-path falls through to the package-level slice

## Where applied

- `internal/telemetry/harvest.go` — `attr := BuildAttributor(...)` before toolStats loop
- `internal/telemetry/correlator.go` — `attr := BuildAttributor(customPrefixes)` at top of `Correlate()`
- `internal/mcp/tools.go` — `handleTelemetryHarvest` passes `ws.Config.Telemetry.AttributionPrefixes`

## References

- PR #89: feat(telemetry): attribution analytics & trend reporting (049-S)
- Copilot review thread PRRT_kwDORzozKM6AeVil
