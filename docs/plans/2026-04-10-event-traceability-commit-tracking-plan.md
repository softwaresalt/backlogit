---
chunk_strategy: h1-h2-h3
description: ""
doc_type: plan
docline:
    date: 2026-04-10T00:00:00Z
    feature: 023-F
    origin: .backlogit/queue/011-DL.md
    review: .copilot-tracking/plan-review/2026-04-10-event-traceability-commit-tracking-plan-review.md
    shipment: 006-S
    status: reviewed
    task: 023.008-T
ingested_at: "2026-06-26T02:33:21Z"
schema_version: "1.0"
source: docs/exec-plans/2026-04-10-event-traceability-commit-tracking-plan.md
title: Event Traceability and Commit Tracking
---

## Problem Frame

Event log entries in `.backlogit/logs/{item-id}.jsonl` do not carry commit
context. The `Event` struct in `internal/events/stream.go` records `item_id`,
`timestamp`, `actor`, `event_type`, and a freeform `delta` map, but lifecycle
operations such as status moves, comments, and archives never populate commit
SHA information. The separate `LinkCommit` path in `internal/core/commits.go`
writes `commit_tracked` events to the same JSONL logs but operates
independently from general lifecycle event emission.

The requirement (origin: stash 834CCDB7, deliberation 011-DL) is to make
commit SHAs available on any event log entry so external tooling can
programmatically correlate backlogit state changes with git history.

## Requirements Trace

| # | Requirement | Origin |
|---|---|---|
| R1 | Event struct carries optional CommitSHA field | 011-DL Chosen Direction |
| R2 | Lifecycle operations pass CommitSHA when available | 011-DL Chosen Direction, Option A |
| R3 | JSONL filenames remain unchanged (no timestamp rename) | 011-DL Chosen Direction |
| R4 | Existing commit_tracked events remain compatible | 011-DL Notes |
| R5 | MCP tools accept optional commit_sha parameter on mutation operations | 023.008-T description |
| R6 | Event log JSON continues to include item_id as top-level field | 011-DL Open Questions #3 |

## Scope Boundaries

### In Scope

* Add `CommitSHA` field to the `Event` struct (`internal/events/stream.go`)
* Thread optional `commitSHA` through lifecycle operations in `internal/core/`
  (move, archive, comment, ship)
* Add optional `commit_sha` parameter to relevant MCP mutation tools
* Tests for new field serialization and lifecycle propagation

### Non-Goals

* External system synchronization (Azure DevOps, Jira, GitHub Issues) — separate
  feature per 011-DL Open Questions #2
* JSONL filename changes or log rotation
* Eager commit resolution (git lookups on every operation) — lazy population only
* CLI surface changes beyond what MCP already provides

### Deferred to Implementation

* Whether `AutoLinkCommits` should be extended to back-fill `CommitSHA` on
  historical events (likely not — append-only logs should not be rewritten)

## Implementation Units

### Unit 1: Extend Event struct with CommitSHA

**Files:** `internal/events/stream.go`
**Test files:** `internal/events/stream_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** Existing `Event` struct field layout; `json:"commit_sha,omitempty"` tag pattern
**Dependencies:** none

**Approach:**
Add `CommitSHA string \`json:"commit_sha,omitempty"\`` to the `Event` struct.
The field is optional — omitempty ensures backward compatibility with existing
log entries that lack it. No changes to `AppendEvent` logic needed; the JSON
marshaler handles the new field automatically.

**Verification:**
- Test that marshaling an Event with CommitSHA set produces the field in JSON
- Test that marshaling an Event without CommitSHA omits the field
- Test that unmarshaling existing JSONL entries without CommitSHA succeeds
- Existing `TestEventWriter_AppendEvent` continues to pass

### Unit 2: Thread commit SHA through core lifecycle operations

**Files:** `internal/core/archive.go`, `internal/core/shipment_lifecycle.go`, `internal/core/shipment.go`
**Test files:** `tests/contract/queue_tools_test.go` (extend existing lifecycle tests)
**Effort size:** medium
**Skill domain:** code
**Execution note:** characterization-first — run existing tests to establish baseline, then extend
**Patterns to follow:** Existing `LinkCommit` in `internal/core/commits.go` shows the pattern for creating events with commit context
**Dependencies:** Unit 1

**Approach:**
Add an optional `commitSHA string` parameter to lifecycle functions that emit
events (archive, ship). When non-empty, set `event.CommitSHA` before calling
`AppendEvent`. The `MoveArtifactStatus` path already emits events via the MCP
tool layer — extend that call site rather than the core function to keep the
core layer commit-agnostic.

**Verification:**
- Archive operation with commit SHA produces event with commit_sha field
- Ship operation with commit SHA produces event with commit_sha field
- Operations without commit SHA produce events without commit_sha field (backward compat)
- All existing contract tests pass without modification

### Unit 3: MCP tool parameter extension

**Files:** `internal/mcp/tools.go`
**Test files:** `tests/contract/queue_tools_test.go`
**Effort size:** small
**Skill domain:** code
**Execution note:** test-first
**Patterns to follow:** Existing optional parameter handling in `backlogit_track_commit` tool definition (line ~220)
**Dependencies:** Unit 2

**Approach:**
Add an optional `commit_sha` string parameter to `backlogit_move_item`,
`backlogit_archive_item`, and `backlogit_append_comment` MCP tool definitions.
Pass the value through to the underlying core function. The parameter is
optional and defaults to empty string.

**Verification:**
- MCP tool call with commit_sha populates the field on the emitted event
- MCP tool call without commit_sha works identically to current behavior
- Tool schema introspection shows the new optional parameter

## Dependency Graph

```
Unit 1 (Event struct) → Unit 2 (core lifecycle) → Unit 3 (MCP tools)
```

Linear chain. Each unit builds on the previous. No parallelism opportunities.

## Decisions

| # | Decision | Rationale | Alternatives Rejected |
|---|---|---|---|
| D1 | Lazy commit population | Avoids git binary dependency on every operation; aligns with 011-DL recommendation | Eager population (requires git on every lifecycle call) |
| D2 | omitempty on CommitSHA field | Backward compatible with existing JSONL entries | Required field (would break deserialization of existing logs) |
| D3 | Thread through MCP layer not core layer | Keeps core functions commit-agnostic; MCP is the agent-facing boundary | Core-level threading (adds parameter pollution to internal APIs) |
| D4 | No JSONL filename changes | Preserves existing log references; JSON timestamp field sufficient | Timestamped filenames (breaks references, adds rotation complexity) |

## Risks and Caveats

1. **Serialization compatibility:** Adding a new field to the Event struct is
   backward-compatible for reading (omitempty + zero value), but any code that
   does strict JSON schema validation on event logs would need updating.
2. **Test coverage scope:** Unit 2 modifies multiple files; characterization
   tests should be run before changes to catch regressions early.

## Learnings Applied

No directly applicable compound learnings found for this scope. The
orphaned-tasks compound learning (`docs/compound/workflow-issues/orphaned-tasks-without-parent-features-2026-04-10.md`)
informed the decision to keep 023.008-T under 023-F rather than creating a
standalone task.

## Standards Check

- **Type-Safe Go (Constitution I):** CommitSHA is a typed string field, not a
  loosely typed map entry
- **Test-First (Constitution III):** Each unit specifies test-first or
  characterization-first execution posture
- **CQRS (Constitution IV):** Event logs remain append-only; no Markdown
  source-of-truth changes needed
- **Workspace Containment:** All changes within `internal/` package boundaries
