---
chunk_strategy: h1-h2-h3
description: ""
doc_type: plan
docline:
    date: 2026-05-06T00:00:00Z
    origin: stash:B76EB8C4,B387FFA9
    status: draft
ingested_at: "2026-06-26T02:33:21Z"
schema_version: "1.0"
source: docs/exec-plans/2026-05-06-cli-agent-interop-plan.md
title: CLI Agent Interop — JSON-RPC Output & Stash Kind Expansion
---

# CLI Agent Interop — JSON-RPC Output & Stash Kind Expansion

## Problem Frame

Backlogit's MCP server provides structured JSON-RPC 2.0 responses that agents
consume directly. The CLI, however, returns either plain-text tables or raw JSON
arrays — neither of which follows the JSON-RPC envelope convention. This means
an agent that invokes the CLI (rather than the MCP server) must use a different
parsing path, and it cannot discover tool capabilities without first connecting
via MCP stdio.

Additionally, the stash subsystem hardcodes five kinds (`feature`, `task`,
`bug`, `epic`, `unknown`) rather than deriving them from the configured WIT
types. This prevents users from stashing `spike`, `subtask`, `deliberation`,
`review`, or `shipment` items directly.

**Scope boundary**: This plan covers CLI output formatting and stash-kind
expansion. It does NOT cover MCP transport changes, HTTP/SSE multiplexing, or
plugin distribution.

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | CLI `--json` output uses JSON-RPC 2.0 result envelope (`{"jsonrpc":"2.0","id":...,"result":...}`) | stash:B76EB8C4 |
| R2 | CLI errors use JSON-RPC 2.0 error envelope (`{"jsonrpc":"2.0","id":...,"error":{"code":...,"message":...}}`) | stash:B76EB8C4 |
| R3 | New `backlogit manifest` command emits tool list matching MCP `tools/list` response | stash:B76EB8C4 |
| R4 | Manifest descriptions must stay consistent with MCP tool descriptions | stash:B76EB8C4 |
| R5 | Add `spike` as a first-class stash kind | stash:B387FFA9 |
| R6 | Add `subtask`, `deliberation`, `review`, `shipment` as stash kinds | stash:B387FFA9 |
| R7 | Stash kinds should be driven from configured WIT metadata rather than a hardcoded list | stash:B387FFA9 |

## Scope Boundaries

### In Scope

- JSON-RPC 2.0 envelope wrapper for CLI `--json` output
- `backlogit manifest` command
- Stash kind list expansion (driven from WIT config)
- Tests for all new behavior

### Non-Goals

- Changing MCP server behavior
- Adding HTTP/SSE transport
- Plugin distribution packaging
- Changing existing `--format table` output
- Backward-incompatible removal of `--format json` (it stays, but gains the envelope)

### Deferred to Implementation

- Exact JSON-RPC `id` generation strategy (counter vs. command name vs. null for notifications)
- Whether `--json` and `--format json` merge into one flag or remain separate

## Implementation Units

### Unit 1: JSON-RPC Envelope Package

**Files:** `internal/cli/format/jsonrpc.go`
**Test files:** `internal/cli/format/jsonrpc_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `internal/cli/format/json.go` renderer pattern
**Dependencies:** none

**Approach:**

Create a `JSONRPCRenderer` implementing the existing `Renderer` interface (or a
parallel envelope writer). The renderer wraps any result payload in:

```json
{"jsonrpc":"2.0","id":1,"result":<payload>}
```

For errors:

```json
{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"<msg>","data":<detail>}}
```

Add a `FormatJSONRPC Format = "jsonrpc"` constant. The renderer accepts a
request ID (defaulting to `1` for CLI invocations) and wraps the inner JSON
payload. Error codes follow the JSON-RPC 2.0 spec: `-32600` for invalid
request, `-32602` for invalid params, `-32000` for application errors.

**Verification:**

- Unit tests confirm envelope structure for success and error cases
- Tests confirm the `"jsonrpc":"2.0"` key is always present
- Tests confirm error envelope includes `code` and `message`

### Unit 2: Wire JSON-RPC Output to CLI Commands

**Files:** `internal/cli/root.go`, `internal/cli/list.go`, `internal/cli/get.go`, `internal/cli/search.go`, `internal/cli/query.go`, `internal/cli/stash.go`, `internal/cli/queue_cmd.go`
**Test files:** `internal/cli/format_flag_test.go`, `internal/cli/root_mcp_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** characterization-first (verify existing JSON output shape, then wrap)
**Patterns to follow:** existing `--format` flag handling in `list.go` and `get.go`
**Dependencies:** Unit 1

**Approach:**

Add a persistent `--json-rpc` flag (or extend `--format` to accept `jsonrpc`)
on the root command. When active, all commands that produce structured output
route through the `JSONRPCRenderer` instead of the plain `JSONRenderer`. Error
returns from `RunE` are caught by a `PersistentPostRunE` or Cobra error hook
and formatted as JSON-RPC error envelopes.

Key decisions:
- Use `--format jsonrpc` rather than a separate boolean to keep the format
  surface unified.
- The `id` field defaults to the Cobra command path (e.g., `"backlogit_list_items"`)
  to align with the MCP tool names.
- Commands that already emit raw JSON (like `stash add`) gain envelope wrapping.

**Verification:**

- Existing `--format json` behavior unchanged (regression tests pass)
- `--format jsonrpc` produces valid JSON-RPC 2.0 envelopes
- Error paths produce JSON-RPC error envelopes with appropriate codes

### Unit 3: `backlogit manifest` Command

**Files:** `internal/cli/manifest.go`
**Test files:** `internal/cli/manifest_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `internal/mcp/tools.go` tool registration, `internal/cli/metadata.go`
**Dependencies:** Unit 1

**Approach:**

Create a `manifest` subcommand that emits a JSON-RPC 2.0 response whose
`result` field contains a tool list matching the shape returned by MCP
`tools/list`. The manifest is generated from a shared tool definition registry
(extracted from or co-located with `internal/mcp/tools.go`).

Implementation path:
1. Extract tool metadata (name, description, input schema) into a shared
   `internal/mcp/toolmeta` or `internal/tooldef` package that both the MCP
   server's `RegisterTools()` and the CLI manifest command can consume.
2. The manifest command reads from this shared registry and renders it as a
   JSON-RPC envelope.
3. This guarantees R4 (descriptions stay consistent) structurally rather than
   by manual synchronization.

**Verification:**

- `backlogit manifest` output is valid JSON-RPC 2.0
- Tool names match those registered in `RegisterTools()`
- Descriptions match exactly (test compares manifest output against MCP tool list)

### Unit 4: Stash Kind Expansion (Config-Driven)

**Files:** `internal/stash/stash.go`, `internal/core/stash.go`, `internal/cli/stash.go`
**Test files:** `internal/stash/stash_test.go`, `internal/core/stash_test.go`, `internal/cli/stash_test.go`
**Effort size:** medium
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** `internal/stash/stash.go` line 41 `allowedKinds`, `internal/config/` WIT type loading
**Dependencies:** none (parallel with Units 1–3)

**Approach:**

Replace the hardcoded `allowedKinds` slice with a function that merges:
1. A built-in base set: `feature`, `task`, `bug`, `epic`, `unknown`
2. Additional kinds derived from the workspace's configured WIT types (loaded
   from `config.yaml` via the existing `config` package)
3. Explicit additions: `spike`, `subtask`, `deliberation`, `review`, `shipment`

Implementation:
- Add a `LoadKindsFromConfig(cfg *config.Config) []string` function in the
  `stash` package that unions the base set with WIT type names.
- `NormalizeKind` gains an optional `validKinds []string` parameter (or the
  function reads from a package-level registry set at workspace open time).
- The MCP tool description for `backlogit_stash` and `backlogit_stash_edit`
  updates to list the expanded kinds.
- The CLI `--kind` flag help text dynamically lists available kinds.

**Verification:**

- `backlogit stash add "test" --kind spike` succeeds
- `backlogit stash add "test" --kind deliberation` succeeds
- Invalid kinds still error
- Kinds list is derived from config rather than hardcoded (test with custom config)

## Dependency Graph

```
Unit 1 (jsonrpc pkg) ──► Unit 2 (wire to CLI)
                    └──► Unit 3 (manifest cmd)
Unit 4 (stash kinds) ── independent, can run in parallel
```

Recommended sequence: Unit 4 first (independent), then Unit 1, then Units 2+3.

## Decisions

| # | Decision | Rationale | Alternatives Rejected |
|---|---|---|---|
| D1 | Use `--format jsonrpc` rather than `--json-rpc` flag | Keeps format surface unified; one flag, multiple values | Separate `--json-rpc` boolean (creates flag interaction complexity) |
| D2 | Set JSON-RPC `id` to the Cobra command path (e.g. `backlogit_list_items`) | Aligns with MCP tool names; agents can correlate responses | Numeric counter (meaningless), null (loses correlation) |
| D3 | Share tool definitions between MCP and manifest via extracted package | Guarantees consistency structurally | Manual sync (drift-prone), code generation (over-engineered for this scope) |
| D4 | Merge WIT config kinds into stash kinds at workspace open | Config-driven, no hardcoded ceiling | Keep hardcoded list + add 5 new entries (violates R7) |
| D5 | Keep `--format json` as raw JSON (backward compat) alongside new `--format jsonrpc` | Avoids breaking existing scripts | Replace json with jsonrpc everywhere (breaking change) |

## Risks and Caveats

- **Tool registry extraction** (Unit 3) touches `internal/mcp/tools.go` which
  is the most-modified file in the codebase. Must not change MCP behavior.
- **Stash kind expansion** changes validation logic used by both CLI and MCP.
  Must ensure MCP stash tool descriptions update to reflect new kinds.
- **Backward compatibility**: existing `--format json` consumers must not break.
  The JSON-RPC envelope is opt-in via `--format jsonrpc`.
- **Error interception**: wrapping Cobra errors in JSON-RPC envelopes requires
  a global error hook. Must not suppress non-JSON output when `--format` is
  `table` or `tile`.

## Plan Hardening Signals (REQUIRED)

* public API, schema, or contract change — **PRESENT** (CLI output contract
  changes for `--format jsonrpc`; manifest is a new public command)
* security, auth, permission, or compliance-sensitive behavior — absent
* migration, backfill, destructive data/config action, or irreversible step — absent
* external integration, operator checkpoint, or external dependency — absent
* high runtime, rollout, or rollback risk — absent

Requires plan hardening: **no**

The contract change is additive (new format value, new command). Existing
behavior is preserved. The risk profile does not warrant hardening beyond
standard review.

## Runtime Verification and Closure

| Unit | Runtime surface | Verification | Closure |
|---|---|---|---|
| Unit 1 | None (library) | Unit tests | N/A |
| Unit 2 | CLI `--format jsonrpc` output | Integration test: run CLI commands with `--format jsonrpc` and validate envelope | Confirm no regression in existing `--format json` |
| Unit 3 | `backlogit manifest` command | Run command, compare output to MCP tool list programmatically | Add to `docs/cli-reference/` |
| Unit 4 | Stash intake validation | `backlogit stash add --kind spike` succeeds end-to-end | Update MCP stash tool descriptions |

## Learnings Applied

No directly applicable compound learnings found for JSON-RPC CLI output.
Stash validation patterns follow the established `NormalizeKind` approach.

## Standards Check

- All new code in Go 1.22+, GoDoc on exports ✓
- Test-first for all units ✓
- `golangci-lint` zero-warning gate applies ✓
- Conventional commits for each unit ✓
- No `panic()` in library code ✓
- Typed errors via `internal/errors` ✓

## Plan Review

**Gate Decision: ADVISORY**

Plan is structurally sound, constitution-compliant, and ready for harvest with
the advisories below acknowledged. No P0 or P1 issues found.

### P0 — Critical

None.

### P1 — High

None.

### P2 — Moderate (3 findings)

**F1 — Envelope application layer ambiguity** (Architecture Strategist, Go Quality)

The existing `format.Renderer` interface accepts `(columns []Column, rows
[]map[string]any)` — a tabular shape. Commands like `get` bypass the renderer
and write single JSON objects directly via `json.Encoder`. The plan proposes a
`JSONRPCRenderer` implementing `Renderer`, but this only covers tabular
commands. The implementer must also wrap single-object outputs (get, stash add,
etc.) in the JSON-RPC envelope.

*Recommendation*: Clarify during implementation that the envelope is applied at
Cobra middleware level (a `PersistentPostRunE` or stdout interceptor) rather
than only through the `Renderer` interface. The plan already hints at this
("PersistentPostRunE or Cobra error hook") but should be the primary path.

**F2 — Import cycle risk for shared tool package** (Go Quality)

Extracting tool metadata from `internal/mcp/tools.go` into a shared package
consumed by both `internal/mcp` and `internal/cli/manifest.go` requires the
new package to have zero imports back into `internal/mcp`. The plan should name
the package explicitly (e.g., `internal/tooldef/`) and confirm it depends only
on the `mcp-go` schema types, not on the `Server` struct.

*Recommendation*: Name the package `internal/tooldef` and ensure it imports
only `github.com/mark3labs/mcp-go/mcp` for schema types.

**F3 — Manifest parameter schema completeness** (Agent-Native Parity)

MCP `tools/list` returns full JSON Schema for each tool's `inputSchema`
(parameter names, types, required flags, descriptions). The plan says the
manifest "matches the shape returned by MCP tools/list" but doesn't explicitly
state that parameter schemas are included. Without them, an agent cannot
construct valid CLI invocations from the manifest alone.

*Recommendation*: Confirm during implementation that `backlogit manifest`
includes `inputSchema` per tool, not just names and descriptions.

### P3 — Low (1 finding)

**F4 — JSON-RPC `id` semantics** (Constitution Reviewer)

Using the Cobra command path as the `id` field is unconventional (JSON-RPC spec
says `id` is caller-provided). This is fine for CLI-initiated requests but
should be documented in the manifest output so agents understand the convention.

### Reviewer Attribution

| Finding | Reviewer | Model |
|---|---|---|
| F1 | Architecture Strategist + Go Quality | claude-opus-4.6 |
| F2 | Go Quality Reviewer | claude-opus-4.6 |
| F3 | Agent-Native Parity Reviewer | claude-opus-4.6 |
| F4 | Constitution Reviewer | claude-opus-4.6 |

### Next Steps

Gate is **ADVISORY**. P2 findings are implementation-time clarifications, not
plan restructuring requirements. Recommend proceeding to `harvest` with the
advisories acknowledged as implementation guidance.
