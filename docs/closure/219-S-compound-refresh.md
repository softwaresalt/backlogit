---
title: "219-S compound refresh and knowledge evaluation"
description: "Post-merge compound-refresh classifications and documentation graduation evaluation for Shipment 219-S (Feature 218-F)."
doc_type: closure
source: docs/closure/219-S-compound-refresh.md
ingested_at: "2026-11-20T19:53:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  shipment_id: 219-S
  feature_id: 218-F
---

## Invocation and evidence

Evaluated relevant `docs/compound/` learnings and durable documentation under Ship Step 6 items 3-5 (`compound-refresh`, `scope: recent`, `mode: propose`). Evidence sources: merged PR `#665` (`7229def0c1308955db257e4eec69d842b93f0256`), `internal/cli/self_install.go`, `internal/cli/root.go`, `internal/cli/mcp_daemon.go`, `internal/config/defaults.go`, `internal/toolchain/verify.go`, `internal/toolchain/launcher.go`, `docs/cli-reference/backlogit_init.md`, [219-S close evidence](evidence/219-S-218-F-close-evidence.json), and [219-S post-merge runtime verification](219-S-post-merge-runtime-verification.md).

## Compound classifications

| Entry | Classification | Rationale |
| --- | --- | --- |
| [Nested os.Root Name is not an absolute validation path](../compound/runtime-errors/nested-os-root-name-is-not-an-absolute-path-2026-11-06.md) | `keep` | Accurate and reinforced by `218.001-T` / `218.007-T`: `openOrCreateValidatedWorkspaceRoot`, `ensureAndOpenTargetToolsDirWithCreated`, and `writeWorkspaceBootstrapDefaults` thread `absRoot` explicitly rather than calling `root.Name()` |
| [Pin the handle, not the path, and bind it to the expected path](../compound/2026-10-27-pin-the-handle-not-the-path.md) | `keep` | Accurate and applied in `openOrCreateValidatedWorkspaceRoot`, `ensureAndOpenTargetToolsDirWithCreated`, and `writeBootstrapFileIfNotExistsInRoot` using segment-by-segment `*os.Root` + `IsSymlinkOrReparsePoint` + `os.SameFile` + `O_EXCL` |
| [Never use a Windows pre-Remove before os.Rename on Go 1.24+](../compound/2026-08-29-windows-preremove-rename-pattern.md) | `keep` | Accurate; `writeFileAtomicallyInRoot` in `internal/cli/self_install.go` uses Go 1.24 `os.Rename` directly without pre-remove data-loss windows |
| [Path confinement helper reuse with relative workspace root](../compound/2026-08-10-path-confinement-helper-reuse-relative-workspace-root-double-join.md) | `keep` | Accurate and reinforced by `218.006-T` / `218.008-T` (`internal/toolchain/verify.go`, `internal/toolchain/launcher.go`) plus `reconcileWorkspaceToolchainYAML` preserving raw `workspace_root` on disk |
| [Derive daemon compatibility from the pinned image](../compound/runtime-errors/derive-daemon-version-from-pinned-image-2026-11-09.md) | `keep` | Accurate; `resolveConfiguredDaemonBinaryRel` and `installWorkspaceRuntimeExecutable` validate the pinned regular binary image separately without ambient `PATH` fallback |
| [MCP-CLI config parity](../compound/2026-05-07-mcp-cli-config-parity.md) | `keep` | Accurate; `BACKLOGIT_WORKSPACE_DIR=""` handling and `toolchain.yaml` overrides align across `init`, `install`, and `toolchain` loaders |
| [Carry-forward continuity files into staging](../compound/workflow-issues/carry-forward-continuity-files-into-staging-2026-09-08.md) | `keep` | Accurate; `219-S` restored `.gsd/config.yaml` and `.gsd/harness-manifest.yaml` forward to `main` (`BE9AD4F1` retained for post-closure `Auto-Tune`) and captured `159B162F` for upstream `gsd` `carry-forward.yaml` schema broadening |

Reviewed: 7 entries (`keep: 7`, `update: 0`, `consolidate: 0`, `replace: 0`, `delete: 0`).

## Documentation graduation assessment

* `docs/cli-reference/backlogit_init.md`: regenerated in `218.005-T` and verified by `CLI Reference Drift` CI.
* `docs/ARCHITECTURE.md` and `AGENTS.md`: no structural updates required (no new packages, MCP tools, or agent/skill boundaries introduced).
* `docs/configuration.md` and `docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md`: updates for `tools_dir` / `.bin` default remain properly tracked in `EF372865` for Stage triage.
