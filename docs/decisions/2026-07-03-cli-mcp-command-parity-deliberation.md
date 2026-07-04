---
chunk_strategy: h1-h2-h3
description: 'Deliberation for stash E16F4664 — audit CLI/MCP command parity grounded in the running binary, make the backlog-registry fallback map honest, fill the highest-value CLI gaps (add_to_shipment, shipment-list null-vs-[]), add drift-detection, and defer net-new command surfaces to a documented phase-2 stash.'
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-07-03-cli-mcp-command-parity-deliberation.md
title: 'CLI/MCP command parity: audit, honest fallback map, and highest-value gap fills'
stash_id: E16F4664
decision_status: decided
promoted_to: plan
---

## Source

- Stash: `E16F4664` (kind=feature, priority=medium) — "Check for CLI command parity with MCP mode tools; ensure full command parity + simple/clear discoverability + a clear agent fallback to CLI mode when MCP mode becomes unavailable." Operator request, 2026-07-03.
- Native deliberation artifact: `050-DL` (created via `backlogit deliberate E16F4664`, linked to the stash for backlog-native recovery).
- Related stash: `7ECBAC7E` (kind=task, priority=low) — CLI `shipment list` null-vs-`[]` parity gap. **Folded into this feature** (see D3).
- Prior learnings (compound, confidence: high):
  - `docs/compound/2026-06-27-cli-mcp-catalog-parity-via-di-and-index-consistency.md` — Rule 1: supply cross-layer parity data by DI and **lock the contract with a parity test**; the `CLICommandProvider` DI seam and `TestMetadataCatalog_CLIAndMCPParity` already exist.
  - `docs/compound/go-patterns/manual-schema-registry-drift-detection-2026-05-22.md` (063-S) — a manually curated registry must be guarded by a reflection/enumeration **drift-detection test** or it drifts silently. This is exactly what happened to the registry operation map.
  - `docs/compound/2026-05-07-mcp-cli-config-parity.md` — both entry surfaces (CLI + MCP) must be audited together.
  - `docs/exec-plans/2026-07-03-consolidate-shipment-items-normalization-plan.md` (17D29DDC) — `core.NormalizeShipmentItems` already exists as the shared never-null shaper.

## Problem Frame

Agents operate primarily through the backlogit MCP tool surface. When MCP is unavailable or degraded, they must fall back to the CLI. The `.autoharness/backlog-registry.yaml` operation map is the contract that tells an agent which CLI command mirrors each MCP tool — it **is** the fallback map. If that map is wrong, the fallback is broken exactly when it is needed most.

Grounding the audit in the running binary (not the registry's own claims): **56 MCP tools** enumerated from `backlogit manifest`, compared against the actual cobra CLI command tree. The registry has drifted in three distinct ways, one of which is actively dangerous.

## Grounded Parity Audit (code-truth)

Legend: ✅ registry correct · ⚠️ registry stale/missing (CLI exists) · ❌ registry over-claims a non-existent CLI command · ➖ true gap (no CLI, correctly empty) · 🔒 CLI-only (intentional).

### Class A — Registry STALE: CLI exists but the registry `cli_command` column is empty (9)

| Operation | MCP tool | Real CLI command | Registry |
|---|---|---|---|
| deliberate | `backlogit_deliberate` | `backlogit deliberate <stash>` | ⚠️ empty |
| get_metadata_catalog | `backlogit_get_metadata_catalog` | `backlogit metadata catalog` | ⚠️ empty |
| export_command_map | `backlogit_export_command_map` | `backlogit metadata export-command-map` | ⚠️ empty |
| get_version | `backlogit_get_version` | `backlogit version` | ⚠️ empty |
| telemetry_harvest | `backlogit_telemetry_harvest` | `backlogit telemetry harvest` | ⚠️ empty |
| stash_get | `backlogit_stash_get` | `backlogit stash get <id>` | ⚠️ empty |
| stash_edit | `backlogit_stash_edit` | `backlogit stash edit <id>` | ⚠️ empty |
| stash_archive | `backlogit_stash_archive` | `backlogit stash archive <id>` | ⚠️ empty |
| stash_remove | `backlogit_stash_remove` | `backlogit stash remove <id>` (alias of `stash archive`) | ⚠️ empty |

### Class B — Registry MISSING the op entirely: MCP tool + CLI both exist, no map row (3)

| Operation | MCP tool | Real CLI command | Registry |
|---|---|---|---|
| docs_lint | `backlogit_docs_lint` | `backlogit docs lint` | ⚠️ absent |
| docs_migrate | `backlogit_docs_migrate` | `backlogit docs migrate` | ⚠️ absent |
| docs_scope | `backlogit_docs_scope` | `backlogit docs scope` | ⚠️ absent |

(Reverse gap: CLI-only `backlogit docs classify` has no MCP tool — intentional CLI convenience.)

### Class C — Registry OVER-CLAIMS a non-existent CLI command (3, dangerous)

| Operation | MCP tool | Registry claims | Reality |
|---|---|---|---|
| add_link | `backlogit_add_link` | `backlogit link add ...` | ❌ **no `link` command exists** |
| remove_link | `backlogit_remove_link` | `backlogit link remove ...` | ❌ **no `link` command exists** |
| get_links | `backlogit_get_links` | `backlogit link list ...` | ❌ **no `link` command exists** |

An agent that follows the fallback map for semantic links today would invoke a command that does not exist. This is the worst failure mode of the three classes because it produces a confident-but-wrong fallback.

### Class D — TRUE GAPS: MCP tool with no CLI equivalent (14, incl. the 3 link ops)

`add_to_shipment` (**HIGH** — the shipment-assembly workflow has no CLI fallback), `add_link`/`remove_link`/`get_links` (**HIGH** — semantic links; also the Class-C over-claim), `create_checkpoint` (**MEDIUM** — asymmetric: `checkpoint list/get/resolve/cleanup` all have CLI, only create is missing), `poll_hook_events`/`ack_hook_events` (**MEDIUM**), `save_memory` (**MEDIUM**), `append_comment` (**MEDIUM**), `get_wit_metadata`/`list_types`/`list_templates` (**MEDIUM** — partly covered by `metadata catalog`), `merge_sync` (**LOW/MEDIUM**), `log_telemetry` (**LOW** — likely intentional MCP-only agent-internal event logging).

### Class E — CLI-only (no MCP tool, intentional)

`init`, `mcp`, `manifest`, `completion`, `migrate`, `status`, `queue bulk-status`, `queue move`, most `telemetry` subcommands, `docs classify`. These are operator/bootstrap conveniences and are **out of scope** for MCP parity.

## Options

**Option A — Fill every gap now.** Reach literal 100% CLI parity, including net-new `link`, `checkpoint create`, hook, memory, comment, wit/types/templates, and `merge_sync` commands. Rejected: mixes ~14 net-new command surfaces + tests into one shipment, blows past the 2-hour-per-task and sensible-shipment-size constraints, and bundles low-value ops (log_telemetry) that are plausibly intentional MCP-only.

**Option B — Make the map honest + fill the highest-value gaps + guard against re-drift; defer net-new surfaces (CHOSEN).** Correct all Class-A/B/C drift so the fallback map never lies, add a drift-detection test so it cannot silently re-drift, fill the single highest-value true workflow gap (`add_to_shipment` → `shipment add`), fold in the concrete `7ECBAC7E` null-vs-`[]` parity bug, and ship discoverability + a documented CLI-fallback guide. Defer the remaining net-new command surfaces to a documented phase-2 stash. This delivers all four operator deliverables (audit, gap-fills-or-documented-exceptions, discoverability, fallback guidance) at a shippable size while neutralizing the dangerous over-claim immediately.

**Option C — Documentation-only.** Publish the audit and a fallback guide but change no code/config. Rejected: leaves the dangerous Class-C over-claim live and leaves the real `add_to_shipment` workflow gap unaddressed.

## Chosen Direction

**Option B.** Scoped into a covering feature with six tasks:

1. **T1 — Parity audit matrix (docs).** Publish the grounded MCP↔CLI parity matrix (this audit) as a durable reference under `docs/reviews/`, including the intentional CLI-only / MCP-only exception list with rationale. Deliverable (1) + the "documented intentional exceptions" half of (2).
2. **T2 — Correct the registry operation map + add a drift-detection test (config + test).** Fill the 9 stale `cli_command` columns, add the 3 missing `docs_*` ops, and rewrite the 3 over-claiming `link` rows so they no longer point at a non-existent command (mark MCP-only until the CLI lands). Add a reflection/enumeration drift-detection test (per 063-S) that fails when the registry op-map diverges from the real MCP-tool and CLI surfaces. Deliverable (2), and the durable guard that prevents recurrence.
3. **T3 — `backlogit shipment add` CLI mirroring `add_to_shipment` (code + test).** The highest-value true workflow gap: Stage/Ship shipment assembly currently has no CLI fallback. Deliverable (2) fill.
4. **T4 — Fold in `7ECBAC7E`: normalize CLI shipment-list items null→`[]` (code + test).** Call the existing `core.NormalizeShipmentItems` in the CLI `shipment list` handler and add a cross-surface guard test asserting `custom_fields.items` is always a JSON array on both CLI and MCP. Deliverable (2) fill.
5. **T5 — Discoverability + documented CLI-fallback guide (docs).** Ensure `metadata export-command-map` output is accurate against the corrected map, document grouped/consistent help conventions, and write a "when MCP is unavailable, use these CLI equivalents" guide keyed by MCP tool. Deliverables (3) + (4).
6. **T6 — `backlogit checkpoint create` CLI mirroring `create_checkpoint` (code + test).** Fill the asymmetric checkpoint gap flagged in Class D: `checkpoint list/get/resolve/cleanup` already had CLI fallbacks and only `create` was missing, breaking session-continuity checkpoint writes when MCP is degraded. Added during phase-1 execution as the sixth task (see Reconciliation note).

Deferred to a **phase-2 stash** (documented intentional deferral, not silent drop): `link` CLI command group (add/remove/list), `poll_hook_events`/`ack_hook_events`, `save_memory`, `append_comment`, `get_wit_metadata`/`list_types`/`list_templates`, `merge_sync`. Rationale: each is a net-new command surface warranting its own design + tests; none is on the critical path for the operator's stated motivation once the map is honest and `add_to_shipment` is filled.

## Decisions and Rationale

- **D1 (fill-all vs. fill-highest-value + defer):** Fill highest-value now, defer net-new surfaces. Keeps every task within the 2-hour rule and keeps the shipment coherent and reviewable; the deferral is documented (deliverable 2's "intentional exception" path), not a silent drop.
- **D2 (Class-C over-claim handling):** Make the registry honest **now** and add a drift-detection test; defer building the actual `link`/other CLI. The danger is that the map *lies*, not that the command is *missing* — correcting the map removes the danger immediately, and the drift test keeps it honest. Building the command is a separable phase-2 effort.
- **D3 (7ECBAC7E fold-in vs. link):** **Fold in** as task T4. It is squarely a CLI↔MCP parity gap (this feature's exact domain), E16F4664 explicitly names it as related-but-broader, and folding avoids duplicated scope. Practically, linking via CLI is impossible today anyway (Class-C: no `link` command), and no MCP tool access exists in this Stage session — so a fold-in is the clean choice. `7ECBAC7E` will be archived with a forward reference to T4.
- **D4 (null→`[]` minimal fix vs. core-shaper by construction):** **Minimal fix** — call the already-consolidated `core.NormalizeShipmentItems` in the CLI list handler + a cross-surface guard test. The larger "never-null by construction in `NewShipmentView`" refactor was already evaluated and deferred by the 17D29DDC plan-review (the `ShipmentView` embeds `*models.Artifact` by pointer as a pure read); re-litigating it is out of scope. Noted as evaluated-but-deferred.
- **D5 (discoverability strategy):** Reuse the existing `metadata export-command-map` / catalog machinery rather than inventing a new discovery surface; the corrected registry + drift test make the exported map trustworthy, and a keyed fallback guide gives agents a direct MCP-tool→CLI lookup.

## Open Questions (resolved)

- *Keep the `link` rows pointing at a future command or mark MCP-only?* → Mark MCP-only now (map must never lie); phase-2 flips them when the `link` CLI lands.
- *Include the never-null-by-construction refactor?* → No; deferred per prior plan-review, minimal handler fix suffices.

## Notes

- No hardening signals are expected: the work is additive CLI commands, a git-reversible config/registry correction, a consistency-improving normalization fully covered by a guard test, and docs. No security, auth, migration, destructive, or external-integration surface. The impl-plan will record `Requires plan hardening: no` with per-signal justification.
- The plan-review gate should trigger the **Agent-Native Parity Reviewer** persona (the plan is entirely about MCP-tool/CLI parity and agent fallback workflows).

## Reconciliation (2026-07-03, Stage — stash `2827CB5F`)

Post-ship audit of stash `2827CB5F` found three drifts between this decision record (written pre-execution) and what shipment **078-S** actually delivered. Corrected in place (Stage owns deliberation-record edits per P-010), with the original claims noted here for auditability:

1. **Parity-matrix location.** T1 originally said the matrix would live under `docs/cli-reference/`; it shipped at `docs/reviews/2026-07-03-cli-mcp-parity-matrix.md` (`docs/cli-reference/` is reserved for generated command references guarded by the cli-reference-drift gate). Corrected T1 to `docs/reviews/`.
2. **Task count 5 → 6.** The scope was originally enumerated as "five tasks" (T1–T5). A sixth task (`checkpoint create` CLI) was added during execution. Corrected the count and added T6.
3. **`checkpoint create` shipped, not deferred.** The original deferred-to-phase-2 list included `checkpoint create`; it was in fact built and shipped in 078-S as T6. Removed it from the deferred list and recorded it as T6.

These corrections do not alter the Option B decision or its rationale; they align the historical record with delivered reality. Phase-2 work (the remaining deferred surfaces) is tracked under stash `6C6ACE00` → feature `079-F` / shipment `079-S`.
