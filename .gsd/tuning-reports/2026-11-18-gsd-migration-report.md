---
title: "GSD 1.6.0 migration report"
doc_type: tuning-report
schema_version: "1.0"
created_at: "2026-11-18T21:13:03Z"
status: complete
source: gsd-migration
---

# GSD 1.6.0 migration report

## Outcome

The backlogit workspace was migrated from the legacy AutoHarness state layout
to GSD 1.6.0 on `chore/gsd-harness-migration`.

## Applied changes

- Installed the GSD 1.6.0 executable and verified asset archive under `.bin/`.
- Uninstalled the local `autoharness@autoharness` Copilot CLI plugin, removed
  its marketplace registration and plugin cache, and cleared the stale
  `enabledPlugins` entry.
- Renamed tracked `.autoharness/` state and evidence to `.gsd/`.
- Normalized harness config, profile, manifest, and generated references to the
  `.gsd/` state root.
- Normalized the folded core capability-pack entries and migrated the legacy
  `graphtor-docs` pack identity to `kbdb`.
- Renamed the KBDB instruction file and updated MCP commands to workspace-local
  executables under `.bin/`.
- Deployed all missing GSD 1.6.0 product skills (`browser-automation`, `observe`,
  `learn`, `evolve`, `grill-me`, `gsd-show-dag-queue`, and `gsd-dark-mode`)
  plus the locally owned maintenance skills previously supplied by the removed
  plugin (`install-harness`, `tune-harness`, `verify-harness`, and
  `workspace-discovery`).
- Deployed the corresponding dark-mode, queue, install, and tune prompt shims;
  the workspace now matches the GSD source skill and prompt inventory.
- Pinned backlogit registry operations to the legacy `.backlogit/` storage root
  and the workspace-local backlogit executable.
- Updated startup-script contracts to resolve `.bin/` tools while preserving the
  read-only KBDB launch posture.
- Refreshed affected manifest checksums and config/profile hashes.

## Verification

`gsd verify-workspace` reports zero strict schema blockers, zero migration
proposals, and zero unresolved findings. The migrated startup configuration
and YAML/JSON syntax checks pass, and `go test -timeout=30m ./...` passes in
the backlogit workspace. The bundled-skill comparison and the full GSD
source-skill comparison both report no missing skills or prompts.
`copilot plugin list` reports no installed plugins,
and the registered marketplace list no longer includes AutoHarness. The
remaining advisory is
`backlogit-engine-version-unverified` for backlogit 1.13.0. No engine-semantic
probe was claimed during this harness migration; the existing unverified-engine
fallback remains authoritative.

## Intentional continuity

- Existing `.backlogit/` work data and legacy compatibility paths were preserved.
- Existing operator customizations recorded in `.gsd/drift-ignore` were retained.
- Runtime binaries and caches under `.bin/` remain local and ignored.
- Historical cascade-close journals and tuning reports were moved without
  rewriting their evidence.
