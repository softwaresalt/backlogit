---
chunk_strategy: h1-h2-h3
description: 'Post-merge operational closure for shipment 061-S — CLI/MCP metadata-catalog parity via dependency injection (no import cycle), export-command-map workspace-root anchoring with containment, MCP section re-upsert to SQLite plus FTS, typed malformed-section errors without duplicate-marker append, and MergeSync dry-run write purity including the fallback rehydrate path (PR #145, merge 006bb854)'
doc_type: closure
docline:
    ms.date: 2026-06-27T00:00:00Z
    ms.topic: reference
ingested_at: "2026-06-28T02:40:00Z"
schema_version: "1.0"
source: docs/closure/2026-06-27-061-S-metadata-section-sync-integrity-closure.md
title: 061-S Metadata and Section Sync Integrity — Post-Merge Operational Closure
---

# Operational Closure — Shipment 061-S (Metadata and Section Sync Integrity)

- **Shipment**: 061-S — Metadata and Section Sync Integrity
- **Feature**: 062-F (5 tasks: 062.001-T … 062.005-T — all done/archived)
- **PR**: #145 — *Metadata and Section Sync Integrity — CLI/MCP parity, section re-upsert, dry-run purity*
- **Merge commit**: `006bb854afa6f56c87f4a80f5d3d6668feef0b58` (merge commit on `main`, P-009 compliant; squash/rebase disabled repo-wide)
- **Closure branch**: `post-merge/062-metadata-section-sync-integrity`
- **Mode**: post-merge
- **Verification**: `docs/closure/2026-06-27-061-S-metadata-section-sync-integrity-runtime-verification.md` — **PASS**
- **Readiness**: **READY** (the change is already merged; this artifact records monitoring + rollback for the shipped scope)
- **ID-offset note**: benign `NNN-S → (NNN+1)-F` cosmetic offset; 061-S legitimately carries 062-F + 062.001-T … 062.005-T (per `docs/decisions/2026-06-25-shipment-manifest-drift-determination-deliberation.md`). There is no `062-S` and no `063` item; 061-S's manifest is authoritative.

## Summary of the change

Fixed five metadata/section/index integrity bugs (origin stashes 5A41B7C3, 6DD3062F, 6235FF06, 51D7384A, EE33B6ED — all confirmed still reproducing on current `main` before the fix):

- **062.001-T — Restore CLI metadata parity in MCP.** The MCP metadata catalog previously passed `nil` for the CLI root, so the catalog omitted CLI command data. The fix injects command data via `Server.CLICommandProvider func() []core.CommandInfo` (wired by the CLI's `wireMCPMetadataProvider`), which **breaks the would-be `internal/mcp → internal/cli` import cycle** while keeping `internal/mcp` cobra-free. A CLI ≡ MCP parity test (`TestMetadataCatalog_CLIAndMCPParity`) locks the contract. Files: `internal/mcp/metadata.go`, `internal/cli/` MCP wiring.
- **062.002-T — Align export-command-map workspace root.** `export-command-map` was anchored at `s.backlogitDir()` instead of the workspace root, so it wrote to / contained against the wrong base. Re-anchored at the workspace root; path containment still rejects `../` escapes (`TestExportCommandMap_RejectsEscapeOutsideWorkspace`). Files: `internal/mcp/tools.go`.
- **062.003-T — Re-upsert rewritten section bodies (MCP path).** `mcp/tools.go writeSectionsToFile` rewrote the file but never re-upserted the artifact into SQLite + FTS, leaving the index stale relative to the file (the CLI path already re-upserted). The MCP write now re-upserts so the index matches the file immediately. Files: `internal/mcp/tools.go`.
- **062.004-T — Stop CLI section corruption fallback.** `cli/update.go` blanket-appended a section block on **any** `WriteSections` error, masking malformed markers and duplicating sections. The fix surfaces typed sentinels (`parser.ErrSectionMalformed` / `parser.ErrSectionNotFound`) and only appends on a genuine "section not found"; malformed input now errors instead of corrupting. Files: `internal/cli/update.go`, `internal/core`/`parser` section validation.
- **062.005-T — Enforce MergeSync dry-run purity.** `db/merge_sync.go` ran the fallback rehydrate **write** before the `dryRun` guard, so a dry run could still mutate the DB. The dry-run short-circuit was moved before all write branches, including the fallback rehydrate (`TestMergeSync_DryRunWithFallback_DoesNotWrite`). Files: `internal/db/merge_sync.go`.

A review-hardening commit (`2f245840`) consolidated section-name validation into `parser.ValidateSectionName` (single source of truth, used by both CLI and MCP write paths) and added symmetric `.tmp` cleanup on WriteFile-error branches.

## Invariants to preserve

1. The MCP metadata catalog carries the **same** CLI command data as the CLI catalog (CLI ≡ MCP parity), supplied via dependency injection — `internal/mcp` must **not** import `internal/cli`.
2. `export-command-map` writes relative to the **workspace root**, and containment rejects any `../` path escape.
3. A section write through **either** surface (CLI or MCP) re-upserts the artifact into SQLite + FTS, so a subsequent read/query reflects the new body immediately (no stale index).
4. Malformed or invalid section markers produce **typed errors**; no surface appends a duplicate section block on a generic write failure.
5. A `MergeSync` dry run performs **zero** DB writes — including on the fallback rehydrate path.

## Pre-deploy audits

Not applicable — no migration, feature flag, config, or access change. Pure library/CLI/MCP correctness fix. Already merged.

## Deployment / rollout path

Merge-only. The fix ships as part of the `backlogit` binary; no service deploy, no data migration, no rollout window. Consumers pick it up on next binary build (repo-root `backlogit.exe` already rebuilt from `006bb854`). MCP clients pick up the catalog-parity and re-upsert behavior on their next server restart against the new binary.

## Post-deploy checks (already executed in this closure)

- `go test ./internal/mcp/... ./internal/cli/... ./internal/db/...` green; the 13 named parity/section/dry-run regression tests pass fresh (`-count=1`), see verification report.
- Live `backlogit metadata catalog` emits a populated `"cli"` command array (catalog carries CLI command data).
- Live `shipment ship 061-S` → `returned_ids: []`, all 7 manifest items `archived`; queued- and active-shipment queues empty.
- `doctor --check-archived-from` → 0 self-referential; only the 2 known malformed legacy records (`038-DL`, `039-DL`).

## Healthy signals

- CLI and MCP metadata catalogs agree (parity test green; live CLI catalog carries command data).
- After a section write, an immediate index query/read reflects the new body (SQLite + FTS in sync with the file).
- Malformed section input returns a typed error and leaves the file uncorrupted (no duplicated section block).
- A `MergeSync` dry run reports its planned delta but mutates nothing.
- `export-command-map` writes under the workspace root and rejects `../` escapes.

## Failure signals (rollback triggers)

- MCP catalog drift: the MCP catalog missing CLI command data the CLI catalog has (parity regression), or a re-introduced `internal/mcp → internal/cli` import cycle.
- Stale index after a section write: a query/read returning the pre-write body.
- Duplicated section blocks or file corruption after a section write against malformed input.
- Any DB write observed during a `MergeSync` dry run.
- `export-command-map` writing outside the workspace root or accepting a `../` escape.

## Monitoring plan

- **CI guardrails**: the parity (`TestMetadataCatalog_CLIAndMCPParity`, `*_IncludesCLICommandsFromProvider`, `*_EmitsCLICommands`), section re-upsert (`*_SectionRewrite_ReupsertsDB`, `*_AfterSectionRewrite_ReturnsUpdatedBody`, `TestUpdateCommand_SectionWrite_SyncsDB`), malformed-section (`*_MalformedSectionMarker_ReturnsError`, `*_InvalidSectionName_Rejected`), export-command-map containment (`*_WritesUnderWorkspaceRoot`, `*_RejectsEscapeOutsideWorkspace`), and dry-run purity (`TestMergeSync_DryRunWithFallback_DoesNotWrite`, `TestMergeSync_DryRunDoesNotModifyDatabase`) suites run on every PR via `test (1.24)`.
- **Per-ship gate**: the `shipment-reconcile` GI/GR double-entry check (pre + post) plus `doctor --check-archived-from` on every Ship Step 6 closure (index-consistency dogfood).
- No dashboards/alerts — these are CLI/MCP/library correctness invariants, watched by tests + the reconcile/doctor gates, not a live service metric.

## Rollback trigger & procedure

- **Trigger**: any healthy-signal inversion above (catalog drift, stale index after a section write, duplicated section blocks, or a dry-run DB write) traced to this change.
- **Procedure**: `git revert 006bb854afa6f56c87f4a80f5d3d6668feef0b58` (revert the merge commit), rebuild the binary, re-run `go test ./internal/mcp/... ./internal/cli/... ./internal/db/...`. No data migration to unwind — the change is pure library/handler logic plus DI wiring.

## Risky-action record

No `strict-safety`-class destructive action. The single state mutation in this closure (`shipment ship 061-S`) was gated by pre-mode reconcile (PROCEED), produced the expected fully-reconciled archive (post-mode reconcile PROCEED), and passed the P-007 deleted-file guard (0 archive-dir deletions).

## Validation window & owner

- **Window**: covered indefinitely by CI + the reconcile/doctor gates; no time-boxed observation needed for a merge-only library fix.
- **Owner**: repository maintainer (softwaresalt) via CI; Ship agent for per-shipment reconcile.

## Source artifact cleanup

- `062-F` carries **no** `source_stash_id` / `source_deliberation_id` custom field (only `harness_status`) → custom-field-driven cleanup is a **no-op** (logged, nothing archived).
- Origin stashes `5A41B7C3`, `6DD3062F`, `6235FF06`, `51D7384A`, `EE33B6ED` — verified **already absent** from the active stash (harvested by Stage upstream) → nothing to remove.
- Durable rationale (`docs/decisions/2026-05-22-new-bug-backlog-grouping.md`, drift determination `2026-06-25-…-deliberation.md`, exec plan `docs/exec-plans/2026-05-22-metadata-section-sync-integrity-plan.md`) is **kept** as institutional knowledge — not retired.

## Feedback into the harness

- New compound learning authored: `docs/compound/2026-06-27-cli-mcp-catalog-parity-via-di-and-index-consistency.md`.
- Compound-refresh review: `docs/closure/2026-06-27-061-S-metadata-section-sync-integrity-compound-refresh.md` (existing `2026-05-07-mcp-cli-config-parity.md` classified **keep** — complementary, distinct mechanism).
- No new safety mode, verification pattern, or reviewer gap surfaced.

## Follow-ups

- None blocking. Pre-existing out-of-scope item: 2 malformed legacy `archived_from` records (`038-DL`, `039-DL`) remain flagged-only, tracked independently.

## Readiness

**READY** — merged change with PASS verification, green CI regression suites, clean live ship (atomic, no torn state, canonical archive stamping), and an explicit revert-based rollback path.
