---
title: "MCP/CLI config parity: both entry points must load config-driven options"
tags: [mcp, cli, config, parity, telemetry, attribution]
date: 2026-05-07
severity: high
---

# MCP/CLI Config Parity Rule

## Problem

When a config-driven option is added to a feature (e.g., `AttributionPrefixes`
from `ws.Config.Telemetry`), it is easy to wire it into the CLI entry point
but forget to load it in the MCP handler for the equivalent tool.

This was a P2 review finding (GQ-002) in 049-S: the CLI `newTelemetryHarvestCmd`
was loading `AttributionPrefixes`, but `handleTelemetryHarvest` in `tools.go`
was not, meaning MCP callers would always get unmodified default prefix behavior.

## Rule

Whenever a config-driven option is added to shared business logic, always check
**both** entry surfaces:

1. The CLI command handler (`internal/cli/`)
2. The MCP tool handler (`internal/mcp/tools.go`)

If the config option belongs to an agent-accessible feature, the MCP handler
is the primary consumer and should always load it.

## Pattern

```go
// In handleTelemetryHarvest (MCP side)
if ws.Config != nil && ws.Config.Telemetry != nil {
    opts.AttributionPrefixes = ws.Config.Telemetry.AttributionPrefixes
}
```

```go
// In newTelemetryHarvestCmd (CLI side)
if ws.Config != nil && ws.Config.Telemetry != nil {
    opts.AttributionPrefixes = ws.Config.Telemetry.AttributionPrefixes
}
```

Both should appear in code review checklists for config-adding PRs.

## Verification checklist

When adding a new config-driven option:

- [ ] `internal/config/schema.go` — field added to the config struct
- [ ] `internal/cli/<command>.go` — CLI handler loads the config field into opts
- [ ] `internal/mcp/tools.go` — MCP handler loads the config field into opts
- [ ] Tests for the MCP path include config loading coverage

## References

- PR #89: feat(telemetry): attribution analytics & trend reporting (049-S)
- Review finding GQ-002 (P2): MCP harvest handler missing AttributionPrefixes
