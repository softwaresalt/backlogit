---
title: Ship 040-S — release workflow slice complete
description: Tasks 039.009-T and 039.010-T completed for shipment 040-S
ms.date: 2026-04-21
---

## Shipment

* Shipment: 040-S
* Branch: ship/040-s-binary-release-telemetry-markdown

## Completed Tasks

* 039.009-T — Complete Release Workflow ldflags
* 039.010-T — Add SHA256 Checksum Generation

## Files Modified

* [.github/workflows/release.yml](../../.github/workflows/release.yml)

## Outcome

The release workflow now:

* injects `Version`, `Commit`, and `BuildDate` into release binaries
* derives commit metadata from `GITHUB_SHA`
* derives build timestamps from `date -u`
* generates a deterministic `SHA256SUMS` file in the release job
* publishes `SHA256SUMS` alongside the downloaded artifacts

## Verification

* 039.009-T harness passed on attempt 2
* 039.010-T harness passed on attempt 2
* `go test ./...` passed after harness gating refinement
* `go vet ./...` passed
* `golangci-lint run` passed for the release workflow slice
* workflow lint script path referenced in repo instructions is currently absent from `scripts/`

## Commits

* `1944671` — build(workflows): complete ldflags for 039.009-T
* `fab305e` — build(workflows): add checksums for 039.010-T

## Next Ready Items

* 039.011-T — Create One-Liner Install Scripts
* 039.013-T — Telemetry Reporter Behavioral Tests
* 039.014-T — End-to-End Harvest Pipeline Test
