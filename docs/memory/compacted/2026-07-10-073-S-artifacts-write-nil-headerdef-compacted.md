---
chunk_strategy: h1-h2-h3
description: Compacted Stage and Ship memory for shipment 073-S create/update nil HeaderDef fail-closed hardening.
doc_type: memory
docline:
    ms.date: 2026-07-10T00:00:00Z
    ms.topic: reference
schema_version: "1.0"
source: docs/memory/compacted/2026-07-10-073-S-artifacts-write-nil-headerdef-compacted.md
title: Compacted memory - 073-S artifacts write-path nil HeaderDef hardening
---
## Summary

Shipment `073-S` closed the create/update write-path nil-`HeaderDef` fail-open family. Stage promoted stash `266816CE` into `073-F` and `073.001-T`; Ship implemented a shared `requireHeaderDef` guard in core write paths and completed merge plus post-merge archival.

## Archived originals

* `docs/archive/memory/2026-07-02-stage-073-S-artifacts-nil-headerdef.md`
* `docs/archive/memory/2026-07-02-ship-073-S-checkpoint.md`
* `docs/archive/memory/2026-07-02-ship-073-S-post-merge-closure-session.md`

## Decisions and outcomes

* Nil `HeaderDef` on mutation writes is a hard fail-closed `blerrors.ErrConfig`, not `ErrValidation`, because the user cannot correct a missing workspace schema.
* `requireHeaderDef(ws)` must run before `ApplyFieldDefaults` and `ValidateArtifactFields`; otherwise `ResolveFieldSchema` can nil-deref before validation.
* The two write paths share one helper and one task because CLI and MCP both route through `core.CreateArtifact` and `core.UpdateArtifact`.
* PR #160 merged by true merge commit `00b9b1de4fa29b3776788df280fc8f75a648d04c`; post-merge `shipment ship 073-S` archived `073.001-T`, `073-F`, and `073-S` with clean reconcile.

## Files and verification

* `internal/core/artifacts.go` added `requireHeaderDef` and rewired create/update before defaults and validation.
* `internal/core/artifacts_headerdef_test.go` added red-to-green create/update fail-closed tests plus loaded-regression coverage.
* Full quality gates and code review passed; runtime verification built the CLI and checked loaded-path add/update still succeed.
* Knowledge graduation updated `exported-cache-zero-value-bypass-2026-06-29.md` as the third and final nil-precondition fail-open recurrence.
