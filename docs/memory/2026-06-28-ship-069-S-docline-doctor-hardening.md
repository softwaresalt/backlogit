---
chunk_strategy: h1-h2-h3
description: Ship session memory for shipment 069-S (docline + doctor robustness hardening) — three TDD tasks complete, branch pushed, PR open, awaiting merge approval
doc_type: memory
ingested_at: "2026-06-28T19:20:00Z"
schema_version: "1.0"
source: docs/memory/2026-06-28-ship-069-S-docline-doctor-hardening.md
title: Ship 069-S — docline + doctor hardening
---

## Shipment

069-S "docline + doctor robustness hardening" — items 069-F, 069.001-T, 069.002-T, 069.003-T.
Branch: `feat/069-docline-doctor-hardening`.

## Items completed (TDD, all done)

- **069.001-T** doctor `--fix-malformed` — commit 36926448. Body-preserving clear of
  malformed `archived_from`, gated behind `--check-archived-from`. Repaired real
  038-DL/039-DL; doctor audit malformed count 2 → 0. Idempotent, self-ref untouched,
  NOT on MCP tool. New `clearMalformedArchivedFrom` + `removeArchivedFromField`.
- **069.002-T** docline ApplyMigration TOCTOU re-read — commit 48e4d042. Re-reads each
  target at apply time, aborts with `ErrConcurrentEdit` (zero writes) if on-disk bytes
  diverge from plan-time Before. 065-S L4.
- **069.003-T** ValidateFields full v1 schema — commit e6d5231f. content_sha256 hex
  pattern + minLength (non-blank). Hand-rolled, zero new deps (Principle VI). 0 false
  positives across migrated corpus. Regenerated doctor CLI reference.

## Decisions

- T3 JSON-schema lib: hand-rolled over santhosh-tekuri/jsonschema — tiny fixed schema,
  avoids new dependency; consistent with the existing 213-doc corpus (docs lint = 0).

## Gates

go test ./... green; go vet clean; golangci-lint (touched pkgs) clean; docs lint 0.
Review gate (report-only): no P0/P1/P2.

## Branch state

069-F + 069-S left queued for post-merge closure. Tasks archived on done.

## Next

PR open; await CI green + Copilot review + 0 unresolved threads. NO merge — operator merges.
