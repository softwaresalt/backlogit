---
title: "218-S compound refresh and knowledge evaluation"
description: "Post-merge compound-refresh classifications and documentation graduation evaluation for Shipment 218-S (Feature 217-F)."
doc_type: closure
source: docs/closure/218-S-compound-refresh.md
ingested_at: "2026-11-19T16:26:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  shipment_id: 218-S
  feature_id: 217-F
---

## Invocation and evidence

Evaluated relevant `docs/compound/` learnings and durable documentation under Ship Step 6 items 3-5 (`compound-refresh`, `scope: recent`, `mode: propose`). Evidence sources: merged PR `#662` (`37966bf4b2af72ca8c822bfd29f186180a27cb08`), `internal/toolchain/types.go`, `internal/toolchain/verify.go`, `internal/toolchain/launcher.go`, `schemas/workspace-toolchain.schema.json`, `internal/config/schema.go`, `internal/config/loader.go`, `internal/cli/mcp_daemon.go`, `internal/core/id_history.go`, and [218-S close evidence](evidence/218-S-217-F-close-evidence.json).

## Compound classifications

| Entry | Classification | Rationale |
| --- | --- | --- |
| [Derive daemon compatibility from the pinned image](../compound/runtime-errors/derive-daemon-version-from-pinned-image-2026-11-09.md) | `keep` | Accurate and reinforced by `217.003-T`: `pinnedMCPDaemonBinary` resolves the pinned binary via `.mcp.json` -> `toolchain.yaml` -> `config.yaml` `tools_dir` -> `.bin` -> `bin` and queries its image version without trusting `PATH` or daemon sidecar claims |
| [A nested os.Root Name is not an absolute validation path](../compound/runtime-errors/nested-os-root-name-is-not-an-absolute-path-2026-11-06.md) | `keep` | Accurate and directly followed by `openLocalFileInRoot` and `verifyPinnedDaemonBinaryRel` in `internal/cli/mcp_daemon.go`, which thread `absWorkspace` rather than `current.Name()` |
| [Pin the handle, not the path, and bind it to the expected path](../compound/2026-10-27-pin-the-handle-not-the-path.md) | `keep` | Accurate and reinforced by `217.003-T`: segment-by-segment `OpenRoot` + `rejectLink` + `os.SameFile` handle traversal plus `unicode.SimpleFold` duplicate-key scanning (`scanDaemonJSON`, `validateDaemonYAMLKeys`) |
| [Exec binary config must be bare-path validated](../compound/2026-07-06-exec-binary-config-must-be-bare-path-validated.md) | `keep` | Remains accurate and scoped strictly to the pre-task gate broker (`gate.binary`), which resolves a bare command on `PATH`. Distinct from the MCP daemon and toolchain workspace-pinning boundary (`validateBacklogitCommandRel`, `validateWorkspaceToolsDir`, `validManifestToolsDir`), which requires workspace-contained subdirectory paths and forbids bare commands and ambient `PATH` fallback |
| [Bounded helper timeout hard cap](../compound/2026-07-06-bounded-helper-timeout-hard-cap.md) | `keep` | Accurate; `scanLocalBranchArtifactIDsContext` preserves `ws.boundedHelperTimeout()` (5s cap), `--no-replace-objects --no-optional-locks`, and sanitized git env while batching `cat-file --batch` |

Reviewed: 5 entries (`keep: 5`, `update: 0`, `consolidate: 0`, `replace: 0`, `delete: 0`).

## Documentation graduation assessment

`docs/ARCHITECTURE.md` and `AGENTS.md` require no structural updates for `218-S`: no new packages, CLI commands/flags, MCP tools, or agent/skill boundaries were introduced, and `CLI Reference Drift` CI confirmed `docs/cli-reference/` has zero drift. Three out-of-scope stash entries (`85442104`, `F1D193CC`, `618AA1C0`) remain active in `.backlogit/stash.jsonl` for Stage triage.
