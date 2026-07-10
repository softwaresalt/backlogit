---
chunk_strategy: h1-h2-h3
description: Compacted Stage and Ship memory for shipment 074-S doctor --target scope-vs-IO classification.
doc_type: memory
docline:
    ms.date: 2026-07-10T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/memory/compacted/2026-07-10-074-S-doctor-target-scope-io-compacted.md
title: Compacted memory - 074-S doctor target scope-vs-IO classification
---
## Summary

Shipment `074-S` resolved Copilot follow-up J from 071-S by distinguishing path-resolution IO faults from genuine containment-scope violations in `doctor --target` handling. Stage harvested `074-F` and `074.001-T`; Ship implemented the classification seam and left the PR merge-ready at P-014.

## Archived originals

* `docs/archive/memory/2026-07-02-stage-074-S-doctor-scope-io.md`
* `docs/archive/memory/2026-07-02-ship-074-S-scope-io-classification.md`

## Decisions and outcomes

* The existing `(ok, err)` contract on `confineToStorageRoot` is the discriminator: `err != nil` means IO/path-resolution fault; `ok == false` with nil error means containment scope violation.
* Exit-code neutrality was preserved: both scope and IO still map to exit 3; no new result kind or schema version was introduced.
* An unexported `confineFn` seam made the normally unreachable IO branch testable without changing `confineToStorageRoot` behavior.
* Security-sensitive symlink-escape behavior from 071-S was preserved byte-for-byte and locked by existing scope tests.

## Files and verification

* `internal/core/doctor_target.go` reclassified the resolution-error branch and preserved wrapped error text.
* `internal/core/doctor_target_test.go` added IO classification and lexical out-of-scope tests.
* `go test -race ./...`, `go vet ./...`, `golangci-lint run`, and changed-file format checks passed.
* Code review found no P0/P1 and verified seam correctness plus no containment regression.
