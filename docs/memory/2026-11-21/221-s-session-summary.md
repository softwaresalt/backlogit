---
title: "221-S pre-PR execution and review readiness summary"
description: "Session summary for Shipment 221-S (Feature 220-F) covering Wave 1 execution, quality gates, and P-021 C2 deferrals."
doc_type: memory
source: docs/memory/2026-11-21/221-s-session-summary.md
ingested_at: "2026-11-22T00:22:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  shipment_id: 221-S
  feature_id: 220-F
  branch: feat/221-s
---

## Durable Summary

* **Shipment / Feature**: `221-S` / `220-F` on branch `feat/221-s` (from `main` `329e2577e98a6dd2ae751c76f1372f726d15d01e`)
* **Routing Degradation Notice (`P-013.5`)**: `ROUTING_DEGRADED: context_tier` recorded at Ship session start; non-blocking, proceeded at default context.
* **Index Pack Degradation (`[PACK-ROUTING]`)**: `ENGRAM_DEGRADED` (daemon readiness timeout); used direct git/file discovery fallback.

## Decisions and Outcomes

* **Windows Separator Normalization (`220.001-T`, Unit U1)**: Delegated `(*Server).validateSessionPath` in `internal/mcp/session_mux.go` to OS-parameterized `(*Server).validateSessionPathForOS(path, goos string)` normalizing both `'/'` and `'\\'` before `isLocalSessionPathForOS`, `..` traversal rejection, and per-component `root.Lstat` + `core.IsSymlinkOrReparsePoint` checks. Cross-OS locality rejects empty `goos`, `""`, `\x00`, leading `'/'` or `'\\'`, lexical `..`, any `:` (drive prefixes and NTFS ADS), trailing `'.'` / `' '` segments, and Windows reserved device stems (`CON`, `PRN`, `AUX`, `NUL`, `CONIN$`, `CONOUT$`, `COM1`–`COM9`/`COM¹`–`COM³`, `LPT1`–`LPT9`/`LPT¹`–`LPT³`) when `goos == "windows"`, guarded by `TestU1_ValidateSessionPathMixedAndForwardSeparators` in `internal/mcp/session_mux_test.go`.
* **Toolchain `tools_dir` Documentation (`220.002-T`, Unit U2)**: Updated `docs/configuration.md` and `docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md` to document optional `tools_dir` in `config.yaml` and `toolchain.yaml`, the 5-source discovery precedence (`.mcp.json` -> `toolchain.yaml` -> `config.yaml` -> `.bin` -> `bin` fallback, distinguishing `--tools-dir` selection from MCP daemon executable lookup), `VerifyProposedLauncherScripts`, and `backlogit init` / `install [--tools-dir <dir>]` provisioning (`--jsonrpc` vs `backlogit_get_metadata_catalog`), leaving `docs/cli-reference/*.md` untouched.

## P-021 C2 Deferred Scope Expansions

* `DCE8FD71`: Reject leading/trailing whitespace in `(*Server).validateSessionPathForOS` (`internal/mcp/session_mux.go`) instead of applying `strings.TrimSpace(path)` (`P-021 C1` out-of-scope pre-existing trim differential).
* `272A805F`: Enforce `validateSessionPath` or reject non-notification request methods carrying missing/`null` `envelope.ID` before `s.mcp.HandleMessage` in `(*Server).dispatchSessionFrame` (`internal/mcp/session_mux.go:207`).
* `5C70ECD0`: Fail closed with `corerrors.ErrValidation` in `(*Server).validateSessionPathForOS` when `strings.TrimSpace(s.RootPath)` is empty instead of letting `filepath.Abs("")` resolve to CWD.
* `8827BED4`: Update `u6BuildBinary` in `internal/cli/mcp_session_shim_test.go:404` on Windows to invoke `go build` directly instead of requiring gitignored `bin/tmp/212-go-env.ps1`.
