---
chunk_strategy: h1-h2-h3
description: 'Design record for F6: governed-operation CLI/MCP parity hardening — one shared core function for commit association, behavioral parity assertion, and deliberate asymmetry documentation.'
doc_type: design
schema_version: "1.0"
source: docs/design-docs/governed-operation-parity.md
title: Governed-Operation Parity Contract (F6)
---

# Governed-Operation Parity Contract (F6)

Source: `docs/exec-plans/2026-08-07-f6-governed-op-cli-mcp-parity-plan.md`

## Problem Statement

Before F6, three surfaces performed commit association but produced different observable state:

| Surface | What it wrote |
|---|---|
| CLI `update --commit` | Frontmatter scalar only |
| MCP `track_commit` | `commit_links` row + JSONL event, **no** frontmatter scalar |
| MCP `update_item(commit=…)` | Frontmatter scalar only |

All three passed the registry surface-parity test while producing three different states. The
registry's `track_commit → cli_command` mapping pointed to `backlogit update --commit`, which
did something materially different.

`LinkCommit` also silently swallowed JSONL append failures (returned `nil` on error), so callers
were told the association succeeded when only one of three representations was written.

## Solution: `core.AssociateCommit`

`core.AssociateCommit` performs commit association as an **ordered list of discrete steps** so that
F5's compensating envelope can wrap each step independently later without rewriting this function:

1. **Frontmatter scalar update** (idempotent, reversible via `UpdateArtifact`)
2. **`commit_links` upsert** (idempotent, reversible via `DELETE`)
3. **JSONL append** (append-only, sequenced **LAST**, explicitly never compensated)

### JSONL append sequencing rationale

`events.EventWriter.AppendEvent` appends with no dedup key and documents `ErrWriteIndeterminate`
on partial/fsync failure as unsafe to blindly retry. It is sequenced last so no subsequent step
requires compensating it. Its `Compensate` in F5 is a documented no-op (an audit trail is never
rewritten). `ErrWriteNotApplied` before any bytes are written leaves the call safely retryable.
`ErrWriteIndeterminate` is surfaced as an error without retrying, matching F5's existing
indeterminate-outcome rule. No new dedup/locking mechanism is introduced.

### EventWriter threading

`AssociateCommit` requires a non-nil `*events.EventWriter`. Callers are responsible for lifecycle
management:

* **MCP server**: passes its shared `s.Events` instance so concurrent calls serialize through that
  writer's mutex exactly as before.
* **CLI**: constructs a per-invocation writer via `core.NewWorkspaceEventWriter`, mirroring the
  established comment/checkpoint disposition plan.

The core function never mints an `EventWriter` itself (see compound rule
`docs/compound/2026-07-04-core-extraction-shared-eventwriter-append-serialization.md`).

## Three Converged Entry Points

After F6/U3, all three surfaces route through `core.AssociateCommit`:

| Surface | Caller |
|---|---|
| CLI `update --commit` | `internal/cli/update.go` RunE |
| MCP `update_item(commit=…)` | `internal/mcp/tools.go` `handleUpdateItem` |
| MCP `track_commit` | `internal/mcp/tools.go` `handleTrackCommit` |

All three now write all three representations.

## Message and Author: Deliberate CLI Limitation

The CLI `update --commit` flag is a convenience shorthand. It has no `--message` or `--author`
flags. When `core.AssociateCommit` is called from the CLI path, `message` and `author` are empty
strings. This is deliberate and documented.

**When to use each surface:**

* Use `MCP track_commit` when full commit metadata (SHA, message, author) is available — the tool
  accepts all three.
* Use `CLI update --commit` when only the SHA is needed and full commit metadata is unavailable or
  not required.
* Use `MCP update_item(commit=…)` when associating a commit SHA as part of a broader field update;
  message and author are stored as empty strings.

The `cli_param_gaps` field in `.autoharness/backlog-registry.yaml` documents this limitation so an
agent can see the deliberate gap without reading the source code.

## `--force-gates` and `--gate-base`: Intentional Human-Terminal Asymmetry

The CLI `update` command exposes `--force-gates` and `--gate-base` flags. These are **intentionally
absent from the MCP surface**. The established rule is that a fallback surface must never be more
dangerous than the surface it mirrors. Gate forcing is a deliberate human-at-a-terminal action with
explicit blast-radius rationale; it is never appropriate for an agent surface.

The registry's `update_task.cli_only_flags` field documents both flags with `human_terminal_only:
true` so an agent can tell deliberate asymmetry from drift. The `TestRegistryParity_ForceGatesAbsentFromMCPParams`
test in `internal/cli/registry_parity_test.go` (F6/U5) is the load-bearing regression assertion.

## Behavioral Parity Assertion (F6/U5)

`TestRegistryParity_GovernedOperationBehavioralParity` in `internal/cli/registry_parity_test.go`
asserts **behavioral** (not merely surface) parity for governed operations:

* The governed set is derived from the registry (`governed: true` marker), not a hand-list, so
  newly added governed operations enter the test automatically.
* The governed set must not be empty (gate 1: no vacuous pass).
* Every required `governed_name` is asserted against its exact registry operation key (gate 2:
  canonical governed operation names cannot be moved to an unrelated row or accidentally removed).
* For each governed operation with both a `mcp_tool` and `cli_command`, both surfaces are executed
  against equivalent fixtures and their observable persisted state is asserted identical.
* The current governed set covers commit association, checkpoint abandon/quarantine, comment append,
  and dependency add/remove; each new marker must have a named behavioral fixture before it can be
  added to the registry.
* A DENYLIST approach covers output-only fields that differ by design (message/author on the CLI
  fallback) so a newly governed operation enters the covered set automatically.

## Registry Markers

The following fields in `.autoharness/backlog-registry.yaml` carry this contract:

| Field | Applies to | Meaning |
|---|---|---|
| `governed: true` | `track_commit`, `append_comment`, `add_dependency`, `remove_dependency`, `abandon_checkpoint`, `quarantine_checkpoint` | Marks an operation as covered by behavioral parity |
| `governed_name: commit_association` | `track_commit` | Canonical name for the commit-association gate |
| `governed_name: comment_append` | `append_comment` | Requires parity for JSONL and indexed comment events |
| `governed_name: dependency_add` | `add_dependency` | Requires parity for persisted dependency edges |
| `governed_name: dependency_remove` | `remove_dependency` | Requires parity for dependency-edge removal |
| `cli_param_gaps.message` | `track_commit` | Documents that CLI stores empty string |
| `cli_param_gaps.author` | `track_commit` | Documents that CLI stores empty string |
| `cli_only_flags.force-gates.human_terminal_only` | `update_task` | Gate-forcing is operator-only |
| `cli_only_flags.gate-base.human_terminal_only` | `update_task` | Base-ref override is operator-only |

## Deprecated: `core.LinkCommit`

`core.LinkCommit` is deprecated. It is retained for backward compatibility but must not be used for
new code. It only writes `commit_links` and JSONL (no frontmatter scalar) and silently swallows
JSONL append failures. Use `core.AssociateCommit` instead.

