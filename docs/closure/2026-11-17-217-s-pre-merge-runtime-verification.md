---
title: "217-S pre-merge runtime verification"
description: "Pre-merge runtime verification evidence for Shipment 217-S (Feature 216-F / CAP-3)."
doc_type: closure
source: docs/closure/2026-11-17-217-s-pre-merge-runtime-verification.md
ingested_at: "2026-11-17T22:55:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  date: "2026-11-17"
  mode: pre-merge
  shipment_id: 217-S
  feature_id: 216-F
  evidence_head: cc59f7b6
  verification_verdict: PASS
---

## Scope and validator contract

Shipment `217-S` (`216-F`: Zero-Flake Concurrency and Fast Ship-Closure Engine,
CAP-3) delivers Units U1, U2, U3a, U3b, U4, U5, U6, and U7 across `216.001-T`..`216.008-T`:

* `internal/core/commits.go` (`AssociateCommit`, `LinkCommit`, `GetCommitLinks` `lifecycle-global -> C -> B` lock ordering)
* `internal/events/reader.go` (`ReadAllEvents` `64 KiB` initial / `1 MiB` max scanner buffer and error wrapping)
* `internal/core/artifacts.go` & `internal/core/shipment_lifecycle.go` (`findArtifactWithPath` two-pass contained direct lookup)
* `internal/core/workspace.go` & `internal/core/shipment_gate.go` (`isAncestor` `--no-replace-objects`, symlink/junction root resolution, and complete/ungrafted-repo bounded cache)
* `internal/db/connection.go` & `internal/db/schema.go` (`busy_timeout(30000)` first, persistent Page 1 WAL `[2, 2]` header initialization with concurrent retry, and `EnsureSchema` readiness verification)
* `internal/core/custody_exec.go` (bounded `custodyGit`, `custodyGitEnv()`, OID validation, and `custodyBlobDigest` deduplication)
* `internal/core/archive.go` and hardened `-race` lock-order / fixture suites

Loaded `.autoharness/workspace-profile.yaml` (`runtime_validation`): `validator_manifest.surfaces: []`, `validation_expectations.required: false`, `releasability.required: true` (`status_when_satisfied: READY`). Using the `cli` command adapter against freshly built `.\bin\backlogit.exe`.

## Environment prechecks

* Built `.\bin\backlogit.exe` via `go build -o bin\backlogit.exe ./cmd/backlogit` (`exit 0`, code `HEAD: cc59f7b6`).
* Verified `gofmt -l internal cmd tests` (clean), `go vet ./...` (`exit 0`), and `.\bin\golangci-lint.exe run --timeout=5m` (`exit 0`).

## Structured validator evidence

### CLI runtime probes (`.\bin\backlogit.exe`)

| Probe | Expected | Observed exit | Observed output |
| --- | --- | --- | --- |
| `.\bin\backlogit.exe init --cwd <temp_repo>` + `add --type feature/task` | Initializes workspace, creates DB with persistent Page 1 WAL header (`[2, 2]`), creates items | `0` | `Created feature: 001-F`, `Created task: 001.001-T`, `DB_BYTES=221184 HDR_18_19=2,2` |
| `.\bin\backlogit.exe update 001.001-T --cwd <temp_repo> --commit <sha>` | Associates commit via `AssociateCommit` under `lifecycle-global -> C -> B` | `0` | `Updated 001.001-T` |
| `.\bin\backlogit.exe sync --cwd <temp_repo>` | Rehydrates SQLite cache cleanly in WAL mode | `0` | `Indexed 2 artifacts`, `CLI_RUNTIME_PROBE_OK` |
| `.\bin\backlogit.exe docs lint --format json` | Zero docline schema violations | `0` | `{"valid": true, "violation_count": 0, "findings": []}` |

### Automated Go test verification (`HEAD: cc59f7b6`)

* `go test -timeout=30m -p=1 ./...`: `PASS` across all 50/50 Go packages (`exit 0`).
* Targeted `-race` suites (`internal/db`, `internal/events`, `internal/core`): `PASS` across `TestU1_`..`TestU7_`, `TestU172_LockOrder_`, `TestArchiveItemGovernance_`, `TestReconcile*`, `TestUR1B*`, `TestCustody*`, `TestExternal*`, `TestOpen*`, `TestEnsureSchema*`, and `TestU175_*_LintFix`.

## Verdict

`PASS`: all CLI probes, WAL Page 1 header verification, linters, and all 50 Go packages passed on `HEAD` `cc59f7b6` with zero blocked prerequisites.
