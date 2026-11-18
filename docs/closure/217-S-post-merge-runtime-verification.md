---
title: "217-S post-merge runtime verification"
description: "Post-merge runtime verification evidence for Shipment 217-S (Feature 216-F / CAP-3) on synchronized main."
doc_type: closure
source: docs/closure/217-S-post-merge-runtime-verification.md
ingested_at: "2026-11-18T03:16:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  date: "2026-11-18"
  mode: post-merge
  shipment_id: 217-S
  feature_id: 216-F
  merge_commit: 75e27861fac309e79113c270bcaba1927dfdc767
  verification_verdict: PASS
---

## Scope and validator contract

Post-merge verification on synchronized `main` (`75e27861fac309e79113c270bcaba1927dfdc767`, PR `#656`)
for Shipment `217-S` (`216-F`: Zero-Flake Concurrency and Fast Ship-Closure Engine, CAP-3).

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
```

## Environment prechecks

* Built `.\bin\backlogit.exe` directly from merged `main` (`go build -o bin\backlogit.exe ./cmd/backlogit`, `exit 0`).
* Verified `.\bin\backlogit.exe docs lint --format json` returns `{"valid": true, "violation_count": 0, "findings": []}`.

## Structured validator evidence

### Post-merge Go unit and invariant probes (`main` @ `75e27861`)

* `go test -count=1 -run "TestU2_" ./internal/events` - `PASS` (`5.275s`)
* `go test -count=1 -run "TestU4_" ./internal/db` - `PASS` (`12.311s`)
* `go test -count=1 -run "TestU1_|TestU3a_|TestU3b_|TestU5_|TestU6_|TestU7_|TestU172_LockOrder_|TestArchiveItemGovernance_" ./internal/core` - `PASS` (`76.592s`)

### Post-merge CLI runtime probes (`.\bin\backlogit.exe`)

| Probe | Expected | Observed exit | Observed output |
| --- | --- | --- | --- |
| `.\bin\backlogit.exe init --cwd <temp_repo>` | Initializes workspace and persistent SQLite WAL Page 1 header | `0` | `Initialized backlogit workspace at ...\.backlog` |
| `.\bin\backlogit.exe add --title "Probe Feature" --type feature --cwd <temp_repo>` | Allocates feature ID | `0` | `Created feature: 001-F` |
| `.\bin\backlogit.exe add --title "Probe Task" --type task --parent 001-F --cwd <temp_repo>` | Allocates child task ID | `0` | `Created task: 001.001-T` |
| `.\bin\backlogit.exe update 001.001-T --commit <sha> --cwd <temp_repo>` | Associates commit via `AssociateCommit` under `lifecycle-global -> C -> B` | `0` | `Updated 001.001-T` |
| `.\bin\backlogit.exe sync --cwd <temp_repo>` | Rehydrates SQLite cache cleanly in WAL mode | `0` | `Indexed 2 artifacts` |
| SQLite file Page 1 WAL header bytes `[18, 19]` | Persistent WAL read/write format bytes `[2, 2]` | `0` | `DB_BYTES=221184 HDR_18_19=2,2` |

## Verdict

`PASS`: all post-merge unit/invariant suites, CLI probes, WAL Page 1 `[2, 2]` checks, and `docs lint` passed on merged `main` (`75e27861fac309e79113c270bcaba1927dfdc767`) with zero blocked prerequisites.
