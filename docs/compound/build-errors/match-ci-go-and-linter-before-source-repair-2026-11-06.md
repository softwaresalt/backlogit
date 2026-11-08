---
title: Match CI Go and linter versions before repairing source findings
description: Reproduce the pinned Go 1.24 and golangci-lint v1 combination before classifying a host-toolchain lint failure
problem_type: build_error
category: build-errors
component: local-quality-gates
root_cause: Local Go and linter combinations differed from the authoritative CI combination
resolution_type: workaround
severity: medium
message: Cannot decode newer Go export data
file_path: .github/workflows/ci.yml
citations:
  - docs/memory/2026-11-06/212-S-confirmed-resume.md
  - docs/memory/2026-11-06/212-S-wave1-U1-harness-manifest.md
  - .github/workflows/ci.yml
tags:
  - go-1.24
  - golangci-lint
  - toolchain
  - workspace-caches
  - 212-S
---

## Problem

The host Go 1.27 toolchain and a v1.64.8 linter built with Go 1.26.5
failed to decode export data. An installed v2.13.2 linter built with
Go 1.27 then reported 22 findings. Neither combination reproduced the
repository's CI environment.

## Root cause

CI explicitly selects Go 1.24 and golangci-lint v1.64.8. Switching only
the linter changed the experiment without matching that contract.
Both linter versions enable errcheck and staticcheck by default.
The version difference alone does not establish that findings are false.

## Resolution

Select the cached workspace Go 1.24.0 toolchain through both `GOROOT`
and the leading `PATH` entry. Set `GOTOOLCHAIN=local`, read-only module
mode, disabled downloads, and workspace-contained build, module, lint,
GOPATH, and temporary caches for each fresh shell invocation.

Run the pinned workspace linter:

```powershell
& ./bin/golangci-lint.exe run --timeout=5m --out-format=json
```

The verified remedy exited 0 with zero issues and empty stderr.
Its bounded original capture is
`logs/diagnostics/212-S-ci-v1-go124-attempt1.json`.
The harness executor independently reproduced a successful result.
No source repair was needed for this lint failure.

## Prevention

* Inspect `go.mod`, CI pins, and lint configuration before source edits
* Record compiler, linter, native exit, and complete diagnostic inventory
* Treat changing versions as a distinct hypothesis, not a cosmetic retry
* Keep unsupported-version findings separate from supported gate outcomes
* Do not claim a lint remedy clears tests: the subsequent Go 1.24 full suite
  independently exposed a Windows daemon path-validation defect
