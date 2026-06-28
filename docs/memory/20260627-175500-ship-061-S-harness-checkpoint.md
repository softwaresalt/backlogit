# Ship Checkpoint — 061-S Harness Generation Complete

- **Date**: 2026-06-27T17:55 -0700
- **Phase**: harness-generated (TDD red verified)
- **Shipment**: 061-S "Metadata and Section Sync Integrity" (active) → carries feature 062-F + 5 tasks
- **Branch**: feat/062-metadata-section-sync-integrity (off main @ 02f5603b)

## Reproduction verdict (all 5 bugs reproduce against current main)

| Task | Bug | Location | Red tests |
|---|---|---|---|
| 062.001-T | MCP metadata catalog omits CLI command data (nil cliRoot) | internal/mcp/metadata.go:60 | metadata_parity_test.go (2 FAIL) |
| 062.002-T | export-command-map resolves under .backlogit, not workspace root | internal/mcp/metadata.go:94 | export_command_map_test.go (1 FAIL + 1 regression-lock PASS) |
| 062.003-T | section rewrite never re-upserts DB/FTS | internal/mcp/tools.go writeSectionsToFile | section_reupsert_test.go (2 FAIL) |
| 062.004-T | CLI append fallback swallows malformed-marker errors + duplicates blocks | internal/cli/update.go:132-139 | update_section_corruption_test.go (2 FAIL) |
| 062.005-T | MergeSync fallback rehydrate runs before dry_run guard | internal/db/merge_sync.go:95-104 | merge_sync_dryrun_test.go (1 FAIL) |

## Notes / divergence from stale 2026-05-22 plan
- 062.003 bug is on the **MCP** path (CLI path already re-upserts at update.go:155-163).
- 062.004 bug is on the **CLI** path (MCP writeSectionsToFile already does per-section handling, lines 1059-1084, but via fragile string-match — consolidate to sentinel errors).
- 062.001 fix needs DI: cli imports mcp, so mcp can't import cli. Added `Server.CLICommandProvider func() []core.CommandInfo` + exported `Server.MetadataCatalog`.

## Structural stubs already added (uncommitted)
- internal/mcp/server.go: CLICommandProvider field
- internal/mcp/metadata.go: exported MetadataCatalog(ctx) wrapper

## Next steps
- Implement each task green in task order, commit per task tracked to task ID.
- Add CLI-package parity test for 062.001 (true CLI==MCP catalog.CLI).
- Gates: go test ./..., go vet, golangci-lint, gofmt; docline lint if docs touched.
- Leave 061-S/062-F for post-merge closure. MERGE GATE: stop for operator.
