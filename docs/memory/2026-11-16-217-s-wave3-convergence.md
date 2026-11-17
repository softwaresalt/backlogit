---
title: "217-S Wave 3 convergence checkpoint"
description: "Wave 3 completion and convergence gate checkpoint for Shipment 217-S (216.004-T)."
doc_type: memory
source: docs/memory/2026-11-16-217-s-wave3-convergence.md
ingested_at: "2026-11-17T03:55:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  shipment_id: 217-S
  feature_id: 216-F
  branch: feat/217-s-zero-flake-concurrency-and-fast-ship-closure-engine
---

## Wave 3 Summary

Completed Wave 3 task `216.004-T` (Unit U3b, commit `f79c477387914d5268bdec6d61391d146df87d03`):

* Enforced `--no-replace-objects` and `ctx.Err()` pre-checks in `(*Workspace).isAncestor` (`internal/core/shipment_gate.go`), preventing `refs/replace/*` from altering commit ancestry checks.
* Added a bounded 4,096-entry mutex-protected `Workspace` ancestry cache (`internal/core/workspace.go`, `internal/core/shipment_gate.go`) keyed by lowercase 40/64-hex `(ancestorSHA, descendantSHA)` pairs, restricted strictly to complete, ungrafted repositories (bypassing and invalidating the cache whenever `.git/shallow` or `.git/info/grafts` exists or cannot be confirmed absent).

## Wave 3 Convergence Gate

* `go test -run=^$ -count=1 ./...`: PASS
* `go vet ./...`: PASS
* `bin/golangci-lint run --timeout=5m`: PASS
* `gofmt -l internal cmd tests`: clean
* `go test -timeout=30m -p=1 ./...` (`GOTOOLCHAIN=go1.27.0`): PASS across all 50/50 packages (`open_red_deliverables_3 = []`).

## Next

Admit Wave 4 (`216.006-T` / Unit U5).
