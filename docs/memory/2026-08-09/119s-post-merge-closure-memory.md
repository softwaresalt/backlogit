---
type: session-memory
date: 2026-08-09
shipment: 119-S
session: dark-factory-ship-119s-post-merge
---

# 119-S Post-Merge Closure Memory

## Outcome: COMPLETE

- PR #338 merged: 5b6e7779a723eecd918a749f5e3ded3ac2ec15ba
- 119-S shipment: SHIPPED
- All 6 tasks (106.019-T through 106.024-T): SHIPPED
- Post-merge closure branch: post-merge/119-s-formal-gate-f6

## What Was Done

### F6 — Governed-Operation CLI/MCP Parity

**core.AssociateCommit** routes all three surfaces (CLI update --commit,
MCP track_commit, MCP update_item(commit=...)) through one shared function
that writes all three representations:
1. Frontmatter scalar (validates item exists)
2. commit_links upsert (conditional: preserves non-empty message/author)
3. JSONL append (last, never compensated)

**Registry**: governed: true + governed_name: commit_association on track_commit.
force-gates/gate-base documented as human_terminal_only in cli_only_flags.

**U5 Tests**: TestRegistryParity_GovernedOperationBehavioralParity and
TestRegistryParity_ForceGatesAbsentFromMCPParams in registry_parity_test.go.

**U6 Doc**: docs/design-docs/governed-operation-parity.md

## Copilot Review Cycle Summary

- 9 threads total across 3 rounds
- Key findings addressed:
  - CLI: AssociateCommit before deferred gate JSON return
  - MCP: commitSHA in size/complexity exclusivity checks
  - MCP: skip UpdateArtifactWithGate for commit-only requests
  - MCP: durabilityOutcomeResult for JSONL failure classification
  - Registry: commit: commit (not sha) in update_task params
  - U5 test: Fatalf not Skipf for unknown governed operations
  - commits.go step order: frontmatter-first (validates existence)
  - conditional upsert to preserve existing message/author metadata

## Follow-Up Stash

- 4CF89803: Extend governed: true to other registry operations

## Worktree Status

- Branch: post-merge/119-s-formal-gate-f6
- Clean (after this commit)
- Pending: push + PR creation for post-merge closure
