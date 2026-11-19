---
title: "218-S pre-merge runtime verification"
description: "Pre-merge runtime verification evidence for Shipment 218-S (Feature 217-F, Units U1-U4)."
doc_type: closure
source: docs/closure/2026-11-19-218-s-pre-merge-runtime-verification.md
ingested_at: "2026-11-19T14:15:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  date: "2026-11-19"
  mode: pre-merge
  shipment_id: 218-S
  feature_id: 217-F
  evidence_head: c8523d6e
  verification_verdict: PASS
---

## Scope and validator contract

Shipment `218-S` (`217-F`: Honor `.mcp.json`, configurable `tools_dir`, and `.bin` default in MCP daemon startup and toolchain verification) delivers Units U1, U2, U3, and U4 across `217.001-T`..`217.004-T`:

* `internal/toolchain/types.go` & `internal/toolchain/verify.go` (`ToolchainManifest.ToolsDir`, `isPinnedBacklogitRelPath`, `validManifestToolsDir`, `validBacklogitToolInManifest`, `loadToolchainManifest`, `verifyWorkspaceToolchain`, `validateMCPConfigPinning`)
* `schemas/workspace-toolchain.schema.json` & `internal/toolchain/launcher.go` (`effectiveManifestToolsDir`, `backlogitRemediationPath`, `validateManifestForProvisioning`, `toolRemediation`, `launcherFinding`, `inspectLauncher`)
* `internal/config/schema.go`, `internal/config/loader.go` & `internal/cli/mcp_daemon.go` (`WorkspaceConfig.ToolsDir`, `pinnedMCPDaemonBinary`, `openLocalFileInRoot`, `verifyPinnedDaemonBinaryRel`, `isReservedDaemonDirSegment`)
* `internal/core/id_history.go` (`scanLocalBranchArtifactIDsContext`, `scanGitTreeBatch`, `readGitCatFileBatch`, `parseGitTreePayload`)

Loaded `.gsd/workspace-profile.yaml` (`runtime_validation`): `validator_manifest.surfaces: []`, `validation_expectations.required: false`, `releasability.required: true` (`status_when_satisfied: READY`). Using the `cli` command adapter against freshly built `.\backlogit.exe`.

## Environment prechecks

* Built `.\backlogit.exe` via `go build ./cmd/backlogit` (`exit 0`, code `HEAD: c8523d6e`).
* Checked `.bin\backlogit.exe`: locked by active MCP server processes (`PID 7816`, `PID 25480`); runtime probes exercised the freshly built `.\backlogit.exe` copied into isolated `.bin\backlogit.exe` and `custom-bin\backlogit.exe` probe workspaces.
* Verified `gofmt -l cmd internal scripts tests` (clean), `go vet ./...` (`exit 0`), and `GOTOOLCHAIN=go1.26.5 golangci-lint run --timeout=5m` (`exit 0`).

## Structured validator evidence

### CLI runtime probes (`.\backlogit.exe`)

| Probe | Expected | Observed exit | Observed output |
| --- | --- | --- | --- |
| Probe 1: isolated workspace with `.bin\backlogit.exe` + `.mcp.json` (`./.bin/backlogit.exe`) -> `mcp prewarm`, `mcp status`, `mcp stop` | Daemon starts from `.bin\backlogit.exe`, attaches cleanly, and stops cleanly | `0` | `running: true, pid: 28328, protocol_version: backlogit-mcp-ipc/v1`, then `stopped: true` |
| Probe 2: isolated workspace with `tools_dir: custom-bin` in `.backlog\toolchain.yaml`, `./custom-bin\backlogit.exe` in `.mcp.json`, and `start.ps1`/`start.sh` -> `mcp status --toolchain --strict`, `mcp prewarm`, `mcp stop` | Toolchain audit passes with zero violations and daemon starts from `custom-bin\backlogit.exe` | `0` | `valid: true, violations: 0`, `running: true, pid: 34400`, then `stopped: true` |
| Probe 3: isolated git repo with 120 local branches `refs/heads/hist-001`..`hist-120` -> `add --type feature --title "Runtime Probe"` | ID allocation completes within 2s without timing out | `0` | `Created feature: 001-F` (`431ms`) |

### Automated Go test verification (`HEAD: c8523d6e`)

* `GOTOOLCHAIN=go1.26.5 go test -timeout=30m -p 1 ./...`: `PASS` across all packages (`exit 0`).
* Targeted `-race` suites (`internal/toolchain`, `internal/config`, `internal/cli`, `internal/core`, `tests/integration`): `PASS` across `TestU1_ToolchainVerifyAcceptsDotBinAndConfigurableToolsDir`, `TestU2_LauncherAndProvisioningSupportDotBinAndConfigurableToolsDir`, `TestU3_PinnedMCPDaemonBinaryHonorsMCPJsonToolsDirAndDotBinDefault`, `TestU4_ScanLocalBranchArtifactIDsBatchesTreesAndPreservesMergedRevertedReservations`, `TestWorkspaceLiteralGuard_NoHardcodedLegacyStorageRoot`, `TestU5_*`, `TestU6_*`, `TestU11_*`, and `TestU12_*`.

## Verdict

`PASS`: all 3 CLI runtime probes, linters, static checks, and the full Go test suite passed on code `HEAD` `c8523d6e` with zero blocked prerequisites.
