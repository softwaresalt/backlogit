---
title: "217-S Wave 1 convergence checkpoint"
description: "Wave 1 completion and convergence gate checkpoint for Shipment 217-S (216.001-T, 216.002-T, 216.005-T)."
doc_type: memory
source: docs/memory/2026-11-16-217-s-wave1-convergence.md
ingested_at: "2026-11-16T20:55:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  shipment_id: 217-S
  feature_id: 216-F
  branch: feat/217-s-zero-flake-concurrency-and-fast-ship-closure-engine
---

## Wave 1 Summary

Completed all three Wave 1 tasks of `217-S` (`216-F` / CAP-3):

* `216.001-T` (U1, commit `63c4303d97966255fd2e64ad25b7f09314af3f0e`): Enforced canonical `lifecycle-global -> C (item-log) -> B (artifact mutation)` lock ordering across `LinkCommit` and `AssociateCommit` in `internal/core/commits.go`, with forced-interleaving shipment-member regression tests in `internal/core/reconstitution_review_regression_test.go`.
* `216.002-T` (U2, commit `8b553667caf263f7c0ad358a731eb2c57b3b0eed`): Changed `events.ReadAllEvents` scanner buffer in `internal/events/reader.go` from eager `1 MiB` to `64 KiB` initial / `1 MiB` max, verified in `internal/events/reader_test.go`.
* `216.005-T` (U4, commit `bc4648fe666e680ac1d5a5f48cf2bdce04d9f317`): Ordered `_pragma=busy_timeout(30000)` first in `db.Open` (`internal/db/connection.go`), materialized persistent Page 1 WAL headers (`hdr[18]==2, hdr[19]==2`) on empty `.db` files, and verified WAL/schema readiness in `EnsureSchema` / `EnsureSchemaWithExtensions` (`internal/db/schema.go`) with concurrent pooled-connection tests in `internal/db/connection_test.go`.

## Wave 1 Convergence Gate

* `go test -run=^$ -count=1 ./...`: PASS
* `go vet ./...`: PASS
* `bin/golangci-lint run --timeout=5m`: PASS
* `gofmt -l internal cmd tests`: clean
* `go test -timeout=30m -p=1 ./...` (with `GOTOOLCHAIN=go1.27.0` matching `golangci-lint v2.13.2`): PASS across all 50/50 packages (`open_red_deliverables_1 = []`).

## Next

Admit Wave 2 (`216.003-T` / Unit U3a).
