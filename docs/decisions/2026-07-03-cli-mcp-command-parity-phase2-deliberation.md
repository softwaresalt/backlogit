---
chunk_strategy: h1-h2-h3
description: 'Deliberation for stash 6C6ACE00 (phase-2 CLI/MCP parity) — evaluate each MCP-only tool deferred from 078-F for genuine CLI fallback value vs intentional-permanent MCP-only, build the high/medium-value fallbacks over their existing shared core paths (link, hooks, memory, comment, wit/types/templates), flip their registry rows guarded by the U2 drift test, and defer merge_sync + the export-command-map pairing enhancement with documented rationale.'
doc_type: decision
schema_version: "1.0"
source: docs/decisions/2026-07-03-cli-mcp-command-parity-phase2-deliberation.md
title: 'CLI/MCP command parity phase-2: build the deferred CLI fallbacks worth building'
stash_id: 6C6ACE00
decision_status: decided
promoted_to: plan
---

## Source

- Stash: `6C6ACE00` (kind=feature, priority=low) — "Phase-2 CLI/MCP parity: net-new command surfaces deferred from E16F4664 (feature 078-F phase-1)." Deferred scope from the 078-F Option-B deliberation, 2026-07-03.
- Predecessor: `docs/decisions/2026-07-03-cli-mcp-command-parity-deliberation.md` (078-F, Option B, decided) and its plan `docs/exec-plans/2026-07-03-cli-mcp-command-parity-plan.md` (plan-review PASS, shipped as 078-S / PR #170, merge `e2ab16c0e893d6bcb260162099b0d3f7e87530c2`).
- Prior learnings (compound, confidence: high):
  - `docs/compound/2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md` (078-S) — Rule 1: declare an HONEST fallback map, never fabricate a CLI command, lock it with a drift test. Rule 2: for each unmapped tool, **deliberately choose add vs defer**; when adding, harness-first with **output-shape parity** tests. Rule 4: a CLI fallback must be no more dangerous than the MCP default it mirrors (blast-radius parity).
  - `docs/compound/2026-06-27-cli-mcp-catalog-parity-via-di-and-index-consistency.md` — lock cross-surface parity with a test; the `CLICommandProvider` DI seam.
  - `docs/compound/manual-schema-registry-drift-detection-2026-05-22.md` (063-S) — a manually curated registry must be guarded by an enumeration drift test or it drifts silently.

## Problem Frame

Phase-1 (078-F) made the `.autoharness/backlog-registry.yaml` fallback map **honest**: it corrected 9 stale rows, added 3 missing `docs_*` rows, neutralized the dangerous Class-C `link` over-claim by marking those rows `mcp_only: true`, filled the two highest-value true gaps (`shipment add`, `checkpoint create`), and added the U2 drift test (`internal/cli/registry_parity_test.go`) that fails if any registered MCP tool lacks either a resolvable `cli_command` or an honest `mcp_only: true`.

That left a documented, intentional deferral: a set of MCP tools that are honestly marked `mcp_only: true` but *could* carry a CLI fallback. Phase-2's job is not "reach 100% parity for its own sake" — it is to **evaluate each deferred tool on its merits** and build the CLI fallbacks that genuinely help an agent operating in CLI-only mode (MCP outage / recovery / debugging), while keeping honestly-permanent MCP-only tools marked as such.

The deferred `mcp_only: true` set (registry lines 91–151, 319–392) and their grounded code paths:

| MCP tool | Handler | Shared core path (reuse target) | CLI-value read |
|---|---|---|---|
| `add_link` | `internal/mcp/links.go:15` | `core.AddArtifactLink` (`internal/core/artifacts.go:778`) | **HIGH** — closes the former dangerous Class-C over-claim honestly |
| `remove_link` | `internal/mcp/links.go:96` | `core.RemoveArtifactLink` (`internal/core/artifacts.go:821`) | **HIGH** |
| `get_links` | `internal/mcp/links.go:56` | **none** — handler reads `db.GetLinks` directly (needs a thin `core.GetLinks` extraction to reuse without duplication) | **HIGH** |
| `poll_hook_events` | `internal/mcp/hook_tools.go:68` | `events.PollHookEvents` (`internal/events/hook_reader.go:33`) | **MEDIUM** — drain/inspect the durable `.backlogit/hooks_queue.jsonl` during MCP outage |
| `ack_hook_events` | `internal/mcp/hook_tools.go:99` | `events.AckHookEvents` (`internal/events/hook_reader.go:73`) | **MEDIUM** |
| `save_memory` | `internal/mcp/tools.go:895` | `events.SaveMemory` (`internal/events/memory.go:16`) | **MEDIUM** — structured backlog-native memory in fallback mode |
| `append_comment` | `internal/mcp/tools.go:844` | **none** — handler builds `events.Event` + `AppendEvent` + `db.IndexEvent` directly (needs a thin `core.AppendComment` extraction) | **MEDIUM** |
| `get_wit_metadata` | `internal/mcp/tools.go:1127` | `core.DescribeType` (`internal/core/wit_metadata.go:53`) | **MEDIUM** — type introspection; natural under existing `metadata` group |
| `list_types` | `internal/mcp/tools.go:1152` | `core.ListTypes` (`internal/core/wit_metadata.go:109`) | **MEDIUM** |
| `list_templates` | `internal/mcp/dynamic.go:39` | `templates.Service.ListTemplates` (`internal/core/templates/service.go:175`) | **MEDIUM** |
| `merge_sync` | `internal/mcp/tools.go:796` | `db.MergeSync` (`internal/db/merge_sync.go:72`) | **LOW** — **writes by default** (`dryRun, _ := args["dry_run"].(bool)` → zero-value `false`); Rule-4 blast-radius care needed |
| `log_telemetry` | `internal/mcp/tools.go:871` | `s.Telemetry.LogTelemetry` (agent-internal event log) | **NONE** — intentional-permanent MCP-only |

Plus an **optional phase-2 enhancement** the 078-F plan named and deferred: extend `ToolInfo` + `RenderCommandMapMarkdown` so `metadata export-command-map` emits a per-tool `cli_command`/`mcp_only` field. **Key constraint (verified in 078-F plan-review):** nothing in `internal/` or `cmd/` reads `.autoharness/backlog-registry.yaml` — the binary is deliberately decoupled from the routing registry, and `export-command-map` renders two *disjoint* name lists (CLI commands, MCP tools) by design. Emitting the pairing would require either coupling the binary to the `.autoharness` registry (reversing that deliberate decision) or a heuristic name-match derivation.

## Research Findings

- **Reuse-the-core is achievable for every candidate**, but two (`get_links`, `append_comment`) currently have their business logic inline in the MCP handler with no core helper. Honoring "no logic duplication" means a small refactor: extract `core.GetLinks(ctx, ws, id, linkType)` and `core.AppendComment(...)` so both the MCP handler and the new CLI command call the same function. This is refactor-preserving (the MCP handler keeps its exact behavior) — the extraction is the safe way to add the CLI without a second copy of the logic (compound `2026-06-27-cli-mcp-catalog-parity`: supply cross-layer behavior once, lock with a test).
- **The U2 drift test is the worklist and the guard** (Rule 2). Flipping a row from `mcp_only: true` to a `cli_command` is asserted two ways: `TestRegistryParity_EveryCLICommandResolves` requires the `cli_command` to resolve to a real cobra command **and** to be mutually exclusive with `mcp_only`. So each new CLI command MUST land before/with its registry flip — the same ordering constraint 078-F hit for `shipment add`/`checkpoint create` before U2.
- **Output-shape parity is a test requirement, not a nicety** (Rule 2 + Rule 3). Each new CLI command's test must assert its success JSON mirrors the MCP sibling's result shape (e.g. `{source_id,target_id,link_type,status:"removed"}` for remove_link), and any collection field must serialize `[]` not `null`.
- **`merge_sync` writes by default.** Its MCP handler defaults `dry_run` to the bool zero-value `false`, i.e. it applies. A CLI fallback that mirrors that default would also apply by default — the highest-blast-radius member of the deferred set. Rule 4 (blast-radius parity) is satisfiable (default `--dry-run=false` to match MCP), but the write posture warrants its own careful slice + runtime verification rather than being folded into an otherwise additive/read-heavy shipment.
- **`log_telemetry`** has no shared core path and is agent-internal event logging with no operator/agent CLI workflow. It is honestly permanent MCP-only.
- **The command homes are already established:** new parent groups `link` and `hooks` follow the `stash`/`checkpoint` pattern (`internal/cli/stash.go:14`, `internal/cli/checkpoint.go:17`); `types`/`wit`/`templates` attach to the existing `metadata` group (`internal/cli/metadata.go:19`, `cmd.AddCommand(...)`).

## Options Evaluated

### Option A — Fill every deferred surface now, including merge_sync and the export-command-map pairing enhancement

Build all six command families (link, hooks, memory, comment, wit/types/templates, merge_sync) plus the `export-command-map` per-tool `cli_command`/`mcp_only` enhancement in one shipment. Reach literal 100% CLI parity minus `log_telemetry`.

- **Pros:** Maximal completeness; nothing left deferred except the one truly-permanent tool.
- **Cons:** Bundles the one write-by-default op (`merge_sync`, Rule-4 risk) with otherwise additive/read-heavy work, raising the shipment's blast radius and forcing plan-hardening it would otherwise not need. The export-command-map enhancement drags in the unresolved binary↔`.autoharness` decoupling design question (078-F deliberately kept the binary from reading the routing registry) — a design deliberation, not a mechanical gap-fill. Largest shipment, lowest coherence.
- **Effort:** high. **Fit:** poor — mixes risk tiers and an open design question into a gap-fill shipment.

### Option B — Value-tiered: build the high/medium-value fallbacks over clean shared core paths; defer merge_sync + the enhancement; keep log_telemetry permanent (CHOSEN)

Build the five families whose CLI fallback is genuinely valuable and reuses (or cheaply extracts) a shared core path: **link** (add/remove/list), **hooks** (poll/ack), **memory** (save), **comment** (add), and **metadata discovery** (types/wit/templates). Each is test-first with output-shape parity, and each flips its registry row from `mcp_only: true` to a resolvable `cli_command` guarded by the U2 drift test. **Defer** `merge_sync` (write-by-default, Rule-4 care + own verification slice, LOW value) and the `export-command-map` pairing enhancement (requires resolving the deliberate binary↔`.autoharness` decoupling) to a documented phase-3 stash. **Keep** `log_telemetry` honestly permanent MCP-only.

- **Pros:** Delivers the operator's phase-2 intent (net-new fallback surfaces) at a coherent, reviewable, additive-only size; every task stays within the 2-hour rule and single-skill-domain; no hardening signal remains in scope; the drift test keeps the flips honest. Deferrals are documented (Rule 2), not silent.
- **Cons:** Two follow-ups remain (merge_sync CLI, export-map enhancement) — but each genuinely warrants its own slice/deliberation.
- **Effort:** medium. **Fit:** strong — matches Rule 2 (deliberate add-vs-defer per tool) and Rule 4 (keep the risky write op out of an additive shipment).

### Option C — Minimal: build only the link group (retire the former dangerous over-claim), defer the rest

Build just the `link` CLI group and defer hooks/memory/comment/metadata/merge_sync.

- **Pros:** Smallest shipment; closes the single highest-value gap (the former Class-C danger).
- **Cons:** Under-delivers the operator's explicit phase-2 list; the medium-value families (hooks/memory/comment/metadata) are clean, low-risk core-path reuses that are cheap to close now — stopping at link forces yet another phase for work that fits comfortably here.
- **Effort:** low. **Fit:** weak — leaves cheap, honest wins on the table.

## Trade-off Comparison

| Criterion | Option A (all) | Option B (value-tiered) | Option C (link-only) |
|---|---|---|---|
| Blast radius | High — folds in write-by-default merge_sync | Low — additive + read-heavy only | Lowest |
| Requires plan hardening | Likely yes (merge_sync write) | **No** (per-signal justified) | No |
| Open design questions dragged in | Yes (export-map binary↔registry coupling) | No (deferred with rationale) | No |
| Delivers operator phase-2 intent | Over-delivers | **Yes** | Under-delivers |
| Shipment coherence / reviewability | Poor | **Strong** | Strong but thin |
| 2-hour-rule compliance per task | Strained (merge_sync + enhancement) | **Clean** | Clean |

## Decision

**Option B.** Scope a covering feature with **seven tasks**:

1. **U1 — `link` CLI group (code + test).** New `internal/cli/link.go` with `link add/remove/list`; extract `core.GetLinks` so the list path and the MCP `handleGetLinks` share one function. Reuse `core.AddArtifactLink`/`core.RemoveArtifactLink`. Register `link` under root. Output-shape parity + sentinel-error tests.
2. **U2 — `hooks` CLI group (code + test).** New `internal/cli/hooks.go` with `hooks poll/ack` reusing `events.PollHookEvents`/`events.AckHookEvents`. Poll emits `{events, derived_signals}`, ack emits `{acked_seq}` — mirroring the MCP shapes.
3. **U3 — `memory save` CLI (code + test).** `backlogit memory save --key --summary` reusing `events.SaveMemory` (writes `.backlogit/memories.json`); output `{ok:true}` parity.
4. **U4 — `comment add` CLI (code + test).** `backlogit comment add <item-id> --actor --comment` after extracting `core.AppendComment` (Event build + `AppendEvent` + `IndexEvent`) shared with the MCP handler; output `{ok:true}` parity.
5. **U5 — `metadata` discovery subcommands (code + test).** `metadata types`, `metadata wit <type>`, `metadata templates` attached to the existing `metadata` group, reusing `core.ListTypes`/`core.DescribeType`/`templates.Service.ListTemplates`.
6. **U6 — Registry flip + drift-test green (config).** Flip the 10 built rows (`add_link`, `remove_link`, `get_links`, `poll_hook_events`, `ack_hook_events`, `save_memory`, `append_comment`, `get_wit_metadata`, `list_types`, `list_templates`) from `mcp_only: true` to resolvable `cli_command` templates; `backlogit sync` after. The U2 drift test enforces every flip resolves and is mutually exclusive with `mcp_only`. **Depends on U1–U5** (commands must resolve first).
7. **U7 — Discoverability docs update (docs).** Update `docs/reviews/2026-07-03-cli-mcp-parity-matrix.md` and `docs/design-docs/2026-07-03-mcp-to-cli-fallback-guide.md` to reclassify the five newly CLI-backed families and keep `merge_sync`, `log_telemetry`, and the export-map enhancement flagged (deferred/permanent). **Depends on U6** (registry is the source of truth). Note: `docs/reviews/` and `docs/design-docs/` are **not** generated zones (unlike `docs/cli-reference/`), so hand edits are safe.

Each code unit (U1–U5) carries an implementation subtask and a test subtask so each subtask stays single-domain and test-first.

## Rejected Alternatives

- **Option A** rejected: folds the write-by-default `merge_sync` (Rule-4 blast radius) and the export-map binary↔registry design question into an otherwise additive gap-fill, raising risk and lowering coherence for no scheduling benefit.
- **Option C** rejected: leaves four cheap, honest, low-risk core-path reuses undone, forcing an extra phase for work that fits within this one.

## Unresolved Questions (deferred, not dropped)

- **merge_sync CLI (phase-3):** build `backlogit merge-sync --dry-run` with the default mirroring the MCP default (`false`) per Rule 4, plus runtime verification of the write path. Deferred to a phase-3 stash.
- **export-command-map pairing enhancement (phase-3):** decide whether to (a) couple the binary to `.autoharness/backlog-registry.yaml` (reversing the 078-F decoupling), (b) derive the pairing heuristically from name-matching, or (c) leave the pairing solely in the registry. Warrants its own deliberation. Deferred to a phase-3 stash.

## Risks and Mitigations

- **Registry flip ordering (U6 after U1–U5).** Mitigation: the plan's dependency graph makes U6 depend on all five command tasks; the U2 drift test fails loudly if a `cli_command` is flipped before its command resolves.
- **Inline-logic extraction regressions (`get_links`, `append_comment`).** Mitigation: extract refactor-preserving core helpers and assert the MCP handler behavior is unchanged; the new CLI test asserts output-shape parity with the MCP sibling.
- **Hook CLI supersedes a phase-1 doc note.** The 078-F fallback guide states poll/ack_hook_events "remain MCP-only." Mitigation: U7 updates that note when the CLI lands so the guide stays honest (no stale MCP-only claim).
- **No hardening signals in scope.** The impl-plan will record `Requires plan hardening: no` with a per-signal justification: additive CLI commands, git-reversible config flips, refactor-preserving core extractions, read-heavy introspection, and low-blast writes (`memory`/`comment`/`hooks ack`) that mirror existing MCP defaults. The one write-by-default op (`merge_sync`) is explicitly deferred to keep hardening out of scope.

## Notes

- Plan-review should trigger the **Agent-Native Parity Reviewer** persona: the plan is entirely about MCP-tool↔CLI parity and agent fallback workflows.
- Traceability: this deliberation carries `stash_id: 6C6ACE00`; a native `backlogit deliberate` artifact is created and linked to the stash for backlog-native recovery.
