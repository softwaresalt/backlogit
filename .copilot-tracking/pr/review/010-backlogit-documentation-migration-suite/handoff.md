<!-- markdownlint-disable-file -->
# PR Review Handoff: 010-backlogit-documentation-migration-suite

## PR Overview

This review focused on the stash-plus-deliberation workflow, the MCP and CLI surfaces that expose it, and the agent and documentation changes that wire the new deliberate flow into the rest of backlogit. The final comment set contains four blocking reliability and contract issues plus eight moderate follow-ups that should be addressed before relying on the workflow as documented.

* Branch: `010-backlogit-documentation-migration-suite`
* Base Branch: `main`
* Total Files Changed: 296
* Total Review Comments: 12

## PR Comments Ready for Submission

### File: `internal/core/stash.go`

#### Comment 1 (Lines 233 through 239)

* Category: Reliability
* Severity: P1

`HarvestStashEntry` guards the later stash-index writes with `ws.DB != nil`, but the earlier `db.UpsertItem(ctx, ws.DB, artifact)` call is unconditional. A caller that builds a workspace without an open DB can panic here after the artifact file has already been created, which leaves the stores out of sync.

**Suggested Change**

```go
if ws.DB != nil {
	if err := db.UpsertItem(ctx, ws.DB, artifact); err != nil {
		return nil, fmt.Errorf("index harvested artifact: %w", err)
	}
}
```

#### Comment 2 (Lines 241 through 250)

* Category: Reliability
* Severity: P2

The harvest path creates the artifact before it updates `.backlogit/queue/.stash.md`. If the stash-file rewrite fails, the stash entry remains active and can be harvested again into a duplicate artifact. The write ordering needs a rollback path or a sequence that cannot leave the stash active after artifact creation succeeds.

**Suggested Change**

```go
// Either commit stash removal before durable artifact indexing, or add
// rollback logic that removes the created artifact when stash persistence fails.
```

#### Comment 3 (Lines 428 through 433)

* Category: Reliability
* Severity: P2

`expandStashEntry` suppresses `db.GetItem` failures entirely. That makes a transient database error indistinguishable from "linked deliberation not found", which hides a real operational failure from callers.

**Suggested Change**

```go
artifact, err := db.GetItem(ctx, ws.DB, entry.DeliberationID)
if err != nil {
	return StashEntryView{}, fmt.Errorf("load linked deliberation %s: %w", entry.DeliberationID, err)
}
view.Deliberation = artifact
```

### File: `internal/mcp/tools.go`

#### Comment 4 (Lines 235 through 253)

* Category: Conventions
* Severity: P1

The new stash MCP tools do not match the CLI defaults described in the docs. The CLI defaults `stash add --kind` and `stash harvest --type` to `task`, but the MCP schema makes both required. That breaks the advertised "same capability set" between the two surfaces.

**Suggested Change**

```go
mcplib.WithString("kind", mcplib.Description("Stash kind (feature, task, bug, epic)"), mcplib.DefaultString("task"))
mcplib.WithString("artifact_type", mcplib.Description("Target artifact type (feature, task, subtask)"), mcplib.DefaultString("task"))
```

#### Comment 5 (Lines 917 through 918)

* Category: Reliability
* Severity: P2

`handleFetchStash` maps every fetch failure to `internal`. Invalid priority input should come back as a validation-style MCP error rather than an internal server failure so callers can correct the request programmatically.

**Suggested Change**

```go
if err != nil {
	return ValidationFailed(err.Error()), nil
}
```

#### Comment 6 (Lines 937 through 940, 974 through 988, 1021 through 1022)

* Category: Reliability
* Severity: P2

The stash and deliberate handlers flatten domain errors into `internal` as well. Missing stash IDs, not-found conditions, and "already linked" conflicts are user-fixable request outcomes, so the MCP surface should preserve that distinction instead of reporting everything as a server fault.

**Suggested Change**

```go
if err != nil {
	return ValidationFailed(err.Error()), nil
}
```

### File: `internal/db/stash.go`

#### Comment 7 (Lines 129 through 152)

* Category: Reliability
* Severity: P1

`RehydrateStashIndex` clears `stash_links` and `stash_entries` before rebuilding them, but the whole rebuild is not wrapped in a transaction. Any failure after `ClearStashIndex` leaves the index empty or partially rebuilt until a later successful rehydration.

**Suggested Change**

```go
tx, err := database.BeginTx(ctx, nil)
if err != nil {
	return fmt.Errorf("begin stash rehydration tx: %w", err)
}
defer tx.Rollback()

// execute the stash index clear-and-rebuild sequence through the transaction

return tx.Commit()
```

#### Comment 8 (Lines 154 through 160)

* Category: Reliability
* Severity: P2

`mustParseTime` silently returns the zero time when parsing fails. That makes malformed timestamps appear valid and can distort stash ordering or linked-at metadata without surfacing any signal that the data was bad.

**Suggested Change**

```go
func parseTime(value string) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed, nil
	}
	return time.Time{}, fmt.Errorf("parse time %q", value)
}
```

### File: `internal/stash/stash.go`

#### Comment 9 (Lines 72 through 79)

* Category: Reliability
* Severity: P2

`ParseContent` silently drops entries when `NormalizePriority` or `NormalizeKind` fails. During reads or rehydration that becomes silent stash data loss, because malformed lines disappear with no diagnostic trail.

**Suggested Change**

```go
if err != nil {
	return nil, nil, fmt.Errorf("invalid stash entry %q: %w", line, err)
}
```

### File: `.github/agents/backlog-harvester.agent.md`

#### Comment 10 (Lines 84 through 92)

* Category: Maintainability
* Severity: P1

The harvester still parses the old impl-plan schema (`## Problem Statement`, `## Approach`, `## Key Decisions`, `## Constitution Check`), but `.github/skills/impl-plan/SKILL.md` now emits `## Problem Frame`, `## Decisions`, and `## Standards Check`. That means the deliberate -> plan -> harvest path can lose key sections or fail to decompose the plan correctly.

**Suggested Change**

```text
Update the parsing contract in backlog-harvester.agent.md to match the current
impl-plan output headings and field names.
```

### File: `docs/configuration.md`

#### Comment 11 (Lines 35 through 45, 131 through 176)

* Category: Documentation
* Severity: P2

The configuration guide still documents the old default artifact set and queue layout. `internal/config/defaults.go` now includes the built-in `deliberation` type and places it at queue level 1, so the docs are no longer describing the actual generated workspace.

**Suggested Change**

```text
Update the documented default artifact_types and queue_layout examples to include
deliberation and the level-1 placement shared with feature.
```

### File: `docs/workflow.md`

#### Comment 12 (Lines 36 through 37, 108 through 111)

* Category: Documentation
* Severity: P2

The workflow guide currently claims that `backlogit init` updates `.gitignore`, and its update example uses `--status in_review`. The implementation does neither: init does not edit ignore files, and the configured status enum uses `review`, not `in_review`.

**Suggested Change**

```text
Remove the .gitignore claim and change the status example from in_review to review.
```

## Review Summary by Category

* Security Issues: 0
* Code Quality: 8
* Convention Violations: 1
* Documentation: 2
* Maintainability: 1

## Review Artifacts

* Findings tracker: `.copilot-tracking/pr/review/010-backlogit-documentation-migration-suite/in-progress-review.md`
* PR reference: `.copilot-tracking/pr/review/010-backlogit-documentation-migration-suite/pr-reference.xml`
* Compound artifacts committed: no
* Memory checkpoints committed: no
