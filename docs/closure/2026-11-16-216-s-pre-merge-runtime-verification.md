---
title: "216-S pre-merge runtime verification"
doc_type: closure
source: docs/closure/2026-11-16-216-s-pre-merge-runtime-verification.md
ingested_at: "2026-11-16T14:45:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  date: "2026-11-16"
  mode: pre-merge
  shipment_id: 216-S
  feature_id: 215-F
  evidence_head: 4f0186cf
  verification_verdict: PASS
---

## Scope and validator contract

Shipment `216-S` (`215-F`: Self-Contained Sandbox and Multi-Session Runtime Hardening (CAP-2)) implements `215.001-T`..`215.008-T` (`U1`, `U2`, `U3a`, `U3b`, `U4`, `U5a`, `U5b`, `U6`) across:

* `internal/atomicfile` (`ReplaceTempFileInRoot` handle-verified replacement and `MoveFileExW` retry semantics)
* `internal/toolchain` (`WriteToolchainManifest` replacement and `backlogit` alias + `--cwd` validation)
* `internal/mcp` and `internal/cli/mcp_daemon.go` (`CloseCoordinator`, `Shutdown`, session-only `CloseAllSessions`, and daemon shutdown wiring)
* `internal/core/gate` (`withTempGitCeiling` for TEMP-rooted repositories)
* `internal/core` (pinned `.locks` `*os.Root` ID sequence ledger, `id_allocator.lock`, cross-branch scan, compensated `CreateArtifact`, `AdoptItem`, and `ReconstituteShipments` rollback)
* `internal/cli/self_install.go` (`init`/`install` self-installing runtime binary and handle-relative `.gitignore` wiring)

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

`surface=auto` infers the `cli` and `daemon` surfaces (`backlogit init`/`install`, `backlogit add`, `backlogit docs lint`, and MCP server shutdown/lifecycle). Because `validator_manifest.surfaces` is empty and `validation_expectations.required` is `false`, verification uses the CLI command adapter against the freshly built `.\backlogit.exe` binary plus automated package and integration suites.

## Environment prechecks

* Built `.\backlogit.exe` via `go build -o .\backlogit.exe ./cmd/backlogit` (`exit 0`, `GOTOOLCHAIN=go1.27.0`, version `1.12.1-0.20261116144151-4f0186cfcac2`).
* Verified `gofmt -l cmd internal scripts tests` (clean), `go vet ./...` (`exit 0`), `.\bin\golangci-lint.exe run --timeout=5m ./...` (`GOTOOLCHAIN=go1.26.5`, `exit 0`), and `.\backlogit.exe docs lint --format json` (`valid: true`, `violation_count: 0`).

## Structured validator evidence

### CLI runtime probes (`.\backlogit.exe`)

| Probe | Expected | Observed exit | Observed output |
| --- | --- | --- | --- |
| `.\backlogit.exe install --cwd <temp_repo>` | Initializes `.backlogit`, installs runtime binary, wires ignore rules | `0` | `Initialized backlogit workspace at ...\.backlogit`, `.backlogit\backlogit.exe` created |
| `git -C <temp_repo> check-ignore -v .backlogit/backlogit.exe` | Installed runtime executable ignored by `.gitignore` | `0` | `.gitignore:4:/.backlogit/backlogit.exe` |
| Installed binary `add --type feature` on `main`, `probe-branch-b`, `probe-branch-c` | Cross-branch ID reservation prevents reusing `002-F` on `probe-branch-c` | `0` | `001-F`, `002-F`, `003-F`; `.locks/id-sequences.json` records `["001-F","002-F","003-F"]` |
| `.\backlogit.exe docs lint --format json` | Zero docline schema violations | `0` | `{"valid": true, "violation_count": 0, "findings": []}` |

### Automated Go test verification (`HEAD: 4f0186cf`)

* `go test -count=1 -timeout=30m ./internal/core`: `PASS` (`1331.836s`, covering `TestU5_*`, `TestU5b_*`, `TestWorkspaceLiteralGuard_NoHardcodedLegacyStorageRoot`, and full `internal/core` suite).
* Remaining 49 Go packages (`cmd/...`, `internal/atomicfile`, `internal/cli`, `internal/core/gate`, `internal/mcp`, `internal/toolchain`, `tests`, `tests/contract`, `tests/integration`, `scripts`, etc.) with `go test -p 1 -count=1 -timeout=30m`: `PASS` (`exit 0`, all 50/50 Go packages green at `4f0186cf`).

## Invariants and manual checkpoints

* `CloseAllSessions` retains session-only closure semantics; `CloseCoordinator` and `Shutdown` stop both session and coordinator admission before waiting on drain.
* Non-test Go files under `internal/{core,cli,mcp,config,telemetry,db}` (excluding `internal/core/workspace.go`) contain no hardcoded `".backlogit"` literal.
* All lock acquisitions respect `stash -> lifecycle-global -> A -> C -> B -> id_allocator.lock` with no `B`/`C` acquisition inside `id_allocator.lock` or `BeforeRename`.

## Verdict and releasability handoff

`PASS`: all CLI probes, cross-branch ID reservation checks, linters, and all 50 Go packages passed on `HEAD` `4f0186cf` with zero blocked prerequisites.
