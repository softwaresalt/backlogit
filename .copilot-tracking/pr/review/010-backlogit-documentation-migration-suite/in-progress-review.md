<!-- markdownlint-disable-file -->
# PR Review Status: 010-backlogit-documentation-migration-suite

## Review Status

* Phase: 4 — Handoff Finalized
* Last Updated: 2026-04-03T18:28:36Z
* Summary: Validated the delegated reviewer output, approved 12 high-confidence findings for PR comments, and generated the final handoff artifact

## Branch and Metadata

* Normalized Branch: `010-backlogit-documentation-migration-suite`
* Source Branch: `010-backlogit-documentation-migration-suite`
* Base Branch: `main`
* Linked Work Items: TASK-010 plus stash and deliberation workflow follow-up changes
* Commits Ahead of Main: 11
  * `878b965` Added new deliberate skill.
  * `a7f68b7` Committing current set of changes to code and documentation
  * `86578c1` Moved completed items to completed directory.
  * `34f6a13` Added verified migration changes
  * `ecb42b4` chore: close TASK-010 epic and all sub-tasks as Done
  * `c04660b` docs: write comprehensive documentation suite for backlogit
  * `54743f1` test: integration harnesses pass for TASK-010.01.04 and TASK-010.04.03/04
  * `4f9eac1` feat(parser): implement Classifier, MigrateWithOptions, and FormatReport
  * `eb1a18d` feat(parser): implement ParseLegacyEnhanced with depth/metadata extraction
  * `87aca0e` feat(parser): implement pluggable migration adapter interface
  * `1043e8b` test(parser,config): add F010 harness - 73 failing tests for migration suite
* Total Files Changed: 296
* Reviewable Files Outside Backlog Tracking: 247
* Focused Review Scope: 16 high-signal files across `internal/`, `.github/`, `README.md`, and `docs/`
* PR Reference Generated: `.copilot-tracking/pr/review/010-backlogit-documentation-migration-suite/pr-reference.xml`
* Parsed Diff Summary: `.copilot-tracking/pr/review/010-backlogit-documentation-migration-suite/diff-summary.json`

## Phase 1 Action Log

* ✅ Tracking directory created: `.copilot-tracking/pr/review/010-backlogit-documentation-migration-suite/`
* ✅ `scripts/dev-tools/pr-ref-gen.sh` not present; generated `pr-reference.xml` via `git diff origin/main...HEAD`
* ✅ Parsed diff summary persisted to `diff-summary.json` for repeatable scope selection
* ✅ Review scope reduced from broad branch churn to 16 behavior-relevant files covering stash persistence, deliberation creation, MCP/CLI exposure, and skill/doc rewiring

## Diff Mapping

| File | Type | New Lines | Old Lines | Notes |
|------|------|-----------|-----------|-------|
| `internal/stash/stash.go` | added | 1-215 | Stash grammar, `[deliberation:DL...]` tag parsing, ID and priority normalization |
| `internal/db/schema.go` | modified | 63-107, 133-146, 165-166 | Adds `stash_entries.deliberation_id` and related schema migration hooks |
| `internal/db/stash.go` | added | 1-160 | SQLite stash index with deliberation linkage persistence |
| `internal/db/rehydration.go` | modified | 24-27, 37-39, 54-56, 92-99, 166-269 | Rebuilds stash and harvested lineage from Markdown source |
| `internal/core/stash.go` | added | 1-435 | Core stash fetch, harvest, and stash-to-deliberation linking behavior |
| `internal/core/templates/deliberation.go` | added | 1-104 | Template-backed deliberation creation from stash entries |
| `internal/cli/deliberate.go` | added | 1-71 | CLI entry point for linked deliberation creation |
| `internal/mcp/tools.go` | modified | 226-268, 905-1025 | MCP stash and deliberate tool registration plus handlers |
| `internal/config/defaults.go` | modified | 27-28, 40-61, 78-157, 222-254, 309-316 | Default type, template, queue layout, and stash bootstrap changes |
| `.github/skills/deliberate/SKILL.md` | added | 1-167 | Replaces brainstorm workflow with backlogit-native deliberate flow |
| `.github/skills/impl-plan/SKILL.md` | modified | 3-4, 9, 16, 37, 67, 86, 89, 132, 215 | Planning skill now accepts `.backlogit/queue/DL...md` as source |
| `.github/skills/plan-review/SKILL.md` | modified | 3, 9, 44, 60, 107, 153 | Plan review references updated for deliberation-origin plans |
| `.github/agents/backlog-harvester.agent.md` | modified | 2-3, 11, 26, 87-92, 184-244 | Harvester retargeted from brainstorm docs to deliberation artifacts |
| `README.md` | modified | 1-118 | Top-level workflow and command documentation updates |
| `docs/configuration.md` | added | 1-732 | Full configuration and MCP stash/deliberate documentation |
| `docs/workflow.md` | added | 1-248 | Operator workflow documentation for stash, deliberate, and harvest |

## Instruction Files Reviewed

* `.github/instructions/constitution.instructions.md`: applies to all files and governs CQRS, test-first behavior, workspace containment, and backlogit-native workflows
* `.github/instructions/go.instructions.md`: applies to `**/*.go` and governs Go code quality, error handling, and test expectations
* `.github/instructions/go-mcp-server.instructions.md`: applies to `**/*.go` and is especially relevant to `internal/mcp/tools.go`
* `.github/instructions/markdown.instructions.md`: applies to `**/*.md` and governs markdown structure
* `.github/instructions/writing-style.instructions.md`: applies to `**/*.md` and governs documentation voice and clarity
* `.github/instructions/prompt-builder.instructions.md`: applies to `**/SKILL.md` and `**/*.agent.md`, covering the changed skill and agent artifacts
* `.github/instructions/pull-request.instructions.md`: applies to `.copilot-tracking/pr/**` and governs this review workspace

## Review Items

### 🔍 In Review

* None. Findings were triaged autonomously because the user was unavailable for interactive review decisions.

### ✅ Approved for PR Comment

* **P1** — `internal/core/stash.go:237`: `HarvestStashEntry` calls `db.UpsertItem(ctx, ws.DB, artifact)` without the `ws.DB != nil` guard used by the later stash-index writes in the same function. A caller that constructs a workspace without a live DB can panic after the artifact file is already written.
* **P1** — `internal/mcp/tools.go:235-253, 923-956`: the MCP stash surface does not match the CLI defaults. `backlogit_stash` requires `kind` and `backlogit_harvest_stash` requires `artifact_type`, while the CLI defaults both to `task`, so the documented CLI/MCP parity is not actually true.
* **P1** — `internal/db/stash.go:27-34, 129-152`: `RehydrateStashIndex` clears and rebuilds the stash index outside a transaction. Any failure after the deletes can leave the cache empty or partially rebuilt until another successful full rehydration.
* **P1** — `.github/agents/backlog-harvester.agent.md:84-92`: the harvester still parses the old impl-plan schema (`## Problem Statement`, `## Approach`, `## Key Decisions`, `## Constitution Check`) even though `.github/skills/impl-plan/SKILL.md` now emits `## Problem Frame`, `## Decisions`, and `## Standards Check`. That breaks the deliberate -> plan -> harvest contract.
* **P2** — `internal/core/stash.go:233-241`: `HarvestStashEntry` writes the artifact before updating `.backlogit/queue/.stash.md`, so a stash-file write failure leaves an active stash entry that can be harvested again into a duplicate artifact.
* **P2** — `internal/mcp/tools.go:917-918, 938-940, 974-988, 1021-1022`: stash and deliberate handlers convert validation, not-found, and conflict failures into generic `internal` MCP errors instead of returning domain-appropriate error types such as `validation_failed`.
* **P2** — `internal/db/stash.go:154-160`: `mustParseTime` silently falls back to zero `time.Time` on parse failure, which makes malformed timestamps look valid and can distort stash ordering.
* **P2** — `internal/stash/stash.go:72-79`: `ParseContent` silently drops entries with invalid priority or kind. During stash reads and rehydration that becomes silent data loss with no diagnostic trail.
* **P2** — `internal/core/stash.go:428-433`: `expandStashEntry` swallows `db.GetItem` failures when resolving a linked deliberation, so transient DB errors are treated the same as "not found".
* **P2** — `docs/configuration.md:35-45, 131-176`: the configuration guide still documents only `feature`, `task`, and `subtask` as defaults even though `internal/config/defaults.go` now includes `deliberation` and places it at queue level 1.
* **P2** — `docs/workflow.md:36`: the workflow guide says `backlogit init` updates `.gitignore`, but the implementation does not modify ignore files.
* **P2** — `docs/workflow.md:108-111`: the workflow guide uses unsupported status `in_review`, while the configured status enum is `review`.

### ❌ Rejected / No Action

* Deferred broad backlog and historical task file churn from this review because it does not materially affect the current feature behavior under review

## Next Steps

* [x] Invoke reviewer subagents on the focused scope
* [x] Merge findings by severity and route actionable issues
* [x] Present findings for collaborative review decisions
* [x] Triage findings autonomously when the user was unavailable
* [x] Generate `handoff.md` with ready-to-submit PR comments
