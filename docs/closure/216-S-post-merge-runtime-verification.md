---
title: "216-S post-merge runtime verification"
doc_type: closure
source: docs/closure/216-S-post-merge-runtime-verification.md
ingested_at: "2026-11-16T17:40:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  date: "2026-11-16"
  mode: post-merge
  shipment_id: 216-S
  feature_id: 215-F
  merge_commit: 04944c1ed866666ec6b22f948ba2822ebbc1612c
  verification_verdict: PASS
---

## Scope and validator contract

Post-merge verification on synchronized `main` (`04944c1ed866666ec6b22f948ba2822ebbc1612c`, PR `#654`)
for Shipment `216-S` (`215-F`: Self-Contained Sandbox and Multi-Session Runtime Hardening (CAP-2)).

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

* Built `.\backlogit.exe` directly from merged `main` (`go build -o .\backlogit.exe ./cmd/backlogit`, `exit 0`).
* Verified `.\backlogit.exe docs lint --format json` returns `{"valid": true, "violation_count": 0, "findings": []}`.

## Structured validator evidence

### Post-merge Go unit & invariant probes (`main` @ `04944c1e`)

* `go test -count=1 -run "TestU1_" ./internal/toolchain` — `PASS`
* `go test -count=1 -run "TestU2_" ./internal/toolchain` — `PASS`
* `go test -count=1 -run "TestU3a_" ./internal/mcp` — `PASS`
* `go test -count=1 -run "TestU3b_" ./internal/cli` — `PASS`
* `go test -count=1 -run "TestU4_" ./internal/core/gate` — `PASS`
* `go test -count=1 -run "TestU5_|TestU5b_|TestWorkspaceLiteralGuard" ./internal/core` — `PASS`
* `go test -count=1 -run "TestU6_|TestSelfInstall" ./internal/cli` — `PASS`

### Post-merge CLI runtime probes (`.\backlogit.exe`)

| Probe | Expected | Observed exit | Observed output |
| --- | --- | --- | --- |
| `.\backlogit.exe install --cwd <temp_repo>` | Installs runtime executable and ignore rules | `0` | `Initialized backlogit workspace at ...\.backlogit` |
| `git -C <temp_repo> check-ignore -v .backlogit/backlogit.exe` | Installed binary ignored | `0` | `.gitignore:5:/.backlogit/backlogit.exe` |
| `git -C <temp_repo> check-ignore -v .backlogit/.locks/` | `.locks/` ignored across branches | `0` | `.gitignore:6:/.backlogit/.locks/` |
| Installed binary `add --type feature` across `main` / `branch-b` / `main` | Sequential non-colliding IDs `001-F`, `002-F`, `003-F` | `0` | `001-F`, `002-F`, `003-F` |

## Verdict

`PASS`: all post-merge CLI probes, cross-branch ID reservation checks, and unit/invariant suites passed on merged `main` (`04944c1ed866666ec6b22f948ba2822ebbc1612c`).
