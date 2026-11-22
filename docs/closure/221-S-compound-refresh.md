---
title: "221-S compound refresh and knowledge evaluation"
description: "Post-merge compound-refresh classifications and documentation graduation evaluation for Shipment 221-S (Feature 220-F)."
doc_type: closure
source: docs/closure/221-S-compound-refresh.md
ingested_at: "2026-11-22T03:02:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  shipment_id: 221-S
  feature_id: 220-F
---

## Invocation and evidence

Evaluated relevant `docs/compound/` learnings and durable documentation under Ship Step 6 items 3-5 (`compound-refresh`, `scope: recent`, `mode: propose`). Evidence sources: merged PR `#670` (`a079f4527a9243413ea992ad0701225fac722cc5`), `internal/mcp/session_mux.go`, `internal/mcp/session_mux_test.go`, `docs/configuration.md`, `docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md`, [221-S close evidence](evidence/221-S-220-F-close-evidence.json), and [221-S post-merge runtime verification](221-S-post-merge-runtime-verification.md).

## Compound classifications

| Entry | Classification | Rationale |
| --- | --- | --- |
| [Match MCP validation to the SDK argument parser](../compound/security-issues/mcp-validator-sdk-parser-differentials-2026-11-08.md) | `keep` | Accurate and reinforced by `220.001-T`: `validateSessionPathForOS` normalizes both `/` and backslashes (`\\`) before `isLocalSessionPathForOS`, `..` rejection, and per-component `root.Lstat` + `core.IsSymlinkOrReparsePoint`, closing Windows separator differentials; remaining `TrimSpace` / missing-`id` / empty-`RootPath` boundary hardening is tracked in `DCE8FD71`, `272A805F`, and `5C70ECD0` |
| [A nested os.Root Name is not an absolute validation path](../compound/runtime-errors/nested-os-root-name-is-not-an-absolute-path-2026-11-06.md) | `keep` | Accurate; `validateSessionPathForOS` computes `absRoot, err := filepath.Abs(s.RootPath)` and passes `filepath.Join(absRoot, relative)` to `core.IsSymlinkOrReparsePoint` while probing via `root.Lstat(relative)` |
| [Match CI Go and linter versions before source repair](../compound/build-errors/match-ci-go-and-linter-before-source-repair-2026-11-06.md) | `keep` | Accurate; running `GOTOOLCHAIN=go1.24.0` with pinned `golangci-lint@v1.64.8` and `-p 1` avoided host Go 1.27 export-data mismatches and concurrent linter lock contention during `221-S` verification |
| [Carry-forward continuity files into staging](../compound/workflow-issues/carry-forward-continuity-files-into-staging-2026-09-08.md) | `keep` | Accurate; `221-S` kept `.gsd/carry-forward.yaml`, `.gsd/worktree-baseline.json`, and `.kbdb/` excluded from every commit across `feat/221-s` and `post-merge/221-s-closure` |

Reviewed: 4 entries (`keep: 4`, `update: 0`, `consolidate: 0`, `replace: 0`, `delete: 0`).

## Documentation graduation assessment

* `docs/configuration.md` and `docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md`: updated directly inside `220.002-T` for optional `tools_dir`, 5-source discovery precedence, `.bin` default / `bin` fallback, `VerifyProposedLauncherScripts`, and `backlogit init` / `install [--tools-dir <dir>]`.
* `docs/ARCHITECTURE.md`, `AGENTS.md`, and `docs/product-specs/`: no structural updates required (no new packages, MCP tools, or role boundaries introduced; `validateSessionPathForOS` is package-internal in `internal/mcp`).
* Remaining pre-existing follow-ups (`DCE8FD71`, `272A805F`, `5C70ECD0`, `8827BED4`, `A748BF57`) are captured in `.backlogit/stash.jsonl` for Stage triage.
