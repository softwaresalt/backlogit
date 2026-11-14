---
title: "214-S pre-merge runtime verification"
doc_type: closure
source: docs/closure/2026-11-14-214-s-pre-merge-runtime-verification.md
ingested_at: "2026-11-14T08:15:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  date: "2026-11-14"
  mode: pre-merge
  shipment_id: 214-S
  feature_id: 213-F
  evidence_head: f72525f4c6f6715652b966c97353bdb48735b7d4
  verification_verdict: PASS
---

## Scope and validator contract

Shipment `214-S` (`213-F`, Track C of `082-DL`) delivers:

* `internal/toolchain/launcher.go` (`VerifyLauncherScripts`, `AuditRepositoryToolchain`, `WriteToolchainManifest`, `RenderPinnedMCPConfig`)
* `internal/cli/root.go` (`backlogit mcp status --toolchain [--strict]`)
* `.backlogit/toolchain.yaml` (canonical tracked Level 1 workspace toolchain manifest)
* `tests/integration/multi_session_workspace_containment_test.go` (`TestU12_*`) and `internal/cli/mcp_toolchain_test.go` (`TestU11_*`)
* `docs/cli-reference/backlogit_mcp_status.md` and `docs/configuration.md` (`213.003-T` / `U13`)

Loaded `.autoharness/workspace-profile.yaml` (`runtime_validation` contract):

```yaml
validator_manifest:
  surfaces: []
validation_expectations:
  required: false
  surfaces_expected: []
  minimum_verdict: PASS
  preserve_invariants: []
  release_blockers: []
releasability:
  required: true
  status_when_satisfied: READY
  required_evidence:
    - kind: monitoring-plan
      required: false
    - kind: rollback-procedure
      required: false
    - kind: owner
      required: false
```

`surface=auto` infers the `cli` surface (`backlogit mcp status --toolchain [--strict]`). Because `validator_manifest.surfaces` is empty and `validation_expectations.required` is `false`, verification uses the CLI command adapter in isolated temporary workspaces (`--cwd <tempDir>` per Plan Invariant `I8`) plus the Go race-enabled unit/integration test suites.

## Environment prechecks

* Built `backlogit.exe` at evidence HEAD `f72525f4c6f6715652b966c97353bdb48735b7d4` via `go build ./cmd/backlogit` (`exit 0`).
* Verified workspace Engram daemon stopped (`Get-CimInstance Win32_Process`) with zero `daemon --workspace C:\Source\GitHub\backlogit` process running.
* Strictly observed Plan Invariant `I8`: never invoked `backlogit mcp stop` or `backlogit mcp --replace` against `C:\Source\GitHub\backlogit`; all CLI runtime probes targeted isolated temporary directories under `bin/tmp/` that were removed immediately after verification.

## Structured validator evidence

### Isolated CLI runtime probes (`--cwd <tempDir>`)

| Probe | Workspace fixture | Observed exit | Observed JSON output |
| --- | --- | --- | --- |
| `backlogit --cwd <pinned> mcp status --toolchain --format json` | Pinned `.backlog/toolchain.yaml`, `./bin/backlogit.exe` SHA-256, `.mcp.json`, `start.ps1`, `start.sh` | `0` | `toolchain.valid: true`, `toolchain.violations: 0`, `containment_level: "level-1-workspace-local-pinned"`, `os_container_sandbox_deferred: true` |
| `backlogit --cwd <pinned> mcp status --toolchain --strict --format json` | Same pinned isolated fixture | `0` | `toolchain.valid: true`, `toolchain.violations: 0` |
| `backlogit --cwd <unpinned> mcp status --toolchain --format json` | Unpinned fixture (missing manifest, `.mcp.json`, launchers) | `0` | `toolchain.valid: false`, `toolchain.violations: 12`, `containment_level: "level-1-workspace-local-pinned"` |
| `backlogit --cwd <unpinned> mcp status --toolchain --strict --format json` | Same unpinned isolated fixture | `1` | JSON report written (`toolchain.valid: false`, `violations: 12`); stderr `MCP toolchain audit found 12 violations: backlogit: validation error` |

### Automated Go verification at evidence HEAD

* `go test -timeout=30m -count=1 -run '^TestU11a_' ./internal/toolchain` — `PASS` (`0.906s`)
* `go test -timeout=30m -count=1 -race -run '^(TestU11_|TestU11a_)' ./internal/cli ./internal/toolchain` — `PASS` (`4.613s` / `3.070s`)
* `go test -timeout=30m -count=1 -race -run '^TestU12_' ./tests/integration` — `PASS` (`4.647s`)
* Unfiltered `go test -timeout=30m ./...` (`GOTOOLCHAIN=go1.27.0`, `GOFLAGS="-count=1 -p=1 -parallel=2"`) — `PASS` across all 50 packages (`exit 0`, including `internal/core` `939.284s`, `tests` `426.433s`, `tests/contract` `40.892s`, `tests/integration` `412.132s`).

## Invariants and manual checkpoints

* Preserve `containment_level: "level-1-workspace-local-pinned"` and `os_container_sandbox_deferred: true` (Level 1 static workspace-local pinning never claims OS/container sandboxing).
* Preserve fail-closed `--strict` validation (`exit != 0` after emitting the full report) and refusal of ambient `PATH` fallback, `..` traversal, and symlink/reparse escapes.
* Operator-owned gitignored local runtime files (`./bin/*.exe`, `.mcp.json`, `.copilot/graphtor-mcp-shim.cjs`, `tools/mcp/node_modules/`) remain outside repository commits per `210-F` custody boundaries.

## Verdict and releasability handoff

`PASS`: all isolated CLI runtime probes (`--toolchain`, `--strict`, pinned positive, unpinned negative) and all 50 unfiltered Go test packages passed at `f72525f4c6f6715652b966c97353bdb48735b7d4` with zero blocked prerequisites.
