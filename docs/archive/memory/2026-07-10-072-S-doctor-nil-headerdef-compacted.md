---
chunk_strategy: h1-h2-h3
description: Compacted Stage and Ship memory for shipment 072-S doctor --target nil HeaderDef fail-closed hardening.
doc_type: memory
docline:
    ms.date: 2026-07-10T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/memory/compacted/2026-07-10-072-S-doctor-nil-headerdef-compacted.md
title: Compacted memory - 072-S doctor nil HeaderDef hardening
---
## Summary

Shipment `072-S` closed the `doctor --target` nil-`HeaderDef` fail-open path. Stage promoted stash `C16DBBEB` into `072-F` and task `072.001-T`; Ship implemented the shared-core fix in `ValidateDoctorTargetResolved`, opened PR #158, resolved CI and Copilot feedback, and completed post-merge archival after operator P-014 approval.

## Archived originals

* `docs/archive/memory/2026-07-01-stage-072-S-doctor-nil-headerdef.md`
* `docs/archive/memory/2026-07-01-ship-072-S-checkpoint.md`

## Decisions and outcomes

* Nil `Workspace.HeaderDef` is a system/config precondition fault, not user validation; it returns `DoctorTargetIO`, `OK=false`, and exit 3 without a new result kind or schema version.
* CLI and MCP parity is structural because both surfaces call `core.ValidateDoctorTargetResolved`.
* The plan reused the durable nil-zero-value fail-closed learning; no separate deliberation or new compound entry was warranted.
* PR #158 merged by true merge commit `d3f0facf530592c526e261b3818dc6e0d0dd0ced`; `backlogit shipment ship 072-S` archived `072.001-T`, `072-F`, and `072-S` with P-007 clean.

## Files and verification

* `internal/core/doctor_target.go` inverted the nil guard and broadened the `DoctorTargetIO` doc comment.
* `internal/core/doctor_target_test.go` added `TestDoctorTarget_NilHeaderDefFailsClosed` with loaded-vs-nil precedence coverage.
* Quality gates passed: `go test ./...`, `go vet ./...`, `golangci-lint run`, and changed-file formatting.
* Fix-CI added docline frontmatter to closure docs; Copilot requested removing a dated plan path from a code comment, then re-review was clean.
* Follow-up stash `266816CE` captured the sibling create/update write-path nil-`HeaderDef` defect, which became `073-S`.
