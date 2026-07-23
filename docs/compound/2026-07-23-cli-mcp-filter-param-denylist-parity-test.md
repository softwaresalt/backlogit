---
title: "Denylist-based CLI/MCP filter parameter parity-lock test for backlogit_list_items"
source: docs/compound/2026-07-23-cli-mcp-filter-param-denylist-parity-test.md
doc_type: learning
description: "When a CLI command exposes both filter flags (forwarded to the MCP tool) and output-only flags (no MCP equivalent), use a DENYLIST of output-only flags to derive the filter set dynamically from live pflag.FlagSet.VisitAll, then cross-check against live srv.ToolDefs() params. A future CLI-only filter flag propagates automatically into the set and fails the test — true drift protection with no maintained allowlist."
chunk_strategy: h1-h2-h3
schema_version: "1.0"
docline:
    date: 2026-07-23T00:00:00Z
    severity: medium
    tags:
        - mcp
        - cli
        - parity
        - drift-test
        - filter-params
        - denylist
        - list-items
        - tdd
---

# Denylist-Based CLI/MCP Filter Parameter Parity-Lock Test

## Context

Graduated from shipment 103-S / feature 122-F / task 122.001-T (PR #290, merged
`311b3840`). The `backlogit_list_items` MCP tool was missing `priority` and
`owner` filter params that the CLI `backlogit list` exposed via `--priority` and
`--owner`. The data layer (`QueryFilters`) already supported both. The test
approach required protecting against future drift between the CLI's filter flags
and the MCP tool's input schema.

## Problem

`newListCommand` in `internal/cli/list.go` registers **nine** flags: six filter
flags (`type`, `status`, `priority`, `assigned-to`, `owner`, `sprint`) that map
to MCP params, plus three output-only flags (`group-by`, `json`, `format`) that
have no MCP equivalent. A naive allowlist of filter flag names would require
manual updates every time a new filter flag is added. An allowlist test can only
catch MCP drift; it cannot catch CLI-only additions that were never wired to MCP.

## Solution — Denylist Pattern

Place the test in **`internal/cli`** (NOT `internal/mcp`). `internal/cli` already
imports `internal/mcp`, so there is no import cycle and no new exported API is needed.

```go
// internal/cli/list_filter_parity_test.go
// package cli  (same package as list.go — accesses unexported newListCommand)

var listOutputOnlyDenylist = map[string]bool{
    "group-by": true,
    "json":     true,
    "format":   true,
}

func TestListCLIMCPFilterParityLock(t *testing.T) {
    // Derive CLI filter set dynamically — no hard-coded list of filter names.
    cliFilters := map[string]bool{}
    cwd := ""
    cmd := newListCommand(&cwd) // newListCommand takes *string, not nil
    cmd.Flags().VisitAll(func(f *pflag.Flag) {
        if !listOutputOnlyDenylist[f.Name] {
            cliFilters[strings.ReplaceAll(f.Name, "-", "_")] = true
        }
    })

    // Derive MCP param set from live ToolDefs().
    // NewServer requires a real *core.Workspace — spin up a temp workspace.
    root := t.TempDir()
    require.NoError(t, os.MkdirAll(filepath.Join(root, ".backlogit"), 0o755))
    require.NoError(t, config.WriteDefaults(filepath.Join(root, ".backlogit")))
    ws, err := core.NewWorkspace(context.Background(), root)
    require.NoError(t, err)
    t.Cleanup(func() { _ = ws.Close() })

    srv := mcpinternal.NewServer(ws)
    mcpParams := make(map[string]bool)
    for _, tool := range srv.ToolDefs() {
        if tool.Name == "backlogit_list_items" {
            for name := range tool.InputSchema.Properties {
                mcpParams[name] = true
            }
            break
        }
    }
    require.NotEmpty(t, mcpParams, "backlogit_list_items tool not found in ToolDefs")
    assert.Equal(t, sortedBoolMapKeys(cliFilters), sortedBoolMapKeys(mcpParams))
}
```

Normalization: `strings.ReplaceAll(f.Name, "-", "_")` converts `assigned-to` →
`assigned_to` to match MCP snake_case param names.

## Why denylist beats allowlist here

| Approach | New CLI filter added without MCP wiring | Test result |
|---|---|---|
| Allowlist of filter names | Not in allowlist → silently ignored | ✅ PASSES (false green) |
| **Denylist of output-only flags** | Not in denylist → enters filter set → MCP missing it | ❌ FAILS (true drift signal) |

The denylist is stable: output-only flags (`group-by`, `json`, `format`) are
presentation concerns unlikely to gain MCP equivalents. Filter flags grow with
domain needs and should always have an MCP twin.

## Applicability

Use this pattern whenever:

1. A CLI command has a mixed flag surface — some flags are request filters (should
   have MCP equivalents), others are output-only (no MCP equivalent expected).
2. The output-only set is small and stable (good denylist candidates).
3. The filter set may grow as the domain evolves (poor allowlist candidate).

For purely command-level parity (does this MCP tool have a CLI fallback at all?),
see `2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md`. For
catalog/index data parity via DI, see
`2026-06-27-cli-mcp-catalog-parity-via-di-and-index-consistency.md`.

## Canonical filter set at graduation (snake_case)

`assigned_to`, `owner`, `priority`, `sprint`, `status`, `type`
