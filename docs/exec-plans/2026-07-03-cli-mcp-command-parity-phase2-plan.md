---
chunk_strategy: h1-h2-h3
description: 'Implementation plan for CLI/MCP command parity phase-2 (6C6ACE00): build the deferred CLI fallbacks worth building — link (add/remove/list), hooks (poll/ack), memory save, comment add, and metadata discovery (types/wit/templates) — each over its existing shared core path (extracting core.GetLinks and core.AppendComment where the logic is currently inline), flip the ten registry rows from mcp_only to resolvable cli_command guarded by the U2 drift test, and update the parity matrix + fallback guide; merge_sync and the export-command-map pairing enhancement are deferred with documented rationale, log_telemetry stays permanent MCP-only.'
doc_type: plan
schema_version: "1.0"
source: docs/exec-plans/2026-07-03-cli-mcp-command-parity-phase2-plan.md
title: 'CLI/MCP command parity phase-2: build the deferred CLI fallbacks worth building'
---

## Source

- Deliberation: `docs/decisions/2026-07-03-cli-mcp-command-parity-phase2-deliberation.md` (decided, Option B). Native deliberation artifact: `051-DL` (linked to stash `6C6ACE00`).
- Stash: `6C6ACE00` (feature, low) — phase-2 net-new CLI command surfaces deferred from `078-F`.
- Predecessor shipped: `078-S` / PR #170 (merge `e2ab16c0e893d6bcb260162099b0d3f7e87530c2`) — honest fallback map + U2 drift test.
- Prior learnings (compound, confidence: high): `2026-07-03-cli-mcp-honest-fallback-map-and-registry-drift-test.md` (Rule 1 honest map + drift test; Rule 2 deliberate add-vs-defer + output-shape-parity tests; Rule 3 arrays never null; Rule 4 fallback blast-radius parity), `2026-06-27-cli-mcp-catalog-parity-via-di-and-index-consistency.md` (supply cross-layer behavior once; lock with a test), `manual-schema-registry-drift-detection-2026-05-22.md` (registry must be enumeration-guarded).

## Problem Frame

The `.autoharness/backlog-registry.yaml` operation map is the MCP→CLI fallback contract. Phase-1 made it honest and marked twelve tools `mcp_only: true`. Ten of those have a clean shared core path and a genuine CLI-fallback use (agent operating in CLI-only mode during MCP outage, recovery, or debugging); one writes by default (`merge_sync`) and one is agent-internal (`log_telemetry`). Phase-2 builds CLI commands for the ten, reusing the existing core path so there is no logic duplication, and flips their registry rows — the U2 drift test (`internal/cli/registry_parity_test.go`) asserts each flipped `cli_command` resolves to a real cobra command and is mutually exclusive with `mcp_only`.

Confirmed code paths (grounded against the v1.2.0 binary tree):

- Links: `internal/mcp/links.go:15` `handleAddLink` → `core.AddArtifactLink` (`internal/core/artifacts.go:778`); `:96` `handleRemoveLink` → `core.RemoveArtifactLink` (`internal/core/artifacts.go:821`); `:56` `handleGetLinks` → **inline** `db.GetLinks`/`db.GetLinksByType` (no core helper).
- Hooks: `internal/mcp/hook_tools.go:68` `handlePollHookEvents` → `events.PollHookEvents` (`internal/events/hook_reader.go:33`); `:99` `handleAckHookEvents` → `events.AckHookEvents` (`internal/events/hook_reader.go:73`). Durable queue: `.backlogit/hooks_queue.jsonl`; consumer checkpoints under `.backlogit/runtime/hooks/{consumer}.checkpoint.json`.
- Memory: `internal/mcp/tools.go:895` `handleSaveMemory` → `events.SaveMemory` (`internal/events/memory.go:16`), path `.backlogit/memories.json`.
- Comment: `internal/mcp/tools.go:844` `handleAppendComment` → **inline** `events.Event{}` build + `s.Events.AppendEvent` + `db.IndexEvent` (no core helper).
- WIT/types/templates: `internal/mcp/tools.go:1127` `handleGetWITMetadata` → `core.DescribeType` (`internal/core/wit_metadata.go:53`); `:1152` `handleListTypes` → `core.ListTypes` (`internal/core/wit_metadata.go:109`); `internal/mcp/dynamic.go:39` `handleListTemplates` → `templates.Service.ListTemplates` (`internal/core/templates/service.go:175`).
- CLI wiring: root registration `internal/cli/root.go` (`root.AddCommand(...)`); parent-group pattern `internal/cli/stash.go:14`, `internal/cli/checkpoint.go:17`; `metadata` group `internal/cli/metadata.go:19` (`NewMetadataCmd`, subcommands via `cmd.AddCommand(...)`).
- Drift test: `internal/cli/registry_parity_test.go` — `TestRegistryParity_EveryMCPToolMappedOrDeferred` (every tool mapped or `mcp_only`), `TestRegistryParity_EveryCLICommandResolves` (`cli_command` resolves AND mutually exclusive with `mcp_only`); `cliOnlyIntentional` allow-list at line 57.
- Deferred (out of scope): `internal/mcp/tools.go:796` `handleMergeSync` → `db.MergeSync`, `dryRun` defaults to bool zero-value `false` (**writes by default**); `internal/mcp/tools.go:871` `handleLogTelemetry` (agent-internal, permanent MCP-only).

## Requirements Trace

| # | Source requirement (6C6ACE00) | Implementation action | Unit |
|---|---|---|---|
| R1 | `link` CLI group (add/remove/list) mirroring add_link/remove_link/get_links | New `link` parent + 3 subcommands over `core.AddArtifactLink`/`core.RemoveArtifactLink` + extracted `core.GetLinks` | U1 |
| R2 | poll_hook_events + ack_hook_events CLI | New `hooks` parent + `poll`/`ack` over `events.PollHookEvents`/`events.AckHookEvents` | U2 |
| R3 | save_memory CLI | `memory save` over `events.SaveMemory` | U3 |
| R4 | append_comment CLI | `comment add` over extracted `core.AppendComment` | U4 |
| R5 | get_wit_metadata / list_types / list_templates CLI | `metadata types`/`metadata wit`/`metadata templates` over `core.ListTypes`/`core.DescribeType`/`templates.ListTemplates` | U5 |
| R6 | Flip each built row from mcp_only to a resolvable cli_command guarded by the U2 drift test | Edit the 10 registry rows; `backlogit sync`; drift test green | U6 |
| R7 | Evaluate merge_sync (CLI value) vs intentional-permanent MCP-only (log_telemetry) | merge_sync **deferred** (write-by-default, Rule-4); log_telemetry **kept** `mcp_only` with rationale; both recorded in the matrix/guide | U7 |
| R8 | Discoverability: keep the matrix + fallback guide honest as gaps close | Update `docs/reviews/...parity-matrix.md` + `docs/design-docs/...fallback-guide.md` | U7 |
| R9 | Optional export-command-map pairing enhancement | **Deferred to phase-3** (requires resolving the binary↔`.autoharness` decoupling); recorded as an open question | — |
| R10 | (derived) New cobra commands must not break the `cli-reference-drift` CI gate | Regenerate `docs/cli-reference/` via `gen-docs` and commit | U8 |

## Implementation Units

Each unit satisfies the 2-hour rule (<3 files, <5 functions, <4 test scenarios), width isolation, and an atomic milestone. Code units carry an implementation slice and a verification slice as subtasks so each subtask stays single-domain.

### U1 — `link` CLI group (code + test)

- **Changes:** New `internal/cli/link.go`: `newLinkCmd()` parent (`Use: "link"`) with `add <source> <target> <type>`, `remove <source> <target> <type>`, `list <id> [--type]` subcommands (positional args matching the `shipment get <id>` sibling convention; `list` carries the optional `--type` filter mirroring the MCP `link_type` arg). `add`/`remove` call `core.AddArtifactLink`/`core.RemoveArtifactLink`. For `list`, extract a thin **`core.GetLinks(ctx, ws, id, linkType)`** into `internal/core/artifacts.go` that wraps the `db.GetLinks`/`db.GetLinksByType` branch (on `linkType != ""`) currently inline in `handleGetLinks`, **normalizes a nil result to `[]db.LinkEdge{}` inside the helper** (so both callers inherit the never-null guarantee, Rule 3), and refactor `handleGetLinks` to call it (refactor-preserving — MCP behavior unchanged; its now-redundant nil-guard may be removed). Register `link` under root in `internal/cli/root.go`. Success JSON mirrors the MCP handlers: add/remove `{source_id,target_id,link_type[,status:"removed"]}`, list `{id,links:[...]}` with `links` always an array (never `null`). Errors wrap with `%w` (per D6) so `errors.Is` stays viable at the CLI boundary.
- **Files:** **4 production** (largest unit): `internal/cli/link.go` (new), `internal/core/artifacts.go` (+`core.GetLinks`), `internal/mcp/links.go` (refactor `handleGetLinks` to delegate), `internal/cli/root.go` (1-line registration) + 1 test file (`internal/cli/link_test.go`). Function count (`newLinkCmd` + add/remove/list RunEs + `core.GetLinks`) sits at the ~5-function 2-hour boundary — this is the heaviest unit; if execution finds it tight, split the `core.GetLinks` extraction + `handleGetLinks` refactor into its own atomic slice.
- **Tests/verification (3 scenarios):** (1) `add` happy-path → success + output-shape parity with `handleAddLink`; (2) `remove` → `status:"removed"` shape parity; (3) `list` including the empty case → `links` is `[]` not `null` (Rule 3) and shape parity with `handleGetLinks`; plus a behavior-preservation assertion that `handleGetLinks` output is unchanged after delegating to `core.GetLinks`. `go test ./internal/cli/... ./internal/core/... ./internal/mcp/...` (the mcp package is included because U1 refactors an mcp handler).
- **Execution posture:** test-first.
- **Acceptance criteria:** `backlogit link add|remove|list` resolve under `link --help`; add/remove/list reuse `core.AddArtifactLink`/`core.RemoveArtifactLink`/`core.GetLinks` (no duplicated selection logic); `handleGetLinks` calls the same extracted `core.GetLinks`; `core.GetLinks` returns `[]` not `nil`; list output is always a JSON array; the MCP behavior-preservation assertion passes; all three scenarios pass.

### U2 — `hooks` CLI group (code + test)

- **Changes:** New `internal/cli/hooks.go`: `newHooksCmd()` parent (`Use: "hooks"`) with `poll --consumer-id <id>` → `events.PollHookEvents`, emitting `{events, derived_signals}`; and `ack --consumer-id <id> --seq <n>` → `events.AckHookEvents`, emitting `{acked_seq}`. Reuse the existing `events` functions verbatim — no new queue logic. Register `hooks` under root.
- **Files:** 1 production (`internal/cli/hooks.go` new; `root.go` 1-line add) + 1 test file (`internal/cli/hooks_test.go`).
- **Tests/verification (3 scenarios):** (1) `poll` on an empty queue → `events` is `[]` not `null` (Rule 3) + `derived_signals` shape parity with `handlePollHookEvents`; (2) `poll` returns appended events in seq order; (3) `ack --seq N` advances the consumer checkpoint, emits `{acked_seq:N}` mirroring `handleAckHookEvents`, and a subsequent `poll` excludes acked events. Seed the queue via the existing `events` append path in the test. `go test ./internal/cli/... ./internal/events/...`.
- **Execution posture:** test-first.
- **Acceptance criteria:** `backlogit hooks poll|ack` resolve; both reuse `events.PollHookEvents`/`events.AckHookEvents`; poll output collection fields are arrays never null; ack advances the durable checkpoint; scenarios pass.

### U3 — `memory save` CLI (code + test)

- **Changes:** New `internal/cli/memory.go`: `newMemoryCmd()` parent (`Use: "memory"`) with `save --key <k> --summary <s>` → `events.SaveMemory` (writes `.backlogit/memories.json`). Success JSON mirrors `handleSaveMemory` (`{ok:true}`). Register `memory` under root. Blast-radius parity (Rule 4): resolve the workspace root first (`core.NewWorkspace` → `ws.RootPath`) and build the `.backlogit/memories.json` path from it — **matching the MCP handler's resolved-root path (`tools.go:904`), not raw `*cwd`** — so a `save` invoked from a subdirectory writes to the correct `.backlogit`; the write mirrors the MCP default exactly (single keyed append, no destructive flag).
- **Files:** 1 production (`internal/cli/memory.go` new; `root.go` 1-line add) + 1 test file (`internal/cli/memory_test.go`).
- **Tests/verification (2 scenarios):** (1) `save` writes a readable entry to `.backlogit/memories.json` and returns `{ok:true}` shape parity; (2) missing `--key` or `--summary` → clear required-flag error (no partial write). `go test ./internal/cli/... ./internal/events/...`.
- **Execution posture:** test-first.
- **Acceptance criteria:** `backlogit memory save` resolves; reuses `events.SaveMemory`; output shape mirrors MCP; required-flag validation covered; scenarios pass.

### U4 — `comment add` CLI + `core.AppendComment` extraction (code + test)

- **Changes:** Extract **`core.AppendComment(ctx, ws, itemID, actor, comment, commitSHA string) error`** capturing the `events.Event{}` build + `AppendEvent` + `db.IndexEvent` sequence currently inline in `handleAppendComment` (`internal/mcp/tools.go:855-868`), and refactor `handleAppendComment` to call it (refactor-preserving). **Preserve the exact value-passing sequence** — the current code passes the same zero-`Timestamp` `event` value to `AppendEvent` (which stamps its own copy) and then to `db.IndexEvent`; the extraction must not pre-stamp `event.Timestamp`, or the indexed row's timestamp changes vs. today. Follow the existing `core.LinkCommit` template (`internal/core/commits.go:27-57`), which already performs the identical `events.NewEventWriter → AppendEvent → db.IndexEvent` sequence. New `internal/cli/comment.go`: `newCommentCmd()` parent (`Use: "comment"`) with `add <item-id> --actor <a> --comment <c> [--commit-sha <sha>]` → `core.AppendComment`, emitting `{ok:true}` parity. Register `comment` under root. Errors wrap with `%w` (D6).
- **Files:** **4 production** (`internal/core/*` +`core.AppendComment` — place next to `core.LinkCommit` in `commits.go`; `internal/cli/comment.go` new; `internal/cli/root.go` 1-line add; `internal/mcp/tools.go` handler now delegates) — 2 substantive edits + 2 one-line/delegation + 1 test file (`internal/cli/comment_test.go`).
- **Tests/verification (3 scenarios):** (1) `add` happy-path → comment persisted + indexed, `{ok:true}` shape parity with the MCP handler; (2) empty `--comment`/`--actor` or missing `<item-id>` → required-arg validation error (mirroring the MCP `item_id != ""` guard at `tools.go:848`) with no partial write — **note: neither surface validates that the item _exists_ (`db.IndexEvent` upserts with no existence check), so an unknown-but-nonempty item-id succeeds on both CLI and MCP symmetrically; the test asserts this parity rather than a nonexistent-item error, keeping the extraction behavior-preserving**; (3) MCP `handleAppendComment` still produces the identical persisted+indexed event (including timestamp) after delegating to `core.AppendComment`. `go test ./internal/cli/... ./internal/mcp/... ./internal/core/...`.
- **Execution posture:** test-first (write the CLI + delegation tests red, extract, then green).
- **Acceptance criteria:** `backlogit comment add` resolves; the append logic lives once in `core.AppendComment`, called by both CLI and the MCP handler (no duplication); output shape mirrors MCP; input-validation and behavior-preservation (incl. timestamp) scenarios pass; append tolerates unknown item-ids identically on both surfaces.

### U5 — `metadata` discovery subcommands (code + test)

- **Changes:** In `internal/cli/metadata.go`, add three subcommands to the existing `NewMetadataCmd` group via `cmd.AddCommand(...)`: `types` → `core.ListTypes` (JSON array of `WITMetadata`); `wit <type>` → `core.DescribeType` (`*WITMetadata`); `templates` → `templates.Service.ListTemplates` (JSON array). All read-only. Register nothing new at root (they hang off the existing `metadata` command).
- **Files:** 1 production (`internal/cli/metadata.go`) + 1 test file (`internal/cli/metadata_discovery_test.go`).
- **Tests/verification (3 scenarios):** (1) `metadata types` output shape parity with `handleListTypes`, including the empty case → `[]` not `null` (Rule 3; `core.ListTypes` can return a nil slice at `wit_metadata.go:116`); (2) `metadata wit <type>` output parity with `handleGetWITMetadata` incl. an unknown-type error path; (3) `metadata templates` output parity with `handleListTemplates` (array never null). Supply `core.ListTypes`/`DescribeType`'s layout arg from `ws.Config.QueueLayout` with the same feature/task/subtask nil-fallback the MCP `queueLayout()` helper uses (`tools.go:1113-1125`) so type parity cannot silently drift. `go test ./internal/cli/...`.
- **Execution posture:** test-first.
- **Acceptance criteria:** `metadata types|wit|templates` resolve under `metadata --help`; each reuses the existing `core`/`templates` function; array outputs are never null; scenarios pass.

### U6 — Registry flip + drift-test flag-parity assertion (config + test)

- **Changes:** In `.autoharness/backlog-registry.yaml`, flip the **10 built rows** from `mcp_only: true` to a resolvable `cli_command` template with `params`:
  - `add_link` → `backlogit link add {{source_id}} {{target_id}} {{link_type}}`
  - `remove_link` → `backlogit link remove {{source_id}} {{target_id}} {{link_type}}`
  - `get_links` → `backlogit link list {{id}}` (keep the optional `link_type` param mapped to the `--type` flag; see optional-param convention below)
  - `poll_hook_events` → `backlogit hooks poll --consumer-id {{consumer_id}}`
  - `ack_hook_events` → `backlogit hooks ack --consumer-id {{consumer_id}} --seq {{seq}}`
  - `save_memory` → `backlogit memory save --key {{key}} --summary {{summary}}`
  - `append_comment` → `backlogit comment add {{item_id}} --actor {{actor}} --comment {{comment}}` (keep the optional `commit_sha` param mapped to `--commit-sha`; rename the phase-1 `task_id` param key to `item_id` for cross-surface consistency with the MCP arg name)
  - `get_wit_metadata` → `backlogit metadata wit {{type}}`
  - `list_types` → `backlogit metadata types`
  - `list_templates` → `backlogit metadata templates`
  Remove the `mcp_only: true` marker on each. **Leave `merge_sync` and `log_telemetry` as `mcp_only: true`.** Preserve existing key/row ordering; touch only the flipped rows. Run `backlogit sync` after.
  - **Optional-param convention (honesty fix):** optional MCP args (`link_type`, `commit_sha`) map to optional CLI flags (`--type`, `--commit-sha`) that an agent appends **only when the mapped arg is non-empty**. Document this append convention in the registry (a YAML comment on the two rows) and in U7's fallback guide, so the contract is honest that these optional params are reachable via CLI — not orphaned. Do **not** leave a param in a row with no way to render it.
  - **Drift-test flag-parity assertion (the load-bearing fix):** the existing U2 drift test (`registry_parity_test.go`) only resolves the leading command **path** (`resolveCLIPath` truncates at the first `--`/`{{`) and asserts `mcp_only` mutual-exclusivity — it does **not** validate flag names or positionals, so a misspelled flag (e.g. `--consumer` vs `--consumer-id`) would pass green yet fail at agent runtime. Extend the drift test with a new assertion that, for every row carrying a `cli_command`, each literal `--flag` token in the template resolves to a real registered flag on the target cobra command, and each `{{positional}}` count matches the command's expected `Args`. This closes the exact gap that would otherwise make the fallback contract silently wrong — the whole point of the feature.
- **Files:** 1 config (`.autoharness/backlog-registry.yaml`) + 1 test (`internal/cli/registry_parity_test.go`, +1 assertion).
- **Tests/verification:** `go test ./internal/cli/...` — `TestRegistryParity_EveryCLICommandResolves` (each flipped `cli_command` resolves + no longer paired with `mcp_only`), the **new flag-parity assertion** (each `--flag`/positional in a flipped row resolves against its command), and `TestRegistryParity_EveryMCPToolMappedOrDeferred` (merge_sync/log_telemetry still honestly `mcp_only`); `backlogit sync` succeeds.
- **Execution posture:** characterization-first — the drift test already exists; flipping a row before its command resolves makes it fail, so land U6 only after U1–U5. Write the new flag-parity assertion first (red against a deliberately-wrong flag) to prove it guards, then flip the rows correctly to green.
- **Acceptance criteria:** the 10 rows carry resolvable `cli_command` templates whose flags/positionals **are now test-asserted** against the real commands (claim corrected — flag parity is test-enforced, not just eyeballed); optional `link_type`/`commit_sha` params carry a documented append convention; `merge_sync`/`log_telemetry` remain `mcp_only: true`; the full U2 drift test (incl. the new assertion) is green; `backlogit sync` indexes cleanly.

### U7 — Discoverability docs update (docs)

- **Changes:** Update two non-generated docs to reflect the closed gaps:
  1. `docs/reviews/2026-07-03-cli-mcp-parity-matrix.md` — reclassify `add_link`/`remove_link`/`get_links`, `poll_hook_events`/`ack_hook_events`, `save_memory`, `append_comment`, `get_wit_metadata`/`list_types`/`list_templates` from "deferred to phase-2" to CLI-backed (naming the phase-2 shipment); keep `merge_sync` (deferred phase-3, note write-by-default) and `log_telemetry` (intentional-permanent MCP-only).
  2. `docs/design-docs/2026-07-03-mcp-to-cli-fallback-guide.md` — update the `mcp_only` tool-class list (remove the ten now-CLI-backed tools, keep `merge_sync`/`log_telemetry`), and correct the note that `poll/ack_hook_events` "remain MCP-only" (they now have a CLI fallback; the durable `.backlogit/hooks_queue.jsonl` note stays).
- **Files:** 2 (both existing docs; both under non-generated zones — safe to hand-edit, unlike `docs/cli-reference/`).
- **Tests/verification:** `go run ./cmd/backlogit docs lint --path docs/reviews/2026-07-03-cli-mcp-parity-matrix.md` and `... --path docs/design-docs/2026-07-03-mcp-to-cli-fallback-guide.md` → 0 violations each.
- **Execution posture:** docs-first.
- **Acceptance criteria:** both docs reflect the ten newly CLI-backed tools; `merge_sync`/`log_telemetry` remain honestly flagged; no stale "poll/ack remain MCP-only" claim; both docs lint clean.

### U8 — Regenerate CLI reference (generated docs)

- **Changes:** U1–U5 add new cobra commands (`link`, `hooks`, `memory`, `comment`) and subcommands (`metadata types/wit/templates`). The `docs/cli-reference/` tree is machine-generated by `cmd/gen-docs` and the `cli-reference-drift.yml` CI gate runs `go run ./cmd/gen-docs docs/cli-reference` and **fails on any git diff**. New commands therefore produce new generated files absent from the tree, turning the gate red on the Ship PR unless regenerated. Run the repo's generation entrypoint (`make docs` / `go run ./cmd/gen-docs docs/cli-reference`) and commit the regenerated files. **Do not hand-author** anything under `docs/cli-reference/` — this is the legitimate machine-regeneration step, distinct from U7's hand edits to the non-generated `docs/reviews/`+`docs/design-docs/`.
- **Files:** generated output under `docs/cli-reference/` (machine-written; count varies with the new command set).
- **Tests/verification:** `go run ./cmd/gen-docs docs/cli-reference` produces no further diff after commit; the `cli-reference-drift` gate is green.
- **Execution posture:** migration-first (regenerate, then verify the drift gate is clean).
- **Acceptance criteria:** `docs/cli-reference/` contains the generated pages for the new commands; re-running gen-docs yields no diff; `cli-reference-drift` passes. **Depends on U1–U5** (commands must exist to be documented).

## Dependency Graph

```text
U1 (link)   ─┬─► U6 (registry flip + flag-parity drift assertion) ─► U7 (docs update)
U2 (hooks)  ─┤
U3 (memory) ─┤
U4 (comment)─┤
U5 (metadata)┴─► U8 (regenerate docs/cli-reference)
```

- U1–U5 are mutually independent (distinct files: `link.go`, `hooks.go`, `memory.go`, `comment.go`, `metadata.go`; U1 and U4 also touch an mcp handler file for their refactor-preserving extraction; the small `root.go` registration lines should be sequenced to avoid churn) and may proceed in any order.
- **U6 depends on U1–U5** — the drift test asserts each flipped `cli_command` resolves, so every command must exist first.
- **U7 depends on U6** — the registry is the source of truth the docs reference.
- **U8 depends on U1–U5** — the generated reference documents the new commands; it is independent of U6/U7 and may run in parallel with them once the commands exist.
- No cycles.

## Decisions and Rationale

- **D1 (build 5, defer 2, keep 1 permanent):** Rule 2 (deliberate add-vs-defer per tool). The five built families have clean shared core paths and a genuine CLI-fallback use; `merge_sync` writes by default (Rule-4 blast radius) and warrants its own verified slice; `log_telemetry` is agent-internal with no CLI workflow.
- **D2 (extract core helpers for get_links + append_comment):** their logic is currently inline in the MCP handler. "No logic duplication" (stash constraint) is honored by a refactor-preserving extraction so both surfaces call one function — not by copying the selection/append sequence into the CLI.
- **D3 (positional args for link, flags for the rest):** `link add|remove|list` take positional IDs to match the `shipment get <id>` sibling convention and keep the registry `cli_command` template unambiguous; `hooks`/`memory`/`comment` use flags because their inputs are named key/value pairs (`--consumer-id`, `--key`, `--actor`).
- **D4 (U6 last among code, U7 last overall):** the U2 drift test makes registry flips depend on resolvable commands; docs reference the registry.
- **D5 (defer the export-command-map enhancement):** 078-F verified the binary does not read `.autoharness/backlog-registry.yaml` and `export-command-map` renders two disjoint lists by design; carrying the pairing there is a design decision (couple vs. derive vs. leave-in-registry), not a mechanical gap-fill. Deferred to phase-3.
- **D6 (error-handling contract for the extracted helpers + CLI validation):** `core.GetLinks` and `core.AppendComment` return the underlying `db`/`events` error via `%w` (like `core.AddArtifactLink` at `artifacts.go:792`); CLI `RunE` wraps at the workspace-open boundary and around core/events calls with `fmt.Errorf("context: %w", err)` (matching the `stash.go`/`checkpoint.go` sibling pattern) so `errors.Is`/`As` stays viable; required-flag/positional validation uses clear sentinel-style messages and never ignores a returned error. The MCP handlers' existing `%v` tool-result boundary is unchanged (they return a tool result, not an error).

## Risks and Caveats

- **Registry-flip-before-command regression.** If U6 lands before a command resolves, the drift test fails. Mitigation: dependency graph + characterization-first posture on U6.
- **Extraction changes MCP behavior.** Mitigation: U1/U4 include a behavior-preservation assertion that the MCP handler output is unchanged after delegating to the extracted core function.
- **Windows CRLF `gofmt -l` false positives** on this checkout (078-S note). Mitigation: do not mass-reformat; rely on `go vet` + `golangci-lint` + CI (LF) as authoritative.
- **Do not author under `docs/cli-reference/`** (gen-docs regenerated zone gated by `cli-reference-drift.yml`). U7 edits only `docs/reviews/` and `docs/design-docs/`, which are not generated.
- **Hook CLI supersedes a phase-1 guide note.** U7 must remove the stale "poll/ack remain MCP-only" statement or the guide lies.

## Plan Hardening Signals (REQUIRED)

- **public API, schema, or contract change:** ABSENT. The CLI surface is purely additive (new commands/subcommands). The 10 registry-row flips are a config-contract change but git-reversible and guarded by the U2 drift test. The `core.GetLinks`/`core.AppendComment` extractions are refactor-preserving (MCP behavior unchanged). No breaking change to any existing signature.
- **security, auth, permission, or compliance-sensitive behavior:** ABSENT. Links, hooks, memory, comments, and metadata introspection are non-security surfaces; no auth/permission logic is touched.
- **migration, backfill, destructive data/config action, or irreversible step:** ABSENT. `memory save`, `comment add`, and `hooks ack` write to `.backlogit/` state but are additive, low-blast, git-tracked, and mirror existing MCP defaults exactly (Rule 4). No schema migration/backfill. The one write-by-default op (`merge_sync`) is explicitly **deferred**.
- **external integration, operator checkpoint, or external dependency:** ABSENT. All work is local to the binary + registry + docs.
- **high runtime, rollout, or rollback risk:** ABSENT. New additive CLI commands; rollback = revert the commit; the drift test guards the registry.

**Requires plan hardening: no.** Every signal is absent with a per-signal justification above; the single higher-blast-radius op (`merge_sync`) was deliberately deferred precisely to keep this shipment additive and hardening-free.

## Constitution Check

Mapping the plan against the workspace constitution (`.github/instructions/constitution.instructions.md`):

| Principle | Unit(s) | Assessment |
|---|---|---|
| I — Code quality / error handling | U1–U6 | Extracted helpers + CLI validation follow the `%w` wrapping / sentinel-error contract (D6); no ignored errors. |
| II — Test-first & verifiable exit state | U1–U8 | Every code unit is test-first with named scenarios and a `go test` exit state; U6 adds a flag-parity assertion so the registry contract is test-enforced (not eyeballed); U8's exit state is a clean `cli-reference-drift` gate. |
| IV — No out-of-tree writes | all | Every write target is in-tree: `internal/*`, `.autoharness/backlog-registry.yaml`, `docs/reviews/`, `docs/design-docs/`, generated `docs/cli-reference/`, and `.backlogit/` runtime state (`memories.json`, `hooks_queue.jsonl`, consumer checkpoints). No `..`/absolute/symlink escape. |
| V — Documentation lifecycle / gen-docs boundary | U7, U8 | U7 hand-edits only non-generated zones; U8 performs the legitimate machine regeneration of `docs/cli-reference/` and commits it so `cli-reference-drift` stays green. |
| VII — Destructive-action safeguards | (deferred) | Not triggered: the sole write-by-default op (`merge_sync`) is deferred; in-scope writes mirror MCP defaults (Rule 4). |
| Single source of truth / no parallel trackers | U6, U7 | The `.autoharness` registry remains the one MCP↔CLI pairing source, guarded by the drift test; U7 docs reference it rather than duplicating the mapping. |
| Quality gates | closure | Full gate chain enumerated in the closure section below. |

No principle is violated. The plan is purely additive with refactor-preserving extractions and in-tree writes only.

## Runtime Verification and Closure

Each of U1–U6 changes a runtime surface (the `backlogit` CLI and the fallback registry); U8 changes the generated CLI reference. Before the work is considered absorbed, Ship's runtime verification should prove, against the built binary:

- `backlogit link add|remove|list`, `hooks poll|ack`, `memory save`, `comment add`, and `metadata types|wit|templates` each run end-to-end and their JSON output matches the corresponding MCP tool result shape. Spot-check at least one per family, and additionally exercise every flipped row's **full flag set** (e.g. `hooks ack --seq`, `link list --type`, `comment add --commit-sha`) since flag correctness is the agent-fallback contract.
- The full quality-gate chain passes: `go build ./...`, `go vet ./...`, `golangci-lint run`, and `go test ./internal/cli/... ./internal/core/... ./internal/mcp/... ./internal/events/...` (incl. the U2 drift test with its new flag-parity assertion and the new per-family output-shape-parity tests). `gofmt -l .` is advisory-only on this CRLF checkout — CI (LF) is authoritative (078-S note); do not mass-reformat.
- `backlogit sync` succeeds after the U6 registry edit; `backlogit docs lint` reports 0 violations on the two U7 docs; and `go run ./cmd/gen-docs docs/cli-reference` produces no diff (the `cli-reference-drift` gate is green) after U8.

Operational closure artifact: a runtime-verification note (as in `docs/closure/2026-07-03-078-S-...-runtime-verification.md`) recording the per-family CLI↔MCP shape checks, the green drift test (incl. flag-parity), and the clean `cli-reference-drift` gate. No monitoring/rollback trigger beyond "revert the commit" is required given the additive, non-destructive surface.

<!-- plan-review-attempt: 1 -->

## Plan Review

**Gate decision: PASS** (advisory P2/P3 findings resolved in-plan before harvest).

Reviewed by five personas (Agent-Native Parity Reviewer, Scope Boundary Auditor, Go Reviewer, Architecture Strategist, Constitution Reviewer), each grounding the plan against the live codebase (MCP handlers, `internal/core`/`internal/events` imports, `internal/cli/registry_parity_test.go`, the gen-docs `cli-reference-drift` gate).

**Severity tally:** 0 P0, 0 P1, 6 P2, 12 P3. Per the plan-review severity scale, P2-only ⇒ ADVISORY; the actionable P2/P3 items were incorporated into the plan directly (Stage owns the planning artifact), raising the gate to PASS.

### Affirmations (all reviewers)

- **No import cycles.** `internal/core` already imports `internal/db` and `internal/events`; `internal/events` does not import `core`; `internal/core/templates` imports `core` (not vice-versa). Both extractions (`core.GetLinks`, `core.AppendComment`) are buildable and cycle-free. `core.AppendComment` has an exact existing template in `core.LinkCommit` (`internal/core/commits.go:27`).
- **Dependency ordering is correct.** U1–U5 → U6 (drift test fails if a `cli_command` is flipped before its command resolves) → U7; U8 depends on U1–U5. Acyclic.
- **Registry decoupling preserved (078-F).** The drift test reads `.autoharness` at test time only; D5 correctly defers the export-command-map pairing to avoid re-coupling the binary.
- **`Requires plan hardening: no` is defensible** — re-checked per-signal; no hidden security/migration/destructive/external/rollout signal; the sole write-by-default op (`merge_sync`) is deferred.
- **In-tree only (Principle IV)**; single-source-of-truth registry (no parallel trackers); deferrals are right-sizing, not scope-cutting; the two core extractions are necessary (no-duplication), not gold-plating.

### P2 findings — resolved in-plan

1. **Flag-parity overclaim** (Parity + Go + Constitution, 3× convergence). The plan claimed U6 flag/positional correctness was enforced "per the U2 test's params check", but `registry_parity_test.go` only resolves the leading command path (`resolveCLIPath` truncates at the first `--`/`{{`) and never asserts flag names — a misspelled flag would pass green yet fail at agent runtime. **Resolved:** U6 rewritten to (a) correct the claim and (b) add a **flag-parity drift-test assertion** (each `--flag`/positional in a flipped row must resolve against its cobra command), written red-first. This is the load-bearing fix — flag correctness is the agent-fallback contract.
2. **U4 scenario-2 self-contradiction** (Parity). `handleAppendComment` does not validate item existence (`db.IndexEvent` upserts with no existence check), so "unknown item-id → error" contradicts the behavior-preserving extraction. **Resolved:** U4 scenario (2) changed to assert empty/required-arg validation (the guard that _does_ exist) and to verify append tolerates unknown item-ids **symmetrically** on both surfaces, keeping the extraction behavior-preserving.
3. **Optional-param drop** (Parity). Flipped templates dropped `--type`/`--commit-sha`, orphaning the `link_type`/`commit_sha` params. **Resolved:** U6 keeps the optional params with a documented **append-when-non-empty** flag convention (registry comment + U7 guide); `link list --type` and `comment add --commit-sha` are in the U1/U4 signatures.
4. **File-count honesty + U1 mcp test target** (Scope + Go). U1/U4 understated file counts (each also touches an mcp handler + `root.go`); U1's test target omitted `./internal/mcp/...`. **Resolved:** counts corrected (U1 = 4 files, flagged as heaviest unit; U4 = 4 files), `./internal/mcp/...` added to U1's test target with an MCP behavior-preservation assertion.
5. **Missing `docs/cli-reference/` regeneration** (Constitution — real CI-red gap). New cobra commands produce new generated files; `cli-reference-drift.yml` fails on any diff. **Resolved:** added **U8** (regenerate `docs/cli-reference/` via gen-docs + commit; depends on U1–U5) and R10.
6. **Missing Constitution Check section** (Constitution — governance). **Resolved:** added a **Constitution Check** section mapping U1–U8 to principles I/II/IV/V/VII + single-source-of-truth + quality gates.

### P3 findings — folded into acceptance criteria / notes

- `core.GetLinks` normalizes nil→`[]` inside the helper (Rule 3) — folded into U1.
- `hooks ack` asserts `{acked_seq}` shape; `metadata types` asserts `[]` not null — folded into U2/U5.
- `core.AppendComment` must preserve the zero-`Timestamp` value-passing sequence — folded into U4.
- `memory save` resolves the workspace root (not raw `*cwd`) for path parity — folded into U3.
- `metadata` type-layout uses the same `ws.Config.QueueLayout` nil-fallback as the MCP `queueLayout()` helper — folded into U5.
- Error-wrapping `%w` contract for the extracted helpers + CLI validation — added as **D6**.
- `append_comment` placeholder renamed `task_id`→`item_id` for cross-surface consistency — folded into U6.
- Advisory (not adopted, noted for the executor): optional `metadata type <x>` naming pairing with `metadata types`, and optional `WorkspaceMemoriesPath`/`WorkspaceHooksDir` path helpers for fuller wiring parity — left to executor discretion; not blocking.

### Runtime verification / closure gaps called out

Closure now enumerates the full quality-gate chain (`go build`/`go vet`/`golangci-lint`/`go test`, with the documented CRLF `gofmt` exception) and requires the `cli-reference-drift` gate green plus per-family full-flag-set exercise. No hardening signal was suppressed.
