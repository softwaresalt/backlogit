---
title: "Queue view mismatch from blank CLI filters"
description: "Compound learning from fixing backlogit queue view after omitted flags were treated as active blank filters and masked real F013 status."
ms.date: 2026-04-05
ms.topic: reference
problem_type: config_issue
category: config_issue
component: cli
root_cause: type_mismatch
resolution_type: code_fix
severity: high
message: "Omitted queue filter flags became blank active filters and hid real queue items."
file_path: "internal/cli/queue_cmd.go, internal/core/queue.go"
resolved: true
tags:
  - queue-view
  - cli
  - filters
  - artifact-cleanup
  - F013
date: 2026-04-05
---

## Problem

`backlogit queue view` could report an empty or misleading queue even when the indexed item data was correct. The immediate symptom showed up while reconciling F013, where the feature looked unfinished from the queue view even though its implementation tasks and subtasks were already complete.

Two issues overlapped:

1. The CLI passed omitted `--type` and `--status` flags through as `[]string{""}`.
2. F013 still had stale duplicate child features and review artifacts in active queue lineage.

## Symptoms

* `backlogit queue view` returned zero results for an unfiltered query
* F013 appeared unfinished in queue output
* Direct item queries and review artifacts showed the implementation work was already done
* MCP `backlogit_get_queue` and the CLI `queue view` disagreed on the same logical request

## What Did Not Work

Inspecting only the F013 artifact statuses did not explain the mismatch. Archiving the duplicate artifacts alone would have improved the lineage, but it would not have fixed the empty queue results caused by blank CLI filter values being treated as active SQL filters.

## Solution

Fix the queue filter handling at both layers:

* In `internal/cli/queue_cmd.go`, only set `QueueFilter.Types` and `QueueFilter.Statuses` when the flag value is non-empty
* In `internal/core/queue.go`, trim and drop blank filter values before building the SQL query
* Add a regression test proving blank filters behave like no filter

### Before

```go
filter := &core.QueueFilter{
    Types:    []string{artifactType},
    Statuses: []string{status},
    GroupBy:  groupBy,
    SortBy:   sortBy,
}
```

```go
if len(filter.Statuses) > 0 {
    placeholders := make([]string, len(filter.Statuses))
    for i, s := range filter.Statuses {
        placeholders[i] = "?"
        args = append(args, s)
    }
    conditions = append(conditions, "status IN ("+strings.Join(placeholders, ",")+")")
}
```

### After

```go
filter := &core.QueueFilter{
    GroupBy: groupBy,
    SortBy:  sortBy,
}
if artifactType != "" {
    filter.Types = []string{artifactType}
}
if status != "" {
    filter.Statuses = []string{status}
}
```

```go
statuses := compactStrings(filter.Statuses)
types := compactStrings(filter.Types)

func compactStrings(values []string) []string {
    if len(values) == 0 {
        return nil
    }

    compacted := make([]string, 0, len(values))
    for _, value := range values {
        value = strings.TrimSpace(value)
        if value == "" {
            continue
        }
        compacted = append(compacted, value)
    }
    if len(compacted) == 0 {
        return nil
    }
    return compacted
}
```

After fixing the query path, archive the stale F013 artifacts that no longer represented active work:

* `F013.F001`
* `F013.F002`
* `F013.R001`
* `F013.R002`

## Why This Works

The CLI change restores the expected semantics of omitted flags: no filter means no constraint. The core `compactStrings` helper adds a second safety layer so future callers cannot accidentally construct `IN ('')` clauses from blank values.

The artifact cleanup then reconciles queue lineage with actual implementation state. Archived items no longer appear in default active queue queries, so duplicate child features and stale reviews stop making a completed feature look unfinished.

## Prevention

* Treat empty flag defaults as absent input, not as meaningful filter values
* Normalize filter slices before building SQL conditions
* Add regression tests for omitted flags and blank slice entries
* Reconcile duplicate harvested artifacts once their work has been absorbed by canonical tasks
* Archive stale review artifacts after a superseding review confirms no blocking work remains

## Related Solutions

* [Compound: GitHub Actions Workflow SHA Pinning (F013)](../github-actions/F013-workflow-sha-pinning.md) documents the underlying workflow fixes that originally made the F013 review artifacts stale.
