---
title: "217-S Wave 2 convergence checkpoint"
description: "Wave 2 completion and convergence gate checkpoint for Shipment 217-S (216.003-T)."
doc_type: memory
source: docs/memory/2026-11-16-217-s-wave2-convergence.md
ingested_at: "2026-11-17T00:25:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  shipment_id: 217-S
  feature_id: 216-F
  branch: feat/217-s-zero-flake-concurrency-and-fast-ship-closure-engine
---

## Wave 2 Summary

Completed Wave 2 task `216.003-T` (Unit U3a, commit `c959ed6d2070a2c110e16d2db6160649b837697f`):

* Added `findArtifactWithPath`, `isDirectProbeID`, `probeArtifactInSearchDir`, and `walkArtifactInSearchDir` in `internal/core/artifacts.go`, implementing a two-pass search across `artifactSearchDirs(ws)` (Pass 1: direct contained `{id}.md` probe with `ensureArtifactLookupContained` before `parseFile`; Pass 2: `filepath.WalkDir` fallback).
* Replaced separate `findArtifact` and `FindArtifactPath` calls in `snapshotShipArtifacts` (`internal/core/shipment_lifecycle.go`) with a single `findArtifactWithPath` call.

## Wave 2 Convergence Gate

* `go test -run=^$ -count=1 ./...`: PASS
* `go vet ./...`: PASS
* `bin/golangci-lint run --timeout=5m`: PASS
* `gofmt -l internal cmd tests`: clean
* `go test -timeout=30m -p=1 ./...` (`GOTOOLCHAIN=go1.27.0`): PASS across all 50/50 packages (`internal/core` dropped from `1374.912s` in Wave 1 to `1132.935s`; `open_red_deliverables_2 = []`).

## Next

Admit Wave 3 (`216.004-T` / Unit U3b).
