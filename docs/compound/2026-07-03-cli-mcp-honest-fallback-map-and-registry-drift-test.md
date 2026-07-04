---
chunk_strategy: h1-h2-h3
description: 'Four durable rules graduated from 078-S — declare an HONEST MCP->CLI fallback map in the registry (never fabricate a CLI command that does not exist) and lock it with a drift test; when a gap is real, either add the CLI command or mark it deferred, driven by the drift test; normalize cross-surface output shape (arrays never null) with a guard test that exercises both CLI and MCP; and keep a CLI fallback template no more dangerous than the MCP default it mirrors (blast-radius parity).'
doc_type: learning
docline:
    date: 2026-07-03T00:00:00Z
    severity: high
    tags:
        - mcp
        - cli
        - parity
        - registry
        - drift-test
        - fallback
        - blast-radius
        - output-normalization
        - discoverability
schema_version: "1.0"
source: docs/compound/2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md
title: 'Honest MCP->CLI fallback map + registry drift test, gap-fill discipline, output-shape parity, and fallback blast-radius parity (078-S)'
---

# Honest MCP->CLI Fallback Map + Registry Drift Test

Four durable rules graduated from shipment 078-S (feature 078-F, "CLI/MCP command
parity: honest fallback map + highest-value gap fills", PR #170, merge
`e2ab16c0e893d6bcb260162099b0d3f7e87530c2`). This entry is the *command-level*
sibling of `2026-06-27-cli-mcp-catalog-parity-via-di-and-index-consistency.md`
(catalog *data* parity via DI) and `2026-05-07-mcp-cli-config-parity.md` (config
*option loading* parity). Those remain accurate and distinct — this one is about
the registry's honest declaration of which MCP tools have a real CLI fallback.

## Rule 1 — Declare an HONEST fallback map; never fabricate a CLI command, and lock it with a drift test

### Problem

`.autoharness/backlog-registry.yaml` pairs each MCP tool with a `cli_command`
fallback so agents in CLI-only mode know what to run. The failure mode is a map
that *lies*: it declares a `cli_command` for a tool whose CLI command does not
actually exist, or omits a fallback for a tool that does have one. An agent that
trusts a fabricated fallback runs a non-existent command and silently loses the
operation.

### Rule

The fallback map must be **honest**: every declared `cli_command` MUST resolve to
a real cobra command (including the exact flags named in the template), and every
MCP tool that genuinely lacks a CLI equivalent MUST be explicitly marked deferred
rather than left ambiguous. Lock the contract with a **registry drift test** that
walks `ListTools()` against `DescribeCLICommands` / the cobra tree:

* `TestRegistryParity_EveryMCPToolMappedOrDeferred` — every tool is either mapped
  to a resolvable CLI command or honestly marked deferred.
* `TestRegistryParity_EveryCLICommandResolves` — every declared `cli_command`
  resolves to a real command in the cobra tree.
* `TestRegistryParity_NoOrphanMCPTool` — no tool falls through both branches.
* `TestRegistryParity_DiscoverabilityConsistency` — the discoverability guide and
  the registry agree.

### Why it works

The test is driven from the *live* command tree, not a hand-maintained list, so it
fails the moment the registry drifts from the actual CLI surface. YAML comments in
the registry do not affect it (the test reads structured fields only), so the map
can carry human-facing rationale without weakening the guard.

## Rule 2 — When a parity gap is real, either add the CLI command or mark it deferred — let the drift test decide what to fill

### Problem

Some MCP operations had no CLI equivalent at all (`shipment add`,
`checkpoint create`). "Parity" is not achieved by pretending the gap is closed; it
is achieved by closing the highest-value gaps and being honest about the rest.

### Rule

Use the drift test as the worklist. For each unmapped tool, make a deliberate
choice: **add** the CLI command (harness-first: write the CLI test red, then
implement `newShipmentAddCmd` / `newCheckpointCreateCmd` green), or **defer** it
with an explicit marker. Do not fabricate. New CLI commands should assert
**output-shape parity** with their MCP sibling in the test, not just exit code:

* `TestShipmentAdd_HappyPath_OutputShapeParity`, `TestShipmentAdd_IdempotentReAdd`,
  `TestShipmentAdd_ItemInAnotherShipment_SentinelError`,
  `TestShipmentAdd_RegisteredInShipmentGroup`.
* `TestCheckpointCreate_WritesReadableCheckpoint`,
  `TestCheckpointCreate_MissingStateDump`, `TestCheckpointCreate_InvalidSchema`.

## Rule 3 — Normalize cross-surface output shape: arrays are `[]`, never `null` — and test BOTH surfaces

### Problem

`shipment list` emitted `items: null` when a shipment had no items stored, while
the MCP pipeline emitted `[]`. A consumer that does `.items.length` breaks on one
surface but not the other — a parity bug that hides on the surface you did not
touch (the same lesson as index-consistency Rule 2 in the 061-S entry).

### Rule

Any field typed as a collection MUST serialize as an empty array, never `null`,
and the guard test MUST exercise **both** the CLI handler and the MCP pipeline
from the same stored state:

* `TestShipmentList_NullStoredItems_CLINeverNull`,
  `TestShipmentList_EmptyItems_CLINeverNull`,
  `TestShipmentList_NullStoredItems_MCPPipelineNeverNull`.

Normalize at the handler boundary (initialize the slice to a non-nil empty slice
before marshaling), not at the call site, so every caller inherits the guarantee.

## Rule 4 — A CLI fallback template must be no more dangerous than the MCP default it mirrors (blast-radius parity)

### Problem

The `docs_migrate` fallback template was drafted as `--apply --yes`, which makes
the CLI fallback **always write** — strictly higher blast radius than the MCP tool
whose default is `apply=false` (plan-only). A fallback that is more destructive
than the primary silently escalates risk whenever an agent drops to CLI mode.

### Rule

A fallback command must mirror the **default safety posture** of the operation it
stands in for. If the MCP default is plan-only / dry-run, the CLI fallback
template is plan-only too (`backlogit docs migrate --path {{path}}`), with the
destructive escalation (`--apply --yes`) documented in a YAML comment as an
*opt-in* the operator must add — never baked into the default template. Blast
radius must be equal or *lower* on the fallback path, never higher.

## References

- PR #170 — feat: CLI/MCP command parity — honest fallback map + gap fills (078-S / 078-F)
- Merge commit `e2ab16c0e893d6bcb260162099b0d3f7e87530c2`
- Registry op-map: `.autoharness/backlog-registry.yaml`
- Drift test: `internal/cli/registry_parity_test.go`
- Output-shape guard: `internal/cli/shipment_list_items_test.go`
- Gap-fill tests: `internal/cli/shipment_add_test.go`, `internal/cli/checkpoint_create_test.go`
- Discoverability guide: `docs/design-docs/2026-07-03-mcp-to-cli-fallback-guide.md`
- Parity audit matrix: `docs/reviews/2026-07-03-cli-mcp-parity-matrix.md`
- Runtime verification: `docs/closure/2026-07-03-078-S-cli-mcp-command-parity-runtime-verification.md`
- Related (complementary, kept): `docs/compound/2026-06-27-cli-mcp-catalog-parity-via-di-and-index-consistency.md`,
  `docs/compound/2026-05-07-mcp-cli-config-parity.md`
