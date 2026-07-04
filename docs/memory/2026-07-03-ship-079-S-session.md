# Ship session — 079-S CLI/MCP command parity phase-2

- **Date:** 2026-07-03
- **Agent:** Ship
- **Shipment:** `079-S` — "CLI/MCP command parity phase-2" (claimed → active)
- **Feature:** `079-F` (+ 8 tasks, 6 subtasks = 15 manifest items)
- **Branch:** `feat/079-cli-mcp-command-parity-phase2` (off local `main` a1c5c8b)
- **Plan:** `docs/exec-plans/2026-07-03-cli-mcp-command-parity-phase2-plan.md`

## Tool gate (Step 0)
- Registry present; MCP tools not directly exposed → **DEGRADED_MODE** via `backlogit.exe` CLI fallbacks (fully declared in registry). Freshness confirmed: `shipment add` + `checkpoint create` present.
- `INDEX_SYNC_OK` (711 artifacts).

## Pre-flight (Step 1)
- P-001: no other active items. Compilation clean (`go test -run=^$ ./...`). Constitution I/II/IV re-read.
- Worktree carried pre-existing benign env/harness changes from Orchestrator prep (agent `.md`, `.gitignore`, `start.ps1`, `hooks_queue.jsonl`). P-011 dirty-worktree gate: proceeded (env-only dirt, not scope) with **surgical `git add <path>`** for every feature commit.

## Orchestration decision
- Skills are markdown process guides in this env, not callable tools. Ship already read all target surfaces (mcp handlers, core extraction targets, CLI patterns, drift test, test helpers), so **direct inline implementation with strict harness-first red→green TDD** avoids duplicate exploration. Independent review gate (Step 4.4) delegated to the `code-review` sub-agent — preserving Ship's core separation-of-concerns value.

## Execution order
U1‖U2‖U3‖U4‖U5 (independent) → U6 (registry flip + drift flag-parity, characterization-first) → U7 (docs); U8 (gen-docs) after U1–U5.

## Grounding notes
- U1 link: extract `core.GetLinks(ctx, ws, id, linkType) []db.LinkEdge` (nil→[]), refactor `handleGetLinks` (internal/mcp/links.go). CLI add/remove reuse `core.AddArtifactLink`/`RemoveArtifactLink`.
- U2 hooks: `events.NewHookEventWriter(backlogitDir)` + `NewCheckpointStore(backlogitDir)` + `events.PollHookEvents`/`AckHookEvents`; resolve root via `core.NewWorkspace`.
- U3 memory: `events.SaveMemory(ctx, filepath.Join(ws.RootPath,".backlogit","memories.json"), key, summary)` — resolved root (not raw cwd).
- U4 comment: extract `core.AppendComment(ctx, ws, itemID, actor, comment, commitSHA)` mirroring `core.LinkCommit` template BUT no Timestamp set (preserve zero-timestamp indexed row); refactor `handleAppendComment` (tools.go:844).
- U5 metadata: subcommands types/wit/templates on existing `NewMetadataCmd`; `core.ListTypes`/`DescribeType` with `ws.Config.QueueLayout` nil-fallback; `templates.NewService(ctx, filepath.Join(core.WorkspaceStorageRoot(ws.RootPath),"templates")).ListTemplates()`.
- U6 drift test: extend `registry_parity_test.go` with flag/positional-parity assertion (existing `resolveCLIPath` truncates at first `--`/`{{`).
- Test helpers (package cli_test): `setupCLIWorkspace(t)`, `runCLIStdout(t,root,args...)`, `runCLIErr`, `cliAddFeature/cliAddTask/cliCreateShipment`.

## Status: MERGE-READY (all units green) — HALT before merge per P-014

### Units delivered (U1–U8, all `done`; harness-first red→green)
- **U1 link** (`079.001-T`+2 ST): `link add/remove/list` CLI; extracted `core.GetLinks` (nil→[]); `handleGetLinks` delegates.
- **U2 hooks** (`079.002-T`): `hooks poll/ack` CLI over `events.PollHookEvents`/`AckHookEvents`.
- **U3 memory** (`079.003-T`): `memory save` CLI over `events.SaveMemory` (resolved workspace root).
- **U4 comment** (`079.004-T`+2 ST): `comment add` CLI; extracted `core.AppendComment` (zero-Timestamp behavior preserved vs `LinkCommit`); `handleAppendComment` delegates; MCP characterization test added.
- **U5 metadata** (`079.005-T`): `metadata types/wit/templates` CLI subcommands.
- **U6 registry** (`079.006-T`+2 ST): flipped 10 rows `mcp_only:true`→resolvable `cli_command`; kept `log_telemetry` (read-only telemetry) + `merge_sync` (deferred phase-3, write-by-default) as mcp_only w/ rationale. Added load-bearing `TestRegistryParity_FlagAndPositionalParity` — caught real pre-existing `stash add` drift (`--text` → positional) + parser gap; fixed parser (bool-aware + literal-flag-value-aware `lookupFlag`) and stash row. **Review hardening:** added required-flag coverage assertion (cobra.BashCompOneRequiredFlag) — green across all rows.
- **U7 docs** (`079.007-T`): reclassified parity-matrix (10 rows→gap-filled-phase-2, totals=56), rewrote flag-level parity note; updated fallback-guide (5th drift condition, new groups, mcp-only list→2). Lint clean.
- **U8 cli-reference** (`079.008-T`): regenerated via gen-docs; committed only real content diffs (`backlogit.md`, `backlogit_metadata.md` + 14 new files); reverted 64 CRLF-churn files; verified idempotent (CI drift gate green).

### Commits (origin/main..HEAD)
- `4da9a21` feat(cli): U1–U5 CLI fallbacks
- `9f0ff62` feat(config): U6 registry flip + flag-parity gate
- `4a4e994` docs: U7 discoverability docs
- `9511334` docs(cli): U8 regenerate cli-reference
- `c19b5c2` test(cli): required-flag coverage hardening (review-driven)
- (+2 pre-existing Stage commits ride into feature PR: `59624cd`, `a1c5c8b`)

### Quality gates (final)
- `go build ./...` ✅ · `go vet ./...` ✅ · `go test ./...` ✅ (all pkgs incl. contract+integration) · `golangci-lint run` ✅ 0 findings · `gofmt -l` = CRLF false-positives only (CI-LF authoritative, advisory).

### Review gate (Step 4.4)
- `code-review` sub-agent, scope `a1c5c8b..HEAD`: **NO P0/P1.** 2 P3 notes — #1 (required-flag false-negative) **remediated** (c19b5c2); #2 (`memory save` stricter `--summary` than MCP) benign-by-design, no action.

### CRLF gotchas (workspace has no .gitattributes, autocrlf on; blobs LF)
- `gofmt -l` flags all touched Go files (false-positive) — do NOT reformat.
- gen-docs rewrites cli-reference with LF; git shows CRLF churn — only commit real `--ignore-cr-at-eol` diffs.

### Deferred / out-of-scope
- `merge_sync` CLI fallback → phase-3 (write-by-default, needs guardrails).
- `log_telemetry` → intentional-permanent mcp_only (read/report-only telemetry).
- External autoharness `.tmpl` edits (stash `EED25928`) → out of scope (Principle IV, stay in-tree).

### Backlog state
- All 13 tasks/subtasks `done`; `079-F` feature `done` (auto-archived queue→archive, matches 078-S pattern in feature PR). Shipment `079-S` stays `active` in queue until post-merge `ship_shipment` (Step 6 closure PR).

### Next: Step 5 PR lifecycle → runtime-verification → operational-closure → §1.9 gate → **HALT.**
- Do NOT merge (P-014/Principle VII). Operator authorizes admin bypass at merge time (PR-Review ruleset needs an approving review the sole author-identity can't self-supply, as with 078-S). Merge-commit strategy only (P-009).

