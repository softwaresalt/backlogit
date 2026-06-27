---
title: Ship 040-S — Harness Generation Complete
description: Failing harnesses scaffolded for Shipment 040-S and verified with go build plus scoped red commands
ms.date: 2026-04-21
---

## Shipment

* Shipment: 040-S
* Branch: ship/040-s-binary-release-telemetry-markdown
* Feature: 039-F

## Harness Files Added

* [shipment_040_release_install_harness_test.go](../../tests/integration/shipment_040_release_install_harness_test.go)
* [shipment_040_report_harness_test.go](../../internal/telemetry/shipment_040_report_harness_test.go)
* [shipment_040_telemetry_cli_harness_test.go](../../internal/cli/shipment_040_telemetry_cli_harness_test.go)

## Task to Harness Mapping

| Task | Harness command |
|---|---|
| 039.009-T | `go test ./tests/integration -run "TestTask039009_" -count=1` |
| 039.010-T | `go test ./tests/integration -run "TestTask039010_" -count=1` |
| 039.011-T | `go test ./tests/integration -run "TestTask039011_" -count=1` |
| 039.012-T | `go test ./tests/integration -run "TestTask039012_" -count=1` |
| 039.013-T | `go test ./internal/telemetry -run "TestTask039013_" -count=1` |
| 039.014-T | `go test ./internal/telemetry -run "TestTask039014_" -count=1` |
| 039.015-T | `go test ./internal/telemetry -run "TestTask039015_" -count=1` |
| 039.016-T | `go test ./internal/cli -run "TestTask039016_" -count=1` |

## Verification

* `go build ./...` passed after scaffolding
* Scoped harness commands fail in the expected red phase

## Red Phase Summary

* 039.009-T: release workflow missing commit and build date ldflags
* 039.010-T: release workflow missing SHA256SUMS generation and publication
* 039.011-T: install scripts do not exist yet
* 039.012-T: installation docs still lead with `go install`
* 039.013-T: reporter behavioral coverage intentionally left as not implemented harness
* 039.014-T: harvest pipeline coverage intentionally left as not implemented harness
* 039.015-T: markdown report output not implemented
* 039.016-T: CLI markdown reporting not implemented and flag text still omits markdown

## Next Steps

Execute build-feature loops in dependency order:

1. 039.009-T
2. 039.013-T
3. 039.014-T
4. 039.010-T
5. 039.015-T
6. 039.011-T
7. 039.016-T
8. 039.012-T
