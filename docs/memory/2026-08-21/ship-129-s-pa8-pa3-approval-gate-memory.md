---
chunk_strategy: h1-h2-h3
description: 'Ship session memory for shipment 129-S — 15 permitted tasks completed test-first; halted at the PA-8/PA-3 approval gate with 8 tasks remaining.'
doc_type: memory
schema_version: "1.0"
source: docs/memory/2026-08-21/ship-129-s-pa8-pa3-approval-gate-memory.md
title: 'Ship session memory — shipment 129-S, halted at PA-8/PA-3 approval gate'
---

## Session scope

Acting as the Ship agent for shipment `129-S` (feature `146-F`, "Eliminate success-shaped
evidence loss on governed diagnostic paths"). Established a compliant implementation branch,
claimed the shipment, and executed every task that does not require PA-8 or PA-3 operator
approval, following the required test-first harness/build workflow (each task landed as an
independent, gated-quality commit). Halted cleanly at the point where the only remaining ready
tasks require gate approval, per explicit operator instruction.

## Branch / worktree topology (P-011 / P-016)

* Root worktree `C:\Source\GitHub\backlogit` was on `chore/stage-129-s` (the merged staging
  branch) with three untracked Azure DevOps artifacts that had to remain untouched:
  `docs/decisions/2026-08-20-azure-devops-sync-spike.md`,
  `docs/exec-plans/2026-08-20-azure-devops-sync-plan.md`, `docs/memory/2026-08-20/`.
* Verified `git status --short --untracked-files=no` was clean (zero tracked changes) before
  switching branches, so the untracked Azure DevOps files were provably unaffected by the
  in-place branch switch (git preserves untracked files across checkout).
* Checked out `main`, pulled to `origin/main` (`3cb5424c9844cced210f8f2bf2c1d7add0625e16` — the
  PR #372 merge commit containing the reviewed staging head `dcc9b0c7`), then created
  `feat/146-f-success-shaped-evidence-loss` from updated `main` in the SAME worktree (no new
  worktree needed — the only additional worktree found,
  `.copilot/session-state/ecebe820-.../dark-factory-worktree` on `chore/121-s-closure`, was
  independently verified already merged into `origin/main` via `6cd5b5ef`, so it is stale/completed
  and not a P-016 violation; it was not touched).
* This is now the single active implementation branch/worktree for the session.

## Shipment / task state

* Claimed shipment `129-S` via `backlogit_claim_shipment` (queued → active). Note:
  `backlogit_claim_shipment` cascades `active` status to the covering feature and every manifest
  task automatically — Step 4.1's per-task "claim" is therefore already satisfied by the cascade;
  only the done-transition per task is a distinct action.
* Ran `backlogit_merge_sync` (full rehydration; branch switch produced 1636 changed files) before
  any shipment reads, per Step 0.1.

### Completed (15 of 23 tasks — every task NOT gated by PA-8/PA-3)

| Task | Unit | Commit | Domain |
|---|---|---|---|
| 146.001-T | U0a | `3fe9652f` | code (prelude: ErrCheckpointUnknownField, CheckpointUnknownFieldError, Extra+Keys() stub, CreateCheckpointResult, 4 call sites) |
| 146.002-T | U0b | `af5c6a1b` | code (prelude: RuleDecodeError, ErrFrontmatterDecode) |
| 146.004-T | U1a | `d0f81903` | tests (red: open context namespace, checkpoint_context_test.go) |
| 146.005-T | U1b | `36a7c055` | tests (red: marshaller-safety guards, checkpoint_marshal_test.go) |
| 146.007-T | U3a | `64929f73` | tests (red: closed schema namespace, N=8 independent-pair table) |
| 146.008-T | U3b | `a47c624c` | tests (green: pre-U2 golden read-path guard) |
| 146.009-T | U3c | `d5dc866c` | tests (green: legacy-corpus/misclassification/drift guards + synthetic fixtures) |
| 146.010-T | U3d | `beca759e` | tests (red: MCP validation outcome + tool contract) |
| 146.013-T | U5a | `95ce1375` | tests (red: context_keys on both transports) |
| 146.014-T | U5b | `6508e8df` | tests (red: empty/legacy context_keys shapes) |
| 146.016-T | U7a | `af67ce9c` | tests (red: report-and-continue docs lint) |
| 146.017-T | U7b | `924a941a` | tests (green: containment/sanitization guards) |
| 146.020-T | U9a | `67f15de4` | tests (red scenarios 1-3, green scenarios 4-5: degraded-corpus shape/exit) |
| 146.021-T | U9b | `83d5e272` | tests (green: clean-corpus always-array shape) |
| 146.024-T | U9c | `ef0d9c3b` | tests (red: docs lint surface contract text) |

Every commit was individually gated: `go build ./...`, `go vet`, `golangci-lint run` (scoped to
touched packages, then full-tree at the end), and a gofmt cleanliness check normalized for
line endings (see Environment note below), plus the relevant `go test` scope, before moving the
task to `done` and calling `backlogit_track_commit`.

### Blocked on approval (8 of 23 tasks — do not start without explicit operator approval)

| Task | Unit | Gate | Depends on (unmet) |
|---|---|---|---|
| **146.006-T** | U2 | **PA-8** (`high`) | ready now — 146.004/005/008/009 done |
| 146.011-T | U4 | — | 146.006-T (PA-8) |
| 146.012-T | U4b | — | 146.011-T |
| 146.015-T | U6 | — | 146.006-T (PA-8), 146.012-T |
| **146.018-T** | U8 | **PA-3** (`high`) | ready now — 146.016/017/020/021 done |
| 146.019-T | U8b | — | 146.018-T (PA-3) |
| 146.022-T | U10a | — | 146.015-T, 146.019-T |
| 146.023-T | U10b | — | 146.015-T, 146.019-T |

**146.006-T and 146.018-T are now both fully ready** (all their dependencies are `done`), which is
exactly the point at which this session must stop per explicit operator instruction. Neither PA-8
nor PA-3 was approved by staging PR #372's approval (that approval covers only the staging PR, per
the operator's explicit constraint). PA-5 (destructive, refresh of the pinned out-of-tree binary)
remains operator-only and blocked; PA-6 (rollback revert) is not yet relevant, no rollback has
occurred.

## Test suite state (expected, intentional red)

Full `go test ./...` after all 15 completions shows exactly 23 failing tests, all of them the
red-harness assertions for work gated behind PA-8/PA-3 (none are regressions):

* `internal/events` (9): `TestCreateCheckpoint_OpenContextNamespace_FlatScalar`,
  `_NestedObject`, `TestCreateCheckpoint_RejectsSingleUnknownTopLevelKey`,
  `_RejectsTwoUnknownTopLevelKeys`, `_RejectsUnknownNestedProgressKey`,
  `_MixedCaseDuplicateProgressAlias_NIndependentPairs`,
  `TestCheckpointContext_MarshalJSON_NonAddressableValue`,
  `TestCreateCheckpoint_ContextHTMLEscapeGuard`, `_ContextKeyInjectionGuard`
* `internal/mcp` (6): `TestHandleCreateCheckpoint_ResultIncludesContextKeys`,
  `_TwoUnknownTopLevelKeys_ValidationFailed`,
  `TestBacklogitCreateCheckpoint_DescriptionEnumeratesLegalKeys`,
  `_DescriptionNamesEveryReflectedKey`, `TestBacklogitDocsLint_ContractText_DecodeErrorSuccessfulResult`,
  `TestDocsLintTool_DegradedCorpus_SuccessfulResultNotInternalError`
* `internal/cli` (6): `TestCheckpointCreate_LegacyDumpContextKeysNamesActualKeys`,
  `_EmptyTaskIDsContextKeysMatchWrittenBytes`, `_ContextKeysInJSONOutput`,
  `_ContextKeysByteIdenticalAcrossSurfaces`,
  `TestCheckpointCreateCommand_ContractText_OpenContextAndContextKeys`,
  `TestDocsLintCommand_ContractText_DecodeErrorAndRetainedExit`,
  `TestDocsLint_DegradedCorpus_ExitsNonZeroWithDecodeErrorFinding`,
  `_FindingsArrayPresentAndNonNull`
* `internal/docline` (2): `TestLintTree_MalformedFrontmatter_ReportsFindingAndContinues`,
  `TestLintTree_OnlyDecodeFailure_YieldsNonNilFindings`

Every other package is green: `internal/core` (433.8s), `internal/db`, `internal/config`,
`internal/errors`, `tests/contract`, `tests/integration`, etc. `go vet ./...` and
`golangci-lint run ./...` are clean across the whole tree.

## Environment note: gofmt vs. CRLF checkout

Local `core.autocrlf=true` combined with the repo's `.gitattributes` (`* text=auto`, blobs stored
as LF) means `gofmt -l .` reports essentially every file in the tree as needing reformatting,
purely due to CRLF line endings from checkout — this predates this session and is not a
regression. Verified each new/edited file for REAL gofmt violations by normalizing a scratch copy
to LF and running `gofmt -l` against that copy; only one real issue was found and fixed (two
stray trailing blank lines in `checkpoint_errors.go`, unrelated to CRLF). Do not trust a raw
`gofmt -l .` count in this environment; normalize line endings first.

## Approval request (STOP — do not proceed without explicit operator response)

**PA-8** — `ActionRisk: high`. Blocks `146.006-T` (U2): give `CheckpointContext.Extra` its real
`UnmarshalJSON`/`MarshalJSON`/`Keys()` behavior in `internal/events/checkpoint_schema.go`, so a
hand-written codec runs on **every checkpoint read path** (`ParseCheckpoint` is exercised by
`ListCheckpoints`, `GetCheckpoint`, `ResolveCheckpoint`, and both disposition verbs). Rollback:
`git revert` of the merge commit if a degenerate on-disk `context` value (null, absent, scalar,
duplicate/mixed-case-alias keys) is newly reclassified as corrupt (rollback trigger A/C in the
plan). Tasks that CANNOT proceed without this approval: 146.006-T, and transitively 146.011-T,
146.012-T, 146.015-T, 146.022-T, 146.023-T.

**PA-3** — `ActionRisk: high`. Blocks `146.018-T` (U8): replace `docline.LintTree`'s blanket
`return nil, err` on a per-file frontmatter decode failure with a three-way sentinel-discriminated
split (containment and I/O failures still hard-error; a frontmatter decode failure becomes a
`decode_error` finding and the scan continues) — a change to CI-gate-facing lint semantics.
Rollback: `git revert` of the merge commit if the docs-lint CI gate stops blocking a real
contract violation. Tasks that CANNOT proceed without this approval: 146.018-T, and transitively
146.019-T, 146.022-T, 146.023-T.

Tasks that **can** still proceed without either approval: none — every remaining task in the
manifest is gated behind PA-8, PA-3, or both.

**PA-5** remains `ActionRisk: destructive`, operator-only, and `blocked`: refreshing the pinned
out-of-tree binary (`C:\Tools\backlogit.exe`). No agent may perform it. Not relevant to the next
step. **PA-6** (rollback revert) is not yet relevant — no rollback has occurred.

## Next permitted action

Await explicit operator approval for PA-8 and/or PA-3 in a subsequent message. On approval for
PA-8: resume with 146.006-T (U2), then 146.011-T, 146.012-T, 146.015-T in dependency order. On
approval for PA-3: resume with 146.018-T (U8), then 146.019-T in dependency order. 146.022-T and
146.023-T require BOTH chains complete (146.015-T and 146.019-T). If only one of PA-8/PA-3 is
approved, the other chain remains blocked and the session should still halt again once that
chain's ready task is reached, rather than guessing at the missing approval.

## Files changed this session

23 test/source files created or edited across `internal/errors`, `internal/events`,
`internal/docline`, `internal/mcp`, `internal/cli`, plus 3 synthetic fixture files under
`internal/events/testdata/legacy-corpus/`. Full list is in the 15 commits on
`feat/146-f-success-shaped-evidence-loss` (see table above). `.backlogit/queue/` and
`.backlogit/archive/` were updated per-task by the backlogit MCP tools as each task was archived
into `done`. No production behavior changed for any gated capability (U2, U4, U4b, U6, U8, U8b) —
only declarations, prior-behavior-preserving preludes, and tests were added.

Not touched: the untracked Azure DevOps artifacts, the `autoharness` workspace (read-only for this
session, never referenced), the stale `dark-factory-worktree`, and any file outside this shipment's
scope.
