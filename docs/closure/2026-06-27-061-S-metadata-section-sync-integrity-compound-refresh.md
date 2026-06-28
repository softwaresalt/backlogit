---
chunk_strategy: h1-h2-h3
description: 'Compound-refresh review for the 061-S Metadata and Section Sync Integrity shipment — classifies the overlapping CLI/MCP parity, SQLite-atomicity, and index-vs-filesystem entries as keep (complementary, distinct mechanisms) and records the new catalog-parity-via-DI + index-consistency + dry-run + typed-error learning'
doc_type: closure
docline:
    ms.date: 2026-06-27T00:00:00Z
    ms.topic: reference
ingested_at: "2026-06-28T02:40:00Z"
schema_version: "1.0"
source: docs/closure/2026-06-27-061-S-metadata-section-sync-integrity-compound-refresh.md
title: 061-S Metadata and Section Sync Integrity — Compound Refresh
---

# Compound Refresh — Shipment 061-S (Metadata and Section Sync Integrity)

- **Scope**: `recent` + entries overlapping the CLI/MCP parity, index-consistency, dry-run, and section-write surfaces
- **Mode**: propose (no rewrites required; one new entry authored separately)
- **Context**: Post-merge closure of 061-S (PR #145, merge `006bb854`)

## Entries reviewed

| Entry | Overlap with 061-S | Classification | Rationale |
|---|---|---|---|
| `docs/compound/2026-05-07-mcp-cli-config-parity.md` | Same *CLI/MCP must stay in parity* theme | **keep** | Complementary, **distinct mechanism**. That entry is about loading the same **config-driven option** in both handlers (a checklist rule). 061-S Rule 1 is about supplying **catalog command data** across layers via **dependency injection** to avoid an `mcp→cli` import cycle, locked by a parity *test*. The new entry cross-links it and extends the parity concern from config-loading to cross-layer data provisioning. Both remain accurate. |
| `docs/compound/database-issues/atomic-rehydration-sqlite-transaction-2026-04-08.md` | Shares the *index/DB must stay consistent* theme | **keep** | Distinct mechanism: wrapping a `DELETE`+rebuild in one SQLite transaction. 061-S Rule 2 is about **re-upserting at the write site** so the index never diverges from the file after a section edit. Different layer/trigger; no overlap in the specific learning. |
| `docs/compound/db-reliability/canonical-filesystem-scan-vs-index-id-allocation-2026-06-25.md` | Shares the *file is source of truth, index is derived* principle | **keep** | That entry applies the principle to **ID allocation** (scan the filesystem, not the index). 061-S Rule 2 applies the same principle to **section-body writes** (refresh the index from the file at write time). Same north star, different application; both kept and mutually reinforcing. |
| `docs/compound/go-patterns/f049-jsonrpc-cli-interceptor-patterns.md` | Adjacent CLI/MCP-surface patterns | **keep** | About the JSON-RPC interceptor / CLI-MCP envelope plumbing for F049. Does not touch metadata-catalog parity, section re-upsert, dry-run guards, or typed section errors. No overlap in the specific learning. |
| `docs/compound/best-practices/empty-string-vs-sentinel-in-classification-2026-05-09.md` | Adjacent *typed sentinel vs catch-all* theme | **keep** | About sentinel-vs-empty-string in classification logic. 061-S Rule 4 is about gating a **fallback action** on a typed error (`ErrSectionNotFound` vs `ErrSectionMalformed`) instead of a catch-all blanket-append. Related philosophy, different concrete bug; both kept. |

## New entry authored

- `docs/compound/2026-06-27-cli-mcp-catalog-parity-via-di-and-index-consistency.md`
  — captures four intertwined durable rules: (1) supply cross-layer parity data by
  **dependency injection** to avoid import cycles and lock it with a **parity test**;
  (2) **re-upsert into SQLite + FTS on every mutating write** so the index never diverges
  from the file; (3) place the **dry-run guard before every write branch**, including
  fallbacks; (4) surface **typed errors** instead of a corrupting blanket-append fallback.
  This is a **new** learning (`compound`), not a refresh of an existing one.

## Evidence used

- Source: `internal/mcp/metadata.go`, `internal/mcp/tools.go`, `internal/cli/update.go` +
  MCP wiring, `internal/db/merge_sync.go`, `internal/core`/`parser` section validation.
- Tests (fresh, `-count=1`, all PASS): `TestMetadataCatalog_CLIAndMCPParity`,
  `TestMetadataCatalog_IncludesCLICommandsFromProvider`, `TestHandleGetMetadataCatalog_EmitsCLICommands`,
  `TestExportCommandMap_WritesUnderWorkspaceRoot/RejectsEscapeOutsideWorkspace`,
  `TestHandleUpdateItem_SectionRewrite_ReupsertsDB`, `TestHandleGetItem_AfterSectionRewrite_ReturnsUpdatedBody`,
  `TestUpdateCommand_SectionWrite_SyncsDB`, `TestUpdateCommand_MalformedSectionMarker_ReturnsError`,
  `TestUpdateCommand_InvalidSectionName_Rejected`, `TestMergeSync_DryRunWithFallback_DoesNotWrite`,
  `TestMergeSync_DryRunDoesNotModifyDatabase`.
- Closure + runtime-verification artifacts for 061-S (this batch).

## Decision

**No stale/superseded entries.** All overlapping entries describe distinct mechanisms or
distinct applications of a shared principle and remain accurate against current source.
One new compound entry added. No consolidate / replace / delete actions taken.
