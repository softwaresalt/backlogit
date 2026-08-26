---
chunk_strategy: h1-h2-h3
description: Compacted Stage and Ship memory for shipment 078-S CLI/MCP command parity phase 1.
doc_type: memory
docline:
    ms.date: 2026-07-10T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/memory/compacted/2026-07-10-078-S-cli-mcp-parity-phase1-compacted.md
title: Compacted memory - 078-S CLI/MCP parity phase 1
---
## Summary

Shipment `078-S` established an honest CLI/MCP fallback map and filled the highest-value phase-1 gaps. Stage promoted `E16F4664` and folded `7ECBAC7E`; Ship delivered registry correction, drift tests, `shipment add`, `checkpoint create`, shipment-list items normalization, docs, and post-merge closure.

## Archived originals

* `docs/archive/memory/2026-07-03-stage-E16F4664-cli-mcp-parity-triage.md`
* `docs/archive/memory/2026-07-03-stage-E16F4664-cli-mcp-parity-shipment-complete.md`
* `docs/archive/memory/2026-07-03-ship-078-S-session.md`

## Decisions and outcomes

* Registry over-claims are more dangerous than missing fallbacks; incorrect `link` CLI mappings were stripped and marked MCP-only until a real CLI group exists.
* Drift detection must be driven from the typed MCP tool set, not prose or generated manifests.
* `shipment add` and `checkpoint create` were built before flipping their registry rows so the drift gate stayed honest.
* `docs/cli-reference/` is generated; authored review/design material belongs in `docs/reviews/` and `docs/design-docs/`.
* PR #170 merged by true merge commit `e2ab16c0e893d6bcb260162099b0d3f7e87530c2`; post-merge closure shipped and archived 15 manifest items plus `078-S`.

## Files and verification

* `.autoharness/backlog-registry.yaml`, `internal/cli/registry_parity_test.go`, shipment/checkpoint CLI files and tests, parity matrix, and fallback guide were updated.
* CLI Reference Drift failed once until generated CLI docs were regenerated and CRLF noise restored.
* Copilot review drove safe-default corrections around `docs migrate` and routed deliberation-record drift to Stage.
* Runtime verification and §1.9 passed. New compound learning `2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md` captured the durable pattern.
