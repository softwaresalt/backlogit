# Stage session checkpoint — E16F4664 CLI/MCP command parity

- **Agent**: Stage
- **Date**: 2026-07-03
- **Phase**: triage + grounding complete; deliberation next
- **Stash**: `E16F4664` (kind=feature, priority=medium) — CLI↔MCP command parity audit + gap fills + discoverability + CLI fallback.
- **Related stash**: `7ECBAC7E` (kind=task, priority=low) — CLI `shipment list` null-vs-`[]` parity gap. **Decision: FOLD IN** as a task under the E16F4664 covering feature (cannot link via CLI — no `link` command exists; folding avoids duplicated scope).

## Tooling status (Step 0.0 / 0.1)
- `TOOL_OK: backlogit CLI` (v1.2.0; go1.26.1). Manual/CLI-backed mode — no direct MCP tool access in this session, so shipment assembly uses `shipment create --items` (all items ordered, feature first).
- `INDEX_SYNC_OK` — 679 artifacts indexed.
- Stale/quarantined checkpoints from Apr 2026 present (validation errors); none relate to E16F4664 — proceeding fresh, not resolving operator's pre-existing quarantine.
- `features.shipments: true` → Step 5.5 shipment assembly MANDATORY.

## Grounded parity findings (code-truth, not registry claims)
Surfaces: **56 MCP tools** (from `backlogit manifest`) vs actual cobra CLI tree.

### A. Registry STALE — CLI exists but registry `cli_command` empty (9)
deliberate, export_command_map (`metadata export-command-map`), get_metadata_catalog (`metadata catalog`), get_version (`version`), stash_archive, stash_edit, stash_get, stash_remove (`stash archive`/alias `remove`), telemetry_harvest (`telemetry harvest`).

### B. Registry MISSING op entirely — MCP tool + CLI both exist, no map row (3)
docs_lint (`docs lint`), docs_migrate (`docs migrate`), docs_scope (`docs scope`). (Also CLI-only `docs classify` has no MCP tool — reverse gap.)

### C. Registry OVER-CLAIM — maps to a CLI command that DOES NOT EXIST (3, most dangerous)
add_link → `link add`, remove_link → `link remove`, get_links → `link list`. There is **no `link` CLI command**. An agent following the fallback map would run a non-existent command.

### D. TRUE GAPS — no CLI, correctly empty (11)
ack_hook_events, add_to_shipment (**HIGH** — shipment workflow fallback), append_comment, create_checkpoint (asymmetric: list/get/resolve/cleanup have CLI), get_wit_metadata, list_templates, list_types, log_telemetry (likely intentional MCP-only), merge_sync, poll_hook_events, save_memory. Plus the 3 link ops from (C) are also true gaps.

### E. CLI-only (no MCP tool — mostly intentional)
init, mcp, manifest, completion, migrate, status, `queue bulk-status`, `queue move`, most `telemetry` subcommands, `docs classify`.

## Prior art (compound, confidence: HIGH)
- `2026-06-27-cli-mcp-catalog-parity-via-di-and-index-consistency.md` — DI seam `CLICommandProvider` + `TestMetadataCatalog_CLIAndMCPParity`; **Rule 1: lock parity with a test.**
- `go-patterns/manual-schema-registry-drift-detection-2026-05-22.md` — curated registry + reflection/enumeration **drift-detection test** (063-S pattern). Directly applies to the registry op-map drift.
- `2026-05-07-mcp-cli-config-parity.md` — both entry surfaces must load options; parity checklist.
- `go-patterns/f015-shipment-stash-patterns.md` + `docs/exec-plans/2026-07-03-consolidate-shipment-items-normalization-plan.md` — `core.NormalizeShipmentItems` already exists (17D29DDC); 7ECBAC7E fix = call it in CLI list handler + guard test.

## Design decisions (for deliberation)
- D1 fill-all vs fill-highest-value+defer → **fill highest-value now, defer net-new command surfaces to a phase-2 stash, document intentional exceptions.**
- D2 registry over-claim → **make registry HONEST now + add drift-detection test; defer building `link`/other net-new CLI to phase-2.**
- D3 7ECBAC7E → **fold in as a task.**
- D4 null→[] → **minimal fix: call existing `core.NormalizeShipmentItems` in CLI list handler + cross-surface guard test** (core-shaper-by-construction noted as evaluated-but-deferred).
- D5 discoverability → grouped help + `export_command_map` accuracy + a CLI-fallback guidance doc.

## Proposed shipment scope (5 tasks)
T1 audit matrix (docs) · T2 registry op-map correction + drift-detection test (config+test) · T3 `shipment add` CLI (code+test) · T4 7ECBAC7E null→[] fold-in (code+test) · T5 discoverability + CLI-fallback guidance (docs). Deferred → phase-2 stash: link CLI group, checkpoint create, hook poll/ack, save_memory, append_comment, wit/types/templates, merge_sync CLI.

## Next steps
1. Create native DL (`backlogit deliberate E16F4664`) + docs/decisions deliberation doc.
2. impl-plan → (plan-harden if triggered; expected: no) → plan-review (multi-persona, must PASS) → harvest → shipment (queued) → archive E16F4664 + fold 7ECBAC7E.
