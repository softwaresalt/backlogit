---
chunk_strategy: h1-h2
description: 'Compound refresh report for shipment 078-S — reviewed the CLI/MCP parity compound entries, classified the two existing entries as keep (distinct layers), and captured one new evidence-backed learning covering the honest registry fallback map, drift test, gap-fill discipline, output-shape parity, and fallback blast-radius parity.'
doc_type: closure
docline:
    date: 2026-07-03T00:00:00Z
    severity: medium
    tags:
        - compound-refresh
        - cli
        - mcp
        - parity
        - closure
schema_version: "1.0"
source: docs/closure/2026-07-03-078-S-cli-mcp-command-parity-compound-refresh.md
title: 'Compound refresh — 078-S CLI/MCP command parity'
---

# Compound Refresh — 078-S CLI/MCP Command Parity

- **Context**: post-merge closure for shipment 078-S (feature 078-F, PR #170, merge `e2ab16c`)
- **Scope**: `docs/compound/` entries tagged `cli` / `mcp` / `parity`
- **Mode**: apply (new capture) + propose (existing entries)
- **Generated**: 2026-07-03

## Entries Reviewed and Classifications

| Entry | Classification | Rationale |
|---|---|---|
| `2026-06-27-cli-mcp-catalog-parity-via-di-and-index-consistency.md` | **keep** | Distinct layer: catalog *data* parity via dependency injection + index/dry-run/typed-error rules. Not superseded by 078-S; still accurate against current code. |
| `2026-05-07-mcp-cli-config-parity.md` | **keep** | Distinct concern: config *option loading* parity in both handlers. Orthogonal to command-level fallback mapping. |
| `go-patterns/manual-schema-registry-drift-detection-2026-05-22.md` | **keep** | Schema-registry drift detection; complementary technique, different registry. 078-S applies the same drift-test philosophy to the MCP->CLI op-map but does not supersede it. |
| `2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md` | **new capture** | Genuinely new, evidence-backed command-level learning from 078-S; see below. |

## New Capture

Created `docs/compound/2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md`
(lint-clean, `doc_type: learning`) with four durable rules:

1. Declare an **honest** MCP->CLI fallback map — never fabricate a CLI command — and lock it with a registry drift test (`registry_parity_test.go`, 4 assertions driven from the live command tree).
2. When a parity gap is real, either **add** the CLI command (harness-first) or **mark it deferred**; let the drift test be the worklist. New commands assert output-shape parity, not just exit code.
3. Normalize cross-surface output shape — arrays serialize as `[]`, never `null` — and test **both** CLI and MCP from the same stored state.
4. A CLI fallback template must be **no more dangerous** than the MCP default it mirrors (the `docs_migrate --apply --yes` blast-radius finding).

## Design Decision Graduation

The MCP-to-CLI fallback discoverability guide shipped as a durable design doc in the
feature PR: `docs/design-docs/2026-07-03-mcp-to-cli-fallback-guide.md`. It is already
on `main` and needs no further graduation. The parity audit matrix
`docs/reviews/2026-07-03-cli-mcp-parity-matrix.md` remains the point-in-time audit
record.

## Follow-up Items

- None requiring manual review. The Stage-owned deliberation-record reconciliation
  (drift on lines 86/88/94) is already tracked in stash `2827CB5F` for Stage and is
  explicitly out of Ship's P-010 boundary.

## Evidence

- Shipped, merged code on `main` at `e2ab16c` (feature PR #170).
- Test names cited verbatim from `internal/cli/registry_parity_test.go`,
  `internal/cli/shipment_list_items_test.go`, `internal/cli/shipment_add_test.go`,
  `internal/cli/checkpoint_create_test.go`.
- Runtime verification PASS: `docs/closure/2026-07-03-078-S-cli-mcp-command-parity-runtime-verification.md`.
