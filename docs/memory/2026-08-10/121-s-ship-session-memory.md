---
doc_type: memory
schema_version: "1.0"
title: 121-S Ship Session Memory
created_at: 2026-08-10T21:55:00Z
session: ship-121-s-135-f
---

# 121-S Ship Session Memory

## Completed

- PR #345 merged at 99e8ecc8 (merge commit)
- All scope items archived: 135-F, 135.001-T..135.009-T, 121-S
- All 12 Copilot review threads (including 1 pre-existing resolved thread on cli/migrate.go) addressed and resolved
- CI passed on all 5 push cycles

## Files Modified (beyond feature implementation)

- internal/core/canonical_scan.go — symlink guard in archive candidate loop
- internal/core/shipment_verify.go — symlink guard in queue candidate loop
- internal/core/artifacts.go — artifactSearchDirs uses workspaceStorageRoot(ws)
- internal/core/doctor_test.go — explicit ws.StorageRoot in test helper
- internal/cli/root.go — Lstat guard before MkdirAll on .backlog
- internal/core/workspace.go — exported IsSymlinkOrReparsePoint helper
- internal/core/archive.go — legacy archived_from root remapping in UnarchiveItem
- internal/core/archive_test.go — TestUnarchiveItem_LegacyArchivedFromAfterMigration
- internal/mcp/server.go — rebind Telemetry/HookEvents on lazy workspace init
- internal/core/migrate_workspace_dir.go — probeWorkspaceCandidate validation
- internal/core/migrate_workspace_dir_test.go — RejectsNonWorkspaceDirectory test
- docs/closure/121-S-closure.md — operational closure record
- .backlogit/archive/121-S.md — shipment archived

## Key Decisions

- WorkspaceStorageRoot fallback change (thread 3) deferred: changing the fallback
  from .backlogit to .backlog requires updating all WorkspaceStorageRoot(ws.RootPath)
  call sites to use workspaceStorageRoot(ws). Tracked as follow-up.
- artifactSearchDirs: changed to use workspaceStorageRoot(ws) to honor ws.StorageRoot
  when set — this is the safe partial fix without the full refactor.

## Failed Approaches

- Changing WorkspaceStorageRoot fallback to .backlog directly: broke
  TestDoctor_FixOrphansArchivesOrphanedTask because probeWorkspaceCandidate
  requires config.yaml which many tests don't create. Deferred to follow-up.

## Open Follow-ups

- [planned] Full fallback fix: change WorkspaceStorageRoot to default to .backlog for
  fresh repos, requiring updating all WorkspaceStorageRoot(ws.RootPath) call sites.
- [planned] workspace_dualroot_test.go parallel env var pollution: remove t.Parallel()
  from tests that mutate BACKLOGIT_WORKSPACE_DIR.
