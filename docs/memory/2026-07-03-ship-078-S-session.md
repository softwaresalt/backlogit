# Ship session — 078-S CLI/MCP command parity

- **Date:** 2026-07-03
- **Agent:** Ship (backlog-to-shipped pipeline)
- **Shipment:** `078-S` (feature `078-F`), status: claimed → active
- **Branch:** `feat/078-cli-mcp-command-parity` (off local `main` @ d9341ca harvest commit)
- **Mode:** CLI-fallback (no direct MCP tools; `backlogit.exe` per registry). Index synced (695 artifacts).
- **Plan:** docs/exec-plans/2026-07-03-cli-mcp-command-parity-plan.md (plan-review PASS attempt 2)

## Environment notes / constraints
- Dirty worktree at start = OUT-OF-SCOPE harness/tooling/editor/hook-state files only
  (.backlogit/hooks_queue.jsonl, agent .md files, .gitignore, start.ps1, .cursor/, .github/copilot/).
  Decision: surgical staging only — NEVER `git add -A`/`.`. Leave those files untouched.
- gofmt -l CRLF false-positive on Windows checkout: do NOT mass-reformat. Authoritative = go vet + golangci-lint + CI(LF).
- P-014/Principle VII: DO NOT MERGE. Halt at merge-ready, await operator approval.
- P-009/Principle XI: merge-commit only (not merging this run regardless).
- Out-of-scope: stash EED25928/B55985DD (external autoharness .tmpl) — do NOT touch.
- `shipment claim` cascaded all 15 items to `active` (backlogit semantics).

## Execution order (per plan): U1 → U3 → U4 → U6 → U2 → U5
- U1 = 078.001-T (docs: parity matrix → docs/reviews/)
- U3 = 078.003-T (.001-ST test, .002-ST code): `shipment add` CLI
- U4 = 078.004-T (.001-ST test, .002-ST code): shipment-list items null→[]
- U6 = 078.006-T (.001-ST test, .002-ST code): `checkpoint create` CLI
- U2 = 078.002-T (.001-ST drift test, .002-ST registry): correct registry + drift test
- U5 = 078.005-T (docs: MCP→CLI fallback guide → docs/design-docs/)

## Canonical registry target (verified against live binary, 56 MCP tools)
- Add cli_command (9 stale): deliberate→`deliberate {{stash_id}} --title {{title}}`,
  get_metadata_catalog→`metadata catalog`, export_command_map→`metadata export-command-map {{path}} --format {{format}}`,
  get_version→`version`, telemetry_harvest→`telemetry harvest`,
  stash_get→`stash get {{stash_id}}`, stash_edit→`stash edit {{stash_id}}`,
  stash_archive→`stash archive {{stash_id}}`, stash_remove→`stash archive {{stash_id}}` (deprecated→archive; `remove` is CLI alias).
- Add rows (3 missing): docs_lint→`docs lint` (path,profile), docs_migrate→`docs migrate` (apply,path), docs_scope→`docs scope`.
- mcp_only: true (9 deferred): ack_hook_events, append_comment, get_wit_metadata, list_templates, list_types, log_telemetry, merge_sync, poll_hook_events, save_memory.
- Over-claim link (3): add_link/remove_link/get_links → strip `backlogit link` cli_command + mcp_only: true.
- After U3: add_to_shipment→`shipment add {{shipment_id}} {{item_id}}`.
- After U6: create_checkpoint→`checkpoint create --state-dump {{state_dump}}`.

## Progress log
- [x] Step 0.0 tool gate (DEGRADED_MODE CLI fallback), 0.1 index sync OK
- [x] Shipment claimed, branch created, pre-flight (compile PASS, P-001 clear)
- [x] U1 (078.001-T): parity matrix doc — DONE, docs lint 0 violations
- [x] U3 (078.003-T): `shipment add` CLI — red→green, mirrors handleAddToShipment
- [x] U4 (078.004-T): shipment list items null→[] — red (raw items:null fixture) → green (normalize loop mirrors tools.go:1554-1562)
- [x] U6 (078.006-T): `checkpoint create` CLI — red→green, mirrors handleCreateCheckpoint
- [x] U2 (078.002-T): registry drift test (4 assertions, drives from ListTools) + registry corrected → green. NOTE: original registry was 501 lines (telemetry_harvest/doctor/cleanup_checkpoints already existed in Telemetry/Maintenance sections; only telemetry_harvest needed a cli_command added; docs_lint/migrate/scope were the genuinely-missing rows)
- [x] U5 (078.005-T): MCP→CLI fallback guide (design doc) — docs lint 0 violations
- [x] All 15 backlog items moved to `done`; shipment 078-S remains `active` (ships post-merge only)
- [x] Quality gates ALL PASS: go test ./... (all ok), go vet (exit 0), golangci-lint (exit 0), gofmt (clean — CRLF-only false positives on modified files, verified via LF-normalized re-run)
- [x] Review gate (code-review agent): NO P0/P1 findings; flag-level parity + merge_sync/stash_remove edge cases verified; genuine red→green tests confirmed
- [ ] Commit (surgical staging), push, PR, Copilot review, CI, runtime-verification, operational-closure, §1.9 gate
- [ ] HALT at merge-ready (P-014: no merge without operator approval)

## Files delivered
- docs/reviews/2026-07-03-cli-mcp-parity-matrix.md (U1)
- internal/cli/shipment.go (U3 newShipmentAddCmd + U4 list normalize loop)
- internal/cli/shipment_add_test.go (U3), shipment_list_items_test.go (U4)
- internal/cli/checkpoint.go (U6 newCheckpointCreateCmd), checkpoint_create_test.go (U6)
- internal/cli/registry_parity_test.go (U2 drift test)
- .autoharness/backlog-registry.yaml (U2 op-map corrections)
- docs/design-docs/2026-07-03-mcp-to-cli-fallback-guide.md (U5)

## Out of scope (NOT touched, per mandate)
- stash EED25928 / B55985DD (external autoharness .tmpl sources — Principle IV out-of-tree)
- Environment artifacts NOT staged: .backlogit/hooks_queue.jsonl, .github/agents/*.md, .gitignore, start.ps1, .cursor/
