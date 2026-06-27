---
title: "Ship session: 048-S CLI Agent Interop shipped"
description: "Session continuity for shipping 048-S — CLI Agent Interop JSON-RPC output and manifest command"
ms.date: 2026-05-07
ms.topic: reference
---

## Session summary

Shipped 048-S (CLI Agent Interop) from feature 049-F through the full Ship pipeline: implementation,
review gate, CI remediation, PR merge (#87), and post-merge closure.

## Tasks Completed

| Task | Description | Status | Notes |
|---|---|---|---|
| 049.005-T | JSON-RPC 2.0 envelope package | done | Canonical task |
| 049.006-T | Wire `--jsonrpc` persistent flag | done | Canonical task |
| 049.007-T | `backlogit manifest` command | done | Canonical task |
| 049.008-T | Config-driven stash kind expansion | done | Canonical task |
| 049.001-T | JSON-RPC Envelope Package (draft) | archived | Superseded by 049.005-T during plan revision |
| 049.002-T | Wire JSON-RPC Output (draft) | archived | Superseded by 049.006-T during plan revision |
| 049.003-T | manifest Command (draft) | archived | Superseded by 049.007-T during plan revision |
| 049.004-T | Stash Kind Expansion (draft) | archived | Superseded by 049.008-T during plan revision |
| 049-F | CLI Agent Interop feature | done | |
| 048-S | Shipment | archived (shipped) | |

> **Note on draft tasks 049.001–049.004**: These were initial task stubs created during the first
> planning pass. They were superseded when the implementation plan was revised to assign canonical
> IDs 049.005–049.008. Both sets are archived; the canonical tasks (005–008) contain the actual
> implementation scope.

## Files Modified

- `internal/stash/stash.go` — `allowedKinds` expanded (10 entries); `NormalizeKindWithExtras`, `AllowedKindsWithExtras` added
- `internal/stash/stash_test.go` — 7 new tests
- `internal/core/stash.go` — `witKindExtras` helper; 3 `NormalizeKind` call sites updated
- `internal/cli/stash.go` — `--kind` help text updated
- `internal/cli/format/jsonrpc.go` — NEW: JSON-RPC 2.0 types and helpers
- `internal/cli/format/jsonrpc_test.go` — NEW: 12 tests
- `internal/cli/format/renderer.go` — `FormatJSONRPC` constant, `NewRenderer` factory
- `internal/cli/root.go` — `jsonrpcInterceptor`, `Execute()`, `newRootCommandImpl`, `PersistentPreRunE`/`PersistentPostRunE`, `--jsonrpc` flag; fixed `--jsonrpc=true` pre-scan, `WrapError` error handling
- `internal/cli/jsonrpc_test.go` — NEW: 7 tests
- `internal/cli/manifest.go` — NEW: `backlogit manifest` command
- `internal/cli/manifest_test.go` — NEW: 6 tests
- `internal/mcp/server.go` — `ToolDefs()` method
- `cmd/backlogit/main.go` — Updated to call `cli.Execute()`
- `docs/cli-reference/` — All 55 files regenerated (incl. new `backlogit_manifest.md`)

## Key Decisions

1. **`Execute()` wrapper over `NewRootCommand().Execute()`**: Cobra's `PersistentPostRunE` is skipped on error. `Execute()` pre-scans `os.Args` and writes error envelopes directly after `root.Execute()` returns an error.

2. **Pre-scan matches `--jsonrpc`, `--jsonrpc=true`, `--jsonrpc=1`**: pflag accepts multiple boolean flag forms; the scan explicitly excludes `=false` and `=0`.

3. **`WrapResult` forces `result` key even when nil**: JSON-RPC 2.0 spec requires the key present. Uses anonymous inner struct to override `omitempty`.

4. **`NewServerForRoot` for manifest**: No workspace initialization needed — just registers tools and returns `ToolDefs()`. No disk writes occur.

5. **Alphabetical sort on manifest output**: Consistent with MCP `tools/list` behavior.

6. **`NormalizeKind` backward-compatible**: Delegates to `NormalizeKindWithExtras(kind, nil)`. All existing call sites unchanged.

## PR & CI

- PR #87 merged via `gh pr merge 87 --merge --admin`
- Merge commit: `cc7d217`
- All CI gates passed (CLI Reference Drift, Go 1.23, Go 1.24)
- 2 Copilot review comments fixed, replied to, and resolved programmatically

## Compound Learnings Written

- `docs/compound/go-patterns/f049-jsonrpc-cli-interceptor-patterns.md`
  — 5 patterns: Cobra PostRun skip, pflag boolean pre-scan, `WrapResult` nil key, MCP manifest via `ToolDefs()`, test-facing vs production execute path

## Source Artifact Cleanup

- 049-F: no `source_stash_id`, no `source_deliberation_id` — no cleanup needed
- 048-S: shipped and archived

## Next Steps

- Await operator approval for closure PR (post-merge/049-cli-agent-interop)
- Then look at next stash items for staging:
  - `B4491F8C` (high): Incremental telemetry JSONL datastore from Copilot session logs
  - `2F295E2B` (high/unknown): Spike on telemetry attribution analytics
  - `A02FA570` (high/epic): Copilot CLI plugin distribution
