---
chunk_strategy: h1-h2-h3
description: "PR #370 readiness summary for 144-F / 128-S implementation"
doc_type: memory
schema_version: "1.0"
source: docs/memory/2026-08-19/ship-128s-pr370-readiness.md
title: "PR #370 readiness summary — 144-F shipped-transition prevention"
---

## PR Readiness Summary

**PR**: #370 — feat(core): shipped-transition prevention hardening (144-F, 128-S)
**URL**: <https://github.com/softwaresalt/backlogit/pull/370>
**Branch**: `feat/144-shipped-transition-prevention`
**Current HEAD**: `6350115a51fe1f211a5a3fd971627c8bdeee9aaa`
**Worktree**: `D:\Source\GitHub\backlogit\.worktrees\stage-47b48db0`

## §1.9 Defense-in-Depth Gate

| Check | Result |
|---|---|
| Check 1: No pending Copilot review | PASS — reviewRequests.nodes = [] |
| Check 2: Review covers current HEAD | PASS — latest review on 6350115a = headRefOid |
| Check 3: Zero unresolved Copilot threads | PASS — 8 threads, all resolved |
| **Overall §1.9 gate** | **PASS** |

## CI Checks (run 32302519985)

| Check | Result |
|---|---|
| Detect code changes | pass |
| Markdown lint (P-008) | pass |
| Docline frontmatter gate | pass |
| CLI Reference Drift | pass |
| test | pass (4m18s) |

## P-009 Merge Strategy

- allow_merge_commit: true ✓
- allow_squash_merge: false ✓
- allow_rebase_merge: false ✓

**P-009 PASS** — merge-commit-only strategy confirmed.

## Copilot Review Summary

3 review cycles. All findings resolved:

| Thread | Finding | Resolution |
|---|---|---|
| PRRT_kwDORzozKM6amisA | Guard 2 runs after cascade | Fixed: moved to `archiveShippedEventPreflight` before cascade (5e7c4875) |
| PRRT_kwDORzozKM6amisU | doc_type compound invalid | Fixed: changed to `learning` (f88f0633) |
| PRRT_kwDORzozKM6amisq | doc_type design-doc invalid | Fixed: changed to `design` (f88f0633) |
| PRRT_kwDORzozKM6amis6 | MCP parity calls handler directly | Acknowledged: deferred to follow-up stash |
| PRRT_kwDORzozKM6amitN | CLI parity only tests mapper | Acknowledged: deferred to follow-up stash |
| PRRT_kwDORzozKM6amitk | Memory doc task table | Acknowledged: informational snapshot |
| PRRT_kwDORzozKM6am8tM | Preflight reads stale archive copy | Fixed: applied 060.002-T queue-preference (6d6aa610) |
| PRRT_kwDORzozKM6anKvt | BulkUpdateStatus bypasses guard 1 | Fixed: abort batch with sentinel; CLI exit 9 (6350115a) |

## Local Review Readiness

**P0/P1 findings**: 0 remaining (all fixed)
**P2/P3 findings**: MCP direct-handler tests and CLI mapper-only tests acknowledged as advisory; stash follow-ups created.

## Quality Gates

| Gate | Result |
|---|---|
| `go test ./...` | PASS |
| `go vet ./...` | PASS |
| `golangci-lint run --timeout 300s` | PASS |
| `gofmt -l .` (changed files) | PASS |

## Merge Command

When operator approves (P-014):

```
gh pr merge 370 --merge --body "Merge pull request #370"
```

**DO NOT MERGE** — execution paused at P-014 operator merge-approval gate.

## Residual Risks / Follow-up Items

1. MCP parity tests use direct handler calls; follow-up stash to add registered-dispatch tests
2. CLI command-level tests for exit code 9 (move, archive, bulk-status); follow-up stash
3. Out-of-band Markdown edits remain out of scope; doctor audit is the detection net
4. Guard 2 fail-closed on governed archival if JSONL unreadable (acceptable per plan)

## P-014 Gate Status

**PAUSED — awaiting operator merge approval.**

This PR requires explicit operator approval before merge. This summary confirms:
- Current HEAD local readiness: PASS
- CI checks: all pass
- Copilot review freshness: covers current HEAD
- Unresolved Copilot threads: zero
- Merge strategy: merge commit (P-009 compliant)

Execution paused at P-014. Ready to merge upon operator approval.
