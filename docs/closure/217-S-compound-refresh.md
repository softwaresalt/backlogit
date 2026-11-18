---
title: "217-S compound refresh and knowledge evaluation"
description: "Post-merge compound-refresh classifications and documentation graduation evaluation for Shipment 217-S (216-F / CAP-3)."
doc_type: closure
source: docs/closure/217-S-compound-refresh.md
ingested_at: "2026-11-18T03:17:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  shipment_id: 217-S
  feature_id: 216-F
---

## Invocation and evidence

Evaluated relevant `docs/compound/` learnings and durable documentation under
Ship Step 6 items 3-5 (`compound-refresh`, `scope: recent`, `mode: propose`).
Evidence sources: merged PR `#656` (`75e27861fac309e79113c270bcaba1927dfdc767`),
`internal/core/commits.go`, `internal/events/reader.go`,
`internal/core/artifacts.go`, `internal/core/shipment_lifecycle.go`,
`internal/core/shipment_gate.go`, `internal/core/workspace.go`,
`internal/db/connection.go`, `internal/db/schema.go`,
`internal/core/custody_exec.go`, `internal/core/archive.go`, and
[217-S close evidence](evidence/217-S-216-F-close-evidence.json).

## Compound classifications

| Entry | Classification | Rationale |
| --- | --- | --- |
| [Ancestor-aware member-evidence staleness](../compound/2026-07-06-ancestor-aware-shipment-gate-staleness.md) | `keep` | Accurate and reinforced by `216.004-T`: `isAncestor` now adds `--no-replace-objects`, `--`, symlink/junction root resolution, and `.git/shallow` / `.git/info/grafts`-guarded bounded memoization while preserving fail-closed `runCtx.Err() != nil` handling |
| [backlogit_ship_shipment MCP timeout means result unknown](../compound/2026-10-06-ship-shipment-mcp-timeout-result-unknown.md) | `keep` | Still accurate for pre-CAP-3 MCP server binaries (`backlogit_ship_shipment` returned `MCP error -32001: Request timed out` and finished `217-S` closure on disk in `661s`) and documents the exact timeout-observation protocol; `216.002-T`/`216.003-T`/`216.004-T` address the underlying per-member latency in `main` |
| [Holding a lock across a side effect changes its lock order](../compound/2026-10-19-lock-held-side-effect-ordering.md) | `keep` | Accurate; directly cites `703835DC`, which `216.001-T` resolved for `LinkCommit` and `AssociateCommit` (`lifecycle-global -> C -> B`), while the general lock-ordering rule remains durable |
| [bufio.Scanner fix for oversized JSONL lines missed in db package](../compound/runtime-errors/bufio-scanner-incomplete-fix-missed-db-package-2026-04-25.md) | `keep` | Accurate; consistent with `216.002-T` (`events.ReadAllEvents` preserves the `1 MiB` max capacity while shrinking initial allocation to `64 KiB` and returning explicit scanner/context errors) |
| [Exported short-circuit caches must treat their zero value as unseeded](../compound/best-practices/exported-cache-zero-value-bypass-2026-06-29.md) | `keep` | Accurate; reinforced by `216.004-T`'s unexported, nil-guarded `Workspace.ancestryCache` `map[string]bool` field lazily initialized under `ancestryMu` and capped at `maxAncestryCacheEntries` (`4096`) |

Reviewed: 5 entries (`keep: 5`, `update: 0`, `consolidate: 0`, `replace: 0`, `delete: 0`).

## Documentation graduation assessment

`docs/ARCHITECTURE.md` and `AGENTS.md` require no structural updates for `217-S`:
no new packages, CLI commands/flags, MCP tools, or agent/skill boundaries were
introduced, and `CLI Reference Drift` CI confirmed `docs/cli-reference/` has
zero drift. Four out-of-scope P-021 C2 `DEFERRED SCOPE EXPANSION` entries
(`451486E7`, `FDFC52FF`, `DA6809A0`, `D1CA631D`) remain active in
`.backlogit/stash.jsonl` for Stage triage, and `df4a2df9` (`101-DL`) remains
explicitly unresolved as a tracked diagnostic risk.
