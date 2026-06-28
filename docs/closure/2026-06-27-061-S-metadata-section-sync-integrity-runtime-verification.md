---
chunk_strategy: h1-h2-h3
description: 'Post-merge lightweight runtime verification for shipment 061-S — CLI/MCP metadata-catalog parity via dependency injection, export-command-map workspace-root anchoring with containment, MCP section re-upsert to SQLite plus FTS, typed malformed-section errors without duplicate-marker append, and MergeSync dry-run write purity including the fallback rehydrate path, proven via fresh package tests plus the live ship of 061-S'
doc_type: closure
docline:
    ms.date: 2026-06-27T00:00:00Z
    ms.topic: reference
ingested_at: "2026-06-28T02:40:00Z"
schema_version: "1.0"
source: docs/closure/2026-06-27-061-S-metadata-section-sync-integrity-runtime-verification.md
title: 061-S Metadata and Section Sync Integrity — Post-Merge Runtime Verification
---

# Runtime Verification — Shipment 061-S (Metadata and Section Sync Integrity)

- **Surface**: CLI + MCP tool layer and the SQLite/FTS index sync library (`internal/mcp`, `internal/cli`, `internal/db`, plus `internal/core`/`parser` section validation). No long-running runtime service, web, or background-job surface.
- **Mode**: automated package tests (fresh, `-count=1`) + a live `shipment ship 061-S` archive mutation + a live CLI metadata-catalog inspection. No throwaway mutation needed — the ship is the real archive operation for this closure.
- **Context**: Ship Step 6 post-merge closure for 061-S; merge commit `006bb854afa6f56c87f4a80f5d3d6668feef0b58` (PR #145), default branch `main`.
- **Feature**: 062-F (tasks 062.001-T … 062.005-T — all done/archived). Benign `NNN-S → (NNN+1)-F` ID offset (cosmetic, per `docs/decisions/2026-06-25-shipment-manifest-drift-determination-deliberation.md`). There is no `062-S` and no `063` item; 061-S's manifest is authoritative.
- **Verdict**: **PASS**

## Invariants under test

1. **CLI/MCP metadata-catalog parity (062.001-T)** — the MCP metadata catalog carries the same CLI command data as the CLI path. Parity is supplied by dependency injection (`Server.CLICommandProvider`) so `internal/mcp` never imports `internal/cli` (no import cycle), and a parity test locks CLI ≡ MCP.
2. **export-command-map workspace-root anchoring (062.002-T)** — `export-command-map` writes relative to the workspace root (not the `.backlogit` dir), and path containment still rejects `../` escapes.
3. **MCP section re-upsert (062.003-T)** — an MCP section write re-upserts the artifact into SQLite + FTS so the index matches the file body immediately (no stale index after a section rewrite).
4. **Typed malformed-section handling (062.004-T)** — malformed or invalid section markers return typed errors (`ErrSectionMalformed` / `ErrSectionNotFound`); the CLI no longer blanket-appends a duplicate section block on generic write failures.
5. **MergeSync dry-run purity (062.005-T)** — `MergeSync` short-circuits before *every* write branch, including the fallback rehydrate path, so a dry run performs zero DB writes.

## Environment prechecks

- Binary under test: the repo-root `backlogit` binary (named `backlogit.exe` on Windows, `backlogit` on macOS/Linux), freshly built from current `main` @ `006bb854` (carries `docs` + `doctor` subcommands). This verification run was executed on Windows, hence the `.exe` form in the command transcripts below.
- Workspace: `.backlogit/` index rebuilt (638 artifacts) before and after the ship.
- Go toolchain present; targeted package tests runnable.
- No external service, port, fixture, or credential dependency — the affected surface is a local CLI/MCP/library.

## Execution & evidence

### Automated test suite (fresh, `-count=1 -v`)

| Test | Package | Invariant | Result |
|---|---|---|---|
| `TestMetadataCatalog_IncludesCLICommandsFromProvider` | mcp | 1 | **PASS** (0.04s) |
| `TestHandleGetMetadataCatalog_EmitsCLICommands` | mcp | 1 | **PASS** (0.06s) |
| `TestMetadataCatalog_CLIAndMCPParity` | cli | 1 | **PASS** (0.08s) |
| `TestExportCommandMap_WritesUnderWorkspaceRoot` | mcp | 2 | **PASS** (0.09s) |
| `TestExportCommandMap_RejectsEscapeOutsideWorkspace` | mcp | 2 | **PASS** (0.05s) |
| `TestWriteSectionsToFile_MixedExistingAndNew` | mcp | 3 | **PASS** (0.11s) |
| `TestHandleUpdateItem_SectionRewrite_ReupsertsDB` | mcp | 3 | **PASS** (0.13s) |
| `TestHandleGetItem_AfterSectionRewrite_ReturnsUpdatedBody` | mcp | 3 | **PASS** (0.13s) |
| `TestUpdateCommand_SectionWrite_SyncsDB` | cli | 3 | **PASS** (0.16s) |
| `TestUpdateCommand_MalformedSectionMarker_ReturnsError` | cli | 4 | **PASS** (0.12s) |
| `TestUpdateCommand_InvalidSectionName_Rejected` | cli | 4 | **PASS** (0.12s) |
| `TestMergeSync_DryRunWithFallback_DoesNotWrite` | db | 5 | **PASS** (0.70s) |
| `TestMergeSync_DryRunDoesNotModifyDatabase` | db | 5 | **PASS** (0.08s) |

Full package runs all green: `ok internal/mcp`, `ok internal/cli` (15.139s), `ok internal/cli/format`, `ok internal/db`.

### Live CLI metadata catalog (invariant 1, production path)

`backlogit metadata catalog` emits a populated `"cli"` array of command descriptors (`backlogit`, `backlogit add`, `backlogit adopt`, `backlogit archive`, …). The DI provider feeds this same command data into the MCP catalog; the parity test above proves the two surfaces are equal.

### Live ship of 061-S (real archive mutation)

`backlogit shipment ship 061-S --sha 006bb854… --message … --author …`:

```json
{
  "shipment_id": "061-S",
  "shipment_status": "shipped",
  "archived_ids": ["062.001-T","062.002-T","062.003-T","062.004-T","062.005-T","061-S","062-F"],
  "returned_ids": [],
  "commit_sha": "006bb854afa6f56c87f4a80f5d3d6668feef0b58"
}
```

- `returned_ids: []` — clean ship, no torn state.
- Post-ship index query: `061-S`, `062-F`, `062.001-T … 062.005-T` → all `archived`; queued- and active-shipment queues now **empty**.
- `doctor --check-archived-from` post-ship: **0 self-referential** records; only the 2 known malformed legacy records (`038-DL`, `039-DL`, value `done`, flagged-only). All 7 newly-archived records carry canonical `archived_from` = `.backlogit/queue/<id>.md` (e.g. `062.001-T` → `.backlogit/queue/062.001-T.md`).

## Risky-action state

No `strict-safety`-class destructive action. The single state-mutating action (`shipment ship 061-S`) was gated behind a pre-mode reconcile (PROCEED) and produced the expected, fully-reconciled archive result (post-mode reconcile PROCEED, P-007 zero archive-dir deletions).

## Follow-up recommendations

- None blocking. Monitoring is fully covered by the CI guardrails: the new parity / section / dry-run regression tests run on every PR via `test (1.24)`, plus the `shipment-reconcile` GI/GR gate on every future ship.
- Pre-existing, out-of-scope: 2 malformed legacy `archived_from` records (`038-DL`, `039-DL`) remain flagged-only — tracked independently of this shipment.

## Verdict

**PASS** — all five invariants verified by fresh green tests, a live CLI catalog inspection, and a clean live ship (atomic, no torn state, canonical archive stamping). Fed to operational-closure as PASS.
