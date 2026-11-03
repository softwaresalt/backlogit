---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for CLI/MCP command parity (E16F4664): publish a grounded parity audit matrix, make the backlog-registry fallback map honest and guard it with a drift-detection test, add the shipment add CLI (add_to_shipment fallback), fold in the 7ECBAC7E shipment-list null-vs-[] fix, and ship discoverability + a CLI-fallback guide.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-07-03-cli-mcp-command-parity-plan.md
title: 'CLI/MCP command parity: honest fallback map + highest-value gap fills'
---

## Source

- Deliberation: `docs/decisions/2026-07-03-cli-mcp-command-parity-deliberation.md` (decided, Option B).
- Stash: `E16F4664` (feature, medium). Native deliberation artifact: `050-DL`.
- Folded-in stash: `7ECBAC7E` (task, low) — CLI `shipment list` null-vs-`[]` parity gap.
- Prior learnings (compound, confidence: high): `2026-06-27-cli-mcp-catalog-parity-via-di-and-index-consistency.md` (Rule 1: lock parity with a test; `CLICommandProvider` DI seam), `go-patterns/manual-schema-registry-drift-detection-2026-05-22.md` (063-S drift-detection test), `go-patterns/f015-shipment-stash-patterns.md` (treat SQLite JSON arrays as lossy; normalize on the read edge).

## Problem Frame

The `.autoharness/backlog-registry.yaml` operation map is the contract an agent consults to fall back from an MCP tool to its CLI equivalent. Grounded against the running binary (`backlogit manifest` → 56 MCP tools, vs. the actual cobra CLI tree), the map has drifted three ways:

- **Stale (9):** `deliberate`, `get_metadata_catalog` (`metadata catalog`), `export_command_map` (`metadata export-command-map`), `get_version` (`version`), `telemetry_harvest` (`telemetry harvest`), `stash_get`/`stash_edit`/`stash_archive`/`stash_remove` — all have real CLI commands but empty registry columns.
- **Missing (3):** `docs_lint`/`docs_migrate`/`docs_scope` — MCP tools + CLI commands exist, but no registry rows.
- **Over-claim (3, dangerous):** `add_link`/`remove_link`/`get_links` map to `backlogit link ...`, but **no `link` CLI command exists** — an agent following the map runs a non-existent command.

True CLI gaps (no CLI) total 14; the highest-value is `add_to_shipment` (the shipment-assembly workflow has no CLI fallback). Separately, the CLI `shipment list` handler emits `items:null` for empty shipments while MCP emits `items:[]` (`7ECBAC7E`).

Confirmed code paths:
- `internal/core/shipment.go:545` — `func NormalizeShipmentItems(artifact *models.Artifact) []string` (exported shared shaper).
- `internal/core/shipment.go:178` — `func AddItemToShipment(ctx, ws, shipmentID, itemID) error` (shared mutation wrapped by MCP `handleAddToShipment`, `internal/mcp/tools.go:1649`).
- `internal/cli/shipment.go:109` — `newShipmentListCmd`; line 133 `db.QueryItems`; line 151 `core.NewShipmentViews(ctx, ws, shipments)` **without** normalizing items. Subcommands register at lines 28–33 (no `add`).
- `internal/core/metadata_catalog.go:163` — `func DescribeCLICommands(root *cobra.Command) []CommandInfo`; parity test `internal/cli/metadata_parity_test.go:24`; DI seam `internal/mcp/server.go:39` + `internal/cli/root.go:367`.

## Requirements Trace

| # | Source requirement (E16F4664) | Implementation action | Unit |
|---|---|---|---|
| R1 | (1) Parity audit/matrix of MCP tools vs CLI commands | Publish grounded matrix + per-gap-labeled exception list under `docs/reviews/` (NOT `docs/cli-reference/`, which is a generated zone) | U1 |
| R2 | (2) Fill parity gaps OR document intentional exceptions | Correct registry drift (stale/missing/over-claim); annotate every deferred MCP-only tool with a machine-checkable `mcp_only: true`; add drift-detection test driven from the MCP tool set | U2 |
| R3 | (2) Fill highest-value true gap | Add `backlogit shipment add` mirroring `add_to_shipment` (positional args, sentinel-error + output-shape parity) | U3 |
| R4 | (2) Close concrete parity bug `7ECBAC7E` | Normalize CLI `shipment list` items via `core.NormalizeShipmentItems` + cross-surface guard test | U4 |
| R5 | (3) Simple/clear discoverability | Guard export-command-map/catalog name lists against the live CLI+MCP surfaces via the U2 drift test; document grouped/consistent help conventions | U2, U5 |
| R6 | (4) Documented CLI fallback path | Write a fallback guide under `docs/design-docs/` that POINTS at `.autoharness/backlog-registry.yaml` (the guarded single source of truth) for the MCP→CLI mapping, not a hand-maintained parallel table | U5 |
| R7 | (2/4) Complete the checkpoint-lifecycle CLI fallback | Add `backlogit checkpoint create` mirroring `create_checkpoint` (sole missing verb; session-continuity during MCP outage) | U6 |

## Implementation Units

Each unit satisfies the 2-hour rule (<3 files, <5 functions, <4 test scenarios), width isolation, and an atomic milestone. Code units carry an implementation slice and a verification slice as subtasks so each subtask stays single-domain.

### U1 — Parity audit matrix document (docs)

- **Changes:** New `docs/reviews/2026-07-03-cli-mcp-parity-matrix.md` (doc_type `review`) containing the full grounded MCP↔CLI matrix (56 tools), the drift classification (stale/missing/over-claim/true-gap/CLI-only), and — for each of the 14 true gaps — an explicit label of either **`intentional MCP-only (permanent)`** (e.g. `log_telemetry`) or **`deferred to phase-2 stash`** so no gap reads as a silent omission. For each corrected/existing row, verify per-row that each registry `params:` / templated `{{param}}` maps to a real flag or positional arg on the target command (flag-level parity, not just command existence).
- **Rationale for location:** `docs/cli-reference/` is a **generated** zone — `cmd/gen-docs` (`refDocRelDir = "docs/cli-reference"`) deletes every non-`README.md` file there and `cli-reference-drift.yml` gates it, so a hand-authored file there would be deleted and fail CI (compound: `cli-reference-drift-check-manual-edits-bypass-gen-docs-2026-04-25.md`). `docs/reviews/` classifies as `review` and is not generated.
- **Files:** 1 (new doc).
- **Tests/verification:** `go run ./cmd/backlogit docs lint --path docs/reviews/2026-07-03-cli-mcp-parity-matrix.md` → 0 violations.
- **Execution posture:** docs-first.
- **Acceptance criteria:** Matrix lists every one of the 56 MCP tools with its real CLI command or an explicit gap classification; each true gap is labeled intentional-permanent vs phase-2-deferred with a one-line rationale; each corrected row's params/flags are confirmed to exist on the target command; doc passes docline lint with 0 violations.

### U2 — Correct registry operation map + drift-detection test (config + test)

- **Changes:**
  1. `.autoharness/backlog-registry.yaml`: (a) add `cli_command` to the 9 stale ops; (b) add `docs_lint`/`docs_migrate`/`docs_scope` rows; (c) rewrite the 3 `link` rows to remove the `backlogit link ...` `cli_command`; (d) add a **machine-checkable `mcp_only: true` marker** to **every** registered MCP tool that is not CLI-backed. The authoritative completeness driver is the red drift test (below) enumerating the live tool set — not a hand-count; the currently-known deferred set is illustrative: the 3 link ops, `create_checkpoint` (flips to a `cli_command` once U6 lands), `poll_hook_events`, `ack_hook_events`, `save_memory`, `append_comment`, `get_wit_metadata`, `list_types`, `list_templates`, `merge_sync`, `log_telemetry`. Preserve existing key/row ordering; touch only affected rows.
  2. New drift-detection test `internal/cli/registry_parity_test.go` that **drives enumeration from the authoritative MCP tool set** via the existing typed accessor `mcp.Server.ListTools()` (confirmed present at `internal/mcp/dynamic.go:52`, a defensive copy of the live registrations — NOT parsed `backlogit manifest` text) and the real CLI command paths (via `core.DescribeCLICommands`). The MCP-tool↔registry join keys on the registry's **`mcp_tool` field** (the `backlogit_`-prefixed name), not the semantic row key. Assertions: (i) every registered MCP tool has either a resolvable `cli_command` or `mcp_only: true` — catches Class-B "missing row" recurrence; (ii) every registry `cli_command` resolves to an existing cobra command — catches Class-C over-claim; (iii) no orphan registry `mcp_tool` references a name absent from `ListTools()`; (iv) discoverability consistency — the `metadata export-command-map` CLI-command and MCP-tool name lists (rendered from `DescribeCLICommands` + `ToolDefs`) stay a superset of / 1:1 with the registry's resolvable `cli_command` leaves and `mcp_tool` values, so the exported discovery artifact cannot silently drift from the live surfaces. (Note: `export-command-map` renders two disjoint name lists and does NOT itself carry the MCP→CLI pairing — the pairing's source of truth is the `.autoharness` registry.) Resolve the repo-root registry path via a walk-up for a root marker (`.autoharness`/`go.mod`), and fail the test loudly on any file-not-found/YAML-unmarshal error (never `_ =`). The intentional-exception allow-list (CLI-only Class-E set) is declared as a single named var with a comment naming U1's matrix as its source of truth.
- **Files:** 2 (registry YAML + test). `mcp.Server.ListTools()` already exists (`internal/mcp/dynamic.go:52`), so no new accessor is needed; `internal/cli` already imports `internal/mcp` (`root.go:21`) so enumeration is cycle-free.
- **Tests/verification:** the new drift test passes; `go test ./internal/cli/... ./internal/mcp/... ./internal/core/...`; `backlogit sync` after the YAML edit.
- **Execution posture:** characterization-first (write the drift test to fail against current drift, then correct the YAML until green).
- **Acceptance criteria:** No registry `cli_command` references a non-existent command; the 9 stale ops and 3 missing `docs_*` ops are present; every deferred MCP-only tool carries `mcp_only: true`; the drift test drives from the MCP tool set (so a future unmapped tool fails), asserts export-command-map/registry consistency, and fails on any future divergence.

### U3 — `backlogit shipment add` CLI (code + test)

- **Changes:** Add `newShipmentAddCmd()` to `internal/cli/shipment.go`, registered alongside the existing shipment subcommands (lines 28–33). Use **positional args `cobra.ExactArgs(2)`** (`<shipment-id> <item-id>`) to match the sibling `shipment get <id>` convention — do NOT offer both positional and flag forms (avoids precedence ambiguity and keeps the registry `cli_command` template unambiguous: `backlogit shipment add {{shipment_id}} {{item_id}}`). Call the existing `core.AddItemToShipment`; wrap surfaced errors with `fmt.Errorf("add item to shipment: %w", err)` matching the sibling `RunE` pattern (shipment.go:52,58,98). No new core logic. On success, emit a JSON result **isomorphic to MCP `handleAddToShipment`** (`{shipment_id, item_id, status:"added"}`).
- **Files:** 1 production (`shipment.go`) + 1 test file.
- **Tests/verification (3 scenarios):** (1) happy-path add → success + output-shape parity with MCP; (2) idempotent re-add of an item already in **this** shipment → no-op success (`core.AddItemToShipment` returns nil, shipment.go:188); (3) item already assigned to **another** shipment → error asserted via `errors.Is` on the shared sentinel (NOT string matching). `go test ./internal/cli/...`.
- **Execution posture:** test-first.
- **Acceptance criteria:** `backlogit shipment add <ship> <item>` adds via `core.AddItemToShipment`, wraps errors with context, and its success JSON mirrors the MCP tool result shape; error and idempotency behavior are locked with `errors.Is` assertions; `shipment add` appears in `shipment --help`; the registry row for `add_to_shipment` gains its `cli_command` (in U2, after this command exists — see Dependency Graph).

### U4 — Fold-in 7ECBAC7E: CLI shipment-list items null→`[]` (code + test)

- **Changes:** In `internal/cli/shipment.go` `newShipmentListCmd` (~line 151), normalize each queried shipment before `core.NewShipmentViews`, mirroring the MCP list adapter **verbatim** (`internal/mcp/tools.go:1554-1562`): guard the nil `CustomFields` map, then assign `shipment.CustomFields["items"] = core.NormalizeShipmentItems(shipment)`. `NormalizeShipmentItems` is a **pure read** (returns `[]string`, does not mutate), so the result MUST be assigned back and the nil-map guard preserved exactly. ~4 lines.
- **Files:** 1 production (`shipment.go`) + 1 test file (cross-surface guard).
- **Tests/verification:** cross-surface shape-consistency test asserting `custom_fields.items` is always a JSON array (never `null`) on **both** the CLI list JSON and the MCP list JSON, covering the empty-items edge; `go test ./internal/cli/...`.
- **Execution posture:** characterization-first (assert current `null` leak, then fix).
- **Acceptance criteria:** CLI `shipment list --format json` never emits `items:null` for empty shipments; the guard test covers the empty-items edge and asserts array-shape parity across CLI and MCP (so the invariant survives a future third consumer of `NewShipmentViews`).
- **Design-debt note (D4):** the durable fix is for `NewShipmentViews` to clone-and-normalize items internally (operating on a copy sidesteps the pointer-mutation objection from 17D29DDC), eliminating the per-caller copy. Deferred per the prior plan-review; recorded here so the debt is explicit, not silent.

### U5 — Discoverability + CLI-fallback guide (docs)

- **Changes:** New `docs/design-docs/2026-07-03-mcp-to-cli-fallback-guide.md` (doc_type `design`; NOT `docs/cli-reference/`, which is generated). The guide is **narrative + a pointer to the single source of truth**, not a second hand-maintained mapping table. The authoritative MCP→CLI fallback mapping lives in **`.autoharness/backlog-registry.yaml`** (each operation's `mcp_tool` + `cli_command`/`mcp_only`), which the U2 drift test guards — the guide directs the agent there for the mapping. It separately references `backlogit metadata export-command-map` / `metadata catalog` as a **command/tool discoverability** aid (deliverable 3) — enumerating what CLI commands and MCP tools exist — while being explicit that those artifacts render two disjoint lists and do NOT themselves encode the MCP→CLI pairing (the binary does not read the `.autoharness` registry). The guide documents grouped/consistent help conventions, names per tool-class where **no CLI fallback exists** (`mcp_only: true` tools) so an agent never infers a phantom CLI path, and notes that hook events persist durably in `.backlogit/hooks_queue.jsonl` for post-recovery consumption while `poll/ack_hook_events` remain MCP-only.
- **Rationale:** A hand-maintained parallel mapping would reproduce the exact 063-S drift anti-pattern this feature fixes. The `.autoharness` registry (guarded by the U2 drift test) is the one source of truth for the pairing; the guide references it rather than duplicating it. (Optional phase-2 enhancement, out of scope here: extend `ToolInfo` + `RenderCommandMapMarkdown` to emit a per-tool `cli_command`/`mcp_only` field sourced from the operation map, so the exported command-map artifact itself carries the pairing.)
- **Files:** 1 (new doc).
- **Tests/verification:** `go run ./cmd/backlogit docs lint --path docs/design-docs/2026-07-03-mcp-to-cli-fallback-guide.md` → 0 violations. (The registry-vs-live-surface consistency assertions live in the U2 drift test, not a manual step here.)
- **Execution posture:** docs-first.
- **Acceptance criteria:** Guide directs agents to `.autoharness/backlog-registry.yaml` (the guarded source) for the authoritative MCP→CLI mapping, references `export-command-map`/`metadata catalog` for command discoverability without overclaiming they carry the pairing, names the `mcp_only` tools that have no CLI fallback, documents the hook-queue posture, and passes docline lint; it contains no hand-maintained per-tool mapping table that could drift.

### U6 — `backlogit checkpoint create` CLI (code + test)

- **Changes:** Add `newCheckpointCreateCmd()` to the existing checkpoint command group (`internal/cli/checkpoint.go`), mirroring the MCP `create_checkpoint` handler and calling the same shared core/state-write path the MCP handler uses. This completes the checkpoint lifecycle on the CLI (`list`/`get`/`resolve`/`cleanup` already exist; `create` is the sole missing verb) so an agent operating in CLI-fallback mode during an MCP outage can persist session state — directly serving the operator's "fallback when MCP unavailable" motivation.
- **Files:** 1 production (`checkpoint.go`) + 1 test file.
- **Tests/verification:** create writes a schema-valid checkpoint recoverable by `checkpoint get`/`list`; error wrapping with context; `go test ./internal/cli/...`. In U2, the registry `create_checkpoint` row flips from `mcp_only: true` to a resolvable `cli_command` (ordering: U6 → U2's `create_checkpoint` row, same constraint as U3).
- **Execution posture:** test-first.
- **Acceptance criteria:** `backlogit checkpoint create` writes a checkpoint that `checkpoint get`/`list` can read back; appears in `checkpoint --help`; the drift test sees a resolvable `cli_command` for `create_checkpoint` (no longer `mcp_only`).

## Dependency Graph

- U1 (audit) → informs U2 and U5. U1 has no code dependency and can start first.
- U3 (`shipment add`) must land **before** U2 sets the `add_to_shipment` `cli_command` row (drift test asserts `cli_command`s resolve). Likewise U6 (`checkpoint create`) must land **before** U2 flips the `create_checkpoint` row from `mcp_only` to a `cli_command`.
- **Shared-file coordination:** U3 and U4 both edit `internal/cli/shipment.go` (different functions: `newShipmentAddCmd` registration vs `newShipmentListCmd` body). They must not be executed in parallel; the suggested sequential order handles this.
- U4 is otherwise independent (list handler only).
- U5 depends on U1 (content) and U2 (accurate registry the guide points at).

Suggested order: U1 → U3 → U4 → U6 → U2 → U5. No cycles.

## Constitution Check

Mapping each unit against the workspace constitution (`.github/instructions/constitution.instructions.md`):

- **I (Code quality / error handling):** U3/U6 wrap surfaced errors with `fmt.Errorf("...: %w", err)` and never discard returned errors; tests assert via `errors.Is` on shared sentinels. Quality gates (`go test`, `go vet`, `golangci-lint`, docline) run before handoff. *gofmt note under Runtime Verification.*
- **II (Test-first):** every code unit (U3, U4, U6) and the config unit (U2) is characterization-/test-first; each materializes as distinct red-phase (test) and green-phase (code) subtasks at harvest so Width Isolation is real, not just labeled.
- **III/IV (Workspace-scoped, no traversal):** all writes are inside the repo tree (`docs/reviews/`, `docs/design-docs/`, `.autoharness/`, `internal/cli/`); no path traversal; the drift test resolves the registry via a repo-root-marker walk-up.
- **V (No secrets / no destructive irreversible steps):** none; the registry edit and stash archive are git-revertible.
- **VI (Reuse over duplication / no new deps):** U3→`core.AddItemToShipment`, U4→`core.NormalizeShipmentItems`, U2→`core.DescribeCLICommands` + the existing `mcp.Server.ListTools()`; no new third-party dependency; U4's residual per-caller copy is recorded as explicit design debt (D4).
- **VII (Diff-first for config):** the registry correction touches only affected rows and preserves existing key/row ordering (IX).
- **IX (Stable file formats):** registry edit preserves sorted/stable ordering to minimize merge conflicts.
- **Governance (2-hour rule / width isolation):** six units, each <3 files / <5 functions / <4 test scenarios, single-domain at the subtask grain.

No justified violations required; no principle is subordinated except the environment-specific gofmt note below, which is recorded as a conflict-resolution rather than a silent skip.

## Decisions and Rationale

- **Reuse shared core, don't duplicate.** U3 calls `core.AddItemToShipment`; U4 calls `core.NormalizeShipmentItems`; U2 reuses `core.DescribeCLICommands`. All avoid re-implementing logic on the CLI surface (the exact anti-pattern that produced the `7ECBAC7E` divergence). Consistent with the config-parity and catalog-parity compound rules.
- **Guard the map with a test, not just a one-time edit.** Per 063-S, a curated registry drifts silently without a drift-detection test. U2 pairs the correction with a guard **driven from the MCP tool set** so all three drift classes (stale, missing, over-claim) cannot silently recur — the highest-leverage part of the plan.
- **Honest-now, build-later for the deferred set.** Correcting the over-claim and adding `mcp_only: true` markers removes the danger immediately; building the `link` CLI and other net-new surfaces is deferred to phase-2 (documented, not dropped).
- **Pull `checkpoint create` into phase-1 (U6).** It is the sole missing verb in an otherwise-complete checkpoint lifecycle and directly serves the "fallback during MCP outage" motivation (session continuity). Small and coherent; worth the extra unit.
- **Minimal null→`[]` fix.** The never-null-by-construction refactor was already deferred by the 17D29DDC plan-review; the handler-level normalization + cross-surface guard test is the 2-hour-sized, low-risk fix (design debt recorded in U4).

## Risks and Caveats

- **R-1 (low): Drift test coverage boundary.** The drift test asserts command **existence** and `mcp_only` completeness, not per-flag parity. Mitigation: U1's audit verifies flag-level parity per row; the boundary is documented so operators know the guard stops at command existence.
- **R-2 (low): `add_to_shipment` / `create_checkpoint` row ordering.** If U2 sets those `cli_command`s before U3/U6 merge, the drift test would reference not-yet-existing commands. Mitigation: enforce U3→U2 and U6→U2 ordering (captured above); Ship lands the commands first.
- **R-3 (low): CLI JSON shape change is agent-visible.** U4 changes CLI `shipment list` empty output from `null` to `[]` — a consistency improvement (matches MCP), fully covered by the guard test; no consumer should depend on `null`.
- **R-4 (low): registry path resolution in tests + index.** `go test` CWD is the package dir, not repo root — resolve `.autoharness/backlog-registry.yaml` via a root-marker walk-up and fail loudly on not-found/unmarshal. Run `backlogit sync` after the YAML edit.
- **R-5 (low): guide/registry drift.** Mitigated by design — U5 points the agent at the guarded `.autoharness/backlog-registry.yaml` for the MCP→CLI mapping rather than hand-maintaining a parallel table (avoids the 063-S anti-pattern); the U2 drift test keeps that registry honest against the live CLI+MCP surfaces.

## Plan Hardening Signals

- Public API, schema, or contract change: **present (low-risk, additive).** New CLI subcommands (`shipment add`, `checkpoint create`) are additive; the registry op-map correction makes an agent-facing contract *more* accurate; the CLI `items` JSON goes `null`→`[]` (consistency fix). None removes or breaks an existing contract. Justification: additive + consistency-improving, fully test-covered.
- Security, auth, permission, or compliance-sensitive behavior: **absent.** No auth, secrets, or permission surface touched.
- Migration, backfill, destructive, or irreversible step: **absent.** The registry edit is a git-tracked text change (revertible); no data migration or destructive action.
- External integration, operator checkpoint, or external dependency: **absent.** No external systems; all changes are local CLI/config/docs.
- High runtime, rollout, or rollback risk: **absent.** Additive CLI + a normalization already proven in the MCP path; rollback is a git revert.

**Requires plan hardening: no.** The single contract-change signal is additive and consistency-improving with full test coverage; no destructive, security, migration, or external-dependency risk is present. Plan-review's Agent-Native Parity Reviewer persona is nonetheless expected to be triggered given the MCP/CLI parity subject matter.

## Runtime Verification and Closure

- **Runtime surfaces changed:** CLI (`shipment add`, `checkpoint create`, `shipment list` output). No API/UI/background-job surface.
- **Runtime verification before absorption:** `backlogit shipment add` adds an item visible in `shipment get`; `backlogit checkpoint create` writes a checkpoint readable by `checkpoint get`/`list`; `backlogit shipment list --format json` on an empty shipment shows `items:[]`; the drift test and cross-surface guard test pass under `go test ./...`; `go vet ./...` and `golangci-lint run` clean.
- **gofmt gate (conflict-resolution note, not a skip):** the mandated `gofmt -l .` gate is honored on CI, which runs on an LF checkout and is authoritative. On this Windows working copy `gofmt -l` false-positives on every `.go` file due to CRLF line endings (a local line-ending-config artifact, not a formatting defect), so Ship must NOT mass-reformat in response to it. The durable fix (a `.gitattributes` `*.go eol=lf` normalization) is out of scope for this feature and recorded here as known debt; per operator standing guidance, `go vet` + `golangci-lint` + CI (LF) are the authoritative local signals for this checkout.
- **Operational closure:** No monitoring/rollback infra needed (local CLI/config/docs). Closure = merged PR with green CI (`go test`, `go vet`, `golangci-lint`, docline gate) and the new guard tests present. Ownership: backlogit maintainers. Validation window: single CI run.

<!-- plan-review-attempt: 1 -->

## Plan Review

**Gate verdict: PASS** (attempt 2). Multi-persona plan-review gate. Attempt 1 FAILed (1 P0 + 1 P1); the plan was revised and re-reviewed. All blocking findings are resolved.

### Personas engaged

- Constitution Reviewer, Go Reviewer, Scope Boundary Auditor, Architecture Strategist, Agent-Native Parity Reviewer, Learnings Researcher (attempt 1, 6 personas in parallel).
- Learnings Researcher, Architecture Strategist, Agent-Native Parity Reviewer (attempt 2 re-review of the three findings-bearing dimensions).

### Findings by severity and resolution

- **P0 (attempt 1) — Generated-docs collision (Learnings Researcher):** the parity matrix and fallback guide were originally slated for `docs/cli-reference/`, which `cmd/gen-docs` regenerates and `cli-reference-drift.yml` guards — a hand-authored file there would be deleted or fail CI. **Resolved:** U1 matrix relocated to `docs/reviews/` (doc_type `review`), U5 guide to `docs/design-docs/` (doc_type `design`) — both non-generated, in docline scope. Re-review: **PASS**.
- **P1 (attempt 1) — Enumeration source not authoritative (Architecture Strategist):** the drift test originally proposed parsing `backlogit manifest` text for the MCP tool set. **Resolved:** U2 now drives enumeration from the live typed accessor `mcp.Server.ListTools()` (`internal/mcp/dynamic.go:52`) and joins on the registry `mcp_tool` field; `internal/cli`→`internal/mcp` import is already established (cycle-free). Re-review: **PASS**.
- **P1 (attempt 2) — export-command-map cannot carry the mapping (Agent-Native Parity Reviewer):** U5/U2(iv)/R5 pointed the agent at `backlogit metadata export-command-map` for the MCP→CLI mapping, but that command reads the `.backlogit` routing registry (verified: nothing in `internal/`/`cmd/` reads `.autoharness/backlog-registry.yaml`) and `RenderCommandMapMarkdown` emits two disjoint lists (`## CLI Commands` line 283, `## MCP Tools` line 296) with no pairing. **Resolved:** repointed U5/R6 at `.autoharness/backlog-registry.yaml` as the guarded single source of truth for the pairing; demoted export-command-map/catalog to a discoverability aid with an explicit disclaimer they do not encode the pairing; restated U2 assertion (iv) as a discoverability-consistency check; noted the pairing-carrying render as an optional out-of-scope phase-2 enhancement. Re-review: **PASS**.
- **P2/P3 (advisory, incorporated):** Constitution Check section added; U2 stated as 2 files (ListTools already exists); join keys documented on the `mcp_tool` field; deferred `mcp_only` set driven by the red drift test rather than a hand-count; gofmt note reframed as a conflict-resolution note (not a gate downgrade); U6 `checkpoint create` pulled into phase-1 as the sole missing checkpoint verb; section ordering normalized.

### Gate outcome

No P0 or P1 findings remain open. The plan proceeds to harvest.

<!-- plan-review-attempt: 2 -->


