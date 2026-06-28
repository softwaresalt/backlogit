---
chunk_strategy: h1-h2-h3
description: 'Four reusable integrity rules from 061-S/062-F — supply cross-layer parity data by dependency injection to avoid import cycles and lock it with a parity test; re-upsert into SQLite plus FTS on every mutating write so the index never diverges from the file; place dry-run guards before every write branch including fallbacks; and surface typed errors instead of a corrupting blanket-append fallback'
doc_type: learning
docline:
    date: 2026-06-27T00:00:00Z
    severity: high
    tags:
        - mcp
        - cli
        - parity
        - dependency-injection
        - import-cycle
        - sqlite
        - fts
        - index-consistency
        - dry-run
        - error-handling
schema_version: "1.0"
source: docs/compound/2026-06-27-cli-mcp-catalog-parity-via-di-and-index-consistency.md
title: 'CLI/MCP catalog parity via DI + index-consistency, dry-run, and typed-error rules (061-S)'
---

# CLI/MCP Catalog Parity via DI + Index-Consistency, Dry-Run, and Typed-Error Rules

Four durable rules graduated from shipment 061-S (feature 062-F, "Metadata and
Section Sync Integrity", PR #145, merge `006bb854`). Each fixed a bug that was
**still reproducing on `main`** at build time.

## Rule 1 — Supply cross-layer parity data by dependency injection; lock it with a parity test

### Problem

The MCP metadata catalog must expose the **same** CLI command data as the CLI
catalog. The natural implementation (`internal/mcp` calling into `internal/cli`
to enumerate cobra commands) creates an **import cycle**: `cli` already imports
`mcp` to wire the server. The original code dodged the cycle by passing `nil`
for the CLI root — so the MCP catalog silently shipped **without** CLI command
data. (This is the catalog-level sibling of the earlier config-parity finding in
`2026-05-07-mcp-cli-config-parity.md`, which was about config *option loading* in
both handlers.)

### Rule

When the lower layer (`mcp`) needs data owned by the higher layer (`cli`), do
**not** import upward. Define a provider seam on the lower layer and have the
higher layer inject it at wiring time:

```go
// internal/mcp — no import of internal/cli
type Server struct {
    // ...
    CLICommandProvider func() []core.CommandInfo // injected; nil-safe
}

// internal/cli — already imports mcp, wires the provider
func wireMCPMetadataProvider(s *mcp.Server) {
    s.CLICommandProvider = func() []core.CommandInfo { return enumerateCLICommands() }
}
```

Then **lock the contract with a parity test** that asserts the two surfaces are
equal (`TestMetadataCatalog_CLIAndMCPParity`). The test is the regression guard
that prevents the catalog from silently drifting again.

### Why it works

The shared payload type lives in a leaf package (`internal/core`), so both layers
depend *downward* only. The provider is nil-safe, so the MCP server still runs
standalone (just with an empty CLI section) when no provider is wired.

## Rule 2 — Re-upsert into SQLite + FTS on every mutating write so the index never diverges from the file

### Problem

The MCP section-write path (`writeSectionsToFile`) rewrote the markdown file but
never re-upserted the artifact into SQLite + FTS. The CLI path already did. Result:
after an MCP section edit, a subsequent query/read returned the **stale** pre-edit
body — the index silently diverged from the file.

### Rule

Any code path that mutates an artifact's on-disk body MUST re-upsert that artifact
into the index (SQLite **and** FTS) in the same operation. The file is the source
of truth; the index is a derived cache that must be refreshed at the write site,
not lazily. Audit **all** write surfaces (CLI *and* MCP) when adding a mutation —
parity bugs hide on the surface you didn't touch.

### Verification

`TestHandleUpdateItem_SectionRewrite_ReupsertsDB` +
`TestHandleGetItem_AfterSectionRewrite_ReturnsUpdatedBody` prove the read-back
reflects the new body immediately on the MCP path; `TestUpdateCommand_SectionWrite_SyncsDB`
covers the CLI path.

## Rule 3 — Place the dry-run guard before EVERY write branch, including fallbacks

### Problem

`MergeSync` honored `dryRun` on its primary incremental path but the **fallback
rehydrate** branch performed a full-DB write *before* reaching the `dryRun` guard.
So a "dry run" could still mutate the database whenever the delta was large enough
to trigger the fallback.

### Rule

A dry-run / read-only mode must short-circuit before **every** code path that
writes — not just the common one. When a function has a primary path and one or
more fallback paths, the dry-run check belongs at the **entry** of the write
decision, or it must be duplicated at the head of each fallback. Test the dry-run
guard specifically **on the fallback path**, not only the happy path.

### Verification

`TestMergeSync_DryRunWithFallback_DoesNotWrite` exercises the large-delta fallback
under dry run and asserts zero DB writes; `TestMergeSync_DryRunDoesNotModifyDatabase`
covers the primary path.

## Rule 4 — Surface typed errors instead of a corrupting blanket-append fallback

### Problem

`cli/update.go` appended a fresh section block on **any** `WriteSections` error.
When the error was actually a *malformed* existing marker, the blanket-append
masked the real failure and **duplicated** the section, corrupting the file.

### Rule

A fallback action must be gated on a **specific, typed** error, never a catch-all.
Define sentinels at the layer that understands the grammar (here
`parser.ErrSectionNotFound` / `parser.ErrSectionMalformed`) and branch on them:
append only on "not found"; **propagate** "malformed" so the caller sees a real
error instead of silently corrupting state. Co-locate the validation
(`parser.ValidateSectionName`) with the grammar it enforces and share it across
all call sites (CLI + MCP) so there is a single source of truth.

### Verification

`TestUpdateCommand_MalformedSectionMarker_ReturnsError` (malformed → typed error,
no duplication) + `TestUpdateCommand_InvalidSectionName_Rejected`.

## References

- PR #145 — fix(core): metadata and section sync integrity (061-S / 062-F)
- Merge commit `006bb854afa6f56c87f4a80f5d3d6668feef0b58`
- Closure: `docs/closure/2026-06-27-061-S-metadata-section-sync-integrity-closure.md`
- Related (complementary, kept): `docs/compound/2026-05-07-mcp-cli-config-parity.md`
  — config-option loading in both handlers; this entry is about catalog *data*
  parity via DI + the broader index/dry-run/typed-error rules.
