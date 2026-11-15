---
title: "214-S compound refresh and knowledge graduation"
doc_type: closure
source: docs/closure/214-S-compound-refresh.md
ingested_at: "2026-11-15T00:50:00Z"
schema_version: "1.0"
chunk_strategy: h1-h2-h3
docline:
  shipment_id: 214-S
  feature_id: 213-F
---

## Invocation and evidence

Invoked `compound-refresh` (`scope: recent`, `mode: apply`) tied to shipment
`214-S` (`213-F`) and evaluated durable documentation under Ship Step 6 items 3
and 4. Evidence sources: merged PR #649 (`f9460bac1c2c3c7d0e2098aa88d3f6095e9779d3`),
`internal/toolchain/launcher.go`, `internal/cli/root.go`, `.backlogit/toolchain.yaml`,
and `docs/closure/evidence/214-S-213-F-close-evidence.json`.

## Compound classifications

| Entry | Classification | Rationale |
| --- | --- | --- |
| [Markdownlint library entrypoint false green](../compound/workflow-issues/markdownlint-library-entrypoint-false-green-2026-11-13.md) | `keep` | Verified accurate; `scripts/md-lint.ps1` runs `npx --yes markdownlint-cli2@0.23.1` and prints markdownlint-cli2's lint summary and file count |
| [Derive daemon compatibility from pinned image](../compound/runtime-errors/derive-daemon-version-from-pinned-image-2026-11-09.md) | `keep` | Verified accurate; U12 multi-workspace containment tests rely on pinned image compatibility |
| [Historical source archival guidance](../compound/workflow-issues/source-artifact-archival-pattern-2026-04-20.md) | `keep` (`status: stale`) | Already marked `stale` with current explicit-membership boundary; `214-S` preflight confirmed zero linked stash entries before cascade close |

Reviewed: 3 entries (`keep: 3`, `update: 0`, `consolidate: 0`, `replace: 0`, `delete: 0`).

## Knowledge graduation updates

Applied direct documentation updates during Ship Step 6 item 4 so durable
architecture and design documentation match shipped `214-S` reality:

* Updated `docs/ARCHITECTURE.md`: added `toolchain` to the `cli` dependency
  arrow (`cli -> core, db, models, telemetry, release, version, toolchain`),
  updated `internal/toolchain/` description, and recorded that
  `VerifyLauncherScripts`, `AuditRepositoryToolchain`, `WriteToolchainManifest`,
  and `RenderPinnedMCPConfig` shipped in `214-S` alongside `.backlogit/toolchain.yaml`.
* Updated `docs/design-docs/2026-11-04-multi-session-mcp-runtime-and-workspace-toolchain.md`:
  updated `Status and scope`, `Level 1 workspace toolchain`, and
  `Operator-owned verified provisioning` to reflect that Track C (`214-S`)
  shipped launcher auditing, manifest writing, MCP config rendering, and
  `backlogit mcp status --toolchain [--strict]`, while operator-owned local
  binaries and `.mcp.json` remain under operator custody.

Pre-existing active stash entries `3466D3CB` and `08A5E6F7` (captured under
P-021 C2 during `213.003-T`) remain untouched in `.backlogit/stash.jsonl` for
Stage triage per Ship Role Boundary (`P-010`) and P-021 C5.
