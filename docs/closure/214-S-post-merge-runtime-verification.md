---
title: "214-S post-merge runtime verification"
doc_type: closure
source: docs/closure/214-S-post-merge-runtime-verification.md
ingested_at: "2026-11-15T00:50:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
shipment_id: 214-S
feature_id: 213-F
merge_commit: f9460bac1c2c3c7d0e2098aa88d3f6095e9779d3
verification_verdict: PASS
---

## Contract and scope

Invoked `runtime-verification` with `surface: auto`, `mode: auto`, and
`validator_manifest` / `validation_expectations` from
`.autoharness/workspace-profile.yaml` after PR #649 merged to `main` at
`2026-11-15T00:24:59Z` (`f9460bac1c2c3c7d0e2098aa88d3f6095e9779d3`).

The profile declares `validator_manifest.surfaces: []` and
`validation_expectations.required: false` (`minimum_verdict: PASS`). Because
`214-S` (`213-F`) changes the CLI (`backlogit mcp status --toolchain [--strict]`)
and `internal/toolchain/launcher.go` runtime surfaces, post-merge verification
exercised the CLI command adapter in an isolated `--cwd <tempDir>` fixture plus
the post-merge U11a, U11, and U12 Go verification suites on the synchronized
`main` tree.

## Environment prechecks

* Verified synchronized `main == origin/main` at
  `f9460bac1c2c3c7d0e2098aa88d3f6095e9779d3` and created
  `post-merge/214-s-closure`.
* Observed Plan Invariant `I8`: never invoked `backlogit mcp stop` or
  `backlogit mcp --replace` against `C:\Source\GitHub\backlogit`. The live
  Engram daemon (`PID 51020`, `overall: green`) remained untouched.
* Isolated CLI probe targeted a fresh temporary directory under
  `bin/tmp/rv-214s-unpinned` and removed it in a `finally` block.

## Structured validator evidence

### Post-merge Go verification probes on merged main

| Command | Scope | Result | Elapsed |
| --- | --- | --- | --- |
| `go test -timeout=30m -count=1 -run '^(TestU11_\|TestU11a_)' ./internal/cli ./internal/toolchain` | U11a declaration shape and U11 launcher/CLI auditor tests | `PASS` (exit 0) | `4.853s` (`internal/cli`), `0.421s` (`internal/toolchain`) |
| `go test -timeout=30m -count=1 -run '^TestU12_' ./tests/integration` | U12 manifest/config provisioning, multi-workspace isolation, negative pinning/traversal tests | `PASS` (exit 0) | `3.714s` |

### Isolated CLI command probe on merged main

* Command: `go run ./cmd/backlogit --cwd <tempDir> mcp status --toolchain --strict --format json`
* Expected: exit `1` with structured JSON output (`valid: false`,
  `violations: 12`, `containment_level: "level-1-workspace-local-pinned"`,
  `os_container_sandbox_deferred: true`) followed by wrapped `ErrValidation`
  error on stderr.
* Observed: exit `1`, JSON report reported `valid: false`, `violations: 12`,
  `containment_level: "level-1-workspace-local-pinned"`,
  `os_container_sandbox_deferred: true`, stderr
  `Error: MCP toolchain audit found 12 violations: backlogit: validation error`.
* Cleanup: `bin/tmp/rv-214s-unpinned` removed cleanly.

### CI and pre-merge baseline

* Merged commit `f9460bac1c2c3c7d0e2098aa88d3f6095e9779d3` `Line Endings`
  workflow run `43658054879` (`2026-11-15T00:25:01Z`) completed `success`.
* Pre-merge PR #649 final head `3c2c1c19d3c4f747c352d7f56ee9b6d58c4d1bf8` passed
  all 8/8 CI checks and P-018 Copilot-review gate (`SATISFIED`), with isolated
  positive and negative CLI runtime probes passing in
  [pre-merge runtime verification](2026-11-14-214-s-pre-merge-runtime-verification.md).

## Risky action and manual checkpoints

* No manual operator checkpoint was blocked or pending for the CLI runtime
  surface.
* Isolated CLI probe (`ActionRisk: low`) executed cleanly against a temporary
  directory under Plan Invariant `I8`.

## Verdict and releasability handoff

`PASS`: post-merge U11a, U11, and U12 suites, merged-tip CI `Line Endings`, and
the isolated `--cwd <tempDir>` `mcp status --toolchain --strict` probe all
passed on `f9460bac1c2c3c7d0e2098aa88d3f6095e9779d3` with zero blocked
prerequisites.
