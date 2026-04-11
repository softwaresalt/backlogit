# Stage Session: Core Data Integrity & CQRS Compliance

**Date**: 2026-04-10
**Session ID**: 0201daed-7079-407b-b68f-4296f9c64da6

## Stash Entries Processed

| Stash ID | Priority | Kind | Description | Disposition |
|---|---|---|---|---|
| DF8FDB7B | high | feature | Make semantic links durable outside SQLite | Harvested → Layer 1 (Units 3-6) |
| 847DCF02 | high | feature | Restore Markdown-first invariants | Harvested → Layer 2 (Units 7-10) |
| AE9DB2B6 | high | task | Enforce SQLite per-connection PRAGMAs | Harvested → Layer 0 (Units 1-2) |
| C710BEDB | high | feature | Harden MCP consistency | Harvested → Layer 3 (Units 11-15) |

## Artifacts Created

| Type | ID | Title |
|---|---|---|
| Deliberation | 012-DL | Core Data Integrity & CQRS Compliance |
| Plan | — | docs/exec-plans/2026-04-10-core-data-integrity-cqrs-plan.md |
| Review | — | .copilot-tracking/plan-review/2026-04-10-core-data-integrity-cqrs-plan-review.md |
| Feature | 026-F | Core Data Integrity & CQRS Compliance |
| Task | 026.001-T | Apply SQLite PRAGMAs via DSN and constrain pool |
| Task | 026.002-T | Connection pragma integration tests |
| Task | 026.003-T | Add links to Markdown frontmatter model |
| Task | 026.004-T | Write-through link operations (Markdown-first) |
| Task | 026.005-T | Rebuild links during rehydration from Markdown |
| Task | 026.006-T | Link durability round-trip tests |
| Task | 026.007-T | Fix UpdateArtifact to fail on missing Markdown path |
| Task | 026.008-T | Flip BulkUpdateStatus to Markdown-first |
| Task | 026.009-T | Add file relocation to move handler |
| Task | 026.010-T | Write-path invariant integration tests |
| Task | 026.011-T | Standardize ErrNotFound mapping in MCP error handler |
| Task | 026.012-T | Normalize shipment response shapes |
| Task | 026.013-T | Add mutex/double-check to ensureWorkspace |
| Task | 026.014-T | Cascade-delete orphaned rows on item deletion |
| Task | 026.015-T | MCP contract consistency tests |
| Shipment | 010-S | Core Data Integrity & CQRS Compliance |

## Dependency Graph

```
Layer 0 (DB Connection):
  026.001-T → 026.002-T

Layer 1 (Links Persistence):
  026.003-T → 026.004-T → 026.005-T → 026.006-T

Layer 2 (Markdown-First):
  026.007-T → 026.008-T ─┐
  026.007-T → 026.009-T ─┼→ 026.010-T
                          ┘

Layer 3 (MCP Contracts):
  026.011-T ─┐
  026.012-T ─┤
  026.013-T ─┼→ 026.015-T
  026.014-T ─┘
```

Layers 0-3 can run in parallel. Within each layer, the graph above governs ordering.

## Review Amendments (carry into implementation)

- **F-1**: Link write-through must be Markdown-first, then SQLite cache
- **F-2**: DSN pragma syntax needs spike test before committing to approach
- **F-3**: Links field is Markdown-only; `upsertItemTx` ignores it
- **F-4**: Use mutex/double-check for `ensureWorkspace`, not `sync.Once`
- **F-7**: Pre-deployment migration needed for existing SQLite-only links

## Next Steps

Ship agent should claim shipment `010-S` for the harness → build → review → CI → PR lifecycle.
