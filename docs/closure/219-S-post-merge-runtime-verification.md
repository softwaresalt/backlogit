---
title: "219-S post-merge runtime verification"
description: "Post-merge runtime verification evidence on synchronized main for Shipment 219-S (Feature 218-F)."
doc_type: closure
source: docs/closure/219-S-post-merge-runtime-verification.md
ingested_at: "2026-11-20T19:52:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  date: "2026-11-20"
  mode: post-merge
  shipment_id: 219-S
  feature_id: 218-F
  merge_commit: 7229def0c1308955db257e4eec69d842b93f0256
  verification_verdict: PASS
---

## Scope and validator contract

Post-merge runtime verification for Shipment `219-S` (`218-F`, merged at `7229def0c1308955db257e4eec69d842b93f0256`, PR `#665`) executed against the compiled `logs/backlogit-probe.exe` binary (`go build -o logs/backlogit-probe.exe ./cmd/backlogit`) on synchronized `main` using the `cli` adapter per `.gsd/workspace-profile.yaml` (`validator_manifest.surfaces: []`, `validation_expectations.required: false`, `releasability.required: true`). All probe workspaces and binaries under `logs/` were removed after verification, leaving host `.bin/backlogit.exe` untouched.

## Environment prechecks

* Synchronized local `main == origin/main == 7229def0c1308955db257e4eec69d842b93f0256` (`POST_MERGE_SYNC_OK`).
* Built `logs/backlogit-probe.exe` via `go build -o logs/backlogit-probe.exe ./cmd/backlogit` (`exit 0`).
* Host `.bin/backlogit.exe` remains untouched; isolated temporary workspaces under `logs/rv-probe1`, `logs/rv-probe2`, and `logs/rv-probe3` exercised `init`, piped stdin prompt, `install` reconciliation, `.mcp.json` / `.gitignore` updates, toolchain audit, and MCP daemon lifecycle.

## Structured validator evidence

| Probe | Expected | Observed exit | Observed output |
| --- | --- | --- | --- |
| Probe 1 (`init --cwd logs/rv-probe1 --tools-dir .custom/bin`) | Creates `.custom/bin/backlogit.exe`, writes canonical `.mcp.json`, records `/.custom/bin/backlogit.exe` in `.gitignore` | `0` | `bin_exists: True`, `mcp_cmd: ./.custom/bin/backlogit.exe`, `gi_has_entry: True` |
| Probe 2 (`init --cwd logs/rv-probe2` with piped stdin `"bin\n"`) | Prompts for target tool directory and installs into `bin/backlogit.exe` with `.mcp.json` pointing to `./bin/backlogit.exe` | `0` | `prompted: True`, `bin_exists: True`, `mcp_cmd: ./bin/backlogit.exe` |
| Probe 3a (negative fail-closed preflight on launcher missing `COPILOT_HOME`/`ENGRAM_DATA_DIR`) | Rejects before creating `.tools/bin` or mutating `toolchain.yaml` | `1` | `launcher_not_fail_closed`; `workspace_root_after: .`, `tools_dir_after: bin` |
| Probe 3b (`install --cwd logs/rv-probe3 --tools-dir .tools/bin` with valid `start.ps1`/`start.sh` and `toolchain.yaml` `workspace_root: "."`, followed by `mcp status --toolchain --strict`, `mcp prewarm`, `mcp status`) | Reconciles `toolchain.yaml` preserving `workspace_root: "."`, updates `tools_dir: .tools/bin`, passes strict toolchain audit, and starts MCP daemon | `0` (install & status) | `workspace_root_after: .`, `tools_dir_after: .tools/bin`, `valid: True, violations: 0`, `running: true (pid 26672)` |

## Verdict

`PASS`: all post-merge CLI probes passed on synchronized `main` (`7229def0c1308955db257e4eec69d842b93f0256`) with zero blocked prerequisites.
