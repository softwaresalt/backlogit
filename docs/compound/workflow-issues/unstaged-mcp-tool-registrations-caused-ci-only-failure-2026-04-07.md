---
chunk_strategy: h1-h2-h3
description: Documents the local-versus-remote drift that hid missing MCP tool registrations until GitHub CI exercised the pushed branch.
doc_type: learning
docline:
    category: workflow_issue
    component: mcp_tools
    date: 2026-04-07T00:00:00Z
    file_path: internal/mcp/tools.go
    message: Local validation can go false-green when required MCP registrations exist only in unstaged changes instead of the pushed branch.
    problem_type: workflow_issue
    resolution_type: code_fix
    resolved: true
    root_cause: missing_test_fixture
    severity: high
    tags:
        - backlogit
        - mcp
        - ci
        - git
        - workflow
        - pr-8
        - shipment
ingested_at: "2026-06-26T02:32:58Z"
schema_version: "1.0"
source: docs/compound/workflow-issues/unstaged-mcp-tool-registrations-caused-ci-only-failure-2026-04-07.md
title: Unstaged MCP tool registrations caused a CI-only failure
---

## Problem

PR `#8` failed in GitHub CI even though local validation had passed. The pushed
branch was missing MCP tool registrations for `backlogit_ship_shipment` and
`backlogit_adopt_item`, but the local worktree still had those changes
unstaged.

## Symptoms

The remote contract tests failed in `tests/contract/shipment_tools_test.go`
with tool-not-found errors for `backlogit_ship_shipment` and
`backlogit_adopt_item`.

Local runs looked healthy because the local checkout executed against the dirty
worktree rather than the exact committed branch state.

## What Did Not Work

Relying on local green test runs was not enough.

The local branch contained shipment-unrelated dirty files and unstaged runtime
changes, so the working tree was ahead of the pushed branch in ways that CI
could see and local branch verification could accidentally hide.

Pushing without first reconciling the working tree left required MCP
registrations outside Git history.

## Solution

Commit the missing MCP registrations and handlers to the feature branch, then
rerun CI against the pushed branch.

### Before

```go
s.addTool(
	mcplib.NewTool("backlogit_claim_shipment",
		mcplib.WithDescription("Move a queued shipment to active"),
		mcplib.WithString("id", mcplib.Required(), mcplib.Description("Shipment ID")),
	),
	s.handleClaimShipment,
)

s.addTool(
	mcplib.NewTool("backlogit_return_blocked",
		mcplib.WithDescription("Return a blocked item from a shipment"),
		mcplib.WithString("shipment_id", mcplib.Required(), mcplib.Description("Shipment ID")),
		mcplib.WithString("item_id", mcplib.Required(), mcplib.Description("Item ID")),
		mcplib.WithString("reason", mcplib.Required(), mcplib.Description("Reason the item is blocked")),
	),
	s.handleReturnBlocked,
)
```

### After

```go
s.addTool(
	mcplib.NewTool("backlogit_ship_shipment",
		mcplib.WithDescription("Close a released shipment, archive the released scope, and record merge commit traceability"),
		mcplib.WithString("id", mcplib.Required(), mcplib.Description("Shipment ID")),
		mcplib.WithString("sha", mcplib.Description("Optional merge commit SHA to record on released artifacts")),
		mcplib.WithString("message", mcplib.Description("Optional merge commit message")),
		mcplib.WithString("author", mcplib.Description("Optional merge commit author")),
	),
	s.handleShipShipment,
)

s.addTool(
	mcplib.NewTool("backlogit_adopt_item",
		mcplib.WithDescription("Adopt an orphaned item under a new parent feature"),
		mcplib.WithString("item_id", mcplib.Required(), mcplib.Description("Item ID to adopt")),
		mcplib.WithString("new_parent_id", mcplib.Required(), mcplib.Description("New parent feature ID")),
	),
	s.handleAdoptItem,
)
```

The fix landed in commit `772cfe1`, which made the pushed branch match the local
runtime surface and cleared CI.

## Why This Works

MCP contract tests validate the server that GitHub actually builds from pushed
commits, not the local unstaged working tree.

Once the missing tool registrations and handlers were committed, the remote
contract surface matched local expectations and the tool lookup failures
disappeared.

## Prevention

Use these guardrails for shipment branches:

* Reconcile the working tree before pushing. `git diff HEAD` should not contain
  required runtime changes that are missing from the branch.
* Treat dirty future-scope work as separate from merge-ready work. Stash or
  move it off the shipment branch before the final PR loop.
* When CI fails with missing tool errors, compare the failing branch against the
  local dirty worktree before assuming the implementation itself is wrong.
* Prefer a clean branch before merge approval so local review and remote review
  are exercising the same code.

## Related Solutions

* [`docs/compound/workflow-issues/stable-contract-before-two-agent-adoption-2026-04-05.md`](stable-contract-before-two-agent-adoption-2026-04-05.md)
  explains why stable contracts matter more than local workflow assumptions.
* [`docs/compound/go-patterns/f015-shipment-stash-patterns.md`](../go-patterns/f015-shipment-stash-patterns.md)
  covers state-transition hygiene during rollout and migration-heavy changes.
