---
chunk_strategy: h1-h2
description: 'Compound refresh report for shipment 079-S — reviewed the CLI/MCP parity compound entries, classified the three existing entries as keep (distinct layers, still accurate), and captured one new evidence-backed learning covering shared-EventWriter threading through a core extraction so a long-lived MCP server preserves per-item JSONL append serialization while a one-shot CLI reuses the same code path.'
doc_type: closure
docline:
    ms.date: 2026-07-03T00:00:00Z
    ms.topic: reference
ingested_at: "2026-07-03T00:00:00Z"
schema_version: "1.0"
source: docs/closure/2026-07-03-079-S-cli-mcp-command-parity-phase2-compound-refresh.md
title: 'Compound refresh — 079-S CLI/MCP command parity phase-2'
---

# Compound Refresh — 079-S CLI/MCP Command Parity Phase-2

- **Context**: post-merge closure for shipment 079-S (feature 079-F, PR #172, merge `a8e07ea`)
- **Scope**: `docs/compound/` entries tagged `cli` / `mcp` / `parity` / `events` / `concurrency`
- **Mode**: apply (new capture) + propose (existing entries)
- **Generated**: 2026-07-03

## Entries Reviewed and Classifications

| Entry | Classification | Rationale |
|---|---|---|
| `2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md` | **keep** | The phase-1 (078-S) command-level parity learning. Phase-2 *extended* its drift test with flag/positional/required-flag parity and flipped 10 more registry rows, but the four durable rules (honest map + drift test; gap-fill discipline; output-shape parity; blast-radius parity) still hold verbatim. Not superseded — strengthened. |
| `2026-06-27-cli-mcp-catalog-parity-via-di-and-index-consistency.md` | **keep** | Distinct layer: catalog *data* parity via dependency injection + index/dry-run/typed-error rules. Orthogonal to command-level fallback mapping and to the append-serialization concern. |
| `2026-05-07-mcp-cli-config-parity.md` | **keep** | Distinct concern: config *option loading* parity in both handlers. Orthogonal to 079-S. |
| `2026-07-04-core-extraction-shared-eventwriter-append-serialization.md` | **new capture** | Genuinely new, evidence-backed concurrency-correctness learning from 079-S's `core.AppendComment` extraction (Copilot Round-1 finding). See below. |

## New Capture

Created `docs/compound/2026-07-04-core-extraction-shared-eventwriter-append-serialization.md`
(lint-clean, `doc_type: learning`). One durable rule:

When extracting an MCP write handler into a shared `core` function so a CLI command
can reuse it, **thread the caller's `*events.EventWriter` through the function** —
the MCP surface passes the server's long-lived shared `s.Events`; the one-shot CLI
passes `nil` (the function constructs a per-invocation writer, nil-guarded).
Minting a fresh `EventWriter` **inside** the core function on every call silently
drops the MCP server's append serialization, because `EventWriter.mu` only
serializes callers that share **one** instance. Under concurrent MCP
`append_comment` calls this would interleave / corrupt per-item JSONL. The fix
mirrors the established `handleMoveItem` pattern (`s.Events.AppendEvent` +
`IndexEvent`). Zero-Timestamp indexing behavior of `AppendComment` is preserved
(deliberately not mirroring `LinkCommit`'s timestamp).

## Design Decision Graduation

The MCP-to-CLI fallback discoverability guide graduated during phase-1
(`docs/design-docs/2026-07-03-mcp-to-cli-fallback-guide.md`, already on `main`).
Phase-2 extended its coverage through the U7 discoverability-doc update shipped in
the feature PR; no separate design-doc graduation is needed. The phase-2 execution
plan and deliberation (`051-DL`) remain the point-in-time decision record.

## Follow-up Items

- None requiring manual review. Phase-3 deferrals (`merge_sync` CLI fallback,
  `export-command-map` pairing) and the permanent-`mcp_only` `log_telemetry` decision
  are pre-documented in the plan and the registry; no new stash entries created
  (per operator instruction). The four low-priority carried-forward stashes
  (`21E17BFC`, `9140F65C`, `EED25928`, `B55985DD`) are Stage-owned and out of Ship's
  P-010 boundary.

## Evidence

- Shipped, merged code on `main` at `a8e07ea` (feature PR #172).
- Concurrency fix commit `6257fab` (`fix(core): thread shared EventWriter through AppendComment`).
- Reference pattern: `internal/mcp/tools.go` `handleMoveItem` (shared `s.Events` writer);
  `internal/core/commits.go` `AppendComment` (now takes `ew *events.EventWriter`, nil-guarded);
  `internal/mcp/tools.go` `handleAppendComment` (passes `s.Events`);
  `internal/cli/comment.go` `newCommentAddCmd` (passes `nil`).
- Test: `internal/mcp/append_comment_test.go`.
- Drift + flag-parity gate: `internal/cli/registry_parity_test.go`
  `TestRegistryParity_FlagAndPositionalParity` (required-flag coverage added in `c19b5c2`).
- Runtime verification PASS:
  `docs/closure/2026-07-03-079-S-cli-mcp-command-parity-phase2-runtime-verification.md`.
