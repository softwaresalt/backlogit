---
title: "217-S Wave 6 convergence checkpoint"
description: "Wave 6 completion and convergence gate checkpoint for Shipment 217-S (216.008-T)."
doc_type: memory
source: docs/memory/2026-11-17-217-s-wave6-convergence.md
ingested_at: "2026-11-17T17:55:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  shipment_id: 217-S
  feature_id: 216-F
  branch: feat/217-s-zero-flake-concurrency-and-fast-ship-closure-engine
---

## Wave 6 Summary

Completed Wave 6 task `216.008-T` (Unit U7, commit `ace6af40ae20b05a4fbc20917a248507e3abd7bc`):

* Emitted deterministic `"attempt_membership"` and `"attempt_item_log"` hook stages on `shipmentLifecycleGlobalLockHookContextKey{}` in `lockArchiveGovernance` (`internal/core/archive.go`).
* Synchronized `TestU172_LockOrder_ArchiveItemDoesNotHoldArtifactLockWhileWaitingForItemLog` (`internal/core/artifact_mutation_lock_order_test.go`) on `"attempt_item_log"` with a `50ms` negative probe and `30s` `u172ObservationWait` writer budget.
* Synchronized `TestArchiveItemGovernance_WaitsForGlobalLifecycleLock` and `TestArchiveItemGovernance_WaitsForShipmentMembershipLock` (`internal/core/shipment_archive_governance_test.go`) on `"contended"` before unlock with `30s` post-unlock completion budgets.
* Kept the unnamed `df4a2df9` `internal/core` panic (`63909DAE` / `101-DL`) explicitly unresolved as a tracked diagnostic risk (unproven origin).

## Wave 6 Convergence Gate and `-race` Verification

* `go test -run=^$ -count=1 ./...`: PASS
* `go vet ./...`: PASS
* `bin/golangci-lint run --timeout=5m`: PASS
* `gofmt -l internal cmd tests`: clean
* `go build -o bin/backlogit.exe ./cmd/backlogit`: PASS
* `go test -timeout=30m -p=1 ./...` (`GOTOOLCHAIN=go1.27.0`): PASS across all 50/50 packages (`internal/core` reduced from `1374.912s` in Wave 1 to `829.363s`; `open_red_deliverables_6 = []`).
* `go test -timeout=30m -count=1 -race -p=1 ./internal/db ./internal/events ./internal/core`: PASS (`internal/db 97.393s`, `internal/events 32.921s`, `internal/core 1140.957s`).
